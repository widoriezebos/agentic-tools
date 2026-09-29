package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// laneVerbBed is a computer with a lane home, two landing checkouts, an owner
// that is alive or not, and the lane's batch records; no process and no Git.
type laneVerbBed struct {
	cwd, home, landingA, landingB string
	alive                         bool
	starts, ends                  int
	pid                           int64
	person                        error
	records                       []batch.Record
}

func newLaneVerbBed(t *testing.T) *laneVerbBed {
	t.Helper()
	base := t.TempDir()
	bed := &laneVerbBed{pid: 4242, cwd: filepath.Join(base, "cwd"), home: filepath.Join(base, "home"), landingA: filepath.Join(base, "landing-a"), landingB: filepath.Join(base, "landing-b")}
	for _, dir := range []string{bed.cwd, bed.home, bed.landingA, bed.landingB} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	bed.home, bed.landingA, bed.landingB = realpath.Resolve(bed.home), realpath.Resolve(bed.landingA), realpath.Resolve(bed.landingB)
	return bed
}

func (bed *laneVerbBed) owners() intentOwners {
	notARepository := func(string) (string, error) { return "", errors.New("not a repository") }
	return intentOwners{resolver: stateroot.NewResolver(notARepository, os.Executable), landing: laneVerbOwners{
		home: func() (string, error) { return bed.home, nil },
		probe: func(string) (lane.OwnerProbe, error) {
			if !bed.alive {
				return lane.OwnerProbe{}, nil
			}
			return lane.OwnerProbe{Alive: true, PID: bed.pid, Since: laneTestNow.Add(-time.Hour)}, nil
		},
		start: func(string) error {
			if !bed.alive {
				bed.starts++
				bed.alive, bed.pid = true, bed.pid+1
			}
			return nil
		},
		end: func(string) (int64, error) {
			if !bed.alive {
				return 0, nil
			}
			bed.ends++
			bed.alive = false
			return bed.pid, nil
		},
		person: func(string) (string, error) {
			if bed.person != nil {
				return "", bed.person
			}
			return "Wido", nil
		},
		records:  func(string) ([]batch.Record, error) { return bed.records, nil },
		validate: func(root, _ string, _ time.Time) (string, error) { return realpath.Resolve(root), nil },
		by:       func(string) string { return "seat" },
		now:      func() time.Time { return laneTestNow },
	}}
}

func (bed *laneVerbBed) run(t *testing.T, words ...string) (int, string, string) {
	t.Helper()
	command, ok := findIntentAction(words[0], words[1])
	if !ok {
		t.Fatalf("no public command %s %s", words[0], words[1])
	}
	var stdout, stderr bytes.Buffer
	code := runIntentIn(command, words[2:], &stdout, &stderr, bed.cwd, bed.owners())
	return code, stdout.String(), stderr.String()
}

func (bed *laneVerbBed) status(t *testing.T) lane.View {
	t.Helper()
	code, stdout, stderr := bed.run(t, "landing", "status", "--json")
	var result struct{ Data lane.View }
	if code != 0 || json.Unmarshal([]byte(stdout), &result) != nil {
		t.Fatalf("landing status --json = %d %q %q", code, stdout, stderr)
	}
	return result.Data
}

