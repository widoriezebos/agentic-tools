package missionrunner

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func cycleState(streams map[string]any) map[string]any {
	return map[string]any{
		"status":      "running",
		"parkReason":  nil,
		"gatePassed":  false,
		"streams":     streams,
		"turnLog":     []any{},
		"ledger":      map[string]any{"cycles": json.Number("2"), "cycleBudget": json.Number("6")},
		"fences":      map[string]any{"startedAt": "2026-08-09T00:00:00Z", "cycles": json.Number("2"), "jobs": json.Number("0"), "activeJobs": json.Number("0")},
		"waitingList": []any{},
	}
}

func activeStreams() map[string]any {
	return map[string]any{"s-app": map[string]any{"state": "active", "reason": nil}}
}

func parkedStreams() map[string]any {
	return map[string]any{"s-app": map[string]any{"state": "parked-reserved", "reason": "waiting"}}
}

func seedFences(t *testing.T, root, mission string) {
	t.Helper()
	writeJSONFile(t, filepath.Join(missionDirPath(root, mission), "fences.json"), map[string]any{
		"startedAt":    "2026-08-10T01:00:00Z",
		"cycles":       3,
		"reservations": map[string]any{"job-done": map[string]any{}, "job-live": map[string]any{}, "job-lost": map[string]any{}},
	})
	writeJSONFile(t, filepath.Join(jobsDirPath(root), "job-done.json"), map[string]any{"jobId": "job-done", "status": "completed"})
	writeJSONFile(t, filepath.Join(jobsDirPath(root), "job-live.json"), map[string]any{"jobId": "job-live", "status": "running"})
	// job-lost has no record at all: it must still count as active.
}

func TestProjectFences(t *testing.T) {
	root := t.TempDir()
	mission := "demo"
	seedFences(t, root, mission)
	writeJSONFile(t, filepath.Join(missionDirPath(root, mission), "usage.json"), map[string]any{
		"units": []any{map[string]any{"kind": "tokens", "amount": 12}},
	})
	state := cycleState(activeStreams())
	if err := ProjectFences(root, mission, state); err != nil {
		t.Fatal(err)
	}
	fences := state["fences"].(map[string]any)
	if fences["startedAt"] != "2026-08-10T01:00:00Z" || fences["jobs"] != 3 || fences["activeJobs"] != 2 {
		t.Fatalf("projected fences: %v", fences)
	}
	if fences["cycles"] != json.Number("3") {
		t.Fatalf("projected cycles: %v", fences["cycles"])
	}
	units, ok := fences["usage"].([]any)
	if !ok || len(units) != 1 {
		t.Fatalf("projected usage: %v", fences["usage"])
	}
}

func TestProjectFencesRefusesBadReservations(t *testing.T) {
	root := t.TempDir()
	mission := "demo"
	writeJSONFile(t, filepath.Join(missionDirPath(root, mission), "fences.json"), map[string]any{"reservations": "three"})
	err := ProjectFences(root, mission, cycleState(activeStreams()))
	if err == nil || !strings.Contains(err.Error(), "reservations are unreadable") {
		t.Fatalf("got %v", err)
	}
}

func TestConcludeTurnStatusDecision(t *testing.T) {
	turn := testTurn()
	cases := []struct {
		name       string
		streams    map[string]any
		gatePassed bool
		status     string
		parkReason any
	}{
		{"gate passed", activeStreams(), true, "completed", nil},
		{"gate passed while parked", parkedStreams(), true, "completed", nil},
		{"no active streams", parkedStreams(), false, "parked", "all-streams-parked"},
		{"still working", activeStreams(), false, "running", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			proposed, err := ConcludeTurn(root, "demo", cycleState(tc.streams), turn, TurnConclusion{GatePassed: tc.gatePassed})
			if err != nil {
				t.Fatal(err)
			}
			if proposed["status"] != tc.status || proposed["parkReason"] != tc.parkReason || proposed["gatePassed"] != tc.gatePassed {
				t.Fatalf("decision: status=%v parkReason=%v gatePassed=%v", proposed["status"], proposed["parkReason"], proposed["gatePassed"])
			}
		})
	}
}

