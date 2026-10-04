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

	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// laneVerbBed is a computer with a lane home, two nested landing checkouts
// and a landing agent that runs or not; no process.
type laneVerbBed struct {
	cwd, home, landingA, landingB string
	alive                         bool
	pid                           int64
	person                        error
	// ready is whether the lane can run (nil: it can); noMachine is a
	// checkout without a machine nickname.
	ready     error
	noMachine bool
	// unset replaces landing unset's steps; nil runs the real ones.
	unset func(home, by string, force bool) (lane.UnsetReport, error)
	// wake are the reasons the keeper's wake source names.
	wake []string
	// keeper is the landing agent's keeper landing run steps; helmed is a
	// lane checkout at the helm.
	keeper func(home, root string) lane.AgentKeeper
	helmed bool
	// pause replaces the write of a person's stop; nil writes it.
	pause func(home, by, reason string, now time.Time) (bool, error)
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
		home: func() (string, error) { return bed.home, nil },
		probe: func(string) (lane.OwnerProbe, error) {
			if !bed.alive {
				return lane.OwnerProbe{}, nil
			}
			return lane.OwnerProbe{Alive: true, PID: bed.pid, Since: laneTestNow.Add(-time.Hour)}, nil
		},
		person: func(string) (string, error) {
			if bed.person != nil {
				return "", bed.person
			}
			return "Wido", nil
		},
		validate: func(root, _ string, _ time.Time) (string, error) { return realpath.Resolve(root), nil },
		by:       func(string) string { return "seat" },
		ready:    func(string) error { return bed.ready },
		machine: func(string) (string, error) {
			if bed.noMachine {
				return "", errors.New("no machine nickname is enrolled on this machine")
			}
			return "landing", nil
		},
		now:    func() time.Time { return laneTestNow },
		unset:  bed.unset,
		keeper: bed.keeper,
		pause:  bed.pause,
		helm: func(root string) helm.State {
			if bed.helmed {
				return helm.State{Active: true}
			}
			return helm.Active(root)
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

// The landing verbs register the lane, stop and start it, and a repeat
// whose effect holds is success that changes nothing. landing start starts
// nothing itself: it resumes the lane, whose steward wakes the landing
// agent when there is work (lane design r10 §3); status shows the agent.
func TestLandingVerbsSetStartStop(t *testing.T) {
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
	if view := bed.status(t); view.Root == nil || *view.Root != bed.landingA || *view.RegisteredBy != "Wido" || view.Owner.State != lane.OwnerIdle {
		t.Fatalf("status after set = %+v", view)
	}
	if code, stdout, _ := bed.run(t, "landing", "start"); code != 0 || !strings.Contains(oneSpaced(stdout), "is already running") {
		t.Fatalf("start of a running lane = %d %q", code, stdout)
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
	if code, stdout, _ := bed.run(t, "landing", "start"); code != 0 || !strings.Contains(oneSpaced(stdout), "resumed the landing lane") {
		t.Fatalf("start after stop = %d %q", code, stdout)
	}
	if _, paused := lane.ReadPause(bed.home); paused {
		t.Fatal("a person's start left the lane paused")
	}
	bed.alive = true
	if code, stdout, _ := bed.run(t, "landing", "status", "--verbose"); code != 0 || !strings.Contains(stdout, "registered   by Wido") || !strings.Contains(stdout, "pid 4242") ||
		!strings.Contains(stdout, "agent is at work") {
		t.Fatalf("status --verbose = %d %q", code, stdout)
	}
	if _, ok := findIntentAction("landing", "restart"); ok {
		t.Fatal("landing restart still restarts a batch owner that no longer exists")
	}
}

// landing stop --reason keeps the reason with the pause (design r10 §3,
// the agent's stop and ask): the stop, status and a repeat name it on line
// 1; line 2 stays the command.
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
	if code, _, _ := bed.run(t, "landing", "start"); code != 0 {
		t.Fatalf("start = %d", code)
	}
	if code, stdout, _ := bed.run(t, "landing", "stop", "--by", "Wido"); code != 0 || strings.Contains(stdout, "(") {
		t.Fatalf("stop with no reason = %d %q", code, stdout)
	}
	if pause, _ := lane.ReadPause(bed.home); pause.Reason != "" {
		t.Fatalf("a stop with no reason kept %q", pause.Reason)
	}
	// A reason is one plain line on line 1: newlines and control
	// characters collapse to a space and a long one is cut.
	if code, _, _ := bed.run(t, "landing", "start"); code != 0 {
		t.Fatalf("start = %d", code)
	}
	code, stdout, _ = bed.run(t, "landing", "stop", "--by", "Wido", "--reason", "red twice\n\nrun: rm -rf /\x1b[31m"+strings.Repeat("x", 400))
	pause, _ := lane.ReadPause(bed.home)
	if code != 0 || strings.ContainsAny(pause.Reason, "\n\x1b") || !strings.HasPrefix(pause.Reason, "red twice run: rm -rf / [31m") ||
		len([]rune(pause.Reason)) > 200 || !strings.HasSuffix(pause.Reason, "…") || strings.Contains(stdout, "\x1b") || strings.Contains(stdout, "\n\nrun: rm") {
		t.Fatalf("a stop with a multi-line reason = %d %q; recorded %q", code, stdout, pause.Reason)
	}
}

// A stop whose pause is written but not confirmed on disk is a stop: the
// lane is stopped, and the person reads that this computer could not
// confirm it is on disk.
func TestLandingStopNotConfirmedOnDiskIsAStop(t *testing.T) {
	t.Parallel()
	bed := newLaneVerbBed(t)
	if code, _, stderr := bed.run(t, "landing", "set", bed.landingA, "--by", "Wido"); code != 0 {
		t.Fatalf("set = %d %q", code, stderr)
	}
	bed.pause = func(home, by, reason string, now time.Time) (bool, error) {
		changed, err := lane.SetPauseBecause(home, by, reason, now)
		return changed, errors.Join(err, lane.ErrNotDurable)
	}
	code, stdout, stderr := bed.run(t, "landing", "stop", "--by", "Wido")
	if code != 0 || !strings.Contains(oneSpaced(stdout), "stopped the landing lane for Wido") || !strings.Contains(oneSpaced(stdout), laneUnconfirmed) {
		t.Fatalf("an unconfirmed stop = %d %q %q; want the stop and %q", code, stdout, stderr, laneUnconfirmed)
	}
	if pause, paused := lane.ReadPause(bed.home); !paused || pause.By != "Wido" {
		t.Fatalf("the pause = %+v %v; want stopped by Wido", pause, paused)
	}
	if code, _, _ := bed.run(t, "landing", "start"); code != 0 {
		t.Fatalf("start = %d", code)
	}
	if code, stdout, _ := bed.run(t, "landing", "stop", "--by", "Wido", "--json"); code != 0 || !strings.Contains(stdout, laneUnconfirmed) {
		t.Fatalf("an unconfirmed stop --json = %d %q; want %q in its summary", code, stdout, laneUnconfirmed)
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
	if line := inv.statusLaneLine(); !strings.Contains(line, "landing lane "+bed.landingA) {
		t.Fatalf("lane line = %q", line)
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

// A lane whose checkout's supervision is not armed cannot run its landing
// agent: landing start says so, names the command a person runs and changes
// nothing; it never claims a start.
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
	code, stdout, stderr := bed.run(t, "landing", "start", "--verbose")
	for _, want := range []string{lane.CodeUnarmed, "supervision is not armed", "  → metasystem system start --repo " + bed.landingA, "nothing was started"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("start = %d %q; lacks %q", code, stderr, want)
		}
	}
	if code != 3 || strings.Contains(stdout+stderr, "resumed the") {
		t.Fatalf("start = %d %q %q; want a person's act named, nothing started", code, stdout, stderr)
	}
	if _, paused := lane.ReadPause(bed.home); !paused {
		t.Fatalf("the refused start changed the lane's pause")
	}
	var result struct{ Outcome string }
	_, stdout, _ = bed.run(t, "landing", "start", "--json")
	if json.Unmarshal([]byte(stdout), &result) != nil || result.Outcome != intentRefused {
		t.Fatalf("start --json = %q; want outcome refused", stdout)
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

// landing status says why the lane can't run its landing agent and the one
// command that fixes it, in its one line and as its next step; set names
// that step too.
func TestLandingStatusSaysWhyTheLaneCannotRun(t *testing.T) {
	t.Parallel()
	bed := newLaneVerbBed(t)
	bed.ready = lane.UnarmedRefusal(bed.landingA)
	code, stdout, stderr := bed.run(t, "landing", "set", bed.landingA)
	if code != 0 || !strings.Contains(stdout, "→ metasystem system start --repo "+bed.landingA) {
		t.Fatalf("set on an unarmed checkout = %d %q %q; want the arming named next", code, stdout, stderr)
	}
	code, stdout, _ = bed.run(t, "landing", "status")
	for _, want := range []string{"! The landing lane can't run its agent", "supervision is not armed", "→ metasystem system start --repo " + bed.landingA} {
		if code != 0 || !strings.Contains(oneSpaced(stdout), want) {
			t.Errorf("status = %d %q; lacks %q", code, stdout, want)
		}
	}
}

// oneSpaced is a page with every run of spaces and line breaks one space:
// what it says, however its lines wrapped.
func oneSpaced(page string) string { return strings.Join(strings.Fields(page), " ") }

// landing unset is a person's act (design r10 §1): refused to anyone else
// with nothing changed; for the person it fences, settles and unregisters
// the lane, after which there is no lane and each seat lands
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

// An unset that cannot finish says what it waits for in line 1 and the
// command that continues it in line 2; only unknown state is offered
// --force. While it is under way nothing starts the lane again.
func TestLandingUnsetListsWhatIsLeftAndContinues(t *testing.T) {
	t.Parallel()
	bed := newLaneVerbBed(t)
	if code, _, stderr := bed.run(t, "landing", "set", bed.landingA); code != 0 {
		t.Fatalf("set = %d %q", code, stderr)
	}
	var report lane.UnsetReport
	forced := false
	bed.unset = func(home, by string, force bool) (lane.UnsetReport, error) {
		forced = force
		if by != "Wido" {
			t.Errorf("unset for %q; want the proven person", by)
		}
		return report, nil
	}
	report = lane.UnsetReport{Record: lane.Record{Root: bed.landingA}, Stopped: lane.StepSettled, Settlement: lane.Settlement{Unknown: []string{"whether the lane's owner runs is unknown"}}}
	code, _, stderr := bed.run(t, "landing", "unset")
	if code == 0 || !strings.Contains(stderr, "whether the lane's owner runs is unknown") || !strings.Contains(stderr, "metasystem landing unset --force") {
		t.Fatalf("unset waiting on unknown state = %d %q", code, stderr)
	}
	if code, _, _ := bed.run(t, "landing", "unset", "--force"); code == 0 || !forced {
		t.Fatalf("--force did not reach the unset")
	}
	report.Settlement = lane.Settlement{Live: []string{"the landing agent landing-1 still runs"}}
	if code, _, stderr := bed.run(t, "landing", "unset"); code == 0 || strings.Contains(stderr, "--force") {
		t.Fatalf("unset waiting on live work offered --force: %d %q", code, stderr)
	}
	bed.unset = nil
	seams := lane.UnsetSeams{
		Settle: func(lane.Layout) (lane.Settlement, error) {
			return lane.Settlement{Live: []string{"the landing agent runs"}}, nil
		},
	}
	if _, err := lane.Unset(bed.home, "Wido", laneTestNow, false, seams); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := bed.run(t, "landing", "start"); code == 0 || !strings.Contains(stderr, "being unset") || !strings.Contains(stderr, "metasystem landing unset") {
		t.Fatalf("start during an unset = %d %q", code, stderr)
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
	if _, paused := lane.ReadPause(bed.home); !paused {
		t.Fatalf("a refused start resumed the lane")
	}
	bed.person = nil
	if code, _, stderr := bed.run(t, "landing", "start"); code != 0 {
		t.Fatalf("start by the person = %d %q", code, stderr)
	}
	if _, paused := lane.ReadPause(bed.home); paused {
		t.Fatalf("the person's start left the lane stopped")
	}
}

// A checkout on a volume that is not mounted is not gone: unset refuses in
// two plain lines and changes nothing. A checkout removed from its folder
// is gone: unset unregisters it and says so.
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
	code, stdout, stderr := bed.run(t, "landing", "unset")
	if text := oneSpaced(stdout); code != 0 || !strings.Contains(text, "its checkout was gone") || !strings.Contains(text, "Each seat lands its own work now") {
		t.Fatalf("unset of a gone checkout = %d %q %q", code, stdout, stderr)
	}
	if _, ok, _ := lane.Read(bed.home); ok {
		t.Fatalf("the gone lane is still registered")
	}
}

// TestLandingStartWithALandingAgentRunning (A-a, re-review 2 and 3): a
// person's landing start while a landing agent runs starts nothing beside
// it, and keeps the keeper's record of that agent.
func TestLandingStartWithALandingAgentRunning(t *testing.T) {
	t.Parallel()
	bed := newLaneVerbBed(t)
	if code, _, stderr := bed.run(t, "landing", "set", bed.landingA); code != 0 {
		t.Fatalf("landing set = %d %q", code, stderr)
	}
	bed.alive = true
	if _, err := lane.SetPause(bed.home, "Wido", laneTestNow); err != nil {
		t.Fatal(err)
	}
	cooled := filepath.Join(lane.HostDir(bed.home), "landing-agent-keeper.json")
	if err := os.WriteFile(cooled, []byte(`{"launch":"landing-0010","reapedAt":"2026-09-30T12:00:00Z"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := bed.run(t, "landing", "start")
	if code != 0 || !strings.Contains(oneSpaced(stdout), "resumed the landing lane") {
		t.Fatalf("landing start with an agent running = %d %q %q", code, stdout, stderr)
	}
	state, err := lane.ReadAgentState(bed.home)
	if err != nil || state.Launch != "landing-0010" {
		t.Fatalf("after a person's start the keeper record is %+v %v; want the launch kept", state, err)
	}
}
