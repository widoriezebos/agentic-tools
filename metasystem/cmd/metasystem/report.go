package main

import (
	"encoding/json"
	"fmt"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/report"
)

// runReportStopBlock ports stop-block.py: print the stop-hook block decision,
// appending any caller detail given as the sole positional argument.
func runReportStopBlock(args []string) int {
	detail := ""
	if len(args) > 0 {
		detail = args[0]
	}
	encoded, _ := json.Marshal(report.StopBlock(detail))
	fmt.Println(string(encoded))
	return 0
}
