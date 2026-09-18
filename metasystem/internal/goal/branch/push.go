package branch

import (
	"bytes"
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

type GitPushTransport struct{}

func (GitPushTransport) RemoteTip(repo, remote, ref string) (string, bool, error) {
	out, err := gitOutput(repo, "ls-remote", "--refs", remote, ref)
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

func (GitPushTransport) Fetch(repo, remote, ref, destination string) error {
	_, err := gitOutput(repo, "fetch", "--no-tags", "--refmap=", remote, "+"+ref+":"+destination)
	return err
}

func (GitPushTransport) Push(repo, remote, ref, expected, tip string) (CASOutcome, error) {
	cmd := exec.Command("git", "-C", repo, "push", remote,
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

func fetchAndValidate(repo, remote, endpointTip, goalID, opid, tip string, transport PushTransport) (err error) {
	temporary := fetchRef(opid)
	defer func() {
		if clearErr := clearPushTxn(repo, temporary); err == nil && clearErr != nil {
			err = clearErr
		}
	}()
	if err = transport.Fetch(repo, remote, goalBranchRef(goalID), temporary); err != nil {
		return err
	}
	_, err = ValidateRange(repo, endpointTip, tip, goalID)
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

func adoptRemoteTip(repo, goalID, localTip, remoteTip string, beforeRefMove, afterRefMove func() error) error {
	ref := goalBranchRef(goalID)
	currentOut, _ := gitOutput(repo, "symbolic-ref", "-q", "HEAD")
	current := strings.TrimSpace(string(currentOut))
	if current == ref {
		_, err := gitOutput(repo, "diff", "--quiet")
		if err != nil {
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 1 {
				return err
			}
			return operationRefusal(StaleCode, "local tip %s cannot adopt remote tip %s while the checkout has tracked changes", localTip, remoteTip)
		}
		_, err = gitOutput(repo, "diff", "--cached", "--quiet")
		if err != nil {
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 1 {
				return err
			}
			return operationRefusal(StaleCode, "local tip %s cannot adopt remote tip %s while the checkout has tracked changes", localTip, remoteTip)
		}
		if _, err := gitOutput(repo, "switch", "--quiet", "--detach", remoteTip); err != nil {
			return err
		}
	}
	if beforeRefMove != nil {
		if err := beforeRefMove(); err != nil {
			if current == ref {
				_, restoreErr := gitOutput(repo, "symbolic-ref", "HEAD", ref)
				return errors.Join(err, restoreErr)
			}
			return err
		}
	}
	if err := updateBranchAndOrigin(repo, goalID, localTip, remoteTip, remoteTip); err != nil {
		if current == ref {
			_, restoreErr := gitOutput(repo, "symbolic-ref", "HEAD", ref)
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
		_, err := gitOutput(repo, "symbolic-ref", "HEAD", ref)
		return err
	}
	if localTip == "" {
		if _, err := gitOutput(repo, "switch", "--quiet", "goal/"+goalID); err != nil {
			return err
		}
	}
	return nil
}

func staleBranch(remote, localTip, remoteTip, originTip string) error {
	if remoteTip == "" {
		remoteTip = "<absent>"
	}
	if originTip == "" {
		originTip = "<absent>"
	}
	return operationRefusal(StaleCode, "local tip %s was based on %s, but %s holds %s", localTip, originTip, remote, remoteTip)
}

func reconcilePushTransactions(req PushRequest) (*PushResult, error) {
	out, err := gitOutput(req.Repo, "for-each-ref", "--format=%(refname)", "refs/metasystem/goals/txn/")
	if err != nil {
		return nil, err
	}
	for _, ref := range strings.Fields(string(out)) {
		txn, err := readPushTxn(req.Repo, ref)
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
			if err := recordOriginTip(req.Repo, req.GoalID, txn.New); err != nil {
				return nil, err
			}
			if err := clearPushTxn(req.Repo, ref); err != nil {
				return nil, err
			}
			if err := checkClaim(req.CheckClaim); err != nil {
				return nil, operationRefusal(ClaimLostCode, "%v; pushed tip %s stands", err, txn.New)
			}
			return &PushResult{State: "reconciled", Tip: txn.New}, nil
		case txn.Expected:
			if err := clearPushTxn(req.Repo, ref); err != nil {
				return nil, err
			}
			return nil, operationRefusal(PushUnknownCode, "%s still holds %s after the prepared push of %s", req.Remote, remoteTip, txn.New)
		default:
			if present {
				if err := fetchAndValidate(req.Repo, req.Remote, req.EndpointTip, req.GoalID, req.OpID, remoteTip, req.Transport); err != nil {
					return nil, err
				}
				landed, err := ancestor(req.Repo, txn.New, remoteTip)
				if err != nil {
					return nil, err
				}
				if landed {
					if err := recordOriginTip(req.Repo, req.GoalID, remoteTip); err != nil {
						return nil, err
					}
					if err := clearPushTxn(req.Repo, ref); err != nil {
						return nil, err
					}
					if err := checkClaim(req.CheckClaim); err != nil {
						return nil, operationRefusal(ClaimLostCode, "%v; pushed tip %s stands", err, txn.New)
					}
					return &PushResult{State: "reconciled", Tip: remoteTip}, nil
				}
			}
			if err := clearPushTxn(req.Repo, ref); err != nil {
				return nil, err
			}
			return nil, operationRefusal(LeaseMovedCode, "%s moved from expected tip %s to %s", req.Remote, txn.Expected, remoteTip)
		}
	}
	return nil, nil
}

func Push(req PushRequest) (PushResult, error) {
	if req.Transport == nil {
		req.Transport = GitPushTransport{}
	}
	if !validName(req.GoalID) || !validName(req.OpID) || req.Remote == "" {
		return PushResult{}, fmt.Errorf("push needs a goal, operation id, and remote")
	}
	if err := checkClaim(req.CheckClaim); err != nil {
		return PushResult{}, err
	}
	if recovered, err := reconcilePushTransactions(req); err != nil || recovered != nil {
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
	localTip, localPresent, err := localBranchTip(req.Repo, ref)
	if err != nil {
		return PushResult{}, err
	}
	if !localPresent {
		if !remotePresent {
			return PushResult{}, fmt.Errorf("goal branch %s has no local or remote tip", ref)
		}
		if err := fetchAndValidate(req.Repo, req.Remote, req.EndpointTip, req.GoalID, req.OpID, remoteTip, req.Transport); err != nil {
			return PushResult{}, err
		}
		if err := adoptRemoteTip(req.Repo, req.GoalID, "", remoteTip, req.Hooks.BeforeAdoptionRefMove, req.Hooks.AfterAdoptionRefMove); err != nil {
			return PushResult{}, err
		}
		return PushResult{State: "adopted", Tip: remoteTip}, nil
	}
	if _, err := ValidateRange(req.Repo, req.EndpointTip, localTip, req.GoalID); err != nil {
		return PushResult{}, err
	}
	if remotePresent && remoteTip == localTip {
		if err := recordOriginTip(req.Repo, req.GoalID, remoteTip); err != nil {
			return PushResult{}, err
		}
		return PushResult{State: "current", Tip: localTip}, nil
	}
	originTip, originPresent, err := localBranchTip(req.Repo, originTipRef(req.GoalID))
	if err != nil {
		return PushResult{}, err
	}
	if remotePresent {
		if err := fetchAndValidate(req.Repo, req.Remote, req.EndpointTip, req.GoalID, req.OpID, remoteTip, req.Transport); err != nil {
			return PushResult{}, err
		}
		remoteBuiltOnLocal, err := ancestor(req.Repo, localTip, remoteTip)
		if err != nil {
			return PushResult{}, err
		}
		if remoteBuiltOnLocal {
			if err := adoptRemoteTip(req.Repo, req.GoalID, localTip, remoteTip, req.Hooks.BeforeAdoptionRefMove, req.Hooks.AfterAdoptionRefMove); err != nil {
				return PushResult{}, err
			}
			return PushResult{State: "adopted", Tip: remoteTip}, nil
		}
		builtOnRemote, err := ancestor(req.Repo, remoteTip, localTip)
		if err != nil {
			return PushResult{}, err
		}
		if !builtOnRemote && !(originPresent && remoteTip == originTip) {
			if originPresent && localTip == originTip {
				if err := adoptRemoteTip(req.Repo, req.GoalID, localTip, remoteTip, req.Hooks.BeforeAdoptionRefMove, req.Hooks.AfterAdoptionRefMove); err != nil {
					return PushResult{}, err
				}
				return PushResult{State: "adopted", Tip: remoteTip}, nil
			}
			return PushResult{}, staleBranch(req.Remote, localTip, remoteTip, originTip)
		}
	} else if originPresent {
		return PushResult{}, staleBranch(req.Remote, localTip, "", originTip)
	}
	expected := remoteTip
	if !remotePresent {
		expected = ""
	}
	txn := pushTxn{SchemaVersion: 1, Goal: req.GoalID, Remote: req.Remote, Ref: ref, Expected: expected, New: localTip}
	if err := writePushTxn(req.Repo, req.OpID, txn); err != nil {
		return PushResult{}, err
	}
	if req.Hooks.AfterRemoteRead != nil {
		if err := req.Hooks.AfterRemoteRead(); err != nil {
			return PushResult{}, err
		}
	}
	outcome, pushErr := req.Transport.Push(req.Repo, req.Remote, ref, expected, localTip)
	if outcome == CASRefused {
		if err := clearPushTxn(req.Repo, txnRef(req.OpID)); err != nil {
			return PushResult{}, err
		}
		return PushResult{}, operationRefusal(LeaseMovedCode, "origin moved after tip %s was observed: %v", expected, pushErr)
	}
	if req.Hooks.AfterPush != nil {
		if err := req.Hooks.AfterPush(); err != nil {
			return PushResult{}, err
		}
	}
	if outcome == CASUnknown {
		observed, present, err := req.Transport.RemoteTip(req.Repo, req.Remote, ref)
		if err != nil {
			return PushResult{}, operationRefusal(PushUnknownCode, "%v; reconciliation fetch failed: %v", pushErr, err)
		}
		if !present {
			observed = ""
		}
		if observed != localTip {
			if err := clearPushTxn(req.Repo, txnRef(req.OpID)); err != nil {
				return PushResult{}, err
			}
			if observed != expected {
				return PushResult{}, operationRefusal(LeaseMovedCode, "origin holds %s instead of proposed tip %s", observed, localTip)
			}
			return PushResult{}, operationRefusal(PushUnknownCode, "origin still holds %s after an unknown push outcome", observed)
		}
	}
	if err := recordOriginTip(req.Repo, req.GoalID, localTip); err != nil {
		return PushResult{}, err
	}
	if err := clearPushTxn(req.Repo, txnRef(req.OpID)); err != nil {
		return PushResult{}, err
	}
	if err := checkClaim(req.CheckClaim); err != nil {
		return PushResult{}, operationRefusal(ClaimLostCode, "%v; pushed tip %s stands", err, localTip)
	}
	return PushResult{State: "pushed", Tip: localTip}, nil
}
