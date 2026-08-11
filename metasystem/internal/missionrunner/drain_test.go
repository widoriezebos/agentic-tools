package missionrunner

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
)

// The proof of the finite drain and the runner's mission-scoped reap
// (plans/patience-mission-reap-drain.md): authority stops at the reservation
// set, verdicts need the standing reaper's proof bar, every drain
// terminates, and the drain-stalled park round-trips through the resume:
// answer into the heal's distinguishable ledger line.

// seedRunnerRecord writes the runner's own record, which the drain's
// per-pass heartbeat reads; a real runner writes it before its first cycle.
func seedRunnerRecord(t *testing.T, engine *Engine) {
	t.Helper()
	recordPath, _, _ := engine.runnerPaths()
	writeJSONFile(t, recordPath, engine.runnerRecord(os.Getpid(), os.Getpid(), 1, "test-runner"))
}

// fixedCustodian binds the custodian prover to one verdict, standing in for
// the kernel discipline in tests.
func fixedCustodian(verdict identity.Liveness) func(int64, int64, string) identity.Liveness {
	return func(int64, int64, string) identity.Liveness { return verdict }
}

func isoAt(at time.Time) string {
	return at.UTC().Format("2006-01-02T15:04:05Z")
}

// reapFixture builds an engine over a bare artifact tree whose mission fence
// reservations name the given jobs; the reap itself needs no mission state.
func reapFixture(t *testing.T, reserved ...string) *Engine {
	t.Helper()
	engine := NewEngine(t.TempDir(), "demo")
	reservations := map[string]any{}
	for _, job := range reserved {
		reservations[job] = map[string]any{}
	}
	writeJSONFile(t, engine.fencesPath(), map[string]any{
		"schemaVersion": 1, "missionId": "demo", "startedAt": "2026-08-11T00:00:00Z",
		"cycles": 1, "reservations": reservations,
	})
	return engine
}

func writeJob(t *testing.T, engine *Engine, job string, fields map[string]any) string {
	t.Helper()
	doc := map[string]any{"jobId": job, "mission": "demo"}
	for key, value := range fields {
		doc[key] = value
	}
	path := filepath.Join(jobsDirPath(engine.Root), job+".json")
	writeJSONFile(t, path, doc)
	return path
}

func TestRunnerReapRefusals(t *testing.T) {
	now := time.Now()
	running := func() map[string]any {
		return map[string]any{"status": "running", "pid": 4242, "pidStartedAt": 100, "instanceTag": "job-tag"}
	}
	cases := []struct {
		name      string
		reserved  bool
		custodian identity.Liveness
		fields    map[string]any
	}{
		{"a foreign record is outside the runner's authority", false, identity.Dead, running()},
		{"an Unknown custodian never reaps", true, identity.Unknown, running()},
		{"a live custodian is not death", true, identity.Alive, running()},
		{"facts without death prove nothing", true, identity.Alive, map[string]any{
			"status": "running", "pid": 4242, "pidStartedAt": 100, "instanceTag": "job-tag",
			// The budget is long expired, but the custodian is alive: the
			// runner has no kill authority, so no verdict lands.
			"startedAt": isoAt(now.Add(-2 * time.Hour)), "capMin": 5,
		}},
		{"an expired handshake with no recorded process survives", true, identity.Dead, map[string]any{
			"status": "pending", "sessionEstablishedTimeoutSec": 5,
			"handshakeDeadline": now.Add(-time.Hour).Unix(),
			"startedAt":         isoAt(now.Add(-time.Hour)),
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var engine *Engine
			if tc.reserved {
				engine = reapFixture(t, "job-a")
			} else {
				engine = reapFixture(t)
			}
			engine.custodianFn = fixedCustodian(tc.custodian)
			path := writeJob(t, engine, "job-a", tc.fields)
			engine.reapReservedRecords(now)
			after := readTestDoc(t, path)
			if after["status"] != tc.fields["status"] {
				t.Fatalf("the record must be left alone: %v", after)
			}
			if _, stamped := after["error"]; stamped {
				t.Fatalf("no verdict may land: %v", after)
			}
		})
	}
}

