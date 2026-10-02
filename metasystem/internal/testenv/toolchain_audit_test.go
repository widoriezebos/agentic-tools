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

const toolchainAllowanceUpdate = "TOOLCHAIN_ALLOWANCE_UPDATE"

// goSubcommands are the go commands a call names right after a literal
// "go" argument, as in mustRun(dir, "go", "build", ...).
var goSubcommands = map[string]bool{
	"build": true, "clean": true, "env": true, "generate": true, "install": true, "list": true,
	"mod": true, "run": true, "telemetry": true, "test": true, "tool": true, "vet": true, "work": true,
}

// nativeDiscoveryCalls run whole-module `go list -json -deps -test ./...`
// discovery (or the go gate) on the module a test names.
var nativeDiscoveryCalls = map[string]bool{"CheckNativeDiscovery": true, "RunGoGateTests": true}

type toolchainCall struct {
	name     string
	position token.Position
}

// TestNoTestRunsTheGoToolchainOutsideTheSlot counts, in every _test.go
// file, the calls that start the Go toolchain directly: exec.Command or
// exec.CommandContext of "go", any call but a path Join passing "go" and a
// go subcommand as adjacent literal arguments, and the native discovery entry points
// (CheckNativeDiscovery, RunGoGateTests). Under a full suite each such call
// is a toolchain beside -p packages times -parallel tests; the toolchain
// helpers of this package (Go, GoContext, Engine, HoldToolchain) run one at a
// time per binary and build the engine once. A file's count may only fall.
// Set TOOLCHAIN_ALLOWANCE_UPDATE=1 to rewrite the allowance after removing a
// call; update mode always fails so the rewritten file must be reviewed.
func TestNoTestRunsTheGoToolchainOutsideTheSlot(t *testing.T) {
	t.Parallel()

	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	callsByPath, err := findTestToolchainCalls(root)
	if err != nil {
		t.Fatal(err)
	}
	allowancePath := filepath.Join(root, "internal", "testenv", "testdata", "toolchain-allowance.tsv")
	allowances, problems := readToolchainAllowances(allowancePath, root)
	if os.Getenv(toolchainAllowanceUpdate) == "1" {
		if err := writeToolchainAllowances(allowancePath, callsByPath, allowances); err != nil {
			t.Fatal(err)
		}
		t.Fatalf("%s=1 rewrote internal/testenv/testdata/toolchain-allowance.tsv; update mode always fails", toolchainAllowanceUpdate)
	}
	for _, problem := range problems {
		t.Error(problem)
	}
	if len(problems) != 0 {
		return
	}
	paths := map[string]bool{}
	for path := range callsByPath {
		paths[path] = true
	}
	for path := range allowances {
		paths[path] = true
	}
	ordered := make([]string, 0, len(paths))
	for path := range paths {
		ordered = append(ordered, path)
	}
	sort.Strings(ordered)
	for _, path := range ordered {
		calls, allowed := callsByPath[path], allowances[path].count
		switch {
		case len(calls) > allowed:
			t.Errorf("%s: %d allowed, %d found; a test runs the Go toolchain through testenv.Go, testenv.Engine or testenv.HoldToolchain:\n%s",
				path, allowed, len(calls), describeToolchainCalls(path, calls))
		case len(calls) < allowed:
			t.Errorf("%s: %d allowed, %d found; lower the allowance number to %d:\n%s", path, allowed, len(calls), len(calls), describeToolchainCalls(path, calls))
		}
	}
}

func findTestToolchainCalls(root string) (map[string][]toolchainCall, error) {
	callsByPath := map[string][]toolchainCall{}
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
		file, err := parser.ParseFile(fileSet, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		execAliases := importedAliases(file, "os/exec", "exec")
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
			if name := toolchainCallName(call, execAliases); name != "" {
				callsByPath[relative] = append(callsByPath[relative], toolchainCall{name: name, position: fileSet.Position(call.Pos())})
			}
			return true
		})
		return nil
	})
	return callsByPath, err
}

