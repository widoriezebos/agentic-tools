package dispatch

import (
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"testing"
)

type briefBoundsCase struct {
	name, text, prefix, header, detail string
	boundary                           []string
	ceiling                            int64
}

var headerRecognitionCases = []briefBoundsCase{{name: "case", text: "boundary: [\"a\"]\nCeiling: 1", header: "Boundary", detail: "required with Ceiling"}, {name: "column", text: " Boundary: [\"a\"]\nCeiling: 1", header: "Boundary", detail: "required with Ceiling"}, {name: "physical-line", text: "Boundary: [\n\"a\"]\nCeiling: 1", header: "Boundary", detail: "expected a JSON array of paths"}, {name: "trim", text: "Boundary:\t [\"a\"] \r\nCeiling: 01 \t", boundary: []string{"a"}, ceiling: 1}, {name: "fence", text: "```\nBoundary: [\"a\"]\nCeiling: 1\n```", boundary: []string{"a"}, ceiling: 1}}
var presenceCases = []briefBoundsCase{{name: "neither", text: "Working Mode: implement"}, {name: "both", text: boundedBrief(`["a"]`, "2"), boundary: []string{"a"}, ceiling: 2}, {name: "empty-array", text: boundedBrief(`[]`, "2"), boundary: []string{}, ceiling: 2}, {name: "duplicates", text: boundedBrief(`["a","a"]`, "2"), boundary: []string{"a", "a"}, ceiling: 2}}
var partialHeaderCases = []briefBoundsCase{{name: "boundary-only", text: "Boundary: []", header: "Ceiling", detail: "required with Boundary"}, {name: "ceiling-only", text: "Ceiling: 1", header: "Boundary", detail: "required with Ceiling"}}
var boundarySyntaxCases = []briefBoundsCase{{name: "empty", text: boundedBrief("", "1"), header: "Boundary", detail: "expected a JSON array of paths"}, {name: "null", text: boundedBrief("null", "1"), header: "Boundary", detail: "expected a JSON array of paths"}, {name: "object", text: boundedBrief("{}", "1"), header: "Boundary", detail: "expected a JSON array of paths"}, {name: "nonstring", text: boundedBrief("[1]", "1"), header: "Boundary", detail: "expected a JSON array of paths"}, {name: "trailing-json", text: boundedBrief("[] []", "1"), header: "Boundary", detail: "expected a JSON array of paths"}, {name: "duplicate-boundary", text: "Boundary: []\nBoundary: []\nCeiling: 1", header: "Boundary", detail: "header occurs more than once"}}
var projectionParseCases = []briefBoundsCase{{name: "missing-prefix", text: boundaryMemberBrief("other/a"), prefix: "metasystem", header: "Boundary", detail: invalidMemberDetail("other/a")}, {name: "unsupported-prefix", text: boundedBrief("[]", "1"), prefix: "meta[system", header: "Boundary", detail: "installation prefix contains unsupported pattern bytes"}, {name: "literal-directory", text: boundaryMemberBrief("metasystem/a[/"), prefix: "metasystem", boundary: []string{"metasystem/a[/"}, ceiling: 1}, {name: "malformed-pattern", text: boundaryMemberBrief("metasystem/a["), prefix: "metasystem", header: "Boundary", detail: invalidMemberDetail("metasystem/a[")}}
var precedenceCases = []briefBoundsCase{{name: "duplicates-first", text: "Ceiling: bad\nCeiling: worse", header: "Ceiling", detail: "header occurs more than once"}, {name: "boundary-duplicate-first", text: "Boundary: bad\nBoundary: worse\nCeiling: bad\nCeiling: worse", header: "Boundary", detail: "header occurs more than once"}, {name: "pair-before-value", text: "Boundary: bad", header: "Ceiling", detail: "required with Boundary"}, {name: "boundary-before-ceiling", text: boundedBrief("bad", "bad"), header: "Boundary", detail: "expected a JSON array of paths"}}

