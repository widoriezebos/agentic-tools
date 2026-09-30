package branch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

const (
	LeaseMovedCode  = "GOAL_BRANCH_LEASE_MOVED"
	ClaimLostCode   = "GOAL_BRANCH_CLAIM_LOST"
	PushUnknownCode = "GOAL_BRANCH_PUSH_UNKNOWN"
	StaleCode       = "GOAL_BRANCH_STALE"
)

type CASOutcome string

const (
	CASLanded  CASOutcome = "landed"
	CASRefused CASOutcome = "refused"
	CASUnknown CASOutcome = "unknown"
)

type PushTransport interface {
	RemoteTip(repo, remote, ref string) (string, bool, error)
	Fetch(repo, remote, ref, destination string) error
	Push(repo, remote, ref, expected, tip string) (CASOutcome, error)
}

// GitPushTransport runs the transport through git. Context, when set, bounds
// every call (the disk sweeper's pass budget); the zero value is unbounded.
type GitPushTransport struct{ Context context.Context }

func (t GitPushTransport) context() context.Context {
	if t.Context == nil {
		return context.Background()
	}
	return t.Context
}

func (t GitPushTransport) RemoteTip(repo, remote, ref string) (string, bool, error) {
	out, err := gitOutputContext(t.context(), repo, "ls-remote", "--refs", remote, ref)
	if err != nil {
		return "", false, err
	}
	fields := strings.Fields(string(out))
	if len(fields) == 0 {
		return "", false, nil
	}
	if len(fields) != 2 || !hex40(fields[0]) || fields[1] != ref {
		return "", false, fmt.Errorf("remote returned a malformed value for %s", ref)
	}
	return fields[0], true, nil
}

func (t GitPushTransport) Fetch(repo, remote, ref, destination string) error {
	_, err := gitOutputContext(t.context(), repo, "fetch", "--no-tags", "--refmap=", remote, "+"+ref+":"+destination)
	return err
}

func (t GitPushTransport) Push(repo, remote, ref, expected, tip string) (CASOutcome, error) {
	cmd := exec.CommandContext(t.context(), "git", "-C", repo, "push", remote,
		"--force-with-lease="+ref+":"+expected, tip+":"+ref)
	cmd.Env = gittree.ScrubbedEnviron("LC_ALL=C")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if err == nil {
		return CASLanded, nil
	}
	if goal.ClassifyPushFailure(stdout.String()+stderr.String()) == goal.CASRefused {
		return CASRefused, fmt.Errorf("git push: %s: %w", strings.TrimSpace(stderr.String()), err)
	}
	return CASUnknown, fmt.Errorf("git push: %s: %w", strings.TrimSpace(stderr.String()), err)
}

type PushHooks struct {
	AfterRemoteRead       func() error
	AfterPush             func() error
	BeforeAdoptionRefMove func() error
	AfterAdoptionRefMove  func() error
}

type PushRequest struct {
	Repo, Remote, EndpointTip, GoalID, OpID string
	CheckClaim                              func() error
	Transport                               PushTransport
	Hooks                                   PushHooks
}

type PushResult struct {
	State string
	Tip   string
}

type pushTxn struct {
	SchemaVersion int    `json:"schemaVersion"`
	Goal          string `json:"goal"`
	Remote        string `json:"remote"`
	Ref           string `json:"ref"`
	Expected      string `json:"expected"`
	New           string `json:"new"`
}

func init() { pushProtocolAvailable = true }

func txnRef(opid string) string { return "refs/metasystem/goals/txn/" + opid }

func originTipRef(goalID string) string { return "refs/metasystem/goals/origin/" + goalID }

func fetchRef(opid string) string { return "refs/metasystem/goals/fetch/" + opid }

func writePushTxn(repo, opid string, txn pushTxn) error {
	encoded, err := json.Marshal(txn)
	if err != nil {
		return err
	}
	out, err := gitInput(repo, encoded, "hash-object", "-w", "--stdin")
	if err != nil {
		return err
	}
	_, err = gitOutput(repo, "update-ref", txnRef(opid), strings.TrimSpace(string(out)))
	return err
}

