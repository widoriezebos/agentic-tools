package goal

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
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
type gitRepository struct {
	endpoint      Endpoint
	ctx           context.Context
	totalDeadline time.Time
}

func (g gitRepository) Capture(opid string) (string, error) {
	if g.ctx != nil {
		return g.capture(opid)
	}
	return CaptureTip(g.endpoint, opid)
}
func (g gitRepository) Accepted() (string, bool, error) {
	if g.ctx != nil {
		return acceptedTipWithRun(g.endpoint.Root, g.git)
	}
	return acceptedTipForGates(g.endpoint.Root)
}
func (g gitRepository) Files(commit string, prefixes ...string) (map[string][]byte, error) {
	return readCommittedFiles(g.ctx, g.endpoint.Root, commit, prefixes, func(input []byte, args ...string) (string, error) {
		cmd := commandWithEnvironment(g.endpoint.commandEnv, "git", args...)
		cmd.Dir = g.endpoint.Root
		cmd.Stdin = bytes.NewReader(input)
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		if err := g.run(cmd); err != nil {
			var exit *exec.ExitError
			if errors.As(err, &exit) {
				return "", &gitError{ExitError: exit, stderr: strings.TrimSpace(stderr.String()), args: args}
			}
			return "", fmt.Errorf("git %s: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
		}
		return stdout.String(), nil
	})
}

func (g gitRepository) Build(opid, parent string, changes []Change, message string) (string, error) {
	return BuildCommit(g.endpoint, opid, parent, changes, message)
}
func (g gitRepository) Publish(parent, commit string) (CASOutcome, error) {
	return PublishCAS(g.endpoint, parent, commit)
}
func (g gitRepository) AcceptedCAS(old, next string) error {
	if g.ctx != nil {
		_, err := g.git("update-ref", AcceptedRef, next, old)
		return err
	}
	return setAcceptedTo(g.endpoint.Root, next, old)
}
func (g gitRepository) IsAncestor(ancestor, descendant string) (bool, error) {
	if g.ctx != nil {
		_, err := g.git("merge-base", "--is-ancestor", ancestor, descendant)
		if gitExitCode(err) == 1 {
			return false, nil
		}
		return err == nil, err
	}
	return IsAncestor(g.endpoint.Root, ancestor, descendant)
}
func (g gitRepository) TrailerPresent(tip, opid string) (bool, error) {
	return TrailerPresent(g.endpoint, tip, opid)
}
func (g gitRepository) CommitWithTrailer(revision, key, value string) (string, error) {
	return commitWithTrailer(g.endpoint.Root, revision, key, value)
}
func (g gitRepository) CommitTime(commit string) (time.Time, error) {
	out, err := g.git("log", "-1", "--format=%ct", commit)
	if err != nil {
		return time.Time{}, err
	}
	var epoch int64
	if _, err := fmt.Sscanf(strings.TrimSpace(out), "%d", &epoch); err != nil {
		return time.Time{}, err
	}
	return time.Unix(epoch, 0), nil
}
func (g gitRepository) Release(opid string) error {
	if g.ctx != nil {
		return g.deleteRefs(fetchRefFor(opid), txnRefFor(opid))
	}
	CleanupRefs(g.endpoint, opid)
	return nil
}

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

// ExistingLedgerIdentityAtEndpoint reads the accepted ledger identity through
// the endpoint's repository, using Git when the endpoint has no repository.
func ExistingLedgerIdentityAtEndpoint(e Endpoint) string {
	return existingLedgerIdentityFor(e)
}

// NewWorldAtEndpoint checks the accepted tree for the committed goal root.
func NewWorldAtEndpoint(e Endpoint) bool {
	if e.Repository == nil {
		return NewWorld(e.Root)
	}
	tip, present, err := e.Repository.Accepted()
	if err != nil || !present || strings.TrimSpace(tip) == "" {
		return false
	}
	files, err := e.Repository.Files(tip, goalsPrefix+"backlog.md")
	_, exists := files[goalsPrefix+"backlog.md"]
	return err == nil && exists
}

// ReadRecordFiles captures the current shared ledger and releases the capture
// after reading one record family through the endpoint's repository.
func ReadRecordFiles(e Endpoint, prefix string) (map[string][]byte, error) {
	nonce, err := NewOperationULID()
	if err != nil {
		return nil, err
	}
	repository := e.repository()
	tip, err := repository.Capture(nonce)
	if err != nil {
		return nil, err
	}
	defer repository.Release(nonce)
	return repository.Files(tip, prefix)
}
