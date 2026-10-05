package config

import (
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

var updateReadSettings = flag.Bool("update-read-settings", false, "regenerate the configuration reader registry")

// readSettingRegistry follows key arguments through configuration getters,
// their wrappers, constants and local expressions. It reads Go source only.
func readSettingRegistry(t *testing.T) (keys, families []string) {
	t.Helper()
	files := map[string]*ast.File{}
	values := map[string]ast.Expr{}
	fset := token.NewFileSet()
	err := filepath.WalkDir("../..", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && (entry.Name() == "testdata" || strings.HasPrefix(entry.Name(), ".") && path != "../.." || entry.Name() == "node_modules") {
			return filepath.SkipDir
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") || strings.HasSuffix(path, "read_settings_generated.go") {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		files[path] = file
		for _, decl := range file.Decls {
			if decl, ok := decl.(*ast.GenDecl); ok {
				for _, spec := range decl.Specs {
					if spec, ok := spec.(*ast.ValueSpec); ok {
						for i, name := range spec.Names {
							if i < len(spec.Values) {
								values[filepath.Dir(path)+"/"+name.Name] = spec.Values[i]
							}
						}
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	name := func(expr ast.Expr) string {
		switch expr := expr.(type) {
		case *ast.Ident:
			return expr.Name
		case *ast.SelectorExpr:
			return expr.Sel.Name
		}
		return ""
	}
	qualified := func(path string, file *ast.File, expr ast.Expr) string {
		pkg := strings.TrimPrefix(filepath.Dir(path), "../../")
		switch expr := expr.(type) {
		case *ast.Ident:
			if expr.Obj != nil {
				if _, local := expr.Obj.Decl.(*ast.AssignStmt); local {
					return fmt.Sprintf("%s:%d:%s", path, expr.Obj.Pos(), expr.Name)
				}
			}
			return pkg + ":" + expr.Name
		case *ast.SelectorExpr:
			for _, imp := range file.Imports {
				importPath, _ := strconv.Unquote(imp.Path.Value)
				alias := filepath.Base(importPath)
				if imp.Name != nil {
					alias = imp.Name.Name
				}
				if name(expr.X) == alias {
					return strings.TrimPrefix(importPath, "github.com/widoriezebos/agentic-tools/metasystem/") + ":" + expr.Sel.Name
				}
			}
			return pkg + ":" + expr.Sel.Name
		}
		return ""
	}
	paramsKey := func(path string, file *ast.File, node ast.Node) ast.Expr {
		if row, ok := node.(*ast.CompositeLit); ok && qualified(path, file, row.Type) == "internal/config:GetParams" {
			for _, elem := range row.Elts {
				if elem, ok := elem.(*ast.KeyValueExpr); ok && name(elem.Key) == "Key" {
					return elem.Value
				}
			}
		}
		return nil
	}
	readers := map[string]int{"internal/config:ConfValue": 1, "internal/config:ConfLookup": 1, "internal/config:CommittedLookup": 1}
	// Wrapper discovery is a fixed point, so a wrapper of another wrapper
	// has exactly the same declaration coverage as a direct getter.
	for changed := true; changed; {
		changed = false
		for path, file := range files {
			ast.Inspect(file, func(node ast.Node) bool {
				var function string
				var typ *ast.FuncType
				var body *ast.BlockStmt
				switch node := node.(type) {
				case *ast.FuncDecl:
					function, typ, body = qualified(path, file, node.Name), node.Type, node.Body
				case *ast.AssignStmt:
					if len(node.Rhs) == 1 && len(node.Lhs) == 1 {
						if fn, ok := node.Rhs[0].(*ast.FuncLit); ok {
							function, typ, body = qualified(path, file, node.Lhs[0]), fn.Type, fn.Body
						}
					}
				}
				if typ == nil || body == nil {
					return true
				}
				position := 0
				for _, field := range typ.Params.List {
					for _, parameter := range field.Names {
						index := position
						position++
						ast.Inspect(body, func(node ast.Node) bool {
							arg := paramsKey(path, file, node)
							switch node := node.(type) {
							case *ast.IndexExpr:
								if strings.HasSuffix(path, "internal/config/validate.go") && (name(node.X) == "values" || name(node.X) == "localValues") {
									arg = node.Index
								}
							case *ast.CallExpr:
								if i, ok := readers[qualified(path, file, node.Fun)]; ok && i < len(node.Args) {
									arg = node.Args[i]
								}
							}
							if name(arg) == parameter.Name {
								if _, ok := readers[function]; !ok {
									readers[function], changed = index, true
								}
							}
							return true
						})
					}
				}
				return true
			})
		}
	}
	valid := regexp.MustCompile(`^[a-z0-9*()|#-]+(?:\.[a-z0-9*()|#-]+)*$`)
	for path, file := range files {
		finite := map[token.Pos]*ast.CompositeLit{}
		ast.Inspect(file, func(node ast.Node) bool {
			if loop, ok := node.(*ast.RangeStmt); ok {
				if ident, ok := loop.Value.(*ast.Ident); ok && ident.Obj != nil {
					if list, ok := loop.X.(*ast.CompositeLit); ok {
						finite[ident.Obj.Pos()] = list
					}
				}
			}
			return true
		})
		var expand func(ast.Expr, int) string
		expand = func(expr ast.Expr, depth int) string {
			if depth > 20 {
				return "*"
			}
			switch expr := expr.(type) {
			case *ast.BasicLit:
				if expr.Kind == token.STRING {
					text, _ := strconv.Unquote(expr.Value)
					return text
				}
			case *ast.BinaryExpr:
				if expr.Op == token.ADD {
					return expand(expr.X, depth+1) + expand(expr.Y, depth+1)
				}
			case *ast.Ident:
				if expr.Obj != nil {
					if list := finite[expr.Obj.Pos()]; list != nil {
						var choices []string
						for _, elem := range list.Elts {
							choices = append(choices, expand(elem, depth+1))
						}
						return "(" + strings.Join(choices, "|") + ")"
					}
				}
				if expr.Obj != nil {
					switch decl := expr.Obj.Decl.(type) {
					case *ast.ValueSpec:
						for i, ident := range decl.Names {
							if ident.Name == expr.Name && i < len(decl.Values) {
								return expand(decl.Values[i], depth+1)
							}
						}
					case *ast.RangeStmt:
						if list, ok := decl.X.(*ast.CompositeLit); ok {
							var choices []string
							for _, elem := range list.Elts {
								choices = append(choices, expand(elem, depth+1))
							}
							return "(" + strings.Join(choices, "|") + ")"
						}
					case *ast.AssignStmt:
						for i, lhs := range decl.Lhs {
							if name(lhs) == expr.Name && i < len(decl.Rhs) {
								return expand(decl.Rhs[i], depth+1)
							}
						}
					}
				}
				if value := values[filepath.Dir(path)+"/"+expr.Name]; value != nil {
					return expand(value, depth+1)
				}
				if expr.Name == "key" {
					return "@"
				}
			case *ast.SelectorExpr:
				if expr.Sel.Name == "Key" {
					return "@"
				}
				for _, imp := range file.Imports {
					importPath, _ := strconv.Unquote(imp.Path.Value)
					alias := filepath.Base(importPath)
					if imp.Name != nil {
						alias = imp.Name.Name
					}
					if name(expr.X) == alias {
						local := strings.TrimPrefix(importPath, "github.com/widoriezebos/agentic-tools/metasystem/")
						if value := values[filepath.Join("../..", local)+"/"+expr.Sel.Name]; value != nil {
							return expand(value, depth+1)
						}
					}
				}
			case *ast.CallExpr:
				called := qualified(path, file, expr.Fun)
				for source, sourceFile := range files {
					for _, decl := range sourceFile.Decls {
						if fn, ok := decl.(*ast.FuncDecl); ok && qualified(source, sourceFile, fn.Name) == called && fn.Body != nil && len(fn.Body.List) == 1 {
							if result, ok := fn.Body.List[0].(*ast.ReturnStmt); ok && len(result.Results) == 1 {
								if _, joined := result.Results[0].(*ast.BinaryExpr); joined {
									return expand(result.Results[0], depth+1)
								}
							}
						}
					}
				}
				if name(expr.Fun) == "Sprintf" && len(expr.Args) > 0 {
					text := expand(expr.Args[0], depth+1)
					for _, arg := range expr.Args[1:] {
						for _, verb := range []string{"%s", "%d"} {
							if strings.Contains(text, verb) {
								replacement := expand(arg, depth+1)
								if verb == "%d" && replacement == "*" {
									replacement = "#"
								}
								text = strings.Replace(text, verb, replacement, 1)
								break
							}
						}
					}
					return text
				}
			}
			return "*"
		}
		add := func(expr ast.Expr) {
			key := expand(expr, 0)
			if !valid.MatchString(key) || !strings.ContainsAny(key, "abcdefghijklmnopqrstuvwxyz") {
				return
			}
			if strings.ContainsAny(key, "*|#") {
				pattern := strings.ReplaceAll(regexp.QuoteMeta(key), `\*`, `[a-z0-9-]+`)
				pattern = strings.ReplaceAll(pattern, "#", "[0-9]+")
				for _, symbol := range []string{"(", ")", "|"} {
					pattern = strings.ReplaceAll(pattern, `\`+symbol, symbol)
				}
				families = append(families, "^"+pattern+"$")
			} else {
				keys = append(keys, key)
			}
		}
		for _, decl := range file.Decls {
			ast.Inspect(decl, func(node ast.Node) bool {
				arg := paramsKey(path, file, node)
				if call, ok := node.(*ast.CallExpr); ok {
					if index, ok := readers[qualified(path, file, call.Fun)]; ok && index < len(call.Args) {
						arg = call.Args[index]
					}
				}
				if arg == nil {
					return true
				}
				add(arg)
				if ident, ok := arg.(*ast.Ident); ok && ident.Obj != nil {
					ast.Inspect(decl, func(node ast.Node) bool {
						if assignment, ok := node.(*ast.AssignStmt); ok {
							for i, target := range assignment.Lhs {
								if target, ok := target.(*ast.Ident); ok && target.Obj == ident.Obj && i < len(assignment.Rhs) {
									add(assignment.Rhs[i])
								}
							}
						}
						return true
					})
				}
				if field, ok := arg.(*ast.SelectorExpr); ok && expand(arg, 0) == "*" {
					ast.Inspect(decl, func(node ast.Node) bool {
						row, ok := node.(*ast.CompositeLit)
						if !ok {
							return true
						}
						typ, rows := row.Type, []ast.Expr{row}
						if array, ok := typ.(*ast.ArrayType); ok {
							typ, rows = array.Elt, row.Elts
						}
						if typ, ok := typ.(*ast.StructType); ok {
							index := 0
							for _, column := range typ.Fields.List {
								for _, ident := range column.Names {
									if ident.Name == field.Sel.Name {
										for _, row := range rows {
											if row, ok := row.(*ast.CompositeLit); ok && index < len(row.Elts) {
												add(row.Elts[index])
											}
										}
									}
									index++
								}
							}
						}
						return true
					})
				}
				return true
			})
		}
	}
	slices.Sort(keys)
	keys = slices.Compact(keys)
	slices.Sort(families)
	families = slices.Compact(families)
	return keys, families
}

func TestEveryReadSettingIsDeclared(t *testing.T) {
	t.Parallel()
	keys, families := readSettingRegistry(t)
	if *updateReadSettings {
		var source strings.Builder
		source.WriteString("// Code generated by go generate; DO NOT EDIT.\npackage config\n\nimport \"regexp\"\n\nvar readSettingKeys = []string{\n")
		for _, key := range keys {
			if _, compiled := compiledSetting(key); !compiled {
				fmt.Fprintf(&source, "%q,\n", key)
			}
		}
		source.WriteString("}\n\nvar readSettingFamilies = []*regexp.Regexp{\n")
		for _, family := range families {
			fmt.Fprintf(&source, "regexp.MustCompile(%q),\n", family)
		}
		source.WriteString("}\n")
		formatted, err := format.Source([]byte(source.String()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile("read_settings_generated.go", formatted, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	var expected []string
	for _, key := range keys {
		if _, compiled := compiledSetting(key); !compiled {
			expected = append(expected, key)
		}
		if err := SettingKeyProblem(key); err != nil {
			t.Errorf("engine reads %s: %v; run go generate ./internal/config", key, err)
		}
	}
	if !slices.Equal(expected, readSettingKeys) {
		t.Errorf("reader registry differs from Go readers: want %v, got %v; run go generate ./internal/config", expected, readSettingKeys)
	}
	if len(families) != len(readSettingFamilies) {
		t.Errorf("reader family count = %d, want %d", len(readSettingFamilies), len(families))
	}
	for _, family := range families {
		found := false
		for _, declared := range readSettingFamilies {
			found = found || declared.String() == family
		}
		if !found {
			t.Errorf("engine reads family %s; run go generate ./internal/config", family)
		}
	}
}
