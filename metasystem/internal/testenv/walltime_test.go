package testenv

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const walltimeAllowanceUpdate = "WALLTIME_ALLOWANCE_UPDATE"

var guardedWalltimeCalls = map[string]map[string]bool{
	"context": {
		"WithDeadline": true,
		"WithTimeout":  true,
	},
	"time": {
		"After":     true,
		"AfterFunc": true,
		"NewTicker": true,
		"NewTimer":  true,
		"Sleep":     true,
		"Tick":      true,
	},
}

type walltimeAllowance struct {
	count  int
	reason string
}

type walltimeCall struct {
	name     string
	position token.Position
	called   bool
}

// TestNoTestWaitsOnWallTime counts calls and function values that reference
// time.Sleep, time.After, time.AfterFunc, time.NewTimer, time.NewTicker,
// time.Tick, context.WithTimeout, or context.WithDeadline. Set
// WALLTIME_ALLOWANCE_UPDATE=1 to rewrite the allowance after removing a
// reference; update mode always fails so the rewritten file must be reviewed.
func TestNoTestWaitsOnWallTime(t *testing.T) {
	t.Parallel()

	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	callsByPath, err := findTestWalltimeCalls(root)
	if err != nil {
		t.Fatal(err)
	}
	allowancePath := filepath.Join(root, "internal", "testenv", "testdata", "walltime-allowance.tsv")
	allowances, problems := readWalltimeAllowances(allowancePath, root)
	if os.Getenv(walltimeAllowanceUpdate) == "1" {
		if err := writeWalltimeAllowances(allowancePath, callsByPath, allowances); err != nil {
			t.Fatal(err)
		}
		relative, err := filepath.Rel(root, allowancePath)
		if err != nil {
			t.Fatal(err)
		}
		t.Fatalf("%s=1 rewrote %s; update mode always fails", walltimeAllowanceUpdate, filepath.ToSlash(relative))
	}
	for _, problem := range problems {
		t.Error(problem)
	}
	if len(problems) != 0 {
		return
	}

	paths := make(map[string]bool, len(callsByPath)+len(allowances))
	for path := range callsByPath {
		paths[path] = true
	}
	for path := range allowances {
		paths[path] = true
	}
	orderedPaths := make([]string, 0, len(paths))
	for path := range paths {
		orderedPaths = append(orderedPaths, path)
	}
	sort.Strings(orderedPaths)

	for _, path := range orderedPaths {
		calls := callsByPath[path]
		allowed := allowances[path].count
		switch {
		case len(calls) > allowed:
			t.Errorf("%s: %d allowed, %d found; wall-clock calls may not increase:\n%s", path, allowed, len(calls), describeWalltimeCalls(path, calls))
		case len(calls) < allowed:
			t.Errorf("%s: %d allowed, %d found; lower the allowance number to %d:\n%s", path, allowed, len(calls), len(calls), describeWalltimeCalls(path, calls))
		}
	}
}

