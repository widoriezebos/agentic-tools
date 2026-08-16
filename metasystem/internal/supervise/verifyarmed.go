package supervise

import (
	"path/filepath"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// The arming success verdict (review script-orchestration-10, relocated
// from arm-supervision.sh): the same freshness judgment `job census-fresh`
// renders for dispatch, computed once. Before this, arming certified in
// shell what dispatch verified in Go, and the two had to move in lockstep
// by hand whenever the freshness rule changed.

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
// generation. One attempt, pure over the given clock; the arming script
// owns the retry loop and the timeout message.
func ArmedNow(agentsDir string, ownerPid, ownerStart int64, ownerTag string, intervalSec int64, now time.Time) bool {
	supervision := filepath.Join(agentsDir, "supervision")
	statePath := filepath.Join(supervision, "state.json")
	lastPath := filepath.Join(supervision, "last-census.json")
	// The fixture authority (agnosticism B1): root is two levels above
	// the agents dir, the reserved-cap fence's own derivation; a refused
	// construction refuses fixtures.
	var probe identity.FixtureProbe
	if authorization, err := fixtureauth.New(filepath.Dir(filepath.Dir(agentsDir))); err == nil {
		probe = authorization.Identity()
	}
	if !armedIdentityAlive(ownerPid, ownerStart, ownerTag, probe) {
		return false
	}
	state, err := readObjectFile(statePath)
	if err != nil {
		return false
	}
	last, err := readObjectFile(lastPath)
	if err != nil {
		return false
	}
	components, _ := state["components"].(map[string]any)
	for _, component := range []string{"watcher", "reaper"} {
		entry, _ := components[component].(map[string]any)
		if entry == nil {
			return false
		}
		pid, pidOK := intField(entry["pid"])
		start, startOK := intField(entry["pidStartedAt"])
		tag, _ := entry["instanceTag"].(string)
		if !pidOK || !startOK || !armedIdentityAlive(pid, start, tag, probe) {
			return false
		}
		heartbeatPath, _ := entry["heartbeat"].(string)
		heartbeat, err := readObjectFile(heartbeatPath)
		if err != nil {
			return false
		}
		observed, observedOK := intField(heartbeat["observedAtEpoch"])
		if !observedOK || now.Unix()-observed > intervalSec*2+2 {
			return false
		}
		if component == "watcher" {
			derived, derivedOK := intField(state["derivedWatcherCapMin"])
			loaded, loadedOK := intField(heartbeat["loadedCapMin"])
			if !derivedOK || derived < 1 || !loadedOK || loaded != derived {
				return false
			}
		}
	}
	verdict, _ := last["verdict"].(string)
	if verdict != "SUCCESS" {
		return false
	}
	expectedFP, _ := state["fingerprint"].(string)
	actualFP, _ := last["fingerprint"].(string)
	if expectedFP == "" || actualFP != expectedFP {
		return false
	}
	expectedGen, expectedGenOK := intField(state["generation"])
	actualGen, actualGenOK := intField(last["generation"])
	if !expectedGenOK || !actualGenOK || actualGen != expectedGen {
		return false
	}
	completed, completedOK := intField(last["completedAtEpoch"])
	return completedOK && now.Unix()-completed <= intervalSec
}
