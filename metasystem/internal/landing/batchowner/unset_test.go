package batchowner

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/ownercall"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

const unsetBatchID = "01j5x00000000000000000ns01"

var unsetNow = time.Date(2026, 9, 30, 20, 0, 0, 0, time.UTC)

// unsetBed is a computer with a registered lane in the nested layout
// (checkout root ≠ installation root): a bare origin whose main holds the
// goal ledger under metasystem/plans/goals, the lane's clone of it, a
// scratch clone that publishes ledger changes the way a goal verb would,
// and one batch in the lane holding a goal member and two change members,
// one of them already returning with its own outcome.
type unsetBed struct {
	home, origin, publisher, checkout, seat string
	layout                                  lane.Layout
	store                                   batch.Store
	// releaseErr fails the goal release; publish makes a release that
	// succeeds also reach the ledger on origin (off: the release reports
	// success and main never shows it).
	releaseErr         error
	publish            bool
	releases, edits    int
	releaseRoots       []string
	probes, ends       int
	pendingChange, fix string
}

func unsetGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", dir}, args...)...)
	command.Env = gittree.ScrubbedEnviron()
	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s: %v: %s", args, dir, err, out)
	}
	return strings.TrimSpace(string(out))
}

func unsetGoalFile(id string, claimed *goal.ClaimRecord) []byte {
	state := goal.StateQueued
	history := []goal.HistoryLine{{At: "2026-09-17T10:00:00Z", Opid: "01J5X0000000000000000000BY-human-1a2b3c4d", Verb: "open", Actor: "human:wido", Keep: -1}}
	if claimed != nil {
		state = goal.StateClaimed
		history = append(history, goal.HistoryLine{At: "2026-09-17T10:00:00Z", Opid: "01J5X0000000000000000000B1-" + claimed.Machine + "-1a2b3c4d", Verb: "claim", Actor: claimed.Machine + "+" + claimed.Lineage, Keep: -1})
	}
	return goal.RenderFile(&goal.GoalFile{Id: id, State: state, Intent: "land " + id, Origin: goal.OriginHuman, OpenedAt: "2026-09-17T10:00:00Z", Revision: 2, Claimed: claimed, History: history})
}

