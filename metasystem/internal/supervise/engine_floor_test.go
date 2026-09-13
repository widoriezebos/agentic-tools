package supervise

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/registry"
)

type engineFloorProber map[int64]identity.Liveness

func (p engineFloorProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	state := p[pid]
	if state == identity.Alive {
		return identity.Exact{Pid: pid, StartedAt: time.Unix(100, 0)}, state, nil
	}
	return identity.Exact{}, state, nil
}

func TestEngineFloorInspectsEveryConsumedSlotClass(t *testing.T) {
	registryHome := t.TempDir()
	t.Setenv("METASYSTEM_SUPERVISION_REGISTRY_HOME", registryHome)
	registryPath, err := registry.DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(registryPath), 0o755); err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	checkouts := map[string]string{}
	appendRow := func(row map[string]any) {
		t.Helper()
		payload, err := json.Marshal(row)
		if err != nil {
			t.Fatal(err)
		}
		if err := registry.AppendFrame(registryPath, payload); err != nil {
			t.Fatal(err)
		}
	}
	common := func(tag string) map[string]any {
		checkout := filepath.Join(registryHome, tag)
		checkouts[tag] = checkout
		if err := os.MkdirAll(filepath.Join(checkout, "artifacts", "agents", "supervision"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(checkout, "artifacts", "agents", "supervision", "state.json"), []byte(`{"engineBuild":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}`), 0o644); err != nil {
			t.Fatal(err)
		}
		return map[string]any{"schemaVersion": 1, "checkoutPath": checkout, "ownerTag": tag, "at": now.Format(time.RFC3339)}
	}
	arming := func(tag string) {
		row := common(tag)
		row["event"] = registry.EventArming
		appendRow(row)
	}
	armed := func(tag string, pid int64) {
		row := common(tag)
		row["event"] = registry.EventArmed
		row["ownerPid"] = pid
		row["ownerPidStartedAt"] = int64(100)
		row["generation"] = int64(1)
		appendRow(row)
	}

	arming("live")
	armed("live", 41)
	arming("reserved")
	arming("unknown")
	armed("unknown", 42)
	arming("dead")
	armed("dead", 43)
	arming("sweepable")
	reaped := common("sweepable")
	reaped["event"] = registry.EventReaped
	reaped["reason"] = "owner-dead"
	reaped["sweepPending"] = true
	appendRow(reaped)
	arming("free")
	exited := common("free")
	exited["event"] = registry.EventExited
	exited["reason"] = "purpose-gone"
	exited["teardownComplete"] = true
	appendRow(exited)

	problems, err := EngineFloorProblems(
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		func(_, _ string) (bool, error) { return false, nil },
		engineFloorProber{41: identity.Alive, 42: identity.Unknown, 43: identity.Dead},
		now,
	)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(problems, "\n")
	for _, want := range []string{"LiveVerified", "OpenReservation", "UnknownLiveness", "DeadOwner", "SweepableClosed"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing consumed slot class %s in:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, checkouts["free"]) {
		t.Fatalf("free slot was inspected:\n%s", joined)
	}

	writeState := func(tag, content string) {
		t.Helper()
		path := filepath.Join(checkouts[tag], "artifacts", "agents", "supervision", "state.json")
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	allTags := []string{"live", "reserved", "unknown", "dead", "sweepable", "free"}
	for _, tag := range allTags {
		writeState(tag, `{"engineBuild":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`)
	}
	problems, err = EngineFloorProblems(
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		func(_, _ string) (bool, error) { return true, nil },
		engineFloorProber{41: identity.Alive, 42: identity.Unknown, 43: identity.Dead},
		now,
	)
	if err != nil || len(problems) != 0 {
		t.Fatalf("at-or-above engines refused: %v %v", problems, err)
	}

	consumed := map[string]string{
		"live":      "LiveVerified",
		"reserved":  "OpenReservation",
		"unknown":   "UnknownLiveness",
		"dead":      "DeadOwner",
		"sweepable": "SweepableClosed",
	}
	for tag, class := range consumed {
		for _, candidate := range allTags {
			writeState(candidate, `{"engineBuild":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`)
		}
		writeState(tag, "{")
		problems, err = EngineFloorProblems(
			"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			func(_, _ string) (bool, error) { return true, nil },
			engineFloorProber{41: identity.Alive, 42: identity.Unknown, 43: identity.Dead},
			now,
		)
		if err != nil || len(problems) != 1 || !strings.Contains(problems[0], class) || !strings.Contains(problems[0], "unreadable") {
			t.Fatalf("%s unreadable state problems = %v, err=%v", class, problems, err)
		}

		writeState(tag, `{"engineBuild":"dev"}`)
		problems, err = EngineFloorProblems(
			"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			func(_, _ string) (bool, error) { return true, nil },
			engineFloorProber{41: identity.Alive, 42: identity.Unknown, 43: identity.Dead},
			now,
		)
		if err != nil || len(problems) != 1 || !strings.Contains(problems[0], class) || !strings.Contains(problems[0], "runs engine dev") {
			t.Fatalf("%s dev state problems = %v, err=%v", class, problems, err)
		}
	}

	for _, tag := range allTags {
		writeState(tag, `{"engineBuild":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`)
	}
	writeState("free", "{")
	problems, err = EngineFloorProblems(
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		func(_, _ string) (bool, error) { return true, nil },
		engineFloorProber{41: identity.Alive, 42: identity.Unknown, 43: identity.Dead},
		now,
	)
	if err != nil || len(problems) != 0 {
		t.Fatalf("an unreadable free slot was inspected: %v %v", problems, err)
	}
}
