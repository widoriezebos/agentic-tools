package missionrunner

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGitAdapterNonRegularStatePublicPreflight(t *testing.T) {
	engine := buildFullCycleRoot(t, "FAKEHOST:close-stream")
	target := filepath.Join(t.TempDir(), "elsewhere-state.json")
	writeText(t, target, "{}\n")
	statePath := filepath.Join(engine.missionDir(), "state.json")
	if err := os.Symlink(target, statePath); err != nil {
		t.Fatal(err)
	}
	fencesBefore := readTestDoc(t, engine.fencesPath())
	if err := engine.armAndPreflight("start"); err == nil ||
		!strings.Contains(err.Error(), "non-regular object") {
		t.Fatalf("the pin ladder must refuse the shape by name: %v", err)
	}
	if !pathExists(engine.approvedContractPath()) {
		t.Fatal("public refusal swept the pin")
	}
	fencesAfter := readTestDoc(t, engine.fencesPath())
	if fencesBefore["startedAt"] != fencesAfter["startedAt"] {
		t.Fatal("public refusal reset the fence clock")
	}
	if info, err := os.Lstat(statePath); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("public refusal replaced state object: %v", err)
	}
	if !pathExists(target) {
		t.Fatal("public refusal removed symlink target")
	}
}

func TestGitAdapterStillbornCleanupAndCorrectedRetry(t *testing.T) {
	engine := buildFullCycleRoot(t, "FAKEHOST:close-stream")
	commitBedBaseline(t, engine.Root)
	leasePath := filepath.Join(engine.Root, "artifacts", "agents", "checkout.lease.json")
	// The bed's own build already armed and pinned (the parent's half).
	// A fileMode flip in the SAME gap refuses at the child's admission
	// before any dirt considerations.
	fixtureGit(t, engine.Root, "config", "core.fileMode", "false")
	if _, _, _, err := engine.initializeState(leasePath); err == nil ||
		!strings.Contains(err.Error(), "core.fileMode") {
		t.Fatalf("the child must recheck the fileMode pin: %v", err)
	}
	fixtureGit(t, engine.Root, "config", "core.fileMode", "true")
	// The fileMode refusal above CLEANED the stillborn pin (that is the
	// contract under test), so the dirt leg needs its own pin — without
	// it the next attempt would die reading the missing approved
	// contract and never reach admission.
	if err := engine.armAndPreflight("start"); err != nil {
		t.Fatalf("re-pin between legs must succeed on a clean bed: %v", err)
	}
	// Dirt lands in the parent-child gap; the child's re-admission must
	// refuse BY NAME and clean the stillborn ledger and pin.
	writeText(t, filepath.Join(engine.Root, "truth", "gap-dirt.txt"), "dirt\n")
	if _, _, _, err := engine.initializeState(leasePath); err == nil ||
		!strings.Contains(err.Error(), "initial baseline is dirty") {
		t.Fatalf("the child re-admission must refuse the gap dirt by name: %v", err)
	}
	if pathExists(filepath.Join(engine.missionDir(), "ledger.md")) {
		t.Fatal("the stillborn ledger must be removed")
	}
	if pathExists(engine.approvedContractPath()) {
		t.Fatal("the stillborn pin must be removed")
	}
	// A STATE-BIRTH failure sweeps too: admission and the E0 anchor
	// succeed, then the atomic state write fails — a directory landing
	// on state.json mid-birth (after the entry checks) forces exactly
	// that. The sweep must remove the ledger, the pin, AND the anchored
	// E0 ref; the squatting object itself stays untouched.
	if err := os.Remove(filepath.Join(engine.Root, "truth", "gap-dirt.txt")); err != nil {
		t.Fatal(err)
	}
	if err := engine.armAndPreflight("start"); err != nil {
		t.Fatalf("re-pin before the state-birth leg must succeed: %v", err)
	}
	statePath := filepath.Join(engine.missionDir(), "state.json")
	engine.afterApprovedParse = func() {
		if err := os.MkdirAll(statePath, 0o755); err != nil {
			t.Errorf("cannot squat the state path mid-birth: %v", err)
		}
	}
	if _, _, _, err := engine.initializeState(leasePath); err == nil ||
		!strings.Contains(err.Error(), "state initialization refused") {
		t.Fatalf("the mid-birth squatter must fail the state write: %v", err)
	}
	engine.afterApprovedParse = nil
	if pathExists(engine.birthRecordPath()) {
		t.Fatal("a proven same-pass publication failure must unstamp the birth record")
	}
	if err := os.Remove(statePath); err != nil {
		t.Fatal(err)
	}
	if pathExists(filepath.Join(engine.missionDir(), "ledger.md")) {
		t.Fatal("the state-birth failure must sweep the stillborn ledger")
	}
	if pathExists(engine.approvedContractPath()) {
		t.Fatal("the state-birth failure must sweep the stillborn pin")
	}
	anchorList := exec.Command("git", "-C", engine.Root, "for-each-ref",
		"--format=%(refname)", "refs/metasystem/missions/"+engine.Mission+"/")
	if refs, err := anchorList.CombinedOutput(); err != nil || strings.TrimSpace(string(refs)) != "" {
		t.Fatalf("the state-birth failure must drop the stillborn E0 anchor: %q (%v)", refs, err)
	}
	// The corrected retry starts cleanly end to end — and the BIRTH
	// RULE, not the cleanup, is what makes it possible: recreate the
	// stillborn artifacts a failed or interrupted cleanup would leave,
	// and the retry must STILL work.
	writeText(t, filepath.Join(engine.missionDir(), "ledger.md"), "stillborn remnant\n")
	// A surviving FENCES remnant with an old clock must not eat the
	// mission's sealed wall time: the re-pin refreshes startedAt.
	writeJSONFile(t, engine.fencesPath(), map[string]any{
		"schemaVersion": 1, "missionId": engine.Mission,
		"startedAt": "2020-01-01T00:00:00Z", "cycles": 0,
		"reservations":           map[string]any{},
		"approvedContractSha256": strings.Repeat("d", 64),
	})
	fixedNow := time.Now().UTC().Truncate(time.Second)
	engine.Now = func() time.Time { return fixedNow }
	if err := engine.armAndPreflight("start"); err != nil {
		t.Fatalf("the corrected retry must re-pin over remnants: %v", err)
	}
	// The refreshed clock is BOUNDED, not merely different: any stale
	// replacement — 2020 or 2021 alike — would eat sealed wall time.
	refreshed := readTestDoc(t, engine.fencesPath())
	started, _ := refreshed["startedAt"].(string)
	stamp, perr := time.Parse(time.RFC3339, started)
	if perr != nil || !stamp.Equal(fixedNow) {
		t.Fatalf("the stillborn re-pin timestamp = %v, want %v (parse error %v)", stamp, fixedNow, perr)
	}
	signal := filepath.Join(t.TempDir(), "start.json")
	if code := engine.internalRun("start", "metasystem-mission-runner-alpha-fixture-sb", signal); code != 0 {
		t.Fatalf("the corrected retry must give birth cleanly, exit %d", code)
	}
	state := readTestDoc(t, filepath.Join(engine.missionDir(), "state.json"))
	// A TURN genuinely ran: exit 0 alone would also fit a pre-turn
	// fence park eating the remnant's stale clock, so demand a terminal
	// that is NOT the fence park and a booked cycle — a fence park
	// before any turn books nothing.
	status, _ := state["status"].(string)
	if status == "running" || (status == "parked" && state["parkReason"] == "fence") {
		t.Fatalf("the corrected retry must reach a worked terminal, not %q (%v)", status, state["parkReason"])
	}
	ledgerBytes, lerr := os.ReadFile(filepath.Join(engine.missionDir(), "ledger.md"))
	if lerr != nil || !strings.Contains(string(ledgerBytes), "Cycle 1") {
		t.Fatalf("the corrected retry must book its first cycle: %v", lerr)
	}
}