func TestBriefBoundsHeaderRecognition(t *testing.T)     { runBriefBoundsCases(t, headerRecognitionCases) }
func TestBriefBoundsPresence(t *testing.T)              { runBriefBoundsCases(t, presenceCases) }
func TestBriefBoundsRejectsPartialHeaders(t *testing.T) { runBriefBoundsCases(t, partialHeaderCases) }
func TestBriefBoundsSyntax(t *testing.T)                { runBriefBoundsCases(t, boundarySyntaxCases) }
func TestBriefBoundsCeilingSyntax(t *testing.T) {
	cases := []briefBoundsCase{}
	for _, value := range []struct{ name, text string }{{"plus", "+1"}, {"fraction", "1.5"}, {"suffix", "1 lines"}, {"placeholder", "<n>"}, {"empty", ""}, {"non-ascii", "١"}, {"overflow", "9223372036854775808"}} {
		cases = append(cases, briefBoundsCase{name: value.name, text: boundedBrief("[]", value.text), header: "Ceiling", detail: briefCeilingDetail})
	}
	cases = append(cases, briefBoundsCase{name: "duplicate", text: "Boundary: []\nCeiling: 1\nCeiling: 2", header: "Ceiling", detail: "header occurs more than once"}, briefBoundsCase{name: "zero", text: boundedBrief("[]", "0"), boundary: []string{}},
		briefBoundsCase{name: "leading-zeroes", text: boundedBrief("[]", "0002"), boundary: []string{}, ceiling: 2},
		briefBoundsCase{name: "maximum", text: boundedBrief("[]", "9223372036854775807"), boundary: []string{}, ceiling: 9223372036854775807})
	runBriefBoundsCases(t, cases)
	t.Run("minus", func(t *testing.T) {
		_, err := ParseBriefBounds([]byte(boundedBrief("[]", "-1")), "")
		requireBoundsRefusal(t, err, "Ceiling", briefCeilingDetail)
		negative := int64(-1)
		requireBoundsRefusal(t, ValidateBriefBounds(BriefBounds{Boundary: []string{}, Ceiling: &negative}), "Ceiling", briefCeilingDetail)
	})
}
func TestBriefBoundsPathSyntax(t *testing.T) {
	cases := []briefBoundsCase{}
	for _, member := range []struct{ name, path string }{{"absolute", "/a"}, {"empty", ""}, {"nul", "a\x00b"}, {"empty-component", "a//b"}, {"dot", "a/./b"}, {"parent", "a/../b"}} {
		cases = append(cases, briefBoundsCase{name: member.name, text: boundaryMemberBrief(member.path), header: "Boundary", detail: invalidMemberDetail(member.path)})
	}
	cases = append(cases, briefBoundsCase{name: "member-whitespace", text: boundaryMemberBrief(" a \t"), boundary: []string{" a \t"}, ceiling: 1})
	runBriefBoundsCases(t, cases)
}
func TestBriefBoundsProjectionSyntax(t *testing.T) {
	for _, tc := range []struct {
		name, member, prefix, project, detail string
		directory                             bool
	}{
		{"root", "a", "", "a", "", false}, {"nested", "metasystem/a", "metasystem", "a", "", false},
		{"double-prefix", "metasystem/metasystem/a", "metasystem", "metasystem/a", "", false}, {"whole-project", "metasystem/", "metasystem", "", "", true},
		{"unsupported-prefix-direct", "meta[system/a", "meta[system", "", "installation prefix contains unsupported pattern bytes", false}, {"projection-before-pattern", `meta\[system/a`, "meta[system", "", "installation prefix contains unsupported pattern bytes", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			project, directory, err := ProjectBriefBoundary(tc.member, tc.prefix)
			if tc.detail != "" {
				requireBoundsRefusal(t, err, "Boundary", tc.detail)
				return
			}
			require(t, err == nil && project == tc.project && directory == tc.directory, "projection = %q, %v, %v", project, directory, err)
		})
	}
	runBriefBoundsCases(t, projectionParseCases)
	t.Run("exact", func(t *testing.T) { require(t, !briefBoundaryIsPattern("a"), "exact member classified as pattern") })
	t.Run("backslash-pattern", func(t *testing.T) {
		require(t, briefBoundaryIsPattern(`a\b`), "backslash member classified as concrete")
	})
}
func TestValidateBriefBounds(t *testing.T) {
	one := int64(1)
	for _, tc := range []struct {
		name           string
		bounds         BriefBounds
		header, detail string
	}{
		{"boundary-only", BriefBounds{Boundary: []string{}}, "Ceiling", "required with Boundary"}, {"ceiling-only", BriefBounds{Ceiling: &one}, "Boundary", "required with Ceiling"},
		{"invalid-member", BriefBounds{Boundary: []string{"/a"}, Ceiling: &one}, "Boundary", invalidMemberDetail("/a")},
	} {
		t.Run(tc.name, func(t *testing.T) { requireBoundsRefusal(t, ValidateBriefBounds(tc.bounds), tc.header, tc.detail) })
	}
}
func TestBriefModeOnly(t *testing.T) {
	dir := t.TempDir()
	brief := dir + "/brief"
	err := os.WriteFile(brief, []byte("Working Mode: implement\nBoundary: []"), 0o600)
	require(t, err == nil, "write brief: %v", err)
	for _, tc := range []struct{ name, path, want string }{
		{"partial-pair", brief, "implement"}, {"read-failure", dir + "/missing", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mode, err := BriefModeOnly(tc.path)
			require(t, (tc.want != "" && err == nil && mode == tc.want) || (tc.want == "" && err != nil), "mode = %q, error = %v", mode, err)
		})
	}
}
func TestBriefBoundsErrorPrecedence(t *testing.T) {
	runBriefBoundsCases(t, precedenceCases)
	t.Run("mode-first-without-authority", func(t *testing.T) {
		_, err := admitBriefBytes([]byte(boundedBrief("bad", "bad")), func() (string, error) { return "", nil }, true, nil)
		var bounds *BriefBoundsRefusal
		require(t, err != nil && !errors.As(err, &bounds), "error = %v", err)
	})
}
func TestBriefAdmission(t *testing.T) {
	partial := t.TempDir() + "/partial"
	require(t, os.WriteFile(partial, []byte("Working Mode: implement\nBoundary: []"), 0o600) == nil, "write partial brief")
	t.Run("brief-mode-paired-bounds", func(t *testing.T) {
		_, err := BriefMode(partial)
		requireBoundsRefusal(t, err, "Ceiling", "required with Boundary")
	})
	t.Run("brief-authority-paired-bounds", func(t *testing.T) {
		err := ValidateBriefAuthority(partial, t.TempDir(), t.TempDir())
		requireBoundsRefusal(t, err, "Ceiling", "required with Boundary")
	})
	t.Run("read-failure", func(t *testing.T) {
		_, err := ReadBriefAdmission(t.TempDir()+"/missing", "", true, nil)
		require(t, err != nil, "missing brief admitted")
	})
	t.Run("authority-read-failure", func(t *testing.T) {
		_, err := ReadBriefAdmission(t.TempDir()+"/missing", "", false, func([]byte, BriefBounds) error { return nil })
		require(t, err != nil, "missing authority brief admitted")
	})
	t.Run("exact-bytes", func(t *testing.T) {
		data := []byte("Working Mode: implement\nBoundary: []\nCeiling: 0")
		var checked []byte
		var checkedBounds BriefBounds
		admission, err := admitBriefBytes(data, func() (string, error) { return "", nil }, true, func(got []byte, bounds BriefBounds) error {
			checked, checkedBounds = append([]byte(nil), got...), bounds
			return nil
		})
		require(t, err == nil && reflect.DeepEqual(admission.Bytes, data) && reflect.DeepEqual(checked, data) && reflect.DeepEqual(checkedBounds, admission.Bounds), "admission = %+v, checked bytes = %q, checked bounds = %+v, error = %v", admission, checked, checkedBounds, err)
	})
	t.Run("parser-ownership", func(t *testing.T) {
		file, err := parser.ParseFile(token.NewFileSet(), "brief.go", nil, 0)
		require(t, err == nil, "parse brief.go: %v", err)
		require(t, boundsParserCalls(file, "admitBriefBytes") == 1 && boundsParserCalls(file, "BriefModeOnly") == 0 && boundsParserCalls(file, "ReadBriefAdmission") == 0 && boundsParserCalls(file, "ReadBriefAdmissionAtRoot") == 0, "bounds parser ownership changed")
	})
}
func boundsParserCalls(file *ast.File, functionName string) (calls int) {
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Name.Name != functionName {
			continue
		}
		ast.Inspect(function.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			name, direct := call.Fun.(*ast.Ident)
			if direct && (name.Name == "parseBriefBounds" || name.Name == "ParseBriefBounds") {
				calls++
			}
			return true
		})
	}
	return calls
}
func runBriefBoundsCases(t *testing.T, cases []briefBoundsCase) {
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bounds, err := ParseBriefBounds([]byte(tc.text), tc.prefix)
			if tc.header != "" {
				requireBoundsRefusal(t, err, tc.header, tc.detail)
				return
			}
			require(t, err == nil, "parse bounds: %v", err)
			if tc.boundary == nil {
				require(t, bounds.Boundary == nil && bounds.Ceiling == nil, "bounds = %+v", bounds)
				return
			}
			require(t, bounds.Boundary != nil && bounds.Ceiling != nil && reflect.DeepEqual(bounds.Boundary, tc.boundary) && *bounds.Ceiling == tc.ceiling, "bounds = %+v", bounds)
		})
	}
}
func requireBoundsRefusal(t *testing.T, err error, header, detail string) {
	t.Helper()
	var refusal *BriefBoundsRefusal
	require(t, errors.As(err, &refusal) && refusal.Header == header && refusal.Detail == detail && err.Error() == "BRIEF_BOUNDS_INVALID: "+header+": "+detail, "error = %#v, want %s: %s", err, header, detail)
}
func require(t *testing.T, condition bool, format string, args ...any) {
	t.Helper()
	if !condition {
		t.Fatalf(format, args...)
	}
}
func boundedBrief(boundary, ceiling string) string {
	return "Boundary: " + boundary + "\nCeiling: " + ceiling
}
func boundaryMemberBrief(member string) string {
	encoded, _ := json.Marshal([]string{member})
	return boundedBrief(string(encoded), "1")
}
func invalidMemberDetail(member string) string {
	encoded, _ := json.Marshal(member)
	return "invalid path or pattern " + string(encoded)
}
