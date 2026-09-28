// Package fakeadapter is an in-memory, non-Go language adapter. Every lane
// witness decides with it, so a lane that reaches around the adapter seam, or
// compares a classname with a unit name, fails instead of passing by
// coincidence: its classnames are JVM-shaped and its units are module names.
package fakeadapter

import (
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/adapter"
)

// Tool is the fake build tool's name in the steps it plans.
const Tool = "fakebuild"

// Adapter scripts a closure, an owner map and identities.
type Adapter struct {
	Scripted adapter.Closure
	// Owners maps a failing test's classname to the unit that owns it.
	Owners map[string]string
	// Unidentified names failures (classname#name) that carry no identity.
	Unidentified map[string]bool
	// Fresh is the fake tool's fresh-execution argv.
	Fresh []string
	// Calls records every method call, for witnesses.
	Calls *[]string
}

// New returns the fake with two units, payments changed and ledger dependent.
func New() *Adapter {
	return &Adapter{
		Scripted: adapter.Closure{Tree: "fake-tree", Module: "com.example",
			Changed: []string{"payments"}, Dependents: []string{"ledger"}},
		Owners: map[string]string{
			"com.example.PaymentTest": "payments",
			"com.example.LedgerTest":  "ledger",
		},
		Unidentified: map[string]bool{"com.example.PaymentTest#<compile>": true},
		Fresh:        []string{"--rerun-fake"},
		Calls:        &[]string{},
	}
}

func (a *Adapter) record(call string) {
	if a.Calls != nil {
		*a.Calls = append(*a.Calls, call)
	}
}

// Closure returns the scripted closure at the requested tree.
func (a *Adapter) Closure(_, _, tree string) (adapter.Closure, error) {
	a.record("Closure")
	closure := a.Scripted
	if tree != "" {
		closure.Tree = tree
	}
	return closure, nil
}

// OwnerUnit looks the classname up in the owner map.
func (a *Adapter) OwnerUnit(failure adapter.Failure, closure adapter.Closure) (string, bool) {
	a.record("OwnerUnit")
	unit, ok := a.Owners[failure.Classname]
	return unit, ok && unit != ""
}

// Identity accepts every failure with a classname and name unless scripted.
func (a *Adapter) Identity(failure adapter.Failure) (adapter.TestIdentity, bool) {
	a.record("Identity")
	if failure.Classname == "" || failure.Name == "" || a.Unidentified[failure.Classname+"#"+failure.Name] {
		return adapter.TestIdentity{}, false
	}
	return adapter.TestIdentity{Report: "junit-xml", Classname: failure.Classname, Name: failure.Name}, true
}

// FreshExecution returns the scripted fresh-execution argv.
func (a *Adapter) FreshExecution() []string {
	a.record("FreshExecution")
	return append([]string(nil), a.Fresh...)
}

// UnitGateSteps plans one fake build step per unit.
func (a *Adapter) UnitGateSteps(closure adapter.Closure) []adapter.GateStep {
	a.record("UnitGateSteps")
	steps := []adapter.GateStep{}
	for _, unit := range closure.Units() {
		steps = append(steps, adapter.GateStep{Name: "unit " + unit, Args: []string{Tool, "test", unit}})
	}
	return steps
}

// GateReds reads lines "FAILED classname#name" and names the owning unit.
func (a *Adapter) GateReds(_ adapter.GateStep, closure adapter.Closure, output string) []adapter.GateRed {
	a.record("GateReds")
	reds := []adapter.GateRed{}
	for _, line := range strings.Split(output, "\n") {
		test, found := strings.CutPrefix(strings.TrimSpace(line), "FAILED ")
		if !found {
			continue
		}
		classname, name, _ := strings.Cut(test, "#")
		unit, ok := a.OwnerUnit(adapter.Failure{Classname: classname, Name: name}, closure)
		if !ok {
			unit = "unowned"
		}
		reds = append(reds, adapter.GateRed{Package: unit, Test: name})
	}
	return reds
}
