package branch

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// PublishReadRequest publishes a collected Goal-Read attestation. The read
// record names the attestation and supplies the retry identity, so a repeat
// after a lost push reconciles the same push transaction instead of
// collecting or committing again.
type PublishReadRequest struct {
	Repo, Remote, EndpointTip, GoalID, UnitCommit string
	CheckClaim                                    func() error
	Transport                                     PushTransport
	Repository                                    BranchReadRepository
	Hooks                                         PushHooks
}

// PublishReadResult is the push owner's outcome and the remote goal-branch
// tip observed after it, which contains the attestation.
type PublishReadResult struct {
	Attestation, OpID, State, RemoteTip string
}

// PublishOperationID is the push operation a collected read publishes under.
func PublishOperationID(gateRunID string) string { return gateRunID + "-publish" }

func PublishCollectedRead(req PublishReadRequest) (PublishReadResult, error) {
	return publishCollectedReadWith(req, gitPushRepository())
}

func publishCollectedReadWith(req PublishReadRequest, pushes pushRepository) (PublishReadResult, error) {
	repository := branchReadRepositoryFor(req.Repository)
	_, recordPath, _, err := branchReadPathsWithRepository(repository, req.Repo, req.GoalID, req.UnitCommit)
	if err != nil {
		return PublishReadResult{}, err
	}
	lock, err := lockBranchRead(recordPath)
	if err != nil {
		return PublishReadResult{}, err
	}
	record, err := loadBranchReadRecord(recordPath)
	_ = unix.Flock(int(lock.Fd()), unix.LOCK_UN)
	_ = lock.Close()
	if err != nil {
		return PublishReadResult{}, err
	}
	if record.AttestationCommit == "" || record.GateRunID == "" || record.Goal != req.GoalID || record.UnitCommit != req.UnitCommit {
		return PublishReadResult{}, operationRefusal(ReadInvalidCode, "unit %s has no collected read attestation to publish", req.UnitCommit)
	}
	if req.Transport == nil {
		req.Transport = GitPushTransport{}
	}
	result := PublishReadResult{Attestation: record.AttestationCommit, OpID: PublishOperationID(record.GateRunID)}
	pushed, err := pushWithRepository(PushRequest{Repo: req.Repo, Remote: req.Remote, EndpointTip: req.EndpointTip, GoalID: req.GoalID,
		OpID: result.OpID, CheckClaim: req.CheckClaim, Transport: req.Transport, Hooks: req.Hooks}, pushes)
	result.State = pushed.State
	if err != nil {
		return result, err
	}
	tip, present, err := req.Transport.RemoteTip(req.Repo, req.Remote, goalBranchRef(req.GoalID))
	if err != nil {
		return result, err
	}
	if !present {
		return result, fmt.Errorf("%s has no goal/%s after publishing attestation %s", req.Remote, req.GoalID, result.Attestation)
	}
	result.RemoteTip = tip
	if tip != result.Attestation {
		contains, err := pushes.Ancestor(req.Repo, result.Attestation, tip)
		if err != nil {
			return result, err
		}
		if !contains {
			return result, fmt.Errorf("%s goal/%s at %s does not contain attestation %s", req.Remote, req.GoalID, tip, result.Attestation)
		}
	}
	return result, nil
}
