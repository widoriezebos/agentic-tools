package testenv

import (
	"context"
	"encoding/json"
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
	"syscall"
	"testing"
	"time"
)

const testenvImport = "github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"

// Every test package is protected. Keep this list explicit and empty unless a
// package cannot use the shared boundary and has no subprocess path.
var testMainExemptions = map[string]string{}

var goOSFilenameSuffixes = map[string]bool{
	"aix": true, "android": true, "darwin": true, "dragonfly": true, "freebsd": true,
	"illumos": true, "ios": true, "js": true, "linux": true, "netbsd": true,
	"openbsd": true, "plan9": true, "solaris": true, "wasip1": true, "windows": true,
}

var goArchFilenameSuffixes = map[string]bool{
	"386": true, "amd64": true, "arm": true, "arm64": true, "loong64": true,
	"mips": true, "mips64": true, "mips64le": true, "mipsle": true, "ppc64": true,
	"ppc64le": true, "riscv64": true, "s390x": true, "wasm": true,
}

type packageTests struct {
	files []testFile
}

type testFile struct {
	name   string
	syntax *ast.File
}

func TestEveryPackageUsesSharedMain(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	t.Run("shared TestMain", func(t *testing.T) {
		problems, err := auditTestPackages(root)
		if err != nil {
			t.Fatal(err)
		}
		for _, problem := range problems {
			t.Error(problem)
		}
	})
	t.Run("fixture engine children use shared reaper", func(t *testing.T) {
		problems, err := auditFixtureEngineChildren(root)
		if err != nil {
			t.Fatal(err)
		}
		for _, problem := range problems {
			t.Error(problem)
		}

		synthetic := t.TempDir()
		writeAuditFixture(t, synthetic, "internal/fixture/unguarded_test.go", `package fixture
import "os/exec"
func child() { _ = exec.Command("/tmp/engine-pin", "steward", "run") }
`)
		problems, err = auditFixtureEngineChildren(synthetic)
		if err != nil || len(problems) != 1 || !strings.Contains(problems[0], "unguarded_test.go") {
			t.Fatalf("unguarded fixture engine child problems=%v err=%v", problems, err)
		}
		writeAuditFixture(t, synthetic, "internal/fixture/unguarded_test.go", `package fixture
import (
    "os/exec"
    "github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)
func child() {
    testenv.ReapFixtureProcessGroups(nil, nil)
    _ = exec.Command("/tmp/engine-pin", "steward", "run")
}
`)
		if problems, err = auditFixtureEngineChildren(synthetic); err != nil || len(problems) != 0 {
			t.Fatalf("shared-reaper fixture engine child problems=%v err=%v", problems, err)
		}
	})
	t.Run("fixture process groups are killed and awaited", func(t *testing.T) {
		recorder := &fixtureProcessGroupRecorder{}
		killed, waited := false, false
		reapFixtureProcessGroups(recorder, []FixtureProcessGroup{{
			Verb: "steward run", Resolve: func() (int, bool, error) { return 42, true, nil },
		}}, nil, fixtureProcessGroupOps{
			groupID: func(pid int) (int, error) { return pid, nil },
			signal: func(pid int, signal syscall.Signal) error {
				if pid == -42 && signal == syscall.SIGKILL {
					killed = true
				}
				return nil
			},
			wait: func(ctx context.Context, target int) error {
				waited = true
				if _, present := ctx.Deadline(); !present {
					return fmt.Errorf("exit wait has no deadline")
				}
				if target != -42 || !killed {
					return context.DeadlineExceeded
				}
				return nil
			},
			cleanupContext: testFixtureContext,
			exitContext:    testFixtureContext,
		})
		recorder.runCleanups()
		if !killed || !waited || len(recorder.errors) != 0 {
			t.Fatalf("group killed=%t waited=%t cleanup errors=%v", killed, waited, recorder.errors)
		}

		survivor := &fixtureProcessGroupRecorder{}
		reapFixtureProcessGroups(survivor, []FixtureProcessGroup{{
			Verb: "steward run", Resolve: func() (int, bool, error) { return 43, true, nil },
		}}, nil, fixtureProcessGroupOps{
			groupID:        func(pid int) (int, error) { return pid, nil },
			signal:         func(int, syscall.Signal) error { return nil },
			wait:           func(context.Context, int) error { return context.DeadlineExceeded },
			cleanupContext: testFixtureContext,
			exitContext:    testFixtureContext,
		})
		survivor.runCleanups()
		if got := strings.Join(survivor.errors, "\n"); !strings.Contains(got, "fixture child outlived test") || !strings.Contains(got, "pid=43") || !strings.Contains(got, `verb="steward run"`) {
			t.Fatalf("survivor failure = %q", got)
		}

		wrongGroup := &fixtureProcessGroupRecorder{}
		reapFixtureProcessGroups(wrongGroup, []FixtureProcessGroup{{
			Verb: "supervise owner", Resolve: func() (int, bool, error) { return 42, true, nil },
		}}, nil, fixtureProcessGroupOps{
			groupID:        func(int) (int, error) { return 7, nil },
			signal:         func(int, syscall.Signal) error { return syscall.ESRCH },
			wait:           func(context.Context, int) error { return nil },
			cleanupContext: testFixtureContext,
			exitContext:    testFixtureContext,
		})
		wrongGroup.runCleanups()
		if got := strings.Join(wrongGroup.errors, "\n"); !strings.Contains(got, "not started in its own process group") || !strings.Contains(got, "pid=42") || !strings.Contains(got, `verb="supervise owner"`) {
			t.Fatalf("wrong-group failure = %q", got)
		}

		freshContexts := &fixtureProcessGroupRecorder{}
		var created int
		var cancelFirst context.CancelFunc
		secondRan := false
		reapFixtureProcessGroups(freshContexts, nil, []FixtureCleanup{
			{Verb: "first stop", Run: func(context.Context) error { cancelFirst(); return nil }},
			{Verb: "second stop", Run: func(ctx context.Context) error {
				secondRan = ctx.Err() == nil
				return ctx.Err()
			}},
		}, fixtureProcessGroupOps{
			groupID: func(pid int) (int, error) { return pid, nil },
			signal:  func(int, syscall.Signal) error { return nil },
			wait:    func(context.Context, int) error { return nil },
			cleanupContext: func() (context.Context, context.CancelFunc) {
				created++
				ctx, cancel := context.WithCancel(context.Background())
				if created == 1 {
					cancelFirst = cancel
				}
				return ctx, cancel
			},
			exitContext: testFixtureContext,
		})
		freshContexts.runCleanups()
		if !secondRan || created != 2 || len(freshContexts.errors) != 1 || !strings.Contains(freshContexts.errors[0], `verb="first stop"`) {
			t.Fatalf("fresh cleanup contexts: second-ran=%t created=%d errors=%v", secondRan, created, freshContexts.errors)
		}
	})
	t.Run("supervision operator trap reaps its steward group", func(t *testing.T) {
		data, err := os.ReadFile(filepath.Join(root, "scripts", "agents", "supervision-fixtures.sh"))
		if err != nil {
			t.Fatal(err)
		}
		source := string(data)
		start := strings.Index(source, "reap_operator_steward_group()")
		end := strings.Index(source, "\ncleanup_started=0")
		if start < 0 || end <= start {
			t.Fatal("supervision fixture has no operator steward process-group reaper before its EXIT trap")
		}
		reaper := source[start:end]
		for _, required := range []string{`kill -KILL -- "-$pid"`, `wait_for_process_group_exit`, `operator steward runner pid=$pid`} {
			if !strings.Contains(reaper, required) {
				t.Errorf("operator steward reaper lacks %q", required)
			}
		}
		cleanupStart := strings.Index(source, "cleanup() {")
		trap := strings.Index(source, "trap cleanup EXIT")
		if cleanupStart < 0 || trap <= cleanupStart || !strings.Contains(source[cleanupStart:trap], "reap_operator_steward_group") {
			t.Error("the supervision fixture EXIT cleanup does not invoke the operator steward process-group reaper")
		}
	})
}