func readPushTxn(repo, ref string) (pushTxn, error) {
	out, err := gitOutput(repo, "cat-file", "blob", ref)
	if err != nil {
		return pushTxn{}, err
	}
	var txn pushTxn
	if err := json.Unmarshal(out, &txn); err != nil || txn.SchemaVersion != 1 || txn.Goal == "" || txn.Ref == "" || !hex40(txn.New) {
		return pushTxn{}, fmt.Errorf("goal branch transaction %s is malformed", ref)
	}
	return txn, nil
}

func clearPushTxn(repo, ref string) error {
	_, err := gitOutput(repo, "update-ref", "-d", ref)
	return err
}

func recordOriginTip(repo, goalID, tip string) error {
	_, err := gitOutput(repo, "update-ref", originTipRef(goalID), tip)
	return err
}

type fetchValidationDependencies struct {
	validateRange func(repo, endpointTip, tip, goalID string) ([]Commit, error)
	clearRef      func(repo, ref string) error
}

func fetchAndValidateWith(repo, remote, endpointTip, goalID, opid, tip string, transport PushTransport, deps fetchValidationDependencies) (err error) {
	temporary := fetchRef(opid)
	defer func() {
		if clearErr := deps.clearRef(repo, temporary); err == nil && clearErr != nil {
			err = clearErr
		}
	}()
	if err = transport.Fetch(repo, remote, goalBranchRef(goalID), temporary); err != nil {
		return err
	}
	_, err = deps.validateRange(repo, endpointTip, tip, goalID)
	return err
}

func ancestor(repo, older, newer string) (bool, error) {
	_, err := gitOutput(repo, "merge-base", "--is-ancestor", older, newer)
	if err == nil {
		return true, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return false, nil
	}
	return false, err
}

func updateBranchAndOrigin(repo, goalID, oldBranch, newBranch, newOrigin string) error {
	oldOrigin, present, err := localBranchTip(repo, originTipRef(goalID))
	if err != nil {
		return err
	}
	if !present {
		oldOrigin = strings.Repeat("0", 40)
	}
	if oldBranch == "" {
		oldBranch = strings.Repeat("0", 40)
	}
	input := fmt.Sprintf("start\nupdate %s %s %s\n", goalBranchRef(goalID), newBranch, oldBranch)
	if newOrigin != "" {
		input += fmt.Sprintf("update %s %s %s\n", originTipRef(goalID), newOrigin, oldOrigin)
	}
	input += "prepare\ncommit\n"
	_, err = gitInput(repo, []byte(input), "update-ref", "--stdin")
	return err
}

func adoptRemoteTipWithRepository(repo, goalID, localTip, remoteTip string, beforeRefMove, afterRefMove func() error, repository pushRepository) error {
	ref := goalBranchRef(goalID)
	current := repository.HeadRef(repo)
	if current == ref {
		clean, err := repository.TrackedClean(repo, false)
		if err != nil {
			return err
		}
		if !clean {
			return operationRefusal(StaleCode, "goal %s's branch moved on origin to %s, and this checkout has changes in its way\nrun: metasystem work status %s", goalID, remoteTip, goalID)
		}
		clean, err = repository.TrackedClean(repo, true)
		if err != nil {
			return err
		}
		if !clean {
			return operationRefusal(StaleCode, "goal %s's branch moved on origin to %s, and this checkout has changes in its way\nrun: metasystem work status %s", goalID, remoteTip, goalID)
		}
		if err := repository.Detach(repo, remoteTip); err != nil {
			return err
		}
	}
	if beforeRefMove != nil {
		if err := beforeRefMove(); err != nil {
			if current == ref {
				restoreErr := repository.RestoreHead(repo, ref)
				return errors.Join(err, restoreErr)
			}
			return err
		}
	}
	if err := repository.MoveRefs(repo, goalID, localTip, remoteTip, remoteTip); err != nil {
		if current == ref {
			restoreErr := repository.RestoreHead(repo, ref)
			return errors.Join(err, restoreErr)
		}
		return err
	}
	if afterRefMove != nil {
		if err := afterRefMove(); err != nil {
			return err
		}
	}
	if current == ref {
		return repository.RestoreHead(repo, ref)
	}
	if localTip == "" {
		if err := repository.SwitchGoal(repo, goalID); err != nil {
			return err
		}
	}
	return nil
}