func toolchainCallName(call *ast.CallExpr, execAliases map[string]bool) string {
	var callee string
	switch fun := call.Fun.(type) {
	case *ast.SelectorExpr:
		callee = fun.Sel.Name
		if receiver, ok := fun.X.(*ast.Ident); ok && execAliases[receiver.Name] && (callee == "Command" || callee == "CommandContext") {
			program := 0
			if callee == "CommandContext" {
				program = 1
			}
			if len(call.Args) > program && stringLiteral(call.Args[program]) == "go" {
				return "exec." + callee + "(go)"
			}
		}
	case *ast.Ident:
		callee = fun.Name
	}
	if nativeDiscoveryCalls[callee] {
		return callee
	}
	if callee == "Join" {
		// filepath.Join(home, ".config", "go", "env") names a file.
		return ""
	}
	for index := 0; index+1 < len(call.Args); index++ {
		if stringLiteral(call.Args[index]) == "go" && goSubcommands[stringLiteral(call.Args[index+1])] {
			return callee + "(go " + stringLiteral(call.Args[index+1]) + ")"
		}
	}
	return ""
}

func stringLiteral(expression ast.Expr) string {
	literal, ok := expression.(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return ""
	}
	value, err := strconv.Unquote(literal.Value)
	if err != nil {
		return ""
	}
	return value
}

func TestToolchainCounterFindsEveryDirectToolchainShape(t *testing.T) {
	t.Parallel()

	const source = `package fixture
import (
	"context"
	run "os/exec"
)
func fixture(ctx context.Context) {
	_ = run.Command("go", "build", ".")
	_ = run.CommandContext(ctx, "go", args...)
	mustRun(dir, "go", "list", "./...")
	_ = proofrun.CheckNativeDiscovery(ctx, "", "", contract, nil)
	_, _, _ = RunGoGateTests(ctx, request)
	_ = run.Command("git", "go", "build")
	_ = testenv.Go("build", ".")
	_ = run.Command("sh", "-c", "go build")
	_ = "exec.Command(\"go\", \"list\")"
	_ = filepath.Join(home, "go", "env")
}
`
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "fixture_test.go"), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	callsByPath, err := findTestToolchainCalls(root)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, call := range callsByPath["fixture_test.go"] {
		got = append(got, fmt.Sprintf("%s:%d", call.name, call.position.Line))
	}
	want := []string{
		"exec.Command(go):7",
		"exec.CommandContext(go):8",
		"mustRun(go list):9",
		"CheckNativeDiscovery:10",
		"RunGoGateTests:11",
		"Command(go build):12",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("toolchain calls=%q, want %q", got, want)
	}
}

type toolchainAllowance struct {
	count  int
	reason string
}

func readToolchainAllowances(path, root string) (map[string]toolchainAllowance, []string) {
	file, err := os.Open(path)
	if err != nil {
		return nil, []string{err.Error()}
	}
	defer file.Close()
	allowances := map[string]toolchainAllowance{}
	var problems []string
	previous := ""
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
		count, countErr := strconv.Atoi(fields[1])
		if fields[0] == "" || countErr != nil || count < 1 {
			problems = append(problems, fmt.Sprintf("%s:%d: invalid path or count", path, lineNumber))
			continue
		}
		if strings.TrimSpace(fields[2]) == "" || fields[2] == "UNEXPLAINED" {
			problems = append(problems, fmt.Sprintf("%s:%d: %s has no reason", path, lineNumber, fields[0]))
			continue
		}
		if previous != "" && fields[0] <= previous {
			problems = append(problems, fmt.Sprintf("%s:%d: allowance paths are not sorted: %s follows %s", path, lineNumber, fields[0], previous))
		}
		previous = fields[0]
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(fields[0]))); err != nil {
			problems = append(problems, fmt.Sprintf("%s:%d: listed file is unreadable: %v", path, lineNumber, err))
		}
		allowances[fields[0]] = toolchainAllowance{count: count, reason: fields[2]}
	}
	if err := scanner.Err(); err != nil {
		problems = append(problems, err.Error())
	}
	return allowances, problems
}

func writeToolchainAllowances(path string, callsByPath map[string][]toolchainCall, existing map[string]toolchainAllowance) error {
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

func describeToolchainCalls(path string, calls []toolchainCall) string {
	if len(calls) == 0 {
		return "  (no counted calls)"
	}
	lines := make([]string, 0, len(calls))
	for _, call := range calls {
		lines = append(lines, fmt.Sprintf("  %s:%d: %s", path, call.position.Line, call.name))
	}
	return strings.Join(lines, "\n")
}