type fixtureProcessGroupRecorder struct {
	cleanups []func()
	errors   []string
}

func (*fixtureProcessGroupRecorder) Helper() {}

func (r *fixtureProcessGroupRecorder) Cleanup(cleanup func()) {
	r.cleanups = append(r.cleanups, cleanup)
}

func (r *fixtureProcessGroupRecorder) Errorf(format string, arguments ...any) {
	r.errors = append(r.errors, fmt.Sprintf(format, arguments...))
}

func (r *fixtureProcessGroupRecorder) runCleanups() {
	for index := len(r.cleanups) - 1; index >= 0; index-- {
		r.cleanups[index]()
	}
}

func testFixtureContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), time.Second)
}

func auditFixtureEngineChildren(root string) ([]string, error) {
	var problems []string
	for _, top := range []string{"cmd", "internal"} {
		scanRoot := filepath.Join(root, top)
		if _, err := os.Stat(scanRoot); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return nil, err
		}
		err := filepath.WalkDir(scanRoot, func(path string, entry fs.DirEntry, walkErr error) error {
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
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			compact := strings.Join(strings.Fields(string(data)), "")
			startsEngineChild := strings.Contains(compact, "exec.Command") &&
				(strings.Contains(compact, "engine-pin") || strings.Contains(compact, `"steward","run"`))
			if !startsEngineChild || strings.Contains(compact, "ReapFixtureProcessGroups(") {
				return nil
			}
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			problems = append(problems, filepath.ToSlash(relative)+" starts an engine-pin or steward run child without testenv.ReapFixtureProcessGroups")
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(problems)
	return problems, nil
}

func TestTestEnvironmentStandardInventoryMatchesPackageTests(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	all, declared, err := declaredTestEnvironmentStandardTests(filepath.Join(root, "testing.json"))
	if err != nil {
		t.Fatal(err)
	}
	packages, _, err := loadPackageTests(root)
	if err != nil {
		t.Fatal(err)
	}
	packageTests := packages["internal/testenv"]
	if packageTests == nil {
		t.Fatal("internal/testenv has no test files")
	}

	const optIn = "TestPackageWalkExternalCheckout"
	observed := packageTests.testFunctionNames()
	if all {
		declared = observed
	}
	observedSet := make(map[string]bool, len(observed))
	for _, name := range observed {
		observedSet[name] = true
	}
	declaredSet := make(map[string]bool, len(declared))
	for _, name := range declared {
		declaredSet[name] = true
	}

	var absentFromGroup, absentFromPackage []string
	for _, name := range observed {
		if name != optIn && !declaredSet[name] {
			absentFromGroup = append(absentFromGroup, name)
		}
	}
	for _, name := range declared {
		if !observedSet[name] {
			absentFromPackage = append(absentFromPackage, name)
		}
	}
	if !observedSet[optIn] {
		t.Errorf("opt-in test %s is absent from the package", optIn)
	}
	if !all && declaredSet[optIn] {
		t.Errorf("opt-in test %s is listed in test-environment-standard", optIn)
	}
	if len(absentFromGroup) != 0 || len(absentFromPackage) != 0 {
		t.Errorf("test-environment-standard drift: package tests absent from group=%v; group tests absent from package=%v", absentFromGroup, absentFromPackage)
	}
}

func declaredTestEnvironmentStandardTests(path string) (bool, []string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, nil, err
	}
	return decodeTestEnvironmentStandardTests(data, path)
}

func decodeTestEnvironmentStandardTests(data []byte, source string) (bool, []string, error) {
	var contract struct {
		Groups []struct {
			ID    string          `json:"id"`
			Tests json.RawMessage `json:"tests"`
		} `json:"groups"`
	}
	if err := json.Unmarshal(data, &contract); err != nil {
		return false, nil, fmt.Errorf("decode %s: %w", source, err)
	}
	for _, group := range contract.Groups {
		if group.ID != "test-environment-standard" {
			continue
		}
		return decodeTestEnvironmentStandardSelector(group.Tests)
	}
	return false, nil, fmt.Errorf("test-environment-standard is absent from %s", source)
}

func decodeTestEnvironmentStandardSelector(raw json.RawMessage) (bool, []string, error) {
	selector := strings.TrimSpace(string(raw))
	if selector == "" {
		return false, nil, fmt.Errorf("test-environment-standard tests selector is missing")
	}
	if selector == "null" {
		return false, nil, fmt.Errorf("test-environment-standard tests selector is malformed")
	}
	if selector[0] == '"' {
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return false, nil, fmt.Errorf("test-environment-standard tests selector is malformed")
		}
		if value != "all" {
			return false, nil, fmt.Errorf("test-environment-standard tests selector has invalid value %q", value)
		}
		return true, nil, nil
	}
	if selector[0] != '[' {
		return false, nil, fmt.Errorf("test-environment-standard tests selector has invalid type")
	}
	var entries []json.RawMessage
	if err := json.Unmarshal(raw, &entries); err != nil {
		return false, nil, fmt.Errorf("test-environment-standard tests selector is malformed")
	}
	if len(entries) == 0 {
		return false, nil, fmt.Errorf("test-environment-standard tests selector has invalid value: named list is empty")
	}
	names := make([]string, 0, len(entries))
	seen := make(map[string]bool, len(entries))
	for _, entry := range entries {
		var name string
		if err := json.Unmarshal(entry, &name); err != nil || name == "" || seen[name] {
			return false, nil, fmt.Errorf("test-environment-standard test names must be nonempty strings without duplicates")
		}
		seen[name] = true
		names = append(names, name)
	}
	sort.Strings(names)
	return false, names, nil
}

