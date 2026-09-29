package steward

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

var attemptsNow = time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)

const (
	attemptA = "proof-a-0000000000000001"
	attemptB = "proof-b-0000000000000002"
	attemptC = "proof-c-0000000000000003"
	attemptD = "proof-d-0000000000000004"
)

func writeRecord(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func namedSet(t *testing.T, namer interface {
	Named(context.Context, time.Time) ([]string, error)
}) []string {
	t.Helper()
	named, err := namer.Named(context.Background(), attemptsNow)
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(named)
	return slices.Compact(named)
}

func fixedLedger(tree *goal.TreeGoals, err error) *ledgerView {
	return &ledgerView{project: func() (goal.Projection, error) { return goal.Projection{Tree: tree}, err }}
}

// A stop batch names its proofs while it is open or its goal is open; a
// complete batch of a concluded goal is history; an unreadable ledger is
// an error.
func TestStopBatchNamerReadsOpenBatchesAndOpenGoals(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	stops := filepath.Join(root, "artifacts", "agents", "goal-stops")
	writeRecord(t, filepath.Join(stops, "s1.json"), goal.StopBatch{GoalID: "done", State: goal.StopBatchOpen, PendingProofs: []string{attemptA}})
	writeRecord(t, filepath.Join(stops, "s2.json"), goal.StopBatch{GoalID: "live", State: goal.StopBatchComplete,
		ObservedProofs: []goal.StopProof{{AttemptID: attemptB}}})
	writeRecord(t, filepath.Join(stops, "s3.json"), goal.StopBatch{GoalID: "done", State: goal.StopBatchComplete, TerminalProofs: []string{attemptC}})
	tree := &goal.TreeGoals{Live: map[string]*goal.GoalFile{"live": {}}}
	named := namedSet(t, stopBatchNamer{Root: root, Ledger: fixedLedger(tree, nil)})
	if !slices.Equal(named, []string{attemptA, attemptB}) {
		t.Fatalf("named = %v", named)
	}
	if _, err := (stopBatchNamer{Root: root, Ledger: fixedLedger(nil, errors.New("no ledger"))}).Named(context.Background(), attemptsNow); err == nil {
		t.Fatal("an unreadable ledger is an error")
	}
}

// An open trunk-red entry names its sightings and fix passes; a closed one
// is history; the cadence status names its attempt and reuse sources.
func TestTrunkRedNamerReadsOpenEntriesAndTheCadence(t *testing.T) {
	t.Parallel()
	tree := &goal.TreeGoals{
		TrunkRed: []goal.TrunkRedEntry{
			{ID: "open", Sightings: []goal.TrunkRedSighting{{Attempt: attemptA}}, FixProof: &goal.TrunkRedFixProof{Passes: []goal.TrunkRedPass{{Attempt: attemptB}}}},
			{ID: "closed", Sightings: []goal.TrunkRedSighting{{Attempt: attemptD}}, Closed: &goal.TrunkRedClosure{Attempt: attemptD}},
		},
		Cadence: &goal.CadenceStatus{AttemptID: attemptC, Groups: []goal.CadenceGroupStatus{{ReuseSource: "not-an-attempt"}}},
	}
	named := namedSet(t, trunkRedNamer{Ledger: fixedLedger(tree, nil)})
	if !slices.Equal(named, []string{attemptA, attemptB, attemptC}) {
		t.Fatalf("named = %v", named)
	}
}

// A landing batch that has not landed or dissolved names its admissions,
// proof, sources, trunk-red hold and prefix receipts; a landed one is
// history; an unreadable record is an error.
func TestLandingBatchNamerReadsUnlandedBatches(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	batches := filepath.Join(root, "artifacts", "agents", "landing-batches")
	writeRecord(t, filepath.Join(batches, "b1.json"), map[string]any{"state": "proving",
		"units":    []any{map[string]any{"admission": map[string]any{"attemptId": attemptA}}},
		"proof":    map[string]any{"attemptId": attemptB, "sources": map[string]any{"g": map[string]any{"kind": "reused", "attempt": attemptC}}},
		"receipts": map[string]any{"x": map[string]any{"AttemptID": attemptD}}})
	writeRecord(t, filepath.Join(batches, "b2.json"), map[string]any{"state": "landed", "proof": map[string]any{"attemptId": "proof-e-0000000000000005"}})
	named := namedSet(t, landingBatchNamer{Roots: []string{root}})
	if !slices.Equal(named, []string{attemptA, attemptB, attemptC, attemptD}) {
		t.Fatalf("named = %v", named)
	}
	if err := os.WriteFile(filepath.Join(batches, "b3.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := (landingBatchNamer{Roots: []string{root}}).Named(context.Background(), attemptsNow); err == nil {
		t.Fatal("an unreadable batch record is an error")
	}
}

// A receipt names its attempts and its result's reuse sources while it is
// younger than the window, or while its hand landing has not landed.
func TestLandingReceiptNamerReadsFreshAndPendingReceipts(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	agents := filepath.Join(root, "artifacts", "agents")
	old := attemptsNow.Add(-30 * 24 * time.Hour).Format(time.RFC3339Nano)
	writeRecord(t, filepath.Join(agents, "landing", "receipts", "t1.json"), map[string]any{"time": attemptsNow.Format(time.RFC3339Nano), "attemptIds": []string{attemptA}})
	writeRecord(t, filepath.Join(agents, "landing", "receipts", "t2.json"), map[string]any{"time": old, "attemptIds": []string{"proof-e-0000000000000005"}})
	writeRecord(t, filepath.Join(agents, "landing-intent", "g", "pending", "receipt-x.json"), map[string]any{"time": old, "proof": map[string]any{"attemptId": attemptB},
		"testing": map[string]any{"attemptId": attemptC, "groups": []any{map[string]any{"id": "g", "reuseAttempt": attemptD}}}})
	writeRecord(t, filepath.Join(agents, "landing-intent", "g", "landed", "receipt-y.json"), map[string]any{"time": old, "attemptIds": []string{"proof-f-0000000000000006"}})
	writeRecord(t, filepath.Join(agents, "landing-intent", "g", "landed", "landed.json"), map[string]any{"Landing": "l"})
	named := namedSet(t, landingReceiptNamer{Installation: root, Keep: 14 * 24 * time.Hour})
	if !slices.Equal(named, []string{attemptA, attemptB, attemptC, attemptD}) {
		t.Fatalf("named = %v", named)
	}
}

// The validation window names the attempts of its kept observations; a
// window with an unknown contract is an error.
func TestValidationWindowNamerReadsTheWindow(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	state := newValidationWindow()
	state.Observations = []validationWindowObservation{{RunID: "r", AttemptID: attemptA}}
	if err := saveValidationWindow(root, state); err != nil {
		t.Fatal(err)
	}
	named := namedSet(t, validationWindowNamer{Root: root})
	if !slices.Equal(named, []string{attemptA}) {
		t.Fatalf("named = %v", named)
	}
	if err := os.WriteFile(validationWindowPath(root), []byte(`{"schema":9}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := (validationWindowNamer{Root: root}).Named(context.Background(), attemptsNow); err == nil {
		t.Fatal("an unknown window is an error")
	}
}
