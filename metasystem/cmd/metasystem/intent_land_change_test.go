package main

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batchowner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/landpath"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
)

// changeLaneBed is a delivery bed whose seat is a real repository with one
// base commit, a configured landing lane, and every lane effect stubbed: the
// landing path's seat steps commit the named paths, the join records its
// request, and the lane's membership answer is the test's.
type changeLaneBed struct {
	*deliveryBed
	lands    []landpath.LandRequest
	joins    []batchowner.ChangeJoinRequest
	member   *batch.Unit
	lookup   error
	advances int
	seatGit  func(args ...string) string
}

func newChangeLaneBed(t *testing.T, configured bool) *changeLaneBed {
	t.Helper()
	b := &changeLaneBed{deliveryBed: newDeliveryBed(t)}
	root := b.install
	b.seatGit = func(args ...string) string {
		t.Helper()
		command := exec.Command("git", append([]string{"-C", root, "-c", "user.name=Wido", "-c", "user.email=wido@example.com"}, args...)...)
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
		return strings.TrimSpace(string(output))
	}
	if err := os.WriteFile(filepath.Join(root, "notes.md"), []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	b.seatGit("checkout", "-q", "-B", "main")
	b.seatGit("add", "notes.md")
	b.seatGit("commit", "-qm", "base")
	b.owners.batchRoot = func(string, time.Time) (string, bool, error) { return "/landing", configured, nil }
	b.owners.landPath = func(_ landpath.Owners, request landpath.LandRequest, _, _ io.Writer) int {
		b.lands = append(b.lands, request)
		b.seatGit("add", "--", "notes.md")
		b.seatGit("commit", "-qm", "record: notes\n\nMachine: m1e+human\nLanding-Provenance-Verdict: would-refuse code=missing-declaration")
		return 0
	}
	b.owners.changeHeld = func(string, string, string, string) (string, int) { return "held: ok", 0 }
	b.owners.changeJoin = func(request batchowner.ChangeJoinRequest) (batch.Record, error) {
		b.joins = append(b.joins, request)
		unit := batch.NewChangeUnit(batch.ChangeMember{Commit: request.Commit, AskedBy: "m1e+human"}, request.SeatRoot, "m1e", "human", nil, nil)
		unit.State = batch.UnitJoined
		b.member = &unit
		return batch.Record{BatchID: "b-1", State: batch.StateOpen, Units: []batch.Unit{unit}}, nil
	}
	b.owners.changeUnit = func(_, id string) (batch.Record, batch.Unit, bool, error) {
		if b.lookup != nil {
			return batch.Record{}, batch.Unit{}, false, b.lookup
		}
		if b.member == nil || b.member.GoalID != id {
			return batch.Record{}, batch.Unit{}, false, nil
		}
		return batch.Record{BatchID: "b-1", State: batch.StateOpen}, *b.member, true, nil
	}
	b.owners.changeAdvance = func(string, string) error { b.advances++; return nil }
	return b
}

func (b *changeLaneBed) edit(content string) {
	b.t.Helper()
	if err := os.WriteFile(filepath.Join(b.install, "notes.md"), []byte(content), 0o644); err != nil {
		b.t.Fatal(err)
	}
}

func (b *changeLaneBed) land() (int, intentResult) {
	b.t.Helper()
	message := filepath.Join(b.t.TempDir(), "message.txt")
	if err := os.WriteFile(message, []byte("record: notes\n"), 0o644); err != nil {
		b.t.Fatal(err)
	}
	return b.do("work", "land", "--message", message, "--path", "notes.md")
}

// TestWorkLandMessageJoinsTheLaneAsAChange (U11b): with a landing lane, work
// land --message runs the landing path's seat steps up to the commit, pins
// the commit and joins it to the lane as a change; the same command with
// nothing new is the continuation (in progress, then landed and the seat
// advanced); an ejected change is refused with its reason and its changes
// given back to the seat; a lane that cannot be read refuses and never joins
// twice; without a lane the hand path lands as before.
func TestWorkLandMessageJoinsTheLaneAsAChange(t *testing.T) {
	t.Parallel()
	b := newChangeLaneBed(t, true)
	b.edit("one\ntwo\n")
	code, result := b.land()
	expectOutcome(t, "change joins", code, result, intentInProgress)
	head := b.seatGit("rev-parse", "HEAD")
	id := batch.ChangeID(head)
	if len(b.lands) != 1 || !b.lands[0].CommitOnly || len(b.joins) != 1 || b.joins[0].Commit != head ||
		!strings.Contains(result.Summary, "change "+id+" joined landing batch b-1") {
		t.Fatalf("join: lands=%+v joins=%+v result=%+v", b.lands, b.joins, result)
	}
	if pinned := b.seatGit("rev-parse", "refs/metasystem/changes/"+strings.TrimPrefix(id, "change:")); pinned != head {
		t.Fatalf("pin=%s head=%s", pinned, head)
	}

	code, result = b.land()
	expectOutcome(t, "repeat in progress", code, result, intentInProgress)
	if len(b.lands) != 1 || len(b.joins) != 1 || !strings.Contains(result.Summary, "change "+id+" is joined in landing batch b-1") {
		t.Fatalf("repeat: lands=%d joins=%d result=%+v", len(b.lands), len(b.joins), result)
	}

	b.lookup = errors.New("batch 01j5x unreadable: unexpected end of JSON input")
	code, result = b.land()
	expectOutcome(t, "unreadable lane", code, result, intentRefused)
	if len(b.joins) != 1 || !strings.Contains(result.Summary, "can't be read") {
		t.Fatalf("unreadable lane: joins=%d result=%+v", len(b.joins), result)
	}
	b.lookup = nil

	ejected := *b.member
	ejected.State, ejected.Outcome = batch.UnitEjected, batch.UnitEjected
	ejected.Failure = "EJECTED from landing batch b-1: TestNotes failed on the batch tip and passes on main; log /tmp/x.log"
	ejected.ReturnDisposition = batch.ReturnRecorded
	b.member = &ejected
	parent := b.seatGit("rev-parse", "HEAD^")
	code, result = b.land()
	expectOutcome(t, "ejected", code, result, intentRefused)
	if !strings.Contains(result.Summary, "TestNotes failed on the batch tip") || b.seatGit("rev-parse", "HEAD") != parent ||
		b.seatGit("status", "--porcelain", "--", "notes.md") != "M notes.md" || len(b.joins) != 1 {
		t.Fatalf("ejected: head=%s joins=%d result=%+v", b.seatGit("rev-parse", "HEAD"), len(b.joins), result)
	}

	// Fixed and landed again: a new change joins, lands, and the seat advances.
	b.edit("one\ntwo fixed\n")
	code, result = b.land()
	expectOutcome(t, "fixed change joins", code, result, intentInProgress)
	fixed := b.seatGit("rev-parse", "HEAD")
	if len(b.lands) != 2 || len(b.joins) != 2 || b.joins[1].Commit != fixed || fixed == head {
		t.Fatalf("fixed: lands=%d joins=%+v", len(b.lands), b.joins)
	}
	landed := *b.member
	landed.State, landed.LandedCommit = batch.UnitLanded, "c0ffee0000000000000000000000000000000000"
	b.member = &landed
	code, result = b.land()
	expectOutcome(t, "landed", code, result, intentUnchanged)
	if b.advances != 1 || !strings.Contains(result.Summary, "change "+batch.ChangeID(fixed)+" already landed as c0ffee0") || changePinned(b.install, fixed) {
		t.Fatalf("landed: advances=%d result=%+v", b.advances, result)
	}

	// A refused join gives the commit back and says what to do (N-2).
	refusing := newChangeLaneBed(t, true)
	refusing.owners.changeJoin = func(batchowner.ChangeJoinRequest) (batch.Record, error) {
		return batch.Record{}, errors.New("BATCH_JOIN_CONFLICT: change does not apply: notes.md")
	}
	refusing.edit("one\nconflicting\n")
	base := refusing.seatGit("rev-parse", "HEAD")
	code, result = refusing.land()
	expectOutcome(t, "refused join", code, result, intentRefused)
	if !strings.Contains(result.Summary, "change does not apply: notes.md") || strings.Contains(result.Summary, "BATCH_JOIN_CONFLICT") ||
		!strings.Contains(strings.Join(result.Details, "\n"), "BATCH_JOIN_CONFLICT") || !strings.Contains(result.Decision, "fix them, then repeat this command") || refusing.seatGit("rev-parse", "HEAD") != base ||
		refusing.seatGit("status", "--porcelain", "--", "notes.md") != "M notes.md" {
		t.Fatalf("refused join: head=%s result=%+v", refusing.seatGit("rev-parse", "HEAD"), result)
	}

	// A change stacked on a change another batch holds is refused and the
	// seat keeps both commits (B-1a); a join whose owner could not be started
	// is joined, nothing given back (N-a).
	stackedBed := newChangeLaneBed(t, true)
	stackedBed.owners.changeJoin = func(request batchowner.ChangeJoinRequest) (batch.Record, error) {
		return batch.Record{}, &batch.StackedChangeRefusal{Reason: "BATCH_CHANGE_STACKED_ELSEWHERE: change " + batch.ChangeID(request.Commit) +
			" is stacked on change change:aaaaaaaaaaaa, which is in batch b-9 (proving); run the same command after change:aaaaaaaaaaaa lands"}
	}
	stackedBed.edit("one\nstacked\n")
	code, result = stackedBed.land()
	expectOutcome(t, "stacked elsewhere", code, result, intentRefused)
	stackedHead := stackedBed.seatGit("rev-parse", "HEAD")
	if !strings.Contains(result.Summary, "which is in batch b-9 (proving); run the same command after") || !changePinned(stackedBed.install, stackedHead) ||
		stackedBed.seatGit("log", "-1", "--format=%s") != "record: notes" {
		t.Fatalf("stacked: result=%+v", result)
	}
	ownerless := newChangeLaneBed(t, true)
	ownerless.owners.changeJoin = func(request batchowner.ChangeJoinRequest) (batch.Record, error) {
		record := batch.Record{BatchID: "b-3", State: batch.StateOpen}
		return record, &batchowner.ChangeOwnerStartError{Record: record, Cause: errors.New("BATCH_OWNER_INDETERMINATE: supervision refused")}
	}
	ownerless.edit("one\nownerless\n")
	code, result = ownerless.land()
	expectOutcome(t, "joined without an owner", code, result, intentInProgress)
	ownerlessHead := ownerless.seatGit("rev-parse", "HEAD")
	if !strings.Contains(result.Summary, "joined landing batch b-3, but the lane couldn't be started: supervision refused") ||
		result.Next == nil || strings.Join(result.Next.Argv, " ") != "metasystem landing start" || !changePinned(ownerless.install, ownerlessHead) {
		t.Fatalf("ownerless: result=%+v", result)
	}

	hand := newChangeLaneBed(t, false)
	hand.edit("one\nthree\n")
	code, result = hand.land()
	expectOutcome(t, "no lane", code, result, intentConfirmed)
	if len(hand.lands) != 1 || hand.lands[0].CommitOnly || len(hand.joins) != 0 {
		t.Fatalf("no lane: lands=%+v joins=%+v", hand.lands, hand.joins)
	}
}

// Design r10 §1 and §6 step 10 (no-lane mode): after a person's landing
// unset, a seat whose landing.batch-root still names the old lane lands its
// own work through the hand path; nothing refuses it, nothing joins, and
// nothing registers the lane again. The lane resolution and the unset are
// the real ones over a real host home and a nested landing checkout.
func TestWorkLandAfterUnsetLandsOnTheSeat(t *testing.T) {
	t.Parallel()
	b := newChangeLaneBed(t, true)
	base := t.TempDir()
	home, landing := filepath.Join(base, "home"), filepath.Join(base, "landing")
	registerLane(t, home, landing, "Wido", time.Now())
	landing = realpath.Resolve(landing)
	if err := os.WriteFile(filepath.Join(b.install, "metasystem.conf.local"), []byte("landing.batch-root="+landing+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	seams := batchowner.LandingLaneSeams{Home: func() (string, error) { return home, nil },
		Validate: func(root, _ string, _ time.Time) (string, error) { return realpath.Resolve(root), nil }}
	b.owners.batchRoot = seams.BatchRoot
	steps := batchowner.ProductionUnsetLane(home, "Wido")
	steps.Probe = func(string) (lane.OwnerProbe, error) { return lane.OwnerProbe{}, nil }
	steps.End = func(string) (int64, error) { return 0, nil }
	report, err := lane.Unset(home, "Wido", time.Now(), false, steps.Seams())
	if err != nil || !report.Unregistered {
		t.Fatalf("unset = %+v %v", report, err)
	}
	b.edit("one\nafter the lane\n")
	code, result := b.land()
	expectOutcome(t, "no lane after unset", code, result, intentConfirmed)
	if len(b.lands) != 1 || b.lands[0].CommitOnly || len(b.joins) != 0 {
		t.Fatalf("after unset: lands=%+v joins=%+v; want the seat's own landing", b.lands, b.joins)
	}
	if _, ok, err := lane.Read(home); ok || err != nil {
		t.Fatalf("landing after unset registered a lane again: %v %v", ok, err)
	}
}
