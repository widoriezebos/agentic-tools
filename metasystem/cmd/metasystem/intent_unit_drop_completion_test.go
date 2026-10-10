package main

import (
	"errors"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

func TestWorkDropStatusContinuesPendingPublication(t *testing.T) {
	t.Parallel()
	f := newDropFixture(t)
	f.connect()
	f.bed.manager.Seat = board.Seat{Machine: filepath.Base(filepath.Dir(t.TempDir())), Installation: f.bed.root()}
	if code, result := f.review(t); code != 1 || result.Next == nil {
		t.Fatalf("prepare: %d %+v", code, result)
	}
	f.bed.head = f.v
	before := f.retained(t)
	publish := f.owners.connection.push
	f.owners.connection.push = func(req branch.PushRequest) (branch.PushResult, error) {
		return branch.PushResult{}, errors.New("origin unavailable")
	}
	path := filepath.Join(before.Rounds[1].Directory, "stop-dispositions.md")
	if code, result := f.review(t, "--dispositions", path); code != 1 {
		t.Fatalf("pending publication: %d %+v", code, result)
	}
	raw := f.owners.work.git
	f.owners.work.git = func(dir string, args ...string) ([]byte, error) {
		if slices.Equal(args, []string{"rev-parse", "--verify", "HEAD^{commit}"}) {
			return []byte(f.bed.head), nil
		}
		if slices.Equal(args, []string{"rev-parse", f.inverse + "^{tree}"}) {
			return []byte(f.tree), nil
		}
		return raw(dir, args...)
	}
	code, status := transferPublic(t, f.bed, f.owners, "work", "status", f.bed.id, "--work", "stopped")
	if code != 0 || status.Next == nil || !strings.Contains(status.Summary, "drop pending") || !slices.Contains(status.Next.Argv, path) {
		t.Fatalf("pending status must continue its exact review: %d %+v", code, status)
	}
	f.owners.connection.push = publish
	if code, result := transferPublic(t, f.bed, f.owners, status.Next.Argv[1:]...); code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("status remedy: %d %+v", code, result)
	}
	code, status = transferPublic(t, f.bed, f.owners, "work", "status", f.bed.id, "--work", "stopped")
	if code != 0 || !strings.Contains(status.Summary, "dropped") {
		t.Fatalf("final status: %d %+v", code, status)
	}
	after := f.retained(t)
	if !reflect.DeepEqual(before.Rounds[0].Reads, after.Rounds[0].Reads) || !reflect.DeepEqual(before.Rounds[1].Reads, after.Rounds[1].Reads) || after.Subjects[0].Commit != f.commit {
		t.Fatal("drop rewrote prior read history")
	}

	stale := after
	stale.Subjects[0].Drop = nil
	transferWriteJSON(t, filepath.Join(f.bed.unitRoot, f.run, "run.json"), stale)
	code, status = transferPublic(t, f.bed, f.owners, "work", "status", f.bed.id, "--work", "stopped")
	if code != 0 || !strings.Contains(status.Summary, "dropped") {
		t.Fatalf("older build bookkeeping hid the published drop: %d %+v", code, status)
	}
	// A different fixture can finish a goal on the shared board with a later clock.
	// Its release must not replace this drop's card.
	other := newStopWorkBed(t)
	endpoint, err := other.dependencies().endpoint(other.root())
	if err != nil {
		t.Fatal(err)
	}
	claim := other.goalFile(other.id).Claimed
	if result, err := goal.Release(goal.VerbRequest{Endpoint: endpoint,
		Actor: goal.Actor{Machine: claim.Machine, Lineage: claim.Lineage},
		Now:   f.bed.manager.Now().Add(24 * time.Hour), Ulid: "01J5X00000000000000000TT03"}, other.id); err != nil || result.Outcome != goal.OutcomeConfirmed {
		t.Fatalf("independent goal release: %+v %v", result, err)
	}
	home, err := board.Home()
	if err != nil {
		t.Fatal(err)
	}
	picture, bad := board.Read(home, []board.Seat{f.bed.manager.Seat}, nil, f.bed.manager.Now(), time.Hour)
	for _, unreadable := range bad {
		if !unreadable.Stray {
			t.Fatalf("the drop's seat could not be read: %+v", unreadable)
		}
	}
	if len(picture.Cards) != 1 {
		t.Fatalf("the drop's seat has no unique card: %+v %v", picture, bad)
	}
	card := picture.Cards[0]
	if card.Job == nil || card.Job.Phase != "dropped" || card.Stop == nil || card.Stop.Decision != "dropped" {
		t.Fatalf("board retained an obsolete stop: %+v", card)
	}
	view := board.NewView([]board.Seat{card.Seat}, board.Picture{Cards: []board.Card{card}})
	if line, ok := view.GoalLine(f.bed.id, f.bed.manager.Now(), time.Local); !ok || !strings.Contains(line, "review dropped") {
		t.Fatalf("board display hid the drop: %q", line)
	}
}

