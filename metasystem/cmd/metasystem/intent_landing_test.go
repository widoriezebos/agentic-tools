package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/cadence"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gaterun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// laneVerbBed is a computer with a lane home, two nested landing checkouts,
// an owner that is alive or not, and the lane's batch records; no process.
type laneVerbBed struct {
	cwd, home, landingA, landingB string
	alive                         bool
	starts, ends                  int
	pid                           int64
	person                        error
	records                       []batch.Record
	// ready is whether an owner could run in the lane (nil: it could);
	// noMachine is a checkout without a machine nickname; staysDown is an
	// owner a start asks for that does not come up.
	ready                error
	noMachine, staysDown bool
	// unset replaces landing unset's steps; nil runs the real ones.
	unset func(home, by string, force bool) (lane.UnsetReport, error)
	// held are the goals the ledger shows the lane holding.
	held []string
	// agent is a landing agent that runs on the lane; empty is none.
	agent string
	// oldClaims are the goals the ledger shows the old owner lineage
	// (landing-m1l) holding; oldClaimsErr an unreadable ledger.
	oldClaims    []string
	oldClaimsErr error
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
	landingCheckout(t, bed.landingA)
	landingCheckout(t, bed.landingB)
	return bed
}

func (bed *laneVerbBed) owners() intentOwners {
	notARepository := func(string) (string, error) { return "", errors.New("not a repository") }
	return intentOwners{resolver: stateroot.NewResolver(notARepository, os.Executable), landing: laneVerbOwners{
		// The lane beds keep no goal ledger: validation is never due.
		validation: func(string, time.Time) (bool, error) { return false, nil },
		agent: func() (string, bool, error) {
			if bed.agent != "" {
				return bed.agent, true, nil
			}
			return "", false, nil
		},
		home: func() (string, error) { return bed.home, nil },
		probe: func(string) (lane.OwnerProbe, error) {
			if !bed.alive {
				return lane.OwnerProbe{}, nil
			}
			return lane.OwnerProbe{Alive: true, PID: bed.pid, Since: laneTestNow.Add(-time.Hour)}, nil
		},
		start: func(string) error {
			if !bed.alive && bed.staysDown {
				bed.starts++
				return nil
			}
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
		ready:    func(string) error { return bed.ready },
		machine: func(string) (string, error) {
			if bed.noMachine {
				return "", errors.New("no machine nickname is enrolled on this machine")
			}
			return "landing", nil
		},
		now:   func() time.Time { return laneTestNow },
		unset: bed.unset,
		laneHeld: func(string) ([]string, error) {
			return bed.held, nil
		},
		oldOwnerClaims: func(string) ([]string, error) {
			return bed.oldClaims, bed.oldClaimsErr
		},
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
	if code, stdout, _ := bed.run(t, "landing", "status"); code != 0 || !strings.Contains(stdout, "No landing lane is registered") {
		t.Fatalf("status without a lane = %d %q", code, stdout)
	}
	if code, _, stderr := bed.run(t, "landing", "start"); code == 0 || !strings.Contains(stderr, "metasystem landing set PATH") {
		t.Fatalf("start without a lane = %d %q", code, stderr)
	}
	if code, stdout, stderr := bed.run(t, "landing", "set", bed.landingA, "--by", "Wido"); code != 0 || !strings.Contains(oneSpaced(stdout), "is now "+bed.landingA) {
		t.Fatalf("set = %d %q %q", code, stdout, stderr)
	}
	if code, stdout, _ := bed.run(t, "landing", "set", bed.landingA); code != 0 || !strings.Contains(oneSpaced(stdout), "already "+bed.landingA) {
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
	if code, stdout, _ := bed.run(t, "landing", "status", "--verbose"); code != 0 || !strings.Contains(stdout, "registered   by Wido") || !strings.Contains(stdout, "pid 4243") {
		t.Fatalf("status --verbose = %d %q", code, stdout)
	}
}

// landing stop --reason keeps the reason with the pause (design r10 §3,
// the agent's stop and ask): the stop, status, a repeat and every gated
// operation the pause holds name it on line 1; line 2 stays the command.
func TestLandingStopRecordsItsReason(t *testing.T) {
	t.Parallel()
	bed := newLaneVerbBed(t)
	if code, _, stderr := bed.run(t, "landing", "set", bed.landingA, "--by", "Wido"); code != 0 {
		t.Fatalf("set = %d %q", code, stderr)
	}
	reason := "goal-a is red twice on its own tests"
	code, stdout, stderr := bed.run(t, "landing", "stop", "--by", "Wido", "--reason", reason)
	if code != 0 || !strings.Contains(oneSpaced(stdout), "stopped the landing lane for Wido ("+reason+")") {
		t.Fatalf("stop --reason = %d %q %q", code, stdout, stderr)
	}
	if pause, paused := lane.ReadPause(bed.home); !paused || pause.By != "Wido" || pause.Reason != reason {
		t.Fatalf("the recorded pause = %+v %v; want its reason", pause, paused)
	}
	view := bed.status(t)
	if view.Owner.StoppedBecause == nil || *view.Owner.StoppedBecause != reason || !strings.Contains(view.Summary, reason) {
		t.Fatalf("status after stop --reason = %+v %q", view.Owner, view.Summary)
	}
	if code, stdout, _ := bed.run(t, "landing", "status"); code != 0 || !strings.Contains(oneSpaced(stdout), "stopped by Wido ("+reason+")") {
		t.Fatalf("status = %d %q", code, stdout)
	}
	if code, stdout, _ := bed.run(t, "landing", "stop", "--reason", "another"); code != 0 || !strings.Contains(oneSpaced(stdout), "already stopped by Wido ("+reason+")") {
		t.Fatalf("stop again = %d %q", code, stdout)
	}
	var refusal *lane.Refusal
	err := lane.Gate(bed.home, lane.OpBegin, lane.AuthorityAgent, func(lane.Record) error { return nil })
	if !errors.As(err, &refusal) || refusal.Code != lane.CodePaused || !strings.Contains(refusal.Message, "stopped by Wido ("+reason+")") {
		t.Fatalf("a gated operation while stopped with a reason = %v", err)
	}
	if code, _, _ := bed.run(t, "landing", "start"); code != 0 {
		t.Fatalf("start = %d", code)
	}
	if code, stdout, _ := bed.run(t, "landing", "stop", "--by", "Wido"); code != 0 || strings.Contains(stdout, "(") {
		t.Fatalf("stop with no reason = %d %q", code, stdout)
	}
	if pause, _ := lane.ReadPause(bed.home); pause.Reason != "" {
		t.Fatalf("a stop with no reason kept %q", pause.Reason)
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
	code, _, stderr := bed.run(t, "landing", "set", bed.landingB, "--verbose")
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
	if code, stdout, stderr := bed.run(t, "landing", "set", bed.landingB); code != 0 || !strings.Contains(oneSpaced(stdout), "it was "+bed.landingA) {
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
		code, _, stderr := bed.run(t, "landing", verb, "--verbose")
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
	registerLane(t, bed.home, bed.landingA, "Wido", laneTestNow)
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

// Registering the lane, and moving it, is a person's act at an enrolled
// terminal (design r10 §1); a repeat of the same registration changes
// nothing and asks no one.
func TestLandingSetIsAPersonsAct(t *testing.T) {
	t.Parallel()
	bed := newLaneVerbBed(t)
	bed.person = humanauthority.Refusedf(humanauthority.OutcomeNotEnrolled, "human authority has no readable terminal enrollment")
	code, _, stderr := bed.run(t, "landing", "set", bed.landingA)
	if code == 0 || !strings.Contains(stderr, "only a person may register the landing lane") || !strings.Contains(stderr, "metasystem system enroll") {
		t.Fatalf("first registration by no person = %d %q", code, stderr)
	}
	if _, ok, _ := lane.Read(bed.home); ok {
		t.Fatalf("a refused registration registered the lane")
	}
	bed.person = nil
	if code, _, stderr := bed.run(t, "landing", "set", bed.landingA); code != 0 {
		t.Fatalf("registration by the person = %d %q", code, stderr)
	}
	bed.person = humanauthority.Refusedf(humanauthority.OutcomeNotEnrolled, "human authority has no readable terminal enrollment")
	if code, stdout, stderr := bed.run(t, "landing", "set", bed.landingA); code != 0 || !strings.Contains(oneSpaced(stdout), "already "+bed.landingA) {
		t.Fatalf("repeat by anyone = %d %q %q", code, stdout, stderr)
	}
	code, _, stderr = bed.run(t, "landing", "set", bed.landingB)
	if code != 3 || !strings.Contains(stderr, "only a person may register the landing lane") {
		t.Fatalf("move by no person = %d %q", code, stderr)
	}
	if record, _, _ := lane.Read(bed.home); record.Root != bed.landingA {
		t.Fatalf("the refused move moved the lane")
	}
	bed.person = nil
	if code, stdout, stderr := bed.run(t, "landing", "set", bed.landingB); code != 0 || !strings.Contains(oneSpaced(stdout), "is now "+bed.landingB) {
		t.Fatalf("move by the person = %d %q %q", code, stdout, stderr)
	}
	if record, _, _ := lane.Read(bed.home); record.RegisteredBy != "Wido" || record.CustodyEpoch != 2 || record.Install != filepath.Join(bed.landingB, "metasystem") {
		t.Fatalf("record = %+v; want the proven person, epoch 2 and the nested installation", record)
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
	if code, stdout, stderr := bed.run(t, "landing", "set", bed.landingA); code != 0 || !strings.Contains(oneSpaced(stdout), "is now "+bed.landingA) {
		t.Fatalf("set over a corrupt record = %d %q %q", code, stdout, stderr)
	}
	if record, ok, err := lane.Read(bed.home); err != nil || !ok || record.Root != bed.landingA {
		t.Fatalf("record after set = %+v %v %v", record, ok, err)
	}
}

// A lane whose checkout's supervision is not armed cannot start its owner:
// landing start and restart say so, name the command a person runs and
// change nothing; they never claim a start.
func TestLandingStartRefusesALaneWhoseSupervisionIsNotArmed(t *testing.T) {
	t.Parallel()
	bed := newLaneVerbBed(t)
	if code, _, stderr := bed.run(t, "landing", "set", bed.landingA); code != 0 {
		t.Fatalf("set = %d %q", code, stderr)
	}
	if _, err := lane.SetPause(bed.home, "Wido", laneTestNow); err != nil {
		t.Fatal(err)
	}
	bed.ready = lane.UnarmedRefusal(bed.landingA)
	for _, verb := range []string{"start", "restart"} {
		code, stdout, stderr := bed.run(t, "landing", verb, "--verbose")
		for _, want := range []string{lane.CodeUnarmed, "supervision is not armed", "  → metasystem system start --repo " + bed.landingA, "nothing was started"} {
			if !strings.Contains(stderr, want) {
				t.Errorf("%s = %d %q; lacks %q", verb, code, stderr, want)
			}
		}
		if code != 3 || strings.Contains(stdout+stderr, "started the") || bed.starts != 0 || bed.ends != 0 {
			t.Fatalf("%s = %d %q %q, starts %d ends %d; want a person's act named, nothing started", verb, code, stdout, stderr, bed.starts, bed.ends)
		}
	}
	if _, paused := lane.ReadPause(bed.home); !paused {
		t.Fatalf("the refused start changed the lane's pause")
	}
	var result struct{ Outcome string }
	_, stdout, _ := bed.run(t, "landing", "start", "--json")
	if json.Unmarshal([]byte(stdout), &result) != nil || result.Outcome != intentRefused {
		t.Fatalf("start --json = %q; want outcome refused", stdout)
	}
}

// A start whose owner does not come up is not a start: the verb says it
// asked the checkout's supervision and that the owner does not run yet.
func TestLandingStartNeverClaimsAnOwnerThatIsNotUp(t *testing.T) {
	t.Parallel()
	bed := newLaneVerbBed(t)
	bed.staysDown = true
	if code, _, stderr := bed.run(t, "landing", "set", bed.landingA); code != 0 {
		t.Fatalf("set = %d %q", code, stderr)
	}
	code, stdout, _ := bed.run(t, "landing", "start", "--json")
	var result struct{ Outcome, Summary string }
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("start --json = %d %q", code, stdout)
	}
	if bed.starts != 1 || result.Outcome != intentInProgress || strings.Contains(result.Summary, "started the") || !strings.Contains(result.Summary, "does not run yet") {
		t.Fatalf("start = %d %+v starts %d; want in-progress without a claimed start", code, result, bed.starts)
	}
}

// landing set refuses a checkout without a machine nickname, first
// registration included, with the exact command that names it.
func TestLandingSetRefusesACheckoutWithoutAMachineNickname(t *testing.T) {
	t.Parallel()
	bed := newLaneVerbBed(t)
	bed.noMachine = true
	code, _, stderr := bed.run(t, "landing", "set", bed.landingA, "--verbose")
	for _, want := range []string{lane.CodeNoMachine, "no machine nickname", "  → git -C " + bed.landingA + " config metasystem.goal.machine landing", "nothing was registered"} {
		if code == 0 || !strings.Contains(stderr, want) {
			t.Errorf("set without a nickname = %d %q; lacks %q", code, stderr, want)
		}
	}
	if _, ok, _ := lane.Read(bed.home); ok {
		t.Fatalf("a checkout without a nickname was registered")
	}
	bed.noMachine = false
	if code, stdout, stderr := bed.run(t, "landing", "set", bed.landingA); code != 0 || !strings.Contains(oneSpaced(stdout), "is now "+bed.landingA) {
		t.Fatalf("set once named = %d %q %q", code, stdout, stderr)
	}
}

// landing status says why the owner does not run and the one command that
// fixes it, in its one line and as its next step; set names that step too.
func TestLandingStatusSaysWhyTheOwnerCannotRun(t *testing.T) {
	t.Parallel()
	bed := newLaneVerbBed(t)
	bed.ready = lane.UnarmedRefusal(bed.landingA)
	code, stdout, stderr := bed.run(t, "landing", "set", bed.landingA)
	if code != 0 || !strings.Contains(stdout, "→ metasystem system start --repo "+bed.landingA) {
		t.Fatalf("set on an unarmed checkout = %d %q %q; want the arming named next", code, stdout, stderr)
	}
	code, stdout, _ = bed.run(t, "landing", "status")
	for _, want := range []string{"! The landing lane's owner is not running", "supervision is not armed", "→ metasystem system start --repo " + bed.landingA} {
		if code != 0 || !strings.Contains(oneSpaced(stdout), want) {
			t.Errorf("status = %d %q; lacks %q", code, stdout, want)
		}
	}
}

// landing status says a failing owner tick the way a person reads it: line 1
// the plain situation (which batch cannot advance and why, in plain words),
// line 2 the one command that shows more; the owner's raw error is only in
// --verbose and --json. Known failure classes get their own words; an
// unknown one gets a generic line.
func TestLandingStatusSaysAFailingTickInPlainWords(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, raw, want string
	}{
		{"re-arm asks for a session",
			`batch 4gr18nm8t3nyev9sssda9jgtsq: bin/metasystem up --repo /lanes/landing/metasystem: exit status 1: up outcome=failed component=session-identity remedy="pass --pid <session-pid> and --start-time <epoch-seconds>, or configure a runtime signature and invoke up from that session"`,
			"batch 4gr18nm8t3nyev9sssda9jgtsq can't advance: the lane owner could not re-arm the lane's engine for its proof"},
		{"engine rebuild",
			`batch b1: go run -trimpath ./cmd/devgate build: exit status 1: compile error`,
			"batch b1 can't advance: the lane owner could not rebuild the lane's engine for its proof"},
		{"fetch",
			`batch b1: BATCH_LAND_PUSH_REFUSED: fetch origin/main: could not resolve host: exit status 128`,
			"batch b1 can't advance: the lane owner could not fetch main"},
		{"unknown",
			`batch b1: something new broke: detail`,
			"batch b1 can't advance: the lane owner's last tick failed"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			bed := newLaneVerbBed(t)
			bed.alive = true
			if code, _, stderr := bed.run(t, "landing", "set", bed.landingA); code != 0 {
				t.Fatalf("set = %d %q", code, stderr)
			}
			path := lane.TickErrorPath(bed.landingA)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(test.raw+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			code, stdout, _ := bed.run(t, "landing", "status")
			if code != 0 || !strings.HasPrefix(oneSpaced(stdout), "! "+test.want+" → metasystem landing status --verbose ") {
				t.Fatalf("status = %d %q; want the situation, then the one command", code, stdout)
			}
			raw := strings.SplitN(test.raw, ": ", 2)[1]
			if strings.Contains(stdout, raw) {
				t.Errorf("status without --verbose carries the raw error: %q", stdout)
			}
			_, verbose, _ := bed.run(t, "landing", "status", "--verbose")
			if !strings.Contains(oneSpaced(verbose), "last tick "+test.raw) || !strings.Contains(verbose, "owner log ") {
				t.Errorf("verbose status lacks the raw error and the log: %q", verbose)
			}
			_, encoded, _ := bed.run(t, "landing", "status", "--json")
			var result struct{ Data lane.View }
			if err := json.Unmarshal([]byte(encoded), &result); err != nil || result.Data.Owner.LastTickError == nil || *result.Data.Owner.LastTickError != test.raw {
				t.Errorf("json status lacks the raw error: %v %q", err, encoded)
			}
		})
	}
}

// oneSpaced is a page with every run of spaces and line breaks one space:
// what it says, however its lines wrapped.
func oneSpaced(page string) string { return strings.Join(strings.Fields(page), " ") }

// landing unset is a person's act (design r10 §1): refused to anyone else
// with nothing changed; for the person it fences, settles, returns and
// unregisters the lane, after which there is no lane and each seat lands
// its own work; a repeat changes nothing, and a later landing set takes a
// new custody epoch.
func TestLandingUnsetIsAPersonsActAndUnregisters(t *testing.T) {
	t.Parallel()
	bed := newLaneVerbBed(t)
	if code, _, stderr := bed.run(t, "landing", "set", bed.landingA); code != 0 {
		t.Fatalf("set = %d %q", code, stderr)
	}
	bed.person = humanauthority.Refusedf(humanauthority.OutcomeNotEnrolled, "human authority has no readable terminal enrollment")
	code, _, stderr := bed.run(t, "landing", "unset")
	if code != 3 || !strings.Contains(stderr, "only a person may unset the landing lane") {
		t.Fatalf("unset by no person = %d %q", code, stderr)
	}
	if _, fenced, _ := lane.ReadUnset(bed.home); fenced {
		t.Fatalf("a refused unset fenced the lane")
	}
	bed.person = nil
	code, stdout, stderr := bed.run(t, "landing", "unset")
	if code != 0 || !strings.Contains(oneSpaced(stdout), "unset this computer's landing lane "+bed.landingA) || !strings.Contains(oneSpaced(stdout), "each seat lands its own work") {
		t.Fatalf("unset by the person = %d %q %q", code, stdout, stderr)
	}
	if _, ok, err := lane.Read(bed.home); ok || err != nil {
		t.Fatalf("the lane is still registered after unset: %v %v", ok, err)
	}
	if view := bed.status(t); view.Root != nil {
		t.Fatalf("status after unset still shows a lane: %+v", view)
	}
	if code, stdout, _ := bed.run(t, "landing", "unset"); code != 0 || !strings.Contains(stdout, "no landing lane is registered") {
		t.Fatalf("unset again = %d %q", code, stdout)
	}
	if code, _, stderr := bed.run(t, "landing", "set", bed.landingA); code != 0 {
		t.Fatalf("set after unset = %d %q", code, stderr)
	}
	if record, _, _ := lane.Read(bed.home); record.CustodyEpoch != 2 {
		t.Fatalf("epoch after set, unset and set = %d; want 2", record.CustodyEpoch)
	}
	if _, paused := lane.ReadPause(bed.home); paused {
		t.Fatalf("a lane set after an unset comes back stopped by the unset's fence")
	}
}

// An unset that cannot finish says what is left in line 1 and the command
// that continues it in line 2; only unknown state is offered --force.
// While it is under way nothing starts the lane again.
func TestLandingUnsetListsWhatIsLeftAndContinues(t *testing.T) {
	t.Parallel()
	bed := newLaneVerbBed(t)
	if code, _, stderr := bed.run(t, "landing", "set", bed.landingA); code != 0 {
		t.Fatalf("set = %d %q", code, stderr)
	}
	report := lane.UnsetReport{Record: lane.Record{Root: bed.landingA}, Stopped: lane.StepReturned,
		Unresolved: []lane.Unresolved{{Batch: "b1", Member: "goal-a", Reason: "the ledger on main still shows landing+owner holding it for this batch"}}}
	forced := false
	bed.unset = func(home, by string, force bool) (lane.UnsetReport, error) {
		forced = force
		if by != "Wido" {
			t.Errorf("unset for %q; want the proven person", by)
		}
		return report, nil
	}
	code, _, stderr := bed.run(t, "landing", "unset", "--verbose")
	for _, want := range []string{"1 member is not confirmed returned yet", "goal-a", "→ metasystem landing unset", "batch b1, goal-a: the ledger on main"} {
		if code == 0 || !strings.Contains(oneSpaced(stderr), want) {
			t.Errorf("unset with a member left = %d %q; lacks %q", code, stderr, want)
		}
	}
	report = lane.UnsetReport{Record: lane.Record{Root: bed.landingA}, Stopped: lane.StepSettled, Settlement: lane.Settlement{Unknown: []string{"whether the lane's owner runs is unknown"}}}
	code, _, stderr = bed.run(t, "landing", "unset")
	if code == 0 || !strings.Contains(stderr, "whether the lane's owner runs is unknown") || !strings.Contains(stderr, "metasystem landing unset --force") {
		t.Fatalf("unset waiting on unknown state = %d %q", code, stderr)
	}
	if code, _, _ := bed.run(t, "landing", "unset", "--force"); code == 0 || !forced {
		t.Fatalf("--force did not reach the unset")
	}
	report.Settlement = lane.Settlement{Live: []string{"batch b1 is publishing to main"}}
	if code, _, stderr := bed.run(t, "landing", "unset"); code == 0 || strings.Contains(stderr, "--force") {
		t.Fatalf("unset waiting on live work offered --force: %d %q", code, stderr)
	}
	bed.unset = nil
	seams := lane.UnsetSeams{
		Settle: func(lane.Layout) (lane.Settlement, error) {
			return lane.Settlement{Live: []string{"a proof runs"}}, nil
		},
		Records:   func(lane.Layout) ([]batch.Record, error) { return nil, nil },
		Reconcile: func(lane.Layout, batch.Record) ([]lane.Unresolved, error) { return nil, nil },
		Return:    func(lane.Layout, batch.Record, string) ([]lane.Unresolved, error) { return nil, nil },
		Confirm:   func(lane.Layout, []batch.Record) ([]lane.Unresolved, error) { return nil, nil },
	}
	if _, err := lane.Unset(bed.home, "Wido", laneTestNow, false, seams); err != nil {
		t.Fatal(err)
	}
	for _, verb := range []string{"start", "restart"} {
		code, _, stderr := bed.run(t, "landing", verb)
		if code == 0 || !strings.Contains(stderr, "being unset") || !strings.Contains(stderr, "metasystem landing unset") || bed.starts != 0 {
			t.Fatalf("%s during an unset = %d %q starts %d", verb, code, stderr, bed.starts)
		}
	}
}

// Only a person clears a pause (design r10 K2): landing start of a stopped
// lane by anyone else is refused and the lane stays stopped.
func TestLandingStartOfAStoppedLaneIsAPersonsAct(t *testing.T) {
	t.Parallel()
	bed := newLaneVerbBed(t)
	if code, _, stderr := bed.run(t, "landing", "set", bed.landingA); code != 0 {
		t.Fatalf("set = %d %q", code, stderr)
	}
	if code, stdout, stderr := bed.run(t, "landing", "stop"); code != 0 || !strings.Contains(stdout, "metasystem landing unset") {
		t.Fatalf("stop = %d %q %q; want line 2 to name landing unset", code, stdout, stderr)
	}
	bed.person = humanauthority.Refusedf(humanauthority.OutcomeNotEnrolled, "human authority has no readable terminal enrollment")
	code, _, stderr := bed.run(t, "landing", "start")
	if code != 3 || !strings.Contains(stderr, "only a person may resume the landing lane") {
		t.Fatalf("start by no person = %d %q", code, stderr)
	}
	if _, paused := lane.ReadPause(bed.home); !paused || bed.starts != 0 {
		t.Fatalf("a refused start resumed the lane")
	}
	bed.person = nil
	if code, _, stderr := bed.run(t, "landing", "start"); code != 0 || bed.starts != 1 {
		t.Fatalf("start by the person = %d %q", code, stderr)
	}
}

// A checkout on a volume that is not mounted is not gone: unset refuses in
// two plain lines and changes nothing. A checkout removed from its folder
// is gone: unset unregisters it and names, in the default output, the goals
// the ledger still shows the lane holding and the command that gives each
// back.
func TestLandingUnsetOfAnUnreachableOrGoneCheckout(t *testing.T) {
	t.Parallel()
	bed := newLaneVerbBed(t)
	volume := filepath.Join(filepath.Dir(bed.landingA), "volume")
	checkout := filepath.Join(volume, "landing")
	if err := os.MkdirAll(checkout, 0o755); err != nil {
		t.Fatal(err)
	}
	landingCheckout(t, checkout)
	if code, _, stderr := bed.run(t, "landing", "set", checkout); code != 0 {
		t.Fatalf("set = %d %q", code, stderr)
	}
	if err := os.RemoveAll(volume); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := bed.run(t, "landing", "unset")
	if code == 0 || !strings.Contains(stderr, "can't be reached") || !strings.Contains(stderr, "metasystem landing unset") || strings.Contains(stderr, lane.CodeUnreachable) {
		t.Fatalf("unset of an unmounted checkout = %d %q", code, stderr)
	}
	if _, ok, _ := lane.Read(bed.home); !ok {
		t.Fatalf("an unreachable checkout was taken for gone and unregistered")
	}
	if err := os.MkdirAll(volume, 0o755); err != nil {
		t.Fatal(err)
	}
	bed.held = []string{"goal-a", "goal-b"}
	code, stdout, stderr := bed.run(t, "landing", "unset")
	text := oneSpaced(stdout)
	if code != 0 || !strings.Contains(text, "its checkout was gone") || !strings.Contains(text, "2 goals: goal-a, goal-b") ||
		!strings.Contains(text, "metasystem goal release goal-a --reason") {
		t.Fatalf("unset of a gone checkout = %d %q %q; want the held goals and goal release in the default output", code, stdout, stderr)
	}
	if _, ok, _ := lane.Read(bed.home); ok {
		t.Fatalf("the gone lane is still registered")
	}
}

// TestLandingStartWithALandingAgentRunning (A-a, re-review 2 and 3): a
// person's landing start while a landing agent runs starts no batch owner
// beside it and says the agent runs, not that an owner was asked for; and
// the start forgets the last agent's cooldown, so work wakes one at once.
func TestLandingStartWithALandingAgentRunning(t *testing.T) {
	t.Parallel()
	bed := newLaneVerbBed(t)
	if code, _, stderr := bed.run(t, "landing", "set", bed.landingA); code != 0 {
		t.Fatalf("landing set = %d %q", code, stderr)
	}
	bed.alive, bed.agent = false, "landing-0011"
	if _, err := lane.SetPause(bed.home, "Wido", laneTestNow); err != nil {
		t.Fatal(err)
	}
	cooled := filepath.Join(lane.HostDir(bed.home), "landing-agent-keeper.json")
	if err := os.WriteFile(cooled, []byte(`{"launch":"landing-0010","reasons":["validation-due"],"reapedAt":"2026-09-30T12:00:00Z"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	starts := bed.starts
	code, stdout, stderr := bed.run(t, "landing", "start")
	if code != 0 || bed.starts != starts || !strings.Contains(stdout, "landing agent landing-0011 runs") || strings.Contains(stdout, "asked the supervision") {
		t.Fatalf("landing start with an agent running = %d %q %q, owner starts %d -> %d", code, stdout, stderr, starts, bed.starts)
	}
	state, err := lane.ReadAgentState(bed.home)
	if err != nil || len(state.Reasons) != 0 || state.Launch != "landing-0010" {
		t.Fatalf("after a person's start the keeper record is %+v %v; want the cooldown reasons forgotten, the launch kept", state, err)
	}
}

// A validation run that is reserved and whose custody has ended awaits its
// finalization (integration of A-a and K-f): landing status --json names
// it as a wake reason, and the keeper wakes the landing agent on the same
// read, so landing validate finalizes it or runs it again.
func TestLandingWakesForAPendingFinalization(t *testing.T) {
	t.Parallel()
	bed := newLaneVerbBed(t)
	registerLane(t, bed.home, bed.landingA, "Wido", laneTestNow)
	if view := bed.status(t); view.Wake == nil || slices.Contains(view.Wake.Reasons, lane.WakeFinalizationPending) {
		t.Fatalf("a lane with no validation reserved wakes for %+v", view.Wake)
	}
	reservation, err := json.Marshal(gaterun.Validation{RunID: "run-1", Key: goal.CadenceClaimKey{TrunkTree: strings.Repeat("a", 40)}, ReservedAt: laneTestNow.Format(time.RFC3339Nano)})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cadence.ReservationPath(bed.home), reservation, 0o600); err != nil {
		t.Fatal(err)
	}
	if view := bed.status(t); view.Wake == nil || !slices.Contains(view.Wake.Reasons, lane.WakeFinalizationPending) {
		t.Fatalf("a reserved validation that runs no more wakes for %+v; want %s", view.Wake, lane.WakeFinalizationPending)
	}
	keeper := newLandingAgentKeeper(filepath.Join(bed.landingA, "metasystem"), bed.home, newLandingAgent())
	record, _, err := lane.Read(bed.home)
	if err != nil {
		t.Fatal(err)
	}
	keeper.Sources.Validation = nil
	if wake := lane.ReadWake(record, laneTestNow, keeper.Sources); !slices.Contains(wake.Reasons, lane.WakeFinalizationPending) {
		t.Fatalf("the keeper's wake = %+v; want %s", wake, lane.WakeFinalizationPending)
	}
}
