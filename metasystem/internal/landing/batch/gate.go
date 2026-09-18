package batch

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

type gateStep struct {
	Name string
	Args []string
}
type gateStepResult struct {
	RunID    string
	ExitCode int
	Detail   string
}
type batchGateExec func(unitTree string, step gateStep) gateStepResult

type GateStep = gateStep
type GateStepResult = gateStepResult
type GateExecutor = batchGateExec
type fixtureSelection struct {
	Groups         []string
	CoveredByProof bool
	Unmapped       []string
}
type patchChange map[string]bool // path -> deleted
type gateChange struct {
	Deleted, BaseAbsent bool
	baseKnown           bool
}
type gateChanges map[string]gateChange

func isSharedFixtureHarness(path string) bool {
	switch path {
	case "scripts/agents/fixture-bed-scenarios.sh", "scripts/agents/fixture-budget.sh", "scripts/agents/fixture-assert.sh":
		return true
	}
	return false
}

func runJoinGate(unitTree string, patch, fixtureMap []byte, unit *Unit, execute batchGateExec) error {
	return runJoinGateAt("", unitTree, patch, fixtureMap, unit, execute)
}

func runJoinGateAt(root, unitTree string, patch, fixtureMap []byte, unit *Unit, execute batchGateExec) error {
	unit.Gate = nil
	groups := parseFixtureBedGroups(fixtureMap)
	changes := patchGateChanges(patch)
	paths := changes.paths()
	fixtures := fixtureGroupsForChangedBeds(paths, groups)
	if len(fixtures.Unmapped) != 0 {
		return refuseBatch("BATCH_JOIN_GATE_RED", "unmapped fixture bed "+strings.Join(fixtures.Unmapped, ", "))
	}
	steps := []gateStep{{Name: "fast gate", Args: []string{"bash", "scripts/agents/go-gate.sh", "--fast"}}}
	moduleRoot := unitGateModuleRoot(root)
	selection, err := unitPackagesFromChanges(moduleRoot, unitTree, changes)
	if err != nil {
		return err
	}
	steps = append(steps, JoinGatePackageSteps(selection)...)
	for _, group := range fixtures.Groups {
		steps = append(steps, gateStep{Name: "fixture group " + group, Args: []string{"bin/metasystem", "test", "run", "--root", ".", "--purpose", "diagnostic", "--groups", group, "--mode", "canary", "--tree", unitTree}})
	}
	for _, step := range steps {
		result := execute(unitTree, step)
		if result.RunID != "" {
			unit.Gate = append(unit.Gate, result.RunID)
		}
		if result.ExitCode != 0 {
			detail := fmt.Sprintf("%s exited %d", step.Name, result.ExitCode)
			if failure := GateFailureDetail(result.Detail); failure != "" {
				detail += ": " + failure
			}
			return refuseBatch("BATCH_JOIN_GATE_RED", detail)
		}
		if result.RunID == "" {
			return refuseBatch("BATCH_JOIN_GATE_RED", step.Name+" returned no run id")
		}
	}
	return nil
}

func RunJoinGate(unitTree string, patch, fixtureMap []byte, unit *Unit, execute GateExecutor) error {
	return runJoinGate(unitTree, patch, fixtureMap, unit, execute)
}

// RunJoinGateAt selects package steps from unitTree in the repository at root.
func RunJoinGateAt(root, unitTree string, patch, fixtureMap []byte, unit *Unit, execute GateExecutor) error {
	return runJoinGateAt(root, unitTree, patch, fixtureMap, unit, execute)
}

