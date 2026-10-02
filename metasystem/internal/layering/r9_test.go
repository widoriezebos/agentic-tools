package layering

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/build"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

const modulePath = "github.com/widoriezebos/agentic-tools/metasystem"

// compositionPackages are the orchestration packages that sit above owners
// (design 6.3). Only cmd/ may import them, in code or in tests; a package
// beneath one (its fakes) may import it.
var compositionPackages = []string{"internal/delegation"}

// ownerPackages are the owners rule R9 names. None may reach a composition
// package, directly or transitively.
var ownerPackages = []string{
	"internal/dispatch", "internal/lease", "internal/steward", "internal/adapter",
	"internal/goal", "internal/launch", "internal/proofrun", "internal/landing",
}

// composedOwners is the positive control: each composition package must
// itself import these owners, so a listing that lost its import data cannot
// pass the witness vacuously.
var composedOwners = map[string][]string{
	"internal/delegation": {"internal/dispatch", "internal/lease", "internal/steward", "internal/adapter", "internal/goal"},
}

type listedPackage struct {
	ImportPath   string
	Imports      []string
	TestImports  []string
	XTestImports []string
	Deps         []string
}

// allImports is a package's code and test imports: a test that imports a
// composition package reaches it as surely as the code does.
func (listed listedPackage) allImports() []string {
	return append(append(append([]string{}, listed.Imports...), listed.TestImports...), listed.XTestImports...)
}

// declaredBuildTags is every build tag the testing contract compiles a group
// under: a file behind a tag the listing omits could import a composition
// package unseen.
func declaredBuildTags(t *testing.T, module string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(module, "testing.json"))
	if err != nil {
		t.Fatalf("read the testing contract: %v", err)
	}
	var contract struct {
		Groups []struct {
			BuildTags []string `json:"buildTags"`
		} `json:"groups"`
	}
	if err := json.Unmarshal(data, &contract); err != nil {
		t.Fatalf("decode the testing contract: %v", err)
	}
	seen := map[string]bool{}
	var tags []string
	for _, group := range contract.Groups {
		for _, tag := range group.BuildTags {
			if !seen[tag] {
				seen[tag] = true
				tags = append(tags, tag)
			}
		}
	}
	sort.Strings(tags)
	return tags
}

// listModule reads every package of the module from source, under every
// build tag the testing contract declares, and keeps the module's own
// packages with their code and test imports and the transitive module
// packages their code imports (Deps), keyed by module-relative path. It
// starts no toolchain: a whole-module go list -deps under a full suite was
// one more toolchain beside every package's tests.
func listModule(t *testing.T) map[string]listedPackage {
	t.Helper()
	module, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(module, "go.mod")); err != nil {
		t.Fatalf("module root %s has no go.mod: %v", module, err)
	}
	packages, err := readModulePackages(module, declaredBuildTags(t, module))
	if err != nil {
		t.Fatal(err)
	}
	return packages
}

func readModulePackages(module string, tags []string) (map[string]listedPackage, error) {
	context := build.Default
	context.BuildTags = tags
	packages := map[string]listedPackage{}
	err := filepath.WalkDir(module, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() {
			return nil
		}
		name := entry.Name()
		if path != module {
			if name == "testdata" || name == "vendor" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
				return filepath.SkipDir
			}
			if _, err := os.Stat(filepath.Join(path, "go.mod")); err == nil {
				return filepath.SkipDir
			}
		}
		listed, err := context.ImportDir(path, 0)
		var noGo *build.NoGoError
		if errors.As(err, &noGo) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read package %s: %w", path, err)
		}
		relative, err := filepath.Rel(module, path)
		if err != nil {
			return err
		}
		packages[filepath.ToSlash(relative)] = listedPackage{
			ImportPath:   modulePath + "/" + filepath.ToSlash(relative),
			Imports:      moduleRelativeAll(listed.Imports),
			TestImports:  moduleRelativeAll(listed.TestImports),
			XTestImports: moduleRelativeAll(listed.XTestImports),
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	for path, listed := range packages {
		seen := map[string]bool{}
		queue := append([]string(nil), listed.Imports...)
		for len(queue) > 0 {
			next := queue[0]
			queue = queue[1:]
			if seen[next] {
				continue
			}
			seen[next] = true
			queue = append(queue, packages[next].Imports...)
		}
		for dependency := range seen {
			listed.Deps = append(listed.Deps, dependency)
		}
		sort.Strings(listed.Deps)
		packages[path] = listed
	}
	return packages, nil
}

