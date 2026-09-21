package batch

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gopackages"
)

// UnitPackages is the package closure that one stacked unit must gate.
type UnitPackages struct {
	Tree       string
	ModulePath string
	Changed    []string
	Dependents []string
}

func unitGateModuleRoot(root string) string {
	if root == "" {
		return ""
	}
	if info, err := os.Stat(filepath.Join(root, "go.mod")); err == nil && !info.IsDir() {
		return root
	}
	nested := filepath.Join(root, "metasystem")
	if info, err := os.Stat(filepath.Join(nested, "go.mod")); err == nil && !info.IsDir() {
		return nested
	}
	return ""
}

// SelectUnitPackages computes the changed packages and their transitive
// reverse dependencies against one exact candidate tree.
func SelectUnitPackages(moduleRoot, base, tree string) (UnitPackages, error) {
	selected, err := gopackages.Select(moduleRoot, base, tree)
	if err != nil {
		return UnitPackages{}, err
	}
	return UnitPackages{Tree: selected.Tree, ModulePath: selected.ModulePath,
		Changed: selected.Changed, Dependents: selected.Dependents}, nil
}

// SelectWorkingUnitPackages compares base with the current module worktree.
// It includes untracked files without staging them or writing Git objects.
func SelectWorkingUnitPackages(moduleRoot, base string) (UnitPackages, error) {
	workspace := gittree.Workspace{Dir: moduleRoot}
	baseTree, err := moduleTree(workspace, base)
	if err != nil {
		return UnitPackages{}, fmt.Errorf("unit gate base: %w", err)
	}
	paths, err := workingChangedPaths(moduleRoot, baseTree)
	if err != nil {
		return UnitPackages{}, err
	}
	changes := gateChanges{}
	for _, path := range paths {
		_, basePresent, readErr := workspace.FileAt(baseTree, path)
		if readErr != nil {
			return UnitPackages{}, readErr
		}
		_, statErr := os.Stat(filepath.Join(moduleRoot, filepath.FromSlash(path)))
		changes[path] = gateChange{Deleted: os.IsNotExist(statErr), BaseAbsent: !basePresent, baseKnown: true}
		if statErr != nil && !os.IsNotExist(statErr) {
			return UnitPackages{}, statErr
		}
	}
	changed, err := changedWorkingGoPackages(moduleRoot, changes)
	if err != nil {
		return UnitPackages{}, err
	}
	goMod, err := os.ReadFile(filepath.Join(moduleRoot, "go.mod"))
	if err != nil {
		return UnitPackages{}, err
	}
	module, err := modulePath(goMod)
	if err != nil {
		return UnitPackages{}, err
	}
	goFiles, err := workingGoFiles(moduleRoot)
	if err != nil {
		return UnitPackages{}, err
	}
	imports, err := packageImports(moduleRoot, module, goFiles)
	if err != nil {
		return UnitPackages{}, err
	}
	return UnitPackages{
		ModulePath: module,
		Changed:    changed,
		Dependents: reverseDependents(module, changed, imports),
	}, nil
}

func workingGoFiles(moduleRoot string) (map[string]bool, error) {
	command := exec.Command("git", "-C", moduleRoot, "ls-files", "--cached", "--others", "--exclude-standard", "-z", "--", "*.go")
	command.Env = gittree.ScrubbedEnviron()
	output, err := command.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("unit gate git ls-files: %s: %w", strings.TrimSpace(string(output)), err)
	}
	files := map[string]bool{}
	for _, path := range strings.Split(string(output), "\x00") {
		if path != "" {
			files[filepath.ToSlash(path)] = true
		}
	}
	return files, nil
}

func workingChangedPaths(moduleRoot, baseTree string) ([]string, error) {
	set := map[string]bool{}
	workspace := gittree.Workspace{Dir: moduleRoot}
	headTree, err := workspace.TreeOf("HEAD")
	if err != nil {
		return nil, err
	}
	committed, err := workspace.ChangedPaths(baseTree, headTree)
	if err != nil {
		return nil, err
	}
	for _, path := range committed {
		set[filepath.ToSlash(path)] = true
	}
	commands := [][]string{
		{"diff", "--name-only", "-z", "--no-renames", "--no-ext-diff", "--no-textconv", "--ignore-submodules=none", "--relative", "HEAD", "--"},
		{"ls-files", "--others", "--exclude-standard", "-z", "--"},
	}
	for _, arguments := range commands {
		command := exec.Command("git", append([]string{"-C", moduleRoot}, arguments...)...)
		command.Env = gittree.ScrubbedEnviron()
		output, err := command.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("unit gate git %s: %s: %w", arguments[0], strings.TrimSpace(string(output)), err)
		}
		for _, path := range strings.Split(string(output), "\x00") {
			if path != "" {
				set[filepath.ToSlash(path)] = true
			}
		}
	}
	paths := make([]string, 0, len(set))
	for path := range set {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths, nil
}

