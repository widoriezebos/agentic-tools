package layering

import (
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

// execAllowance is one program the engine may name literally when it execs.
// R-138-m1e (Go decides natively) allows exactly two exec shapes: launching a
// process the system supervises, and invoking a component whose job is the
// shell (an adapter or a declared command). A program named by a variable is
// one of those launches (the engine itself, an adapter's argv, a declared
// command) and is not judged here; a program named by a literal is a fixed
// tool, and every fixed tool must be listed with the reason it is lawful.
type execAllowance struct {
	// files are the module-relative files or directory prefixes (ending in
	// "/") that may name the program; empty means any file.
	files  []string
	reason string
}

var lawfulExecPrograms = map[string]execAllowance{
	"git": {reason: "the repository is git's: no in-module Go library reads or writes it, so git is the version-control adapter every owner drives"},
	"go": {
		files: []string{"cmd/devgate/main.go", "cmd/metasystem/proof_run.go", "cmd/metasystem/intent_adopt.go", "cmd/metasystem/landing_path.go",
			"internal/testenv/toolchain.go"},
		reason: "the Go toolchain builds and runs the engine and the development gate: a supervised launch of the language adapter; " +
			"internal/testenv/toolchain.go: test support, the one place tests start the toolchain, under a per-binary slot",
	},
	"bash": {
		files:  []string{"internal/landing/receipt.go", "internal/contract/measure.go", "internal/testutil/"},
		reason: "runs a command the adopter declared (a proof or measurement command, an extension point) or drives a fixture from test support",
	},
	"/usr/bin/getconf": {
		files: []string{"internal/diskstore/host_darwin.go"},
		reason: "the adapter to confstr(_CS_DARWIN_USER_TEMP_DIR): the per-user temporary root Darwin assigns, which ignores TMPDIR; " +
			"Go without cgo has no confstr, and the path is derived from the user's directory-services UUID, not a file the engine can read",
	},
	"/bin/sh": {
		files: []string{"internal/testutil/", "internal/landing/plain/prove.go"},
		reason: "test support drives a fixture script; never production decision code. " +
			"internal/landing/plain/prove.go: adapter: runs the project's configured landing.prove.command",
	},
}

// execCallPrograms maps a package-qualified exec entry point to the index of
// the argument that names the program.
var execCallPrograms = map[string]map[string]int{
	"os/exec": {"Command": 0, "CommandContext": 1},
	"os":      {"StartProcess": 0},
	"syscall": {"Exec": 0, "ForkExec": 0},
}

// judgeNativeExec returns one line per literal program a file execs that is
// not a lawful fixed tool for that file.
func judgeNativeExec(relative string, source []byte) ([]string, error) {
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, relative, source, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	importNames := map[string]string{}
	for _, spec := range file.Imports {
		path, _ := strconv.Unquote(spec.Path.Value)
		if _, judged := execCallPrograms[path]; !judged {
			continue
		}
		name := filepath.Base(path)
		if spec.Name != nil {
			name = spec.Name.Name
		}
		importNames[name] = path
	}
	if len(importNames) == 0 {
		return nil, nil
	}
	constants := fileStringConstants(file)
	var violations []string
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		qualifier, ok := selector.X.(*ast.Ident)
		if !ok {
			return true
		}
		path, imported := importNames[qualifier.Name]
		if !imported {
			return true
		}
		index, entry := execCallPrograms[path][selector.Sel.Name]
		if !entry || index >= len(call.Args) {
			return true
		}
		program, literal := literalProgram(call.Args[index], constants)
		if !literal {
			return true
		}
		if lawfulExecFor(program, relative) {
			return true
		}
		violations = append(violations, fileSet.Position(call.Pos()).String()+": execs "+strconv.Quote(program)+"; Go decides natively (R-138-m1e): read the kernel or use a Go library, or list the program in lawfulExecPrograms with the reason it is a supervised launch or an adapter")
		return true
	})
	return violations, nil
}

func literalProgram(argument ast.Expr, constants map[string]string) (string, bool) {
	switch value := argument.(type) {
	case *ast.BasicLit:
		if value.Kind != token.STRING {
			return "", false
		}
		program, err := strconv.Unquote(value.Value)
		return program, err == nil
	case *ast.Ident:
		program, known := constants[value.Name]
		return program, known
	}
	return "", false
}

