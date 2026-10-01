package pattern

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// untypedFields are the free-text fields of the records the readers see:
// a history entry's Detail, a commit subject, a failure's or a helm's
// words. No reader or detector reads them.
var untypedFields = map[string]bool{"Detail": true, "Subject": true, "Failure": true, "Reason": true, "StartReason": true}

// untypedCalls read log text or a tick's last error.
var untypedCalls = map[string]bool{"LastErrorLine": true, "LastTickErrorLine": true, "LastErrorPath": true, "TickErrorPath": true, "TickProblem": true}

// untypedFormats is a git pretty format that prints a commit's subject,
// body or notes.
var untypedFormats = regexp.MustCompile(`%(s|b|B|f|N|w)`)

// TestPatternReadersTypedOnly holds design §4: every reader and detector in
// this package decides on typed fields only, never Detail, a commit subject,
// a log file or last-tick-error.
func TestPatternReadersTypedOnly(t *testing.T) {
	t.Parallel()
	sources, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, source := range sources {
		if strings.HasSuffix(source, "_test.go") {
			continue
		}
		data, err := os.ReadFile(source)
		if err != nil {
			t.Fatal(err)
		}
		checked++
		for _, problem := range untypedReads(t, source, data) {
			t.Error(problem)
		}
	}
	if checked == 0 {
		t.Fatal("no reader source was checked")
	}
}

func untypedReads(t *testing.T, name string, data []byte) []string {
	t.Helper()
	files := token.NewFileSet()
	file, err := parser.ParseFile(files, name, data, 0)
	if err != nil {
		t.Fatal(err)
	}
	var problems []string
	report := func(node ast.Node, what string) {
		problems = append(problems, files.Position(node.Pos()).String()+": "+what)
	}
	ast.Inspect(file, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.SelectorExpr:
			if untypedFields[node.Sel.Name] {
				report(node, "reads the free-text field "+node.Sel.Name)
			}
			if untypedCalls[node.Sel.Name] {
				report(node, "reads log text through "+node.Sel.Name)
			}
		case *ast.BasicLit:
			if node.Kind != token.STRING {
				return true
			}
			value, err := strconv.Unquote(node.Value)
			if err != nil {
				return true
			}
			switch {
			case strings.HasSuffix(value, ".log") || strings.Contains(value, "last-tick-error") || strings.Contains(value, "last-error"):
				report(node, "names a log file "+strconv.Quote(value))
			case (strings.HasPrefix(value, "--format") || strings.HasPrefix(value, "--pretty") || strings.HasPrefix(value, "format:")) && untypedFormats.MatchString(value):
				report(node, "asks git for a commit's words "+strconv.Quote(value))
			}
		}
		return true
	})
	return problems
}

// The audit itself catches each kind it names.
func TestPatternReadersTypedOnlyCatches(t *testing.T) {
	t.Parallel()
	for _, source := range []string{
		"package p\nfunc f(e struct{ Detail string }) string { return e.Detail }\n",
		"package p\nvar format = \"--format=%H%x00%s\"\n",
		"package p\nvar path = \"helm-yields.log\"\n",
		"package p\nfunc f(l interface{ LastTickErrorLine(string) string }) string { return l.LastTickErrorLine(\"x\") }\n",
	} {
		if problems := untypedReads(t, "fixture.go", []byte(source)); len(problems) == 0 {
			t.Errorf("the audit passed %q", source)
		}
	}
}
