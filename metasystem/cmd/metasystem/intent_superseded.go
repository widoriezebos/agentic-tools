package main

import (
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// Accepted decisions keep their original chain identity when another read
// covers the work. Status and refusals expose them without rewriting evidence.
func acceptedRiskLines(file *goal.GoalFile) []string {
	if file == nil {
		return nil
	}
	var lines []string
	for _, risk := range file.AcceptedRisks {
		lines = append(lines, fmt.Sprintf("Accepted risk %s on read %s, by %s", risk.Finding, risk.Chain, risk.By))
	}
	return lines
}
