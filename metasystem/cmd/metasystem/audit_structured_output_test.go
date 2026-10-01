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

// The structured-output audit (design structured-output.md §5, R5): a
// MetaSystem process never decides anything from another MetaSystem
// process's human text. It reads the child's --json envelope through
// internal/verbresult. The audit finds every place a non-test Go function
// captures a child's output (CombinedOutput, Output, or a buffer set as the
// command's Stdout) and refuses the capture unless the command's program is
// a literal tool name (git, go, ps, vm_stat...: other tools' text is
// allowed), the capture is the structured reader itself, or the function is
// a listed exception with its reason. A program held in a variable is
// presumed to be our own engine.

// structuredOutputExceptions are the narrow file:function captures of a
// program held in a variable that are not our engine's human text: other
// tools (git, go, vm_stat, runtime and application CLIs, an adopter's
// adapter) and the few machine protocols of our engine that are already
// structured, where the words are at most quoted to a person. There is no
// baseline: every other capture of our engine reads its --json envelope.
// Each entry names its reason.
var structuredOutputExceptions = map[string]string{
	"cmd/metasystem/app.go:toolVersionLine":                      "an application tool's --version line, shown to a person",
	"cmd/metasystem/test_protection.go:runFrozenWorkerProbe":     "our engine's test worker: its --result file is the answer; its words are quoted only when a probe fails",
	"internal/adapter/supervisor/identity.go:Deps.cliVersion":    "a runtime CLI's --version",
	"internal/candidateengine/engine.go:Build":                   "go build",
	"internal/candidateengine/engine.go:commitTree":              "git commit-tree through scratchGitCommand",
	"internal/candidateengine/engine.go:projectionTree":          "git plumbing through scratchGitCommand",
	"internal/landing/laneengine/advance.go:ProductionSteps":     "go run ./cmd/devgate build: the exit decides, the output's tail is only quoted in the error",
	"internal/goal/attention.go:captureLocalTipBounded":          "git rev-parse --verify",
	"internal/hostload/memory.go:AvailableMemory":                "vm_stat and getconf",
	"internal/ledgerfence/fence.go:Ensure":                       "the git hook chain (adopter-extensible scripts) answering the guard's nonce ack with exit 42",
	"internal/proofrun/resource_custody.go:startResourceCustody": "our custodian: the --ready-fd protocol is the answer; its stderr is quoted only in a failure",
	"internal/runtimes/external/external.go:Adapter.Call":        "an adopter's external adapter JSON protocol",
	"internal/seat/launch/host.go:OSRunner.Run":                  "seat launch steps (git and our engine): the exit decides, the words are quoted into the launch record",
	"internal/testrun/worker.go:RequireWorkerCapabilities":       "our engine's worker handshake: one typed JSON answer on stdout, not words",
}

// structuredCapture is one capture of a child's output the audit judged.
type structuredCapture struct {
	Site    string // module-relative file:function
	Program string // the program expression, as written
	Line    int
}