func TestDeclaredTestEnvironmentStandardTests(t *testing.T) {
	t.Parallel()

	valid := []struct {
		name      string
		fixture   string
		wantAll   bool
		wantNames []string
	}{
		{name: "named", fixture: `{"groups":[{"id":"test-environment-standard","tests":["TestZulu","TestAlpha"]}]}`, wantNames: []string{"TestAlpha", "TestZulu"}},
		{name: "all", fixture: `{"groups":[{"id":"test-environment-standard","tests":"all"}]}`, wantAll: true},
	}
	for _, test := range valid {
		t.Run(test.name, func(t *testing.T) {
			all, names, err := decodeTestEnvironmentStandardTests([]byte(test.fixture), "fixture")
			if err != nil || all != test.wantAll || !slices.Equal(names, test.wantNames) {
				t.Fatalf("all=%t names=%v err=%v; want all=%t names=%v", all, names, err, test.wantAll, test.wantNames)
			}
		})
	}

	errors := []struct {
		name    string
		fixture string
		want    string
	}{
		{name: "missing group", fixture: `{"groups":[]}`, want: "test-environment-standard is absent from fixture"},
		{name: "missing selector", fixture: `{"groups":[{"id":"test-environment-standard"}]}`, want: "test-environment-standard tests selector is missing"},
		{name: "malformed selector", fixture: `{"groups":[{"id":"test-environment-standard","tests":null}]}`, want: "test-environment-standard tests selector is malformed"},
		{name: "invalid selector type", fixture: `{"groups":[{"id":"test-environment-standard","tests":{}}]}`, want: "test-environment-standard tests selector has invalid type"},
		{name: "invalid selector value", fixture: `{"groups":[{"id":"test-environment-standard","tests":"named"}]}`, want: `test-environment-standard tests selector has invalid value "named"`},
		{name: "empty named list", fixture: `{"groups":[{"id":"test-environment-standard","tests":[]}]}`, want: "test-environment-standard tests selector has invalid value: named list is empty"},
		{name: "non-string name", fixture: `{"groups":[{"id":"test-environment-standard","tests":[17]}]}`, want: "test-environment-standard test names must be nonempty strings without duplicates"},
		{name: "empty name", fixture: `{"groups":[{"id":"test-environment-standard","tests":[""]}]}`, want: "test-environment-standard test names must be nonempty strings without duplicates"},
		{name: "duplicate name", fixture: `{"groups":[{"id":"test-environment-standard","tests":["TestOne","TestOne"]}]}`, want: "test-environment-standard test names must be nonempty strings without duplicates"},
	}
	for _, test := range errors {
		t.Run(test.name, func(t *testing.T) {
			_, _, err := decodeTestEnvironmentStandardTests([]byte(test.fixture), "fixture")
			if err == nil || err.Error() != test.want {
				t.Fatalf("err=%v, want %q", err, test.want)
			}
		})
	}

	if _, _, err := decodeTestEnvironmentStandardSelector(json.RawMessage(`[`)); err == nil || err.Error() != "test-environment-standard tests selector is malformed" {
		t.Fatalf("malformed selector err=%v", err)
	}
}