func staleBranch(goalID, remote, localTip, remoteTip, originTip string) error {
	if remoteTip == "" {
		remoteTip = "<absent>"
	}
	if originTip == "" {
		originTip = "<absent>"
	}
	return operationRefusal(StaleCode, "goal %s's branch here (%s) was based on %s, but %s now holds %s\nrun: metasystem work status %s", goalID, localTip, originTip, remote, remoteTip, goalID)
}

func reconcilePushTransactionsWithRepository(req PushRequest, repository pushRepository) (*PushResult, error) {
	refs, err := repository.TxnRefs(req.Repo)
	if err != nil {
		return nil, err
	}
	for _, ref := range refs {
		txn, err := repository.Txn(req.Repo, ref)
		if err != nil {
			return nil, err
		}
		if txn.Goal != req.GoalID || txn.Remote != req.Remote {
			continue
		}
		remoteTip, present, err := req.Transport.RemoteTip(req.Repo, req.Remote, txn.Ref)
		if err != nil {
			return nil, err
		}
		if !present {
			remoteTip = ""
		}
		switch remoteTip {
		case txn.New:
			if err := repository.RecordOrigin(req.Repo, req.GoalID, txn.New); err != nil {
				return nil, err
			}
			if err := repository.ClearRef(req.Repo, ref); err != nil {
				return nil, err
			}
			if err := checkClaim(req.CheckClaim); err != nil {
				return nil, operationRefusal(ClaimLostCode, "%s; the pushed commit %s stays on goal %s's branch\nrun: metasystem goal claim %s", firstLine(err), txn.New, req.GoalID, req.GoalID)
			}
			return &PushResult{State: "reconciled", Tip: txn.New}, nil
		case txn.Expected:
			if err := repository.ClearRef(req.Repo, ref); err != nil {
				return nil, err
			}
			return nil, operationRefusal(PushUnknownCode, "the earlier push of goal %s's branch didn't arrive: %s still holds %s, not %s\nrun: metasystem work status %s", req.GoalID, req.Remote, remoteTip, txn.New, req.GoalID)
		default:
			if present {
				if err := fetchAndValidateFromRepository(req, remoteTip, repository); err != nil {
					return nil, err
				}
				landed, err := repository.Ancestor(req.Repo, txn.New, remoteTip)
				if err != nil {
					return nil, err
				}
				if landed {
					if err := repository.RecordOrigin(req.Repo, req.GoalID, remoteTip); err != nil {
						return nil, err
					}
					if err := repository.ClearRef(req.Repo, ref); err != nil {
						return nil, err
					}
					if err := checkClaim(req.CheckClaim); err != nil {
						return nil, operationRefusal(ClaimLostCode, "%s; the pushed commit %s stays on goal %s's branch\nrun: metasystem goal claim %s", firstLine(err), txn.New, req.GoalID, req.GoalID)
					}
					return &PushResult{State: "reconciled", Tip: remoteTip}, nil
				}
			}
			if err := repository.ClearRef(req.Repo, ref); err != nil {
				return nil, err
			}
			return nil, operationRefusal(LeaseMovedCode, "goal %s's branch moved on %s from %s to %s\nrun: metasystem work status %s", req.GoalID, req.Remote, txn.Expected, remoteTip, req.GoalID)
		}
	}
	return nil, nil
}

func Push(req PushRequest) (PushResult, error) {
	return pushWithRepository(req, gitPushRepository())
}

func fetchAndValidateFromRepository(req PushRequest, tip string, repository pushRepository) error {
	return fetchAndValidateWith(req.Repo, req.Remote, req.EndpointTip, req.GoalID, req.OpID, tip, req.Transport,
		fetchValidationDependencies{validateRange: repository.Range, clearRef: repository.ClearRef})
}

