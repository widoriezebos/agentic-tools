package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	pathpkg "path"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// processStreamSwapAllowance names the functions whose subject is the
// process's own standard stream, keyed "file#Function" relative to the
// module root, each with the reason it may replace os.Stdout or os.Stderr.
// Every other function hands the code it calls writers of its own.
var processStreamSwapAllowance = map[string]string{}

// TestNoTestSwapsTheProcessStreams: os.Stdout and os.Stderr are process
// globals, so a test that replaces them receives whatever every parallel test
// prints while its capture is open, and hands its own output to theirs.
// Code under test prints on the writers its caller passes; no Go file in the
// module (a test, or a helper without the _test suffix a test calls)
// assigns either stream, directly, through its address or a **os.File, or
// duplicates a descriptor onto 1 or 2, outside the allowance above.
func TestNoTestSwapsTheProcessStreams(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..")
	used := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := entry.Name()
		if entry.IsDir() {
			if path != root && auditSkipsDirectory(name) {
				return filepath.SkipDir
			}
			return nil
		}
		if !processStreamAuditReads(name) {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		fileSet := token.NewFileSet()
		parsed, err := parser.ParseFile(fileSet, path, nil, 0)
		if err != nil {
			return err
		}
		for _, swap := range processStreamSwaps(fileSet, parsed) {
			key := relative + "#" + swap.function
			if reason, ok := processStreamSwapAllowance[key]; ok {
				used[key] = reason
				continue
			}
			t.Errorf("%s:%d %s in %s; pass the code under test a writer of the test's own instead", relative, swap.line, swap.act, swap.function)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk the Go tree: %v", err)
	}
	if !reflect.DeepEqual(used, processStreamSwapAllowance) {
		t.Errorf("used allowances = %v, want exactly %v; remove an allowance whose site is gone", used, processStreamSwapAllowance)
	}
}

// processStreamAuditReads says which files the audit parses: every Go file.
func processStreamAuditReads(name string) bool { return strings.HasSuffix(name, ".go") }

type processStreamSwap struct {
	function, act string
	line          int
}

// processStreamSwaps finds, in file, every act that replaces the process's
// standard output or error, and the top-level function it is in ("" outside
// every function): an assignment to os.Stdout or os.Stderr (plain or
// through *), taking either's address, an assignment through a variable or
// parameter of type **os.File, and syscall or unix Dup2/Dup3 onto
// descriptor 1 or 2. Package names follow the file's imports.
func processStreamSwaps(fileSet *token.FileSet, file *ast.File) []processStreamSwap {
	imports := map[string]map[string]bool{"os": {}, "syscall": {}, "golang.org/x/sys/unix": {}}
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil || imports[path] == nil {
			continue
		}
		name := pathpkg.Base(path)
		if spec.Name != nil {
			name = spec.Name.Name
		}
		imports[path][name] = true
	}
	osNames := imports["os"]
	descriptorNames := map[string]bool{}
	for name := range imports["syscall"] {
		descriptorNames[name] = true
	}
	for name := range imports["golang.org/x/sys/unix"] {
		descriptorNames[name] = true
	}
	if len(osNames) == 0 && len(descriptorNames) == 0 {
		return nil
	}
	stream := func(expression ast.Expr) (string, bool) {
		selector, ok := ast.Unparen(expression).(*ast.SelectorExpr)
		if !ok || (selector.Sel.Name != "Stdout" && selector.Sel.Name != "Stderr") {
			return "", false
		}
		pkg, ok := selector.X.(*ast.Ident)
		if !ok || !osNames[pkg.Name] {
			return "", false
		}
		return selector.Sel.Name, true
	}
	isFileSlot := func(expression ast.Expr) bool {
		outer, ok := expression.(*ast.StarExpr)
		if !ok {
			return false
		}
		inner, ok := outer.X.(*ast.StarExpr)
		if !ok {
			return false
		}
		selector, ok := inner.X.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "File" {
			return false
		}
		pkg, ok := selector.X.(*ast.Ident)
		return ok && osNames[pkg.Name]
	}
	// standardDescriptor says whether a dup's target is descriptor 1 or 2:
	// the number, a Stdout/Stderr constant of syscall or unix, or any
	// expression reading os.Stdout or os.Stderr (their Fd()).
	standardDescriptor := func(expression ast.Expr) bool {
		found := false
		ast.Inspect(expression, func(node ast.Node) bool {
			switch typed := node.(type) {
			case *ast.BasicLit:
				found = found || (typed.Kind == token.INT && (typed.Value == "1" || typed.Value == "2"))
			case *ast.SelectorExpr:
				if _, ok := stream(typed); ok {
					found = true
				}
				if pkg, ok := typed.X.(*ast.Ident); ok && descriptorNames[pkg.Name] && (typed.Sel.Name == "Stdout" || typed.Sel.Name == "Stderr") {
					found = true
				}
			}
			return !found
		})
		return found
	}
	var swaps []processStreamSwap
	for _, declaration := range file.Decls {
		function := ""
		if decl, ok := declaration.(*ast.FuncDecl); ok {
			function = decl.Name.Name
			if decl.Recv != nil && len(decl.Recv.List) == 1 {
				function = receiverTypeName(decl.Recv.List[0].Type) + "." + function
			}
		}
		slots := map[string]bool{}
		ast.Inspect(declaration, func(node ast.Node) bool {
			switch typed := node.(type) {
			case *ast.Field:
				if isFileSlot(typed.Type) {
					for _, name := range typed.Names {
						slots[name.Name] = true
					}
				}
			case *ast.ValueSpec:
				if typed.Type != nil && isFileSlot(typed.Type) {
					for _, name := range typed.Names {
						slots[name.Name] = true
					}
				}
			}
			return true
		})
		record := func(node ast.Node, act string) {
			swaps = append(swaps, processStreamSwap{function: function, act: act, line: fileSet.Position(node.Pos()).Line})
		}
		ast.Inspect(declaration, func(node ast.Node) bool {
			switch typed := node.(type) {
			case *ast.AssignStmt:
				for _, left := range typed.Lhs {
					target := ast.Unparen(left)
					if star, ok := target.(*ast.StarExpr); ok {
						if name, ok := stream(star.X); ok {
							record(left, "assigns os."+name)
							continue
						}
						if slot, ok := ast.Unparen(star.X).(*ast.Ident); ok && slots[slot.Name] {
							record(left, "assigns through the **os.File "+slot.Name)
						}
						continue
					}
					if name, ok := stream(target); ok {
						record(left, "assigns os."+name)
					}
				}
			case *ast.UnaryExpr:
				if typed.Op == token.AND {
					if name, ok := stream(typed.X); ok {
						record(typed, "takes the address of os."+name)
					}
				}
			case *ast.CallExpr:
				selector, ok := typed.Fun.(*ast.SelectorExpr)
				if !ok || (selector.Sel.Name != "Dup2" && selector.Sel.Name != "Dup3") || len(typed.Args) < 2 {
					return true
				}
				if pkg, ok := selector.X.(*ast.Ident); ok && descriptorNames[pkg.Name] && standardDescriptor(typed.Args[1]) {
					record(typed, "duplicates a descriptor onto standard output or error")
				}
			}
			return true
		})
	}
	return swaps
}

