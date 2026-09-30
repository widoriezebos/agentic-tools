package delegation_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegation"
)

func TestStatusPrintsTheRecordAndTheCensusVerdict(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.writeRecord("job-a", map[string]any{"status": "running"})
	result := b.run("status", "--job", "job-a")
	requireExit(t, result, 0, b.stderr.String())
	if string(result.Stdout) != "running\n" || strings.TrimSpace(b.stderr.String()) != "last census: none recorded" {
		t.Fatalf("stdout %q stderr %q", result.Stdout, b.stderr.String())
	}
	b.writeFile("artifacts/agents/supervision/last-census.json", `{"verdict":"CLEAN","completedAtEpoch":`+jsonInt(b.doubles.Clock.Now().Unix()-7)+`,"fingerprint":"fp-1"}`)
	b.run("status", "--job", "job-a")
	if got := strings.TrimSpace(b.stderr.String()); got != "last census: CLEAN, 7s ago (fingerprint fp-1)" {
		t.Fatalf("census line %q", got)
	}
	b.writeRecord("job-b", map[string]any{"status": "pending-setup"})
	requireExit(t, b.run("status", "--job", "job-b"), 7, b.stderr.String())
	requireExit(t, b.run("status", "--job", "Bad_Id"), 2, b.stderr.String())
}

func TestWatchKnowsAVanishedJobNowAndOtherwiseRidesTheWatcher(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	result := b.run("watch", "--job", "never-was")
	requireExit(t, result, 5, b.stderr.String())
	if !strings.Contains(b.stderr.String(), "never existed here or was reaped") {
		t.Fatalf("stderr %q", b.stderr.String())
	}
	b.writeRecord("job-a", map[string]any{"status": "running", "workspaceRoot": "/work/tree"})
	b.doubles.Host.WatchFunc = func(root, job string, callerPid int64, progressRoot string) int {
		if root != b.root || job != "job-a" || callerPid != int64(os.Getpid()) || progressRoot != "/work/tree" {
			t.Errorf("watch(%s, %s, %d, %s)", root, job, callerPid, progressRoot)
		}
		return 8
	}
	requireExit(t, b.run("watch", "--job", "job-a"), 8, b.stderr.String())
	b.writeRecord("job-b", map[string]any{"status": "running"})
	b.doubles.Host.WatchFunc = func(_, _ string, _ int64, progressRoot string) int {
		if progressRoot != b.root {
			t.Errorf("a record without a workspace is watched from %s, want the root", progressRoot)
		}
		return 0
	}
	requireExit(t, b.run("watch", "--job", "job-b"), 0, b.stderr.String())
}

// --- cancel -----------------------------------------------------------

func TestCancelConcludesARecordThatNeverPublishedAProcess(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.writeRecord("husk", map[string]any{"status": "pending-setup"})
	result := b.run("cancel", "--job", "husk")
	requireExit(t, result, 0, b.stderr.String())
	record := b.record("husk")
	if record["status"] != "cancelled" || record["phase"] != "cancelled" {
		t.Fatalf("record %v", record)
	}
	if _, has := record["groupDeathProvenAt"]; has {
		t.Fatal("a record that never had a group claims a death")
	}
	if len(b.doubles.Process.Signals) != 0 || len(b.calls("adapter.Cancel")) != 0 {
		t.Fatalf("an unpublished record was signalled or sent to its runtime: %v", b.doubles.Log.Calls())
	}
	if len(b.calls("lease.Held")) != 1 || len(b.calls("lease.Authorize pid")) != 3 || !strings.Contains(b.calls("lease.Authorize")[0], "mode=holder-only job=husk") {
		t.Fatalf("the owned cancel ran outside the lease or without its authority: %v", b.doubles.Log.Calls())
	}
}

