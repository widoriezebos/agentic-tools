package main

import (
	"fmt"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

// runTestReport summarizes an existing result without selecting or running
// any checks. The caller supplies the observed-cost threshold for its cohort.
func runTestReport(args []string) int {
	flags := newFlagSet("test report")
	resultPath := flags.String("result", "", "retained TestResult JSON path")
	expensiveMS := flags.Int64("expensive-ms", 0, "positive observed group-duration threshold in milliseconds")
	if flags.Parse(args) != nil || *resultPath == "" || *expensiveMS <= 0 || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem test status --result FILE --expensive-ms N")
		return 2
	}
	result, err := readTestingWorkerResult(*resultPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "metasystem test status:", err)
		return 1
	}
	report, err := proofrun.SummarizeTestResultCost(result, *expensiveMS)
	if err != nil {
		fmt.Fprintln(os.Stderr, "metasystem test status:", err)
		return 1
	}
	printJSON(report)
	return 0
}
