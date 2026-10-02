package applaunch

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestNoTestWaitsForReadinessOnTheWallClock holds the readiness waits of
// every test in the module to events. A launcher started by a test waits for
// its supervisor's one answer or its exit (WaitForReport), never a duration;
// an app verb run by a test is given the same wait; and the readiness and
// rejoin waits a test calls end on a channel the test owns (nil, or one it
// fires), never on a timer or the exported Rejoin's clock. A readiness clock
// on a test's path decides the test on a loaded host instead of the code.
func TestNoTestWaitsForReadinessOnTheWallClock(t *testing.T) {
	t.Parallel()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	findings, err := readinessClockFindings(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, finding := range findings {
		t.Error(finding)
	}
}

func readinessClockFindings(root string) ([]string, error) {
	var findings []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		name := entry.Name()
		if entry.IsDir() {
			if path != root && (name == "artifacts" || name == "testdata" || name == "vendor" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, "_test.go") || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
			return nil
		}
		files := token.NewFileSet()
		file, err := parser.ParseFile(files, path, nil, 0)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		at := func(node ast.Node) string {
			return fmt.Sprintf("%s:%d", filepath.ToSlash(relative), files.Position(node.Pos()).Line)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			switch node := node.(type) {
			case *ast.CallExpr:
				switch calledName(node.Fun) {
				case "LaunchSupervisor":
					if len(node.Args) >= 3 && !namesWaitForReport(node.Args[2]) {
						findings = append(findings, at(node)+": a test's LaunchSupervisor waits for its supervisor's answer or exit (WaitForReport), never a duration")
					}
				case "Rejoin":
					findings = append(findings, at(node)+": a test's rejoin ends on a channel it owns (rejoin), never on Rejoin's clock")
				case "AwaitReady", "rejoin":
					if len(node.Args) > 0 {
						if _, isCall := node.Args[len(node.Args)-1].(*ast.CallExpr); isCall {
							findings = append(findings, at(node)+": a test's readiness wait ends on a channel it owns (nil or one it fires), never a timer")
						}
					}
				}
			case *ast.AssignStmt:
				for index, left := range node.Lhs {
					if selector, ok := left.(*ast.SelectorExpr); ok && selector.Sel.Name == "appSupervisorWait" && index < len(node.Rhs) && !namesWaitForReport(node.Rhs[index]) {
						findings = append(findings, at(node)+": an app verb a test runs waits for its supervisor's answer or exit (applaunch.WaitForReport), never a duration")
					}
				}
			case *ast.KeyValueExpr:
				if key, ok := node.Key.(*ast.Ident); ok && key.Name == "appSupervisorWait" && !namesWaitForReport(node.Value) {
					findings = append(findings, at(node)+": an app verb a test runs waits for its supervisor's answer or exit (applaunch.WaitForReport), never a duration")
				}
			}
			return true
		})
		return nil
	})
	sort.Strings(findings)
	return findings, err
}

func calledName(function ast.Expr) string {
	switch function := function.(type) {
	case *ast.Ident:
		return function.Name
	case *ast.SelectorExpr:
		return function.Sel.Name
	}
	return ""
}

func namesWaitForReport(expression ast.Expr) bool {
	switch expression := expression.(type) {
	case *ast.Ident:
		return expression.Name == "WaitForReport"
	case *ast.SelectorExpr:
		return expression.Sel.Name == "WaitForReport"
	}
	return false
}