func TestWorkLandCountsDroppedUnitAsResolved(t *testing.T) {
	t.Parallel()
	b, owners, _ := admissionBed(t, "hand")
	writeLandingUnits(b, "u1")
	owners.status.Status.Prefix = 0
	owners.status.Status.Units[0].ReadState = "built"
	owners.status.Status.Units[1].ReadState = "dropped"
	owners.status.Status.Units[1].PriorReadState = "needs read"
	owners.status.Status.Units[1].Drop = &goal.UnitDrop{Unit: "u2", Operation: "drop-u2", Commit: strings.Repeat("d", 40)}
	code, result := b.do("work", "land", "standing-validation")
	expectOutcome(t, "unrelated unread work", code, result, intentRefused)
	if !strings.Contains(result.Summary, "u1 has no clean read") || strings.Contains(result.Summary, "u2 has no clean read") || result.Next == nil || flagValue(result.Next.Argv, "--work") != "u1" {
		t.Fatalf("drop demanded a read or hid unrelated work: %+v", result)
	}
	owners.status.Status.Units[0].ReadState = "read clean"
	owners.status.Status.Prefix = 2
	owners.status.Sources = []string{"critic-root", "dropped"}
	writeLandingUnits(b, "u1", "u2")
	code, result = b.do("work", "land", "standing-validation")
	expectOutcome(t, "drop grants no required scope exclusion", code, result, intentRefused)
	if !strings.Contains(result.Summary, "u2 is not built") {
		t.Fatalf("drop completed required scope: %+v", result)
	}
	writeLandingUnits(b, "u1")
	code, result = b.do("work", "land", "standing-validation")
	expectOutcome(t, "dropped work resolved", code, result, intentConfirmed)
}

func TestWorkReviewDroppedCommitNeedsNoRead(t *testing.T) {
	t.Parallel()
	f := newDropFixture(t)
	f.connect()
	if code, result := f.review(t); code != 1 {
		t.Fatalf("prepare: %d %+v", code, result)
	}
	f.bed.head = f.v
	path := filepath.Join(f.retained(t).Rounds[1].Directory, "stop-dispositions.md")
	if code, result := f.review(t, "--dispositions", path); code != 0 {
		t.Fatalf("drop: %d %+v", code, result)
	}
	f.owners.delivery.branchRead = func(args []string) (branch.BranchReadResult, int, error) {
		drop := f.bed.goalFile(f.bed.id).UnitDrops[0]
		if flagValue(args, "--unit") != drop.Commit {
			t.Fatalf("reviewed another commit: %v", args)
		}
		return branch.BranchReadResult{State: "dropped", Published: true, GateRunID: drop.Proof}, 0, nil
	}
	code, result := transferPublic(t, f.bed, f.owners, "work", "review", "--commit", f.inverse, "--goal", f.bed.id)
	if code != 0 || result.Outcome != intentConfirmed || result.Next != nil || !strings.Contains(result.Summary, "no new read") {
		t.Fatalf("drop commit incorrectly awaits examination: %d %+v", code, result)
	}
}
