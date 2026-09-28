package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/audit"
	goalpkg "github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/up"
)

func runAuditMetasystem(args []string) int {
	flags := flag.NewFlagSet("audit metasystem", flag.ContinueOnError)
	root := pathFlag(flags, "root", ".", "checkout root to audit")
	maxWords := flags.Int("max-always-loaded-words", 0,
		fmt.Sprintf("always-loaded word budget (0 = %d)", audit.DefaultMaxAlwaysLoadedWords))
	allowPlaceholders := flags.Bool("allow-placeholders", false, "tolerate template placeholders (adoption's structural pass)")
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
		fmt.Fprintln(os.Stderr, "usage: metasystem internal audit dependency-ratchet [--root CHECKOUT]")
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

func runAuditHookStartExits(args []string) int {
	flags := flag.NewFlagSet("audit hook-start-exits", flag.ContinueOnError)
	root := pathFlag(flags, "root", ".", "metasystem installation to audit")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem internal audit hook-start-exits [--root INSTALLATION]")
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
	return runAuditStopDecisionSurfaceWith(args, stopDecisionSurfaceDependencies{
		declare: audit.DeclareStopDecisionSurface,
		audit:   audit.AuditStopDecisionSurface,
	})
}

type stopDecisionSurfaceDependencies struct {
	declare func(string, audit.StopSurfaceOptions, string, string) (string, error)
	audit   func(string, audit.StopSurfaceOptions) (audit.StopSurfaceResult, error)
}

func runAuditStopDecisionSurfaceWith(args []string, dependencies stopDecisionSurfaceDependencies) int {
	flags := flag.NewFlagSet("audit stop-decision-surface", flag.ContinueOnError)
	root := pathFlag(flags, "root", ".", "metasystem installation to audit")
	base := flags.String("base", "", "base commit (default: merge-base HEAD origin/main, or HEAD)")
	jsonOutput := flags.Bool("json", false, "print the result as JSON")
	declare := flags.Bool("declare", false, "write a declaration for the current removed set")
	goal := flags.String("goal", "", "ledger goal that permits a declaration")
	reason := flags.String("reason", "", "one-line reason for a declaration")
	if flags.Parse(args) != nil || flags.NArg() != 0 || (*declare && *jsonOutput) || (!*declare && (*goal != "" || *reason != "")) {
		fmt.Fprintln(os.Stderr, "usage: metasystem internal audit stop-decision-surface [--root INSTALLATION] [--base COMMIT] [--json] [--declare --goal GOAL --reason TEXT]")
		return 2
	}
	if dependencies.declare == nil || dependencies.audit == nil {
		fmt.Fprintln(os.Stderr, "stop decision surface: audit and declaration dependencies are required")
		return 1
	}
	options := audit.StopSurfaceOptions{Base: *base, GoalRecord: goalpkg.StopSurfaceGoalReader}
	if *declare {
		path, err := dependencies.declare(*root, options, *goal, *reason)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Println(path)
		return 0
	}
	result, err := dependencies.audit(*root, options)
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
	for _, line := range result.Reworded {
		fmt.Printf("reworded: %s: %s -> %s\n", line.File, line.From, line.To)
	}
	for _, line := range result.Removed {
		fmt.Fprintf(os.Stderr, "removed: %s: %s\n", line.File, line.Line)
	}
	for _, problem := range result.Problems {
		fmt.Fprintln(os.Stderr, "stop decision surface: "+problem)
	}
	fmt.Println(result.Summary())
	if result.Refused() {
		fmt.Fprintln(os.Stderr, "restore the assertion, or run metasystem internal audit stop-decision-surface --declare --goal <goal-id> --reason <text> with a goal allowed stop-test changes; a person allows the move with metasystem goal allow <goal-id> stop-test-changes --reason <text>")
		return 1
	}
	return 0
}

// runAuditProductionCommands checks this host for the production command
// inventory (up.ProductionCommands) and names each missing command with its
// Debian-family package: `audit production-commands`.
func runAuditProductionCommands(args []string) int {
	return auditProductionCommands(args, exec.LookPath, os.Stderr)
}

func auditProductionCommands(args []string, lookPath func(string) (string, error), errOut io.Writer) int {
	if len(args) != 0 {
		fmt.Fprintln(errOut, "usage: metasystem internal audit production-commands")
		return 2
	}
	missing := up.MissingProductionCommands(lookPath)
	if len(missing) == 0 {
		return 0
	}
	fmt.Fprintln(errOut, "command preflight: this host is missing production commands:")
	for _, command := range missing {
		fmt.Fprintf(errOut, "  %s (package: %s)\n", command.Name, command.Package)
	}
	return 1
}
