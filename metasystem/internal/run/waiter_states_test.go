package run

import (
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestWaiterStatesMatchTheWriter(t *testing.T) {
	listed := map[string]bool{}
	for _, state := range WaiterStates {
		listed[state.Name] = true
	}
	constants := map[string]string{"WaiterStateRegistering": WaiterStateRegistering, "WaiterStatePending": WaiterStatePending, "WaiterStateReady": WaiterStateReady, "WaiterStateFailed": WaiterStateFailed, "WaiterStateInterrupted": WaiterStateInterrupted, "WaiterStateDeadline": WaiterStateDeadline}
	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	var files []*ast.File
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, file)
	}
	recorded, forwarded := map[string]bool{}, 0
	record := func(expr ast.Expr, allowForward bool) {
		if id, ok := expr.(*ast.Ident); ok {
			if allowForward && id.Name == "state" {
				forwarded++
				return
			}
			if value, ok := constants[id.Name]; ok {
				recorded[value] = true
				return
			}
		}
		literal, ok := expr.(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING {
			t.Fatalf("%s: waiter state is computed instead of a literal or listed constant", fset.Position(expr.Pos()))
		}
		value, _ := strconv.Unquote(literal.Value)
		recorded[value] = true
	}
	for _, file := range files {
		ast.Inspect(file, func(node ast.Node) bool {
			switch value := node.(type) {
			case *ast.CallExpr:
				if id, ok := value.Fun.(*ast.Ident); ok && id.Name == "waitResult" {
					record(value.Args[2], true)
				}
				if selector, ok := value.Fun.(*ast.SelectorExpr); ok && selector.Sel.Name == "finishV2" {
					record(value.Args[5], false)
				}
			case *ast.AssignStmt:
				for index, left := range value.Lhs {
					if selector, ok := left.(*ast.SelectorExpr); ok && selector.Sel.Name == "State" {
						record(value.Rhs[index], true)
					}
				}
			case *ast.CompositeLit:
				name, ok := value.Type.(*ast.Ident)
				if !ok || name.Name != "Waiter" {
					break
				}
				for _, element := range value.Elts {
					field, ok := element.(*ast.KeyValueExpr)
					if !ok {
						continue
					}
					key, keyOK := field.Key.(*ast.Ident)
					if keyOK && key.Name == "State" {
						record(field.Value, false)
					}
				}
			}
			return true
		})
	}
	if forwarded != 2 || !maps.Equal(recorded, listed) {
		t.Fatalf("writer states %v and %d checked forwards do not match listed states %v and 2 forwards", recorded, forwarded, listed)
	}
}
