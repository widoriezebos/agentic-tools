package missionrunner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
)

const orphanTestSHA = "77b6f9ab2c13e302782555a4830ad9ce08d738eb"

// seedLandedChain plants one completed delegate chain with a landed round-1
// return and a stub return checker that accepts it, so landed-return
// derivation has something real to list.
func seedLandedChain(t *testing.T, root, missionID, jobID string) {
	t.Helper()
	writeJSONFile(t, filepath.Join(jobsDirPath(root), jobID+".json"), map[string]any{
		"jobId": jobID, "mission": missionID, "status": "completed", "round": 1, "parentJob": nil,
	})
	returnPath := filepath.Join(root, "artifacts", "agents", jobID, "rounds", "1", "return.json")
	if err := os.MkdirAll(filepath.Dir(returnPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(returnPath, []byte(`{"jobId":"`+jobID+`"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	checker := filepath.Join(root, "scripts", "assert-return-complete.sh")
	if err := os.MkdirAll(filepath.Dir(checker), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(checker, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}

// Terminal delivery: at the completion conclude the derived Landed Returns
// list lands in the FINAL cycle's ledger block as Landed unconsumed
// annotation lines, and every ledger parser keeps reading.
func TestDeliverLandedUnconsumedWritesFinalBlock(t *testing.T) {
	root := t.TempDir()
	engine := NewEngine(root, "m1")
	ledger := filepath.Join(missionDirPath(root, "m1"), "ledger.md")
	if err := mission.InitLedger(ledger, 8, 6); err != nil {
		t.Fatal(err)
	}
	if _, err := mission.AppendCycle(ledger, 1, "contract-improved", orphanTestSHA, "score=1", "yes"); err != nil {
		t.Fatal(err)
	}
	seedLandedChain(t, root, "m1", "job-landed")

	engine.deliverLandedUnconsumed(ledger, 1, map[string]any{"turnLog": []any{}}, "")

	_, _, cycles, err := mission.ParseLedger(ledger)
	if err != nil {
		t.Fatalf("annotated ledger must parse: %v", err)
	}
	want := "Landed unconsumed: chain=job-landed round=1 path=artifacts/agents/job-landed/rounds/1/return.json"
	if len(cycles) != 1 || len(cycles[0].Annotations) != 1 || cycles[0].Annotations[0] != want {
		t.Fatalf("terminal delivery must land in the final block: %+v", cycles)
	}

	// A certified round retires: delivery with the host's recorded action
	// appends nothing further.
	acted := map[string]any{"turnLog": []any{map[string]any{
		"certified": []any{map[string]any{"jobId": "job-landed"}},
	}}}
	engine.deliverLandedUnconsumed(ledger, 1, acted, "")
	_, _, cycles, err = mission.ParseLedger(ledger)
	if err != nil || len(cycles[0].Annotations) != 1 {
		t.Fatalf("a certified round must not re-deliver: %v %+v", err, cycles)
	}
}

// The runner FAILURE ramp settles the usage books before the lease release
// and writes NO ledger annotation - there is no safe ledger position on that
// ramp; an interrupted mission's landed returns are re-listed on resume.
func TestFailureRampAggregatesUsageWithoutLedgerWrite(t *testing.T) {
	root := t.TempDir()
	engine := NewEngine(root, "m1")
	writeJSONFile(t, filepath.Join(jobsDirPath(root), "job-a.json"), map[string]any{
		"jobId": "job-a", "mission": "m1", "status": "completed", "runtime": "codex",
		"usage": map[string]any{"outputTokens": 5},
	})

	// Resume against a mission with no state fails after the lease is held:
	// the one exit ramp for a runner dying mid-mission.
	code := engine.internalRun("resume", "ramp-test", filepath.Join(t.TempDir(), "signal.json"))
	if code == 0 {
		t.Fatal("a resume without state must fail")
	}

	usagePath := filepath.Join(missionDirPath(root, "m1"), "usage.json")
	data, err := os.ReadFile(usagePath)
	if err != nil {
		t.Fatalf("the failure ramp must aggregate usage before the lease release: %v", err)
	}
	if !strings.Contains(string(data), "tokens.outputTokens") {
		t.Fatalf("the aggregate must carry the terminal job's tokens:\n%s", data)
	}
	if _, err := os.Stat(filepath.Join(missionDirPath(root, "m1"), "ledger.md")); !os.IsNotExist(err) {
		t.Fatalf("the failure ramp must never write the ledger: %v", err)
	}
	// The lease was released on the ramp.
	if _, err := os.Stat(filepath.Join(missionDirPath(root, "m1"), "lease.d")); !os.IsNotExist(err) {
		t.Fatalf("the ramp must still release the lease: %v", err)
	}
}

// O4 ordering: the park proposal aggregates before it projects, so the
// parked state's fence usage carries every terminal round.
func TestParkProposalAggregatesBeforeProjection(t *testing.T) {
	root := t.TempDir()
	writeJSONFile(t, filepath.Join(jobsDirPath(root), "job-a.json"), map[string]any{
		"jobId": "job-a", "mission": "m1", "status": "completed", "runtime": "codex",
		"usage": map[string]any{"outputTokens": 11},
	})
	outcome, err := ParkProposal(root, "m1", cycleState(activeStreams()), "host-failure", "2026-08-11T00:00:00Z")
	if err != nil {
		t.Fatalf("park proposal: %v", err)
	}
	fences, _ := outcome.State["fences"].(map[string]any)
	units, _ := fences["usage"].([]any)
	found := false
	for _, raw := range units {
		item, _ := raw.(map[string]any)
		if item["unit"] == "tokens.outputTokens" {
			found = true
		}
	}
	if !found {
		t.Fatalf("the park must project the freshly aggregated usage: %v", fences["usage"])
	}
}

// An aggregation failure is witnessed and swallowed: the park still parks,
// and the projection reads whatever aggregate already existed.
func TestParkProposalSurvivesAggregationFailure(t *testing.T) {
	root := t.TempDir()
	// A directory where the fence lock file must live makes aggregation
	// fail before it can write anything.
	if err := os.MkdirAll(filepath.Join(missionDirPath(root, "m1"), "mission-fence.lock"), 0o755); err != nil {
		t.Fatal(err)
	}
	outcome, err := ParkProposal(root, "m1", cycleState(activeStreams()), "host-failure", "2026-08-11T00:00:00Z")
	if err != nil {
		t.Fatalf("an aggregation failure must never fail the park: %v", err)
	}
	if outcome.State["status"] != "parked" {
		t.Fatalf("the park proposal must still park: %v", outcome.State["status"])
	}
	if _, err := os.Stat(filepath.Join(missionDirPath(root, "m1"), "usage.json")); !os.IsNotExist(err) {
		t.Fatalf("a failed aggregation must write nothing: %v", err)
	}
}
