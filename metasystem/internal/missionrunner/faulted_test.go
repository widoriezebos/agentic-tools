package missionrunner

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/contract"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
)

// The engine-level proof of the faulted-turn path: a turn whose return was
// rejected (or capped) still drains, measures the committed tree, appends
// the ledger line with its fault annotations, and concludes with the empty
// verdict plus the measurement.

const faultedProjectRules = `### Mission envelope eligibility

| id | Description | pre-authorizable | bound |
| --- | --- | --- | --- |
| ` + "`dependencies`" + ` | Adding or upgrading dependencies | yes | dependency allowlist |
`

// faultedContract declares one score metric whose gate emits score=1 and an
// audit guard reading 1, so sealing measures baseline 1 and every later
// measurement passes the gate.
func faultedContract() string {
	return strings.Join([]string{
		"# Intent", "",
		"Reach the declared score with frozen instruments.", "",
		"# Non-goals", "",
		"Do not publish or deploy.", "",
		"# Initial streams", "",
		"Keep one stream active.", "",
		"```mission",
		"gate.command=scripts/gate.sh",
		"gate.ref=instruments",
		"gate.paths=scripts/gate.sh",
		"truth.paths=truth/*.txt",
		"truth.certification=certified",
		"gate.direction=max",
		"gate.threshold.score=>=1",
		"gate.noise-floor.score=0",
		"guard.audit.command=scripts/gate.sh",
		"guard.audit.floor=1",
		"guard.audit.noise=0",
		"guard.cadence=1",
		"ledger.cycle-budget=5",
		"ledger.no-gain-budget=5",
		"fence.wall-clock-hours=2",
		"fence.cycles=5",
		"fence.jobs=4",
		"fence.concurrency=2",
		"fence.job-cap-min=30",
		"host.runtime=fake",
		"host.model=fake-model",
		"host.turn-cap-min=20",
		"stream.primary=Reach the acceptance score.",
		"envelope.dependencies=jq",
		"exposure=EUR:10",
		"```",
		"",
	}, "\n")
}

