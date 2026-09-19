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
}

// TestNoTestWaitsOnWallTime forbids tests from using time.Sleep, time.After,
// time.AfterFunc, time.NewTimer, time.NewTicker, time.Tick,
// context.WithTimeout, or context.WithDeadline to wait for a condition. Set
// WALLTIME_ALLOWANCE_UPDATE=1 to rewrite the allowance after removing a call;
// update mode always fails so the rewritten file must be reviewed.
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
		for importPath := range guardedWalltimeCalls {
			for alias := range importedAliases(file, importPath, importPath) {
				aliases[alias] = importPath
			}
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			identifier, ok := selector.X.(*ast.Ident)
			if !ok {
				return true
			}
			importPath, ok := aliases[identifier.Name]
			if !ok || !guardedWalltimeCalls[importPath][selector.Sel.Name] {
				return true
			}
			callsByPath[relative] = append(callsByPath[relative], walltimeCall{
				name:     importPath + "." + selector.Sel.Name,
				position: fileSet.Position(selector.Sel.Pos()),
			})
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
		lines = append(lines, fmt.Sprintf("  %s:%d: %s", path, call.position.Line, call.name))
	}
	return strings.Join(lines, "\n")
}