func moduleRelative(importPath string) (string, bool) {
	return strings.CutPrefix(importPath, modulePath+"/")
}

func moduleRelativeAll(importPaths []string) []string {
	var relative []string
	for _, importPath := range importPaths {
		if path, ok := moduleRelative(importPath); ok {
			relative = append(relative, path)
		}
	}
	return relative
}

// compositionOf reports the composition package path is, or lies beneath.
func compositionOf(path string, compositions []string) (string, bool) {
	for _, composition := range compositions {
		if path == composition || strings.HasPrefix(path, composition+"/") {
			return composition, true
		}
	}
	return "", false
}

// judgeLayering returns every R9 violation in a module listing: an owner
// that reaches a composition package in code or tests, any package outside
// cmd/ (and outside the composition package itself) whose code or tests
// import one, a missing owner or composition package, and a composition
// package that no longer composes its owners.
func judgeLayering(packages map[string]listedPackage, compositions, owners []string, composed map[string][]string) []string {
	var violations []string
	for _, required := range append(append([]string{}, owners...), compositions...) {
		if _, ok := packages[required]; !ok {
			violations = append(violations, fmt.Sprintf("%s is absent from the module listing; the witness cannot judge a package it does not see", required))
		}
	}
	for composition, wanted := range composed {
		listed, ok := packages[composition]
		if !ok {
			continue
		}
		imports := map[string]bool{}
		for _, path := range listed.Imports {
			imports[path] = true
		}
		for _, owner := range wanted {
			if !imports[owner] {
				violations = append(violations, fmt.Sprintf("%s does not import %s; the composition package must compose its owners", composition, owner))
			}
		}
	}
	for _, owner := range owners {
		listed, ok := packages[owner]
		if !ok {
			continue
		}
		for _, dependency := range append(listed.allImports(), listed.Deps...) {
			if composition, above := compositionOf(dependency, compositions); above {
				violations = append(violations, fmt.Sprintf("owner %s reaches composition package %s through %s", owner, composition, dependency))
			}
		}
	}
	var paths []string
	for path := range packages {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		if path == "cmd" || strings.HasPrefix(path, "cmd/") {
			continue
		}
		if _, inside := compositionOf(path, compositions); inside {
			continue
		}
		for _, imported := range uniqueSorted(packages[path].allImports()) {
			if composition, above := compositionOf(imported, compositions); above {
				violations = append(violations, fmt.Sprintf("%s imports %s; only cmd/ may import composition package %s", path, imported, composition))
			}
		}
	}
	return uniqueSorted(violations)
}

func uniqueSorted(values []string) []string {
	seen := map[string]bool{}
	var unique []string
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			unique = append(unique, value)
		}
	}
	sort.Strings(unique)
	return unique
}

// TestR9OrchestrationSitsAboveOwners is the R9 witness.
func TestR9OrchestrationSitsAboveOwners(t *testing.T) {
	t.Parallel()
	packages := listModule(t)
	internal := 0
	for path := range packages {
		if strings.HasPrefix(path, "internal/") {
			internal++
		}
	}
	t.Logf("R9: judged %d module packages (%d under internal/), owners %v, composition packages %v",
		len(packages), internal, ownerPackages, compositionPackages)
	if violations := judgeLayering(packages, compositionPackages, ownerPackages, composedOwners); len(violations) > 0 {
		t.Fatalf("R9 orchestration-above-owners violations (plans/designs/verbs-object-action.md 6.3):\n  %s",
			strings.Join(violations, "\n  "))
	}
}