// The landing verbs register the lane, start, stop and restart its owner,
// and a repeat whose effect holds is success that changes nothing.
func TestLandingVerbsSetStartStopRestart(t *testing.T) {
	t.Parallel()
	bed := newLaneVerbBed(t)
	if code, stdout, _ := bed.run(t, "landing", "status"); code != 0 || !strings.Contains(stdout, "no landing lane is registered") {
		t.Fatalf("status without a lane = %d %q", code, stdout)
	}
	if code, _, stderr := bed.run(t, "landing", "start"); code == 0 || !strings.Contains(stderr, "metasystem landing set PATH") {
		t.Fatalf("start without a lane = %d %q", code, stderr)
	}
	if code, stdout, stderr := bed.run(t, "landing", "set", bed.landingA, "--by", "Wido"); code != 0 || !strings.Contains(stdout, "is now "+bed.landingA) {
		t.Fatalf("set = %d %q %q", code, stdout, stderr)
	}
	if code, stdout, _ := bed.run(t, "landing", "set", bed.landingA); code != 0 || !strings.Contains(stdout, "already "+bed.landingA) {
		t.Fatalf("set again = %d %q", code, stdout)
	}
	if view := bed.status(t); view.Root == nil || *view.Root != bed.landingA || *view.RegisteredBy != "Wido" || view.Owner.State != lane.OwnerNotStarted {
		t.Fatalf("status after set = %+v", view)
	}
	if code, stdout, _ := bed.run(t, "landing", "start"); code != 0 || bed.starts != 1 || !strings.Contains(stdout, "started") {
		t.Fatalf("start = %d %q starts=%d", code, stdout, bed.starts)
	}
	if code, stdout, _ := bed.run(t, "landing", "start"); code != 0 || bed.starts != 1 || !strings.Contains(stdout, "already running (pid 4243)") {
		t.Fatalf("start again = %d %q starts=%d", code, stdout, bed.starts)
	}
	if code, stdout, _ := bed.run(t, "landing", "stop", "--by", "Wido"); code != 0 || !strings.Contains(stdout, "stopped the landing lane") {
		t.Fatalf("stop = %d %q", code, stdout)
	}
	view := bed.status(t)
	if view.Owner.State != lane.OwnerStopped || view.Owner.StoppedBy == nil || *view.Owner.StoppedBy != "Wido" || !strings.Contains(view.Summary, "metasystem landing start") {
		t.Fatalf("status after stop = %+v %q", view.Owner, view.Summary)
	}
	if code, stdout, _ := bed.run(t, "landing", "stop"); code != 0 || !strings.Contains(stdout, "already stopped by Wido") {
		t.Fatalf("stop again = %d %q", code, stdout)
	}
	if code, stdout, _ := bed.run(t, "landing", "start"); code != 0 {
		t.Fatalf("start after stop = %d %q", code, stdout)
	}
	if code, stdout, _ := bed.run(t, "landing", "status", "--verbose"); code != 0 || !strings.Contains(stdout, "registered by Wido") || !strings.Contains(stdout, "pid 4243") {
		t.Fatalf("status --verbose = %d %q", code, stdout)
	}
}

func provingRecord(id, state string) batch.Record {
	return batch.Record{BatchID: id, State: state, Units: []batch.Unit{{GoalID: "g1", State: batch.UnitJoined, Claim: batch.Claim{Machine: "m1e"}}},
		History: []batch.HistoryEntry{{At: laneTestNow.Format(time.RFC3339Nano), Verb: "seal", From: batch.StateOpen, To: state}}}
}

// Moving the lane while a batch proves in it is refused with the way
// forward; after a person pauses the lane, the move goes through.
func TestLandingSetRefusesAMoveWhileABatchProves(t *testing.T) {
	t.Parallel()
	bed := newLaneVerbBed(t)
	bed.alive = true
	if code, _, stderr := bed.run(t, "landing", "set", bed.landingA); code != 0 {
		t.Fatalf("set = %d %q", code, stderr)
	}
	bed.records = []batch.Record{provingRecord("b1", batch.StateProving)}
	code, _, stderr := bed.run(t, "landing", "set", bed.landingB)
	for _, want := range []string{codeLandingLaneBusy, "b1 proving", bed.landingA, "metasystem landing stop", "metasystem landing set " + bed.landingB} {
		if code == 0 || !strings.Contains(stderr, want) {
			t.Errorf("move while proving = %d %q; lacks %q", code, stderr, want)
		}
	}
	if record, _, _ := lane.Read(bed.home); record.Root != bed.landingA {
		t.Fatalf("the refused move moved the lane to %s", record.Root)
	}
	if code, _, stderr := bed.run(t, "landing", "stop"); code != 0 {
		t.Fatalf("stop = %d %q", code, stderr)
	}
	if code, stdout, stderr := bed.run(t, "landing", "set", bed.landingB); code != 0 || !strings.Contains(stdout, "it was "+bed.landingA) {
		t.Fatalf("move after the pause = %d %q %q", code, stdout, stderr)
	}
}

// A batch pushing to main is never paused mid-push.
func TestLandingStopRefusesWhileABatchPushes(t *testing.T) {
	t.Parallel()
	bed := newLaneVerbBed(t)
	bed.alive = true
	if code, _, stderr := bed.run(t, "landing", "set", bed.landingA); code != 0 {
		t.Fatalf("set = %d %q", code, stderr)
	}
	bed.records = []batch.Record{provingRecord("b1", batch.StateLanding)}
	for _, verb := range []string{"stop", "restart"} {
		code, _, stderr := bed.run(t, "landing", verb)
		if code == 0 || !strings.Contains(stderr, codeLandingLanePushing) || !strings.Contains(stderr, "metasystem landing stop again") {
			t.Fatalf("%s while pushing = %d %q", verb, code, stderr)
		}
	}
	if _, paused := lane.ReadPause(bed.home); paused {
		t.Fatalf("a pushing lane was paused")
	}
}