func TestCancelOfALaunchedJobGoesThroughItsRuntime(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.writeRecord("live", map[string]any{"status": "running", "pid": 4242, "pgid": 4242})
	var cancelled []string
	b.doubles.Adapter.CancelFunc = func(runtime, job string) error {
		cancelled = append(cancelled, runtime+":"+job)
		return nil
	}
	requireExit(t, b.run("cancel", "--job", "live"), 0, b.stderr.String())
	if !reflect.DeepEqual(cancelled, []string{"fake:live"}) || len(b.calls("lease.Held")) != 1 {
		t.Fatalf("cancelled %v calls %v", cancelled, b.doubles.Log.Calls())
	}
	b.doubles.Adapter.CancelFunc = func(string, string) error {
		return &delegation.AdapterCancelError{Code: 4, Output: "runtime refused"}
	}
	requireExit(t, b.run("cancel", "--job", "live"), 4, b.stderr.String())
	requireExit(t, b.run("cancel", "--job", "absent"), 1, b.stderr.String())
}

// The owned cancel marks the record before any signal, TERMs the owned
// group, escalates to KILL when TERM is ignored, and stamps the death.
func TestOwnedCancelMarksThenWindsDownThenConcludes(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.writeRecord("stubborn", map[string]any{"status": "running", "pid": 7001, "pgid": 7001})
	b.doubles.Process.Groups[7001] = true
	b.doubles.Process.Owned[7001] = true
	b.doubles.Process.Survivors = map[int64]bool{7001: true}
	requireExit(t, b.run("__cancel-owned", "--job", "stubborn"), 0, b.stderr.String())
	record := b.record("stubborn")
	if record["status"] != "cancelled" || record["cancelEscalatedToKill"] != true || record["groupDeathProvenAt"] == nil {
		t.Fatalf("record %v", record)
	}
	if !reflect.DeepEqual(b.doubles.Process.Signals, []string{"7001:15", "7001:9"}) {
		t.Fatalf("signals %v", b.doubles.Process.Signals)
	}
}

func TestOwnedCancelRefusesToSignalAnUnownedGroup(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.writeRecord("stranger", map[string]any{"status": "running", "pid": 7002, "pgid": 7002})
	b.doubles.Process.Groups[7002] = true
	requireExit(t, b.run("__cancel-owned", "--job", "stranger"), 1, b.stderr.String())
	if !strings.Contains(b.stderr.String(), "refusing to signal unowned process group 7002") || len(b.doubles.Process.Signals) != 0 {
		t.Fatalf("stderr %q signals %v", b.stderr.String(), b.doubles.Process.Signals)
	}
	if record := b.record("stranger"); record["status"] != "running" || record["phase"] != "cancelling" {
		t.Fatalf("the marker must stand while the kill is refused: %v", record)
	}
}

func TestOwnedCancelRefusesAProcessWithoutAGroup(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.writeRecord("groupless", map[string]any{"status": "running", "pid": 7003})
	result := b.run("__cancel-owned", "--job", "groupless")
	requireExit(t, result, 1, b.stderr.String())
	if !strings.Contains(b.stderr.String(), "groupless has a recorded process but no primary process group") {
		t.Fatalf("stderr %q", b.stderr.String())
	}
	if outcome := outcomeOf(t, result); outcome["outcome"] != "REFUSED-INTERNAL" {
		t.Fatalf("outcome %v", outcome)
	}
}

func TestOwnedCancelOfATerminalRecordIsANoOp(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.writeRecord("done", map[string]any{"status": "completed"})
	requireExit(t, b.run("__cancel-owned", "--job", "done"), 0, b.stderr.String())
	if b.record("done")["status"] != "completed" {
		t.Fatal("a terminal record was cancelled")
	}
}

// --- reap -------------------------------------------------------------

func TestReapConcludesALostProcessAndWitnessesTheVerdict(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.writeRecord("lost", map[string]any{"status": "running", "pid": 7101, "pgid": 7101, "sessionId": "s-1",
		"startedAt": b.doubles.Clock.Now().Format(time.RFC3339), "capMin": 60})
	requireExit(t, b.run("reap", "--job", "lost"), 0, b.stderr.String())
	record := b.record("lost")
	if record["status"] != "failed" || record["error"] != "process-lost" || record["groupDeathProvenAt"] == nil {
		t.Fatalf("record %v", record)
	}
	if names := b.eventNames(); !reflect.DeepEqual(names, []string{"job-verdict"}) {
		t.Fatalf("events %v", names)
	}
	if len(b.calls("lease.RequireHolder")) != 1 || len(b.calls("lease.Held")) != 1 {
		t.Fatalf("the reap ran outside the entry check or the lease: %v", b.doubles.Log.Calls())
	}
}

