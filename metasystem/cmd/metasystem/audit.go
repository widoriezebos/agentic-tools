package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/audit"
	goalpkg "github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/parallelratchet"
)

// The audit family holds mechanical fences: pure
// judges the gate bootstrap consults between steps.

func runAuditCoverageRatchet(args []string) int {
	flags := flag.NewFlagSet("audit coverage-ratchet", flag.ContinueOnError)
	baselinePath := flags.String("baseline", "", "ratchet baseline JSON")
	module := flags.String("module", "github.com/widoriezebos/agentic-tools/metasystem/", "module prefix to strip")
	input := flags.String("input", "-", "go test -cover output file, - for stdin")
	packages := flags.String("packages", "", "go list output file: the module's package inventory (optional)")
	if flags.Parse(args) != nil {
		return 2
	}
	if *baselinePath == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem audit coverage-ratchet --baseline FILE [--input FILE]")
		return 2
	}
	baseline, err := audit.ReadCoverageBaseline(*baselinePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	var data []byte
	if *input == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(*input)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "coverage input unreadable: %v\n", err)
		return 1
	}
	var inventory []string
	if *packages != "" {
		listing, listErr := os.ReadFile(*packages)
		if listErr != nil {
			fmt.Fprintf(os.Stderr, "package inventory unreadable: %v\n", listErr)
			return 1
		}
		for _, line := range strings.Split(string(listing), "\n") {
			if pkg := strings.TrimSpace(line); pkg != "" {
				inventory = append(inventory, strings.TrimPrefix(pkg, *module))
			}
		}
		if len(inventory) == 0 {
			fmt.Fprintln(os.Stderr, "package inventory is empty; refusing a joinless ratchet run")
			return 1
		}
	}
	violations := audit.CheckCoverage(baseline, audit.ParseCoverage(string(data), *module), inventory)
	for _, violation := range violations {
		fmt.Fprintln(os.Stderr, "coverage ratchet: "+violation)
	}
	if len(violations) > 0 {
		return 1
	}
	return 0
}

func runAuditMetasystem(args []string) int {
	flags := flag.NewFlagSet("audit metasystem", flag.ContinueOnError)
	root := pathFlag(flags, "root", ".", "checkout root to audit")
	maxWords := flags.Int("max-always-loaded-words", 0,
		fmt.Sprintf("always-loaded word budget (0 = %d)", audit.DefaultMaxAlwaysLoadedWords))
	allowPlaceholders := flags.Bool("allow-placeholders", false, "tolerate template placeholders (adopt.sh's structural pass)")
	if flags.Parse(args) != nil {
		return 2
	}
	result, err := audit.AuditMetasystem(*root, audit.AuditOptions{
		MaxAlwaysLoadedWords: *maxWords,
		AllowPlaceholders:    *allowPlaceholders,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 3
	}
	for _, line := range result.Report {
		fmt.Println(line)
	}
	for _, violation := range result.Violations {
		fmt.Fprintln(os.Stderr, violation)
	}
	if len(result.Violations) > 0 {
		return 1
	}
	fmt.Println("metasystem audit passed")
	return 0
}

func runAuditDependencyRatchet(args []string) int {
	flags := flag.NewFlagSet("audit dependency-ratchet", flag.ContinueOnError)
	root := pathFlag(flags, "root", ".", "checkout root to audit")
	if flags.Parse(args) != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem audit dependency-ratchet [--root CHECKOUT]")
		return 2
	}
	findings, err := audit.AuditDependencies(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	for _, finding := range findings {
		fmt.Fprintln(os.Stderr, "dependency ratchet: "+finding.String())
	}
	if len(findings) != 0 {
		return 1
	}
	fmt.Println("dependency ratchet passed")
	return 0
}

func runAuditParallelRatchet(args []string) int {
	flags := flag.NewFlagSet("audit parallel-ratchet", flag.ContinueOnError)
	root := pathFlag(flags, "root", ".", "Go module root to audit")
	update := flags.Bool("update", false, "lower recorded serial-test counts to their current values")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem audit parallel-ratchet [--root MODULE] [--update]")
		return 2
	}
	baselinePath := filepath.Join(*root, "testing-parallel-ratchet.json")
	baseline, err := parallelratchet.ReadParallelRatchet(baselinePath)
	if err != nil {
		return refuseParallelRatchet(err.Error())
	}
	inventory, err := parallelratchet.ScanParallelTests(*root)
	if err != nil {
		return refuseParallelRatchet(err.Error())
	}
	if *update {
		lowered, drops, violations := parallelratchet.LowerParallelRatchet(baseline, inventory)
		if reportParallelViolations(violations) != 0 {
			return refuseParallelRatchet("serial Go test count increased; edit the baseline by hand to raise a count")
		}
		if err := parallelratchet.WriteParallelRatchet(baselinePath, *root, lowered); err != nil {
			return refuseParallelRatchet(err.Error())
		}
		for _, drop := range drops {
			fmt.Printf("parallel ratchet: package %s dropped from %d to %d serial tests\n", drop.Package, drop.From, drop.To)
		}
		fmt.Println("parallel ratchet updated")
		return 0
	}
	_, violations := parallelratchet.CheckParallelRatchet(baseline, inventory)
	if reportParallelViolations(violations) != 0 {
		return refuseParallelRatchet("serial Go test count increased")
	}
	fmt.Println("parallel ratchet passed")
	return 0
}