func TestConcludeTurnRecordsTheTurn(t *testing.T) {
	root := t.TempDir()
	mission := "demo"
	turn := testTurn()
	seedFences(t, root, mission)
	writeJSONFile(t, filepath.Join(asksDirPath(root, mission), "open.json"), map[string]any{"askId": "open", "answeredAt": nil})
	measurement := map[string]any{"metrics": map[string]any{"speed": "1.2"}, "guards": map[string]any{}, "candidateSha": "abc"}
	proposed, err := ConcludeTurn(root, mission, cycleState(activeStreams()), turn, TurnConclusion{
		SessionID:      "sess-1",
		Measurement:    measurement,
		GatePassed:     false,
		Accepted:       []any{map[string]any{"kind": "dispatched"}},
		Rejected:       []any{},
		Certified:      []any{},
		FactsForLedger: []any{"fact"},
		Gaps:           []any{},
	})
	if err != nil {
		t.Fatal(err)
	}
	log := proposed["turnLog"].([]any)
	if len(log) != 1 {
		t.Fatalf("turn log: %v", log)
	}
	entry := log[0].(map[string]any)
	if entry["turnId"] != turn.TurnID || entry["outcome"] != "completed" || entry["detail"] != "host return accepted" {
		t.Fatalf("entry: %v", entry)
	}
	if entry["sessionId"] != "sess-1" || !reflect.DeepEqual(entry["measurement"], measurement) {
		t.Fatalf("entry payload: %v", entry)
	}
	if proposed["ledger"].(map[string]any)["cycles"] != turn.Cycle {
		t.Fatalf("ledger cycles: %v", proposed["ledger"])
	}
	if !reflect.DeepEqual(proposed["waitingList"], []string{"open"}) {
		t.Fatalf("waiting list: %v", proposed["waitingList"])
	}
	if proposed["fences"].(map[string]any)["activeJobs"] != 2 {
		t.Fatalf("fences not projected: %v", proposed["fences"])
	}
}

func TestConclusionSession(t *testing.T) {
	turn := testTurn()
	turn.AnnouncedSession = "s-announced"
	if got := ConclusionSession(turn, nil); got != "s-announced" {
		t.Fatalf("without a witness the announced session propagates: %v", got)
	}
	if got := ConclusionSession(turn, "s-envelope"); got != "s-envelope" {
		t.Fatalf("a legacy unstamped turn falls back to the envelope session: %v", got)
	}
	turn.ObservedSession = "s-observed"
	if got := ConclusionSession(turn, "s-envelope"); got != "s-observed" {
		t.Fatalf("the observed session wins whatever the return's fate: %v", got)
	}
}

// TestConcludeFaultedTurnEmptyVerdict pins the one application rule: a
// not-accepted return applies nothing — streams keep their turn-start
// states, no asks appear — while the measurement and the fault both land in
// the turn-log entry.
func TestConcludeFaultedTurnEmptyVerdict(t *testing.T) {
	root := t.TempDir()
	mission := "demo"
	turn := testTurn()
	turn.AnnouncedSession = "s-stale"
	turn.ObservedSession = "s-live"
	seedFences(t, root, mission)
	writeJSONFile(t, filepath.Join(asksDirPath(root, mission), "open.json"), map[string]any{"askId": "open", "answeredAt": nil})
	measurement := map[string]any{"metrics": map[string]any{"score": "2"}, "guards": map[string]any{}, "candidateSha": "abc"}
	fault := TurnFault{
		Outcome:      "failed",
		Detail:       "orchestrator return session identity matches neither the announced nor the observed session",
		FeedsBreaker: true,
		Annotations:  []string{"Return: rejected:session identity"},
	}
	state := cycleState(activeStreams())
	proposed, err := ConcludeFaultedTurn(root, mission, state, turn, fault, measurement, false, 1)
	if err != nil {
		t.Fatal(err)
	}
	entry := proposed["turnLog"].([]any)[0].(map[string]any)
	if entry["outcome"] != "failed" || entry["detail"] != fault.Detail {
		t.Fatalf("entry fault facts: %v", entry)
	}
	if entry["sessionId"] != "s-live" {
		t.Fatalf("the observed session must propagate to the next announcement: %v", entry["sessionId"])
	}
	if !reflect.DeepEqual(entry["measurement"], measurement) {
		t.Fatalf("the measurement must land beside the fault: %v", entry["measurement"])
	}
	if !reflect.DeepEqual(entry["annotations"], []string{"Return: rejected:session identity"}) || entry["feedsBreaker"] != true {
		t.Fatalf("entry must mirror the ledger facts: %v", entry)
	}
	streams := proposed["streams"].(map[string]any)["s-app"].(map[string]any)
	if streams["state"] != "active" {
		t.Fatalf("streams must keep their turn-start states: %v", streams)
	}
	if !reflect.DeepEqual(proposed["waitingList"], []string{"open"}) {
		t.Fatalf("open asks stay open, nothing else appears: %v", proposed["waitingList"])
	}
	if proposed["status"] != "running" || proposed["parkReason"] != nil {
		t.Fatalf("a first witnessed fault keeps the mission running: %v/%v", proposed["status"], proposed["parkReason"])
	}
	if proposed["ledger"].(map[string]any)["cycles"] != turn.Cycle {
		t.Fatalf("a faulted turn still spends its cycle: %v", proposed["ledger"])
	}
}

