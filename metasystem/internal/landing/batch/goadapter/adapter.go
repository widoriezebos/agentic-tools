// Package goadapter is the Go language adapter of the landing lane: package
// closures, test identities from test2json, and the unit gate's go test steps.
// Nothing outside this package in the lane names a Go tool.
package goadapter

import (
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/adapter"
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

// Adapter is the Go implementation of adapter.Adapter and adapter.UnitGate.
type Adapter struct{}

func init() { adapter.Register(Name, Adapter{}) }

// Detects recognises a Go module root, or a checkout holding one under metasystem/.
func (Adapter) Detects(root string) bool { return unitGateModuleRoot(root) != "" }

// Closure selects the changed packages and their reverse dependents; tree
// HEAD compares base with the working tree, untracked files included.
func (Adapter) Closure(root, base, tree string) (adapter.Closure, error) {
	var selection UnitPackages
	var err error
	if tree == "HEAD" {
		selection, err = SelectWorkingUnitPackages(root, base)
	} else {
		selection, err = SelectUnitPackages(root, base, tree)
	}
	return closureOf(selection), err
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

// UnitGateSteps is the standalone unit gate's aggregated package run.
func (Adapter) UnitGateSteps(closure adapter.Closure) []adapter.GateStep {
	return AggregateUnitGateSteps(packagesOf(closure))
}

// GateReds maps one failed step's output to package/test lines.
func (Adapter) GateReds(step adapter.GateStep, closure adapter.Closure, output string) []adapter.GateRed {
	return GateReds(step, closure.Module, output)
}

func closureOf(selection UnitPackages) adapter.Closure {
	return adapter.Closure{Tree: selection.Tree, Module: selection.ModulePath,
		Changed: selection.Changed, Dependents: selection.Dependents}
}

func packagesOf(closure adapter.Closure) UnitPackages {
	return UnitPackages{Tree: closure.Tree, ModulePath: closure.Module,
		Changed: closure.Changed, Dependents: closure.Dependents}
}