func newUnsetBed(t *testing.T) *unsetBed {
	t.Helper()
	base := t.TempDir()
	bed := &unsetBed{home: filepath.Join(base, "home"), origin: filepath.Join(base, "origin.git"), publisher: filepath.Join(base, "publisher"),
		checkout: filepath.Join(base, "landing"), seat: filepath.Join(base, "seat")}
	for _, dir := range []string{bed.home, bed.seat, filepath.Join(bed.publisher, "metasystem", "plans", "goals")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string][]byte{
		"metasystem/go.mod":          []byte("module example.com/m\n\ngo 1.22\n"),
		"metasystem/metasystem.conf": nil,
		// The ledger's root record, so a finalization reads a valid ledger.
		"metasystem/plans/goals/backlog.md": goal.RenderRoot(&goal.RootRecord{Identity: "01J5X000000000000000000000", FormatVersion: "1",
			SyncMode: goal.SyncRemote, MigrationEpoch: "2026-08-20T00:00:00Z", ManifestDigest: strings.Repeat("ab", 32), MigrationMode: "manifest", Revision: 1,
			History: []goal.HistoryLine{{At: "2026-08-20T09:00:00Z", Opid: "01J5X0000000000000000000A0-mac-a-1a2b3c4d", Verb: "migrate", Actor: "mac-a+lin-1", Keep: -1}}}),
		// goal-a is held by the lane for this batch, handed over by the seat.
		"metasystem/plans/goals/goal-a.md": unsetGoalFile("goal-a", &goal.ClaimRecord{Machine: "landing", Lineage: "owner", At: "2026-09-17T10:00:00Z", Revision: 2, AccountingRevision: 1,
			HandedOver: goal.HandedOver{FromMachine: "seat", FromLineage: "lineage-a", FromEpoch: 4, Batch: unsetBatchID}}),
		// The seat holds another goal, so goal-a cannot go back to it: the
		// return releases goal-a instead of handing it back.
		"metasystem/plans/goals/goal-z.md": unsetGoalFile("goal-z", &goal.ClaimRecord{Machine: "seat", Lineage: "lineage-z", At: "2026-09-17T10:00:00Z", Revision: 2, AccountingRevision: 1}),
	}
	for path, data := range files {
		if err := os.WriteFile(filepath.Join(bed.publisher, path), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	unsetGit(t, base, "init", "-q", "--bare", "-b", "main", bed.origin)
	unsetGit(t, bed.publisher, "init", "-q", "-b", "main")
	unsetGit(t, bed.publisher, "config", "user.name", "Test")
	unsetGit(t, bed.publisher, "config", "user.email", "test@example.com")
	unsetGit(t, bed.publisher, "add", ".")
	unsetGit(t, bed.publisher, "commit", "-qm", "ledger")
	unsetGit(t, bed.publisher, "remote", "add", "origin", bed.origin)
	unsetGit(t, bed.publisher, "push", "-q", "origin", "main")
	unsetGit(t, base, "clone", "-q", bed.origin, bed.checkout)
	layout, err := lane.NewLayout(bed.checkout)
	if err != nil {
		t.Fatal(err)
	}
	if string(layout.Install) == string(layout.Checkout) {
		t.Fatalf("the bed's layout is flat: %+v", layout)
	}
	bed.layout = layout
	if _, _, err := lane.Register(bed.home, layout, "Wido", unsetNow.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	change := batch.NewChangeUnit(batch.ChangeMember{Commit: strings.Repeat("5", 40), Parent: strings.Repeat("4", 40), AskedBy: "seat+lineage-c", Subject: "a change"},
		bed.seat, "seat", "lineage-c", []string{"a.go"}, nil)
	change.State = batch.UnitJoined
	pending := batch.NewChangeUnit(batch.ChangeMember{Commit: strings.Repeat("7", 40), Parent: strings.Repeat("4", 40), AskedBy: "seat+lineage-d", Subject: "a red change"},
		bed.seat, "seat", "lineage-d", []string{"b.go"}, nil)
	pending.State = batch.UnitJoined
	bed.fix, bed.pendingChange = change.GoalID, pending.GoalID
	bed.store = batch.NewStore(string(layout.Checkout), nil)
	if err := bed.store.Create(batch.Record{Schema: 1, BatchID: unsetBatchID, State: batch.StateOpen, Units: []batch.Unit{
		{GoalID: "goal-a", Chain: "chain-a", SeatRoot: bed.seat, Claim: batch.Claim{Machine: "seat", Lineage: "lineage-a", Epoch: 4, Revision: 2, AccountingRevision: 1}, State: batch.UnitJoined},
		change, pending,
	}}); err != nil {
		t.Fatal(err)
	}
	// Custody the lane had already begun: a return with its own outcome,
	// which the unset finishes and never replaces.
	if err := batch.RequestReturn(bed.store, unsetBatchID, pending.GoalID, batch.UnitEjected, "its own proof was red", "lane", unsetNow.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	return bed
}

// publishRelease rewrites goal-a unclaimed on origin's main, as the
// person's goal release does.
func (bed *unsetBed) publishRelease(t *testing.T, goalID string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(bed.publisher, "metasystem", "plans", "goals", goalID+".md"), unsetGoalFile(goalID, nil), 0o644); err != nil {
		t.Fatal(err)
	}
	unsetGit(t, bed.publisher, "commit", "-qam", "release "+goalID)
	unsetGit(t, bed.publisher, "push", "-q", "origin", "main")
}

func (bed *unsetBed) unset(t *testing.T) lane.UnsetReport {
	t.Helper()
	steps := UnsetLane{Home: bed.home, By: "Wido", Now: func() time.Time { return unsetNow },
		Calls: BatchOwnerCallSet{
			Handover: func(ownercall.Invocation, ownercall.HandoverRequest) error {
				t.Fatalf("goal-a was handed back to a seat that holds another goal")
				return nil
			},
			EditNext: func(_ ownercall.Invocation, root, goalID, next string) error {
				bed.edits++
				bed.releaseRoots = append(bed.releaseRoots, root)
				return nil
			},
			Release: func(_ ownercall.Invocation, root, goalID string) error {
				bed.releases++
				bed.releaseRoots = append(bed.releaseRoots, root)
				if bed.releaseErr != nil {
					return bed.releaseErr
				}
				if bed.publish {
					bed.publishRelease(t, goalID)
				}
				return nil
			},
		},
		Probe: func(root string) (lane.OwnerProbe, error) {
			bed.probes++
			return lane.OwnerProbe{}, nil
		},
		End: func(string) (int64, error) { bed.ends++; return 0, nil },
	}
	report, err := lane.Unset(bed.home, "Wido", unsetNow, false, steps.Seams())
	if err != nil {
		t.Fatalf("landing unset: %v", err)
	}
	return report
}

func (bed *unsetBed) unit(t *testing.T, id string) batch.Unit {
	t.Helper()
	record, err := bed.store.Load(unsetBatchID)
	if err != nil {
		t.Fatal(err)
	}
	for _, unit := range record.Units {
		if unit.GoalID == id {
			return unit
		}
	}
	t.Fatalf("unit %s is absent", id)
	return batch.Unit{}
}

func unresolvedMembers(report lane.UnsetReport) []string {
	var members []string
	for _, entry := range report.Unresolved {
		members = append(members, entry.Member)
	}
	return members
}

// Design r10 §1 step 4 and 5: every returned member is confirmed by reading
// afterwards — a goal member by the ledger on main, a change member by its
// durable disposition — and the lane is unregistered only when all are. A
// release that reports success while main still shows the lane holding the
// goal keeps the lane registered and fenced, with the member listed; the
// same command then returns it again and unregisters.
func TestUnsetConfirmsEveryReturnBeforeUnregister(t *testing.T) {
	t.Parallel()
	bed := newUnsetBed(t)
	report := bed.unset(t)
	if report.Unregistered || report.Stopped != lane.StepReturned {
		t.Fatalf("unset with an unconfirmed release = %+v; want it stopped at the returns", report)
	}
	if members := unresolvedMembers(report); len(members) != 1 || members[0] != "goal-a" || !strings.Contains(report.Unresolved[0].Reason, "still") {
		t.Fatalf("unresolved = %+v; want goal-a alone, the ledger still showing the lane", report.Unresolved)
	}
	if _, ok, err := lane.Read(bed.home); !ok || err != nil {
		t.Fatalf("the lane was unregistered with goal-a unconfirmed: %v %v", ok, err)
	}
	if err := lane.Gate(bed.home, lane.OpJoin, lane.AuthorityAgent, nil); !isLaneRefusal(err, lane.CodeUnsetting) {
		t.Fatalf("a join during the unset = %v; want %s", err, lane.CodeUnsetting)
	}
	if _, paused := lane.ReadPause(bed.home); !paused {
		t.Fatalf("the unset left the lane unpaused")
	}
	if change := bed.unit(t, bed.fix); change.State != batch.UnitWithdrawn || change.ReturnDisposition != batch.ReturnRecorded {
		t.Fatalf("change member = %s/%s; want withdrawn and recorded", change.State, change.ReturnDisposition)
	}
	for _, root := range bed.releaseRoots {
		if root != string(bed.layout.Install) {
			t.Fatalf("the return published at %s; want the lane's installation %s", root, bed.layout.Install)
		}
	}
	bed.publish = true
	report = bed.unset(t)
	if !report.Unregistered || len(report.Unresolved) != 0 {
		t.Fatalf("unset after the release reached main = %+v; want unregistered", report)
	}
	if bed.releases != 2 {
		t.Fatalf("releases = %d; want the unconfirmed one issued again", bed.releases)
	}
	if _, ok, err := lane.Read(bed.home); ok || err != nil {
		t.Fatalf("the lane is still registered after every member was confirmed: %v %v", ok, err)
	}
	if _, fenced, _ := lane.ReadUnset(bed.home); fenced {
		t.Fatalf("the unset journal outlived the unregistration")
	}
}

// Design r10 §1: unset is resumable. A return that fails stays listed and
// keeps the lane registered and fenced; the same command continues from its
// journal (the settle step is not repeated), finishes the custody that was
// already returning with its own outcome, and unregisters.
func TestUnsetResumesAfterFailedReturn(t *testing.T) {
	t.Parallel()
	bed := newUnsetBed(t)
	bed.releaseErr = errors.New("the ledger push was rejected")
	report := bed.unset(t)
	if report.Unregistered || report.Stopped != lane.StepReturned {
		t.Fatalf("unset with a failed return = %+v; want it stopped at the returns", report)
	}
	if members := unresolvedMembers(report); len(members) != 1 || members[0] != "goal-a" || !strings.Contains(report.Unresolved[0].Reason, "rejected") {
		t.Fatalf("unresolved = %+v; want goal-a with the release's failure", report.Unresolved)
	}
	if goalA := bed.unit(t, "goal-a"); goalA.State != batch.UnitReturnPending {
		t.Fatalf("goal-a = %s after a failed release; want it still returning", goalA.State)
	}
	journal, fenced, err := lane.ReadUnset(bed.home)
	if err != nil || !fenced || !journal.Done(lane.StepSettled) || len(journal.Unresolved) != 1 {
		t.Fatalf("journal = %+v %v %v; want fenced, settled, goal-a listed", journal, fenced, err)
	}
	probes := bed.probes
	bed.releaseErr, bed.publish = nil, true
	report = bed.unset(t)
	if !report.Unregistered || !report.Resumed {
		t.Fatalf("resumed unset = %+v; want it to continue and unregister", report)
	}
	if bed.probes != probes {
		t.Fatalf("the resumed unset settled again (%d probes, was %d)", bed.probes, probes)
	}
	if pending := bed.unit(t, bed.pendingChange); pending.State != batch.UnitEjected || pending.ReturnDisposition != batch.ReturnRecorded || pending.Failure != "its own proof was red" {
		t.Fatalf("custody already returning = %s/%s %q; want its own outcome kept", pending.State, pending.ReturnDisposition, pending.Failure)
	}
	if goalA := bed.unit(t, "goal-a"); goalA.State != batch.UnitWithdrawn || goalA.ReturnDisposition != batch.ReturnReleased {
		t.Fatalf("goal-a = %s/%s; want withdrawn and released", goalA.State, goalA.ReturnDisposition)
	}
	if _, ok, _ := lane.Read(bed.home); ok {
		t.Fatalf("the lane is still registered")
	}
}

func isLaneRefusal(err error, code string) bool {
	var refusal *lane.Refusal
	return errors.As(err, &refusal) && refusal.Code == code
}

// beginLanding records that the batch began its landing on origin's main
// as it stands now: its base is that tree, and no push is recorded.
func (bed *unsetBed) beginLanding(t *testing.T) {
	t.Helper()
	unsetGit(t, bed.publisher, "pull", "-q", "--ff-only", "origin", "main")
	base := unsetGit(t, bed.publisher, "rev-parse", "HEAD^{tree}")
	if err := bed.store.Update(unsetBatchID, func(record *batch.Record) error {
		record.Landing = &batch.LandingProgress{Base: base}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// publishLanding pushes to origin's main a landing commit carrying the
// trailers the lane writes for the batch's joined members, as a push that
// reached main does.
func (bed *unsetBed) publishLanding(t *testing.T, trailers ...string) string {
	t.Helper()
	unsetGit(t, bed.publisher, "pull", "-q", "--ff-only", "origin", "main")
	if err := os.WriteFile(filepath.Join(bed.publisher, "landed.txt"), []byte("landed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	unsetGit(t, bed.publisher, "add", "landed.txt")
	unsetGit(t, bed.publisher, "commit", "-qm", "landing batch\n\n"+strings.Join(trailers, "\n"))
	unsetGit(t, bed.publisher, "push", "-q", "origin", "main")
	return unsetGit(t, bed.publisher, "rev-parse", "HEAD")
}

// Design r10 §1 step 3: an owner that died after its push reached main but
// before recording it leaves a batch with no completed push. When every
// joined member's trailer is on main, the unset records the push and
// finalizes the members as landed, so it finishes and unregisters instead
// of holding the batch (and the host) forever. Only some on main keeps
// the batch and lists those members.
func TestUnsetFinishesAPushThatReachedMainUnrecorded(t *testing.T) {
	t.Parallel()
	partial := newUnsetBed(t)
	partial.publish = true
	partial.beginLanding(t)
	partial.publishLanding(t, batch.LandingChangeTrailer+": "+partial.fix)
	report := partial.unset(t)
	listedOnMain := false
	for _, entry := range report.Unresolved {
		listedOnMain = listedOnMain || entry.Member == partial.fix && strings.Contains(entry.Reason, "is on main")
	}
	if report.Unregistered || !listedOnMain {
		t.Fatalf("unset with part of a batch on main = %+v; want that member listed and the lane kept", report)
	}
	if goalA := partial.unit(t, "goal-a"); goalA.State != batch.UnitJoined {
		t.Fatalf("goal-a = %s; a batch partly on main keeps its members", goalA.State)
	}

	bed := newUnsetBed(t)
	bed.publish = true
	bed.beginLanding(t)
	tip := bed.publishLanding(t, batch.LandingChangeTrailer+": "+bed.fix, "Landing-Provenance: chain=chain-a")
	report = bed.unset(t)
	if !report.Unregistered {
		t.Fatalf("unset after an unrecorded push reached main = %+v; want it finalized and unregistered", report)
	}
	record, err := bed.store.Load(unsetBatchID)
	if err != nil {
		t.Fatal(err)
	}
	if record.Landing == nil || !record.Landing.PushComplete || record.Landing.PushedTip != tip {
		t.Fatalf("landing = %+v; want the push recorded at %s", record.Landing, tip)
	}
	for _, id := range []string{"goal-a", bed.fix} {
		if unit := bed.unit(t, id); unit.State != batch.UnitLanded || !unit.P6Done || unit.LandedCommit != tip {
			t.Fatalf("%s = %s p6=%v commit=%s; want landed at %s", id, unit.State, unit.P6Done, unit.LandedCommit, tip)
		}
	}
}

// Settling re-reads the owner after ending it: an owner still running is
// live work the unset waits for, never taken as ended.
func TestUnsetSettleRereadsTheOwnerAfterEndingIt(t *testing.T) {
	t.Parallel()
	bed := newUnsetBed(t)
	steps := UnsetLane{Home: bed.home, By: "Wido", Now: func() time.Time { return unsetNow },
		Probe: func(string) (lane.OwnerProbe, error) {
			bed.probes++
			return lane.OwnerProbe{Alive: true, PID: 4242}, nil
		},
		End: func(string) (int64, error) { bed.ends++; return 4242, nil }}
	settlement, err := steps.settle(bed.layout)
	if err != nil || settlement.Settled(true) || bed.ends != 1 || bed.probes != 2 {
		t.Fatalf("settle with an owner that outlives its end = %+v %v (ends %d, probes %d); want live work", settlement, err, bed.ends, bed.probes)
	}
}

// Only commits after the batch's base count as its push: trailers of an
// earlier landing of the same members, before the base (landed and later
// reverted), never finalize this batch. Its members are returned instead.
func TestUnsetIgnoresTrailersBeforeTheBatchBase(t *testing.T) {
	t.Parallel()
	bed := newUnsetBed(t)
	bed.publish = true
	bed.publishLanding(t, batch.LandingChangeTrailer+": "+bed.fix, "Landing-Provenance: chain=chain-a")
	unsetGit(t, bed.publisher, "revert", "--no-edit", "HEAD")
	unsetGit(t, bed.publisher, "push", "-q", "origin", "main")
	bed.beginLanding(t)
	report := bed.unset(t)
	if !report.Unregistered {
		t.Fatalf("unset = %+v; want the members returned and the lane unregistered", report)
	}
	for _, id := range []string{"goal-a", bed.fix} {
		if unit := bed.unit(t, id); unit.State != batch.UnitWithdrawn || unit.P6Done {
			t.Fatalf("%s = %s p6=%v; an earlier, reverted landing was taken for this batch's push", id, unit.State, unit.P6Done)
		}
	}
}
