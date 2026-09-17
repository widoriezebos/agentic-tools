package batch

import (
	"bytes"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type gateStep struct {
	Name string
	Args []string
}
type gateStepResult struct {
	RunID    string
	ExitCode int
}
type batchGateExec func(unitTree string, step gateStep) gateStepResult
type fixtureSelection struct {
	Groups         []string
	CoveredByProof bool
	Unmapped       []string
}
type patchChange map[string]bool // path -> deleted

func isSharedFixtureHarness(path string) bool {
	switch path {
	case "scripts/agents/fixture-bed-scenarios.sh", "scripts/agents/fixture-budget.sh", "scripts/agents/fixture-assert.sh":
		return true
	}
	return false
}

func runJoinGate(unitTree string, patch, fixtureMap []byte, unit *Unit, execute batchGateExec) error {
	unit.Gate = nil
	groups := parseFixtureBedGroups(fixtureMap)
	paths := patchChangedPaths(patch)
	fixtures := fixtureGroupsForChangedBeds(paths, groups)
	if len(fixtures.Unmapped) != 0 {
		return refuseBatch("BATCH_JOIN_GATE_RED", "unmapped fixture bed "+strings.Join(fixtures.Unmapped, ", "))
	}
	steps := []gateStep{{Name: "fast gate", Args: []string{"bash", "scripts/agents/go-gate.sh", "--fast"}}}
	for _, pkg := range changedGoPackages(paths) {
		steps = append(steps, gateStep{Name: "package " + pkg, Args: []string{"go", "test", "-count=1", pkg}})
	}
	for _, group := range fixtures.Groups {
		steps = append(steps, gateStep{Name: "fixture group " + group, Args: []string{"bin/metasystem", "test", "run", "--root", ".", "--purpose", "diagnostic", "--groups", group, "--mode", "canary", "--tree", unitTree}})
	}
	for _, step := range steps {
		result := execute(unitTree, step)
		if result.RunID != "" {
			unit.Gate = append(unit.Gate, result.RunID)
		}
		if result.ExitCode != 0 {
			return refuseBatch("BATCH_JOIN_GATE_RED", fmt.Sprintf("%s exited %d", step.Name, result.ExitCode))
		}
		if result.RunID == "" {
			return refuseBatch("BATCH_JOIN_GATE_RED", step.Name+" returned no run id")
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

func changedGoPackages(paths patchChange) []string {
	set := map[string]bool{}
	for changed, deleted := range paths {
		changed = strings.TrimPrefix(filepath.ToSlash(changed), "metasystem/")
		if strings.HasSuffix(changed, ".go") {
			dir := filepath.ToSlash(filepath.Dir(changed))
			pkg := "."
			if dir != "." {
				pkg = "./" + dir
			}
			if deleted {
				parent := filepath.ToSlash(filepath.Dir(dir))
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
	return packages
}

func patchChangedPaths(patch []byte) patchChange {
	set := patchChange{}
	old := ""
	header := false
	for _, line := range bytes.Split(patch, []byte("\n")) {
		if bytes.HasPrefix(line, []byte("diff --git ")) {
			if _, changed, ok := strings.Cut(string(line), " b/"); ok {
				set[changed] = false
			}
			header, old = true, ""
		} else if header && bytes.HasPrefix(line, []byte("rename from ")) {
			set[decodePatchPath(string(line[len("rename from "):]))] = true
		} else if header && bytes.HasPrefix(line, []byte("rename to ")) {
			set[decodePatchPath(string(line[len("rename to "):]))] = false
		} else if header && bytes.HasPrefix(line, []byte("--- ")) {
			old = decodePatchPath(string(line[4:]))
		} else if header && bytes.HasPrefix(line, []byte("+++ ")) {
			changed := decodePatchPath(string(line[4:]))
			deleted := changed == "/dev/null"
			if deleted {
				changed = old
			}
			if changed != "" && changed != "/dev/null" {
				set[strings.TrimPrefix(strings.TrimPrefix(changed, "a/"), "b/")] = deleted
			}
			header = false
		}
	}
	return set
}

func decodePatchPath(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, `"`) {
		if decoded, err := strconv.Unquote(raw); err == nil {
			return decoded
		}
	}
	return raw
}