func TestRunnerReapRefusesRecordThatOutranTheCompare(t *testing.T) {
	engine := reapFixture(t, "job-a")
	engine.custodianFn = fixedCustodian(identity.Dead)
	now := time.Now()
	path := writeJob(t, engine, "job-a", map[string]any{
		"status": "running", "pid": 4242, "pidStartedAt": 100, "instanceTag": "job-tag",
	})
	doc := readTestDoc(t, path)
	facts, err := dispatch.ComputeReapFacts(path, dispatch.HandshakeBackstopGraceSec, now)
	if err != nil {
		t.Fatal(err)
	}
	// The record advances under another authority between the facts read and
	// the compare-and-swap: the lost compare must leave it exactly as it is.
	advanced := readTestDoc(t, path)
	advanced["status"] = "completed"
	writeJSONFile(t, path, advanced)
	engine.applyReapVerdict("job-a", doc, facts)
	after := readTestDoc(t, path)
	if after["status"] != "completed" {
		t.Fatalf("a record that outran the compare must be untouched: %v", after)
	}
	if _, stamped := after["error"]; stamped {
		t.Fatalf("no verdict may land on a lost compare: %v", after)
	}
}

func TestRunnerReapVerdicts(t *testing.T) {
	now := time.Now()

	t.Run("a dead custodian past budget books timeout budget-cap", func(t *testing.T) {
		engine := reapFixture(t, "job-a")
		engine.custodianFn = fixedCustodian(identity.Dead)
		path := writeJob(t, engine, "job-a", map[string]any{
			"status": "running", "pid": 4242, "pidStartedAt": 100, "instanceTag": "job-tag",
			"capDeadline": isoAt(now.Add(-time.Minute)),
		})
		engine.reapReservedRecords(now)
		after := readTestDoc(t, path)
		if after["status"] != "timeout" || after["error"] != "budget-cap" {
			t.Fatalf("budget expiry on a dead custodian is judged first: %v", after)
		}
		if ended, _ := after["endedAt"].(string); ended == "" {
			t.Fatalf("a terminal verdict must stamp endedAt: %v", after)
		}
	})

	t.Run("a dead custodian otherwise books failed process-lost", func(t *testing.T) {
		engine := reapFixture(t, "job-a")
		engine.custodianFn = fixedCustodian(identity.Dead)
		path := writeJob(t, engine, "job-a", map[string]any{
			"status": "running", "pid": 4242, "pidStartedAt": 100, "instanceTag": "job-tag",
			"capDeadline": isoAt(now.Add(time.Hour)),
		})
		engine.reapReservedRecords(now)
		after := readTestDoc(t, path)
		if after["status"] != "failed" || after["error"] != "process-lost" {
			t.Fatalf("a dead custodian inside budget is process-lost: %v", after)
		}
	})

	t.Run("a pending record with a dead custodian books failed process-lost", func(t *testing.T) {
		engine := reapFixture(t, "job-a")
		engine.custodianFn = fixedCustodian(identity.Dead)
		path := writeJob(t, engine, "job-a", map[string]any{
			"status": "pending", "pid": 4242, "pidStartedAt": 100, "instanceTag": "job-tag",
		})
		engine.reapReservedRecords(now)
		after := readTestDoc(t, path)
		if after["status"] != "failed" || after["error"] != "process-lost" {
			t.Fatalf("a pending record with a dead custodian is process-lost: %v", after)
		}
	})

	t.Run("a setup husk past grace books failed abandoned-setup without custodian proof", func(t *testing.T) {
		engine := reapFixture(t, "job-husk")
		engine.custodianFn = func(int64, int64, string) identity.Liveness {
			t.Fatal("a never-launched husk needs no custodian proof")
			return identity.Unknown
		}
		// A husk dies before its mission stamp is written; the reservation
		// key is what makes it this mission's to judge.
		path := writeJob(t, engine, "job-husk", map[string]any{
			"mission": "", "status": "pending-setup",
			"createdAt": isoAt(now.Add(-20 * time.Minute)),
		})
		engine.reapReservedRecords(now)
		after := readTestDoc(t, path)
		if after["status"] != "failed" || after["error"] != "abandoned-setup" {
			t.Fatalf("an abandoned husk provably never launched: %v", after)
		}
	})

	t.Run("a setup husk inside grace is left alone", func(t *testing.T) {
		engine := reapFixture(t, "job-husk")
		path := writeJob(t, engine, "job-husk", map[string]any{
			"mission": "", "status": "pending-setup",
			"createdAt": isoAt(now.Add(-time.Minute)),
		})
		engine.reapReservedRecords(now)
		if after := readTestDoc(t, path); after["status"] != "pending-setup" {
			t.Fatalf("a live dispatcher must never be raced: %v", after)
		}
	})
}

