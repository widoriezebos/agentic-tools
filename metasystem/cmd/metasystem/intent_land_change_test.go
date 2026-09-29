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
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/landpath"
)

// changeLaneBed is a delivery bed whose seat is a real repository with one
// base commit, a configured landing lane, and every lane effect stubbed: the
// landing path's seat steps commit the named paths, the join records its
// request, and the lane's membership answer is the test's.
type changeLaneBed struct {
	*deliveryBed
	lands    []landpath.LandRequest
	joins    []changeJoinRequest
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
	b.owners.changeJoin = func(request changeJoinRequest) (batch.Record, error) {
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
	if len(b.joins) != 1 || !strings.Contains(result.Summary, "unreadable") {
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
	refusing.owners.changeJoin = func(changeJoinRequest) (batch.Record, error) {
		return batch.Record{}, errors.New("BATCH_JOIN_CONFLICT: change does not apply: notes.md")
	}
	refusing.edit("one\nconflicting\n")
	base := refusing.seatGit("rev-parse", "HEAD")
	code, result = refusing.land()
	expectOutcome(t, "refused join", code, result, intentRefused)
	if !strings.Contains(result.Summary, "BATCH_JOIN_CONFLICT") || strings.Contains(result.Summary, "joins it again") ||
		!strings.Contains(result.Summary, "fix them and run the same command") || refusing.seatGit("rev-parse", "HEAD") != base ||
		refusing.seatGit("status", "--porcelain", "--", "notes.md") != "M notes.md" {
		t.Fatalf("refused join: head=%s result=%+v", refusing.seatGit("rev-parse", "HEAD"), result)
	}

	hand := newChangeLaneBed(t, false)
	hand.edit("one\nthree\n")
	code, result = hand.land()
	expectOutcome(t, "no lane", code, result, intentConfirmed)
	if len(hand.lands) != 1 || hand.lands[0].CommitOnly || len(hand.joins) != 0 {
		t.Fatalf("no lane: lands=%+v joins=%+v", hand.lands, hand.joins)
	}
}