func pushWithRepository(req PushRequest, repository pushRepository) (PushResult, error) {
	if req.Transport == nil {
		req.Transport = GitPushTransport{}
	}
	if !validName(req.GoalID) || !validName(req.OpID) || req.Remote == "" {
		return PushResult{}, fmt.Errorf("push needs a goal, operation id, and remote")
	}
	if err := checkClaim(req.CheckClaim); err != nil {
		return PushResult{}, err
	}
	if recovered, err := reconcilePushTransactionsWithRepository(req, repository); err != nil || recovered != nil {
		if recovered != nil {
			return *recovered, err
		}
		return PushResult{}, err
	}
	ref := goalBranchRef(req.GoalID)
	remoteTip, remotePresent, err := req.Transport.RemoteTip(req.Repo, req.Remote, ref)
	if err != nil {
		return PushResult{}, err
	}
	localTip, localPresent, err := repository.Tip(req.Repo, ref)
	if err != nil {
		return PushResult{}, err
	}
	if !localPresent {
		if !remotePresent {
			return PushResult{}, fmt.Errorf("goal branch %s has no local or remote tip", ref)
		}
		if err := fetchAndValidateFromRepository(req, remoteTip, repository); err != nil {
			return PushResult{}, err
		}
		if err := adoptRemoteTipWithRepository(req.Repo, req.GoalID, "", remoteTip, req.Hooks.BeforeAdoptionRefMove, req.Hooks.AfterAdoptionRefMove, repository); err != nil {
			return PushResult{}, err
		}
		return PushResult{State: "adopted", Tip: remoteTip}, nil
	}
	if _, err := repository.Range(req.Repo, req.EndpointTip, localTip, req.GoalID); err != nil {
		return PushResult{}, err
	}
	if remotePresent && remoteTip == localTip {
		if err := repository.RecordOrigin(req.Repo, req.GoalID, remoteTip); err != nil {
			return PushResult{}, err
		}
		return PushResult{State: "current", Tip: localTip}, nil
	}
	originTip, originPresent, err := repository.Tip(req.Repo, originTipRef(req.GoalID))
	if err != nil {
		return PushResult{}, err
	}
	if remotePresent {
		if err := fetchAndValidateFromRepository(req, remoteTip, repository); err != nil {
			return PushResult{}, err
		}
		remoteBuiltOnLocal, err := repository.Ancestor(req.Repo, localTip, remoteTip)
		if err != nil {
			return PushResult{}, err
		}
		if remoteBuiltOnLocal {
			if err := adoptRemoteTipWithRepository(req.Repo, req.GoalID, localTip, remoteTip, req.Hooks.BeforeAdoptionRefMove, req.Hooks.AfterAdoptionRefMove, repository); err != nil {
				return PushResult{}, err
			}
			return PushResult{State: "adopted", Tip: remoteTip}, nil
		}
		builtOnRemote, err := repository.Ancestor(req.Repo, remoteTip, localTip)
		if err != nil {
			return PushResult{}, err
		}
		if !builtOnRemote && !(originPresent && remoteTip == originTip) {
			if originPresent && localTip == originTip {
				if err := adoptRemoteTipWithRepository(req.Repo, req.GoalID, localTip, remoteTip, req.Hooks.BeforeAdoptionRefMove, req.Hooks.AfterAdoptionRefMove, repository); err != nil {
					return PushResult{}, err
				}
				return PushResult{State: "adopted", Tip: remoteTip}, nil
			}
			return PushResult{}, staleBranch(req.GoalID, req.Remote, localTip, remoteTip, originTip)
		}
	} else if originPresent {
		return PushResult{}, staleBranch(req.GoalID, req.Remote, localTip, "", originTip)
	}
	expected := remoteTip
	if !remotePresent {
		expected = ""
	}
	txn := pushTxn{SchemaVersion: 1, Goal: req.GoalID, Remote: req.Remote, Ref: ref, Expected: expected, New: localTip}
	if err := repository.WriteTxn(req.Repo, req.OpID, txn); err != nil {
		return PushResult{}, err
	}
	if req.Hooks.AfterRemoteRead != nil {
		if err := req.Hooks.AfterRemoteRead(); err != nil {
			return PushResult{}, err
		}
	}
	outcome, pushErr := req.Transport.Push(req.Repo, req.Remote, ref, expected, localTip)
	if outcome == CASRefused {
		if err := repository.ClearRef(req.Repo, txnRef(req.OpID)); err != nil {
			return PushResult{}, err
		}
		return PushResult{}, operationRefusal(LeaseMovedCode, "goal %s's branch moved on origin while it was pushed: %v\nrun: metasystem work status %s", req.GoalID, pushErr, req.GoalID)
	}
	if req.Hooks.AfterPush != nil {
		if err := req.Hooks.AfterPush(); err != nil {
			return PushResult{}, err
		}
	}
	if outcome == CASUnknown {
		observed, present, err := req.Transport.RemoteTip(req.Repo, req.Remote, ref)
		if err != nil {
			return PushResult{}, operationRefusal(PushUnknownCode, "%v; reading origin back failed too (%v)\nrun: metasystem work status %s", pushErr, err, req.GoalID)
		}
		if !present {
			observed = ""
		}
		if observed != localTip {
			if err := repository.ClearRef(req.Repo, txnRef(req.OpID)); err != nil {
				return PushResult{}, err
			}
			if observed != expected {
				return PushResult{}, operationRefusal(LeaseMovedCode, "origin holds %s for goal %s's branch, not the pushed %s\nrun: metasystem work status %s", observed, req.GoalID, localTip, req.GoalID)
			}
			return PushResult{}, operationRefusal(PushUnknownCode, "the push of goal %s's branch may have failed: origin still holds %s\nrun: metasystem work status %s", req.GoalID, observed, req.GoalID)
		}
	}
	if err := repository.RecordOrigin(req.Repo, req.GoalID, localTip); err != nil {
		return PushResult{}, err
	}
	if err := repository.ClearRef(req.Repo, txnRef(req.OpID)); err != nil {
		return PushResult{}, err
	}
	if err := checkClaim(req.CheckClaim); err != nil {
		return PushResult{}, operationRefusal(ClaimLostCode, "%s; the pushed commit %s stays on goal %s's branch\nrun: metasystem goal claim %s", firstLine(err), localTip, req.GoalID, req.GoalID)
	}
	return PushResult{State: "pushed", Tip: localTip}, nil
}

