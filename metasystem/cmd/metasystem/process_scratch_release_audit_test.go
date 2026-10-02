package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// TestProcessScratchIsReleasedOnlyAtTheProcessEnd: the process scratch is
// one root per process, so only the process's own end may release it. A
// release at the end of each command run (the old dispatchWithFamilies)
// replaced the root under every other command the test binary ran in the
// same process at once. The audit holds both halves: production code calls
// diskstore.ReleaseProcessScratch only in a func main or in
// dispatchProcess, and this package's tests reach dispatchProcess (or
// dispatch) only where the test binary is the process: TestMain, or a
// helper command a re-exec'd child runs (testHelperCommands).
func TestProcessScratchIsReleasedOnlyAtTheProcessEnd(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..")
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := entry.Name()
		if entry.IsDir() {
			if path != root && (name == "testdata" || name == "vendor" || name == "node_modules" || name == "artifacts" || strings.HasPrefix(name, ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, ".go") {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		test := strings.HasSuffix(name, "_test.go")
		if test && filepath.ToSlash(filepath.Dir(relative)) != "cmd/metasystem" {
			return nil
		}
		fileSet := token.NewFileSet()
		file, err := parser.ParseFile(fileSet, path, nil, 0)
		if err != nil {
			return err
		}
		for _, decl := range file.Decls {
			function, ok := decl.(*ast.FuncDecl)
			if !ok || function.Body == nil {
				continue
			}
			if !test {
				if function.Recv == nil && (function.Name.Name == "main" || relative == "cmd/metasystem/main.go" && function.Name.Name == "dispatchProcess") {
					continue
				}
				ast.Inspect(function.Body, func(node ast.Node) bool {
					call, ok := node.(*ast.CallExpr)
					if !ok {
						return true
					}
					if selector, ok := call.Fun.(*ast.SelectorExpr); ok && selector.Sel.Name == "ReleaseProcessScratch" {
						t.Errorf("%s:%d %s releases the process scratch; only a func main or dispatchProcess may, at the process's end",
							relative, fileSet.Position(call.Pos()).Line, function.Name.Name)
					}
					return true
				})
				continue
			}
			if function.Recv == nil && function.Name.Name == "TestMain" {
				continue
			}
			ast.Inspect(function.Body, func(node ast.Node) bool {
				// A helper command runs in a re-exec'd child, which is the
				// process: testHelperCommands[name] = func(...) int {...}.
				if assign, ok := node.(*ast.AssignStmt); ok && len(assign.Lhs) == 1 {
					if index, ok := assign.Lhs[0].(*ast.IndexExpr); ok {
						if ident, ok := index.X.(*ast.Ident); ok && ident.Name == "testHelperCommands" {
							return false
						}
					}
				}
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				ident, ok := call.Fun.(*ast.Ident)
				// A local of the same name (ident.Obj set) is not the function.
				if !ok || ident.Obj != nil || (ident.Name != "dispatch" && ident.Name != "dispatchProcess") {
					return true
				}
				t.Errorf("%s:%d %s runs %s in this test process, which releases the process scratch under every other test; run the command through dispatchWithFamilies (dispatchOn)",
					relative, fileSet.Position(call.Pos()).Line, function.Name.Name, ident.Name)
				return true
			})
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk the Go tree: %v", err)
	}
}
