package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// The error-text audit (goal error-checks-use-typed-errors): Go code decides
// on an error with errors.Is / errors.As and a typed error (a Code field, a
// RefusalCode method, refusal.Coded), never by searching the error's text
// for a code or a phrase. The words of an error are for a person, and a
// person's words may change. The audit walks the non-test Go of the module
// and refuses, in any function, a decision (strings.Contains, HasPrefix,
// HasSuffix, Index, EqualFold, CutPrefix..., a regexp match, == or !=, a
// switch) on an error's text: an .Error() call, or a value derived from one
// (TrimSpace, ToLower, Split..., an index, a local variable holding it).
// Other tools' text wrapped in an error (git, go) is allowed only as a
// named file:function exception with its reason.

// errorTextExceptions are the narrow file:function decisions on an error's
// text that are not our own error's words. Each names its reason.
var errorTextExceptions = map[string]string{
	"internal/adopt/adopt.go:hookPreflight":                               "git's own words, \"not a git repository\" (the structured-output design's U2 makes it an exit-code check)",
	"internal/goal/genesis.go:headTracksLedgerWithEnvironment":            "git's own words, \"not a git repository\" (the structured-output design's U2 makes it an exit-code check)",
	"internal/launch/unit_plan.go:readUnitPlan":                           "encoding/json names an unknown field only in its words; the field is shown, nothing is decided",
	"cmd/metasystem/intent_exception.go:intentInvocation.landException":   "a child process's output, not an error of this process: the structured-output design's U2 reads its envelope",
	"cmd/metasystem/test.go:printTestingRefusalAs":                        "splits a two-line message at its run: line to lay it out; decides no cause",
	"internal/steward/counselor_carriage.go:recordCounselorRenderFailure": "the same failure again: compares with the words it recorded so it records them once",
}

// errorTextDecision is one decision on an error's text the audit found.
type errorTextDecision struct {
	Site string // module-relative file:function
	Line int
	What string // the deciding call or operator
}

// errorTextStringTransforms keep an error's text an error's text.
var errorTextStringTransforms = map[string]bool{
	"TrimSpace": true, "ToLower": true, "ToUpper": true, "TrimPrefix": true, "TrimSuffix": true,
	"Trim": true, "TrimLeft": true, "TrimRight": true, "ReplaceAll": true, "Replace": true,
	"SplitN": true, "Split": true, "Fields": true, "SplitAfter": true, "Cut": true, "Join": true,
}

// errorTextDetailReaders return an error's code-first detail line, which is
// its text as records keep it (refusal.DetailOf, launch.ErrorDetail...).
var errorTextDetailReaders = map[string]bool{
	"DetailOf": true, "ErrorDetail": true, "RecordText": true, "RefusalDetail": true,
}

// errorTextStringDecisions decide on a string.
var errorTextStringDecisions = map[string]bool{
	"Contains": true, "ContainsAny": true, "ContainsRune": true, "HasPrefix": true, "HasSuffix": true,
	"Index": true, "IndexAny": true, "LastIndex": true, "EqualFold": true, "Count": true,
	"CutPrefix": true, "CutSuffix": true,
}

// errorTextRegexpDecisions are the regexp methods that match a string.
var errorTextRegexpDecisions = map[string]bool{
	"MatchString": true, "FindString": true, "FindStringSubmatch": true, "FindStringIndex": true,
	"FindAllString": true, "FindAllStringSubmatch": true, "FindStringSubmatchIndex": true,
}

// errorTextDecisions scans one parsed file.
func errorTextDecisions(fset *token.FileSet, rel string, file *ast.File) []errorTextDecision {
	var decisions []errorTextDecision
	for _, decl := range file.Decls {
		function, ok := decl.(*ast.FuncDecl)
		if !ok || function.Body == nil {
			continue
		}
		name := function.Name.Name
		if function.Recv != nil && len(function.Recv.List) == 1 {
			receiver := function.Recv.List[0].Type
			if star, ok := receiver.(*ast.StarExpr); ok {
				receiver = star.X
			}
			if index, ok := receiver.(*ast.IndexExpr); ok {
				receiver = index.X
			}
			if ident, ok := receiver.(*ast.Ident); ok {
				name = ident.Name + "." + name
			}
		}
		decisions = append(decisions, errorTextDecisionsIn(fset, rel+":"+name, function.Body)...)
	}
	return decisions
}

