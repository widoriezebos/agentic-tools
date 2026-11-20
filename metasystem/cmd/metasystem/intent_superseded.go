package main

import (
	"fmt"
	"slices"
	"strings"

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

// A repair read keeps the unit list it started with. Once it is published,
// the new tip can host a fresh read of the entire outstanding range.
func (inv *intentInvocation) changedReadResult(targets []intentTarget, root, goalID, unit, repair string, file *goal.GoalFile) intentResult {
	git := inv.work().git
	tip, err := git(root, "rev-parse", "HEAD")
	if err != nil {
		return inv.treeFailure(err)
	}
	head := strings.TrimSpace(string(tip))
	if repair == "" {
		repair = head
	}
	later, _ := inv.work().inspectRead(root, goalID, repair)
	if later.Published && repair != head {
		repair = head
		later, _ = inv.work().inspectRead(root, goalID, repair)
	}
	details := acceptedRiskLines(file)
	if paths, err := git(root, "diff-tree", "-r", "-z", "--no-commit-id", "--no-relative", "--no-renames", "--name-only", unit+"^", unit); err == nil {
		details = append(details, "Still unread from "+shortSHA(unit)+": "+strings.Join(splitNUL(paths), ", "))
	}
	result := intentResult{Targets: targets, Outcome: intentRefused, code: 1, Data: map[string]any{"cause": "environment"},
		Summary: "the worktree changed after the read; review a new version of this work before publishing", Details: details,
		next: inv.publicArgv("work", "review", "--commit", repair, "--goal", goalID)}
	if !later.Published && (later.RootJob != "" || later.AttestationCommit != "") {
		if later.CoveredCommit != "" {
			result.next = inv.publicArgv("work", "review", "--commit", later.CoveredCommit, "--goal", goalID)
		}
		result.Summary = "the repair has a read awaiting collection or publication"
		result.Details = append(details, "Repair read "+chooseUnitValue(later.RootJob, shortSHA(later.AttestationCommit))+" on "+shortSHA(repair))
		result.nextReason = "continues the repair's existing read and publishes it when ready"
		return result
	}
	names, err := git(root, "log", "--reverse", "--format=%B", unit+"^..HEAD")
	if err != nil {
		return inv.treeFailure(err)
	}
	units := strings.FieldsFunc(inv.input.text("units"), func(r rune) bool { return r == ',' })
	for _, line := range strings.Split(string(names), "\n") {
		if tail, ok := strings.CutPrefix(line, "Goal-Unit: "+goalID+"/"); ok {
			for _, name := range strings.Split(tail, "+") {
				if !slices.Contains(units, name) {
					units = append(units, name)
				}
			}
		}
	}
	if len(units) > 0 {
		result.next = inv.publicArgv("work", "review", "--commit", repair, "--goal", goalID, "--units", strings.Join(units, ","))
	}
	result.nextReason = "examines the named units from the branch base; commit or remove uncommitted changes first"
	return result
}
