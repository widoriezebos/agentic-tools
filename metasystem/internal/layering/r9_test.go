package layering

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

const modulePath = "github.com/widoriezebos/agentic-tools/metasystem"

// compositionPackages are the orchestration packages that sit above owners
// (design 6.3). Only cmd/ may import them; a package beneath one (its fakes)
// may import it.
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
	ImportPath string
	Imports    []string
	Deps       []string
	Error      *struct{ Err string }
}

// listModule runs go list -e -deps over the whole module and keeps the
// module's own packages, keyed by module-relative path. -e keeps a package
// that fails to load (an import cycle) in the listing with its imports, so
// the witness can name the offending edge.
func listModule(t *testing.T) map[string]listedPackage {
	t.Helper()
	module, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(module, "go.mod")); err != nil {
		t.Fatalf("module root %s has no go.mod: %v", module, err)
	}
	command := exec.Command("go", "list", "-e", "-deps", "-json=ImportPath,Imports,Deps,Error", "./...")
	command.Dir = module
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		t.Fatalf("go list -e -deps ./... failed: %v\n%s", err, stderr.String())
	}
	packages := map[string]listedPackage{}
	decoder := json.NewDecoder(&stdout)
	for {
		var listed listedPackage
		err := decoder.Decode(&listed)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("go list output is not a JSON package stream: %v", err)
		}
		if relative, ok := moduleRelative(listed.ImportPath); ok {
			listed.Imports = moduleRelativeAll(listed.Imports)
			listed.Deps = moduleRelativeAll(listed.Deps)
			packages[relative] = listed
		}
	}
	return packages
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
// that reaches a composition package, any other package under internal/
// that imports one, a missing owner or composition package, and a
// composition package that no longer composes its owners.
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
		for _, dependency := range append(append([]string{}, listed.Imports...), listed.Deps...) {
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
		if !strings.HasPrefix(path, "internal/") {
			continue
		}
		if _, inside := compositionOf(path, compositions); inside {
			continue
		}
		for _, imported := range packages[path].Imports {
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
			"internal/dispatch":        {},
			"internal/lease":           {Imports: []string{"internal/dispatch"}, Deps: []string{"internal/dispatch"}},
			"internal/delegation":      {Imports: []string{"internal/dispatch", "internal/lease"}},
			"internal/delegation/fake": {Imports: []string{"internal/delegation"}},
			"cmd/metasystem":           {Imports: []string{"internal/delegation"}},
			"internal/ui":              {Imports: []string{"internal/lease"}},
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