// status shows the lane's headline in its board block.
func TestStatusShowsTheLandingLaneLine(t *testing.T) {
	t.Parallel()
	bed := newLaneVerbBed(t)
	inv := &intentInvocation{owners: bed.owners()}
	if line := inv.statusLaneLine(); line != "" {
		t.Fatalf("no lane: line %q", line)
	}
	if _, _, err := lane.Register(bed.home, bed.landingA, "Wido", laneTestNow); err != nil {
		t.Fatal(err)
	}
	bed.records = []batch.Record{provingRecord("b1", batch.StateProving)}
	if line := inv.statusLaneLine(); !strings.Contains(line, "landing lane "+bed.landingA) || !strings.Contains(line, "batch b1 proving") {
		t.Fatalf("lane line = %q", line)
	}
}

// landing restart gives a fresh owner process: the running owner is ended
// by its recorded identity, a new one runs with another pid, the pause is
// cleared and the keep-alive's restarts are forgotten.
func TestLandingRestartGivesAFreshOwner(t *testing.T) {
	t.Parallel()
	bed := newLaneVerbBed(t)
	bed.alive = true
	if code, _, stderr := bed.run(t, "landing", "set", bed.landingA); code != 0 {
		t.Fatalf("set = %d %q", code, stderr)
	}
	if _, err := lane.SetPause(bed.home, "Wido", laneTestNow); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(lane.HostDir(bed.home), "landing-lane-keeper.json"), []byte(`{"failures":2,"restarts":2}`), 0o600); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := bed.run(t, "landing", "restart")
	if code != 0 || bed.ends != 1 || !strings.Contains(stdout, "4242") || !strings.Contains(stdout, "4243") {
		t.Fatalf("restart = %d %q %q, ends %d; want pid 4242 ended and 4243 running", code, stdout, stderr, bed.ends)
	}
	view := bed.status(t)
	if view.Owner.State != lane.OwnerRunning || view.Owner.PID == nil || *view.Owner.PID != 4243 || view.Owner.Restarts != 0 {
		t.Fatalf("after restart = %+v", view.Owner)
	}
	if _, paused := lane.ReadPause(bed.home); paused {
		t.Fatalf("restart left the lane paused")
	}
}

// Moving a registered lane elsewhere is a person's act at an enrolled
// terminal; the first registration and a repeat of the same root are not.
func TestLandingSetMoveIsAPersonsAct(t *testing.T) {
	t.Parallel()
	bed := newLaneVerbBed(t)
	bed.person = errors.New("human authority has no readable terminal enrollment")
	if code, _, stderr := bed.run(t, "landing", "set", bed.landingA); code != 0 {
		t.Fatalf("first registration by anyone = %d %q", code, stderr)
	}
	if code, _, stderr := bed.run(t, "landing", "set", bed.landingA); code != 0 {
		t.Fatalf("repeat by anyone = %d %q", code, stderr)
	}
	code, _, stderr := bed.run(t, "landing", "set", bed.landingB)
	if code == 0 || !strings.Contains(stderr, "a person's act") || !strings.Contains(stderr, "metasystem system enroll") {
		t.Fatalf("move by no person = %d %q", code, stderr)
	}
	if record, _, _ := lane.Read(bed.home); record.Root != bed.landingA {
		t.Fatalf("the refused move moved the lane")
	}
	bed.person = nil
	if code, stdout, stderr := bed.run(t, "landing", "set", bed.landingB); code != 0 || !strings.Contains(stdout, "is now "+bed.landingB) {
		t.Fatalf("move by the person = %d %q %q", code, stdout, stderr)
	}
	if record, _, _ := lane.Read(bed.home); record.RegisteredBy != "Wido" {
		t.Fatalf("registered by %q; want the proven person", record.RegisteredBy)
	}
}

// F-5: a corrupt lane record is replaced by landing set.
func TestLandingSetReplacesACorruptRecord(t *testing.T) {
	t.Parallel()
	bed := newLaneVerbBed(t)
	if err := os.MkdirAll(lane.HostDir(bed.home), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lane.RecordPath(bed.home), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if code, stdout, stderr := bed.run(t, "landing", "set", bed.landingA); code != 0 || !strings.Contains(stdout, "is now "+bed.landingA) {
		t.Fatalf("set over a corrupt record = %d %q %q", code, stdout, stderr)
	}
	if record, ok, err := lane.Read(bed.home); err != nil || !ok || record.Root != bed.landingA {
		t.Fatalf("record after set = %+v %v %v", record, ok, err)
	}
}