func TestReapJudgesTheCapBeforeLiveness(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	started := b.doubles.Clock.Now().Add(-2 * time.Hour)
	b.writeRecord("capped", map[string]any{"status": "running", "pid": 7201, "pgid": 7201, "sessionId": "s-1",
		"startedAt": started.UTC().Format(time.RFC3339), "capMin": 30})
	b.doubles.Process.Tags[7201] = "tag-capped"
	b.doubles.Process.Groups[7201] = true
	b.doubles.Process.Owned[7201] = true
	requireExit(t, b.run("reap", "--job", "capped"), 0, b.stderr.String())
	record := b.record("capped")
	if record["status"] != "timeout" || record["error"] != "budget-cap" {
		t.Fatalf("record %v", record)
	}
	if !reflect.DeepEqual(b.doubles.Process.Signals, []string{"7201:15"}) {
		t.Fatalf("signals %v", b.doubles.Process.Signals)
	}
}

func TestReapDefersToALiveOrUninspectableSupervisor(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	for _, job := range []string{"live", "unknown"} {
		b.writeRecord(job, map[string]any{"status": "running", "pid": 7301, "pgid": 7301, "sessionId": "s-1",
			"startedAt": b.doubles.Clock.Now().Format(time.RFC3339), "capMin": 60})
	}
	b.doubles.Process.Tags[7301] = "tag-live"
	requireExit(t, b.run("reap", "--job", "live"), 0, b.stderr.String())
	b.doubles.Process.Unknown = map[int64]bool{7301: true}
	requireExit(t, b.run("reap", "--job", "unknown"), 0, b.stderr.String())
	for _, job := range []string{"live", "unknown"} {
		if b.record(job)["status"] != "running" {
			t.Fatalf("%s was concluded under a live or uninspectable supervisor", job)
		}
	}
}

func TestReapConcludesACancelThatWonBeforeLaunch(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.writeRecord("early", map[string]any{"status": "pending", "phase": "cancelling"})
	requireExit(t, b.run("reap", "--job", "early"), 0, b.stderr.String())
	if record := b.record("early"); record["status"] != "cancelled" || record["phase"] != "supervision" {
		t.Fatalf("record %v", record)
	}
}

func TestReapSweepVisitsEveryRecordPastAFailure(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.writeRecord("a-stranger", map[string]any{"status": "running", "pid": 7401, "pgid": 7401, "sessionId": "s",
		"startedAt": b.doubles.Clock.Now().Format(time.RFC3339), "capMin": 60})
	b.doubles.Process.Groups[7401] = true
	b.writeRecord("b-lost", map[string]any{"status": "running", "pid": 7402, "pgid": 7402, "sessionId": "s",
		"startedAt": b.doubles.Clock.Now().Format(time.RFC3339), "capMin": 60})
	result := b.run("reap")
	requireExit(t, result, 1, b.stderr.String())
	if !strings.Contains(b.stderr.String(), "reap sweep finished with failures") {
		t.Fatalf("stderr %q", b.stderr.String())
	}
	if b.record("a-stranger")["status"] != "running" || b.record("b-lost")["status"] != "failed" {
		t.Fatal("the sweep stopped at the first failure")
	}
}

// --- handshake timeout --------------------------------------------------

func TestHandshakeTimeoutStandsDownForALateSession(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.writeRecord("late", map[string]any{"status": "running", "sessionId": "s-late", "pid": 7501, "pgid": 7501})
	requireExit(t, b.run("__handshake-timeout", "--job", "late"), 0, b.stderr.String())
	if b.record("late")["status"] != "running" || len(b.doubles.Process.Signals) != 0 {
		t.Fatal("a landed session was killed")
	}
	log, err := os.ReadFile(filepath.Join(b.root, "artifacts", "agents", "jobs", "late.log"))
	if err != nil || !strings.Contains(string(log), "stood down; session s-late landed before wind-down") {
		t.Fatalf("job log %q, %v", log, err)
	}
}

