// Package goadapter is the Go language adapter of the landing lane: package
// closures, test identities from test2json, and the unit gate's go test steps.
// Nothing outside this package in the lane names a Go tool.
package goadapter

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gopackages"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/adapter"
	"strings"
)

// GateStep, GateStepResult and GateRed are the lane's neutral gate types.
type (
	GateStep       = adapter.GateStep
	GateStepResult = adapter.GateStepResult
	GateRed        = adapter.GateRed
)

// Name is the testing contract's adapter value this package binds to.
const Name = "go"

// packageBuildIdentity is the proof runner's pseudo-test for a package's
// build terminal; it names no test.
const packageBuildIdentity = "package-build"

// Adapter is the Go implementation of adapter.Adapter.
type Adapter struct{}

func init() { adapter.Register(Name, Adapter{}) }

// Detects recognises a Go module root, or a checkout holding one under metasystem/.
func (Adapter) Detects(root string) bool { return unitGateModuleRoot(root) != "" }

// Closure finds the module under root and selects changed packages and their
// reverse dependents. Exact trees report unowned repository-relative paths;
// HEAD compares base with the working tree, untracked files included.
func (Adapter) Closure(root, base, tree string) (adapter.Closure, error) {
	if tree == "HEAD" {
		moduleRoot := unitGateModuleRoot(root)
		if moduleRoot == "" {
			return adapter.Closure{}, fmt.Errorf("no Go module under %s", root)
		}
		selection, err := SelectWorkingUnitPackages(moduleRoot, base)
		closure := closureOf(selection)
		closure.Root = moduleRoot
		return closure, err
	}
	return closureWithWorkspaceSnapshot(gittree.Workspace{Dir: root}, base, tree, nil)
}

func closureWithWorkspaceSnapshot(workspace gittree.Workspace, base, tree string, openSnapshot func(string) (string, func() error, error)) (adapter.Closure, error) {
	root := workspace.Dir
	moduleRoot := unitGateModuleRoot(root)
	if moduleRoot == "" {
		return adapter.Closure{}, fmt.Errorf("no Go module under %s", root)
	}
	prefix, err := filepath.Rel(root, moduleRoot)
	if err != nil {
		return adapter.Closure{}, err
	}
	workspace.Dir = moduleRoot
	if openSnapshot == nil {
		openSnapshot = func(tree string) (string, func() error, error) {
			detached, err := workspace.NewDetachedWorktree(tree)
			if err != nil {
				return "", nil, err
			}
			return detached.Workspace().Dir, detached.Close, nil
		}
	}
	selected, err := gopackages.SelectOwnedWithWorkspaceSnapshot(workspace, base, tree, nil, os.Environ(), openSnapshot)
	if err != nil {
		return adapter.Closure{}, err
	}
	closure := adapter.Closure{Tree: selected.Tree, Module: selected.ModulePath, Root: moduleRoot,
		Changed: selected.Changed, Dependents: selected.Dependents}
	for _, path := range selected.Unowned {
		closure.Unowned = append(closure.Unowned, filepath.ToSlash(filepath.Join(prefix, filepath.FromSlash(path))))
	}
	return closure, nil
}

// TestSteps runs each changed package whole; dependents remain the lane's proof.
func (Adapter) TestSteps(closure adapter.Closure) []adapter.GateStep {
	steps := make([]adapter.GateStep, 0, len(closure.Changed))
	for index, unit := range closure.Changed {
		steps = append(steps, adapter.GateStep{Name: fmt.Sprintf("package-%d", index+1),
			Args: []string{"go", "test", "-count=1", "-timeout", "30m", strings.TrimRight(unit, "/") + "/"}})
	}
	return steps
}

// OwnerUnit maps test2json's package import path to the closure's relative package.
func (Adapter) OwnerUnit(failure adapter.Failure, closure adapter.Closure) (string, bool) {
	if closure.Module == "" || failure.Classname == "" {
		return "", false
	}
	unit := relativePackage(closure.Module, failure.Classname)
	return unit, unit != ""
}

// Identity accepts a named test and rejects the package build terminal.
func (Adapter) Identity(failure adapter.Failure) (adapter.TestIdentity, bool) {
	if failure.Classname == "" || failure.Name == "" || failure.Name == packageBuildIdentity {
		return adapter.TestIdentity{}, false
	}
	return adapter.TestIdentity{Report: failure.Report, Classname: failure.Classname, Name: failure.Name}, true
}

func closureOf(selection UnitPackages) adapter.Closure {
	return adapter.Closure{Tree: selection.Tree, Module: selection.ModulePath,
		Changed: selection.Changed, Dependents: selection.Dependents}
}