func TestDrainDeadlinePerRecordClocks(t *testing.T) {
	now := time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC)
	grace := time.Duration(dispatch.HandshakeBackstopGraceSec) * time.Second

	capped := jobRecord{doc: map[string]any{"status": "running", "capDeadline": isoAt(now.Add(10 * time.Minute))}}
	if due := recordDrainDue(capped.doc, now); !due.Equal(now.Add(10 * time.Minute).Add(grace)) {
		t.Fatalf("a parseable capDeadline plus the handshake grace is the clock: %v", due)
	}
	launched := jobRecord{doc: map[string]any{"status": "running", "startedAt": isoAt(now.Add(-5 * time.Minute)), "capMin": 30}}
	if due := recordDrainDue(launched.doc, now); !due.Equal(now.Add(25 * time.Minute).Add(grace)) {
		t.Fatalf("a launched record without capDeadline uses startedAt plus its own capMin: %v", due)
	}
	husk := jobRecord{doc: map[string]any{"status": "pending-setup", "createdAt": isoAt(now.Add(-2 * time.Minute))}}
	if due := recordDrainDue(husk.doc, now); !due.Equal(now.Add(-2 * time.Minute).Add(dispatch.AbandonedSetupGrace)) {
		t.Fatalf("a pending-setup husk uses createdAt plus the setup grace: %v", due)
	}
	bare := jobRecord{doc: map[string]any{"status": "pending"}}
	if due := recordDrainDue(bare.doc, now); due.After(now) {
		t.Fatalf("a record with nothing parseable is already due: %v", due)
	}

	// The set deadline is the latest surviving clock — here the launched
	// record's fallback, 25 minutes plus grace out.
	deadline := drainDeadline([]jobRecord{capped, launched, husk, bare}, now)
	if !deadline.Equal(now.Add(25 * time.Minute).Add(grace)) {
		t.Fatalf("the deadline must be the latest per-record clock: %v", deadline)
	}
}

func TestDrainDeadlineRecomputesOverTheCurrentSet(t *testing.T) {
	now := time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC)
	overdue := jobRecord{doc: map[string]any{"status": "pending"}}
	if deadline := drainDeadline([]jobRecord{overdue}, now); deadline.After(now) {
		t.Fatalf("an unparseable record alone must already be due: %v", deadline)
	}
	// A follow-up reserved mid-drain joins the active set and lawfully
	// extends the next pass's deadline: new real work, new clock.
	followUp := jobRecord{doc: map[string]any{"status": "running", "capDeadline": isoAt(now.Add(15 * time.Minute))}}
	if deadline := drainDeadline([]jobRecord{overdue, followUp}, now); !deadline.After(now) {
		t.Fatal("a record reserved mid-drain must extend the recomputed deadline")
	}
}

// drainContract carries the streams InitState reads; the drain and its park
// never touch the gate.
const drainContract = "# Intent\n\n```mission\ncandidate.branch=main\nstream.primary=Do the work\n```\n"

// drainMission builds a running mission whose fence reservations name the
// given jobs, with a stubbed anchor and a seeded runner record so the
// drain's heartbeat and park have everything a real runner has.
func drainMission(t *testing.T, reserved map[string]any) (engine *Engine, statePath, ledgerPath string) {
	t.Helper()
	engine = NewEngine(t.TempDir(), "demo")
	engine.anchorFn = func(string, string, string) error { return nil }
	seedRunnerRecord(t, engine)
	dir := engine.missionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	contractPath := filepath.Join(dir, "mission-demo.contract.md")
	if err := os.WriteFile(contractPath, []byte(drainContract), 0o644); err != nil {
		t.Fatal(err)
	}
	writeJSONFile(t, engine.fencesPath(), map[string]any{
		"schemaVersion": 1, "missionId": "demo", "startedAt": "2026-08-11T00:00:00Z",
		"cycles": 1, "reservations": reserved,
	})
	statePath = filepath.Join(dir, "state.json")
	ledgerPath = filepath.Join(dir, "ledger.md")
	if err := mission.InitLedger(ledgerPath, 8, 4); err != nil {
		t.Fatal(err)
	}
	if err := mission.InitState(statePath, contractPath, ledgerPath, "", "main"); err != nil {
		t.Fatal(err)
	}
	return engine, statePath, ledgerPath
}

