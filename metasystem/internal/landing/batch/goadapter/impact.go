package goadapter

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"os"
	pathpkg "path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// Impact names the packages, tests and module-relative paths changed by a unit.
type Impact struct {
	Paths    []string
	Packages []testpolicy.Group
}

// UnitImpact selects tests from the current working snapshot against the unit's base.
func UnitImpact(moduleRoot, base, _ string) (impact Impact, err error) {
	workspace := gittree.Workspace{Dir: moduleRoot}
	selection, err := SelectWorkingUnitPackages(moduleRoot, base)
	if err != nil {
		return impact, err
	}
	base, impact.Paths = selection.base, selection.paths
	files, imports, module := selection.files, selection.imports, selection.ModulePath
	for pkg := range imports {
		if strings.Contains(pkg+"/", "/testdata/") {
			delete(imports, pkg)
		}
	}
	changed, symbols := map[string]bool{}, map[string]bool{}
	full := slices.Contains(impact.Paths, "go.mod") || slices.Contains(impact.Paths, "go.sum")
	for _, path := range impact.Paths {
		if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") && !strings.Contains("/"+path, "/testdata/") {
			changed[relativePackage(module, pathpkg.Join(module, filepath.ToSlash(filepath.Dir(path))))] = true
			symbols["own/"+strings.TrimSuffix(path, ".go")+"_test.go"] = true
			if _, err := impactFile(workspace, base, path, true, symbols); err != nil {
				return impact, err
			}
		}
		for dir := filepath.ToSlash(filepath.Dir(path)); ; dir = filepath.ToSlash(filepath.Dir(dir)) {
			pkg := "./" + dir
			if dir == "." {
				pkg = "."
			}
			if _, fixture, ok := strings.Cut("/"+dir, "/testdata/"); ok {
				symbols[dir], symbols["testdata/"+fixture], symbols[filepath.Base(dir)] = true, true, true
			}
			if imports[pkg] != nil {
				changed[pkg] = true
				break
			}
			if dir == "." {
				break
			}
		}
		symbols[path], symbols[filepath.Base(path)] = true, true
	}
	dependents := reverseDependents(module, slices.Collect(maps.Keys(changed)), imports)
	for _, pkg := range slices.Sorted(maps.Keys(imports)) {
		if !full && !changed[pkg] && !slices.Contains(dependents, pkg) {
			continue
		}
		names := map[string]bool{}
		for path := range files {
			if strings.TrimPrefix(pkg, "./") != filepath.ToSlash(filepath.Dir(path)) || !strings.HasSuffix(path, "_test.go") {
				continue
			}
			selected, err := impactFile(workspace, base, path, slices.Contains(impact.Paths, path), symbols)
			if err != nil {
				return impact, err
			}
			maps.Copy(names, selected)
		}
		tests := slices.Sorted(maps.Keys(names))
		whole := full || changed[pkg] && (strings.HasPrefix(pkg, "./internal/") || len(tests) == 0)
		if !whole && len(tests) == 0 {
			continue
		}
		encoded, _ := json.Marshal(tests)
		if whole {
			encoded = json.RawMessage(`"all"`)
		}
		impact.Packages = append(impact.Packages, testpolicy.Group{ID: "unit/" + strings.TrimPrefix(pkg, "./"), Adapter: "go", CWD: ".", Packages: []string{pkg}, Tests: encoded})
	}
	return impact, nil
}

// impactFile reads both sides so removed declarations and messages select their readers.
func impactFile(workspace gittree.Workspace, base, path string, changed bool, symbols map[string]bool) (map[string]bool, error) {
	after, err := os.ReadFile(filepath.Join(workspace.Dir, path))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	var before, diff []byte
	if changed {
		before, _, err = workspace.FileAt(base, path)
		if err == nil {
			diff, err = workspace.DiffBlobs(before, after)
		}
		if err != nil {
			return nil, err
		}
	}
	hunks := regexp.MustCompile(`(?m)^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`).FindAllStringSubmatch(string(diff), -1)
	tests, all := map[string]bool{}, map[string]bool{}
	isTest, referenced := strings.HasSuffix(path, "_test.go"), symbols["own/"+path]
	for side, data := range [][]byte{before, after} {
		if len(data) == 0 {
			continue
		}
		positions := token.NewFileSet()
		file, err := parser.ParseFile(positions, path, data, 0)
		if err != nil {
			return nil, err
		}
		overlaps := func(node ast.Node) bool {
			first, last := positions.Position(node.Pos()).Line, positions.Position(node.End()).Line
			for _, hunk := range hunks {
				start, _ := strconv.Atoi(hunk[1+2*side])
				count := 1
				if hunk[2+2*side] != "" {
					count, _ = strconv.Atoi(hunk[2+2*side])
				}
				if count > 0 && first <= start+count-1 && last >= start {
					return true
				}
			}
			return false
		}
		for name, object := range file.Scope.Objects {
			if node, ok := object.Decl.(ast.Node); ok && !isTest && overlaps(node) {
				symbols[name] = true
			}
		}
		ast.Inspect(file, func(node ast.Node) bool {
			switch node := node.(type) {
			case *ast.FuncDecl:
				if !isTest && overlaps(node) {
					symbols[node.Name.Name] = true
				} else if isTest && side == 1 && node.Recv == nil && strings.HasPrefix(node.Name.Name, "Test") {
					all[node.Name.Name] = true
					if overlaps(node) {
						tests[node.Name.Name] = true
					}
				}
			case *ast.Ident:
				referenced = referenced || isTest && side == 1 && symbols[node.Name]
			case *ast.BasicLit:
				if node.Kind == token.STRING {
					value, _ := strconv.Unquote(node.Value)
					referenced = referenced || isTest && side == 1 && symbols[value]
					if !isTest && overlaps(node) && len([]rune(value)) >= 3 {
						symbols[value] = true
					}
				}
			}
			return true
		})
	}
	if referenced {
		tests = all
	}
	return tests, nil
}
