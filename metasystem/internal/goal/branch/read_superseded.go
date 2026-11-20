package branch

import (
	"crypto/sha256"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// readRangeCovers verifies the range actually supplied to the closed critic.
// CoveredUnits alone is a claim, not evidence of the examined base. Missing
// local frozen evidence leaves coverage unknown, including after a rebase.
func readRangeCovers(repo string, att Attestation, unit UnitStatus) bool {
	if att.Source.Kind != "critic-root" || !att.namesUnit(unit) || att.Subject.Commit == unit.Commit {
		return false
	}
	_, path, _, err := branchReadPathsWithRepository(defaultBranchReadRepository{}, repo, att.Goal, att.Subject.Commit)
	if err != nil {
		return false
	}
	record, err := loadBranchReadRecord(path)
	if err != nil || record.Goal != att.Goal || record.UnitCommit != att.Subject.Commit || record.Tree != att.Subject.Tree || record.RootJob != att.Source.RootJob || record.GateRunID != att.Gate.RunID || !slices.Equal(record.CoveredUnits, att.CoveredUnits) || record.FrozenBriefSHA256 == "" {
		return false
	}
	brief, err := os.ReadFile(record.Brief)
	if err != nil || fmt.Sprintf("%x", sha256.Sum256(brief)) != record.FrozenBriefSHA256 {
		return false
	}
	marker := "\n# Covered units\n\nThis read covers " + strings.Join(record.CoveredUnits, ", ") + ". Examine all their changes on this branch, including earlier commits:\n\n```sh\ngit diff "
	_, tail, found := strings.Cut(string(brief), marker)
	if !found {
		return false
	}
	base, tail, found := strings.Cut(tail, "...")
	if !found || !hex40(base) || !strings.HasPrefix(tail, att.Subject.Commit+" --\n```\n") {
		return false
	}
	// The brief uses a three-dot diff. Its lower bound is the common
	// ancestor, even when main advanced independently of the goal.
	diffBase, err := gitOutput(repo, "merge-base", base, att.Subject.Commit)
	if err != nil {
		return false
	}
	before, err := ancestor(repo, strings.TrimSpace(string(diffBase)), unit.Commit+"^")
	if err != nil || !before {
		return false
	}
	inside, err := ancestor(repo, unit.Commit, att.Subject.Commit)
	return err == nil && inside
}

// coveringRead derives completion from retained evidence; it never changes
// the earlier critic, its accepted risks, or its retired publication journal.
func coveringRead(repo, goalID, unitCommit string) (BranchReadResult, error) {
	tip, present, err := localBranchTip(repo, goalBranchRef(goalID))
	if err != nil || !present {
		return BranchReadResult{}, err
	}
	commits, err := ValidateRange(repo, goal.AcceptedRef, tip, goalID)
	if err != nil {
		return BranchReadResult{}, err
	}
	var source UnitStatus
	for _, c := range commits {
		if c.ID == unitCommit && c.Kind == Unit {
			source = UnitStatus{Unit: c.Unit, Units: c.Units, Commit: c.ID}
		}
	}
	if source.Commit == "" {
		return BranchReadResult{}, nil
	}
	paths, err := gitOutput(repo, "diff-tree", "-r", "-z", "--no-commit-id", "--no-relative", "--no-renames", "--name-only", unitCommit+"^", unitCommit)
	if err != nil {
		return BranchReadResult{}, err
	}
	for i := len(commits) - 1; i >= 0; i-- {
		c := commits[i]
		if c.Kind != Read {
			continue
		}
		kind, e := KindOf(repo, c.ID, goalID)
		if e != nil || kind.CommitID == unitCommit {
			continue
		}
		att, e := ValidateAttestationAt(repo, tip, goal.AcceptedRef, goalID, c.Unit, kind.CommitID)
		if e != nil || !readRangeCovers(repo, att, source) {
			continue
		}
		changes, e := gitOutput(repo, "diff-tree", "-r", "-z", "--no-commit-id", "--no-relative", "--no-renames", "--name-only", unitCommit, att.Subject.Commit)
		if e != nil {
			return BranchReadResult{}, e
		}
		published, e := attestationPublished(repo, goalID, c.ID)
		if e != nil {
			return BranchReadResult{}, e
		}
		result := BranchReadResult{State: "collected", CoveredCommit: att.Subject.Commit, AttestationCommit: c.ID, Published: published, RootJob: att.Source.RootJob, GateRunID: att.Gate.RunID}
		changed := strings.Split(string(changes), "\x00")
		for _, path := range strings.Split(string(paths), "\x00") {
			if path != "" && slices.Contains(changed, path) {
				result.State, result.SupersededBy = "superseded", c.Unit
				break
			}
		}
		return result, nil
	}
	return BranchReadResult{}, nil
}

// ReadCoversUnits checks a repeat's requested coverage against retained content.
func ReadCoversUnits(repo, goalID, commit string, names []string) bool {
	tip, present, err := localBranchTip(repo, goalBranchRef(goalID))
	if err != nil || !present {
		return false
	}
	commits, err := ValidateRange(repo, goal.AcceptedRef, tip, goalID)
	if err != nil {
		return false
	}
	for _, c := range commits {
		if c.Kind != Read {
			continue
		}
		kind, err := KindOf(repo, c.ID, goalID)
		if err != nil || kind.CommitID != commit {
			continue
		}
		att, err := ValidateAttestationAt(repo, tip, goal.AcceptedRef, goalID, c.Unit, commit)
		if err != nil {
			return false
		}
		for _, name := range names {
			found := false
			for _, u := range commits {
				if u.Kind != Unit || !slices.Contains(u.Units, name) {
					continue
				}
				if u.ID == commit || readRangeCovers(repo, att, UnitStatus{Unit: u.Unit, Units: u.Units, Commit: u.ID}) {
					found = true
				}
			}
			if !found {
				return false
			}
		}
		return true
	}
	return false
}