func TestHandshakeTimeoutRecordsTheVerdictBeforeWindingDown(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.writeRecord("stalled", map[string]any{"status": "pending", "pid": 7502, "pgid": 7502})
	b.doubles.Process.Groups[7502] = true
	b.doubles.Process.Owned[7502] = true
	requireExit(t, b.run("__handshake-timeout", "--job", "stalled"), 0, b.stderr.String())
	record := b.record("stalled")
	if record["status"] != "failed" || record["error"] != "handshake_timeout" || record["phase"] != "handshake" {
		t.Fatalf("record %v", record)
	}
	if !reflect.DeepEqual(b.doubles.Process.Signals, []string{"7502:15"}) {
		t.Fatalf("signals %v", b.doubles.Process.Signals)
	}
	log, _ := os.ReadFile(filepath.Join(b.root, "artifacts", "agents", "jobs", "stalled.log"))
	if !strings.Contains(string(log), "handshake-timeout recorded from pending") {
		t.Fatalf("job log %q", log)
	}
}

// --- close --------------------------------------------------------------

func TestCloseMarksANonCriticRootClosed(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.writeRecord("chain", map[string]any{"status": "completed", "role": "implementer", "round": 1})
	result := b.run("close", "--job", "chain", "--runner-closed")
	if result.ExitCode != 0 {
		t.Logf("close refused (the close check owns the verdict): %s", b.stderr.String())
		return
	}
	record := b.record("chain")
	if record["chainClosed"] != true || record["runnerClosed"] != true {
		t.Fatalf("record %v", record)
	}
}

func TestCloseRequiresTheChainRoot(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.writeRecord("root", map[string]any{"status": "completed"})
	b.writeRecord("root-r2", map[string]any{"status": "completed", "parentJob": "root"})
	result := b.run("close", "--job", "root-r2")
	requireExit(t, result, 1, b.stderr.String())
	if !strings.Contains(b.stderr.String(), "close requires the root job id: root") {
		t.Fatalf("stderr %q", b.stderr.String())
	}
	requireExit(t, b.run("close", "--job", "absent"), 1, b.stderr.String())
	requireExit(t, b.run("close", "--job", "root", "--bogus"), 2, b.stderr.String())
}

// A custody record naming two groups: the recycled, unowned one is refused
// by name, and the wind-down still reaches the later owned group (the
// retired script's sourced wind_down_group leg).
func TestWindDownRefusesAnUnownedGroupAndStillReachesTheOwnedOne(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.writeRecord("two-groups", map[string]any{"status": "running", "pid": 320, "pgid": 320})
	b.doubles.Process.CustodyTargets = func(map[string]any) ([]int64, error) { return []int64{310, 320}, nil }
	b.doubles.Process.Groups[310], b.doubles.Process.Groups[320] = true, true
	b.doubles.Process.Owned[320] = true
	requireExit(t, b.run("__cancel-owned", "--job", "two-groups"), 1, b.stderr.String())
	if !strings.Contains(b.stderr.String(), "refusing to signal unowned process group 310") {
		t.Fatalf("stderr %q", b.stderr.String())
	}
	if !reflect.DeepEqual(b.doubles.Process.Signals, []string{"320:15"}) {
		t.Fatalf("signals %v; the owned group must still be wound down and the stranger never signalled", b.doubles.Process.Signals)
	}
}

// authority-regression WC-9: the standing shell reaper mode stays dead; reap
// takes no interval.
func TestReapHasNoStandingMode(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	for _, argv := range [][]string{{"reap", "--interval", "5"}, {"reap", "--purpose", "standing"}} {
		requireExit(t, b.run(argv...), 2, b.stderr.String())
	}
	if len(b.calls("lease.")) != 0 {
		t.Fatalf("a refused reap reached the lease: %v", b.doubles.Log.Calls())
	}
}
