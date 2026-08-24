package missionrunner

// The provider-outage posture at the runner's host-exit ramp
// (provider-outage-posture, Wido 2026-08-24): a provider-overload host
// exit is nobody's failure — it marks the shared outage record, stays
// off the consecutive-host-failure breaker, and paces its retry — and
// any provider success clears the mark.

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/outage"
)

// A host that keeps exiting on the provider's 529 never parks
// host-failure: every failed turn carries feedsBreaker=false, the
// outage mark stands and grows, and the mission spends its cycles
// retrying instead of blaming the host.
func TestInternalRunOverloadedHostStaysOffTheBreaker(t *testing.T) {
	t.Setenv("METASYSTEM_FIXTURE_CAP_SCALE_MILLI", "10")
	engine := buildFullCycleRoot(t, "FAKEHOST:exit-overloaded")
	signal := filepath.Join(t.TempDir(), "start.json")
	code := engine.internalRun("start", "metasystem-mission-runner-alpha-fixture", signal)
	state, err := readJSONDoc(filepath.Join(engine.missionDir(), "state.json"))
	if err != nil {
		t.Fatalf("no state (rc=%d): %v", code, err)
	}
	status, _ := state["status"].(string)
	if status == "" || status == "running" {
		t.Fatalf("no terminal after overloaded turns: %q rc=%d", status, code)
	}
	if reason, _ := state["parkReason"].(string); reason == "host-failure" {
		t.Fatalf("a provider overload must never park host-failure: %v", state["parkReason"])
	}
	turnLog, _ := state["turnLog"].([]any)
	overloadedTurns := 0
	for _, raw := range turnLog {
		entry, _ := raw.(map[string]any)
		if entry == nil {
			continue
		}
		detail, _ := entry["detail"].(string)
		if !strings.Contains(detail, "provider-overloaded") {
			continue
		}
		overloadedTurns++
		if entry["feedsBreaker"] != false {
			t.Fatalf("an overloaded turn must not feed the breaker: %v", entry)
		}
	}
	if overloadedTurns < 2 {
		t.Fatalf("the fixture must witness repeated overloads staying unparked: %d turn(s)\n%v",
			overloadedTurns, turnLog)
	}
	mark, ok := outage.Read(engine.Root)
	if !ok || mark.LastClass != "overloaded" || mark.Source != "mission-runner" {
		t.Fatalf("the outage mark must record the provider's weather: %+v ok=%v", mark, ok)
	}
	if mark.ConsecutiveFailures < 2 {
		t.Fatalf("each overloaded turn feeds the mark: %+v", mark)
	}
	turns, _ := filepath.Glob(filepath.Join(engine.missionDir(), "turns", "*", "turn.json"))
	if len(turns) == 0 {
		t.Fatal("no turns ran")
	}
	first, _ := readJSONDoc(turns[0])
	if errField, _ := first["error"].(string); errField != "provider-overloaded" {
		t.Fatalf("the turn record names the overload: error=%q detail=%v", errField, first["detail"])
	}
}

// The provider's error document can ride home on a CLEAN exit: that
// shape must reach the same posture — off the breaker, mark fed —
// or the missed shape feeds the breaker the ruling exempted.
func TestInternalRunCleanExitOverloadDocumentStaysOffTheBreaker(t *testing.T) {
	t.Setenv("METASYSTEM_FIXTURE_CAP_SCALE_MILLI", "10")
	engine := buildFullCycleRoot(t, "FAKEHOST:overloaded-result")
	signal := filepath.Join(t.TempDir(), "start.json")
	code := engine.internalRun("start", "metasystem-mission-runner-alpha-fixture", signal)
	state, err := readJSONDoc(filepath.Join(engine.missionDir(), "state.json"))
	if err != nil {
		t.Fatalf("no state (rc=%d): %v", code, err)
	}
	if reason, _ := state["parkReason"].(string); reason == "host-failure" {
		t.Fatalf("an overload document behind a clean exit must never park host-failure: %v", reason)
	}
	turns, _ := filepath.Glob(filepath.Join(engine.missionDir(), "turns", "*", "turn.json"))
	if len(turns) == 0 {
		t.Fatal("no turns ran")
	}
	first, _ := readJSONDoc(turns[0])
	if errField, _ := first["error"].(string); errField != "provider-overloaded" {
		t.Fatalf("the clean-exit overload names itself: error=%q detail=%v", errField, first["detail"])
	}
	if mark, ok := outage.Read(engine.Root); !ok || mark.LastClass != "overloaded" {
		t.Fatalf("the document must feed the mark: %+v ok=%v", mark, ok)
	}
}

// Any provider success ends the outage: the clear-on-success half is
// witnessed by TestInternalRunFullCycle, which seeds a standing mark
// and asserts the happy path removed it — one mission run serves both
// concerns, because this package already runs close to the go test
// per-package time cap.