// writeUnprovableSurvivor writes the one record the runner may never reap: a
// pending job whose handshake expired with no recorded process, its cap
// clock long past — the drain must park rather than fail it.
func writeUnprovableSurvivor(t *testing.T, engine *Engine, job string, now time.Time) {
	t.Helper()
	writeJob(t, engine, job, map[string]any{
		"status": "pending", "sessionEstablishedTimeoutSec": 5,
		"handshakeDeadline": now.Add(-time.Hour).Unix(),
		"startedAt":         isoAt(now.Add(-2 * time.Hour)), "capMin": 5,
	})
}

func TestDrainClearsReapableHuskWithoutParking(t *testing.T) {
	engine, statePath, ledgerPath := drainMission(t, map[string]any{"job-husk": map[string]any{}})
	engine.custodianFn = fixedCustodian(identity.Unknown)
	path := writeJob(t, engine, "job-husk", map[string]any{
		"mission": "", "status": "pending-setup",
		"createdAt": isoAt(time.Now().Add(-20 * time.Minute)),
	})
	parked, err := engine.drainJobs(statePath, ledgerPath, "demo-t1-aaaa", 1)
	if err != nil || parked != nil {
		t.Fatalf("a reapable husk must clear without parking: parked=%v err=%v", parked, err)
	}
	after := readTestDoc(t, path)
	if after["status"] != "failed" || after["error"] != "abandoned-setup" {
		t.Fatalf("the husk must be reaped by the runner itself: %v", after)
	}
	if state := readTestDoc(t, statePath); state["status"] != "running" {
		t.Fatalf("a clean drain must not park: %v", state["status"])
	}
}

func TestDrainParksStalledAtDeadline(t *testing.T) {
	engine, statePath, ledgerPath := drainMission(t, map[string]any{"job-ghost": map[string]any{}})
	writeUnprovableSurvivor(t, engine, "job-ghost", time.Now())
	parked, err := engine.drainJobs(statePath, ledgerPath, "demo-t1-aaaa", 1)
	if err != nil {
		t.Fatal(err)
	}
	if parked == nil || parked["status"] != "parked" || parked["parkReason"] != "drain-stalled" {
		t.Fatalf("an unprovable survivor must park at the deadline: %v", parked)
	}
	askPath := filepath.Join(asksDirPath(engine.Root, "demo"), "drain-stalled.json")
	ask := readTestDoc(t, askPath)
	if ask["reasonClass"] != "drain-stalled" || ask["answeredAt"] != nil {
		t.Fatalf("the park must raise the survivors ask: %v", ask)
	}
	question, _ := ask["question"].(string)
	if !strings.Contains(question, "job-ghost") ||
		!strings.Contains(question, "handshake expired, no recorded process to prove") {
		t.Fatalf("the ask must name the survivor and its missing proof: %q", question)
	}
	stall, _ := ask["drainStall"].(map[string]any)
	if cycle, _ := jsonInt(stall["cycle"]); cycle != 1 {
		t.Fatalf("the snapshot must carry the reserved cycle: %v", stall)
	}
	if survivors, _ := stall["survivors"].([]any); len(survivors) != 1 || survivors[0] != "job-ghost" {
		t.Fatalf("the snapshot must carry the survivor ids: %v", stall)
	}
	waiting, _ := parked["waitingList"].([]any)
	if len(waiting) != 1 || waiting[0] != "drain-stalled" {
		t.Fatalf("the parked state must name the ask: %v", waiting)
	}
	// The park writes NO ledger line: the reserved cycle never concluded,
	// which is exactly the gap the resume heal recovers.
	if _, _, cycles, err := mission.ParseLedger(ledgerPath); err != nil || len(cycles) != 0 {
		t.Fatalf("a drain-stalled park must not book the cycle: %v (%d)", err, len(cycles))
	}
	// The drain beat the runner heartbeat, carrying the turn it was draining.
	_, heartbeatPath, _ := engine.runnerPaths()
	heartbeat := readTestDoc(t, heartbeatPath)
	if heartbeat["turnId"] != "demo-t1-aaaa" {
		t.Fatalf("the drain must beat the runner heartbeat every pass: %v", heartbeat)
	}
}

