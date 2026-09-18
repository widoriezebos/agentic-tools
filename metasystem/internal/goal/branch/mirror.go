package branch

import "fmt"

type MirrorHooks struct {
	AfterTransportRead func() error
}

type MirrorRequest struct {
	Repo, Origin, Transport, EndpointTip, GoalID, OpID string
	CheckClaim                                         func() error
	PushTransport                                      PushTransport
	Hooks                                              MirrorHooks
}

func Mirror(req MirrorRequest) (PushResult, error) {
	if req.PushTransport == nil {
		req.PushTransport = GitPushTransport{}
	}
	if !validName(req.GoalID) || req.Origin == "" || req.Transport == "" || req.OpID == "" {
		return PushResult{}, fmt.Errorf("transport mirror needs a goal, origin, transport, and operation id")
	}
	if err := checkClaim(req.CheckClaim); err != nil {
		return PushResult{}, err
	}
	ref := goalBranchRef(req.GoalID)
	originTip, present, err := req.PushTransport.RemoteTip(req.Repo, req.Origin, ref)
	if err != nil {
		return PushResult{}, err
	}
	if !present {
		return PushResult{}, fmt.Errorf("origin has no %s", ref)
	}
	if err := fetchAndValidate(req.Repo, req.Origin, req.EndpointTip, req.GoalID, req.OpID, originTip, req.PushTransport); err != nil {
		return PushResult{}, err
	}
	transportTip, transportPresent, err := req.PushTransport.RemoteTip(req.Repo, req.Transport, ref)
	if err != nil {
		return PushResult{}, err
	}
	expected := ""
	if transportPresent {
		expected = transportTip
	}
	if req.Hooks.AfterTransportRead != nil {
		if err := req.Hooks.AfterTransportRead(); err != nil {
			return PushResult{}, err
		}
	}
	outcome, pushErr := req.PushTransport.Push(req.Repo, req.Transport, ref, expected, originTip)
	if outcome == CASRefused {
		return PushResult{}, operationRefusal(LeaseMovedCode, "transport moved after tip %s was observed: %v", expected, pushErr)
	}
	if outcome == CASUnknown {
		observed, observedPresent, err := req.PushTransport.RemoteTip(req.Repo, req.Transport, ref)
		if err != nil || !observedPresent || observed != originTip {
			return PushResult{}, operationRefusal(PushUnknownCode, "transport mirror outcome is unknown: %v", pushErr)
		}
	}
	return PushResult{State: "mirrored", Tip: originTip}, nil
}

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
