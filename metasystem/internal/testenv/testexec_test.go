package testenv

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// This scan cannot resolve modes stored in variables or connect writes and
// chmod calls made by different functions; those sites need manual review.
func TestTestExecutablesAreWrittenUnderTheForkLock(t *testing.T) {
	t.Parallel()

	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	problems, err := findUnlockedTestExecutableWrites(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, problem := range problems {
		t.Error(problem)
	}
}

func findUnlockedTestExecutableWrites(root string) ([]string, error) {
	var problems []string
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
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if strings.HasPrefix(relative, "internal/testexec/") || !testExecutableSource(relative) {
			return nil
		}

		fileSet := token.NewFileSet()
		file, err := parser.ParseFile(fileSet, path, nil, 0)
		if err != nil {
			return err
		}
		osAliases := importedAliases(file, "os", "os")
		fsAliases := importedAliases(file, "io/fs", "fs")
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Body == nil {
				continue
			}
			problems = append(problems, executableWriteProblems(relative, fileSet, function.Body, osAliases, fsAliases)...)
		}
		return nil
	})
	sort.Strings(problems)
	return problems, err
}

func testExecutableSource(path string) bool {
	if strings.HasSuffix(path, "_test.go") {
		return true
	}
	if !strings.HasSuffix(path, ".go") {
		return false
	}
	return strings.HasPrefix(path, "internal/testutil/") || strings.HasPrefix(path, "internal/testenv/")
}

func executableWriteProblems(path string, fileSet *token.FileSet, body *ast.BlockStmt, osAliases, fsAliases map[string]bool) []string {
	var problems []string
	written := make(map[string]bool)
	var executableChmods []*ast.CallExpr
	ast.Inspect(body, func(node ast.Node) bool {
		if literal, ok := node.(*ast.FuncLit); ok {
			problems = append(problems, executableWriteProblems(path, fileSet, literal.Body, osAliases, fsAliases)...)
			return false
		}
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		name, ok := osCallName(call, osAliases)
		if !ok {
			return true
		}
		switch name {
		case "WriteFile", "OpenFile":
			if len(call.Args) >= 1 {
				written[printedExpression(fileSet, call.Args[0])] = true
			}
			if len(call.Args) >= 3 && executableMode(call.Args[2], osAliases, fsAliases) {
				position := fileSet.Position(call.Pos())
				problems = append(problems, fmt.Sprintf("%s:%d: os.%s writes an executable without testexec", path, position.Line, name))
			}
		case "Create":
			if len(call.Args) >= 1 {
				written[printedExpression(fileSet, call.Args[0])] = true
			}
		case "Chmod":
			if len(call.Args) >= 2 && executableMode(call.Args[1], osAliases, fsAliases) {
				executableChmods = append(executableChmods, call)
			}
		}
		return true
	})
	for _, call := range executableChmods {
		if !written[printedExpression(fileSet, call.Args[0])] {
			continue
		}
		position := fileSet.Position(call.Pos())
		problems = append(problems, fmt.Sprintf("%s:%d: os.Chmod makes an unlocked write executable", path, position.Line))
	}
	return problems
}

func osCallName(call *ast.CallExpr, aliases map[string]bool) (string, bool) {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	identifier, ok := selector.X.(*ast.Ident)
	if !ok || !aliases[identifier.Name] {
		return "", false
	}
	return selector.Sel.Name, true
}

func executableMode(expression ast.Expr, osAliases, fsAliases map[string]bool) bool {
	switch expression := expression.(type) {
	case *ast.BasicLit:
		if expression.Kind != token.INT {
			return false
		}
		value, err := strconv.ParseUint(strings.ReplaceAll(expression.Value, "_", ""), 0, 64)
		return err == nil && value&0o111 != 0
	case *ast.SelectorExpr:
		identifier, ok := expression.X.(*ast.Ident)
		return ok && expression.Sel.Name == "ModePerm" && (osAliases[identifier.Name] || fsAliases[identifier.Name])
	default:
		return false
	}
}

func printedExpression(fileSet *token.FileSet, expression ast.Expr) string {
	var output bytes.Buffer
	if err := format.Node(&output, fileSet, expression); err != nil {
		return ""
	}
	return output.String()
}
