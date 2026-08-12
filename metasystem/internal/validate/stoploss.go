package validate

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// The investigation stop-loss check (ported whole from
// scripts/assert-stop-loss.sh under plans/kill-shell.md Phase A): it blocks
// further cycles when a machine-checkable trigger has fired — a dead end, two
// no-progress cycles, an exhausted cycle budget, or as many trailing cycles
// without a contract-improved as the declared no-gain budget. It enforces
// only what the ledger already states; the judgment triggers stay with the
// agent and the human.

var (
	classificationLine  = regexp.MustCompile(`^- Classification:[ \t]*`)
	classificationTerms = regexp.MustCompile(`contract-improved|falsified-continue|falsified-dead-end|no-progress|unresolved|invalid-run`)
	cycleHeading        = regexp.MustCompile(`^### Cycle`)
	leadingNumber       = regexp.MustCompile(`^[0-9]+`)
	confirmedGain       = regexp.MustCompile("^- Classification:[ \t]*`?contract-improved(`|[^a-zA-Z-]|$)")
)

// declaredBudget extracts the first "- <name>:" line's leading integer, or
// "" when no line declares one.
func declaredBudget(lines []string, name string) string {
	prefix := regexp.MustCompile(`^- ` + name + `:[ \t]*`)
	for _, line := range lines {
		if location := prefix.FindString(line); location != "" {
			if number := leadingNumber.FindString(line[len(location):]); number != "" {
				return number
			}
		}
	}
	return ""
}

// StopLoss reads the cycle classifications from an investigation ledger and
// returns the verdict lines and exit code: 0 more cycles are allowed, 1 a
// stop-loss trigger fired, 2 the ledger is unreadable.
func StopLoss(file string) (out, errs []string, code int) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, []string{"missing --file ledger"}, 2
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	totalCycles, deadEnds, noProgress := 0, 0, 0
	// The trailing-cycle count follows the shell's awk exactly: cycles are
	// counted, and a confirmed gain records which cycle it landed in; a
	// cycle with a missing or unrecognized classification is certainly not
	// a confirmed gain, so it counts toward the budget.
	lastGainCycle := 0
	for _, line := range lines {
		if cycleHeading.MatchString(line) {
			totalCycles++
		}
		if confirmedGain.MatchString(line) {
			lastGainCycle = totalCycles
		}
		if classificationLine.MatchString(line) {
			for _, term := range classificationTerms.FindAllString(line[len(classificationLine.FindString(line)):], -1) {
				switch term {
				case "falsified-dead-end":
					deadEnds++
				case "no-progress":
					noProgress++
				}
			}
		}
	}
	budget := declaredBudget(lines, "Cycle budget")
	noGainBudget := declaredBudget(lines, "No-gain budget")

	if deadEnds > 0 {
		return nil, []string{"stop-loss triggered: a cycle was classified falsified-dead-end. Stop, preserve the learning, revert failed behavior, and take the decision up a level"}, 1
	}
	if noProgress >= 2 {
		return nil, []string{fmt.Sprintf(
			"stop-loss triggered: %d cycles classified no-progress. Stop stacking changes and take the decision up a level", noProgress)}, 1
	}
	if budget != "" {
		if limit := atoiPrefix(budget); totalCycles >= limit {
			return nil, []string{fmt.Sprintf(
				"stop-loss triggered: %d cycles recorded against a budget of %s. Stop, or renegotiate the budget with the human", totalCycles, budget)}, 1
		}
	}
	if noGainBudget != "" {
		trailing := totalCycles - lastGainCycle
		if trailing >= atoiPrefix(noGainBudget) {
			return nil, []string{fmt.Sprintf(
				"stop-loss triggered: %d trailing cycles without a contract-improved against a no-gain budget of %s. Stop and hand over the frontier and what was learned", trailing, noGainBudget)}, 1
		}
	}
	budgetText, noGainText := budget, noGainBudget
	if budgetText == "" {
		budgetText = "none"
	}
	if noGainText == "" {
		noGainText = "none"
	}
	return []string{fmt.Sprintf("stop-loss not triggered: %d cycles, %d no-progress, budget %s, no-gain budget %s",
		totalCycles, noProgress, budgetText, noGainText)}, nil, 0
}

func atoiPrefix(value string) int {
	n := 0
	for _, r := range value {
		if r < '0' || r > '9' {
			break
		}
		n = n*10 + int(r-'0')
	}
	return n
}
