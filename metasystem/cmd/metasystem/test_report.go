package main

import (
	"fmt"
	"io"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testrun"
)

// runTestReport summarizes an existing result without selecting or running
// any checks. The caller supplies the observed-cost threshold for its cohort.
func runTestReport(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet("test status", stdout, stderr)
	resultPath := flags.String("result", "", "retained TestResult JSON path")
	expensiveMS := flags.Int64("expensive-ms", 0, "positive observed group-duration threshold in milliseconds")
	if flags.Parse(args) != nil || *resultPath == "" || *expensiveMS <= 0 || flags.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: metasystem test status --result FILE --expensive-ms N")
		return 2
	}
	result, err := testrun.ReadWorkerResult(*resultPath)
	if err != nil {
		fmt.Fprintln(stderr, "metasystem test status:", err)
		return 1
	}
	report, err := proofrun.SummarizeTestResultCost(result, *expensiveMS)
	if err != nil {
		fmt.Fprintln(stderr, "metasystem test status:", err)
		return 1
	}
	writeJSONLine(stdout, stderr, report)
	return 0
}