func TestDrainStalledParkWritesStateThenAsk(t *testing.T) {
	engine, statePath, ledgerPath := drainMission(t, map[string]any{"job-ghost": map[string]any{}})
	writeUnprovableSurvivor(t, engine, "job-ghost", time.Now())
	// The anchor sits between the state write and the ask write: failing it
	// proves the order — the parked state lands, the ask does not.
	engine.anchorFn = func(string, string, string) error { return errors.New("anchor unavailable") }
	if _, err := engine.drainJobs(statePath, ledgerPath, "demo-t1-aaaa", 1); err == nil {
		t.Fatal("a failed park must surface its error")
	}
	if state := readTestDoc(t, statePath); state["status"] != "parked" || state["parkReason"] != "drain-stalled" {
		t.Fatalf("the state write comes first: %v", state)
	}
	askPath := filepath.Join(asksDirPath(engine.Root, "demo"), "drain-stalled.json")
	if pathExists(askPath) {
		t.Fatal("the ask write comes after the state write")
	}

	// Resume's recovery: the missing ask is re-raised idempotently from the
	// live set, before anything else.
	engine.anchorFn = func(string, string, string) error { return nil }
	state := readTestDoc(t, statePath)
	if err := engine.ensureDrainStallAsk(state); err != nil {
		t.Fatal(err)
	}
	ask := readTestDoc(t, askPath)
	if ask["reasonClass"] != "drain-stalled" || ask["answeredAt"] != nil {
		t.Fatalf("the re-raised ask must be open: %v", ask)
	}
	stall, _ := ask["drainStall"].(map[string]any)
	if cycle, _ := jsonInt(stall["cycle"]); cycle != 1 {
		t.Fatalf("the re-raised snapshot must carry the reserved cycle: %v", stall)
	}
	if survivors, _ := stall["survivors"].([]any); len(survivors) != 1 || survivors[0] != "job-ghost" {
		t.Fatalf("the re-raised snapshot re-proves against the live set: %v", stall)
	}
	// Idempotent: a second resume creates no duplicate.
	if err := engine.ensureDrainStallAsk(state); err != nil {
		t.Fatal(err)
	}
	paths, _ := filepath.Glob(filepath.Join(asksDirPath(engine.Root, "demo"), "drain-stalled*.json"))
	if len(paths) != 1 {
		t.Fatalf("re-raising must not duplicate the ask: %v", paths)
	}
}

// parkedDrainStalledMission drives a real drain into its park, returning the
// parked mission and its ask.
func parkedDrainStalledMission(t *testing.T) (engine *Engine, statePath, ledgerPath, askPath string) {
	t.Helper()
	engine, statePath, ledgerPath = drainMission(t, map[string]any{"job-ghost": map[string]any{}})
	writeUnprovableSurvivor(t, engine, "job-ghost", time.Now())
	parked, err := engine.drainJobs(statePath, ledgerPath, "demo-t1-aaaa", 1)
	if err != nil || parked == nil {
		t.Fatalf("fixture park failed: parked=%v err=%v", parked, err)
	}
	askPath = filepath.Join(asksDirPath(engine.Root, "demo"), "drain-stalled.json")
	return engine, statePath, ledgerPath, askPath
}

func TestAnswerDrainStalledResumeUnparksAndWritesTheLabel(t *testing.T) {
	engine, statePath, _, askPath := parkedDrainStalledMission(t)
	// Every other answer shape keeps the refusal, vocally.
	for _, bad := range []string{"acknowledged", "reset: wrong verb", "resume:", "resume:   "} {
		if code := engine.Answer("drain-stalled", bad); code == 0 {
			t.Fatalf("%q must be refused", bad)
		}
	}
	if state := readTestDoc(t, statePath); state["status"] != "parked" {
		t.Fatal("refused answers must leave the park in place")
	}
	if ask := readTestDoc(t, askPath); ask["answeredAt"] != nil {
		t.Fatal("refused answers must leave the ask open")
	}

	if code := engine.Answer("drain-stalled", "resume: verified and cleared the ghost job by hand"); code != 0 {
		t.Fatalf("the resume: answer was refused with exit %d", code)
	}
	ask := readTestDoc(t, askPath)
	if ask["answeredAt"] == nil || ask["answer"] != "resume: verified and cleared the ghost job by hand" {
		t.Fatalf("the ask must be marked answered: %v", ask)
	}
	state := readTestDoc(t, statePath)
	if state["status"] != "running" || state["parkReason"] != nil {
		t.Fatalf("the resume: answer must unpark: %v %v", state["status"], state["parkReason"])
	}
	stall, ok := state["lastDrainStall"].(map[string]any)
	if !ok {
		t.Fatalf("the unpark must write the durable label: %v", state)
	}
	if cycle, _ := jsonInt(stall["cycle"]); cycle != 1 {
		t.Fatalf("the label must carry the stalled cycle: %v", stall)
	}
	if survivors, _ := stall["survivors"].([]any); len(survivors) != 1 || survivors[0] != "job-ghost" {
		t.Fatalf("the label must carry the snapshotted survivors: %v", stall)
	}
	if waiting, _ := state["waitingList"].([]any); len(waiting) != 0 {
		t.Fatalf("the answered ask must leave the waiting list: %v", waiting)
	}
	// One-shot: the answered ask refuses a second answer.
	if code := engine.Answer("drain-stalled", "resume: again"); code == 0 {
		t.Fatal("an already answered ask must refuse a second answer")
	}
}

