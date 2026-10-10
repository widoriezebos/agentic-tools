package goadapter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"maps"
	"os"
	"os/exec"
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

// TestPackagePaths names each test-bearing package relative to its module.
func TestPackagePaths(moduleRoot string, buildTags []string) ([]string, error) {
	command := exec.Command("go", "list", "-tags", strings.Join(buildTags, ","), "-f", "{{if or .TestGoFiles .XTestGoFiles}}{{.Dir}}{{end}}", "./...")
	command.Dir = moduleRoot
	output, err := command.Output()
	if err != nil {
		return nil, err
	}
	var packages []string
	for _, dir := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if dir == "" {
			continue
		}
		pkg, err := filepath.Rel(moduleRoot, dir)
		if err != nil {
			return nil, err
		}
		packages = append(packages, filepath.ToSlash(pkg))
	}
	return packages, nil
}

// UnitImpact selects tests from the current working snapshot against the unit's base.
func UnitImpact(moduleRoot, base string) (impact Impact, err error) {
	workspace := gittree.Workspace{Dir: moduleRoot}
	commit, err := workspace.ResolveCommit(base)
	if err != nil {
		return impact, fmt.Errorf("unit check base %q is not a readable commit", base)
	}
	selection, err := SelectWorkingUnitPackages(moduleRoot, commit)
	if err != nil {
		return impact, err
	}
	base, impact.Paths = selection.base, selection.paths
	files, imports, module := selection.files, selection.imports, selection.ModulePath
	if err := impactPackageImports(moduleRoot, module, imports); err != nil {
		return impact, err
	}
	for pkg := range imports {
		if strings.Contains(pkg+"/", "/testdata/") {
			delete(imports, pkg)
		}
	}
	changed, symbols := map[string]bool{}, map[string]bool{}
	full := slices.Contains(impact.Paths, "go.mod") || slices.Contains(impact.Paths, "go.sum")
	for _, path := range impact.Paths {
		if strings.HasSuffix(path, "_test.go") && !strings.Contains("/"+path, "/testdata/") {
			if _, _, err := impactFile(workspace, base, path, true, symbols); err != nil {
				return impact, err
			}
		}
		if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") && !strings.Contains("/"+path, "/testdata/") {
			changed[relativePackage(module, pathpkg.Join(module, filepath.ToSlash(filepath.Dir(path))))] = true
			symbols["own/"+strings.TrimSuffix(path, ".go")+"_test.go"] = true
			if _, _, err := impactFile(workspace, base, path, true, symbols); err != nil {
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
		wholeFile := false
		for path := range files {
			if strings.TrimPrefix(pkg, "./") != filepath.ToSlash(filepath.Dir(path)) || !strings.HasSuffix(path, "_test.go") {
				continue
			}
			selected, whole, err := impactFile(workspace, base, path, slices.Contains(impact.Paths, path), symbols)
			if err != nil {
				return impact, err
			}
			maps.Copy(names, selected)
			wholeFile = wholeFile || whole
		}
		tests := slices.Sorted(maps.Keys(names))
		whole := full || wholeFile || strings.HasPrefix(pkg, "./internal/") && slices.Contains(dependents, pkg) || changed[pkg] && (strings.HasPrefix(pkg, "./internal/") || len(tests) == 0)
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
func impactFile(workspace gittree.Workspace, base, path string, changed bool, symbols map[string]bool) (map[string]bool, bool, error) {
	after, err := os.ReadFile(filepath.Join(workspace.Dir, path))
	if err != nil && !os.IsNotExist(err) {
		return nil, false, err
	}
	var before, diff []byte
	if changed {
		before, _, err = workspace.FileAt(base, path)
		if err == nil {
			diff, err = workspace.DiffBlobs(before, after)
		}
		if err != nil {
			return nil, false, err
		}
	}
	hunks := regexp.MustCompile(`(?m)^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`).FindAllStringSubmatch(string(diff), -1)
	tests, all := map[string]bool{}, map[string]bool{}
	isTest, referenced := strings.HasSuffix(path, "_test.go"), symbols["own/"+path]
	hasTestMain := false
	for side, data := range [][]byte{before, after} {
		if len(data) == 0 {
			continue
		}
		positions := token.NewFileSet()
		file, err := parser.ParseFile(positions, path, data, 0)
		if err != nil {
			return nil, false, err
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
				if isTest && node.Recv == nil && node.Name.Name == "TestMain" {
					hasTestMain = hasTestMain || side == 1
					return true
				}
				entrypoint := isTest && node.Recv == nil && testpolicy.GoTestName(node.Name.Name)
				if overlaps(node) && !entrypoint {
					symbols[node.Name.Name] = true
				} else if side == 1 && entrypoint {
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
	return tests, referenced && hasTestMain && len(all) == 0, nil
}

// impactPackageImports adds the Go tool's build and test dependencies to the module graph.
func impactPackageImports(root, module string, imports map[string]map[string]bool) error {
	command := exec.Command("go", "list", "-e", "-json", "./...")
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		return fmt.Errorf("unit check cannot discover Go dependencies: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(output))
	for {
		var pkg struct {
			ImportPath                      string
			Deps, TestImports, XTestImports []string
		}
		if err := decoder.Decode(&pkg); err == io.EOF {
			return nil
		} else if err != nil {
			return fmt.Errorf("unit check cannot read Go dependencies: %w", err)
		}
		local := relativePackage(module, pkg.ImportPath)
		if imports[local] == nil {
			continue
		}
		for _, dependencies := range [][]string{pkg.Deps, pkg.TestImports, pkg.XTestImports} {
			for _, imported := range dependencies {
				if relativePackage(module, imported) != "" {
					imports[local][imported] = true
				}
			}
		}
	}
}