func receiverTypeName(expression ast.Expr) string {
	switch typed := expression.(type) {
	case *ast.StarExpr:
		return receiverTypeName(typed.X)
	case *ast.IndexExpr:
		return receiverTypeName(typed.X)
	case *ast.IndexListExpr:
		return receiverTypeName(typed.X)
	case *ast.Ident:
		return typed.Name
	}
	return ""
}

// TestProcessStreamSwapsFindsEveryAssignment is the audit's own witness: a
// plain, an aliased, a parenthesised, a dereferenced and a method-held
// assignment are all found, as are the stream's address taken, a helper
// assigning through a **os.File, and a dup2 or dup3 onto descriptor 1 or 2
// by number, by syscall or unix constant, or by the stream's Fd; a read of
// the stream, another package's Stdout, a local named os, a dup onto
// another descriptor and a *os.File dereference that is no stream are not.
func TestProcessStreamSwapsFindsEveryAssignment(t *testing.T) {
	t.Parallel()
	source := `package p

import (
	"os"
	sys "os"
	"syscall"
	other "example.com/other"
	"golang.org/x/sys/unix"
)

func plain(w *os.File) { os.Stdout = w }

func aliased(w *os.File) { saved := sys.Stderr; sys.Stderr = w; sys.Stderr = saved }

func paren(w *os.File) { (os.Stdout), _ = w, 1 }

type bed struct{}

func (b *bed) method(w *os.File) { os.Stderr = w }

func reads() { _ = os.Stdout; other.Stdout = nil }

func deref(w *os.File) { *os.Stdout = *w }

func address() { swapStream(&os.Stderr, nil) }

func swapStream(target **os.File, w *os.File) { saved := *target; *target = w; _ = saved }

func held() { var slot **os.File; *slot = nil }

func dups(fd int) {
	syscall.Dup2(fd, 1)
	unix.Dup2(fd, 2)
	unix.Dup2(fd, 3)
	syscall.Dup2(fd, int(os.Stderr.Fd()))
	unix.Dup3(fd, syscall.Stdout, 0)
}

func notStreams(file *os.File, copy os.File) { *file = copy }
`
	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(fileSet, "fixture_test.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	got := processStreamSwaps(fileSet, parsed)
	want := []processStreamSwap{
		{function: "plain", act: "assigns os.Stdout", line: 11},
		{function: "aliased", act: "assigns os.Stderr", line: 13},
		{function: "aliased", act: "assigns os.Stderr", line: 13},
		{function: "paren", act: "assigns os.Stdout", line: 15},
		{function: "bed.method", act: "assigns os.Stderr", line: 19},
		{function: "deref", act: "assigns os.Stdout", line: 23},
		{function: "address", act: "takes the address of os.Stderr", line: 25},
		{function: "swapStream", act: "assigns through the **os.File target", line: 27},
		{function: "held", act: "assigns through the **os.File slot", line: 29},
		{function: "dups", act: "duplicates a descriptor onto standard output or error", line: 32},
		{function: "dups", act: "duplicates a descriptor onto standard output or error", line: 33},
		{function: "dups", act: "duplicates a descriptor onto standard output or error", line: 35},
		{function: "dups", act: "duplicates a descriptor onto standard output or error", line: 36},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("swaps = %+v, want %+v", got, want)
	}
}

// TestProcessStreamAuditReadsEveryGoFile: a helper file without the _test
// suffix swaps the streams for the tests that call it just the same, so the
// audit reads every Go file, not only the test files.
func TestProcessStreamAuditReadsEveryGoFile(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]bool{"capture_test.go": true, "capture.go": true, "fixture.txt": false, "go.mod": false} {
		if got := processStreamAuditReads(name); got != want {
			t.Errorf("processStreamAuditReads(%q) = %v, want %v", name, got, want)
		}
	}
}
