package lane

import (
	"fmt"
	"strings"
)

// LandingProofTrailer names, on every commit a lane publication lands, the
// batch and proof attempt that proved its tree.
const LandingProofTrailer = "Landing-Proof"

// ProofValue is the Landing-Proof trailer's value for an attempt.
func ProofValue(batch, attempt string) string { return batch + " attempt " + attempt }

// TrailedSeries is the series base..head as it lands: each commit again
// with the Landing-Proof trailer value added, on the same tree, the same
// author and committer, in the same order, so the published trees are the
// proven trees commit by commit. It is deterministic: the same series and
// value give the same commits, so a repeated publication is the same
// tuple. The range must be linear (every commit one parent, first-parent
// ancestry from base); a commit already carrying value is kept as it is.
func TrailedSeries(repo, base, head, value string) (string, error) {
	listing, err := laneGit(repo, nil, "rev-list", "--reverse", "--topo-order", "--parents", base+".."+head)
	if err != nil {
		return "", fmt.Errorf("the series %s..%s can't be read: %w", short(base), short(head), err)
	}
	parent := base
	rewritten := base
	for _, line := range strings.Split(listing, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if len(fields) != 2 || fields[1] != parent {
			return "", fmt.Errorf("the series %s..%s is not linear at %s", short(base), short(head), short(fields[0]))
		}
		commit := fields[0]
		parent = commit
		rewritten, err = trailed(repo, commit, rewritten, value)
		if err != nil {
			return "", err
		}
	}
	if parent != head {
		return "", fmt.Errorf("%s does not descend linearly from %s", short(head), short(base))
	}
	return rewritten, nil
}

// trailed is commit again on newParent with the trailer added.
func trailed(repo, commit, newParent, value string) (string, error) {
	meta, err := laneGit(repo, nil, "log", "-1", "--format=%T%n%an%n%ae%n%ad%n%cn%n%ce%n%cd", "--date=raw", commit)
	if err != nil {
		return "", err
	}
	fields := strings.Split(meta, "\n")
	if len(fields) != 7 {
		return "", fmt.Errorf("commit %s can't be read", short(commit))
	}
	message, err := laneGit(repo, nil, "log", "-1", "--format=%B", commit)
	if err != nil {
		return "", err
	}
	present, err := laneGit(repo, nil, "log", "-1", "--format=%(trailers:key="+LandingProofTrailer+",valueonly)", commit)
	if err != nil {
		return "", err
	}
	parent, err := laneGit(repo, nil, "rev-parse", commit+"^")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(present) == value && parent == newParent {
		return commit, nil
	}
	if strings.TrimSpace(present) != "" && strings.TrimSpace(present) != value {
		return "", fmt.Errorf("commit %s already names another batch's tests (%s)", short(commit), strings.TrimSpace(present))
	}
	if strings.TrimSpace(present) == "" {
		message, err = interpretTrailer(repo, message, value)
		if err != nil {
			return "", err
		}
	}
	env := []string{"GIT_AUTHOR_NAME=" + fields[1], "GIT_AUTHOR_EMAIL=" + fields[2], "GIT_AUTHOR_DATE=" + fields[3],
		"GIT_COMMITTER_NAME=" + fields[4], "GIT_COMMITTER_EMAIL=" + fields[5], "GIT_COMMITTER_DATE=" + fields[6]}
	return laneGitInput(repo, env, message, "commit-tree", fields[0], "-p", newParent, "-F", "-")
}

func interpretTrailer(repo, message, value string) (string, error) {
	return laneGitInput(repo, nil, message+"\n", "interpret-trailers", "--if-exists", "addIfDifferent", "--trailer", LandingProofTrailer+": "+value)
}
