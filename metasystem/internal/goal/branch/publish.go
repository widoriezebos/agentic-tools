package branch

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

const LandTrunkMovedCode = "GOAL_LAND_TRUNK_MOVED"

type LandPushHooks struct {
	AfterRemoteRead func() error
}

type LandPushRequest struct {
	Repo, Remote, EndpointRef, GoalID, Prepared string
	CheckClaim                                  func() error
	Hooks                                       LandPushHooks
}

type PreparedLanding struct {
	Endpoint, Candidate, Landing, Branch string
}

func ReadPreparedLanding(dir string) (PreparedLanding, error) {
	data, err := os.ReadFile(filepath.Join(dir, "trunk"))
	if err != nil {
		return PreparedLanding{}, err
	}
	values := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok || values[key] != "" {
			return PreparedLanding{}, fmt.Errorf("prepared landing trunk is malformed")
		}
		values[key] = value
	}
	prepared := PreparedLanding{Endpoint: values["endpoint"], Candidate: values["candidate"], Landing: values["landing"], Branch: values["branch"]}
	if !hex40(prepared.Endpoint) || !hex40(prepared.Candidate) || !hex40(prepared.Landing) || prepared.Branch == "" {
		return PreparedLanding{}, fmt.Errorf("prepared landing trunk is incomplete")
	}
	return prepared, nil
}

func pushLandingAtomically(req LandPushRequest, prepared PreparedLanding) (CASOutcome, error) {
	landingRef := "refs/heads/" + prepared.Branch
	cmd := exec.Command("git", "-C", req.Repo, "push", "--atomic", req.Remote,
		"--force-with-lease="+req.EndpointRef+":"+prepared.Endpoint,
		"--force-with-lease="+landingRef+":"+prepared.Landing,
		prepared.Landing+":"+req.EndpointRef, ":"+landingRef)
	cmd.Env = gittree.ScrubbedEnviron("LC_ALL=C")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		if goal.ClassifyPushFailure(stdout.String()+stderr.String()) == goal.CASRefused {
			return CASRefused, fmt.Errorf("git push: %s: %w", strings.TrimSpace(stderr.String()), err)
		}
		return CASUnknown, fmt.Errorf("git push: %s: %w", strings.TrimSpace(stderr.String()), err)
	}
	return CASLanded, nil
}

func LandPush(req LandPushRequest) (PreparedLanding, error) {
	return landPushWithRepository(req, gitLandPushRepository())
}

func landPushWithRepository(req LandPushRequest, repository landPushRepository) (PreparedLanding, error) {
	if req.Repo == "" || req.Remote == "" || req.EndpointRef == "" || !validName(req.GoalID) || req.Prepared == "" {
		return PreparedLanding{}, fmt.Errorf("land-push needs a repository, remote, endpoint, goal, and prepared directory")
	}
	if err := checkClaim(req.CheckClaim); err != nil {
		return PreparedLanding{}, err
	}
	prepared, err := ReadPreparedLanding(req.Prepared)
	if err != nil {
		return PreparedLanding{}, err
	}
	if prepared.Branch != "landing/"+req.GoalID {
		return PreparedLanding{}, fmt.Errorf("prepared landing branch %s does not belong to goal %s", prepared.Branch, req.GoalID)
	}
	endpoint, present, err := repository.RemoteTip(req.Repo, req.Remote, req.EndpointRef)
	if err != nil || !present {
		return PreparedLanding{}, operationRefusal(LandTrunkMovedCode, "endpoint holds %s, not prepared tip %s", endpoint, prepared.Endpoint)
	}
	landingRef := "refs/heads/" + prepared.Branch
	landingTip, present, err := repository.RemoteTip(req.Repo, req.Remote, landingRef)
	if err == nil && endpoint == prepared.Landing && !present {
		return prepared, nil
	}
	if endpoint != prepared.Endpoint {
		return PreparedLanding{}, operationRefusal(LandTrunkMovedCode, "endpoint holds %s, not prepared tip %s", endpoint, prepared.Endpoint)
	}
	if err != nil || !present || landingTip != prepared.Landing {
		return PreparedLanding{}, operationRefusal(LandBranchMovedCode, "landing branch holds %s, not prepared tip %s", landingTip, prepared.Landing)
	}
	fetched := "refs/metasystem/goals/landing-push/" + req.GoalID
	defer repository.Clear(req.Repo, fetched)
	if err := repository.Fetch(req.Repo, req.Remote, landingRef, fetched); err != nil {
		return PreparedLanding{}, err
	}
	if err := repository.Ancestor(req.Repo, prepared.Endpoint, prepared.Landing); err != nil {
		return PreparedLanding{}, operationRefusal(LandTrunkMovedCode, "prepared landing %s is not a fast-forward of %s", prepared.Landing, prepared.Endpoint)
	}
	if req.Hooks.AfterRemoteRead != nil {
		if err := req.Hooks.AfterRemoteRead(); err != nil {
			return PreparedLanding{}, err
		}
	}
	if err := checkClaim(req.CheckClaim); err != nil {
		return PreparedLanding{}, err
	}
	outcome, pushErr := repository.Publish(req, prepared)
	if outcome == CASRefused {
		endpointNow, _, _ := repository.RemoteTip(req.Repo, req.Remote, req.EndpointRef)
		landingNow, landingPresent, _ := repository.RemoteTip(req.Repo, req.Remote, landingRef)
		if endpointNow != prepared.Endpoint {
			return PreparedLanding{}, operationRefusal(LandTrunkMovedCode, "endpoint moved from %s to %s", prepared.Endpoint, endpointNow)
		}
		if !landingPresent || landingNow != prepared.Landing {
			return PreparedLanding{}, operationRefusal(LandBranchMovedCode, "landing branch moved from %s to %s", prepared.Landing, landingNow)
		}
		return PreparedLanding{}, operationRefusal(LandTrunkMovedCode, "atomic landing lease refused: %v", pushErr)
	}
	if outcome == CASUnknown {
		endpointNow, _, endpointErr := repository.RemoteTip(req.Repo, req.Remote, req.EndpointRef)
		_, landingPresent, landingErr := repository.RemoteTip(req.Repo, req.Remote, landingRef)
		if endpointErr != nil || landingErr != nil || endpointNow != prepared.Landing || landingPresent {
			return PreparedLanding{}, operationRefusal(PushUnknownCode, "landing outcome is unknown: %v", pushErr)
		}
	}
	return prepared, nil
}