func (group *packageTests) testFunctionNames() []string {
	seen := make(map[string]bool)
	for _, file := range group.files {
		for _, declaration := range file.syntax.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Recv != nil || function.Name.Name == "TestMain" || !strings.HasPrefix(function.Name.Name, "Test") {
				continue
			}
			seen[function.Name.Name] = true
		}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func auditTestPackages(root string) ([]string, error) {
	packages, _, err := loadPackageTests(root)
	if err != nil {
		return nil, err
	}
	problems := make([]string, 0)
	for directory, group := range packages {
		if reason, exempt := testMainExemptions[directory]; exempt {
			if strings.TrimSpace(reason) == "" {
				problems = append(problems, fmt.Sprintf("%s has an exemption without a reason", directory))
			}
			continue
		}
		for _, problem := range group.sharedMainProblems(directory == "internal/testenv") {
			problems = append(problems, fmt.Sprintf("%s %s", directory, problem))
		}
	}
	for directory, reason := range testMainExemptions {
		if _, exists := packages[directory]; !exists {
			problems = append(problems, fmt.Sprintf("%s has a stale TestMain exemption (%s)", directory, reason))
		}
	}
	sort.Strings(problems)
	return problems, nil
}

func loadPackageTests(root string) (map[string]*packageTests, []string, error) {
	packages := map[string]*packageTests{}
	var walkedDirectories []string
	for _, top := range []string{"cmd", "internal"} {
		scanRoot := filepath.Join(root, top)
		if _, err := os.Stat(scanRoot); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return nil, nil, err
		}
		err := filepath.WalkDir(scanRoot, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				if path != root && skippedDirectory(entry.Name()) {
					return filepath.SkipDir
				}
				if path != root {
					if _, err := os.Stat(filepath.Join(path, "go.mod")); err == nil {
						return filepath.SkipDir
					} else if !os.IsNotExist(err) {
						return err
					}
				}
				relative, err := filepath.Rel(root, path)
				if err != nil {
					return err
				}
				walkedDirectories = append(walkedDirectories, filepath.ToSlash(relative))
				return nil
			}
			if strings.HasPrefix(entry.Name(), ".") || strings.HasPrefix(entry.Name(), "_") || !strings.HasSuffix(entry.Name(), "_test.go") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			file, err := parser.ParseFile(token.NewFileSet(), path, data, parser.ParseComments)
			if err != nil {
				return err
			}
			directory, err := filepath.Rel(root, filepath.Dir(path))
			if err != nil {
				return err
			}
			directory = filepath.ToSlash(directory)
			group := packages[directory]
			if group == nil {
				group = &packageTests{}
				packages[directory] = group
			}
			group.files = append(group.files, testFile{name: entry.Name(), syntax: file})
			return nil
		})
		if err != nil {
			return nil, nil, err
		}
	}
	return packages, walkedDirectories, nil
}