func TestAnswerDrainStalledRollsBackWhenTheStateWriteRefuses(t *testing.T) {
	engine, statePath, _, askPath := parkedDrainStalledMission(t)
	// A snapshot the state validator refuses (a survivor id outside the id
	// grammar) makes the unpark's state write fail after the ask was marked
	// answered — the rollback window under test.
	ask := readTestDoc(t, askPath)
	ask["drainStall"].(map[string]any)["survivors"] = []any{"NOT A JOB ID"}
	writeJSONFile(t, askPath, ask)
	if code := engine.Answer("drain-stalled", "resume: cleared"); code == 0 {
		t.Fatal("a refused state write must refuse the answer")
	}
	// No ledger line exists on this path, so the ask and the state advance
	// together or not at all.
	if ask := readTestDoc(t, askPath); ask["answeredAt"] != nil {
		t.Fatal("the ask must be rolled back")
	}
	state := readTestDoc(t, statePath)
	if state["status"] != "parked" {
		t.Fatal("the mission must stay parked")
	}
	if _, present := state["lastDrainStall"]; present {
		t.Fatal("no label may land without the unpark")
	}
}

// measurableDrainMission builds a mission over a real measurable repository
// (the heal_test/faulted_test fixture pattern): sealed contract in plans/,
// pinned approved snapshot, gate instruments committed and tagged, and a
// gate whose score comes from a committed file so the tree can improve.
func measurableDrainMission(t *testing.T) (engine *Engine, statePath, ledgerPath string, git func(args ...string) string, write func(rel, content string, mode os.FileMode)) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	git = func(args ...string) string {
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
	write = func(rel, content string, mode os.FileMode) {
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
	write("scripts/gate.sh", "#!/usr/bin/env bash\nset -euo pipefail\nprintf 'metric=score=%s\\nmetric=audit=1\\n' \"$(cat score.txt)\"\n", 0o755)
	write("score.txt", "1", 0o644)
	write("truth/reference.txt", "certified truth\n", 0o644)
	write("docs/project-rules.md", faultedProjectRules, 0o644)
	git("add", ".")
	git("commit", "-qm", "instruments")
	git("tag", "instruments")
	git("checkout", "-q", "-B", "main")

	engine = NewEngine(root, "demo")
	engine.anchorFn = func(string, string, string) error { return nil }
	seedRunnerRecord(t, engine)
	write(filepath.Join("plans", "mission-demo.contract.md"), faultedContract(), 0o644)
	if _, err := mission.ContractSeal(engine.contractPath()); err != nil {
		t.Fatalf("seal failed: %v", err)
	}
	sealedBytes, err := os.ReadFile(engine.contractPath())
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
		"cycles": 1, "reservations": map[string]any{"job-ghost": map[string]any{}},
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
	return engine, statePath, ledgerPath, git, write
}

// TestDrainStallEndToEnd drives the whole severed shape: the drain parks at
// its deadline, the human answers resume:, the heal books the cycle as
// honestly lost with the drain-stalled label, and the NEXT cycle measures
// the committed tree — the ratchet banks the value one cycle late rather
// than never.
func TestDrainStallEndToEnd(t *testing.T) {
	engine, statePath, ledgerPath, git, write := measurableDrainMission(t)

	// The stalled cycle's work landed in the tree before the drain wedged.
	write("score.txt", "2", 0o644)
	git("add", "score.txt")
	git("commit", "-qm", "the stalled cycle's committed improvement")

	// 1) The drain parks: an unprovable survivor at an expired deadline.
	writeUnprovableSurvivor(t, engine, "job-ghost", time.Now())
	parked, err := engine.drainJobs(statePath, ledgerPath, "demo-t1-abcd", 1)
	if err != nil || parked == nil || parked["parkReason"] != "drain-stalled" {
		t.Fatalf("the drain must park: parked=%v err=%v", parked, err)
	}
	if _, _, cycles, err := mission.ParseLedger(ledgerPath); err != nil || len(cycles) != 0 {
		t.Fatalf("the park writes no ledger line: %v (%d)", err, len(cycles))
	}

	// 2) The human answers resume: — unparked, labeled.
	if code := engine.Answer("drain-stalled", "resume: ghost job verified dead by hand"); code != 0 {
		t.Fatalf("resume answer refused with exit %d", code)
	}

	// 3) The heal books the reserved cycle distinguishably and clears the
	//    label in the same conclude write.
	healed, err := engine.healReservedCycle(statePath, ledgerPath, readTestDoc(t, statePath))
	if err != nil || !healed {
		t.Fatalf("the resume heal must book the stalled cycle: healed=%v err=%v", healed, err)
	}
	ledger, _ := os.ReadFile(ledgerPath)
	if !strings.Contains(string(ledger), "observed=unmeasurable:drain-stalled; best=no") ||
		!strings.Contains(string(ledger), "\n- Drain: stalled:1\n") {
		t.Fatalf("the healed line must carry the drain-stalled observation and count:\n%s", ledger)
	}
	state := readTestDoc(t, statePath)
	if _, present := state["lastDrainStall"]; present {
		t.Fatal("the label must be consumed by the heal")
	}

	// 4) The next cycle measures the committed tree and the ratchet banks it.
	classification, observed, measurement, gatePassed := engine.measure(state)
	if classification != "contract-improved" || !gatePassed {
		t.Fatalf("the next cycle must measure the committed tree: %s %s gate=%v", classification, observed, gatePassed)
	}
	candidateSHA, _ := measurement["candidateSha"].(string)
	if err := engine.appendLedger(state, ledgerPath, 2, classification, candidateSHA, observed); err != nil {
		t.Fatal(err)
	}
	ledger, _ = os.ReadFile(ledgerPath)
	if !strings.Contains(string(ledger), "### Cycle 2\n- Classification: contract-improved;") ||
		!strings.Contains(string(ledger), "best=yes") {
		t.Fatalf("the ratchet must bank the committed value one cycle late:\n%s", ledger)
	}
	// The banked best resets stagnation: the stalled cycle registered
	// exactly once and the fuse holds.
	verdict, err := engine.stopLossVerdict(state, ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	if verdict.Cycles != 2 || verdict.Stagnant != 0 || verdict.Tripped {
		t.Fatalf("replay after the banked best: %+v", verdict)
	}
}

func TestSecondDrainStallParksWithAFreshAsk(t *testing.T) {
	engine, statePath, ledgerPath, askPath := parkedDrainStalledMission(t)
	if code := engine.Answer("drain-stalled", "resume: first stall acknowledged"); code != 0 {
		t.Fatal("first resume refused")
	}
	// The survivor is still there: the next drain stalls again and parks
	// with a fresh ask beside the answered one.
	parked, err := engine.drainJobs(statePath, ledgerPath, "demo-t2-bbbb", 2)
	if err != nil || parked == nil {
		t.Fatalf("the second stall must park again: parked=%v err=%v", parked, err)
	}
	first := readTestDoc(t, askPath)
	if first["answeredAt"] == nil {
		t.Fatal("the first ask stays answered")
	}
	second := readTestDoc(t, filepath.Join(asksDirPath(engine.Root, "demo"), "drain-stalled-2.json"))
	if second["answeredAt"] != nil || second["reasonClass"] != "drain-stalled" {
		t.Fatalf("the second park needs a fresh open ask: %v", second)
	}
	stall, _ := second["drainStall"].(map[string]any)
	if cycle, _ := jsonInt(stall["cycle"]); cycle != 2 {
		t.Fatalf("the fresh ask snapshots the new cycle: %v", stall)
	}
}
