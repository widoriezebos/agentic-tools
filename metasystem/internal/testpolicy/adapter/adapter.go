// Package adapter is the language seam of the landing lane. The lane decides
// with units, test identities, owner units and results; everything that
// differs per language reaches it through the methods of Adapter, bound to a
// testing-contract group by its adapter value.
package adapter

import (
	"fmt"
	"sort"
	"sync"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// Closure is one unit's changed units and their dependents at an exact tree.
// A unit is the adapter's name for a buildable part: a relative package for
// Go, a module or project for a JVM build.
type Closure struct {
	Tree       string
	Module     string
	Changed    []string
	Dependents []string
	// Unowned holds repository-relative changed paths no language unit owns.
	// These paths do not contribute to Units or Contains.
	Unowned []string
	// Root is the directory in which the closure's test steps run.
	// An adapter that leaves it empty uses the worktree root.
	Root string
}

// Units returns the changed and dependent units, each once, in order.
func (c Closure) Units() []string {
	seen := map[string]bool{}
	units := []string{}
	for _, unit := range append(append([]string{}, c.Changed...), c.Dependents...) {
		if !seen[unit] {
			seen[unit] = true
			units = append(units, unit)
		}
	}
	return units
}

// Contains reports whether unit is in the closure.
func (c Closure) Contains(unit string) bool {
	for _, candidate := range c.Units() {
		if candidate == unit {
			return true
		}
	}
	return false
}

// Failure is one failed test in the JUnit shape every adapter reports.
type Failure struct {
	Report    string
	Classname string
	Name      string
	Status    string
	Reason    string
}

// TestIdentity names one test the way the incident register keys it.
type TestIdentity struct {
	Report    string
	Classname string
	Name      string
}

// Adapter supplies the language facts the lane decides with.
type Adapter interface {
	// Closure computes the changed units between base and tree and the units
	// that depend on them.
	Closure(root, base, tree string) (Closure, error)
	// TestSteps tests each changed unit whole, without a test-name selector.
	TestSteps(closure Closure) []GateStep
	// OwnerUnit maps a failure to the closure unit that owns its test; ok is
	// false when no unit of the closure's module owns it.
	OwnerUnit(failure Failure, closure Closure) (string, bool)
	// Identity returns the failure's test identity; ok is false for a
	// pseudo-identity such as a build terminal or a section leg.
	Identity(failure Failure) (TestIdentity, bool)
}

// GateStep is one command the unit gate runs.
type GateStep struct {
	Name string
	Args []string
}

// GateStepResult is one step's exit and output.
type GateStepResult struct {
	RunID    string
	ExitCode int
	Detail   string
}

// GateRed is one unit/test line the unit gate reports.
type GateRed struct {
	Package string
	Test    string
}

// Detector is implemented by adapters that recognise their project root.
type Detector interface {
	Detects(root string) bool
}

var (
	registryMu sync.RWMutex
	registry   = map[string]Adapter{}
)

// Register binds a language adapter to a testing-contract adapter value.
func Register(name string, adapter Adapter) {
	registryMu.Lock()
	defer registryMu.Unlock()
	if name == "" || adapter == nil {
		panic("adapter: register needs a name and an adapter")
	}
	if name == "command" || name == "section" {
		panic("adapter: " + name + " is the engine's own adapter")
	}
	registry[name] = adapter
}

// Resolve returns the adapter a testing-contract group binds to.
func Resolve(group testpolicy.Group) (Adapter, error) {
	switch group.Adapter {
	case "command":
		return opaque{reportsIdentities: group.Format == "junit-xml"}, nil
	case "section":
		return opaque{}, nil
	}
	registryMu.RLock()
	defer registryMu.RUnlock()
	if adapter, ok := registry[group.Adapter]; ok {
		return adapter, nil
	}
	return nil, fmt.Errorf("adapter %q is not bound in this engine", group.Adapter)
}

// Detect returns the one registered adapter that recognises root.
func Detect(root string) (Adapter, error) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	var found []string
	for _, name := range names {
		if detector, ok := registry[name].(Detector); ok && detector.Detects(root) {
			found = append(found, name)
		}
	}
	if len(found) != 1 {
		return nil, fmt.Errorf("no single language adapter recognises %s (matched %v)", root, found)
	}
	return registry[found[0]], nil
}

// opaque is the command and section adapters' answer: no closure, no owner
// unit, and an identity only for JUnit-reporting commands.
type opaque struct{ reportsIdentities bool }

func (opaque) Closure(_, _, tree string) (Closure, error) { return Closure{Tree: tree}, nil }

func (opaque) TestSteps(Closure) []GateStep { return nil }

func (opaque) OwnerUnit(Failure, Closure) (string, bool) { return "", false }

func (o opaque) Identity(failure Failure) (TestIdentity, bool) {
	if !o.reportsIdentities || failure.Classname == "" || failure.Name == "" {
		return TestIdentity{}, false
	}
	return TestIdentity{Report: failure.Report, Classname: failure.Classname, Name: failure.Name}, true
}
