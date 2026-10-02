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
		"Since":     true,
		"Sleep":     true,
		"Tick":      true,
		"Until":     true,
	},
}

// walltimeReadings are the guarded names that read the wall clock rather than
// wait on it: an elapsed time or a deadline. A reading inside the arguments of
// a log or failure message (walltimeMessageMethods) only reports and is not
// counted; the wait functions count wherever they appear.
var walltimeReadings = map[string]bool{"time.Since": true, "time.Until": true, "time.Now().Add": true, "time.Now().AddDate": true, "time.Now().Sub": true}

// walltimeNowArithmetic are the methods that turn the current wall time into a
// deadline, a backdated or forward-dated fixture time, or an elapsed time.
var walltimeNowArithmetic = map[string]bool{"Add": true, "AddDate": true, "Sub": true}

// walltimeNowViews are the methods that keep a wall reading a wall reading, so
// time.Now().UTC().Add(d) counts like time.Now().Add(d).
var walltimeNowViews = map[string]bool{"UTC": true, "Local": true, "In": true, "Round": true, "Truncate": true}

// walltimeMessageMethods are testing.TB's report methods; an elapsed time in
// their arguments describes a result and decides nothing.
var walltimeMessageMethods = map[string]bool{"Log": true, "Logf": true, "Error": true, "Errorf": true, "Fatal": true, "Fatalf": true, "Skip": true, "Skipf": true}

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
// time.Tick, context.WithTimeout, or context.WithDeadline, and every reading
// of the wall clock as a deadline or an elapsed time: time.Since, time.Until,
// and time.Now() (through UTC, Local, In, Round or Truncate) followed by Add,
// AddDate or Sub. A test that fails because its work took longer than a wall
// budget is red on a loaded or slower host with nothing wrong; it waits on the
// event instead, or runs on an injected clock. The scan covers every _test.go
// file and every test-support file (walltimeSupportFiles). Set
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
	support, err := walltimeSupportFiles(root)
	if err != nil {
		return nil, err
	}
	callsByPath := make(map[string][]walltimeCall)
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != root && skippedDirectory(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(entry.Name(), ".") || strings.HasPrefix(entry.Name(), "_") || !strings.HasSuffix(entry.Name(), ".go") {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if !strings.HasSuffix(entry.Name(), "_test.go") && !support[relative] {
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
		timeNow := func(expression ast.Expr) bool {
			call, ok := ast.Unparen(expression).(*ast.CallExpr)
			if !ok || len(call.Args) != 0 {
				return false
			}
			switch function := ast.Unparen(call.Fun).(type) {
			case *ast.SelectorExpr:
				identifier, ok := function.X.(*ast.Ident)
				return ok && identifier.Obj == nil && aliases[identifier.Name] == "time" && function.Sel.Name == "Now"
			case *ast.Ident:
				return function.Obj == nil && function.Name == "Now" && timeDotImported(file)
			}
			return false
		}
		// wallReading says whether expression is time.Now() seen through any
		// number of walltimeNowViews.
		var wallReading func(ast.Expr) bool
		wallReading = func(expression ast.Expr) bool {
			if timeNow(expression) {
				return true
			}
			call, ok := ast.Unparen(expression).(*ast.CallExpr)
			if !ok {
				return false
			}
			selector, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
			return ok && walltimeNowViews[selector.Sel.Name] && wallReading(selector.X)
		}
		callees := make(map[token.Pos]bool)
		selectorNames := make(map[token.Pos]bool)
		var messages [][2]token.Pos
		ast.Inspect(file, func(node ast.Node) bool {
			switch expression := node.(type) {
			case *ast.CallExpr:
				if position := walltimeReferencePosition(expression.Fun); position.IsValid() {
					callees[position] = true
				}
				if selector, ok := ast.Unparen(expression.Fun).(*ast.SelectorExpr); ok && walltimeMessageMethods[selector.Sel.Name] {
					messages = append(messages, [2]token.Pos{expression.Lparen, expression.Rparen})
				}
			case *ast.SelectorExpr:
				selectorNames[expression.Sel.Pos()] = true
			}
			return true
		})
		inMessage := func(position token.Pos) bool {
			for _, span := range messages {
				if span[0] < position && position < span[1] {
					return true
				}
			}
			return false
		}
		record := func(name string, position token.Pos, called bool) {
			if walltimeReadings[name] && inMessage(position) {
				return
			}
			callsByPath[relative] = append(callsByPath[relative], walltimeCall{
				name:     name,
				position: fileSet.Position(position),
				called:   called,
			})
		}
		ast.Inspect(file, func(node ast.Node) bool {
			switch reference := node.(type) {
			case *ast.SelectorExpr:
				if walltimeNowArithmetic[reference.Sel.Name] && wallReading(reference.X) {
					record("time.Now()."+reference.Sel.Name, reference.Sel.Pos(), callees[reference.Sel.Pos()])
					return true
				}
				identifier, ok := reference.X.(*ast.Ident)
				if !ok || identifier.Obj != nil {
					return true
				}
				importPath, ok := aliases[identifier.Name]
				if !ok || !guardedWalltimeCalls[importPath][reference.Sel.Name] {
					return true
				}
				record(importPath+"."+reference.Sel.Name, reference.Sel.Pos(), callees[reference.Sel.Pos()])
				return true
			case *ast.Ident:
				if reference.Obj != nil || selectorNames[reference.Pos()] {
					return true
				}
				importPath, ok := dotNames[reference.Name]
				if !ok || !guardedWalltimeCalls[importPath][reference.Name] {
					return true
				}
				record(importPath+"."+reference.Name, reference.Pos(), callees[reference.Pos()])
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

func timeDotImported(file *ast.File) bool {
	return importedAliases(file, "time", "time")["."]
}

// walltimeSupportFiles names, relative to root, the Go files without the
// _test suffix that exist for tests: every file of a package that no main
// package of the module reaches through the imports of its non-test files,
// and every file whose name starts with "fake" or "fixture" (a fake or a
// fixture compiled into a package production also builds, for tests to
// drive). A wait in them is a test's wait reached through a helper.
func walltimeSupportFiles(root string) (map[string]bool, error) {
	module := ""
	if contents, err := os.ReadFile(filepath.Join(root, "go.mod")); err == nil {
		for _, line := range strings.Split(string(contents), "\n") {
			if fields := strings.Fields(line); len(fields) == 2 && fields[0] == "module" {
				module = fields[1]
				break
			}
		}
	}
	type goPackage struct {
		main    bool
		imports map[string]bool
		files   []string
	}
	packages := map[string]*goPackage{}
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
		name := entry.Name()
		if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		directory := filepath.ToSlash(filepath.Dir(relative))
		current := packages[directory]
		if current == nil {
			current = &goPackage{imports: map[string]bool{}}
			packages[directory] = current
		}
		current.main = current.main || file.Name.Name == "main"
		current.files = append(current.files, relative)
		for _, spec := range file.Imports {
			importPath, err := strconv.Unquote(spec.Path.Value)
			if err != nil || module == "" {
				continue
			}
			if importPath == module {
				current.imports["."] = true
			} else if strings.HasPrefix(importPath, module+"/") {
				current.imports[strings.TrimPrefix(importPath, module+"/")] = true
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	reached := map[string]bool{}
	var queue []string
	for directory, candidate := range packages {
		if candidate.main {
			reached[directory] = true
			queue = append(queue, directory)
		}
	}
	for len(queue) > 0 {
		directory := queue[0]
		queue = queue[1:]
		for imported := range packages[directory].imports {
			if _, known := packages[imported]; known && !reached[imported] {
				reached[imported] = true
				queue = append(queue, imported)
			}
		}
	}
	support := map[string]bool{}
	for directory, candidate := range packages {
		for _, relative := range candidate.files {
			base := filepath.Base(relative)
			if !reached[directory] || strings.HasPrefix(base, "fake") || strings.HasPrefix(base, "fixture") {
				support[relative] = true
			}
		}
	}
	return support, nil
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

func TestWalltimeCounterIncludesWallReadings(t *testing.T) {
	t.Parallel()

	const source = `package fixture
import (
	"testing"
	"time"
)
func fixture(t *testing.T, started time.Time) {
	_ = time.Since(started)
	_ = time.Until(started)
	_ = time.Now().Add(time.Second)
	_ = time.Now().UTC().Truncate(time.Second).Add(-time.Hour)
	_ = time.Now().Sub(started)
	_ = time.Now().AddDate(0, 0, 1)
	_ = time.Since
	t.Logf("took %s", time.Since(started))
	t.Fatalf("deadline %s", time.Now().Add(time.Second))
	_ = time.Now()
	_ = started.Add(time.Second)
	t.Logf("slept %v", func() bool { time.Sleep(0); return true }())
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
		"time.Since:7",
		"time.Until:8",
		"time.Now().Add:9",
		"time.Now().Add:10",
		"time.Now().Sub:11",
		"time.Now().AddDate:12",
		"time.Since (value):13",
		"time.Sleep:18",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("wall-time references=%q, want %q", got, want)
	}
}

func TestWalltimeScanCoversTestSupportFiles(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := map[string]string{
		"go.mod":                           "module example.test/fixture\n\ngo 1.22\n",
		"cmd/tool/main.go":                 "package main\nimport _ \"example.test/fixture/internal/product\"\nfunc main() {}\n",
		"internal/product/product.go":      "package product\nimport \"time\"\nfunc Pause() { time.Sleep(time.Second) }\n",
		"internal/product/fake_runtime.go": "package product\nimport \"time\"\nfunc fakePause() { time.Sleep(time.Second) }\n",
		"internal/product/fixture.go":      "package product\nimport \"time\"\nfunc fixtureAge(at time.Time) time.Duration { return time.Since(at) }\n",
		"internal/helper/helper.go":        "package helper\nimport \"time\"\nfunc Deadline() time.Time { return time.Now().Add(time.Second) }\n",
	}
	for relative, contents := range files {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	callsByPath, err := findTestWalltimeCalls(root)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for path, calls := range callsByPath {
		for _, call := range calls {
			got = append(got, path+" "+call.name)
		}
	}
	sort.Strings(got)
	want := []string{
		"internal/helper/helper.go time.Now().Add",
		"internal/product/fake_runtime.go time.Sleep",
		"internal/product/fixture.go time.Since",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("scanned wall-time references=%q, want %q (production product.go is out of scope)", got, want)
	}
}
