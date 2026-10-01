package proofrun

import ()

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
