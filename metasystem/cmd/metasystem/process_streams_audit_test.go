package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// processStreamSwapAllowance names the test functions whose subject is the
// process's own standard stream, keyed "file#Function" relative to the
// module root, each with the reason it may assign os.Stdout or os.Stderr.
// Every other test hands the code under test writers of its own.
var processStreamSwapAllowance = map[string]string{}

// TestNoTestSwapsTheProcessStreams: os.Stdout and os.Stderr are process
// globals, so a test that assigns them receives whatever every parallel test
// prints while its capture is open, and hands its own output to theirs.
// Code under test prints on the writers its caller passes; no test in the
// module assigns either stream outside the allowance above.
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
			if path != root && (name == "testdata" || name == "vendor" || name == "node_modules" || strings.HasPrefix(name, ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, "_test.go") {
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
			t.Errorf("%s:%d assigns os.%s in %s; pass the code under test a writer of the test's own instead", relative, swap.line, swap.stream, swap.function)
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

type processStreamSwap struct {
	function, stream string
	line             int
}

// processStreamSwaps finds every assignment to os.Stdout or os.Stderr in
// file, under whatever name the file imports package os, and the top-level
// function it is in ("" outside every function).
func processStreamSwaps(fileSet *token.FileSet, file *ast.File) []processStreamSwap {
	osNames := map[string]bool{}
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil || path != "os" {
			continue
		}
		if spec.Name != nil {
			osNames[spec.Name.Name] = true
		} else {
			osNames["os"] = true
		}
	}
	if len(osNames) == 0 {
		return nil
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
		ast.Inspect(declaration, func(node ast.Node) bool {
			assignment, ok := node.(*ast.AssignStmt)
			if !ok {
				return true
			}
			for _, left := range assignment.Lhs {
				selector, ok := ast.Unparen(left).(*ast.SelectorExpr)
				if !ok || (selector.Sel.Name != "Stdout" && selector.Sel.Name != "Stderr") {
					continue
				}
				if pkg, ok := selector.X.(*ast.Ident); ok && osNames[pkg.Name] {
					swaps = append(swaps, processStreamSwap{function: function, stream: selector.Sel.Name, line: fileSet.Position(left.Pos()).Line})
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
// plain, an aliased, a parenthesised and a method-held assignment are all
// found; a read of the stream, another package's Stdout and a local named
// os are not.
func TestProcessStreamSwapsFindsEveryAssignment(t *testing.T) {
	t.Parallel()
	source := `package p

import (
	"os"
	sys "os"
	other "example.com/other"
)

func plain(w *os.File) { os.Stdout = w }

func aliased(w *os.File) { saved := sys.Stderr; sys.Stderr = w; sys.Stderr = saved }

func paren(w *os.File) { (os.Stdout), _ = w, 1 }

type bed struct{}

func (b *bed) method(w *os.File) { os.Stderr = w }

func reads() { _ = os.Stdout; other.Stdout = nil }
`
	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(fileSet, "fixture_test.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	got := processStreamSwaps(fileSet, parsed)
	want := []processStreamSwap{
		{function: "plain", stream: "Stdout", line: 9},
		{function: "aliased", stream: "Stderr", line: 11},
		{function: "aliased", stream: "Stderr", line: 11},
		{function: "paren", stream: "Stdout", line: 13},
		{function: "bed.method", stream: "Stderr", line: 17},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("swaps = %+v, want %+v", got, want)
	}
}