// errorTextDecisionsIn finds the decisions inside one function body.
func errorTextDecisionsIn(fset *token.FileSet, site string, body ast.Node) []errorTextDecision {
	tainted := map[string]bool{} // local names holding an error's text
	var isText func(expr ast.Expr) bool
	isText = func(expr ast.Expr) bool {
		switch typed := expr.(type) {
		case *ast.ParenExpr:
			return isText(typed.X)
		case *ast.Ident:
			return tainted[typed.Name]
		case *ast.IndexExpr:
			return isText(typed.X)
		case *ast.SliceExpr:
			return isText(typed.X)
		case *ast.CallExpr:
			switch fun := typed.Fun.(type) {
			case *ast.SelectorExpr:
				if fun.Sel.Name == "Error" && len(typed.Args) == 0 || errorTextDetailReaders[fun.Sel.Name] {
					return true
				}
				if pkg, ok := fun.X.(*ast.Ident); ok && pkg.Name == "strings" && errorTextStringTransforms[fun.Sel.Name] && len(typed.Args) > 0 {
					return isText(typed.Args[0])
				}
			case *ast.Ident:
				if fun.Name == "string" && len(typed.Args) == 1 {
					return isText(typed.Args[0])
				}
				return errorTextDetailReaders[fun.Name]
			}
		}
		return false
	}
	// Taint to a fixed point: a variable assigned from an error's text holds it.
	for changed := true; changed; {
		changed = false
		mark := func(lhs ast.Expr) {
			if ident, ok := lhs.(*ast.Ident); ok && ident.Name != "_" && !tainted[ident.Name] {
				tainted[ident.Name] = true
				changed = true
			}
		}
		ast.Inspect(body, func(node ast.Node) bool {
			switch typed := node.(type) {
			case *ast.AssignStmt:
				if len(typed.Lhs) == len(typed.Rhs) {
					for index := range typed.Lhs {
						if isText(typed.Rhs[index]) {
							mark(typed.Lhs[index])
						}
					}
				} else if len(typed.Rhs) == 1 && isText(typed.Rhs[0]) {
					for _, lhs := range typed.Lhs {
						mark(lhs)
					}
				}
			case *ast.ValueSpec:
				for index, ident := range typed.Names {
					if index < len(typed.Values) && isText(typed.Values[index]) {
						mark(ident)
					}
				}
			case *ast.RangeStmt:
				if isText(typed.X) && typed.Value != nil {
					mark(typed.Value)
				}
			}
			return true
		})
	}
	var decisions []errorTextDecision
	record := func(at token.Pos, what string) {
		decisions = append(decisions, errorTextDecision{Site: site, Line: fset.Position(at).Line, What: what})
	}
	ast.Inspect(body, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.CallExpr:
			fun, ok := typed.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, _ := fun.X.(*ast.Ident)
			isStrings := pkg != nil && pkg.Name == "strings" && errorTextStringDecisions[fun.Sel.Name]
			// strings.Cut decides on its separator, except at the first
			// line break, which only takes the first line to show.
			if pkg != nil && pkg.Name == "strings" && fun.Sel.Name == "Cut" && len(typed.Args) == 2 {
				literal, ok := typed.Args[1].(*ast.BasicLit)
				isStrings = !ok || literal.Value != `"\n"`
			}
			isRegexp := errorTextRegexpDecisions[fun.Sel.Name] || pkg != nil && pkg.Name == "regexp" && fun.Sel.Name == "MatchString"
			if !isStrings && !isRegexp {
				return true
			}
			for _, arg := range typed.Args {
				if isText(arg) {
					record(typed.Pos(), structuredExprText(fun))
					break
				}
			}
		case *ast.BinaryExpr:
			// An error's text against words is a decision; against "" (is
			// there a message) or another error's text (the same failure
			// again) it is not.
			if (typed.Op == token.EQL || typed.Op == token.NEQ) && isText(typed.X) != isText(typed.Y) {
				other := typed.Y
				if isText(typed.Y) {
					other = typed.X
				}
				if literal, ok := other.(*ast.BasicLit); !ok || literal.Value != `""` {
					record(typed.Pos(), typed.Op.String())
				}
			}
		case *ast.SwitchStmt:
			if typed.Tag != nil && isText(typed.Tag) {
				record(typed.Pos(), "switch")
			}
		}
		return true
	})
	return decisions
}

