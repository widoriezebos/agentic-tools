package missionrunner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
)

// The named cap outcome is certified — a provider
// result carrying is_error + error_max_turns names the cutoff; a crash
// without it stays generic; absence stays generic.
func TestProviderErrorSubtype(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	write := func(name, body string) string {
		path := filepath.Join(dir, name)
		os.WriteFile(path, []byte(body), 0o644)
		return path
	}
	capped := write("capped.json", `{"is_error":true,"subtype":"error_max_turns","num_turns":51}`)
	if got := providerErrorSubtype(capped); got != "error_max_turns" {
		t.Fatalf("capped subtype = %q", got)
	}
	crash := write("crash.json", `{"is_error":true,"subtype":"error_during_execution"}`)
	if got := providerErrorSubtype(crash); got != "error_during_execution" {
		t.Fatalf("crash subtype = %q", got)
	}
	fine := write("fine.json", `{"is_error":false,"subtype":"success"}`)
	if got := providerErrorSubtype(fine); got != "" {
		t.Fatalf("non-error subtype leaked: %q", got)
	}
	if got := providerErrorSubtype(filepath.Join(dir, "absent.json")); got != "" {
		t.Fatalf("absent file subtype = %q", got)
	}
}

// The cap detail rides the failure into the LEDGER's classification line,
// not just turn.json (acceptance trace (b) certified as bytes).
func TestCapExhaustedReachesLedger(t *testing.T) {
	bed := newRecoveryFileBedWithContract(t, nil, strings.Replace(healContract, "stream.primary=Do the work", "stream.solo=Do solo", 1))
	engine, statePath, ledgerPath := bed.e, bed.state, bed.ledger
	trace := newThreeAnchorTrace(t, engine.Root, statePath, ledgerPath, "")
	trace.install(engine)
	engine.anchorFn = nil
	turnID := engine.Mission + "-t1-live"
	fences := readTestDoc(t, engine.fencesPath())
	fences["cycles"] = 1
	writeJSONFile(t, engine.fencesPath(), fences)
	runnersDir := filepath.Join(engine.Root, "artifacts", "agents", "missions", "runners")
	os.MkdirAll(runnersDir, 0o755)
	record := engine.runnerRecord(os.Getpid(), os.Getpid(), 1, "fixture")
	writeJSONFile(t, filepath.Join(runnersDir, engine.Mission+".json"), record)

	state, err := engine.verifyState(statePath, false)
	if err != nil {
		t.Fatal(err)
	}
	trace.verify(trace.tip)
	trace.begin()
	if _, _, err := engine.continuity().VerifyStateWithAnchor(statePath, engine.Root, ledgerPath); err != nil {
		t.Fatalf("open position is not anchored: %v", err)
	}
	trace.done()
	turnDir := bed.turnDir
	turnPath := filepath.Join(turnDir, "turn.json")
	writeJSONFile(t, turnPath, map[string]any{
		"missionId": engine.Mission, "turnId": turnID, "cycle": 1,
		"runtime": "fake", "model": "fake-model", "status": "running", "outcome": nil, "error": nil, "detail": nil,
		"startedAt": "2026-08-18T00:00:00Z", "endedAt": nil,
	})
	bed.facts.ledgerGuard()
	bed.facts.pass(recoveryPre)
	bed.facts.add("Git", scopePolicyGit{stdout: recoveryHead + "\n"}, engine.Root,
		[]string{"-C", engine.Root, "rev-parse", "main"})
	bed.facts.capture(recoveryPre, recoveryPre)
	engine.pinnedAnchorEffect = func(gotState, gotLedger, identity, hash, sha string) error {
		if gotState != statePath || gotLedger != ledgerPath || identity != turnID {
			t.Fatalf("pinned anchor arguments: %q %q %q", gotState, gotLedger, identity)
		}
		_, actualHash, err := mission.VerifyStateShape(statePath)
		if err != nil || hash != actualHash {
			t.Fatalf("pinned state hash = %q, actual %q: %v", hash, actualHash, err)
		}
		data, err := os.ReadFile(ledgerPath)
		if err != nil || sha != sha256Hex(string(data)) {
			t.Fatalf("pinned ledger SHA = %q, actual %q: %v", sha, sha256Hex(string(data)), err)
		}
		next := trace.fact(string(data), trace.tip, "")
		trace.publication(next, identity)
		trace.begin()
		err = mission.AnchorNamedWithRawAnchorOperations(trace.raw(), gotState, engine.Root, gotLedger, identity, hash, sha)
		trace.done()
		bed.facts.original = append([]byte(nil), data...)
		bed.facts.namespace()
		bed.facts.ledgerGuard()
		bed.facts.add("Git", scopePolicyGit{}, engine.Root,
			[]string{"update-ref", "-d", mission.MissionRefNamespace(engine.Mission) + "turn-open-head"})
		engine.anchorFn = func(state, ledger, name string) error {
			closing, err := os.ReadFile(ledger)
			if err != nil {
				return err
			}
			final := trace.fact(string(closing), trace.tip, "")
			trace.publication(final, name)
			trace.begin()
			err = mission.AnchorNamedWithRawAnchorOperations(trace.raw(), state, engine.Root, ledger, name, final.hash, final.sha)
			trace.done()
			return err
		}
		return err
	}
	detail := "host-cap-exhausted: the adapter's native turn cap ended the turn (error_max_turns)"
	if _, err := engine.recordFailedTurn(statePath, ledgerPath, state, turnPath, detail, "failed", 1, true); err != nil {
		t.Fatalf("recordFailedTurn: %v", err)
	}
	ledger, err := os.ReadFile(ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(ledger), "host-cap-exhausted") {
		t.Fatalf("ledger does not name the cap:\n%s", ledger)
	}
}