func skippedDirectory(name string) bool {
	return name == "artifacts" || name == "testdata" || name == "vendor" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")
}

func (group *packageTests) sharedMainProblems(localTestenv bool) []string {
	var problems []string
	found := false
	for _, file := range group.files {
		testenvAliases := importedAliases(file.syntax, testenvImport, "testenv")
		osAliases := importedAliases(file.syntax, "os", "os")
		for _, declaration := range file.syntax.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Name.Name != "TestMain" {
				continue
			}
			found = true
			if constraint := testMainConstraint(file); constraint != "" {
				problems = append(problems, fmt.Sprintf("has a TestMain in build-constrained file %s (%s)", file.name, constraint))
			}
			if function.Body == nil {
				problems = append(problems, fmt.Sprintf("has a TestMain without a body in %s", file.name))
				continue
			}
			if problem := validateTestMain(function, testenvAliases, osAliases, localTestenv); problem != "" {
				problems = append(problems, fmt.Sprintf("%s in %s", problem, file.name))
			}
		}
	}
	if !found {
		problems = append(problems, "has _test.go files but no TestMain that uses testenv.Main")
	}
	return problems
}

func (group *packageTests) sharedMainProblem(localTestenv bool) string {
	return strings.Join(group.sharedMainProblems(localTestenv), "; ")
}

func testMainConstraint(file testFile) string {
	for _, group := range file.syntax.Comments {
		for _, comment := range group.List {
			fields := strings.Fields(comment.Text)
			if len(fields) > 0 && fields[0] == "//go:build" {
				return "//go:build line"
			}
		}
	}
	base := strings.TrimSuffix(file.name, "_test.go")
	parts := strings.Split(base, "_")
	if len(parts) < 2 {
		return ""
	}
	suffix := parts[len(parts)-1]
	if goOSFilenameSuffixes[suffix] {
		return "GOOS filename suffix"
	}
	if goArchFilenameSuffixes[suffix] {
		return "GOARCH filename suffix"
	}
	return ""
}