// structuredOutputCaptures scans one parsed file for captures of a child
// whose program is not a string literal.
func structuredOutputCaptures(fset *token.FileSet, rel string, file *ast.File) []structuredCapture {
	var captures []structuredCapture
	for _, decl := range file.Decls {
		switch typed := decl.(type) {
		case *ast.FuncDecl:
			if typed.Body == nil {
				continue
			}
			name := typed.Name.Name
			if typed.Recv != nil && len(typed.Recv.List) == 1 {
				receiver := typed.Recv.List[0].Type
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
			captures = append(captures, structuredOutputCapturesIn(fset, rel+":"+name, typed.Body)...)
		case *ast.GenDecl:
			// A package variable holding a function (a seam) is its own site.
			for _, spec := range typed.Specs {
				value, ok := spec.(*ast.ValueSpec)
				if !ok || len(value.Names) == 0 {
					continue
				}
				for index, initial := range value.Values {
					name := value.Names[0].Name
					if index < len(value.Names) {
						name = value.Names[index].Name
					}
					captures = append(captures, structuredOutputCapturesIn(fset, rel+":"+name, initial)...)
				}
			}
		}
	}
	return captures
}

// structuredOutputCapturesIn finds the captures inside one function body
// (or a package variable's initial value) named site.
func structuredOutputCapturesIn(fset *token.FileSet, site string, body ast.Node) []structuredCapture {
	type binding struct {
		at      token.Pos
		program ast.Expr // nil: built by a helper
		command bool
		buffer  bool
	}
	bindings := map[string][]binding{} // variable -> its assignments in source order
	bind := func(lhs ast.Expr, rhs ast.Expr, at token.Pos) {
		ident, ok := lhs.(*ast.Ident)
		if !ok {
			return
		}
		if program, isCommand := execProgram(rhs); isCommand {
			bindings[ident.Name] = append(bindings[ident.Name], binding{at: at, program: program, command: true})
			return
		}
		if call, ok := rhs.(*ast.CallExpr); ok && returnsCommand(call) {
			bindings[ident.Name] = append(bindings[ident.Name], binding{at: at, command: true})
			return
		}
		if isBufferExpr(rhs) {
			bindings[ident.Name] = append(bindings[ident.Name], binding{at: at, buffer: true})
		}
	}
	ast.Inspect(body, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.AssignStmt:
			if len(typed.Lhs) == len(typed.Rhs) {
				for index := range typed.Lhs {
					bind(typed.Lhs[index], typed.Rhs[index], typed.Pos())
				}
			}
		case *ast.ValueSpec:
			for index, ident := range typed.Names {
				if index < len(typed.Values) {
					bind(ident, typed.Values[index], typed.Pos())
				} else if typed.Type != nil && isBufferType(typed.Type) {
					bindings[ident.Name] = append(bindings[ident.Name], binding{at: typed.Pos(), buffer: true})
				}
			}
		}
		return true
	})
	// latest is the binding of name in force at a use: the last one before it.
	latest := func(name string, at token.Pos) (binding, bool) {
		var found binding
		ok := false
		for _, candidate := range bindings[name] {
			if candidate.at < at {
				found, ok = candidate, true
			}
		}
		return found, ok
	}
	program := func(target ast.Expr, at token.Pos) (ast.Expr, bool) {
		if program, isCommand := execProgram(target); isCommand {
			return program, true
		}
		if ident, ok := target.(*ast.Ident); ok {
			bound, known := latest(ident.Name, at)
			return bound.program, known && bound.command
		}
		if call, ok := target.(*ast.CallExpr); ok && returnsCommand(call) {
			return nil, true
		}
		return nil, false
	}
	inMemory := func(expr ast.Expr, at token.Pos) bool {
		if unary, ok := expr.(*ast.UnaryExpr); ok && unary.Op == token.AND {
			if ident, ok := unary.X.(*ast.Ident); ok {
				bound, known := latest(ident.Name, at)
				return known && bound.buffer
			}
			if literal, ok := unary.X.(*ast.CompositeLit); ok {
				return isBufferType(literal.Type)
			}
		}
		if ident, ok := expr.(*ast.Ident); ok {
			bound, known := latest(ident.Name, at)
			return known && bound.buffer
		}
		return isBufferExpr(expr)
	}
	var captures []structuredCapture
	record := func(target ast.Expr, at token.Pos) {
		expr, isCommand := program(target, at)
		if !isCommand {
			return
		}
		if literal, ok := expr.(*ast.BasicLit); ok && literal.Kind == token.STRING {
			return
		}
		captures = append(captures, structuredCapture{Site: site, Program: structuredExprText(expr), Line: fset.Position(at).Line})
	}
	ast.Inspect(body, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.CallExpr:
			selector, ok := typed.Fun.(*ast.SelectorExpr)
			if ok && len(typed.Args) == 0 && (selector.Sel.Name == "CombinedOutput" || selector.Sel.Name == "Output") {
				record(selector.X, typed.Pos())
			}
		case *ast.AssignStmt:
			for index, lhs := range typed.Lhs {
				selector, ok := lhs.(*ast.SelectorExpr)
				if !ok || (selector.Sel.Name != "Stdout" && selector.Sel.Name != "Stderr") || index >= len(typed.Rhs) {
					continue
				}
				if inMemory(typed.Rhs[index], typed.Pos()) {
					record(selector.X, typed.Pos())
				}
			}
		}
		return true
	})
	return captures
}

// execProgram is the program of an exec.Command or exec.CommandContext call.
func execProgram(expr ast.Expr) (ast.Expr, bool) {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return nil, false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil, false
	}
	pkg, ok := selector.X.(*ast.Ident)
	if !ok || pkg.Name != "exec" {
		return nil, false
	}
	switch selector.Sel.Name {
	case "Command":
		if len(call.Args) > 0 {
			return call.Args[0], true
		}
	case "CommandContext":
		if len(call.Args) > 1 {
			return call.Args[1], true
		}
	}
	return nil, false
}

// returnsCommand reports a call to a helper whose name says it builds a
// command (BatchProofCommand, ...): its program is unknown here.
func returnsCommand(call *ast.CallExpr) bool {
	name := ""
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		name = fun.Name
	case *ast.SelectorExpr:
		name = fun.Sel.Name
	}
	return strings.HasSuffix(name, "Command") && name != "Command"
}

func isBufferType(expr ast.Expr) bool {
	selector, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := selector.X.(*ast.Ident)
	return ok && (pkg.Name == "bytes" && selector.Sel.Name == "Buffer" || pkg.Name == "strings" && selector.Sel.Name == "Builder")
}

