package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gocache"
	_ "github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch/goadapter"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/adapter"
)

// gateUnitDependencies binds the unit gate to a language adapter: the adapter
// selects the closure, plans the steps and reads the reds; the gate only runs.
type gateUnitDependencies struct {
	adapter func(root string) (adapter.Adapter, error)
	runStep func(root, tree string, step adapter.GateStep) adapter.GateStepResult
}

func runGateUnit(args []string) int {
	return runGateUnitWith(args, gateUnitDependencies{
		adapter: adapter.Detect,
		runStep: runUnitGateStep,
	}, os.Stdout, os.Stderr)
}

func runGateUnitWith(args []string, dependencies gateUnitDependencies, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("gate unit", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := pathFlag(flags, "root", "", "project root")
	base := flags.String("base", "", "base commit or tree")
	tree := flags.String("tree", "HEAD", "candidate commit or tree")
	if flags.Parse(args) != nil {
		return 2
	}
	if *root == "" || *base == "" || *tree == "" || flags.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: metasystem internal gate unit --root ROOT --base SHA [--tree SHA]")
		return 2
	}

	language, err := dependencies.adapter(*root)
	var gate adapter.UnitGate
	if err == nil {
		var ok bool
		if gate, ok = language.(adapter.UnitGate); !ok {
			err = errors.New("the project's language adapter has no unit gate")
		}
	}
	var selection adapter.Closure
	if err == nil {
		selection, err = language.Closure(*root, *base, *tree)
	}
	if err != nil {
		fmt.Fprintln(stderr, "gate unit:", err)
		return 1
	}
	fmt.Fprintf(stdout, "CHANGED PACKAGES %d\n", len(selection.Changed))
	for _, pkg := range selection.Changed {
		fmt.Fprintln(stdout, "CHANGED", pkg)
	}
	fmt.Fprintf(stdout, "DEPENDENT PACKAGES %d\n", len(selection.Dependents))
	for _, pkg := range selection.Dependents {
		fmt.Fprintln(stdout, "DEPENDENT", pkg)
	}

	reds := []adapter.GateRed{}
	for _, step := range gate.UnitGateSteps(selection) {
		result := dependencies.runStep(*root, selection.Tree, step)
		if result.ExitCode == 0 {
			continue
		}
		stepReds := gate.GateReds(step, selection, result.Detail)
		reds = append(reds, stepReds...)
		for _, red := range stepReds {
			fmt.Fprintf(stdout, "RED %s %s\n", red.Package, red.Test)
		}
	}
	if len(reds) != 0 {
		fmt.Fprintf(stdout, "GATE-UNIT RED %d reds\n", len(reds))
		return 1
	}
	fmt.Fprintf(stdout, "GATE-UNIT GREEN %d packages\n", len(selection.Units()))
	return 0
}

func runUnitGateStep(root, tree string, step adapter.GateStep) (result adapter.GateStepResult) {
	if tree == "" {
		return executeUnitGateCommand(root, step)
	}
	detached, err := (gittree.Workspace{Dir: root}).NewDetachedWorktree(tree)
	if err != nil {
		return adapter.GateStepResult{ExitCode: 1, Detail: err.Error()}
	}
	defer func() {
		if closeErr := detached.Close(); closeErr != nil {
			result.ExitCode = 1
			result.Detail = errors.Join(errors.New(result.Detail), closeErr).Error()
		}
	}()
	return executeUnitGateCommand(detached.Workspace().Dir, step)
}

func executeUnitGateCommand(root string, step adapter.GateStep) (result adapter.GateStepResult) {
	if len(step.Args) == 0 {
		return adapter.GateStepResult{ExitCode: 1, Detail: "unit gate step has no command"}
	}
	command := exec.Command(step.Args[0], step.Args[1:]...)
	command.Dir = root
	command.Env = gocache.Carry(gittree.ScrubbedEnviron())
	output, commandErr := command.CombinedOutput()
	result.Detail = string(output)
	if commandErr != nil {
		result.ExitCode = 1
		if command.ProcessState != nil {
			result.ExitCode = command.ProcessState.ExitCode()
		}
	}
	return result
}