func TestR9JudgeNamesEveryViolationClass(t *testing.T) {
	t.Parallel()
	compositions := []string{"internal/delegation"}
	owners := []string{"internal/dispatch", "internal/lease"}
	composed := map[string][]string{"internal/delegation": {"internal/dispatch", "internal/lease"}}
	clean := func() map[string]listedPackage {
		return map[string]listedPackage{
			"internal/dispatch":                {},
			"internal/lease":                   {Imports: []string{"internal/dispatch"}, Deps: []string{"internal/dispatch"}},
			"internal/delegation":              {Imports: []string{"internal/dispatch", "internal/lease"}},
			"internal/delegation/fake":         {Imports: []string{"internal/delegation"}},
			"cmd/metasystem":                   {Imports: []string{"internal/delegation"}},
			"internal/ui":                      {Imports: []string{"internal/lease"}},
			"internal/delegation/fake/fixture": {TestImports: []string{"internal/delegation"}},
			"cmd/devgate":                      {TestImports: []string{"internal/delegation/fake"}},
		}
	}
	if got := judgeLayering(clean(), compositions, owners, composed); len(got) != 0 {
		t.Fatalf("a clean layering was refused: %v", got)
	}
	for _, mutation := range []struct {
		name   string
		mutate func(map[string]listedPackage)
		want   string
	}{
		{"owner imports composition", func(p map[string]listedPackage) {
			p["internal/dispatch"] = listedPackage{Imports: []string{"internal/delegation"}}
		}, "owner internal/dispatch reaches composition package internal/delegation"},
		{"owner reaches composition transitively", func(p map[string]listedPackage) {
			p["internal/lease"] = listedPackage{Imports: []string{"internal/ui"}, Deps: []string{"internal/ui", "internal/delegation"}}
		}, "owner internal/lease reaches composition package internal/delegation through internal/delegation"},
		{"non-owner imports composition", func(p map[string]listedPackage) {
			p["internal/ui"] = listedPackage{Imports: []string{"internal/delegation"}}
		}, "internal/ui imports internal/delegation; only cmd/ may import"},
		{"non-owner imports a composition subpackage", func(p map[string]listedPackage) {
			p["internal/ui"] = listedPackage{Imports: []string{"internal/delegation/fake"}}
		}, "internal/ui imports internal/delegation/fake"},
		{"owner test imports composition", func(p map[string]listedPackage) {
			p["internal/dispatch"] = listedPackage{TestImports: []string{"internal/delegation"}}
		}, "owner internal/dispatch reaches composition package internal/delegation through internal/delegation"},
		{"owner external test imports composition", func(p map[string]listedPackage) {
			p["internal/lease"] = listedPackage{Imports: []string{"internal/dispatch"}, XTestImports: []string{"internal/delegation/fake"}}
		}, "owner internal/lease reaches composition package internal/delegation through internal/delegation/fake"},
		{"non-owner test imports composition", func(p map[string]listedPackage) {
			p["internal/ui"] = listedPackage{XTestImports: []string{"internal/delegation"}}
		}, "internal/ui imports internal/delegation; only cmd/ may import"},
		{"package outside internal/ and cmd/ imports composition", func(p map[string]listedPackage) {
			p["tools/witness"] = listedPackage{Imports: []string{"internal/delegation"}}
		}, "tools/witness imports internal/delegation; only cmd/ may import"},
		{"owner missing from the listing", func(p map[string]listedPackage) {
			delete(p, "internal/lease")
		}, "internal/lease is absent from the module listing"},
		{"composition stops composing", func(p map[string]listedPackage) {
			p["internal/delegation"] = listedPackage{Imports: []string{"internal/dispatch"}}
		}, "internal/delegation does not import internal/lease"},
	} {
		packages := clean()
		mutation.mutate(packages)
		got := judgeLayering(packages, compositions, owners, composed)
		if !strings.Contains(strings.Join(got, "\n"), mutation.want) {
			t.Fatalf("%s: violations %v do not contain %q", mutation.name, got, mutation.want)
		}
	}
}