func changedWorkingGoPackages(moduleRoot string, changes gateChanges) ([]string, error) {
	set := map[string]bool{}
	for changed, change := range changes {
		if !strings.HasSuffix(changed, ".go") {
			continue
		}
		dir := filepath.ToSlash(filepath.Dir(changed))
		pkg := "."
		if dir != "." {
			pkg = "./" + dir
		}
		if change.Deleted {
			if change.BaseAbsent {
				continue
			}
			parent := filepath.ToSlash(filepath.Dir(dir))
			for parent != "." {
				entries, err := os.ReadDir(filepath.Join(moduleRoot, filepath.FromSlash(parent)))
				if err == nil && len(entries) != 0 {
					break
				}
				if err != nil && !os.IsNotExist(err) {
					return nil, err
				}
				parent = filepath.ToSlash(filepath.Dir(parent))
			}
			pkg = "./..."
			if parent != "." {
				pkg = "./" + parent + "/..."
			}
		}
		set[pkg] = true
	}
	packages := make([]string, 0, len(set))
	for pkg := range set {
		packages = append(packages, pkg)
	}
	sort.Strings(packages)
	return packages, nil
}

func unitPackagesFromChanges(moduleRoot, tree string, changes gateChanges) (UnitPackages, error) {
	resolvedTree := tree
	var err error
	if moduleRoot != "" {
		resolvedTree, err = moduleTree(gittree.Workspace{Dir: moduleRoot}, tree)
		if err != nil {
			return UnitPackages{}, err
		}
	}
	changed, err := changedGoPackages(moduleRoot, resolvedTree, changes)
	if err != nil {
		return UnitPackages{}, err
	}
	selection := UnitPackages{Tree: resolvedTree, Changed: changed}
	if moduleRoot == "" || len(changed) == 0 {
		return selection, nil
	}
	selection.Dependents, err = ReverseDependents(moduleRoot, resolvedTree, changed)
	if err != nil {
		return UnitPackages{}, err
	}
	data, present, err := (gittree.Workspace{Dir: moduleRoot}).FileAt(resolvedTree, "go.mod")
	if err != nil {
		return UnitPackages{}, err
	}
	if !present {
		return UnitPackages{}, fmt.Errorf("unit gate tree %s has no go.mod", resolvedTree)
	}
	selection.ModulePath, err = modulePath(data)
	return selection, err
}

// ReverseDependents returns every package in moduleRoot that imports a
// changed package directly or through another package. Every Go file in the
// tree participates, including tests and files excluded by build tags.
func ReverseDependents(moduleRoot, tree string, changed []string) (_ []string, err error) {
	return gopackages.ReverseDependents(moduleRoot, tree, changed)
}

func reverseDependents(module string, changed []string, imports map[string]map[string]bool) []string {
	changedSet := packagePatterns(module, changed)
	dependentImports := map[string]bool{}
	dependentPackages := map[string]bool{}
	for {
		added := false
		for pkg, packageImports := range imports {
			if dependentPackages[pkg] || changedSet.matchesPackage(pkg) {
				continue
			}
			for imported := range packageImports {
				if changedSet.matchesImport(imported) || dependentImports[imported] {
					dependentPackages[pkg] = true
					dependentImports[packageImportPath(module, pkg)] = true
					added = true
					break
				}
			}
		}
		if !added {
			break
		}
	}

	dependents := make([]string, 0, len(dependentPackages))
	for pkg := range dependentPackages {
		dependents = append(dependents, pkg)
	}
	sort.Strings(dependents)
	return dependents
}

func moduleTree(workspace gittree.Workspace, tree string) (string, error) {
	if data, present, err := workspace.FileAt(tree, "go.mod"); err == nil && present {
		if _, parseErr := modulePath(data); parseErr != nil {
			return "", parseErr
		}
		return tree, nil
	}
	return workspace.TreeOf(tree)
}

