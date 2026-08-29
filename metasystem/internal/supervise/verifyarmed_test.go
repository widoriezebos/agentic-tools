package supervise

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// armedFixture lays out agents/supervision with a state, two component
// heartbeats, and a census verdict, all owned by THIS live test process
// (the fake identity table pins its start to 100). Component tags are
// empty: tag semantics live in identity.TagState's own tests.
func armedFixture(t *testing.T, mutate func(state, last, hbWatcher, hbReaper map[string]any)) (string, int64) {
	t.Helper()
	repo := watchdogRepo(t)
	agents := filepath.Join(repo, "artifacts", "agents")
	supervision := filepath.Join(agents, "supervision")
	self := int64(os.Getpid())

	hbW := map[string]any{"observedAtEpoch": watchdogNow - 5, "loadedCapMin": 330}
	hbR := map[string]any{"observedAtEpoch": watchdogNow - 5}
	state := map[string]any{
		"fingerprint": "fp-1", "generation": 3, "derivedWatcherCapMin": 330,
		"components": map[string]any{
			"watcher": map[string]any{"pid": self, "pidStartedAt": 100, "instanceTag": "",
				"heartbeat": filepath.Join(supervision, "hb-watcher.json")},
			"reaper": map[string]any{"pid": self, "pidStartedAt": 100, "instanceTag": "",
				"heartbeat": filepath.Join(supervision, "hb-reaper.json")},
		},
	}
	last := map[string]any{
		"verdict": "SUCCESS", "fingerprint": "fp-1", "generation": 3,
		"completedAtEpoch": watchdogNow - 10,
	}
	if mutate != nil {
		mutate(state, last, hbW, hbR)
	}
	for name, value := range map[string]map[string]any{
		"state.json": state, "last-census.json": last,
		"hb-watcher.json": hbW, "hb-reaper.json": hbR,
	} {
		writeSupervisionFile(t, repo, name, jsonLine(t, value))
	}
	return agents, self
}

func jsonLine(t *testing.T, value map[string]any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestArmedNowVerdicts(t *testing.T) {
	now := time.Unix(watchdogNow, 0)
	t.Run("healthy arming verifies", func(t *testing.T) {
		agents, self := armedFixture(t, nil)
		if !ArmedNow(agents, self, 100, "", 60, now) {
			t.Fatal("a live, fresh, matching fleet must verify")
		}
	})
	cases := []struct {
		name   string
		mutate func(state, last, hbW, hbR map[string]any)
	}{
		{"failed census refuses", func(_, last, _, _ map[string]any) { last["verdict"] = "CENSUS-FAILED" }},
		{"fingerprint drift refuses", func(_, last, _, _ map[string]any) { last["fingerprint"] = "fp-OLD" }},
		{"generation drift refuses", func(_, last, _, _ map[string]any) { last["generation"] = 2 }},
		{"stale census refuses", func(_, last, _, _ map[string]any) { last["completedAtEpoch"] = watchdogNow - 500 }},
		{"stale watcher heartbeat refuses", func(_, _, hbW, _ map[string]any) { hbW["observedAtEpoch"] = watchdogNow - 500 }},
		{"ceiling mismatch refuses", func(_, _, hbW, _ map[string]any) { hbW["loadedCapMin"] = 300 }},
		{"dead reaper refuses", func(state, _, _, _ map[string]any) {
			components := state["components"].(map[string]any)
			components["reaper"].(map[string]any)["pid"] = int64(999999)
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			agents, self := armedFixture(t, c.mutate)
			if ArmedNow(agents, self, 100, "", 60, now) {
				t.Fatal("must not verify")
			}
		})
	}
	t.Run("dead owner refuses", func(t *testing.T) {
		agents, _ := armedFixture(t, nil)
		inspection := InspectArmed(agents, 999999, 100, "", 60, now)
		if inspection.Armed() {
			t.Fatal("a dead owner must not verify")
		}
		if inspection.Component != "supervision-owner" {
			t.Fatalf("dead owner failure was attributed to %q", inspection.Component)
		}
	})
	t.Run("missing component entry refuses", func(t *testing.T) {
		agents, self := armedFixture(t, func(state, _, _, _ map[string]any) {
			delete(state["components"].(map[string]any), "reaper")
		})
		inspection := InspectArmed(agents, self, 100, "", 60, now)
		if inspection.Armed() {
			t.Fatal("a fleet without a reaper must not verify")
		}
		if inspection.Component != "job-reaper" {
			t.Fatalf("missing reaper failure was attributed to %q", inspection.Component)
		}
	})
	t.Run("unreadable state refuses", func(t *testing.T) {
		agents, self := armedFixture(t, nil)
		if err := os.WriteFile(filepath.Join(agents, "supervision", "state.json"), []byte("{broken"), 0o644); err != nil {
			t.Fatal(err)
		}
		if ArmedNow(agents, self, 100, "", 60, now) {
			t.Fatal("an unparsable state must not verify")
		}
	})
	t.Run("a component tag provably absent from argv refuses", func(t *testing.T) {
		agents, self := armedFixture(t, func(state, _, _, _ map[string]any) {
			components := state["components"].(map[string]any)
			components["watcher"].(map[string]any)["instanceTag"] = "no-such-tag-xyzzy"
		})
		if ArmedNow(agents, self, 100, "", 60, now) {
			t.Fatal("a stale component tag must not verify")
		}
	})
}
