package run

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

type wallTimeException struct {
	function string
	reason   string
}

type wallTimeCallVisitor struct {
	t          *testing.T
	fset       *token.FileSet
	function   string
	exceptions map[string]wallTimeException
	seen       map[string]int
}

func (visitor *wallTimeCallVisitor) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		return nil
	}
	if declaration, ok := node.(*ast.FuncDecl); ok {
		child := *visitor
		child.function = wallTimeFunctionName(declaration)
		return &child
	}
	call, ok := node.(*ast.CallExpr)
	if !ok {
		return visitor
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return visitor
	}
	owner, ok := selector.X.(*ast.Ident)
	if !ok || owner.Name != "time" || !forbiddenWallTimeCall(selector.Sel.Name) {
		return visitor
	}
	if _, allowed := visitor.exceptions[visitor.function]; allowed {
		visitor.seen[visitor.function]++
		return visitor
	}
	position := visitor.fset.Position(call.Pos())
	visitor.t.Errorf("%s: forbidden wall-time call time.%s in %s", position, selector.Sel.Name, visitor.function)
	return visitor
}

func forbiddenWallTimeCall(name string) bool {
	switch name {
	case "Sleep", "After", "NewTimer", "NewTicker", "Tick":
		return true
	default:
		return false
	}
}

func wallTimeFunctionName(declaration *ast.FuncDecl) string {
	if declaration.Recv == nil {
		return declaration.Name.Name
	}
	receiver := declaration.Recv.List[0].Type
	if pointer, ok := receiver.(*ast.StarExpr); ok {
		if name, ok := pointer.X.(*ast.Ident); ok {
			return fmt.Sprintf("(*%s).%s", name.Name, declaration.Name.Name)
		}
	}
	if name, ok := receiver.(*ast.Ident); ok {
		return fmt.Sprintf("(%s).%s", name.Name, declaration.Name.Name)
	}
	return declaration.Name.Name
}

func TestWaiterSourcesUseNoWallTime(t *testing.T) {
	exceptionList := []wallTimeException{
		{function: "defaultWaitSleep", reason: "the injected sleep seam needs a real-time default"},
		{function: "withWaiterLock", reason: "the file-lock retry has its own five-second bound"},
	}
	exceptions := make(map[string]wallTimeException, len(exceptionList))
	for _, exception := range exceptionList {
		exceptions[exception.function] = exception
	}
	seen := make(map[string]int, len(exceptionList))

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	parsedFiles := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") ||
			(!strings.HasPrefix(name, "waiter") && !strings.HasPrefix(name, "wait_hint")) {
			continue
		}
		fset := token.NewFileSet()
		parsed, parseErr := parser.ParseFile(fset, name, nil, 0)
		if parseErr != nil {
			t.Errorf("parse %s: %v", name, parseErr)
			continue
		}
		parsedFiles++
		ast.Walk(&wallTimeCallVisitor{
			t: t, fset: fset, function: "package scope",
			exceptions: exceptions, seen: seen,
		}, parsed)
	}
	if parsedFiles == 0 {
		t.Fatal("no waiter source files were parsed")
	}
	for _, exception := range exceptionList {
		if seen[exception.function] == 0 {
			t.Errorf("wall-time exception %s is stale: %s; it contains no forbidden call", exception.function, exception.reason)
		}
	}
}