func reportParallelViolations(violations []parallelratchet.ParallelViolation) int {
	for _, violation := range violations {
		fmt.Fprintln(os.Stderr, "parallel ratchet: "+violation.String())
	}
	return len(violations)
}

func refuseParallelRatchet(reason string) int {
	fmt.Fprintln(os.Stderr, "PARALLEL_RATCHET_REFUSED: "+reason)
	return 1
}

func runAuditHookStartExits(args []string) int {
	flags := flag.NewFlagSet("audit hook-start-exits", flag.ContinueOnError)
	root := pathFlag(flags, "root", ".", "metasystem installation to audit")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem audit hook-start-exits [--root INSTALLATION]")
		return 2
	}
	findings, err := audit.AuditHookStartExits(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	for _, finding := range findings {
		fmt.Fprintln(os.Stderr, "hook start exit audit: "+finding.String())
	}
	if len(findings) != 0 {
		return 1
	}
	fmt.Println("hook start exit audit passed")
	return 0
}

func runAuditStopDecisionSurface(args []string) int {
	flags := flag.NewFlagSet("audit stop-decision-surface", flag.ContinueOnError)
	root := pathFlag(flags, "root", ".", "metasystem installation to audit")
	base := flags.String("base", "", "base commit (default: merge-base HEAD origin/main, or HEAD)")
	jsonOutput := flags.Bool("json", false, "print the result as JSON")
	declare := flags.Bool("declare", false, "write a declaration for the current removed set")
	goal := flags.String("goal", "", "ledger goal that permits a declaration")
	reason := flags.String("reason", "", "one-line reason for a declaration")
	if flags.Parse(args) != nil || flags.NArg() != 0 || (*declare && *jsonOutput) || (!*declare && (*goal != "" || *reason != "")) {
		fmt.Fprintln(os.Stderr, "usage: metasystem audit stop-decision-surface [--root INSTALLATION] [--base COMMIT] [--json] [--declare --goal GOAL --reason TEXT]")
		return 2
	}
	options := audit.StopSurfaceOptions{Base: *base, GoalRecord: goalpkg.StopSurfaceGoalReader}
	if *declare {
		path, err := audit.DeclareStopDecisionSurface(*root, options, *goal, *reason)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Println(path)
		return 0
	}
	result, err := audit.AuditStopDecisionSurface(*root, options)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if *jsonOutput {
		if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if result.Refused() {
			return 1
		}
		return 0
	}
	for _, line := range result.Added {
		fmt.Printf("added: %s: %s\n", line.File, line.Line)
	}
	for _, line := range result.Moved {
		fmt.Printf("moved: %s: %s (goal %s)\n", line.File, line.Line, line.Goal)
	}
	for _, line := range result.Removed {
		fmt.Fprintf(os.Stderr, "removed: %s: %s\n", line.File, line.Line)
	}
	for _, problem := range result.Problems {
		fmt.Fprintln(os.Stderr, "stop decision surface: "+problem)
	}
	fmt.Println(result.Summary())
	if result.Refused() {
		fmt.Fprintln(os.Stderr, "restore the assertion, or run metasystem audit stop-decision-surface --declare --goal <goal-id> --reason <text> with the goal that permits the move")
		return 1
	}
	return 0
}