// CheckProtectedTests keeps every named base-contract test in the package
// where the base tree defines it. Candidate policy cannot make a removed test
// disappear from the protected contract, so join rejects that invalid tree
// before admitting it to a batch.
func CheckProtectedTests(root, baseTree, candidateTree string) error {
	workspace := gittree.Workspace{Dir: root}
	modulePrefix := ""
	data, present, err := workspace.FileAt(baseTree, "testing.json")
	if err != nil {
		return err
	}
	if !present {
		modulePrefix = "metasystem"
		data, present, err = workspace.FileAt(baseTree, "metasystem/testing.json")
		if err != nil {
			return err
		}
		if !present {
			return fmt.Errorf("base testing.json is absent from tree %s", baseTree)
		}
	}
	contract, err := testpolicy.Decode(data)
	if err != nil {
		return fmt.Errorf("decode base testing contract: %w", err)
	}
	type packageTests struct {
		names map[string]bool
		err   error
	}
	cache := map[string]packageTests{}
	read := func(tree, pkg string) (map[string]bool, error) {
		key := tree + "\x00" + pkg
		if result, ok := cache[key]; ok {
			return result.names, result.err
		}
		result := packageTests{names: map[string]bool{}}
		packagePath := filepath.ToSlash(filepath.Join(modulePrefix, strings.TrimPrefix(pkg, "./")))
		entries, listErr := workspace.Entries(tree, []string{packagePath})
		if listErr != nil {
			result.err = listErr
			cache[key] = result
			return nil, listErr
		}
		for path := range entries {
			if filepath.ToSlash(filepath.Dir(path)) != packagePath || !strings.HasSuffix(path, "_test.go") {
				continue
			}
			source, exists, readErr := workspace.FileAt(tree, path)
			if readErr != nil || !exists {
				result.err = readErr
				if readErr == nil {
					result.err = fmt.Errorf("test file %s disappeared from tree %s", path, tree)
				}
				break
			}
			file, parseErr := parser.ParseFile(token.NewFileSet(), path, source, 0)
			if parseErr != nil {
				result.err = fmt.Errorf("parse candidate test file %s: %w", path, parseErr)
				break
			}
			for _, declaration := range file.Decls {
				function, ok := declaration.(*ast.FuncDecl)
				if ok && function.Recv == nil && strings.HasPrefix(function.Name.Name, "Test") {
					result.names[function.Name.Name] = true
				}
			}
		}
		cache[key] = result
		return result.names, result.err
	}
	return checkProtectedTestContract(contract, baseTree, candidateTree, read)
}

func checkProtectedTestContract(contract testpolicy.Contract, baseTree, candidateTree string, read func(string, string) (map[string]bool, error)) error {
	for _, group := range contract.Groups {
		if len(group.Tests) == 0 {
			continue
		}
		all, names, err := testpolicy.GoTests(group)
		if err != nil {
			return fmt.Errorf("read base testing group %s: %w", group.ID, err)
		}
		if all {
			continue
		}
		for _, name := range names {
			var definingPackages []string
			for _, pkg := range group.Packages {
				baseNames, readErr := read(baseTree, pkg)
				if readErr != nil {
					return readErr
				}
				if baseNames[name] {
					definingPackages = append(definingPackages, pkg)
				}
			}
			if len(definingPackages) == 0 {
				return refuseBatch("BATCH_JOIN_TEST_DROPPED", fmt.Sprintf("group %s package %s test %s is absent from the base tree", group.ID, strings.Join(group.Packages, ","), name))
			}
			for _, pkg := range definingPackages {
				candidateNames, readErr := read(candidateTree, pkg)
				if readErr != nil {
					return readErr
				}
				if !candidateNames[name] {
					return refuseBatch("BATCH_JOIN_TEST_DROPPED", fmt.Sprintf("group %s package %s test %s is absent from the candidate tree", group.ID, pkg, name))
				}
			}
		}
	}
	return nil
}

func parseFixtureBedGroups(data []byte) map[string][]string {
	groups := map[string][]string{}
	for _, raw := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		fields := strings.SplitN(raw, "\t", 2)
		if len(fields) == 2 {
			groups[fields[0]] = append(groups[fields[0]], fields[1])
		}
	}
	return groups
}

func fixtureGroupsForChangedBeds(paths patchChange, bedGroups map[string][]string) fixtureSelection {
	selected := map[string]bool{}
	coverage := fixtureSelection{}
	for changed := range paths {
		changed = strings.TrimPrefix(filepath.ToSlash(changed), "metasystem/")
		if isSharedFixtureHarness(changed) {
			coverage.CoveredByProof = true
			continue
		}
		for _, group := range bedGroups[changed] {
			selected[group] = true
		}
		if len(bedGroups[changed]) == 0 && isFixtureBedPath(changed) {
			coverage.Unmapped = append(coverage.Unmapped, changed)
		}
	}
	for group := range selected {
		coverage.Groups = append(coverage.Groups, group)
	}
	sort.Strings(coverage.Groups)
	sort.Strings(coverage.Unmapped)
	return coverage
}

func isFixtureBedPath(path string) bool {
	dir, name := filepath.ToSlash(filepath.Dir(path)), filepath.Base(path)
	return (dir == "scripts" || dir == "scripts/agents") && strings.Contains(name, "fixtures") && strings.HasSuffix(name, ".sh")
}

