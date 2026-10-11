package plain

import (
	"fmt"
	"slices"
	"strings"
)

// Coordination paths are shared by proof inheritance and main-move classification.
var ledgerPaths = []string{
	"metasystem/plans/goals/", "metasystem/plans/goals.md", "metasystem/plans/goals-accepted.json",
	"metasystem/records/goals/", "metasystem/records/counselor/",
	"metasystem/memory/receipts.log", "metasystem/records/narrator-digest.log",
}

func ledgerPath(path string) bool {
	for _, ledger := range ledgerPaths {
		if path == ledger || strings.HasSuffix(ledger, "/") && strings.HasPrefix(path, ledger) {
			return true
		}
	}
	return false
}

// MainMove describes fetched main relative to the main parent already in flight.
type MainMove struct {
	Kind    string   `json:"kind"`
	Count   int      `json:"count"`
	Commits []string `json:"commits,omitempty"`
}

func mainMove(checkout, commit, main string, s ProveSeams, members ...GoalSHA) (MainMove, error) {
	move := MainMove{Kind: "none"}
	if commit == main {
		return move, nil
	}
	chain, err := s.git(checkout, "rev-list", "--first-parent", "--parents", main+".."+commit)
	if err != nil {
		return move, err
	}
	parent := ""
	if strings.TrimSpace(chain) == "" {
		parent = commit
	}
	for _, line := range strings.Split(strings.TrimSpace(chain), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		for i, candidate := range fields[1:] {
			if i > 0 && slices.ContainsFunc(members, func(m GoalSHA) bool { return m.SHA == candidate }) {
				continue
			}
			inside, err := batchContains(checkout, main, candidate, s)
			if err != nil {
				return move, err
			}
			if inside {
				parent = candidate
				break
			}
		}
		if parent != "" {
			break
		}
	}
	if parent == "" {
		return move, &Refusal{Code: CodeNotFastForward, Reason: "the work in flight has no parent on fetched main, so pushing it would rewrite main; nothing was pushed", Next: "rebuild the recorded batch on fetched main, prove it and push again"}
	}
	listed, err := s.git(checkout, "rev-list", "--reverse", parent+".."+main)
	if err != nil {
		return move, err
	}
	move.Commits = strings.Fields(listed)
	move.Count = len(move.Commits)
	if move.Count == 0 {
		return move, nil
	}
	for _, sha := range move.Commits {
		paths, err := s.git(checkout, "diff-tree", "--root", "--diff-merges=first-parent", "--no-commit-id", "--name-only", "--no-renames", "-r", "-z", sha)
		if err != nil {
			return move, err
		}
		for _, path := range strings.Split(paths, "\x00") {
			if path != "" && !ledgerPath(path) {
				move.Kind = "code"
				return move, nil
			}
		}
	}
	move.Kind = "records-only"
	return move, nil
}

// foldRecords merges coordination updates without invoking the commit guard
// or leaving a pending merge in the checkout.
func foldRecords(install, checkout, main, tree string, move MainMove, s ProveSeams) (string, string, error) {
	refuse := func(reason string) (string, string, error) {
		return "", "", &Refusal{Code: CodeNotFastForward, Reason: reason + "; nothing was pushed", Next: "rebuild the recorded batch on fetched main, prove it and push again"}
	}
	for _, args := range [][]string{{"diff", "--quiet", "HEAD", "--"}, {"diff", "--cached", "--quiet", "HEAD", "--"}} {
		if _, err := s.git(checkout, args...); err != nil {
			return refuse("the record merge requires a clean batch tree: " + err.Error())
		}
	}
	batch, _, err := head(s.git, checkout)
	if err != nil {
		return "", "", err
	}
	merged, err := s.git(checkout, "merge-tree", "--write-tree", batch, main)
	if err != nil {
		return refuse("the record merge failed: " + err.Error())
	}
	foldedTree := strings.TrimSpace(merged)
	if !ledgerOnlyBetween(s.git, checkout, tree, foldedTree) {
		return refuse("the record merge cannot preserve the proven code tree")
	}
	commit, err := s.git(checkout, "commit-tree", foldedTree, "-p", batch, "-p", main, "-m", fmt.Sprintf("Merge coordination updates from main %s into the proven batch", Short(main)))
	if err != nil {
		return "", "", err
	}
	if _, err := s.git(checkout, "checkout", "--detach", commit); err != nil {
		return "", "", err
	}
	commit, foldedTree, err = head(s.git, checkout)
	if err != nil {
		return "", "", err
	}
	if !ledgerOnlyBetween(s.git, checkout, tree, foldedTree) {
		return refuse("the committed record merge changed the proven code tree")
	}
	source, ok, err := ResultFor(install, tree)
	if err != nil || !ok {
		return refuse("the proven source tree cannot be read")
	}
	reason := inheritedReason(source)
	source.FoldedCommits = append(source.FoldedCommits, move.Commits...)
	source.Commit, source.Tree, source.CountedFull = commit, foldedTree, false
	source.Base, source.Scope, source.Reason = tree, "scoped", reason
	source.ScopeReason = source.Reason
	return commit, foldedTree, appendLine(resultsPath(install), source)
}