// faultedMission builds a running one-cycle-reserved mission over a real
// measurable repository: gate instruments committed and tagged, the sealed
// contract in plans/, the approved snapshot pinned in the fence counters,
// and a turn record whose announced and observed sessions differ.
func faultedMission(t *testing.T) (engine *Engine, statePath, ledgerPath, turnPath, turnDir string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
		return string(out)
	}
	write := func(rel, content string, mode os.FileMode) {
		t.Helper()
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), mode); err != nil {
			t.Fatal(err)
		}
	}
	git("-c", "init.defaultBranch=main", "init", "-q")
	git("config", "commit.gpgsign", "false")
	write("scripts/gate.sh", "#!/usr/bin/env bash\nset -euo pipefail\nprintf 'metric=score=1\\nmetric=audit=1\\n'\n", 0o755)
	write("truth/reference.txt", "certified truth\n", 0o644)
	write("docs/project-rules.md", faultedProjectRules, 0o644)
	git("add", ".")
	git("commit", "-qm", "instruments")
	git("tag", "instruments")
	git("checkout", "-q", "-B", "main")

	engine = NewEngine(root, "demo")
	engine.anchorFn = func(string, string, string) error { return nil }
	// The drain beats the runner heartbeat every pass, which reads the
	// runner's own record; a real runner writes it before its first cycle.
	seedRunnerRecord(t, engine)
	contractPath := engine.contractPath()
	write(filepath.Join("plans", "mission-demo.contract.md"), faultedContract(), 0o644)
	if _, err := contract.Seal(contractPath); err != nil {
		t.Fatalf("seal failed: %v", err)
	}
	sealedBytes, err := os.ReadFile(contractPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(engine.missionDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(engine.approvedContractPath(), sealedBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(sealedBytes)
	writeJSONFile(t, engine.fencesPath(), map[string]any{
		"schemaVersion": 1, "missionId": "demo", "startedAt": "2026-08-11T00:00:00Z",
		"cycles": 1, "reservations": map[string]any{},
		"approvedContractSha256": hex.EncodeToString(sum[:]),
	})

	statePath = filepath.Join(engine.missionDir(), "state.json")
	ledgerPath = filepath.Join(engine.missionDir(), "ledger.md")
	if err := mission.InitLedger(ledgerPath, 5, 3); err != nil {
		t.Fatal(err)
	}
	if err := mission.InitState(statePath, engine.approvedContractPath(), ledgerPath, "", "main"); err != nil {
		t.Fatal(err)
	}

	turnDir = filepath.Join(engine.missionDir(), "turns", "demo-t1-abcd")
	turnPath = filepath.Join(turnDir, "turn.json")
	writeJSONFile(t, turnPath, map[string]any{
		"missionId": "demo", "turnId": "demo-t1-abcd", "cycle": 1,
		"runtime": "fake", "model": "fake-model",
		"hostSession": "s-stale", "announcedSession": "s-stale", "observedSession": "s-live",
		"status": "failed", "outcome": "failed", "error": "protocol-error",
	})
	return engine, statePath, ledgerPath, turnPath, turnDir
}

// TestStampObservedSession pins the observed-identity source order: the
// terminal result envelope is the universal host source (no host adapter
// emits a launch signal today), a session already on disk still counts when
// the in-memory envelope is absent, and an empty envelope leaves the field
// null — never the return's own claim.
func TestStampObservedSession(t *testing.T) {
	engine := NewEngine(t.TempDir(), "demo")
	turnDir := filepath.Join(engine.missionDir(), "turns", "demo-t1-aaaa")
	turnPath := filepath.Join(turnDir, "turn.json")
	writeJSONFile(t, turnPath, map[string]any{"turnId": "demo-t1-aaaa", "observedSession": nil})

	got, err := engine.stampObservedSession(turnDir, map[string]any{"sessionId": "s-live"})
	if err != nil || got != "s-live" {
		t.Fatalf("envelope session must stamp: %v, %v", got, err)
	}
	if doc := readTestDoc(t, turnPath); doc["observedSession"] != "s-live" {
		t.Fatalf("turn record not stamped: %v", doc["observedSession"])
	}

	writeJSONFile(t, turnPath, map[string]any{"turnId": "demo-t1-aaaa", "observedSession": nil})
	if got, err := engine.stampObservedSession(turnDir, map[string]any{"sessionId": nil}); err != nil || got != nil {
		t.Fatalf("an envelope without a session stamps nothing: %v, %v", got, err)
	}
	if doc := readTestDoc(t, turnPath); doc["observedSession"] != nil {
		t.Fatalf("absent witness must stay null: %v", doc["observedSession"])
	}

	// A capped turn's envelope may exist only on disk.
	writeJSONFile(t, filepath.Join(turnDir, "result.json"), map[string]any{"sessionId": "s-disk"})
	if got, err := engine.stampObservedSession(turnDir, nil); err != nil || got != "s-disk" {
		t.Fatalf("the on-disk envelope is still the harness's artifact: %v, %v", got, err)
	}
}

func TestConcludeFaultedTurnMeasuresAndCompletes(t *testing.T) {
	engine, statePath, ledgerPath, turnPath, turnDir := faultedMission(t)
	fault := TurnFault{
		Outcome:      "failed",
		Detail:       "orchestrator return session identity matches neither the announced nor the observed session",
		FeedsBreaker: true,
		Annotations:  []string{mission.ReturnRejectedAnnotation("session identity mismatch")},
	}
	updated, err := engine.concludeFaultedTurn(statePath, ledgerPath, readTestDoc(t, statePath), turnPath, turnDir, fault, 1)
	if err != nil {
		t.Fatal(err)
	}
	// The measured gate passed: measurement is runner-run truth, so the
	// mission completes on the measured product with the fault recorded.
	if updated["status"] != "completed" || updated["gatePassed"] != true {
		t.Fatalf("measured gate pass must complete the mission: %v/%v", updated["status"], updated["gatePassed"])
	}
	ledger, _ := os.ReadFile(ledgerPath)
	if !strings.Contains(string(ledger), "### Cycle 1\n- Classification: unresolved;") {
		t.Fatalf("the cycle must book its measured classification:\n%s", ledger)
	}
	if !strings.Contains(string(ledger), "\n- Return: rejected:session identity mismatch\n") {
		t.Fatalf("the fault annotation must land in the same cycle block:\n%s", ledger)
	}
	measurementDoc := readTestDoc(t, filepath.Join(turnDir, "measurement.json"))
	if measurementDoc["gatePassed"] != true || measurementDoc["measurement"] == nil {
		t.Fatalf("measurement artifact: %v", measurementDoc)
	}
	entry := updated["turnLog"].([]any)[0].(map[string]any)
	if entry["outcome"] != "failed" || entry["sessionId"] != "s-live" || entry["measurement"] == nil {
		t.Fatalf("turn-log entry must carry the measurement, the fault, and the observed session: %v", entry)
	}
	// Streams were untouched by the completion: the recorded state is still
	// the turn-start one.
	stream := updated["streams"].(map[string]any)["primary"].(map[string]any)
	if stream["state"] != "active" {
		t.Fatalf("completion must not rewrite stream states: %v", stream)
	}
	// The ledger stays parseable and contiguous for any later reader.
	if _, _, cycles, err := mission.ParseLedger(ledgerPath); err != nil || len(cycles) != 1 || len(cycles[0].Annotations) != 1 {
		t.Fatalf("annotated ledger must parse: %v (%+v)", err, cycles)
	}
}

func TestConcludeFaultedTurnUnmeasurableStillBooksUnmeasurable(t *testing.T) {
	engine, statePath, ledgerPath, turnPath, turnDir := faultedMission(t)
	// Remove the authored contract: measurement now fails, and the failure
	// is itself the measurement — an unmeasurable no-progress cycle.
	if err := os.Remove(engine.contractPath()); err != nil {
		t.Fatal(err)
	}
	fault := TurnFault{
		Outcome:      "failed",
		Detail:       "orchestrator return is invalid: schema violation",
		FeedsBreaker: true,
		Annotations:  []string{mission.ReturnRejectedAnnotation("orchestrator return is invalid: schema violation")},
	}
	updated, err := engine.concludeFaultedTurn(statePath, ledgerPath, readTestDoc(t, statePath), turnPath, turnDir, fault, 1)
	if err != nil {
		t.Fatal(err)
	}
	if updated["status"] != "running" {
		t.Fatalf("a first witnessed fault keeps the mission running: %v", updated["status"])
	}
	ledger, _ := os.ReadFile(ledgerPath)
	if !strings.Contains(string(ledger), "- Classification: no-progress; candidate-sha=") ||
		!strings.Contains(string(ledger), "observed=unmeasurable:") {
		t.Fatalf("an unmeasurable tree still books unmeasurable:\n%s", ledger)
	}
	if !strings.Contains(string(ledger), "\n- Return: rejected:orchestrator return is invalid: schema violation\n") {
		t.Fatalf("the fault annotation must still land:\n%s", ledger)
	}
	entry := updated["turnLog"].([]any)[0].(map[string]any)
	if entry["measurement"] != nil || entry["outcome"] != "failed" {
		t.Fatalf("unmeasurable entry: %v", entry)
	}
}

func TestConcludeFaultedTurnCappedKeepsItsOutcome(t *testing.T) {
	engine, statePath, ledgerPath, turnPath, turnDir := faultedMission(t)
	// The launch path patched the cap before conclusion; the conclusion must
	// not rewrite it to failed.
	if _, err := patchTurn(turnPath, map[string]any{
		"status": "failed", "outcome": "capped", "error": "turn-cap",
	}); err != nil {
		t.Fatal(err)
	}
	fault := TurnFault{
		Outcome:      "capped",
		Detail:       "host turn reached host.turn-cap-min",
		FeedsBreaker: true,
		Annotations:  []string{mission.CappedAnnotation},
	}
	updated, err := engine.concludeFaultedTurn(statePath, ledgerPath, readTestDoc(t, statePath), turnPath, turnDir, fault, 1)
	if err != nil {
		t.Fatal(err)
	}
	turnDoc := readTestDoc(t, turnPath)
	if turnDoc["outcome"] != "capped" {
		t.Fatalf("outcome=capped must survive conclusion: %v", turnDoc["outcome"])
	}
	ledger, _ := os.ReadFile(ledgerPath)
	if !strings.Contains(string(ledger), "\n- Outcome: capped\n") {
		t.Fatalf("the capped annotation must land in the cycle block:\n%s", ledger)
	}
	entry := updated["turnLog"].([]any)[0].(map[string]any)
	if entry["outcome"] != "capped" || entry["measurement"] == nil {
		t.Fatalf("a cap that landed real work registers its measurement: %v", entry)
	}
	// The measured gate passed here too, so even a capped turn completes on
	// the measured product.
	if updated["status"] != "completed" {
		t.Fatalf("measured gate pass on a capped turn: %v", updated["status"])
	}
}