func modulePath(data []byte) (string, error) {
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != "module" {
			continue
		}
		path := fields[1]
		if strings.HasPrefix(path, "\"") || strings.HasPrefix(path, "`") {
			unquoted, err := strconv.Unquote(path)
			if err != nil {
				return "", fmt.Errorf("reverse dependents: invalid module directive: %w", err)
			}
			path = unquoted
		}
		if path != "" {
			return path, nil
		}
	}
	return "", fmt.Errorf("reverse dependents: go.mod has no module directive")
}

func packageImports(root, module string, allowed map[string]bool) (map[string]map[string]bool, error) {
	imports := map[string]map[string]bool{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != root && (entry.Name() == "vendor" || entry.Name() == ".git") {
				return filepath.SkipDir
			}
			if path != root {
				if _, statErr := os.Stat(filepath.Join(path, "go.mod")); statErr == nil {
					return filepath.SkipDir
				} else if !os.IsNotExist(statErr) {
					return statErr
				}
			}
			return nil
		}
		if !strings.HasSuffix(entry.Name(), ".go") {
			return nil
		}
		relativeFile, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if allowed != nil && !allowed[filepath.ToSlash(relativeFile)] {
			return nil
		}
		relative, err := filepath.Rel(root, filepath.Dir(path))
		if err != nil {
			return err
		}
		pkg := "."
		if relative != "." {
			pkg = "./" + filepath.ToSlash(relative)
		}
		if imports[pkg] == nil {
			imports[pkg] = map[string]bool{}
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return fmt.Errorf("reverse dependents: parse %s: %w", filepath.ToSlash(path), err)
		}
		for _, specification := range file.Imports {
			imported, err := strconv.Unquote(specification.Path.Value)
			if err != nil {
				return fmt.Errorf("reverse dependents: parse import in %s: %w", filepath.ToSlash(path), err)
			}
			if imported == module || strings.HasPrefix(imported, module+"/") {
				imports[pkg][imported] = true
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return imports, nil
}

type packagePatternSet struct {
	module string
	exact  map[string]bool
	prefix []string
}

func packagePatterns(module string, packages []string) packagePatternSet {
	set := packagePatternSet{module: module, exact: map[string]bool{}}
	for _, pkg := range packages {
		path := strings.TrimPrefix(pkg, "./")
		if pkg == "." {
			path = ""
		}
		if path == "..." {
			set.prefix = append(set.prefix, "")
			continue
		}
		if strings.HasSuffix(path, "/...") {
			set.prefix = append(set.prefix, strings.TrimSuffix(path, "/..."))
			continue
		}
		set.exact[path] = true
	}
	return set
}

func (set packagePatternSet) matchesPackage(pkg string) bool {
	path := strings.TrimPrefix(pkg, "./")
	if pkg == "." {
		path = ""
	}
	if set.exact[path] {
		return true
	}
	for _, prefix := range set.prefix {
		if prefix == "" || path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	return false
}

func (set packagePatternSet) matchesImport(imported string) bool {
	if imported != set.module && !strings.HasPrefix(imported, set.module+"/") {
		return false
	}
	path := strings.TrimPrefix(strings.TrimPrefix(imported, set.module), "/")
	if set.exact[path] {
		return true
	}
	for _, prefix := range set.prefix {
		if prefix == "" || path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	return false
}

func packageImportPath(module, pkg string) string {
	if pkg == "." {
		return module
	}
	return module + "/" + strings.TrimPrefix(pkg, "./")
}

// JoinGatePackageSteps gives each package its own recorded batch-gate step.
func JoinGatePackageSteps(selection UnitPackages) []GateStep {
	steps := make([]GateStep, 0, len(selection.Changed)+len(selection.Dependents)+1)
	for _, pkg := range selection.Changed {
		steps = append(steps, GateStep{Name: "package " + pkg, Args: []string{"go", "test", "-count=1", "-timeout", "900s", pkg}})
	}
	for _, pkg := range selection.Dependents {
		steps = append(steps, GateStep{Name: "dependent package " + pkg, Args: []string{"go", "test", "-count=1", "-timeout", "40m", pkg}})
	}
	if selectionContainsPackage(selection, "./cmd/metasystem") {
		steps = append(steps, batchTestStep())
	}
	return steps
}

// AggregateUnitGateSteps lets go test schedule all ordinary packages in one
// process while retaining the separately compiled cmd/metasystem batch tests.
func AggregateUnitGateSteps(selection UnitPackages) []GateStep {
	packages := append(append([]string{}, selection.Changed...), selection.Dependents...)
	if len(packages) == 0 {
		return nil
	}
	plain := GateStep{
		Name: "unit packages",
		Args: append([]string{"go", "test", "-count=1", "-timeout", "40m"}, packages...),
	}
	steps := []GateStep{plain}
	if selectionContainsPackage(selection, "./cmd/metasystem") {
		steps = append(steps, batchTestStep())
	}
	return steps
}

func batchTestStep() GateStep {
	return GateStep{
		Name: "package ./cmd/metasystem batchtest",
		Args: []string{"go", "test", "-count=1", "-timeout", "40m", "-tags", "batchtest", "./cmd/metasystem"},
	}
}

func selectionContainsPackage(selection UnitPackages, wanted string) bool {
	set := packagePatterns(selection.ModulePath, append(append([]string{}, selection.Changed...), selection.Dependents...))
	return set.matchesPackage(wanted)
}

// FailingTests extracts the first Go test failure line for each failed test.
func FailingTests(output string) []string {
	var failures []string
	seen := map[string]bool{}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "--- FAIL: ") {
			continue
		}
		name := strings.TrimPrefix(line, "--- FAIL: ")
		if field := strings.Fields(name); len(field) != 0 {
			name = field[0]
		}
		if name != "" && !seen[name] {
			seen[name] = true
			failures = append(failures, name)
		}
	}
	return failures
}

// GateFailureDetail keeps test names at the refusal boundary and bounds the
// ordinary command output used when no Go test failure line was printed.
func GateFailureDetail(output string) string {
	if failures := FailingTests(output); len(failures) != 0 {
		return "failing tests: " + strings.Join(failures, ", ")
	}
	detail := strings.TrimSpace(output)
	const maximum = 4 << 10
	if len(detail) > maximum {
		detail = detail[:maximum] + "..."
	}
	return detail
}

// GateRed is one package/test line emitted by the standalone unit gate.
type GateRed struct {
	Package string
	Test    string
}

// GateReds maps one failed go test invocation back to package/test lines.
func GateReds(step GateStep, module, output string) []GateRed {
	packages := gateStepPackages(step)
	pending := []string{}
	reds := []GateRed{}
	seen := map[GateRed]bool{}
	appendRed := func(red GateRed) {
		if red.Package != "" && red.Test != "" && !seen[red] {
			seen[red] = true
			reds = append(reds, red)
		}
	}
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "--- FAIL: ") {
			name := strings.TrimPrefix(trimmed, "--- FAIL: ")
			if fields := strings.Fields(name); len(fields) != 0 {
				pending = append(pending, fields[0])
			}
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) < 2 || fields[0] != "FAIL" {
			continue
		}
		pkg := relativePackage(module, fields[1])
		if pkg == "" {
			continue
		}
		if len(pending) == 0 {
			appendRed(GateRed{Package: pkg, Test: "build"})
		} else {
			for _, test := range pending {
				appendRed(GateRed{Package: pkg, Test: test})
			}
		}
		pending = nil
	}
	if len(pending) != 0 {
		pkg := "packages"
		if len(packages) == 1 {
			pkg = packages[0]
		}
		for _, test := range pending {
			appendRed(GateRed{Package: pkg, Test: test})
		}
	}
	if len(reds) == 0 {
		pkg := "packages"
		if len(packages) == 1 {
			pkg = packages[0]
		}
		appendRed(GateRed{Package: pkg, Test: "build"})
	}
	return reds
}

func gateStepPackages(step GateStep) []string {
	var packages []string
	for _, argument := range step.Args {
		if argument == "." || strings.HasPrefix(argument, "./") {
			packages = append(packages, argument)
		}
	}
	return packages
}

func relativePackage(module, imported string) string {
	if imported == module {
		return "."
	}
	if strings.HasPrefix(imported, module+"/") {
		return "./" + strings.TrimPrefix(imported, module+"/")
	}
	if strings.HasPrefix(imported, "./") || imported == "." {
		return imported
	}
	return ""
}