func importedAliases(file *ast.File, importPath, defaultName string) map[string]bool {
	aliases := map[string]bool{}
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil || path != importPath {
			continue
		}
		name := defaultName
		if spec.Name != nil {
			name = spec.Name.Name
		}
		aliases[name] = true
	}
	return aliases
}

func validateTestMain(function *ast.FuncDecl, testenvAliases, osAliases map[string]bool, localTestenv bool) string {
	parameter := testMainParameter(function)
	if parameter == "" {
		return "has a TestMain whose testing.M parameter cannot be identified"
	}
	if containsReturn(function.Body) {
		return "has a TestMain return path that bypasses testenv.Main"
	}
	statements := function.Body.List
	if len(statements) == 0 {
		return "has an empty TestMain"
	}
	exitArgument, ok := osExitArgument(statements[len(statements)-1], osAliases)
	if !ok {
		return "has a TestMain whose parent path does not end with os.Exit(testenv.Main(m))"
	}
	if sharedMainCall(exitArgument, parameter, testenvAliases, localTestenv) {
		return ""
	}
	code, ok := exitArgument.(*ast.Ident)
	if !ok {
		return "has a TestMain whose parent path does not exit with testenv.Main's code"
	}
	mainAssignment := -1
	for index, statement := range statements[:len(statements)-1] {
		assignment, ok := statement.(*ast.AssignStmt)
		if !ok {
			continue
		}
		for assignedIndex, left := range assignment.Lhs {
			identifier, ok := left.(*ast.Ident)
			if !ok || identifier.Name != code.Name {
				continue
			}
			if mainAssignment >= 0 || assignedIndex >= len(assignment.Rhs) || !sharedMainCall(assignment.Rhs[assignedIndex], parameter, testenvAliases, localTestenv) {
				return "has a TestMain that changes the testenv.Main exit code before os.Exit"
			}
			mainAssignment = index
		}
	}
	if mainAssignment < 0 {
		return "has a TestMain whose parent path does not exit with testenv.Main's code"
	}
	for _, statement := range statements[mainAssignment+1 : len(statements)-1] {
		if assignsIdentifier(statement, code.Name) {
			return "has a TestMain that changes the testenv.Main exit code before os.Exit"
		}
	}
	return ""
}

func testMainParameter(function *ast.FuncDecl) string {
	if function.Type.Params == nil || len(function.Type.Params.List) != 1 || len(function.Type.Params.List[0].Names) != 1 {
		return ""
	}
	return function.Type.Params.List[0].Names[0].Name
}

func containsReturn(body *ast.BlockStmt) bool {
	found := false
	ast.Inspect(body, func(node ast.Node) bool {
		if _, nested := node.(*ast.FuncLit); nested {
			return false
		}
		if _, ok := node.(*ast.ReturnStmt); ok {
			found = true
		}
		return !found
	})
	return found
}

func assignsIdentifier(node ast.Node, name string) bool {
	found := false
	ast.Inspect(node, func(node ast.Node) bool {
		switch expression := node.(type) {
		case *ast.AssignStmt:
			for _, left := range expression.Lhs {
				if identifier, ok := left.(*ast.Ident); ok && identifier.Name == name {
					found = true
					return false
				}
			}
		case *ast.IncDecStmt:
			if identifier, ok := expression.X.(*ast.Ident); ok && identifier.Name == name {
				found = true
				return false
			}
		}
		return !found
	})
	return found
}

func osExitArgument(statement ast.Stmt, osAliases map[string]bool) (ast.Expr, bool) {
	expression, ok := statement.(*ast.ExprStmt)
	if !ok {
		return nil, false
	}
	call, ok := expression.X.(*ast.CallExpr)
	if !ok || len(call.Args) != 1 || !qualifiedCall(call.Fun, osAliases, "Exit") {
		return nil, false
	}
	return call.Args[0], true
}

func sharedMainCall(expression ast.Expr, parameter string, testenvAliases map[string]bool, localTestenv bool) bool {
	call, ok := expression.(*ast.CallExpr)
	if !ok || len(call.Args) == 0 {
		return false
	}
	argument, ok := call.Args[0].(*ast.Ident)
	if !ok || argument.Name != parameter {
		return false
	}
	if localTestenv {
		identifier, ok := call.Fun.(*ast.Ident)
		return ok && identifier.Name == "Main"
	}
	return qualifiedCall(call.Fun, testenvAliases, "Main")
}

