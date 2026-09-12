package proofrun

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/audit"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

type goEvent struct {
	Action  string `json:"Action"`
	Package string `json:"Package"`
	Test    string `json:"Test"`
	Output  string `json:"Output"`
}

type goDiscovery struct {
	Tests        map[string][]string
	Inputs       []string
	Inventory    []string
	ModulePrefix string
}

type goPackageCatalog struct {
	ModuleRoot string
	ModuleName string
	Packages   map[string]goListPackage
}

type goDiscoveryCache struct {
	catalogs map[string]goPackageCatalog
}

type goListPackage struct {
	Dir, ImportPath, ForTest string
	Standard                 bool
	Module                   *struct {
		Path, Dir string
		Main      bool
	}
	Imports, TestImports, XTestImports                                             []string
	GoFiles, CgoFiles, CFiles, CXXFiles, MFiles, HFiles, FFiles, SFiles, SysoFiles []string
	SwigFiles, SwigCXXFiles, TestGoFiles, XTestGoFiles                             []string
	EmbedFiles, TestEmbedFiles, XTestEmbedFiles                                    []string
}

func goArguments(ctx context.Context, group testpolicy.Group, cwd string, environment []string) ([]string, []NativeTestIdentity, goDiscovery, bool, error) {
	return goArgumentsCached(ctx, group, cwd, environment, nil)
}

func goArgumentsCached(ctx context.Context, group testpolicy.Group, cwd string, environment []string, cache *goDiscoveryCache) ([]string, []NativeTestIdentity, goDiscovery, bool, error) {
	all, names, err := testpolicy.GoTests(group)
	if err != nil {
		return nil, nil, goDiscovery{}, false, err
	}
	discovery, started, err := discoverGoTestsCached(ctx, cwd, environment, group.Packages, cache)
	if err != nil {
		return nil, nil, discovery, started, err
	}
	args := []string{"go", "test", "-json", "-count=1", "-timeout", "0"}
	if group.Race {
		args = append(args, "-race")
	}
	if group.Coverage {
		args = append(args, "-cover")
	}
	var expected []NativeTestIdentity
	if all {
		for name, packages := range discovery.Tests {
			for _, packageName := range packages {
				expected = append(expected, NativeTestIdentity{Classname: packageName, Name: name, Status: "expected"})
			}
		}
	} else {
		patterns := make([]string, len(names))
		for index, name := range names {
			patterns[index] = regexp.QuoteMeta(name)
			packages := discovery.Tests[name]
			if len(packages) == 0 {
				expected = append(expected, NativeTestIdentity{Name: name, Status: "expected"})
			}
			for _, packageName := range packages {
				expected = append(expected, NativeTestIdentity{Classname: packageName, Name: name, Status: "expected"})
			}
		}
		args = append(args, "-run", "^("+strings.Join(patterns, "|")+")$")
	}
	for _, pkg := range group.Packages {
		args = append(args, "./"+strings.TrimPrefix(pkg, "./"))
	}
	sortNative(expected)
	return args, expected, discovery, started, nil
}

func discoverGoTestsCached(ctx context.Context, cwd string, environment []string, packages []string, cache *goDiscoveryCache) (goDiscovery, bool, error) {
	moduleRoot, moduleName, err := nearestGoModule(cwd)
	if err != nil {
		return goDiscovery{}, false, err
	}
	key := canonicalGoPath(moduleRoot) + "\x00" + digestEnvironment(environment)
	if cache != nil {
		if catalog, ok := cache.catalogs[key]; ok {
			return discoveryFromGoCatalog(catalog, cwd, packages, false)
		}
	}
	catalog, started, err := loadGoPackageCatalog(ctx, moduleRoot, moduleName, environment)
	if err != nil {
		return goDiscovery{}, started, err
	}
	if cache != nil {
		if cache.catalogs == nil {
			cache.catalogs = map[string]goPackageCatalog{}
		}
		cache.catalogs[key] = catalog
	}
	return discoveryFromGoCatalog(catalog, cwd, packages, started)
}

