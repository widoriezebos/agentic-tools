package missionrunner

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
)

// parkedResetMission builds a real hash-chained mission on disk, parked for
// stop-loss, with an open stop-loss ask of the given kind. The anchor is
// stubbed to a no-op: anchoring shells out to the metasystem binary, which a
// unit test does not have; the anchor's own behavior is proven in the
// mission package.
func parkedResetMission(t *testing.T, kind string) (engine *Engine, statePath, ledgerPath, askPath string) {
	t.Helper()
	root := t.TempDir()
	engine = NewEngine(root, "demo")
	engine.anchorFn = func(string, string, string) error { return nil }
	dir := engine.missionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	contractPath := filepath.Join(dir, "mission-demo.contract.md")
	contract := "# Intent\n\n```mission\ncandidate.branch=main\nstream.primary=Do the work\n```\n"
	if err := os.WriteFile(contractPath, []byte(contract), 0o644); err != nil {
		t.Fatal(err)
	}
	statePath = filepath.Join(dir, "state.json")
	ledgerPath = filepath.Join(dir, "ledger.md")
	if err := mission.InitLedger(ledgerPath, 8, 2); err != nil {
		t.Fatal(err)
	}
	if err := mission.InitStateWithBaseline(statePath, contractPath, ledgerPath, "", "main", strings.Repeat("b", 40), testAdmissionOrigins()); err != nil {
		t.Fatal(err)
	}

	// Park the mission for stop-loss through the compare-and-write, exactly
	// as the runner's park does.
	state, err := readJSONDoc(statePath)
	if err != nil {
		t.Fatal(err)
	}
	integrity, _ := state["integrity"].(map[string]any)
	hash, _ := integrity["hash"].(string)
	proposed := deepCopyDoc(state)
	proposed["status"] = "parked"
	proposed["parkReason"] = "stop-loss"
	proposed["waitingList"] = []any{"stop-loss"}
	source := filepath.Join(t.TempDir(), "proposed.json")
	writeJSONFile(t, source, proposed)
	if err := mission.WriteState(statePath, source, hash); err != nil {
		t.Fatal(err)
	}

	askPath = filepath.Join(dir, "asks", "stop-loss.json")
	ask := askRecord("stop-loss", "primary", "stop-loss", "q", "2026-08-11T00:00:00Z")
	if kind != "" {
		ask["stopLossKind"] = kind
	}
	writeJSONFile(t, askPath, ask)
	return engine, statePath, ledgerPath, askPath
}

func readTestDoc(t *testing.T, path string) map[string]any {
	t.Helper()
	doc, err := readJSONDoc(path)
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func TestAnswerResetUnparksInBindingOrder(t *testing.T) {
	engine, statePath, ledgerPath, askPath := parkedResetMission(t, StopLossStagnation)
	if code := engine.Answer("stop-loss", "reset: the tail work justifies more of the sealed fences"); code != 0 {
		t.Fatalf("reset answer refused with exit %d", code)
	}
	ledger, _ := os.ReadFile(ledgerPath)
	if !strings.Contains(string(ledger), "Stop-loss reset: ask=stop-loss; reason=the tail work justifies more of the sealed fences") {
		t.Fatalf("the authoritative ledger line is missing:\n%s", ledger)
	}
	ask := readTestDoc(t, askPath)
	if ask["answeredAt"] == nil || ask["answer"] != "reset: the tail work justifies more of the sealed fences" {
		t.Fatalf("ask was not marked answered: %v", ask)
	}
	state := readTestDoc(t, statePath)
	if state["status"] != "running" || state["parkReason"] != nil {
		t.Fatalf("mission did not unpark: %v %v", state["status"], state["parkReason"])
	}
	if list, _ := state["waitingList"].([]any); len(list) != 0 {
		t.Fatalf("answered ask must leave the waiting list: %v", list)
	}
	// The budget line is untouched: the human spends the still-sealed
	// fences, not a new allowance.
	if !strings.Contains(string(ledger), "- No-gain budget: 2\n") {
		t.Fatalf("the sealed budget line must not change:\n%s", ledger)
	}

	// A second answer of the now-answered ask stays refused, vocally.
	if code := engine.Answer("stop-loss", "reset: again"); code == 0 {
		t.Fatal("an already answered ask must refuse a second answer")
	}
}

func TestAnswerResetRefusals(t *testing.T) {
	cases := []struct {
		name   string
		kind   string
		answer string
	}{
		{"non-reset answer keeps amendment guidance", StopLossStagnation, "please continue"},
		{"cycle-budget park refuses reset", StopLossCycleBudget, "reset: fund the tail"},
		{"legacy ask refuses reset", "", "reset: fund the tail"},
		{"newline reason refused", StopLossStagnation, "reset: line one\nline two"},
		{"empty reason refused", StopLossStagnation, "reset:   "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			engine, statePath, ledgerPath, askPath := parkedResetMission(t, tc.kind)
			before, _ := os.ReadFile(ledgerPath)
			if code := engine.Answer("stop-loss", tc.answer); code == 0 {
				t.Fatal("answer must be refused")
			}
			after, _ := os.ReadFile(ledgerPath)
			if string(before) != string(after) {
				t.Fatal("a refused answer must not touch the ledger")
			}
			if ask := readTestDoc(t, askPath); ask["answeredAt"] != nil {
				t.Fatal("a refused answer must leave the ask open")
			}
			if state := readTestDoc(t, statePath); state["status"] != "parked" {
				t.Fatal("a refused answer must leave the mission parked")
			}
		})
	}
}