func qualifiedCall(expression ast.Expr, aliases map[string]bool, function string) bool {
	selector, ok := expression.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != function {
		return false
	}
	identifier, ok := selector.X.(*ast.Ident)
	return ok && aliases[identifier.Name]
}

func TestBoundaryAuditRequiresParentPathEvenWhenAHelperUsesSharedMain(t *testing.T) {
	group := parsePackageSource(t, `package fixture
import (
    "os"
    "testing"
    "github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)
func TestMain(m *testing.M) {
    if os.Getenv("GO_WANT_HELPER") == "1" {
        os.Exit(testenv.Main(m))
    }
    os.Exit(m.Run())
}
`)
	if problem := group.sharedMainProblem(false); problem == "" {
		t.Fatal("a helper-only testenv.Main call protected the parent path")
	}
}

func TestBoundaryAuditAcceptsHelperBypassWithProtectedParent(t *testing.T) {
	group := parsePackageSource(t, `package fixture
import (
    "os"
    "testing"
    "github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)
func TestMain(m *testing.M) {
    if os.Getenv("GO_WANT_HELPER") == "1" {
        os.Exit(m.Run())
    }
    os.Exit(testenv.Main(m))
}
`)
	if problem := group.sharedMainProblem(false); problem != "" {
		t.Fatal(problem)
	}
}

func TestBoundaryAuditAcceptsCleanupBeforeExitingWithSharedCode(t *testing.T) {
	group := parsePackageSource(t, `package fixture
import (
    "os"
    "testing"
    "github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)
func TestMain(m *testing.M) {
    code := testenv.Main(m)
    cleanup()
    os.Exit(code)
}
`)
	if problem := group.sharedMainProblem(false); problem != "" {
		t.Fatal(problem)
	}
}

func TestBoundaryAuditRejectsChangedSharedExitCode(t *testing.T) {
	group := parsePackageSource(t, `package fixture
import (
    "os"
    "testing"
    "github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)
func TestMain(m *testing.M) {
    code := testenv.Main(m)
    if code == 0 {
        code = 1
	}
	os.Exit(code)
}
`)
	if problem := group.sharedMainProblem(false); !strings.Contains(problem, "changes the testenv.Main exit code") {
		t.Fatalf("changed exit code problem = %q", problem)
	}
}

func TestBoundaryAuditValidatesEveryTestMain(t *testing.T) {
	valid := parseTestFile(t, "first_test.go", `package fixture
import (
    "os"
    "testing"
    "github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)
func TestMain(m *testing.M) { os.Exit(testenv.Main(m)) }
`)
	invalid := parseTestFile(t, "second_test.go", `package fixture
import (
    "os"
    "testing"
)
func TestMain(m *testing.M) { os.Exit(m.Run()) }
`)
	group := &packageTests{files: []testFile{valid, invalid}}
	problems := group.sharedMainProblems(false)
	if len(problems) != 1 || !strings.Contains(problems[0], "second_test.go") || !strings.Contains(problems[0], "testenv.Main") {
		t.Fatalf("all TestMain declarations were not validated: %v", problems)
	}
}

func TestBoundaryAuditRejectsBuildConstrainedTestMain(t *testing.T) {
	for _, fixture := range []struct {
		label  string
		name   string
		source string
		want   string
	}{
		{label: "build_line", name: "testmain_test.go", source: "//go:build darwin\n\n" + sharedMainFixtureSource, want: "//go:build line"},
		{label: "goos_filename_suffix", name: "testmain_linux_test.go", source: sharedMainFixtureSource, want: "GOOS filename suffix"},
		{label: "goarch_filename_suffix", name: "testmain_arm64_test.go", source: sharedMainFixtureSource, want: "GOARCH filename suffix"},
	} {
		t.Run(fixture.label, func(t *testing.T) {
			group := &packageTests{files: []testFile{parseTestFile(t, fixture.name, fixture.source)}}
			problem := group.sharedMainProblem(false)
			if !strings.Contains(problem, "build-constrained") || !strings.Contains(problem, fixture.want) {
				t.Fatalf("build-constrained TestMain problem = %q", problem)
			}
		})
	}
}

