package proofrun

import (
	"context"
	"path/filepath"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// RetainedGroupIdentity states explicitly whether retained metadata proved an
// execution identity. The old revalidator's missing-metadata digest remains
// opaque to existing callers, but is never a known forecast identity.
type RetainedGroupIdentity struct {
	Identity string
	Known    bool
}

// RetainedGroupForecastObservation exposes the same newest-observation order
// used by ReusedTestResult. Duration is an observation of prior native work,
// not a prediction or a budget reservation.
type RetainedGroupForecastObservation struct {
	Status     string
	AttemptID  string
	DurationMS *int64
}

func ForecastRetainedGroupObservation(template TestResult, attempts []Attempt, id, identity string) RetainedGroupForecastObservation {
	observation, found := newestReuseObservation(template, attempts, id, identity, "")
	if !found {
		return RetainedGroupForecastObservation{Status: "missing"}
	}
	status := "failed"
	if observation.live {
		status = "live"
	} else if observation.passed {
		status = "reusable"
	}
	return RetainedGroupForecastObservation{Status: status, AttemptID: observation.attemptID,
		DurationMS: positiveDuration(observation.group.DurationMS)}
}

func RevalidateRetainedGroupIdentityFacts(ctx context.Context, request TestRunRequest, attempts []Attempt) (map[string]RetainedGroupIdentity, error) {
	identities, err := RevalidateRetainedGroupExecutionIdentities(ctx, request, attempts)
	if err != nil {
		return nil, err
	}
	facts := make(map[string]RetainedGroupIdentity, len(identities))
	for id, identity := range identities {
		missing := digestBytes([]byte("missing-retained-metadata\x00" + id + "\x00" + request.CandidateTree))
		facts[id] = RetainedGroupIdentity{Identity: identity, Known: identity != "" && identity != missing}
	}
	return facts, nil
}

// GroupConsumesCandidateEngine follows the same engine binding as the native
// execution identity. A missing retained engine digest makes only these
// groups' forecast identities unknown.
func GroupConsumesCandidateEngine(request TestRunRequest, group testpolicy.Group, detachedProjectRoot string) bool {
	if group.Adapter == "section" {
		return true
	}
	cwd := filepath.Join(detachedProjectRoot, filepath.FromSlash(group.CWD))
	return metaSystemStewardConsumesCandidateEngine(request, group, cwd)
}