func findTestWalltimeCalls(root string) (map[string][]walltimeCall, error) {
	callsByPath := make(map[string][]walltimeCall)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != root && skippedDirectory(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(entry.Name(), ".") || strings.HasPrefix(entry.Name(), "_") || !strings.HasSuffix(entry.Name(), "_test.go") {
			return nil
		}

		fileSet := token.NewFileSet()
		file, err := parser.ParseFile(fileSet, path, nil, 0)
		if err != nil {
			return err
		}
		aliases := make(map[string]string)
		dotNames := make(map[string]string)
		for importPath := range guardedWalltimeCalls {
			for alias := range importedAliases(file, importPath, importPath) {
				if alias == "." {
					for name := range guardedWalltimeCalls[importPath] {
						dotNames[name] = importPath
					}
					continue
				}
				aliases[alias] = importPath
			}
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		callees := make(map[token.Pos]bool)
		selectorNames := make(map[token.Pos]bool)
		ast.Inspect(file, func(node ast.Node) bool {
			switch expression := node.(type) {
			case *ast.CallExpr:
				if position := walltimeReferencePosition(expression.Fun); position.IsValid() {
					callees[position] = true
				}
			case *ast.SelectorExpr:
				selectorNames[expression.Sel.Pos()] = true
			}
			return true
		})
		ast.Inspect(file, func(node ast.Node) bool {
			switch reference := node.(type) {
			case *ast.SelectorExpr:
				identifier, ok := reference.X.(*ast.Ident)
				if !ok || identifier.Obj != nil {
					return true
				}
				importPath, ok := aliases[identifier.Name]
				if !ok || !guardedWalltimeCalls[importPath][reference.Sel.Name] {
					return true
				}
				callsByPath[relative] = append(callsByPath[relative], walltimeCall{
					name:     importPath + "." + reference.Sel.Name,
					position: fileSet.Position(reference.Sel.Pos()),
					called:   callees[reference.Sel.Pos()],
				})
				return true
			case *ast.Ident:
				if reference.Obj != nil || selectorNames[reference.Pos()] {
					return true
				}
				importPath, ok := dotNames[reference.Name]
				if !ok || !guardedWalltimeCalls[importPath][reference.Name] {
					return true
				}
				callsByPath[relative] = append(callsByPath[relative], walltimeCall{
					name:     importPath + "." + reference.Name,
					position: fileSet.Position(reference.Pos()),
					called:   callees[reference.Pos()],
				})
			}
			return true
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	for path := range callsByPath {
		sort.Slice(callsByPath[path], func(left, right int) bool {
			return callsByPath[path][left].position.Offset < callsByPath[path][right].position.Offset
		})
	}
	return callsByPath, nil
}

func walltimeReferencePosition(expression ast.Expr) token.Pos {
	for {
		parenthesized, ok := expression.(*ast.ParenExpr)
		if !ok {
			break
		}
		expression = parenthesized.X
	}
	switch reference := expression.(type) {
	case *ast.SelectorExpr:
		return reference.Sel.Pos()
	case *ast.Ident:
		return reference.Pos()
	default:
		return token.NoPos
	}
}

func TestWalltimeCounterIncludesFunctionValues(t *testing.T) {
	t.Parallel()

	const source = `package fixture
import (
	. "context"
	tm "time"
	"time"
)
func fixture() {
	time.Sleep(0)
	_ = time.After
	_ = tm.NewTimer
	(time.Tick)(0)
	_ = WithDeadline
	time := struct{ Sleep func(int) }{}
	_ = time.Sleep
}
`
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "fixture_test.go"), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	callsByPath, err := findTestWalltimeCalls(root)
	if err != nil {
		t.Fatal(err)
	}
	calls := callsByPath["fixture_test.go"]
	got := make([]string, 0, len(calls))
	for _, call := range calls {
		name := call.name
		if !call.called {
			name += " (value)"
		}
		got = append(got, fmt.Sprintf("%s:%d", name, call.position.Line))
	}
	want := []string{
		"time.Sleep:8",
		"time.After (value):9",
		"time.NewTimer (value):10",
		"time.Tick:11",
		"context.WithDeadline (value):12",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("wall-time references=%q, want %q", got, want)
	}
}

func readWalltimeAllowances(path, root string) (map[string]walltimeAllowance, []string) {
	file, err := os.Open(path)
	if err != nil {
		return nil, []string{err.Error()}
	}
	defer file.Close()

	allowances := make(map[string]walltimeAllowance)
	var problems []string
	previousPath := ""
	scanner := bufio.NewScanner(file)
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) != 3 {
			problems = append(problems, fmt.Sprintf("%s:%d: expected path, count, and reason separated by tabs", path, lineNumber))
			continue
		}
		relative := fields[0]
		count, countErr := strconv.Atoi(fields[1])
		if relative == "" || countErr != nil || count < 0 {
			problems = append(problems, fmt.Sprintf("%s:%d: invalid path or count", path, lineNumber))
			continue
		}
		if strings.TrimSpace(fields[2]) == "" {
			problems = append(problems, fmt.Sprintf("%s:%d: %s has an empty reason", path, lineNumber, relative))
			continue
		}
		if previousPath != "" && relative <= previousPath {
			problems = append(problems, fmt.Sprintf("%s:%d: allowance paths are not sorted: %s follows %s", path, lineNumber, relative, previousPath))
		}
		previousPath = relative
		if _, duplicate := allowances[relative]; duplicate {
			problems = append(problems, fmt.Sprintf("%s:%d: duplicate allowance for %s", path, lineNumber, relative))
			continue
		}
		if info, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(relative))); statErr != nil {
			if os.IsNotExist(statErr) {
				problems = append(problems, fmt.Sprintf("%s:%d: listed file does not exist: %s", path, lineNumber, relative))
			} else {
				problems = append(problems, fmt.Sprintf("%s:%d: stat %s: %v", path, lineNumber, relative, statErr))
			}
		} else if !info.Mode().IsRegular() {
			problems = append(problems, fmt.Sprintf("%s:%d: listed path is not a regular file: %s", path, lineNumber, relative))
		}
		allowances[relative] = walltimeAllowance{count: count, reason: fields[2]}
	}
	if err := scanner.Err(); err != nil {
		problems = append(problems, err.Error())
	}
	return allowances, problems
}

func writeWalltimeAllowances(path string, callsByPath map[string][]walltimeCall, existing map[string]walltimeAllowance) error {
	paths := make([]string, 0, len(callsByPath))
	for path, calls := range callsByPath {
		if len(calls) != 0 {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	var contents strings.Builder
	for _, path := range paths {
		reason := existing[path].reason
		if strings.TrimSpace(reason) == "" {
			reason = "UNEXPLAINED"
		}
		fmt.Fprintf(&contents, "%s\t%d\t%s\n", path, len(callsByPath[path]), reason)
	}
	return os.WriteFile(path, []byte(contents.String()), 0o644)
}

func describeWalltimeCalls(path string, calls []walltimeCall) string {
	if len(calls) == 0 {
		return "  (no counted calls)"
	}
	lines := make([]string, 0, len(calls))
	for _, call := range calls {
		name := call.name
		if !call.called {
			name += " (value)"
		}
		lines = append(lines, fmt.Sprintf("  %s:%d: %s", path, call.position.Line, name))
	}
	return strings.Join(lines, "\n")
}