// fileStringConstants is every constant in the file bound to a string
// literal, so naming a tool through a constant is judged like the literal.
func fileStringConstants(file *ast.File) map[string]string {
	constants := map[string]string{}
	ast.Inspect(file, func(node ast.Node) bool {
		declaration, ok := node.(*ast.GenDecl)
		if !ok || declaration.Tok != token.CONST {
			return true
		}
		for _, spec := range declaration.Specs {
			valueSpec := spec.(*ast.ValueSpec)
			for index, name := range valueSpec.Names {
				if index >= len(valueSpec.Values) {
					continue
				}
				if literal, ok := valueSpec.Values[index].(*ast.BasicLit); ok && literal.Kind == token.STRING {
					if value, err := strconv.Unquote(literal.Value); err == nil {
						constants[name.Name] = value
					}
				}
			}
		}
		return true
	})
	return constants
}

func lawfulExecFor(program, relative string) bool {
	allowance, listed := lawfulExecPrograms[program]
	if !listed {
		return false
	}
	if len(allowance.files) == 0 {
		return true
	}
	for _, file := range allowance.files {
		if relative == file || strings.HasSuffix(file, "/") && strings.HasPrefix(relative, file) {
			return true
		}
	}
	return false
}

// TestGoDecidesNativelyExecsOnlyLawfulPrograms walks every production Go file
// in the module and refuses a literal program that is not a lawful fixed
// tool. The positive control is that git, the one tool every owner drives, is
// actually seen, so a walk that read nothing cannot pass.
func TestGoDecidesNativelyExecsOnlyLawfulPrograms(t *testing.T) {
	t.Parallel()
	module, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	var violations []string
	judged := 0
	err = filepath.WalkDir(module, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			name := entry.Name()
			if path != module && (name == "testdata" || name == "vendor" || strings.HasPrefix(name, ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		relative, err := filepath.Rel(module, path)
		if err != nil {
			return err
		}
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		found, err := judgeNativeExec(filepath.ToSlash(relative), source)
		if err != nil {
			return err
		}
		if strings.Contains(string(source), `"git"`) {
			judged++
		}
		violations = append(violations, found...)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if judged == 0 {
		t.Fatal("the walk judged no file that names git; it read nothing")
	}
	sort.Strings(violations)
	if len(violations) > 0 {
		t.Fatalf("production code execs a fixed program it may not:\n%s", strings.Join(violations, "\n"))
	}
}

// TestGoDecidesNativelyJudgeNamesEveryShape is the judge's own witness: a
// literal, a constant, an aliased import and CommandContext are all seen; a
// lawful tool in its own file and a variable program pass.
func TestGoDecidesNativelyJudgeNamesEveryShape(t *testing.T) {
	t.Parallel()
	source := `package p

import (
	"context"
	run "os/exec"
	"os"
)

const lister = "ps"

func f(ctx context.Context, engine string) {
	_ = run.Command("ps", "-axo", "pid=")
	_ = run.Command(lister)
	_ = run.CommandContext(ctx, "tar", "-xf", "a.tar")
	_, _ = os.StartProcess("/bin/sh", nil, nil)
	_ = run.Command("git", "status")
	_ = run.Command(engine, "mission")
	_ = run.Command("bash", "-c", "true")
}
`
	violations, err := judgeNativeExec("internal/example/example.go", []byte(source))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{`"ps"`, `"ps"`, `"tar"`, `"/bin/sh"`, `"bash"`}
	if len(violations) != len(want) {
		t.Fatalf("violations = %q, want one per %v", violations, want)
	}
	for index, program := range want {
		if !strings.Contains(violations[index], "execs "+program) {
			t.Fatalf("violation %d = %q, want %s", index, violations[index], program)
		}
	}
	lawful, err := judgeNativeExec("internal/landing/receipt.go", []byte(source))
	if err != nil {
		t.Fatal(err)
	}
	for _, violation := range lawful {
		if strings.Contains(violation, `"bash"`) {
			t.Fatalf("bash is lawful in the landing receipt's declared-command runner: %q", violation)
		}
	}
}