func loadGoPackageCatalog(ctx context.Context, moduleRoot, moduleName string, environment []string) (goPackageCatalog, bool, error) {
	command, err := explicitEnvironmentCommand(ctx, moduleRoot, environment, []string{"go", "list", "-json", "-deps", "-test", "./..."})
	if err != nil {
		return goPackageCatalog{}, false, err
	}
	var stdout, stderr synchronizedBuffer
	activity := newOutputActivity(time.Now())
	command.Stdout = &activityWriter{activity: activity, writer: &stdout}
	command.Stderr = &activityWriter{activity: activity, writer: &stderr}
	outcome := superviseCommand(command, supervisorOptions{Context: ctx, Activity: activity,
		Limits: supervisorLimits{CPUBudgetSeconds: 60}, SampleInterval: supervisorSampleInterval})
	if !outcome.Started {
		return goPackageCatalog{}, false, fmt.Errorf("start go package discovery: %w", outcome.WaitErr)
	}
	if outcome.Verdict != "" {
		return goPackageCatalog{}, true, fmt.Errorf("go package discovery %s: %s", outcome.Verdict, outcome.Reason)
	}
	waitErr := outcome.WaitErr
	if waitErr != nil {
		return goPackageCatalog{}, true, fmt.Errorf("go package discovery failed: %w: %s", waitErr, strings.TrimSpace(stderr.String()))
	}
	decoder := json.NewDecoder(bytes.NewReader(stdout.Bytes()))
	canonicalModuleRoot := canonicalGoPath(moduleRoot)
	allPackages := map[string]goListPackage{}
	for {
		var pkg goListPackage
		if err := decoder.Decode(&pkg); err == io.EOF {
			break
		} else if err != nil {
			return goPackageCatalog{}, true, fmt.Errorf("decode go package discovery: %w", err)
		}
		if pkg.ImportPath == "" || pkg.Dir == "" || pkg.ForTest != "" || pkg.Standard || strings.HasSuffix(pkg.ImportPath, ".test") || strings.Contains(pkg.ImportPath, " [") {
			continue
		}
		pkg.Dir = canonicalGoPath(pkg.Dir)
		rel, relErr := filepath.Rel(canonicalModuleRoot, pkg.Dir)
		if relErr != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		allPackages[pkg.ImportPath] = pkg
	}
	return goPackageCatalog{ModuleRoot: canonicalModuleRoot, ModuleName: moduleName, Packages: allPackages}, true, nil
}

