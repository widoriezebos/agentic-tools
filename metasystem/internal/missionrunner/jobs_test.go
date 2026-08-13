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
	stub := `#!/bin/sh
verb=$1; shift
job=""
while [ $# -gt 0 ]; do
  if [ "$1" = "--job" ]; then job=$2; shift; fi
  shift
done
[ "$verb" = "reap" ] && exit 0
echo "$job" >> "$(dirname "$0")/closes.log"
if [ "$job" = "aa" ]; then echo "cannot close an unmirrored chain" >&2; exit 1; fi
exit 0
`
	scripts := filepath.Join(root, "scripts", "agents")
	if err := os.MkdirAll(scripts, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scripts, "dispatch.sh"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}

	e := &Engine{Root: root, Mission: mission}
	err := e.closeTerminalChains()
	if err == nil || !strings.Contains(err.Error(), "aa: cannot close an unmirrored chain") {
		t.Fatalf("the failure must name the refusing chain: %v", err)
	}
	logBytes, readErr := os.ReadFile(filepath.Join(scripts, "closes.log"))
	if readErr != nil {
		t.Fatalf("no close attempts recorded: %v", readErr)
	}
	if got := string(logBytes); got != "aa\nbb\n" {
		t.Fatalf("the sweep must continue past the failure, attempts: %q", got)
	}
}

// missionrunner-5: the drain's reap cadence is decoupled from the
// millisecond heartbeat — reaps run on their own coarser interval, so a
// cap-length drain no longer spawns dispatch.sh at heartbeat speed.
func TestDrainReapCadenceIsDecoupled(t *testing.T) {
	root := t.TempDir()
	mission := "demo"
	jobs := jobsDirPath(root)
	// One active job whose deadline is far away, so the drain loops.
	writeJSONFile(t, filepath.Join(jobs, "busy.json"), map[string]any{
		"jobId": "busy", "mission": mission, "status": "running",
		"startedAt": time.Now().UTC().Format(time.RFC3339), "capMin": 30,
	})
	// The runner record the heartbeat reads.
	writeJSONFile(t, filepath.Join(root, "artifacts", "agents", "missions", "runners", mission+".json"),
		map[string]any{"pid": 1, "pidStartedAt": 1, "instanceTag": "t", "status": "running"})
	// A dispatch stub that counts reap invocations, then completes the job
	// on the second reap so the drain ends.
	scripts := filepath.Join(root, "scripts", "agents")
	if err := os.MkdirAll(scripts, 0o755); err != nil {
		t.Fatal(err)
	}
	stub := `#!/bin/sh
count_file="$(dirname "$0")/reaps"
echo x >> "$count_file"
if [ "$(wc -l < "$count_file")" -ge 2 ]; then
  record="$(dirname "$0")/../../artifacts/agents/jobs/busy.json"
  python3 - "$record" <<'PY'
import json, sys
v = json.load(open(sys.argv[1])); v["status"] = "completed"
open(sys.argv[1], "w").write(json.dumps(v))
PY
fi
exit 0
`
	if err := os.WriteFile(filepath.Join(scripts, "dispatch.sh"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("METASYSTEM_HEARTBEAT_INTERVAL_MS", "10")
	t.Setenv("METASYSTEM_DRAIN_REAP_INTERVAL_MS", "300")
	e := &Engine{Root: root, Mission: mission}
	started := time.Now()
	state, err := e.drainJobs(filepath.Join(root, "state.json"), filepath.Join(root, "ledger.md"), "t1", 1)
	if err != nil || state != nil {
		t.Fatalf("drain: state=%v err=%v", state, err)
	}
	// Two reap passes 300ms apart means the drain ran at least ~300ms while
	// heartbeating every 10ms; at heartbeat-coupled cadence the stub would
	// have been called dozens of times before its second line landed.
	if elapsed := time.Since(started); elapsed < 250*time.Millisecond {
		t.Fatalf("drain finished before a second decoupled reap could run: %v", elapsed)
	}
	data, _ := os.ReadFile(filepath.Join(scripts, "reaps"))
	if reaps := strings.Count(string(data), "x"); reaps > 4 {
		t.Fatalf("reap ran at heartbeat speed: %d invocations", reaps)
	}
}
