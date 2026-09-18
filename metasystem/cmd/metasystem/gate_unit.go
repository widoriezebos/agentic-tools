package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
)

type gateUnitDependencies struct {
	selectPackages func(root, base, tree string) (batch.UnitPackages, error)
	runStep        func(root, tree string, step batch.GateStep) batch.GateStepResult
}

func runGateUnit(args []string) int {
	return runGateUnitWith(args, gateUnitDependencies{
		selectPackages: selectUnitPackagesFromWorktree,
		runStep:        runUnitGateStep,
	}, os.Stdout, os.Stderr)
}

func runGateUnitWith(args []string, dependencies gateUnitDependencies, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("gate unit", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := pathFlag(flags, "root", "", "Go module root")
	base := flags.String("base", "", "base commit or tree")
	tree := flags.String("tree", "HEAD", "candidate commit or tree")
	if flags.Parse(args) != nil {
		return 2
	}
	if *root == "" || *base == "" || *tree == "" || flags.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: metasystem gate unit --root ROOT --base SHA [--tree SHA]")
		return 2
	}

	selection, err := dependencies.selectPackages(*root, *base, *tree)
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

	reds := []batch.GateRed{}
	for _, step := range batch.AggregateUnitGateSteps(selection) {
		result := dependencies.runStep(*root, selection.Tree, step)
		if result.ExitCode == 0 {
			continue
		}
		stepReds := batch.GateReds(step, selection.ModulePath, result.Detail)
		reds = append(reds, stepReds...)
		for _, red := range stepReds {
			fmt.Fprintf(stdout, "RED %s %s\n", red.Package, red.Test)
		}
	}
	if len(reds) != 0 {
		fmt.Fprintf(stdout, "GATE-UNIT RED %d reds\n", len(reds))
		return 1
	}
	fmt.Fprintf(stdout, "GATE-UNIT GREEN %d packages\n", selectedPackageCount(selection))
	return 0
}

func selectUnitPackagesFromWorktree(root, base, tree string) (batch.UnitPackages, error) {
	if tree == "HEAD" {
		return batch.SelectWorkingUnitPackages(root, base)
	}
	return batch.SelectUnitPackages(root, base, tree)
}

func runUnitGateStep(root, tree string, step batch.GateStep) (result batch.GateStepResult) {
	if tree == "" {
		return executeUnitGateCommand(root, step)
	}
	detached, err := (gittree.Workspace{Dir: root}).NewDetachedWorktree(tree)
	if err != nil {
		return batch.GateStepResult{ExitCode: 1, Detail: err.Error()}
	}
	defer func() {
		if closeErr := detached.Close(); closeErr != nil {
			result.ExitCode = 1
			result.Detail = errors.Join(errors.New(result.Detail), closeErr).Error()
		}
	}()
	return executeUnitGateCommand(detached.Workspace().Dir, step)
}

func executeUnitGateCommand(root string, step batch.GateStep) (result batch.GateStepResult) {
	if len(step.Args) == 0 {
		return batch.GateStepResult{ExitCode: 1, Detail: "unit gate step has no command"}
	}
	command := exec.Command(step.Args[0], step.Args[1:]...)
	command.Dir = root
	command.Env = gittree.ScrubbedEnviron()
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

func selectedPackageCount(selection batch.UnitPackages) int {
	set := map[string]bool{}
	for _, pkg := range append(append([]string{}, selection.Changed...), selection.Dependents...) {
		set[pkg] = true
	}
	return len(set)
}