// TestConcludeFaultedTurnStatusMatrix pins the decoupled decisions: measured
// gate truth completes the mission from any stream configuration, a
// witnessed fault parks host-failure on the second consecutive failure, and
// a no-witness fault never feeds the breaker.
func TestConcludeFaultedTurnStatusMatrix(t *testing.T) {
	turn := testTurn()
	cases := []struct {
		name         string
		streams      map[string]any
		gatePassed   bool
		feedsBreaker bool
		failures     int
		status       string
		parkReason   any
	}{
		{"gate pass completes", activeStreams(), true, true, 1, "completed", nil},
		{"gate pass completes even on the second consecutive failure", activeStreams(), true, true, 2, "completed", nil},
		{"gate pass completes with every stream parked", parkedStreams(), true, true, 1, "completed", nil},
		{"witnessed second failure parks", activeStreams(), false, true, 2, "parked", "host-failure"},
		{"witnessed first failure keeps running", activeStreams(), false, true, 1, "running", nil},
		{"no witness never parks", activeStreams(), false, false, 5, "running", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			fault := TurnFault{Outcome: "failed", Detail: "x", FeedsBreaker: tc.feedsBreaker,
				Annotations: []string{"Return: rejected:x"}}
			proposed, err := ConcludeFaultedTurn(root, "demo", cycleState(tc.streams), turn, fault, nil, tc.gatePassed, tc.failures)
			if err != nil {
				t.Fatal(err)
			}
			if proposed["status"] != tc.status || proposed["parkReason"] != tc.parkReason {
				t.Fatalf("status=%v parkReason=%v, want %v/%v", proposed["status"], proposed["parkReason"], tc.status, tc.parkReason)
			}
			if gate, _ := proposed["gatePassed"].(bool); gate != tc.gatePassed {
				t.Fatalf("gatePassed=%v, want %v", gate, tc.gatePassed)
			}
		})
	}
}

// TestConcludeFaultedTurnCapped pins the capped outcome surviving end to
// end in the proposal: the entry says capped and mirrors the ledger's
// Outcome annotation.
func TestConcludeFaultedTurnCapped(t *testing.T) {
	root := t.TempDir()
	turn := testTurn()
	fault := TurnFault{Outcome: "capped", Detail: "host turn reached host.turn-cap-min",
		FeedsBreaker: true, Annotations: []string{"Outcome: capped"}}
	proposed, err := ConcludeFaultedTurn(root, "demo", cycleState(activeStreams()), turn, fault, nil, false, 1)
	if err != nil {
		t.Fatal(err)
	}
	entry := proposed["turnLog"].([]any)[0].(map[string]any)
	if entry["outcome"] != "capped" || !reflect.DeepEqual(entry["annotations"], []string{"Outcome: capped"}) {
		t.Fatalf("capped entry: %v", entry)
	}
	if entry["measurement"] != nil {
		t.Fatalf("an unmeasurable capped turn records no measurement: %v", entry["measurement"])
	}
}

