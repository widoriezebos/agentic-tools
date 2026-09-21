package goal

// One kept read of an identified commit. A reader that answers many
// questions about one accepted tip — which goals are open, which the claim
// gate admits, how old the tree is — must not re-parse and re-validate the
// commit for each of them, and must not trust a tree that only parsed:
// duplicate ranks, a missing dependency, a cycle, or a channel problem are
// refusals of the whole tree, and a reader that skipped them would present a
// broken ledger as trustworthy. ValidateCommit answers only lawful or not
// and discards what it read; this keeps the tree, the typed problems, and
// the commit's own time, all immutable facts of one commit.

import (
	"fmt"
	"strings"
	"time"
)

// ValidatedTree is one immutable read of an identified commit: the ledger
// tree every at-rest rule accepts, and the commit's committer time.
type ValidatedTree struct {
	Tip         string
	Tree        *TreeGoals
	CommittedAt time.Time // zero when git could not report it; never fatal
}

// ReadValidatedTree reads the commit's ledger subtree once and refuses it
// whole, by name, unless every rule ValidateCommit applies passes:
// ParseTreeFiles, then ValidateTree, then ValidateChannelTree. A parse or
// validation problem is a *TreeReadError carrying the typed problems, so a
// reader can name each one instead of showing part of a tree the engine
// refuses; any other error is the read's own.
//
// The committer time is reported when git answers and left zero when it does
// not: a tree that validates is still the truth without its clock, and a
// missing time is an unknown age, never a refusal.
//
// environments is the seam ReadCommitGoals has. Whatever a caller passes,
// the repository-steering variables are stripped from it before git runs, so
// the commit read is always the one in root.
func ReadValidatedTree(root, tip string, environments ...[]string) (ValidatedTree, error) {
	files, err := ReadCommitGoals(root, tip, environments...)
	if err != nil {
		return ValidatedTree{}, err
	}
	tree, problems := ParseTreeFiles(files)
	if len(problems) == 0 {
		problems = ValidateTree(tree)
	}
	if len(problems) == 0 {
		problems = append(problems, ValidateChannelTree(root, tip)...)
	}
	if len(problems) > 0 {
		return ValidatedTree{}, &TreeReadError{Tip: tip, Problems: problems, Files: files}
	}
	var environment []string
	if len(environments) > 0 {
		environment = environments[0]
	}
	read := ValidatedTree{Tip: tip, Tree: tree}
	if out, timeErr := goalGitWithEnvironment(root, environment, nil, "log", "-1", "--format=%ct", tip); timeErr == nil {
		if seconds := strings.TrimSpace(out); seconds != "" {
			var epoch int64
			if _, scanErr := fmt.Sscanf(seconds, "%d", &epoch); scanErr == nil {
				read.CommittedAt = time.Unix(epoch, 0).UTC()
			}
		}
	}
	return read, nil
}

// NewApprovalHorizon builds the observation ApprovalExpired judges, exactly
// as claim admission builds it. Approval expiry has one owner, so a reader
// outside this package composes the horizon here rather than assembling its
// own from the root record, which would drift the moment the fleet's
// enrollment rule changes.
func NewApprovalHorizon(t *TreeGoals, now time.Time) ApprovalHorizon {
	return approvalHorizon(t, now)
}