// The public-resume terminal heal: a completed
// mission owing nothing re-runs delivery idempotently and reconciles to
// a clean verdict; a mission whose records cannot reconcile surfaces
// the runner error instead of silently refusing.
func TestHealTerminalPublication(t *testing.T) {
	bed := crashedFileMission(t, 1, 1)
	engine, statePath, ledgerPath := bed.engine, bed.statePath, bed.ledgerPath
	trace := newThreeAnchorTrace(t, engine.Root, statePath, ledgerPath, "")
	trace.install(engine)
	state, err := readJSONDoc(statePath)
	if err != nil {
		t.Fatal(err)
	}
	beforeHash := state["integrity"].(map[string]any)["hash"]
	beforeLedger, err := os.ReadFile(ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	trace.verify(trace.tip)
	trace.begin()
	if _, _, err := engine.continuity().VerifyStateWithAnchor(statePath, engine.Root, ledgerPath); err != nil {
		t.Fatalf("one-cycle position is not anchored: %v", err)
	}
	trace.done()
	trace.verify(trace.tip)
	trace.begin()
	if err := engine.healTerminalPublication(statePath, state); err != nil {
		t.Fatalf("a consistent completed position must heal cleanly: %v", err)
	}
	trace.done()
	_, afterHash, err := mission.VerifyStateShape(statePath)
	if err != nil {
		t.Fatalf("reconciled state shape: %v", err)
	}
	if afterHash != beforeHash {
		t.Fatalf("reconciliation changed state hash: %s, want %s", afterHash, beforeHash)
	}
	afterLedger, err := os.ReadFile(ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(afterLedger) != string(beforeLedger) {
		t.Fatal("reconciliation changed ledger bytes")
	}
}

// The could-not-run ramps: a git that cannot spawn is the
// runner's failure at every retained-object probe — never a repository
// verdict that could park the mission falsely.
func TestScopeProbesTypeCouldNotRun(t *testing.T) {
	engine := crashedFileMission(t, 0, 1).engine
	engine.wallReadFacts = nil
	acct := &wallAccountant{
		workspace:      gittree.Workspace{Dir: engine.Root},
		ledgerRel:      missionLedgerRel(engine.Mission),
		anchoredLedger: strings.Repeat("a", 40),
	}
	t.Setenv("PATH", filepath.Join(t.TempDir(), "empty"))
	if violation, err := acct.accountedOID(engine, strings.Repeat("b", 40)); err == nil || violation != "" ||
		!strings.Contains(err.Error(), "could not probe retained object") {
		t.Fatalf("a spawn failure must ride the runner ramp: %q, %v", violation, err)
	}
	if violation, err := acct.rawLedgerCarrier(strings.Repeat("c", 40), "commit test"); err == nil || violation != "" {
		t.Fatalf("an unreadable ledger carrier probe must be the runner's error: %q, %v", violation, err)
	}
	if violation, err := engine.judgeCommitLedgerCarrier(strings.Repeat("d", 40), map[string]any{}); err == nil || violation != "" {
		t.Fatalf("a spawn failure must be an error, never a violation: %q, %v", violation, err)
	}
}
