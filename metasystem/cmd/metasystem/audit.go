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
