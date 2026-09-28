package branch

import "fmt"

type DeleteRequest struct {
	Repo, Remote, GoalID, Expected string
	CheckClaim                     func() error
	PushTransport                  PushTransport
	AfterRemoteRead                func() error
}

func Delete(req DeleteRequest) error {
	if req.PushTransport == nil {
		req.PushTransport = GitPushTransport{}
	}
	if !validName(req.GoalID) || req.Remote == "" || !hex40(req.Expected) {
		return fmt.Errorf("goal branch delete needs a goal, remote, and expected full object id")
	}
	if err := checkClaim(req.CheckClaim); err != nil {
		return err
	}
	ref := goalBranchRef(req.GoalID)
	observed, present, err := req.PushTransport.RemoteTip(req.Repo, req.Remote, ref)
	if err != nil {
		return err
	}
	if !present || observed != req.Expected {
		return operationRefusal(LeaseMovedCode, "%s holds %s, not deletable tip %s", req.Remote, observed, req.Expected)
	}
	if req.AfterRemoteRead != nil {
		if err := req.AfterRemoteRead(); err != nil {
			return err
		}
	}
	outcome, pushErr := req.PushTransport.Push(req.Repo, req.Remote, ref, req.Expected, "")
	if outcome == CASRefused {
		return operationRefusal(LeaseMovedCode, "%s moved before leased deletion: %v", req.Remote, pushErr)
	}
	if outcome == CASUnknown {
		observed, present, err := req.PushTransport.RemoteTip(req.Repo, req.Remote, ref)
		if err != nil || present {
			return operationRefusal(PushUnknownCode, "%s delete outcome is unknown; it now holds %s: %v", req.Remote, observed, pushErr)
		}
	}
	return nil
}