func isBufferExpr(expr ast.Expr) bool {
	switch typed := expr.(type) {
	case *ast.UnaryExpr:
		if literal, ok := typed.X.(*ast.CompositeLit); ok {
			return isBufferType(literal.Type)
		}
	case *ast.CallExpr:
		if ident, ok := typed.Fun.(*ast.Ident); ok && ident.Name == "new" && len(typed.Args) == 1 {
			return isBufferType(typed.Args[0])
		}
		if selector, ok := typed.Fun.(*ast.SelectorExpr); ok {
			if pkg, ok := selector.X.(*ast.Ident); ok && pkg.Name == "bytes" && strings.HasPrefix(selector.Sel.Name, "NewBuffer") {
				return true
			}
		}
	}
	return false
}

func structuredExprText(expr ast.Expr) string {
	switch typed := expr.(type) {
	case nil:
		return "(built by a helper)"
	case *ast.Ident:
		return typed.Name
	case *ast.SelectorExpr:
		return structuredExprText(typed.X) + "." + typed.Sel.Name
	case *ast.CallExpr:
		return structuredExprText(typed.Fun) + "(...)"
	case *ast.IndexExpr:
		return structuredExprText(typed.X) + "[...]"
	case *ast.BasicLit:
		return typed.Value
	}
	return "expression"
}

// scanStructuredOutput walks the non-test Go under dir (module-relative
// names from moduleRoot) and returns every capture of a variable program.
func scanStructuredOutput(t *testing.T, moduleRoot string, dirs ...string) []structuredCapture {
	t.Helper()
	var captures []structuredCapture
	fset := token.NewFileSet()
	for _, dir := range dirs {
		err := filepath.WalkDir(filepath.Join(moduleRoot, dir), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if name := entry.Name(); name == "testdata" || strings.HasPrefix(name, ".") {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			rel, err := filepath.Rel(moduleRoot, path)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			if strings.HasPrefix(rel, "internal/verbresult/") {
				return nil // the structured reader itself
			}
			file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
			if err != nil {
				return err
			}
			captures = append(captures, structuredOutputCaptures(fset, rel, file)...)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return captures
}

func TestAuditNoProcessReadsAnotherProcessText(t *testing.T) {
	t.Parallel()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	captures := scanStructuredOutput(t, root, "cmd", "internal")
	seen := map[string]bool{}
	var refused []string
	for _, capture := range captures {
		seen[capture.Site] = true
		if structuredOutputExceptions[capture.Site] != "" {
			continue
		}
		refused = append(refused, capture.Site+" (line "+strconv.Itoa(capture.Line)+", program "+capture.Program+")")
	}
	slices.Sort(refused)
	refused = slices.Compact(refused)
	if os.Getenv("STRUCTURED_OUTPUT_DUMP") != "" {
		for _, line := range refused {
			t.Log(line)
		}
	}
	if len(refused) != 0 {
		t.Errorf("%d captures read a child process's text instead of its --json envelope:\n  %s\n"+
			"read the child through internal/verbresult.Run, or, for a program that is not our engine, add its file:function "+
			"with a reason to structuredOutputExceptions in cmd/metasystem/audit_structured_output_test.go",
			len(refused), strings.Join(refused, "\n  "))
	}
	for site := range structuredOutputExceptions {
		if !seen[site] {
			t.Errorf("exception %s no longer captures a child's output: delete it from structuredOutputExceptions", site)
		}
	}
}

// TestAuditStructuredOutputFixtures (R5): the audit catches a capture of
// our engine even when a helper does the text classifying, and passes
// literal tool captures (git, go) and the structured reader.
func TestAuditStructuredOutputFixtures(t *testing.T) {
	t.Parallel()
	source := `package fixture

import (
	"os"
	"os/exec"
	"strings"
)

func classify(text string) string {
	if strings.Contains(text, "GOAL_REVISION_MOVED") {
		return "revision"
	}
	return ""
}

func helperClassifier(binary string) string {
	command := exec.Command(binary, "internal", "test", "run")
	output, _ := command.CombinedOutput()
	return classify(string(output))
}

func bufferClassifier() string {
	self, _ := os.Executable()
	command := exec.Command(self, "test", "plan")
	var stdout bytes.Buffer
	command.Stdout = &stdout
	_ = command.Run()
	return classify(stdout.String())
}

func builtByHelper(binary string) string {
	output, _ := BatchProofCommand(binary).Output()
	return classify(string(output))
}

func gitTop() string {
	output, _ := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	return strings.TrimSpace(string(output))
}

func goList() string {
	command := exec.Command("go", "list", "./...")
	output, _ := command.CombinedOutput()
	return string(output)
}

func streamed(binary string) error {
	command := exec.Command(binary, "up")
	command.Stdout = os.Stderr
	return command.Run()
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "fixture.go", source, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	var sites []string
	for _, capture := range structuredOutputCaptures(fset, "fixture.go", file) {
		sites = append(sites, capture.Site)
	}
	want := []string{"fixture.go:helperClassifier", "fixture.go:bufferClassifier", "fixture.go:builtByHelper"}
	if !slices.Equal(sites, want) {
		t.Fatalf("caught %v, want %v", sites, want)
	}
}