// scanErrorText walks the non-test Go under dirs, skipping testdata and the
// packages that only serve tests (fakes, fixtures, test helpers).
func scanErrorText(t *testing.T, moduleRoot string, dirs ...string) []errorTextDecision {
	t.Helper()
	var decisions []errorTextDecision
	fset := token.NewFileSet()
	for _, dir := range dirs {
		err := filepath.WalkDir(filepath.Join(moduleRoot, dir), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(moduleRoot, path)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			if entry.IsDir() {
				if name := entry.Name(); name == "testdata" || strings.HasPrefix(name, ".") || messageTraceFixtureDir.MatchString(rel) {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
			if err != nil {
				return err
			}
			decisions = append(decisions, errorTextDecisions(fset, rel, file)...)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return decisions
}

func TestAuditNoDecisionOnErrorText(t *testing.T) {
	t.Parallel()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	var refused []string
	for _, decision := range scanErrorText(t, root, "cmd", "internal") {
		seen[decision.Site] = true
		if errorTextExceptions[decision.Site] != "" {
			continue
		}
		refused = append(refused, decision.Site+" (line "+strconv.Itoa(decision.Line)+", "+decision.What+")")
	}
	slices.Sort(refused)
	if os.Getenv("ERROR_TEXT_DUMP") != "" {
		for _, line := range refused {
			t.Log(line)
		}
	}
	if len(refused) != 0 {
		t.Errorf("%d decisions search an error's text instead of its type:\n  %s\n"+
			"decide with errors.Is or errors.As on a typed error (refusal.Coded and refusal.CodeOf, a sentinel, a Code field), or, "+
			"for another tool's words wrapped in an error, add the file:function with a reason to errorTextExceptions in "+
			"cmd/metasystem/audit_error_text_test.go",
			len(refused), strings.Join(refused, "\n  "))
	}
	for site := range errorTextExceptions {
		if !seen[site] {
			t.Errorf("exception %s no longer decides on an error's text: delete it from errorTextExceptions", site)
		}
	}
}

// TestAuditErrorTextFixtures: the audit catches a decision on an error's
// text directly, through a variable and through a transform, and passes
// errors.Is / errors.As and text only shown to a person.
func TestAuditErrorTextFixtures(t *testing.T) {
	t.Parallel()
	source := `package fixture

import (
	"errors"
	"regexp"
	"strings"
)

var pattern = regexp.MustCompile("x")

func direct(err error) bool { return strings.Contains(err.Error(), "UNIT_BUSY") }

func throughVariable(err error) bool {
	message := strings.TrimSpace(err.Error())
	lower := strings.ToLower(message)
	return strings.HasPrefix(lower, "no capability")
}

func equal(err error) bool { return err.Error() == "not a JSON object" }

func matched(err error) bool { return pattern.MatchString(err.Error()) }

func switched(err error) int {
	switch err.Error() {
	case "a":
		return 1
	}
	return 0
}

func firstToken(err error) string {
	first := strings.Fields(strings.SplitN(err.Error(), "\n", 2)[0])
	if first[0] == strings.ToUpper(first[0]) && strings.Contains(first[0], "_") {
		return first[0]
	}
	return ""
}

var errKnown = errors.New("known")

func typed(err error) bool {
	var coded interface{ RefusalCode() string }
	return errors.Is(err, errKnown) || errors.As(err, &coded)
}

func detailFact(err error) string {
	if _, rest, found := strings.Cut(refusal.DetailOf(err), "ref="); found {
		return rest
	}
	return ""
}

func shown(err error) string {
	line, _, _ := strings.Cut(err.Error(), "\n")
	return "refused: " + strings.TrimSpace(line)
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "fixture.go", source, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	var sites []string
	for _, decision := range errorTextDecisions(fset, "fixture.go", file) {
		sites = append(sites, decision.Site)
	}
	want := []string{"fixture.go:direct", "fixture.go:throughVariable", "fixture.go:equal", "fixture.go:matched",
		"fixture.go:switched", "fixture.go:firstToken", "fixture.go:detailFact"}
	if !slices.Equal(sites, want) {
		t.Fatalf("caught %v, want %v", sites, want)
	}
}
