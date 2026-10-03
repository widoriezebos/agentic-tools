package main

import (
	"errors"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
)

// heldCause is the cause a person can act on that err carries, in the two
// lines of "Messages a Person Reads": a goal branch commit's shape refusal
// in plain words with its fix, or an owner's message that ends in its own
// line 2 ("run: C" or "nothing to do; why"). ok is false for an error with
// nothing to act on but trying again.
func heldCause(err error) (cause string, next []string, then string, ok bool) {
	if err == nil {
		return "", nil, "", false
	}
	var shape *branch.RangeError
	if errors.As(err, &shape) {
		cause = commitCause(shape.Commit, shape.Reason)
		switch remedy, isRun := strings.CutPrefix(shape.Remedy, "run: "); {
		case isRun && strings.HasPrefix(remedy, "metasystem "):
			return cause, strings.Fields(remedy), "", true
		default:
			return cause, nil, remedy, true
		}
	}
	first, _, hint, owned := ownerRemedy(strings.TrimSpace(err.Error()))
	if !owned || strings.TrimSpace(first) == "" {
		return "", nil, "", false
	}
	return strings.TrimSpace(first), hint.Argv, hint.Reason, true
}

// commitCause is a goal branch commit's refusal as one sentence about the
// commit: "commit 289c41c7f doesn't say which goal and unit it builds".
func commitCause(commit, reason string) string {
	short := commit
	if len(short) > 9 {
		short = short[:9]
	}
	switch {
	case strings.HasPrefix(reason, "it "):
		return "commit " + short + " " + strings.TrimPrefix(reason, "it ")
	case strings.HasPrefix(reason, "its "):
		return "commit " + short + "'s " + strings.TrimPrefix(reason, "its ")
	}
	return "commit " + short + ": " + reason
}

// withCause puts the cause err carries into the result's line 1 and its fix
// into line 2, keeping the verb's consequence ("so nothing was landed") and
// the raw error among the details. A result whose error carries no cause a
// person can act on is returned as it was.
func (result intentResult) withCause(err error) intentResult {
	cause, next, then, ok := heldCause(err)
	if !ok {
		return result
	}
	consequence := ""
	if !strings.Contains(cause, "nothing was") {
		for _, mark := range []string{", so ", "; "} {
			if at := strings.LastIndex(result.Summary, mark); at >= 0 {
				consequence = result.Summary[at:]
				break
			}
		}
	}
	// The consequence follows the situation, before any advice the cause
	// ends in: "commit X lacks Y, so nothing was landed; add Y".
	situation, advice, advised := strings.Cut(cause, "; ")
	if advised && consequence != "" && !strings.HasPrefix(consequence, "; ") {
		result.Summary = situation + consequence + "; " + advice
	} else {
		result.Summary = cause + consequence
	}
	result.next, result.nextReason, result.retry, result.Decision = next, then, "", ""
	return result
}

// withCauseRef is withCause for a result returned by reference.
func withCauseRef(err error, result intentResult) *intentResult {
	result = result.withCause(err)
	return &result
}

// argError is the first error among a refusal's format arguments: the
// cause its detail line was written from.
func argError(args []any) error {
	for _, arg := range args {
		if err, ok := arg.(error); ok {
			return err
		}
	}
	return nil
}

// trailerAmend is the command that adds the kind trailer a goal branch
// commit lacks, offered only where it is right: the commit is goal/G's tip
// as read, and the trailer owner names the goal worktree that has exactly
// that commit checked out off main's history. git -C names that worktree,
// never the installation's checkout, where main is. nil otherwise.
func (inv *intentInvocation) trailerAmend(err error, goalID string, state intentBranchState) []string {
	var shape *branch.RangeError
	owners := inv.delivery()
	if !errors.As(err, &shape) || shape.Trailer == "" || shape.Commit == "" || shape.Commit != state.BranchTip || owners.trailerWorktree == nil {
		return nil
	}
	worktree := owners.trailerWorktree(inv.layout.InstallationRoot.Path(), goalID, shape.Commit, state.EndpointTip)
	if worktree == "" {
		return nil
	}
	return []string{"git", "-C", worktree, "commit", "--amend", "--no-edit", "--trailer", shape.Trailer}
}

// productionTrailerWorktree is the worktree of root's repository that has
// goal/G checked out at commit, when commit is not on main's history (at
// endpointTip); "" when any of that is not so or can't be read.
func productionTrailerWorktree(root, goalID, commit, endpointTip string) string {
	if endpointTip != "" {
		if _, err := branch.ScrubbedGit(root, "merge-base", "--is-ancestor", commit, endpointTip); err == nil {
			return ""
		}
	}
	listing, err := branch.ScrubbedGit(root, "worktree", "list", "--porcelain")
	if err != nil {
		return ""
	}
	for _, block := range strings.Split(listing, "\n\n") {
		path, ref := "", ""
		for _, line := range strings.Split(block, "\n") {
			if value, ok := strings.CutPrefix(line, "worktree "); ok {
				path = value
			}
			if value, ok := strings.CutPrefix(line, "branch "); ok {
				ref = value
			}
		}
		if path == "" || ref != "refs/heads/goal/"+goalID {
			continue
		}
		if head, err := branch.ScrubbedGit(path, "rev-parse", "HEAD"); err == nil && strings.TrimSpace(head) == commit {
			return path
		}
	}
	return ""
}