func discoveryFromGoCatalog(catalog goPackageCatalog, cwd string, packages []string, started bool) (goDiscovery, bool, error) {
	canonicalModuleRoot, moduleName, allPackages := catalog.ModuleRoot, catalog.ModuleName, catalog.Packages
	rootPackages := map[string]bool{}
	rootDirs := map[string]bool{}
	for _, declared := range packages {
		relative := strings.TrimPrefix(declared, "./")
		if relative == "." {
			relative = ""
		}
		directory := canonicalGoPath(filepath.Join(cwd, filepath.FromSlash(relative)))
		rootDirs[directory] = true
		moduleRelative, relErr := filepath.Rel(canonicalModuleRoot, directory)
		if relErr != nil || moduleRelative == ".." || strings.HasPrefix(moduleRelative, ".."+string(filepath.Separator)) {
			return goDiscovery{}, started, fmt.Errorf("declared go package %s escapes its module", declared)
		}
		name := moduleName
		if moduleRelative != "." {
			name += "/" + filepath.ToSlash(moduleRelative)
		}
		rootPackages[name] = true
	}
	missingRoot := false
	for name := range rootPackages {
		if _, ok := allPackages[name]; !ok {
			missingRoot = true
		}
	}
	if missingRoot {
		var declaredDirs, observedDirs []string
		for dir := range rootDirs {
			declaredDirs = append(declaredDirs, dir)
		}
		for _, pkg := range allPackages {
			observedDirs = append(observedDirs, filepath.Clean(pkg.Dir))
		}
		sort.Strings(declaredDirs)
		sort.Strings(observedDirs)
		return goDiscovery{}, started, fmt.Errorf("go package discovery omitted one or more declared package directories: declared=%v observed=%v", declaredDirs, observedDirs)
	}
	// The closure is what the roots' test binaries compile and run: the roots
	// with their test imports, then the plain imports of everything reached
	// (a dependency's own tests never run here). What imports a root is not
	// followed: a dependent cannot change a root's outcome, and following it
	// once swept the whole module into every group's identity.
	relevant := map[string]bool{}
	queue := []string{}
	for name := range rootPackages {
		relevant[name] = true
		queue = append(queue, name)
	}
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		pkg, ok := allPackages[name]
		if !ok {
			continue
		}
		imports := append([]string{}, pkg.Imports...)
		if rootPackages[name] {
			imports = append(append(imports, pkg.TestImports...), pkg.XTestImports...)
		}
		for _, imported := range imports {
			if allPackages[imported].ImportPath == "" || relevant[imported] {
				continue
			}
			relevant[imported] = true
			queue = append(queue, imported)
		}
	}
	discovery := goDiscovery{Tests: map[string][]string{}, ModulePrefix: moduleName + "/"}
	inputSet := map[string]bool{"go.mod": true, "go.sum": true}
	for name, pkg := range allPackages {
		if rootPackages[name] {
			discovery.Inventory = append(discovery.Inventory, strings.TrimPrefix(name, discovery.ModulePrefix))
			for _, fileName := range append(append([]string{}, pkg.TestGoFiles...), pkg.XTestGoFiles...) {
				file, parseErr := parser.ParseFile(token.NewFileSet(), filepath.Join(pkg.Dir, fileName), nil, 0)
				if parseErr != nil {
					return goDiscovery{}, started, fmt.Errorf("parse discovered Go test file %s: %w", fileName, parseErr)
				}
				for _, declaration := range file.Decls {
					function, ok := declaration.(*ast.FuncDecl)
					if ok && function.Recv == nil && goTestName(function.Name.Name) {
						discovery.Tests[function.Name.Name] = append(discovery.Tests[function.Name.Name], name)
					}
				}
			}
		}
		if !relevant[name] {
			continue
		}
		// A dependency contributes what the root's test binary compiles from
		// it: its package files and embeds. Its own test files, test embeds
		// and testdata belong to the groups that run its tests.
		files := [][]string{pkg.GoFiles, pkg.CgoFiles, pkg.CFiles, pkg.CXXFiles, pkg.MFiles, pkg.HFiles, pkg.FFiles, pkg.SFiles,
			pkg.SysoFiles, pkg.SwigFiles, pkg.SwigCXXFiles, pkg.EmbedFiles}
		if rootPackages[name] {
			files = append(files, pkg.TestGoFiles, pkg.XTestGoFiles, pkg.TestEmbedFiles, pkg.XTestEmbedFiles)
		}
		for _, names := range files {
			for _, fileName := range names {
				absolute := filepath.Join(pkg.Dir, fileName)
				relative, relErr := filepath.Rel(canonicalModuleRoot, absolute)
				if relErr == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
					inputSet[filepath.ToSlash(relative)] = true
				}
			}
		}
		if !rootPackages[name] {
			continue
		}
		testdata := filepath.Join(pkg.Dir, "testdata")
		walkErr := filepath.WalkDir(testdata, func(path string, entry os.DirEntry, walkErr error) error {
			if os.IsNotExist(walkErr) {
				return filepath.SkipDir
			}
			if walkErr != nil || entry.IsDir() {
				return walkErr
			}
			relative, relErr := filepath.Rel(canonicalModuleRoot, path)
			if relErr == nil {
				inputSet[filepath.ToSlash(relative)] = true
			}
			return nil
		})
		if walkErr != nil && !os.IsNotExist(walkErr) {
			return goDiscovery{}, started, fmt.Errorf("read Go testdata closure for %s: %w", name, walkErr)
		}
	}
	for name := range discovery.Tests {
		sort.Strings(discovery.Tests[name])
	}
	for path := range inputSet {
		discovery.Inputs = append(discovery.Inputs, path)
	}
	sort.Strings(discovery.Inputs)
	sort.Strings(discovery.Inventory)
	return discovery, started, nil
}

