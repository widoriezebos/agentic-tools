package supervise

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// ArmedInspection names the first supervision component that prevents an
// arming attempt from being certified. Component is empty when the complete
// owner, watcher, reaper, and census generation verifies.
type ArmedInspection struct {
	Component string
	Reason    string
}

// Armed reports whether the complete supervision generation verifies.
func (i ArmedInspection) Armed() bool { return i.Component == "" }

// The arming success verdict: the same freshness judgment `job census-fresh`
// renders for dispatch, computed once — arming and dispatch must agree
// on one freshness rule, so neither side carries its own copy.

// armedIdentityAlive is the arming ladder's liveness: the recorded (pid,
// start) must be live by the census's one-source rule, and a recorded tag
// must not be provably absent — an uninspectable argv is never proof
// (live and unknown pass; stale and dead fail).
func armedIdentityAlive(pid, start int64, tag string, probe identity.FixtureProbe) bool {
	if !census.Alive(pid, start, probe) {
		return false
	}
	if tag == "" {
		return true
	}
	switch identity.TagState(identity.KernelProber{}, pid, tag) {
	case "live", "unknown":
		return true
	}
	return false
}

// ArmedNow reports whether supervision is verifiably armed at this instant:
// a live owner, live watcher and reaper components with fresh heartbeats,
// the watcher's loaded cap matching the derived ceiling the state attests,
// and a fresh successful census carrying the state's fingerprint and
// generation. One attempt, pure over the given clock; `up` owns the retry
// loop and the typed failure outcome.
func ArmedNow(agentsDir string, ownerPid, ownerStart int64, ownerTag string, intervalSec int64, now time.Time) bool {
	return InspectArmed(agentsDir, ownerPid, ownerStart, ownerTag, intervalSec, now).Armed()
}

// InspectArmed applies the arming verdict once and identifies the first
// component that fails it. The ordering follows the dependency chain so the
// remedy points at the earliest fact that must be repaired.
func InspectArmed(agentsDir string, ownerPid, ownerStart int64, ownerTag string, intervalSec int64, now time.Time) ArmedInspection {
	root := filepath.Dir(filepath.Dir(agentsDir))
	return InspectArmedAt(agentsDir, root, ownerPid, ownerStart, ownerTag, intervalSec, now)
}

// InspectArmedAt reads supervision state from agentsDir and fixture authority
// from the installed metasystem root.
func InspectArmedAt(agentsDir, metasystemRoot string, ownerPid, ownerStart int64, ownerTag string, intervalSec int64, now time.Time) ArmedInspection {
	supervision := filepath.Join(agentsDir, "supervision")
	statePath := filepath.Join(supervision, "state.json")
	lastPath := filepath.Join(supervision, "last-census.json")
	// A refused fixture authority refuses verification instead of falling back
	// to kernel-only facts.
	authorization, authErr := fixtureauth.New(metasystemRoot)
	if authErr != nil {
		// A leaked fixture is a refusal of the VERIFICATION, not a
		// kernel-only fallback.
		return ArmedInspection{Component: "supervision-owner", Reason: authErr.Error()}
	}
	probe := identity.FixtureProbe(authorization.Identity())
	if !armedIdentityAlive(ownerPid, ownerStart, ownerTag, probe) {
		return ArmedInspection{Component: "supervision-owner", Reason: "the recorded owner identity is not live"}
	}
	state, err := readObjectFile(statePath)
	if err != nil {
		return ArmedInspection{Component: "supervision-owner", Reason: fmt.Sprintf("the supervision state is unreadable: %v", err)}
	}
	last, err := readObjectFile(lastPath)
	if err != nil {
		return ArmedInspection{Component: "repo-watcher", Reason: fmt.Sprintf("the watcher census is unreadable: %v", err)}
	}
	components, _ := state["components"].(map[string]any)
	for _, component := range []string{"watcher", "reaper"} {
		componentName := "repo-watcher"
		if component == "reaper" {
			componentName = "job-reaper"
		}
		entry, _ := components[component].(map[string]any)
		if entry == nil {
			return ArmedInspection{Component: componentName, Reason: "the component identity is absent from supervision state"}
		}
		pid, pidOK := intField(entry["pid"])
		start, startOK := intField(entry["pidStartedAt"])
		tag, _ := entry["instanceTag"].(string)
		if !pidOK || !startOK || !armedIdentityAlive(pid, start, tag, probe) {
			return ArmedInspection{Component: componentName, Reason: "the recorded component identity is not live"}
		}
		heartbeatPath, _ := entry["heartbeat"].(string)
		heartbeat, err := readObjectFile(heartbeatPath)
		if err != nil {
			return ArmedInspection{Component: componentName, Reason: fmt.Sprintf("the component heartbeat is unreadable: %v", err)}
		}
		observed, observedOK := intField(heartbeat["observedAtEpoch"])
		if !observedOK || now.Unix()-observed > intervalSec*2+2 {
			return ArmedInspection{Component: componentName, Reason: "the component heartbeat is stale"}
		}
		if component == "watcher" {
			derived, derivedOK := intField(state["derivedWatcherCapMin"])
			loaded, loadedOK := intField(heartbeat["loadedCapMin"])
			if !derivedOK || derived < 1 || !loadedOK || loaded != derived {
				return ArmedInspection{Component: componentName, Reason: "the watcher ceiling does not match the armed generation"}
			}
		}
	}
	verdict, _ := last["verdict"].(string)
	if verdict != "SUCCESS" {
		return ArmedInspection{Component: "repo-watcher", Reason: "the latest census did not succeed"}
	}
	expectedFP, _ := state["fingerprint"].(string)
	actualFP, _ := last["fingerprint"].(string)
	if expectedFP == "" || actualFP != expectedFP {
		return ArmedInspection{Component: "repo-watcher", Reason: "the latest census belongs to another engine generation"}
	}
	expectedGen, expectedGenOK := intField(state["generation"])
	actualGen, actualGenOK := intField(last["generation"])
	if !expectedGenOK || !actualGenOK || actualGen != expectedGen {
		return ArmedInspection{Component: "repo-watcher", Reason: "the latest census belongs to another supervision generation"}
	}
	completed, completedOK := intField(last["completedAtEpoch"])
	if !completedOK || now.Unix()-completed > intervalSec {
		return ArmedInspection{Component: "repo-watcher", Reason: "the latest successful census is stale"}
	}
	return ArmedInspection{}
}
