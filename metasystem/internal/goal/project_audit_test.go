package goal

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"testing"
)

func TestAuditFreshProjectionsUseInjectedDeadline(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..")
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "artifacts", "node_modules", "vendor", "testdata":
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}
		set := token.NewFileSet()
		file, err := parser.ParseFile(set, path, nil, 0)
		if err != nil {
			return err
		}
		goalNames := map[string]bool{}
		for _, imported := range file.Imports {
			value, _ := strconv.Unquote(imported.Path.Value)
			if value == "github.com/widoriezebos/agentic-tools/metasystem/internal/goal" {
				name := "goal"
				if imported.Name != nil {
					name = imported.Name.Name
				}
				goalNames[name] = true
			}
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || len(call.Args) != 3 {
				return true
			}
			fresh, ok := call.Args[1].(*ast.Ident)
			if ok && fresh.Name == "false" {
				return true
			}
			direct := false
			switch name := call.Fun.(type) {
			case *ast.SelectorExpr:
				pkg, ok := name.X.(*ast.Ident)
				direct = ok && goalNames[pkg.Name] && name.Sel.Name == "Project"
			case *ast.Ident:
				direct = file.Name.Name == "goal" && name.Name == "Project"
			}
			if direct {
				t.Errorf("%s: fresh projection must use ProjectWithDeadline", set.Position(call.Pos()))
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
