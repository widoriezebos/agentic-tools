package missionrunner

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestActiveJobs(t *testing.T) {
	root := t.TempDir()
	mission := "demo"
	jobs := jobsDirPath(root)
	writeJSONFile(t, filepath.Join(jobs, "done.json"), map[string]any{"jobId": "done", "mission": mission, "status": "completed"})
	writeJSONFile(t, filepath.Join(jobs, "busy.json"), map[string]any{"jobId": "busy", "mission": mission, "status": "running"})
	writeJSONFile(t, filepath.Join(jobs, "limbo.json"), map[string]any{"jobId": "limbo", "mission": mission})
	writeJSONFile(t, filepath.Join(jobs, "foreign.json"), map[string]any{"jobId": "foreign", "mission": "other", "status": "running"})
	writeJSONFile(t, filepath.Join(jobs, "anon.json"), map[string]any{"mission": mission, "status": "pending"})
	if err := os.WriteFile(filepath.Join(jobs, "corrupt.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := []string{}
	for _, record := range activeJobRecords(root, mission) {
		got = append(got, jobRecordID(record))
	}
	sort.Strings(got)
	// A record without a status is active; one without a jobId reports under
	// its file stem; other missions' jobs and unreadable records are ignored.
	want := []string{"anon", "busy", "limbo"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("active jobs: got %v, want %v", got, want)
	}
}

func TestCloseableChains(t *testing.T) {
	root := t.TempDir()
	mission := "demo"
	jobs := jobsDirPath(root)
	// r1's chain is fully terminal and unclosed: closeable.
	writeJSONFile(t, filepath.Join(jobs, "r1.json"), map[string]any{"jobId": "r1", "mission": mission, "status": "completed"})
	writeJSONFile(t, filepath.Join(jobs, "c1.json"), map[string]any{"jobId": "c1", "mission": mission, "status": "failed", "parentJob": "r1"})
	// r2's chain still has a running member: not closeable.
	writeJSONFile(t, filepath.Join(jobs, "r2.json"), map[string]any{"jobId": "r2", "mission": mission, "status": "completed"})
	writeJSONFile(t, filepath.Join(jobs, "c2.json"), map[string]any{"jobId": "c2", "mission": mission, "status": "running", "parentJob": "r2"})
	// r3 is terminal but already closed.
	writeJSONFile(t, filepath.Join(jobs, "r3.json"), map[string]any{"jobId": "r3", "mission": mission, "status": "completed", "chainClosed": true})
	// x and y point at each other: a cycle belongs to no chain and blocks none.
	writeJSONFile(t, filepath.Join(jobs, "x.json"), map[string]any{"jobId": "x", "mission": mission, "status": "running", "parentJob": "y"})
	writeJSONFile(t, filepath.Join(jobs, "y.json"), map[string]any{"jobId": "y", "mission": mission, "status": "running", "parentJob": "x"})
	// orphan's parent is not among this mission's records: dropped, not blocking.
	writeJSONFile(t, filepath.Join(jobs, "orphan.json"), map[string]any{"jobId": "orphan", "mission": mission, "status": "running", "parentJob": "elsewhere"})

	got := CloseableChains(root, mission)
	if !reflect.DeepEqual(got, []string{"r1"}) {
		t.Fatalf("closeable chains: got %v, want [r1]", got)
	}
}

func TestCloseableChainsRootAlone(t *testing.T) {
	root := t.TempDir()
	mission := "demo"
	writeJSONFile(t, filepath.Join(jobsDirPath(root), "solo.json"), map[string]any{"jobId": "solo", "mission": mission, "status": "cancelled"})
	if got := CloseableChains(root, mission); !reflect.DeepEqual(got, []string{"solo"}) {
		t.Fatalf("a lone terminal root closes its own chain: got %v", got)
	}
}

func TestActiveJobsSeesUnstampedReservationHusks(t *testing.T) {
	// A dispatch that crashes during setup leaves a pending-setup record with
	// no mission stamp while the mission's fence reservation already names
	// the job. The drain must see it, or the concurrency slot leaks forever.
	root := t.TempDir()
	mission := "demo"
	writeJSONFile(t, filepath.Join(jobsDirPath(root), "husk-1.json"),
		map[string]any{"jobId": "husk-1", "status": "pending-setup"})
	writeJSONFile(t, filepath.Join(root, "artifacts", "agents", "missions", mission, "fences.json"),
		map[string]any{"reservations": map[string]any{"husk-1": map[string]any{"capMin": 15}}})
	got := []string{}
	for _, record := range activeJobRecords(root, mission) {
		got = append(got, jobRecordID(record))
	}
	sort.Strings(got)
	want := []string{"husk-1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("active jobs: got %v, want %v", got, want)
	}
}

func TestCloseableChainsSkipsDispatchRefusedHusks(t *testing.T) {
	// A dispatch-refused husk (setup phase, no mirror, held only by the
	// fence reservation) has no evidence to attest: it must not list, or
	// its refusing close strands every chain sorted behind it. A chain
	// that RAN but never mirrored still lists — that failure stays loud.
	root := t.TempDir()
	mission := "demo"
	jobs := jobsDirPath(root)
	writeJSONFile(t, filepath.Join(jobs, "aa-husk.json"),
		map[string]any{"jobId": "aa-husk", "status": "failed", "phase": "setup", "error": "dispatch-refused"})
	writeJSONFile(t, filepath.Join(root, "artifacts", "agents", "missions", mission, "fences.json"),
		map[string]any{"reservations": map[string]any{"aa-husk": map[string]any{"capMin": 15}}})
	writeJSONFile(t, filepath.Join(jobs, "ran.json"),
		map[string]any{"jobId": "ran", "mission": mission, "status": "completed", "phase": "reaped"})

	if got := CloseableChains(root, mission); !reflect.DeepEqual(got, []string{"ran"}) {
		t.Fatalf("closeable chains: got %v, want [ran]", got)
	}
}

func TestCloseTerminalChainsFinishesTheSweepPastAFailure(t *testing.T) {
	// The first chain's close refuses; the second must still be attempted,
	// and the error must carry the failing chain by name.
	root := t.TempDir()
	mission := "demo"
	jobs := jobsDirPath(root)
	writeJSONFile(t, filepath.Join(jobs, "aa.json"),
		map[string]any{"jobId": "aa", "mission": mission, "status": "failed", "phase": "reaped"})
	writeJSONFile(t, filepath.Join(jobs, "bb.json"),
		map[string]any{"jobId": "bb", "mission": mission, "status": "completed", "phase": "reaped"})
	var closes []string
	delegate := func(args ...string) (string, string, int) {
		if args[0] == "reap" {
			return "", "", 0
		}
		job := args[2]
		closes = append(closes, job)
		if !reflect.DeepEqual(args, []string{"close", "--job", job, "--runner-closed"}) {
			t.Errorf("close argv %v", args)
		}
		if job == "aa" {
			return "", "cannot close an unmirrored chain\n", 1
		}
		return "", "", 0
	}

	e := &Engine{Root: root, Mission: mission, Delegate: delegate}
	err := e.closeTerminalChains()
	if err == nil || !strings.Contains(err.Error(), "aa: cannot close an unmirrored chain") {
		t.Fatalf("the failure must name the refusing chain: %v", err)
	}
	if !reflect.DeepEqual(closes, []string{"aa", "bb"}) {
		t.Fatalf("the sweep must continue past the failure, attempts: %v", closes)
	}
}

// missionrunner-5: the drain's reap cadence is decoupled from the
// millisecond heartbeat — reaps run on their own coarser interval, so a
// cap-length drain no longer reaps at heartbeat speed.
func TestDrainReapCadenceIsDecoupled(t *testing.T) {
	root := t.TempDir()
	mission := "demo"
	jobs := jobsDirPath(root)
	clockNow := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	// One active job whose deadline is far away, so the drain loops.
	writeJSONFile(t, filepath.Join(jobs, "busy.json"), map[string]any{
		"jobId": "busy", "mission": mission, "status": "running",
		"startedAt": clockNow.Format(time.RFC3339), "capMin": 30,
	})
	// The runner record the heartbeat reads.
	writeJSONFile(t, filepath.Join(root, "artifacts", "agents", "missions", "runners", mission+".json"),
		map[string]any{"pid": 1, "pidStartedAt": 1, "instanceTag": "t", "status": "running"})
	// A delegate that counts reap invocations, then completes the job on
	// the second reap so the drain ends.
	reaps := 0
	delegate := func(args ...string) (string, string, int) {
		if args[0] != "reap" {
			t.Errorf("drain ran %v", args)
		}
		reaps++
		if reaps >= 2 {
			writeJSONFile(t, filepath.Join(jobs, "busy.json"), map[string]any{"jobId": "busy", "mission": "demo", "status": "completed"})
		}
		return "", "", 0
	}

	t.Setenv("METASYSTEM_HEARTBEAT_INTERVAL_MS", "10")
	t.Setenv("METASYSTEM_DRAIN_REAP_INTERVAL_MS", "300")
	original := runClock
	sleeps := 0
	runClock.now = func() time.Time { return clockNow }
	runClock.sleep = func(wait time.Duration) {
		if wait != 10*time.Millisecond {
			t.Fatalf("drain sleep = %s, want heartbeat poll 10ms", wait)
		}
		clockNow = clockNow.Add(wait)
		sleeps++
	}
	t.Cleanup(func() { runClock = original })
	e := &Engine{Root: root, Mission: mission, Delegate: delegate}
	state, err := e.drainJobs(filepath.Join(root, "state.json"), filepath.Join(root, "ledger.md"), "t1", 1)
	if err != nil || state != nil {
		t.Fatalf("drain: state=%v err=%v", state, err)
	}
	// The artificial clock must cross the 300ms reap cadence through 10ms
	// heartbeat sleeps before the second reap can finish the job.
	if sleeps != 30 {
		t.Fatalf("heartbeat sleeps before the second reap = %d, want 30", sleeps)
	}
	if reaps != 2 {
		t.Fatalf("reap count = %d, want exactly two cadence passes", reaps)
	}
}