func TestAnswerResetAppendFailureBlocksEverything(t *testing.T) {
	engine, statePath, ledgerPath, askPath := parkedResetMission(t, StopLossStagnation)
	// Break the ledger so the locked append refuses: nothing after step (1)
	// may happen without the ledger line.
	if err := os.WriteFile(ledgerPath, []byte("# Mission Ledger\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := engine.Answer("stop-loss", "reset: fund the tail"); code == 0 {
		t.Fatal("a failed ledger append must refuse the answer")
	}
	if ask := readTestDoc(t, askPath); ask["answeredAt"] != nil {
		t.Fatal("the ask must stay open when the append fails")
	}
	if state := readTestDoc(t, statePath); state["status"] != "parked" {
		t.Fatal("the mission must stay parked, loudly")
	}
}

func TestAnswerResetCrashAfterAppendIsReanswerable(t *testing.T) {
	// Crash boundary (1): the reset line landed, the ask is still open. A
	// re-answer appends a second line — lawful, vocal, harmless — and
	// completes the transaction.
	engine, statePath, ledgerPath, _ := parkedResetMission(t, StopLossStagnation)
	if err := mission.AppendReset(ledgerPath, "stop-loss", "first attempt, crashed"); err != nil {
		t.Fatal(err)
	}
	if code := engine.Answer("stop-loss", "reset: second attempt"); code != 0 {
		t.Fatalf("re-answer after a crash refused with exit %d", code)
	}
	ledger, _ := os.ReadFile(ledgerPath)
	if strings.Count(string(ledger), "Stop-loss reset:") != 2 {
		t.Fatalf("both reset lines must stand:\n%s", ledger)
	}
	if state := readTestDoc(t, statePath); state["status"] != "running" {
		t.Fatal("the re-answer must unpark")
	}
}

func TestAnswerResetUnparkFailureLeavesAnsweredAsk(t *testing.T) {
	// Crash boundary (2)/(3): the line landed and the ask is answered, but
	// the unpark did not apply. Nothing is rolled back; resume owns the rest.
	engine, _, _, askPath := parkedResetMission(t, StopLossStagnation)
	engine.anchorFn = func(string, string, string) error { return errors.New("anchor unavailable") }
	if code := engine.Answer("stop-loss", "reset: fund the tail"); code == 0 {
		t.Fatal("a failed unpark must exit non-zero")
	}
	if ask := readTestDoc(t, askPath); ask["answeredAt"] == nil {
		t.Fatal("the answered ask must not be rolled back: the ledger line is authoritative")
	}
}

func TestApplyPendingResetOnResume(t *testing.T) {
	// Crash boundary (2): answered ask, recorded reset line, parked state.
	// The next resume applies the unpark.
	engine, statePath, ledgerPath, askPath := parkedResetMission(t, StopLossStagnation)
	if err := mission.AppendReset(ledgerPath, "stop-loss", "fund the tail"); err != nil {
		t.Fatal(err)
	}
	ask := readTestDoc(t, askPath)
	ask["answeredAt"] = "2026-08-11T01:00:00Z"
	ask["answer"] = "reset: fund the tail"
	writeJSONFile(t, askPath, ask)

	state := readTestDoc(t, statePath)
	applied, err := engine.applyPendingReset(statePath, ledgerPath, state)
	if err != nil || !applied {
		t.Fatalf("pending reset must apply: applied=%v err=%v", applied, err)
	}
	if state := readTestDoc(t, statePath); state["status"] != "running" || state["parkReason"] != nil {
		t.Fatalf("resume did not unpark: %v %v", state["status"], state["parkReason"])
	}
}

func TestApplyPendingResetLeavesOpenAskForTheHuman(t *testing.T) {
	// Crash boundary (1): the reset line exists but the ask is still open —
	// resume must NOT unpark; the human answers first.
	engine, statePath, ledgerPath, _ := parkedResetMission(t, StopLossStagnation)
	if err := mission.AppendReset(ledgerPath, "stop-loss", "fund the tail"); err != nil {
		t.Fatal(err)
	}
	state := readTestDoc(t, statePath)
	applied, err := engine.applyPendingReset(statePath, ledgerPath, state)
	if err != nil || applied {
		t.Fatalf("an open ask must block the unpark: applied=%v err=%v", applied, err)
	}
	if state := readTestDoc(t, statePath); state["status"] != "parked" {
		t.Fatal("the mission must stay parked until the ask is answered")
	}
}

func TestStopLossParkProposalRecordsTheKind(t *testing.T) {
	root := t.TempDir()
	verdict := &StopLossVerdict{
		Semantics: 2, Tripped: true, Kind: StopLossStagnation,
		Stagnant: 3, NoGainBudget: 3, Cycles: 5, CycleBudget: 10,
	}
	outcome, err := StopLossParkProposal(root, "demo", cycleState(activeStreams()), verdict.Kind, verdict.askQuestion(), "2026-08-11T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if len(outcome.Asks) != 1 {
		t.Fatalf("asks: %v", outcome.Asks)
	}
	ask := outcome.Asks[0]
	if ask["stopLossKind"] != StopLossStagnation || ask["reasonClass"] != "stop-loss" {
		t.Fatalf("stagnation ask: %v", ask)
	}
	if question, _ := ask["question"].(string); !strings.Contains(question, "reset:") {
		t.Fatalf("a stagnation ask must name the reset answer: %q", question)
	}

	budget := &StopLossVerdict{Semantics: 2, Tripped: true, Kind: StopLossCycleBudget, Cycles: 10, CycleBudget: 10}
	outcome, err = StopLossParkProposal(root, "demo2", cycleState(activeStreams()), budget.Kind, budget.askQuestion(), "2026-08-11T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	ask = outcome.Asks[0]
	if ask["stopLossKind"] != StopLossCycleBudget {
		t.Fatalf("cycle-budget ask: %v", ask)
	}
	if question, _ := ask["question"].(string); !strings.Contains(question, "exhausted sealed allowance") {
		t.Fatalf("a cycle-budget ask must name the exhausted allowance: %q", question)
	}

	legacy := &StopLossVerdict{Semantics: 1, Tripped: true, Kind: StopLossLegacy}
	outcome, err = StopLossParkProposal(root, "demo3", cycleState(activeStreams()), legacy.Kind, legacy.askQuestion(), "2026-08-11T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	ask = outcome.Asks[0]
	if _, present := ask["stopLossKind"]; present {
		t.Fatalf("legacy parks keep the ask shape they started with: %v", ask)
	}
	if ask["question"] != "Amend, price, reseal, and sign the mission budget before requesting stop-loss unpark." {
		t.Fatalf("legacy question changed: %v", ask["question"])
	}
}