func canonicalGoPath(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return filepath.Clean(resolved)
	}
	return filepath.Clean(path)
}

func nearestGoModule(directory string) (string, string, error) {
	current, err := filepath.Abs(directory)
	if err != nil {
		return "", "", err
	}
	for {
		data, readErr := os.ReadFile(filepath.Join(current, "go.mod"))
		if readErr == nil {
			scanner := bufio.NewScanner(bytes.NewReader(data))
			for scanner.Scan() {
				fields := strings.Fields(scanner.Text())
				if len(fields) == 2 && fields[0] == "module" && fields[1] != "" {
					return current, fields[1], nil
				}
			}
			return "", "", fmt.Errorf("go.mod has no module directive")
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", "", fmt.Errorf("go.mod is unavailable")
		}
		current = parent
	}
}

func goTestName(name string) bool {
	if name == "TestMain" || !strings.HasPrefix(name, "Test") || len(name) == len("Test") {
		return false
	}
	next := name[len("Test")]
	return next < 'a' || next > 'z'
}

// CheckNativeDiscovery validates the committed native inventories without
// compiling or executing application tests. Go declarations are parsed from
// source and section identifiers are compared with the finite selector
// catalog exposed by the installed plumbing.
func CheckNativeDiscovery(projectRoot, installation string, contract testpolicy.Contract) error {
	sectionIDs := map[string]bool{}
	discoveryCache := &goDiscoveryCache{catalogs: map[string]goPackageCatalog{}}
	needsSections := false
	for _, group := range contract.Groups {
		if group.Adapter == "section" {
			needsSections = true
		}
	}
	if needsSections {
		selector := filepath.Join(installation, "scripts", "agents", "validate-section-selector.sh")
		command := exec.Command(selector, "catalog")
		command.Env = append([]string(nil), os.Environ()...)
		output, err := command.Output()
		if err != nil {
			return fmt.Errorf("read section selector catalog: %w", err)
		}
		for _, line := range strings.Split(string(output), "\n") {
			id, _, ok := strings.Cut(line, "\t")
			if ok && id != "" {
				sectionIDs[id] = true
			}
		}
	}
	for _, group := range contract.Groups {
		switch group.Adapter {
		case "go":
			_, expected, _, _, err := goArgumentsCached(context.Background(), group, filepath.Join(projectRoot, filepath.FromSlash(group.CWD)), os.Environ(), discoveryCache)
			if err != nil {
				return fmt.Errorf("testing group %s native discovery: %w", group.ID, err)
			}
			if len(expected) == 0 {
				return fmt.Errorf("testing group %s native discovery found no tests", group.ID)
			}
			for _, identity := range expected {
				if identity.Classname == "" {
					return fmt.Errorf("testing group %s declares missing Go test %s", group.ID, identity.Name)
				}
			}
		case "section":
			if !sectionIDs[group.Section] {
				return fmt.Errorf("testing group %s references unknown section %s", group.ID, group.Section)
			}
		}
	}
	return nil
}

