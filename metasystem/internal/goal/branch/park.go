package branch

// The park branch-safety check, and the three raw readers it is built from.
//
// It lives here rather than at the command edge because two callers need it
// now: `goal park` at the terminal, and the interface's own park under
// R-125-m1u. Both must ask the same question of the same refs — a goal whose
// branch is not on origin is a goal a park would strand — so the readers are
// one implementation with one-line wrappers left at the command edge rather
// than a second copy that could drift from it.
//
// Nothing here decides anything. The policy is CheckParkBranch's, in
// status.go; this supplies it with a scrubbed git, a fresh operation id and
// the two tips it reads the remote through, and hands back the summary the
// verb writes onto the next step.

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os/exec"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// ScrubbedGit runs one git command in root with the scrubbed environment
// every goal-branch read runs under, and answers its trimmed stdout.
//
// stderr is kept out of the answer on purpose: a wrapper that writes a
// warning would otherwise have that warning parsed as an object id.
func ScrubbedGit(root string, args ...string) (string, error) {
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	command.Env = gittree.ScrubbedEnviron()
	var stderr bytes.Buffer
	command.Stderr = &stderr
	out, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(stderr.String()), err)
	}
	return strings.TrimSpace(string(out)), nil
}

// OperationID names one disposable ref for the length of one read. It is
// random rather than derived so two reads in flight at once cannot collide on
// the temporary ref either of them deletes.
func OperationID() (string, error) {
	raw := make([]byte, 12)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return "branch-" + hex.EncodeToString(raw), nil
}

// EndpointTip is the endpoint's main as the remote has it, read through a
// disposable ref that is deleted whether the read succeeded or not.
func EndpointTip(root string, endpoint goal.Endpoint) (tip string, err error) {
	return EndpointTipWithGit(root, endpoint, ScrubbedGit)
}

// EndpointTipWithGit is EndpointTip over a caller's git, which is how a test
// observes the exact argument vectors without a repository.
func EndpointTipWithGit(root string, endpoint goal.Endpoint, git func(string, ...string) (string, error)) (tip string, err error) {
	opid, err := OperationID()
	if err != nil {
		return "", err
	}
	temporary := "refs/metasystem/goals/endpoint/" + opid
	defer func() {
		if _, clearErr := git(root, "update-ref", "-d", temporary); err == nil && clearErr != nil {
			err = clearErr
		}
	}()
	if _, err = git(root, "fetch", "--no-tags", "--refmap=", endpoint.Remote, "+refs/heads/main:"+temporary); err != nil {
		return "", err
	}
	return git(root, "rev-parse", "--verify", temporary+"^{commit}")
}

// OriginTip is the goal's own branch as the remote has it, and whether the
// remote carries it at all.
//
// A fetch that fails is asked again as a bare tip read, because the two
// answers are different facts: a branch the remote does not have is an absent
// branch, and a branch it has that could not be fetched is a failure the
// caller must see rather than read as absence.
func OriginTip(root string, endpoint goal.Endpoint, goalID string) (tip string, present bool, err error) {
	transport := GitPushTransport{}
	opid, err := OperationID()
	if err != nil {
		return "", false, err
	}
	temporary := "refs/metasystem/goals/check/" + opid
	defer func() {
		if _, clearErr := ScrubbedGit(root, "update-ref", "-d", temporary); err == nil && clearErr != nil {
			err = clearErr
		}
	}()
	goalRef := "refs/heads/goal/" + goalID
	if fetchErr := transport.Fetch(root, endpoint.Remote, goalRef, temporary); fetchErr != nil {
		_, present, err = transport.RemoteTip(root, endpoint.Remote, goalRef)
		if err != nil || !present {
			return "", present, err
		}
		return "", false, fetchErr
	}
	tip, err = ScrubbedGit(root, "rev-parse", "--verify", temporary+"^{commit}")
	return tip, true, err
}

// The three readers CheckParkBranch is driven through, named so a caller can
// substitute one without a repository.
type (
	// ParkLocalTipReader answers one raw local ref.
	ParkLocalTipReader func(repo, ref string) (string, bool, error)
	// ParkEndpointTipReader answers the endpoint's main on the remote.
	ParkEndpointTipReader func(root string, endpoint goal.Endpoint) (string, error)
	// ParkOriginTipReader answers the goal's branch on the remote.
	ParkOriginTipReader func(root string, endpoint goal.Endpoint, goalID string) (string, bool, error)
)

// ParkCheck is the check a park verb runs before it pauses a goal: the one
// the command edge has always run, now available to every caller that can
// resolve an endpoint.
func ParkCheck(root string, endpoint goal.Endpoint) func(goalID, next string) (string, error) {
	return ParkCheckWithReaders(root, endpoint, nil, EndpointTip, OriginTip)
}

// ParkCheckWithReaders is ParkCheck over a caller's own readers. A nil local
// tip reader takes the package's ordinary one.
func ParkCheckWithReaders(root string, endpoint goal.Endpoint, localTip ParkLocalTipReader,
	endpointTipReader ParkEndpointTipReader, originTipReader ParkOriginTipReader) func(goalID, next string) (string, error) {
	return func(goalID, next string) (string, error) {
		readRemote := func() (string, string, bool, error) {
			if endpoint.Branch != "refs/heads/main" {
				return "", "", false, fmt.Errorf("GOAL_BRANCH_ENDPOINT_UNSUPPORTED: endpoint %s is not refs/heads/main", endpoint.Branch)
			}
			endpointTip, err := endpointTipReader(root, endpoint)
			if err != nil {
				return "", "", false, err
			}
			originTip, present, err := originTipReader(root, endpoint, goalID)
			return endpointTip, originTip, present, err
		}
		var state ParkBranchState
		var err error
		if localTip == nil {
			state, err = CheckParkBranch(root, goalID, next, readRemote)
		} else {
			state, err = CheckParkBranchWithLocalTip(root, goalID, next, readRemote, localTip)
		}
		return state.Summary, err
	}
}
