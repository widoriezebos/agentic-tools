package lane

import (
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/state"
)

func TestKeeperWritesBarrenTransitionsAndRetainsTheirClock(t *testing.T) {
	t.Parallel()
	home, _, module := nestedLaneDirs(t)
	now := laneNow
	reasons := []string{queuedReason}
	agent := &fakeAgent{}
	keeper := agent.keeper(home, module, &now, wakeFor(&reasons))
	fingerprint := "unchanged"
	keeper.Fingerprint = func(string) (string, error) { return fingerprint, nil }
	for range 3 {
		keeper.Run()
		agent.running = ""
		now = now.Add(time.Minute)
	}
	saved, err := ReadAgentState(home)
	if err != nil || saved.Barren != 2 {
		t.Fatalf("barren record: %+v %v", saved, err)
	}
	at, err := time.Parse(time.RFC3339Nano, saved.BarrenAt)
	if err != nil || !at.Equal(now.Add(-time.Minute)) {
		t.Fatalf("barren clock: %s %v", saved.BarrenAt, err)
	}
	dir := module + "/artifacts/agents/landing"
	shadow, _, err := state.Read(dir)
	if err != nil || shadow.Holds.Barren != 2 || !shadow.Holds.Since.At.Equal(at) || shadow.Holds.Since.Observed {
		t.Fatalf("barren shadow: %+v %v", shadow.Holds, err)
	}
	fingerprint = "changed"
	if run := keeper.Run(); run.Outcome != AgentStarted {
		t.Fatalf("changed lane did not resume: %+v", run)
	}
	saved, err = ReadAgentState(home)
	if err != nil || saved.Barren != 0 || saved.BarrenAt != now.UTC().Format(time.RFC3339Nano) {
		t.Fatalf("clear clock: %+v %v", saved, err)
	}
	events, _, err := state.ReadEvents(dir)
	if err != nil {
		t.Fatal(err)
	}
	var counts []string
	for _, event := range events {
		if event.By != "keeper" || event.Kind == "defect" {
			t.Fatalf("keeper event: %+v", event)
		}
		if event.Kind == "holds" {
			counts = append(counts, event.To)
		}
	}
	want := []string{"barren=1, selection_pending=false", "barren=2, selection_pending=false", "barren=0, selection_pending=false"}
	if !reflect.DeepEqual(counts, want) {
		t.Fatalf("barren transitions: %v", counts)
	}
}

func TestKeeperRepairEmitsItsOwnTransition(t *testing.T) {
	t.Parallel()
	home, _, module := nestedLaneDirs(t)
	now := laneNow
	reasons := []string{queuedReason}
	agent := &fakeAgent{}
	keeper := agent.keeper(home, module, &now, wakeFor(&reasons))
	if run := keeper.Run(); run.Outcome != AgentStarted {
		t.Fatal(run)
	}
	if err := os.WriteFile(agentStatePath(home), []byte(`{torn`), 0600); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute)
	if err := RepairAgentRecord(home, now); err != nil {
		t.Fatal(err)
	}
	dir := module + "/artifacts/agents/landing"
	shadow, _, err := state.Read(dir)
	if err != nil || shadow.Agent.Session != "" || shadow.Agent.Word != "ended" {
		t.Fatalf("repair shadow: %+v %v", shadow.Agent, err)
	}
	events, _, err := state.ReadEvents(dir)
	if err != nil {
		t.Fatal(err)
	}
	last := events[len(events)-1]
	if last.Kind != "agent" || last.By != "keeper" || last.To != "ended" || !last.At.Equal(now) {
		t.Fatalf("repair event: %+v", last)
	}
}