func TestRecordFailureProposal(t *testing.T) {
	root := t.TempDir()
	turn := testTurn()
	proposed, err := RecordFailureProposal(root, "demo", cycleState(activeStreams()), turn, "host exited non-zero (3)", "failed", 1)
	if err != nil {
		t.Fatal(err)
	}
	if proposed["status"] != "running" {
		t.Fatalf("a first failure keeps the mission running: %v", proposed["status"])
	}
	entry := proposed["turnLog"].([]any)[0].(map[string]any)
	if entry["outcome"] != "failed" || entry["detail"] != "host exited non-zero (3)" || entry["sessionId"] != nil || entry["measurement"] != nil {
		t.Fatalf("entry: %v", entry)
	}
	if proposed["ledger"].(map[string]any)["cycles"] != turn.Cycle {
		t.Fatalf("a failed turn still spends its cycle: %v", proposed["ledger"])
	}

	parked, err := RecordFailureProposal(root, "demo", cycleState(activeStreams()), turn, "start-unverified", "failed", 2)
	if err != nil {
		t.Fatal(err)
	}
	if parked["status"] != "parked" || parked["parkReason"] != "host-failure" || parked["gatePassed"] != false {
		t.Fatalf("second consecutive failure must park: %v", parked["status"])
	}
}

func TestParkProposalRaisesAnswerableAsk(t *testing.T) {
	root := t.TempDir()
	mission := "demo"
	outcome, err := ParkProposal(root, mission, cycleState(activeStreams()), "host-failure", "2026-08-10T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if outcome.State["status"] != "parked" || outcome.State["parkReason"] != "host-failure" || outcome.State["gatePassed"] != false {
		t.Fatalf("park state: %v", outcome.State)
	}
	if len(outcome.Asks) != 1 {
		t.Fatalf("asks: %v", outcome.Asks)
	}
	ask := outcome.Asks[0]
	if ask["askId"] != "host-failure" || ask["streamId"] != "s-app" || ask["question"] != "Acknowledge the host failure before resuming the mission." {
		t.Fatalf("ask: %v", ask)
	}
	if !reflect.DeepEqual(outcome.State["waitingList"], []string{"host-failure"}) {
		t.Fatalf("waiting list: %v", outcome.State["waitingList"])
	}
}

func TestParkProposalReusesOpenAsk(t *testing.T) {
	root := t.TempDir()
	mission := "demo"
	writeJSONFile(t, filepath.Join(asksDirPath(root, mission), "host-failure.json"), map[string]any{
		"askId": "host-failure", "reasonClass": "host-failure", "answeredAt": nil,
	})
	outcome, err := ParkProposal(root, mission, cycleState(activeStreams()), "host-failure", "2026-08-10T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if len(outcome.Asks) != 0 {
		t.Fatalf("an open ask with the reason must not be duplicated: %v", outcome.Asks)
	}
	if !reflect.DeepEqual(outcome.State["waitingList"], []string{"host-failure"}) {
		t.Fatalf("waiting list: %v", outcome.State["waitingList"])
	}
}

func TestParkProposalStopLossAndOtherReasons(t *testing.T) {
	root := t.TempDir()
	outcome, err := ParkProposal(root, "demo", cycleState(parkedStreams()), "stop-loss", "2026-08-10T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if len(outcome.Asks) != 1 || outcome.Asks[0]["question"] != "Amend, price, reseal, and sign the mission budget before requesting stop-loss unpark." {
		t.Fatalf("stop-loss ask: %v", outcome.Asks)
	}
	if outcome.Asks[0]["streamId"] != "s-app" {
		t.Fatalf("with no active stream the ask lands on the first stream: %v", outcome.Asks[0])
	}

	fence, err := ParkProposal(root, "demo", cycleState(activeStreams()), "fence", "2026-08-10T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if len(fence.Asks) != 0 || fence.State["parkReason"] != "fence" {
		t.Fatalf("only host-failure and stop-loss parks raise their own ask: %v", fence.Asks)
	}
}