func parseGoJSON(output []byte, expected []NativeTestIdentity) (observed, missing, unexpected []NativeTestIdentity, complete bool) {
	terminal := map[string]string{}
	started := map[string]bool{}
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		var event goEvent
		if json.Unmarshal(scanner.Bytes(), &event) != nil || event.Test == "" {
			continue
		}
		key := event.Package + "\x00" + event.Test
		if event.Action == "run" {
			started[key] = true
		}
		if event.Action == "pass" || event.Action == "fail" || event.Action == "skip" {
			terminal[key] = event.Action
		}
	}
	expectedNames := map[string]map[string]bool{}
	for _, item := range expected {
		if expectedNames[item.Classname] == nil {
			expectedNames[item.Classname] = map[string]bool{}
		}
		expectedNames[item.Classname][item.Name] = true
	}
	for key, status := range terminal {
		parts := strings.SplitN(key, "\x00", 2)
		item := NativeTestIdentity{Classname: parts[0], Name: parts[1], Status: map[string]string{"pass": "passed", "fail": "failed", "skip": "skipped"}[status]}
		if len(expectedNames) > 0 && !goExpectedName(expectedNames, item.Classname, item.Name) {
			unexpected = append(unexpected, item)
		} else {
			observed = append(observed, item)
		}
	}
	for key := range started {
		if _, terminalSeen := terminal[key]; terminalSeen {
			continue
		}
		parts := strings.SplitN(key, "\x00", 2)
		missing = append(missing, NativeTestIdentity{Classname: parts[0], Name: parts[1], Status: "missing-terminal"})
	}
	for _, item := range expected {
		found := false
		for key := range started {
			parts := strings.SplitN(key, "\x00", 2)
			if (item.Classname == "" || parts[0] == item.Classname) && parts[1] == item.Name {
				found = true
			}
		}
		if !found {
			item.Status = "missing"
			missing = append(missing, item)
		}
	}
	complete = len(started) > 0 && len(missing) == 0 && len(unexpected) == 0
	sortNative(observed)
	sortNative(missing)
	sortNative(unexpected)
	return
}

func checkGroupCoverage(root, modulePrefix string, inventory []string, output string) ([]string, error) {
	if len(inventory) == 0 {
		return nil, fmt.Errorf("go coverage discovery produced no selected package inventory")
	}
	var baselinePath string
	for _, relative := range coverageBaselineInputs() {
		candidate := filepath.Join(root, filepath.FromSlash(relative))
		if _, err := os.Stat(candidate); err == nil {
			baselinePath = candidate
			break
		}
	}
	if baselinePath == "" {
		return nil, fmt.Errorf("go coverage floor file %s is unavailable", filepath.Base(coverageBaselineInputs()[0]))
	}
	baseline, err := audit.ReadCoverageBaseline(baselinePath)
	if err != nil {
		return nil, err
	}
	selected := &audit.CoverageBaseline{Floors: map[string]float64{}, Exempt: map[string]string{}}
	for _, pkg := range inventory {
		if floor, ok := baseline.Floors[pkg]; ok {
			selected.Floors[pkg] = floor
		}
		if reason, ok := baseline.Exempt[pkg]; ok {
			selected.Exempt[pkg] = reason
		}
	}
	var native strings.Builder
	decoder := json.NewDecoder(strings.NewReader(output))
	for {
		var event goEvent
		if err := decoder.Decode(&event); err != nil {
			// A decoder cannot step past malformed input; retrying the same
			// error forever hung a runner on 2026-09-11. What was read stands.
			break
		}
		native.WriteString(event.Output)
	}
	return audit.CheckCoverage(selected, audit.ParseCoverage(native.String(), modulePrefix), inventory), nil
}

// coverageBaselineInputs returns both supported repository layouts so the
// selected floor and the absence of a competing policy input are part of the
// group execution identity.
func coverageBaselineInputs() []string {
	name := "coverage-ratchet.json"
	if runtime.GOOS == "linux" {
		name = "coverage-ratchet-linux.json"
	}
	return []string{
		filepath.ToSlash(filepath.Join("scripts", "agents", name)),
		filepath.ToSlash(filepath.Join("metasystem", "scripts", "agents", name)),
	}
}

func goExpectedName(expected map[string]map[string]bool, packageName, name string) bool {
	for _, class := range []string{packageName, ""} {
		for root := range expected[class] {
			if name == root || strings.HasPrefix(name, root+"/") {
				return true
			}
		}
	}
	return false
}