// LocalTrackingTransport answers a remote's refs from this repository's
// remote-tracking refs alone (refs/remotes/<remote>/<branch>), reading no
// network: the disk sweeper's plan (Round D3 N2). A branch whose
// remote-tracking ref is absent is not known locally, which is an error the
// plan keeps the worktree for, never an absence. It fetches and pushes
// nothing.
type LocalTrackingTransport struct{ Context context.Context }

// ErrRemoteNotKnownLocally is a remote ref no local remote-tracking ref
// answers for.
var ErrRemoteNotKnownLocally = errors.New("the remote's state is not known locally")

func (t LocalTrackingTransport) RemoteTip(repo, remote, ref string) (string, bool, error) {
	ctx := t.Context
	if ctx == nil {
		ctx = context.Background()
	}
	branch, ok := strings.CutPrefix(ref, "refs/heads/")
	if !ok {
		return "", false, fmt.Errorf("%s is not a branch", ref)
	}
	tracking := "refs/remotes/" + remote + "/" + branch
	out, err := gitOutputContext(ctx, repo, "rev-parse", "--verify", "--quiet", tracking+"^{commit}")
	if err != nil {
		if ctx.Err() != nil {
			return "", false, ctx.Err()
		}
		return "", false, fmt.Errorf("%s: %w (no %s; git fetch %s, then the next pass judges it)", ref, ErrRemoteNotKnownLocally, tracking, remote)
	}
	return strings.TrimSpace(string(out)), true, nil
}

func (LocalTrackingTransport) Fetch(string, string, string, string) error {
	return errors.New("a local-refs plan fetches nothing")
}

func (LocalTrackingTransport) Push(string, string, string, string, string) (CASOutcome, error) {
	return CASUnknown, errors.New("a local-refs plan pushes nothing")
}
