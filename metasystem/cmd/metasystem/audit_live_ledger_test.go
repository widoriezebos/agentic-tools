package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func liveLedgerPageSites(root string) ([]string, error) {
	page := regexp.MustCompile(`(^|/)(plans|records)/goals/[^/*?\[]+\.md$`)
	var sites []string
	literal := func(expression ast.Expr) (string, bool) {
		value, ok := expression.(*ast.BasicLit)
		if !ok || value.Kind != token.STRING {
			return "", false
		}
		text, err := strconv.Unquote(value.Value)
		return text, err == nil
	}
	for _, directory := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(filepath.Join(root, directory), func(filePath string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			name := entry.Name()
			if entry.IsDir() {
				if name == "testdata" || name == "artifacts" || name == "vendor" || name == "node_modules" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(name, "_test.go") {
				return nil
			}
			relative, err := filepath.Rel(root, filePath)
			if err != nil {
				return err
			}
			positions := token.NewFileSet()
			file, err := parser.ParseFile(positions, filePath, nil, parser.SkipObjectResolution)
			if err != nil {
				return err
			}
			record := func(node ast.Node, value string) {
				value = path.Clean(value)
				if page.MatchString(value) {
					sites = append(sites, fmt.Sprintf("%s:%d: %s", filepath.ToSlash(relative), positions.Position(node.Pos()).Line, value))
				}
			}
			var stack []ast.Node
			resolve := func(expression ast.Expr) []string {
				if value, ok := literal(expression); ok {
					return []string{value}
				}
				identifier, ok := expression.(*ast.Ident)
				if !ok {
					return nil
				}
				for i := len(stack) - 1; i >= 0; i-- {
					loop, ok := stack[i].(*ast.RangeStmt)
					if !ok {
						continue
					}
					variable, ok := loop.Value.(*ast.Ident)
					if !ok || variable.Name != identifier.Name {
						continue
					}
					key, keyOK := loop.Key.(*ast.Ident)
					values, valuesOK := loop.X.(*ast.CompositeLit)
					if loop.Tok != token.DEFINE || !keyOK || key.Name != "_" || !valuesOK {
						return nil
					}
					array, ok := values.Type.(*ast.ArrayType)
					if !ok || array.Len != nil {
						return nil
					}
					elementType, ok := array.Elt.(*ast.Ident)
					if !ok || elementType.Name != "string" {
						return nil
					}
					var result []string
					for _, element := range values.Elts {
						value, ok := literal(element)
						if !ok {
							return nil
						}
						result = append(result, value)
					}
					return result
				}
				return nil
			}
			ast.Inspect(file, func(node ast.Node) bool {
				if node == nil {
					stack = stack[:len(stack)-1]
					return true
				}
				stack = append(stack, node)
				if expression, ok := node.(*ast.BasicLit); ok {
					if value, ok := literal(expression); ok && strings.HasPrefix(value, "../") {
						record(node, value)
					}
				}
				call, ok := node.(*ast.CallExpr)
				if !ok || len(call.Args) == 0 {
					return true
				}
				function, ok := call.Fun.(*ast.SelectorExpr)
				first, _ := literal(call.Args[0])
				if !ok || function.Sel.Name != "Join" || first != ".." {
					return true
				}
				joined := []string{""}
				for _, argument := range call.Args {
					var next []string
					for _, prefix := range joined {
						for _, value := range resolve(argument) {
							next = append(next, path.Join(prefix, value))
						}
					}
					joined = next
				}
				for _, value := range joined {
					record(call, value)
				}
				return true
			})
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	slices.Sort(sites)
	return sites, nil
}

func TestAuditNoTestReadsANamedLiveLedgerPage(t *testing.T) {
	t.Parallel()
	sites, err := liveLedgerPageSites("../..")
	if err != nil {
		t.Fatal(err)
	}
	for _, site := range sites {
		t.Errorf("%s: a test reads a named page of the live ledger, which moves when its goal concludes; write the page into the test's own bed or read it from a testdata fixture", site)
	}
}

func TestAuditLiveLedgerPageSeesEveryShape(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	sources := map[string]string{
		"cmd/x/a_test.go": `package fixture
func fixture() {
 for _, path := range []string{"go.mod", "plans/goals/live.md"} { read(filepath.Join("..", "..", path)) }
 _ = filepath.Join("..", "..", "records", "goals", "done.md")
 _ = "../../plans/goals/other.md"
 _ = filepath.Join(root, "plans/goals/bed.md")
 _ = filepath.Join("..", "..", "plans", "goals", "*.md")
 _ = filepath.Join("..", "..", "plans")
 _ = filepath.Join("..", "..", "internal", "x", "testdata", "main-move", "backlog.md")
 _ = filepath.Join("..", "..", name())
 for _, path := range []string{"plans/goals/outer.md"} {
  for _, path := range []string{"go.mod"} { read(filepath.Join("..", "..", path)) }
 }
}
`,
		"internal/b_test.go":       "package fixture\nvar page = `../../records/goals/internal.md`\n",
		"cmd/x/prod.go":            "package fixture\nvar page = `../../plans/goals/prod.md`\n",
		"tools/c_test.go":          "package fixture\nvar page = `../../plans/goals/tool.md`\n",
		"cmd/x/testdata/d_test.go": "package fixture\nvar page = `../../plans/goals/data.md`\n",
	}
	for filePath, source := range sources {
		filePath = filepath.Join(root, filePath)
		if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filePath, []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	sites, err := liveLedgerPageSites(root)
	want := []string{"cmd/x/a_test.go:3: ../../plans/goals/live.md", "cmd/x/a_test.go:4: ../../records/goals/done.md", "cmd/x/a_test.go:5: ../../plans/goals/other.md", "internal/b_test.go:2: ../../records/goals/internal.md"}
	if err != nil || !reflect.DeepEqual(sites, want) {
		t.Fatalf("live ledger sites=%v want=%v err=%v", sites, want, err)
	}
}
