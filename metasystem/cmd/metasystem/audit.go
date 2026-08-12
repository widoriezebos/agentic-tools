package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/audit"
)

// The audit family holds the kill-shell program's mechanical fences: pure
// judges the gate bootstrap consults between steps.

func runAuditCoverageRatchet(args []string) int {
	flags := flag.NewFlagSet("audit coverage-ratchet", flag.ContinueOnError)
	baselinePath := flags.String("baseline", "", "ratchet baseline JSON")
	module := flags.String("module", "github.com/widoriezebos/agentic-tools/metasystem/", "module prefix to strip")
	input := flags.String("input", "-", "go test -cover output file, - for stdin")
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
	violations := audit.CheckCoverage(baseline, audit.ParseCoverage(string(data), *module))
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
	root := flags.String("root", ".", "checkout root to audit")
	maxWords := flags.Int("max-always-loaded-words", 0, "always-loaded word budget (0 = 1400)")
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

// runAuditShellBudget runs the shell complexity fence over the disposition
// registry's scripts. Exit 0 within budget; 1 refused; 2 usage.
func runAuditShellBudget(args []string) int {
	flags := flag.NewFlagSet("audit shell-budget", flag.ContinueOnError)
	root := flags.String("root", ".", "checkout root holding the registry and scripts")
	budget := flags.String("budget", "", "path to the shell budget file")
	if flags.Parse(args) != nil {
		return 2
	}
	budgetPath := *budget
	if budgetPath == "" {
		budgetPath = *root + "/scripts/agents/shell-budget.json"
	}
	violations, report := audit.AuditShellBudget(*root, budgetPath)
	for _, line := range report {
		fmt.Println(line)
	}
	for _, line := range violations {
		fmt.Fprintln(os.Stderr, "shell budget violation: "+line)
	}
	if len(violations) > 0 {
		fmt.Fprintln(os.Stderr, "shell budget refused: the fence only ratchets down (plans/kill-shell.md)")
		return 1
	}
	return 0
}