const sharedMainFixtureSource = `package fixture
import (
    "os"
    "testing"
    "github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)
func TestMain(m *testing.M) { os.Exit(testenv.Main(m)) }
`

func TestBoundaryAuditRefusesThenAcceptsSyntheticPackage(t *testing.T) {
	root := t.TempDir()
	writeAuditFixture(t, root, "internal/fixture/fixture_test.go", "package fixture\n")
	problems, err := auditTestPackages(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 1 || !strings.Contains(problems[0], "internal/fixture has _test.go files but no TestMain") {
		t.Fatalf("unprotected package was not refused: %v", problems)
	}
	t.Logf("unprotected audit refused: %s", problems[0])

	writeAuditFixture(t, root, "internal/fixture/testmain_test.go", `package fixture
import (
    "os"
    "testing"
    "github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)
func TestMain(m *testing.M) { os.Exit(testenv.Main(m)) }
`)
	problems, err = auditTestPackages(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 0 {
		t.Fatalf("protected package was refused: %v", problems)
	}
	t.Log("protected audit accepted the same package")
}

func TestPackageWalkSkipsIgnoredGoTrees(t *testing.T) {
	root := t.TempDir()
	writeAuditFixture(t, root, "cmd/kept/kept_test.go", "package kept\n")
	writeAuditFixture(t, root, "cmd/kept/_broken_test.go", "this is not Go")
	writeAuditFixture(t, root, "cmd/kept/.broken_test.go", "this is not Go")
	writeAuditFixture(t, root, "internal/nested/go.mod", "module example.com/nested\n")
	writeAuditFixture(t, root, "internal/nested/broken_test.go", "this is not Go")
	writeAuditFixture(t, root, "internal/_ignored/broken_test.go", "this is not Go")
	writeAuditFixture(t, root, "internal/.ignored/broken_test.go", "this is not Go")
	writeAuditFixture(t, root, "internal/vendor/broken_test.go", "this is not Go")
	writeAuditFixture(t, root, "internal/testdata/broken_test.go", "this is not Go")
	writeAuditFixture(t, root, "artifacts/copied/internal/broken_test.go", "this is not Go")

	packages, _, err := loadPackageTests(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(packages) != 1 || packages["cmd/kept"] == nil {
		t.Fatalf("walked packages = %v, want only cmd/kept", sortedPackageNames(packages))
	}
}

func TestPackageWalkExternalCheckout(t *testing.T) {
	root := os.Getenv("TESTENV_PROTECTION_EXTERNAL_ROOT")
	if root == "" {
		t.Skip("TESTENV_PROTECTION_EXTERNAL_ROOT is not set")
	}
	pruningProbe := t.TempDir()
	writeAuditFixture(t, pruningProbe, "internal/kept/kept_test.go", "package kept\n")
	writeAuditFixture(t, pruningProbe, "internal/artifacts/copied/copied_test.go", "package copied\n")
	_, pruningWalk, err := loadPackageTests(pruningProbe)
	if err != nil {
		t.Fatal(err)
	}
	for _, directory := range pruningWalk {
		if slices.Contains(strings.Split(directory, "/"), "artifacts") {
			t.Fatalf("walk entered pruned directory %s", directory)
		}
	}
	packages, walkedDirectories, err := loadPackageTests(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, directory := range walkedDirectories {
		if directory != "cmd" && !strings.HasPrefix(directory, "cmd/") && directory != "internal" && !strings.HasPrefix(directory, "internal/") {
			t.Fatalf("walk escaped cmd and internal into %s", directory)
		}
		if slices.Contains(strings.Split(directory, "/"), "artifacts") {
			t.Fatalf("walk entered pruned directory %s", directory)
		}
	}
	t.Logf("walk read %d package directories across %d directories; none came from artifacts", len(packages), len(walkedDirectories))
}

func parsePackageSource(t *testing.T, source string) *packageTests {
	t.Helper()
	return &packageTests{files: []testFile{parseTestFile(t, "fixture_test.go", source)}}
}

func parseTestFile(t *testing.T, name, source string) testFile {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), name, source, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	return testFile{name: name, syntax: file}
}

func writeAuditFixture(t *testing.T, root, name, contents string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

func sortedPackageNames(packages map[string]*packageTests) []string {
	names := make([]string, 0, len(packages))
	for name := range packages {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
