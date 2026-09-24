package goal

import (
	"fmt"
	"strings"
	"time"
)

// Repository supplies committed facts and atomic ref effects for one endpoint.
// Files returns immutable bytes at a commit for the requested path prefixes.
type Repository interface {
	Capture(opid string) (string, error)
	Accepted() (tip string, present bool, err error)
	Files(commit string, prefixes ...string) (map[string][]byte, error)
	Build(opid, parent string, changes []Change, message string) (string, error)
	Publish(parent, commit string) (CASOutcome, error)
	AcceptedCAS(old, next string) error
	IsAncestor(ancestor, descendant string) (bool, error)
	TrailerPresent(tip, opid string) (bool, error)
	CommitWithTrailer(revision, key, value string) (string, error)
	CommitTime(commit string) (time.Time, error)
	Release(opid string) error
}

func (e Endpoint) repository() Repository {
	if e.Repository != nil {
		return e.Repository
	}
	return gitRepository{endpoint: e}
}

// gitRepository keeps the production Git implementation in the goal package.
type gitRepository struct{ endpoint Endpoint }

func (g gitRepository) Capture(opid string) (string, error) { return CaptureTip(g.endpoint, opid) }
func (g gitRepository) Accepted() (string, bool, error) {
	return acceptedTipForGates(g.endpoint.Root)
}
func (g gitRepository) Files(commit string, prefixes ...string) (map[string][]byte, error) {
	args := append([]string{"ls-tree", "-r", "--name-only", commit, "--"}, prefixes...)
	out, err := gitIn(g.endpoint.Root, args...)
	if err != nil {
		return nil, fmt.Errorf("cannot list committed files at %s: %w", commit, err)
	}
	var paths []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if path := strings.TrimSpace(line); path != "" {
			paths = append(paths, path)
		}
	}
	return readCommitGoalBlobs(g.endpoint.Root, commit, paths, nil)
}
func (g gitRepository) Build(opid, parent string, changes []Change, message string) (string, error) {
	return BuildCommit(g.endpoint, opid, parent, changes, message)
}
func (g gitRepository) Publish(parent, commit string) (CASOutcome, error) {
	return PublishCAS(g.endpoint, parent, commit)
}
func (g gitRepository) AcceptedCAS(old, next string) error {
	return setAcceptedTo(g.endpoint.Root, next, old)
}
func (g gitRepository) IsAncestor(ancestor, descendant string) (bool, error) {
	return IsAncestor(g.endpoint.Root, ancestor, descendant)
}
func (g gitRepository) TrailerPresent(tip, opid string) (bool, error) {
	return TrailerPresent(g.endpoint, tip, opid)
}
func (g gitRepository) CommitWithTrailer(revision, key, value string) (string, error) {
	return commitWithTrailer(g.endpoint.Root, revision, key, value)
}
func (g gitRepository) CommitTime(commit string) (time.Time, error) {
	out, err := goalGit(g.endpoint.Root, nil, "log", "-1", "--format=%ct", commit)
	if err != nil {
		return time.Time{}, err
	}
	var epoch int64
	if _, err := fmt.Sscanf(strings.TrimSpace(out), "%d", &epoch); err != nil {
		return time.Time{}, err
	}
	return time.Unix(epoch, 0), nil
}
func (g gitRepository) Release(opid string) error { CleanupRefs(g.endpoint, opid); return nil }

func readCommitFiles(e Endpoint, commit string, prefixes ...string) (map[string][]byte, error) {
	return e.repository().Files(commit, prefixes...)
}

func readCommitGoals(e Endpoint, commit string) (map[string][]byte, error) {
	return readCommitFiles(e, commit, goalsPrefix, recordsGoalsPrefix)
}

func existingLedgerIdentityFor(e Endpoint) string {
	if e.Repository == nil {
		return ExistingLedgerIdentity(e.Root)
	}
	tip, present, err := e.repository().Accepted()
	if err != nil || !present {
		return ""
	}
	identity, err := treeIdentityFor(e, tip)
	if err != nil {
		return ""
	}
	return identity
}