func changedGoPackages(root, tree string, changes gateChanges) ([]string, error) {
	set := map[string]bool{}
	moduleRoot := unitGateModuleRoot(root)
	modulePrefix := ""
	if moduleRoot != "" {
		workspace := gittree.Workspace{Dir: moduleRoot}
		prefix, err := workspace.Prefix()
		if err != nil {
			return nil, err
		}
		modulePrefix = prefix
		resolved, err := moduleTree(workspace, tree)
		if err != nil {
			return nil, err
		}
		tree = resolved
	} else {
		for changed := range changes {
			if strings.HasPrefix(filepath.ToSlash(changed), "metasystem/") {
				modulePrefix = "metasystem/"
				break
			}
		}
	}
	directoryExists := func(dir string) (bool, error) {
		if root == "" {
			return true, nil
		}
		workspaceRoot := moduleRoot
		path := strings.TrimPrefix(filepath.ToSlash(dir), "./")
		if workspaceRoot == "" {
			workspaceRoot = root
			path = filepath.ToSlash(filepath.Join(strings.TrimSuffix(modulePrefix, "/"), path))
		}
		if path == "" || path == "." {
			path = strings.TrimSuffix(modulePrefix, "/")
		}
		entries, err := (gittree.Workspace{Dir: workspaceRoot}).Entries(tree, []string{path})
		return len(entries) != 0, err
	}
	for changed, change := range changes {
		changed = strings.TrimPrefix(filepath.ToSlash(changed), modulePrefix)
		if strings.HasSuffix(changed, ".go") {
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
					exists, err := directoryExists(parent)
					if err != nil {
						return nil, err
					}
					if exists {
						break
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
	}
	packages := make([]string, 0, len(set))
	for pkg := range set {
		packages = append(packages, pkg)
	}
	sort.Strings(packages)
	return packages, nil
}

func patchChangedPaths(patch []byte) patchChange {
	return patchGateChanges(patch).paths()
}

func (changes gateChanges) paths() patchChange {
	paths := patchChange{}
	for path, change := range changes {
		paths[path] = change.Deleted
	}
	return paths
}

func mergeGateChanges(target gateChanges, next gateChanges) {
	for path, change := range next {
		if prior, ok := target[path]; ok {
			prior.Deleted = change.Deleted
			target[path] = prior
		} else {
			target[path] = change
		}
	}
}

func patchGateChanges(patch []byte) gateChanges {
	set := gateChanges{}
	old := ""
	header := false
	update := func(path string, deleted, baseAbsent, baseKnown bool) {
		path = strings.TrimPrefix(strings.TrimPrefix(path, "a/"), "b/")
		if path == "" || path == "/dev/null" {
			return
		}
		change, present := set[path]
		if !present || baseKnown && !change.baseKnown {
			change.BaseAbsent, change.baseKnown = baseAbsent, baseKnown
		}
		change.Deleted = deleted
		set[path] = change
	}
	for _, line := range bytes.Split(patch, []byte("\n")) {
		if bytes.HasPrefix(line, []byte("diff --git ")) {
			if _, changed, ok := strings.Cut(string(line), " b/"); ok {
				update(decodePatchPath(changed), false, false, false)
			}
			header, old = true, ""
		} else if header && bytes.HasPrefix(line, []byte("rename from ")) {
			update(decodePatchPath(string(line[len("rename from "):])), true, false, true)
		} else if header && bytes.HasPrefix(line, []byte("rename to ")) {
			update(decodePatchPath(string(line[len("rename to "):])), false, true, true)
		} else if header && bytes.HasPrefix(line, []byte("--- ")) {
			old = decodePatchPath(string(line[4:]))
		} else if header && bytes.HasPrefix(line, []byte("+++ ")) {
			changed := decodePatchPath(string(line[4:]))
			deleted := changed == "/dev/null"
			if deleted {
				changed = old
			}
			update(changed, deleted, old == "/dev/null", true)
			header = false
		}
	}
	return set
}

// ChangedPaths derives a unit's path manifest from its certified patch.
func ChangedPaths(patch []byte) map[string]bool { return patchChangedPaths(patch) }

func decodePatchPath(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, `"`) {
		if decoded, err := strconv.Unquote(raw); err == nil {
			return decoded
		}
	}
	return raw
}
