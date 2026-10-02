package proofrun

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"
)

// TestEveryReservationInTheProofrunTestsIsPrivate: this package's tests
// reserve only through reserveLocked, so no two parallel tests share one
// admission lock (flaky "host proof admission is busy").
func TestEveryReservationInTheProofrunTestsIsPrivate(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob("*_test.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range files {
		fileSet := token.NewFileSet()
		file, err := parser.ParseFile(fileSet, path, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range file.Decls {
			function, ok := decl.(*ast.FuncDecl)
			if !ok || function.Body == nil || function.Recv == nil && function.Name.Name == "reserveLocked" {
				continue
			}
			ast.Inspect(function.Body, func(node ast.Node) bool {
				if ident, ok := node.(*ast.Ident); ok && ident.Name == "ReserveLocked" {
					t.Errorf("%s:%d %s uses ReserveLocked; reserve through reserveLocked, which gives the request an admission directory of its own",
						path, fileSet.Position(ident.Pos()).Line, function.Name.Name)
				}
				return true
			})
		}
	}
}
