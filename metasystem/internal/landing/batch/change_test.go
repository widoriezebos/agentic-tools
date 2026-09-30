package batch

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/adapter"
)

const changeCommit = "abcdef0123456789abcdef0123456789abcdef01"

func changeMemberUnit() Unit {
	return NewChangeUnit(ChangeMember{Commit: changeCommit, Parent: testCommit(90), AskedBy: "m1e+human", Subject: "record: notes"},
		"/seats/m1e/metasystem", "m1e", "human", []string{"records/notes.md"}, nil)
}

func openBatchWithGoalMember(base string) Record {
	return Record{Schema: 1, BatchID: testBatchID, State: StateOpen, TipTree: testCommit(103),
		Units:             []Unit{{GoalID: "goal-a", Chain: "chain-a", Claim: Claim{Machine: "landing", Lineage: "owner", Epoch: 1, Revision: 7, AccountingRevision: 5}, State: UnitJoined}},
		History:           []HistoryEntry{{At: ten.Format(time.RFC3339Nano), Verb: "join", Detail: "goal-a joined"}},
		batchRecordFields: batchRecordFields{BaseTree: base, PrefixTrees: []string{testCommit(103)}}}
}

// TestChangeRidesWithGoalMemberAndLandsWithTrailer (U11b): a change joins the
// open batch that holds a goal member, as one joined member with the asker's
// seat and no claim revisions; a repeat changes nothing; the batch's proof is
// charged to the goal member; the landing replays the change after the goal
// member's commit and the replay carries the Landing-Change trailer.
func TestChangeRidesWithGoalMemberAndLandsWithTrailer(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	base := testCommit(101)
	store := NewStore(root, nil)
	strictReassembly(t, &store,
		expectedAssembly(base, []string{"goal-a", "change:abcdef012345"}, []string{"chain-a", "change:abcdef012345"}, []string{testCommit(103), testCommit(104)}),
		expectedAssembly(base, []string{"change:abcdef012345"}, []string{"change:abcdef012345"}, []string{testCommit(105)}),
	)
	must(t, store.Create(openBatchWithGoalMember(base)))
	join := ChangeJoin{Unit: changeMemberUnit(), BaseTree: base, NewID: "01j5x00000000000000000ba99", Actor: "m1e+human", At: ten.Add(time.Minute)}
	record, err := JoinChange(store, join)
	must(t, err)
	if record.BatchID != testBatchID || len(record.Units) != 2 || !slices.Equal(record.PrefixTrees, []string{testCommit(103), testCommit(104)}) || record.TipTree != testCommit(104) {
		t.Fatalf("joined record=%+v", record)
	}
	unit := record.Units[1]
	if unit.GoalID != "change:abcdef012345" || unit.Chain != unit.GoalID || unit.State != UnitJoined || !unit.IsChange() ||
		unit.Claim != (Claim{Machine: "m1e", Lineage: "human"}) || unit.Admission == nil || unit.Admission.Tree != testCommit(105) || unit.Admission.Status != AdmissionCarried {
		t.Fatalf("change member=%+v admission=%+v", unit, unit.Admission)
	}
	// A repeat reads the membership: nothing is assembled or written again.
	again, err := JoinChange(store, join)
	must(t, err)
	if len(again.History) != len(record.History) || len(again.Units) != 2 {
		t.Fatalf("repeat changed the record: %+v", again)
	}
	if charge, ok := ChargeMember(joinedUnits(again.Units)); !ok || charge.GoalID != "goal-a" {
		t.Fatalf("charge member=%+v ok=%v", charge, ok)
	}
	must(t, store.Update(testBatchID, func(current *Record) error {
		current.Proof = &Proof{Status: "green", AttemptID: "tip-attempt"}
		current.Receipts = map[string]PrefixReceipt{"goal-a": {GoalID: "goal-a", Tree: testCommit(103), AttemptID: "prefix-attempt"}}
		current.Transition(StateLanding, ten.Add(2*time.Minute), "prove", "owner", "green")
		return nil
	}))
	var events []string
	seams := greenLandSeams(&events)
	seams.ReplayChange = func(unit Unit) (string, error) {
		events = append(events, "replay:"+unit.GoalID)
		return "replayed-change", nil
	}
	must(t, LandSeries(store, testBatchID, "owner", ten.Add(3*time.Minute), seams))
	if want := []string{"apply:goal-a", "receipt:goal-a", "commit:goal-a", "replay:change:abcdef012345", "held", "push", "cleanup"}; !slices.Equal(events, want) {
		t.Fatalf("landing events=%v want %v", events, want)
	}
	landed := load(t, store)
	if landed.Landing == nil || landed.Landing.Commits["change:abcdef012345"] != "replayed-change" || landed.Landing.PushedTip != "replayed-change" {
		t.Fatalf("landing progress=%+v", landed.Landing)
	}
	message := ChangeLandingMessage("record: notes\n\nWhy.\n\nMachine: m1e+human\n", "change:abcdef012345")
	if message != "record: notes\n\nWhy.\n\nMachine: m1e+human\nLanding-Change: change:abcdef012345\n" {
		t.Fatalf("replay message=%q", message)
	}
}

// TestReplayChangeGitAdapterKeepsTheAskersTrailersAndAddsLandingChange: the
// replay's claim is Git's: the change's patch lands on the landing branch as
// one commit whose parent is the branch head, whose tree is the change's, and
// whose message keeps the asker's trailers and identity and adds one
// Landing-Change trailer.
func TestReplayChangeGitAdapterKeepsTheAskersTrailersAndAddsLandingChange(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	run := func(env []string, args ...string) string {
		t.Helper()
		command := exec.Command("git", append([]string{"-C", root}, args...)...)
		command.Env = append(os.Environ(), env...)
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
		return strings.TrimSpace(string(output))
	}
	run(nil, "init", "-q", "-b", "main")
	run(nil, "config", "user.name", "Landing Owner")
	run(nil, "config", "user.email", "owner@example.com")
	must(t, os.WriteFile(filepath.Join(root, "notes.md"), []byte("one\n"), 0o644))
	run(nil, "add", "notes.md")
	run(nil, "commit", "-qm", "base")
	base := run(nil, "rev-parse", "HEAD")
	run(nil, "checkout", "-q", "-b", "seat")
	must(t, os.WriteFile(filepath.Join(root, "notes.md"), []byte("one\ntwo\n"), 0o644))
	run(nil, "add", "notes.md")
	asker := []string{"GIT_AUTHOR_NAME=Wido", "GIT_AUTHOR_EMAIL=wido@example.com", "GIT_COMMITTER_NAME=Wido", "GIT_COMMITTER_EMAIL=wido@example.com"}
	run(asker, "commit", "-qm", "record: notes\n\nMachine: m1e+human\nLanding-Provenance-Verdict: would-refuse code=missing-declaration")
	change := run(nil, "rev-parse", "HEAD")
	changeTree := run(nil, "rev-parse", "HEAD^{tree}")
	run(nil, "checkout", "-q", "-B", "landing/"+testBatchID, base)
	unit := NewChangeUnit(ChangeMember{Commit: change, Parent: base, AskedBy: "m1e+human", Subject: "record: notes"}, root, "m1e", "human", []string{"notes.md"}, nil)
	replayed, err := ReplayChange(root, unit)
	must(t, err)
	if head := run(nil, "rev-parse", "HEAD"); head != replayed || replayed == change {
		t.Fatalf("head=%s replayed=%s change=%s", head, replayed, change)
	}
	if parent := run(nil, "rev-parse", replayed+"^"); parent != base {
		t.Fatalf("replay parent=%s want %s", parent, base)
	}
	if tree := run(nil, "rev-parse", replayed+"^{tree}"); tree != changeTree {
		t.Fatalf("replay tree=%s want %s", tree, changeTree)
	}
	message := run(nil, "show", "-s", "--format=%B", replayed)
	if !strings.Contains(message, "Machine: m1e+human\n") || !strings.Contains(message, "Landing-Provenance-Verdict: would-refuse code=missing-declaration\nLanding-Change: "+unit.GoalID) {
		t.Fatalf("replay message=%q", message)
	}
	if author := run(nil, "show", "-s", "--format=%an <%ae>|%cn <%ce>", replayed); author != "Wido <wido@example.com>|Wido <wido@example.com>" {
		t.Fatalf("replay identity=%q", author)
	}
	if status := run(nil, "status", "--porcelain"); status != "" {
		t.Fatalf("replay left the checkout dirty: %q", status)
	}
}

// TestChangeOnlyBatchStartsAtOnce (U11b, the coordinator's ruling): a batch
// whose joined members are all changes starts by the knowledge-driven start
// rule like any batch, nothing reachable meaning now; it never waits for a
// goal member, since its proof is charged to the lane. Its diagnosis is keyed
// to its last change, which the lane owner charges to the lane.
func TestChangeOnlyBatchStartsAtOnce(t *testing.T) {
	t.Parallel()
	record := Record{Schema: 1, BatchID: testBatchID, State: StateOpen, Units: []Unit{changeMemberUnit()},
		History: []HistoryEntry{{At: ten.Format(time.RFC3339Nano), Verb: "join", Detail: "change:abcdef012345 joined"}}}
	record.Units[0].State = UnitJoined
	bed := newOwnerBed(t, record, ten)
	bed.owner.pipeline = &scriptedBoard{picture: BoardPicture{Readable: true}}
	started := tickAt(t, bed, ten)
	if bed.launches != 1 || started.Wait != nil || started.StartReason != "nothing within reach" {
		t.Fatalf("change-only batch: launches=%d wait=%+v reason=%q", bed.launches, started.Wait, started.StartReason)
	}
	if charge := ChargeUnit(joinedUnits(started.Units)); charge.GoalID != "change:abcdef012345" {
		t.Fatalf("charge unit=%+v", charge)
	}
}

// TestRedChangeIsEjectedAndTheAskerIsTold (U11b): an ejected change settles
// at once with its reason kept for the asker (there is no ledger claim to
// hand back, and the goal ledger is never read for it); a known flake a
// change's closure owns is owned by the charge member's approver.
func TestRedChangeIsEjectedAndTheAskerIsTold(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	base := testCommit(101)
	store := NewStore(root, nil)
	strictReassembly(t, &store, expectedAssembly(base, []string{"goal-a"}, []string{"chain-a"}, []string{testCommit(103)}))
	record := openBatchWithGoalMember(base)
	change := changeMemberUnit()
	change.State = UnitJoined
	record.Units = append(record.Units, change)
	record.PrefixTrees = []string{testCommit(103), testCommit(104)}
	must(t, store.Create(record))
	reason := "EJECTED from landing batch " + testBatchID + ": TestNotes (records) failed on the batch tip and passes on main"
	must(t, ReassembleSurvivorsWithReturns(store, testBatchID, "owner", ten, []ReturnDecision{{GoalID: change.GoalID, Outcome: UnitEjected, Reason: reason}}))
	reads := 0
	must(t, returnUnitsErr(store, testBatchID, base, "owner", ten, ReturnSeams{
		Read: func(_, _, goalID string) (ReturnLedgerGoal, error) {
			reads++
			return ReturnLedgerGoal{}, errors.New("the goal ledger has no " + goalID)
		},
		Target: func(Unit) ReturnTarget { return ReturnTarget{State: ReturnTargetUnknown} },
	}))
	settled := load(t, store)
	got := settled.Units[1]
	if reads != 0 || got.State != UnitEjected || got.ReturnDisposition != ReturnRecorded || got.Failure != reason || settled.Units[0].State != UnitJoined {
		t.Fatalf("reads=%d change=%+v goal=%+v", reads, got, settled.Units[0])
	}
	owned := change
	owned.Closure = &adapter.Closure{Changed: []string{"./records"}}
	goal := record.Units[0]
	goal.Approver = "wido"
	if owner := flakeOwner([]Unit{goal, owned}, nil, Failure{}); owner.GoalID != "goal-a" {
		t.Fatalf("flake owner=%s, want the charge member", owner.GoalID)
	}
}

// TestChangeRecoveryFindsLandingChangeTrailer (U11b): after the push a change
// is recognized by its Landing-Change trailer on origin, never by the chain
// provenance a certified chain carries, and has no goal Next to finalize; a
// repeat changes nothing, and an unreadable origin log fails the recovery
// instead of reading as "not landed yet".
func TestChangeRecoveryFindsLandingChangeTrailer(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	base := testCommit(101)
	store := NewStore(root, nil)
	record := openBatchWithGoalMember(base)
	change := changeMemberUnit()
	change.State = UnitJoined
	record.Units = append(record.Units, change)
	record.State, record.PrefixTrees, record.TipTree = StateLanding, []string{testCommit(103), testCommit(104)}, testCommit(104)
	record.Proof = &Proof{Status: "green", AttemptID: "tip"}
	record.Landing = &LandingProgress{Base: base, PushComplete: true, PushedTip: "pushed", CandidateTip: "pushed", ReceiptTip: "pushed"}
	must(t, store.Create(record))
	finalized := map[string]int{}
	failing := RecoverySeams{
		OriginCommit: func(unit Unit) (string, bool, error) { return "origin-" + unit.Chain, true, nil },
		OriginChange: func(Unit) (string, bool, error) { return "", false, errors.New("git log: unreadable") },
		Finalize:     func(unit Unit, _ string) error { finalized[unit.GoalID]++; return nil },
		Rearm:        func(string) error { return nil },
	}
	if err := RecoverPushedSeries(store, testBatchID, "owner", ten, failing); err == nil || !strings.Contains(err.Error(), "unreadable") {
		t.Fatalf("unreadable origin recovery=%v", err)
	}
	if load(t, store).State == StateLanded {
		t.Fatal("an unreadable origin landed the batch")
	}
	seams := RecoverySeams{
		OriginCommit: func(unit Unit) (string, bool, error) {
			if unit.IsChange() {
				t.Fatalf("a change was looked up by chain provenance")
			}
			return "origin-" + unit.Chain, true, nil
		},
		OriginChange: func(unit Unit) (string, bool, error) {
			return "origin-" + strings.TrimPrefix(unit.GoalID, "change:"), true, nil
		},
		Finalize: func(unit Unit, _ string) error { finalized[unit.GoalID]++; return nil },
		Rearm:    func(string) error { return nil },
		Cleanup:  func() error { return nil },
	}
	must(t, RecoverPushedSeries(store, testBatchID, "owner", ten, seams))
	must(t, RecoverPushedSeries(store, testBatchID, "owner", ten.Add(time.Minute), seams))
	landed := load(t, store)
	got := landed.Units[1]
	if landed.State != StateLanded || got.Outcome != UnitLanded || !got.P6Done || got.LandedCommit != "origin-abcdef012345" || finalized["goal-a"] != 1 || finalized[change.GoalID] != 0 {
		t.Fatalf("recovered state=%s change=%+v finalized=%v", landed.State, got, finalized)
	}
}

// TestHeldRefusalEjectsTheChangeAndTheGoalMembersLand (U11b): when held
// refuses the series at the push because of a change's replayed commit, the
// change is ejected with the refusal as its reason and the survivors reopen;
// the step is never tried again as it was, and the goal member then lands.
func TestHeldRefusalEjectsTheChangeAndTheGoalMembersLand(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	base := testCommit(101)
	store := NewStore(root, nil)
	strictReassembly(t, &store, expectedAssembly(base, []string{"goal-a"}, []string{"chain-a"}, []string{testCommit(103)}))
	record := openBatchWithGoalMember(base)
	change := changeMemberUnit()
	change.State = UnitJoined
	record.Units = append(record.Units, change)
	record.State, record.PrefixTrees, record.TipTree = StateLanding, []string{testCommit(103), testCommit(104)}, testCommit(104)
	record.Proof = &Proof{Status: "green", AttemptID: "tip"}
	record.Receipts = map[string]PrefixReceipt{"goal-a": {GoalID: "goal-a", Tree: testCommit(103), AttemptID: "prefix"}}
	record.History = append(record.History, HistoryEntry{At: ten.Format(time.RFC3339Nano), Verb: "prove", From: StateProving, To: StateLanding, Actor: "owner"})
	must(t, store.Create(record))
	var events []string
	seams := greenLandSeams(&events)
	seams.ReplayChange = func(unit Unit) (string, error) {
		events = append(events, "replay:"+unit.GoalID)
		return "replayed-change", nil
	}
	seams.Held = func(_, tip string) error {
		events = append(events, "held:"+tip)
		return &HeldCommitRefusal{Commit: "replayed-change", Cause: errors.New("held refused: goal-item-not-held: replayed-change: goal goal-g is released at abc")}
	}
	must(t, LandSeries(store, testBatchID, "owner", ten.Add(time.Minute), seams))
	after := load(t, store)
	got := after.Units[1]
	if after.State != StateOpen || after.Units[0].State != UnitJoined || got.State != UnitReturnPending || got.Outcome != UnitEjected ||
		!strings.Contains(got.Failure, "held refused change "+change.GoalID) || !strings.Contains(got.Failure, "goal-item-not-held") || !slices.Contains(events, "reset") {
		t.Fatalf("state=%s goal=%+v change=%+v events=%v", after.State, after.Units[0], got, events)
	}
	// The survivors prove again and land; nothing retries the refused step.
	must(t, store.Update(testBatchID, func(current *Record) error {
		current.Proof = &Proof{Status: "green", AttemptID: "tip-2"}
		current.Transition(StateLanding, ten.Add(2*time.Minute), "prove", "owner", "green")
		return nil
	}))
	events = nil
	seams.Held = func(string, string) error { events = append(events, "held"); return nil }
	must(t, LandSeries(store, testBatchID, "owner", ten.Add(3*time.Minute), seams))
	if want := []string{"apply:goal-a", "receipt:goal-a", "commit:goal-a", "held", "push", "cleanup"}; !slices.Equal(events, want) {
		t.Fatalf("survivor landing events=%v want %v", events, want)
	}
}

// TestChangeOnlyBatchLands (U11b): a batch of changes alone, proved on the
// lane's account, replays its change, pushes once and is recognized after the
// push by its Landing-Change trailer.
func TestChangeOnlyBatchLands(t *testing.T) {
	t.Parallel()
	store := NewStore(t.TempDir(), nil)
	change := changeMemberUnit()
	change.State = UnitJoined
	must(t, store.Create(Record{Schema: 1, BatchID: testBatchID, State: StateLanding, Units: []Unit{change},
		TipTree: testCommit(104), Proof: &Proof{Status: "green", AttemptID: "lane-attempt"},
		History:           []HistoryEntry{{At: ten.Format(time.RFC3339Nano), Verb: "prove", From: StateProving, To: StateLanding, Actor: "owner"}},
		batchRecordFields: batchRecordFields{BaseTree: testCommit(101), PrefixTrees: []string{testCommit(104)}}}))
	var events []string
	seams := greenLandSeams(&events)
	seams.ReplayChange = func(unit Unit) (string, error) {
		events = append(events, "replay:"+unit.GoalID)
		return "replayed", nil
	}
	must(t, LandSeries(store, testBatchID, "owner", ten.Add(time.Minute), seams))
	if want := []string{"replay:change:abcdef012345", "held", "push", "cleanup"}; !slices.Equal(events, want) {
		t.Fatalf("events=%v", events)
	}
	must(t, RecoverPushedSeries(store, testBatchID, "owner", ten.Add(2*time.Minute), RecoverySeams{
		OriginChange: func(Unit) (string, bool, error) { return "on-main", true, nil },
		Finalize:     func(Unit, string) error { t.Fatal("a change finalized a goal"); return nil },
		Rearm:        func(string) error { return nil },
	}))
	if landed := load(t, store); landed.State != StateLanded || landed.Units[0].LandedCommit != "on-main" {
		t.Fatalf("landed=%+v", landed)
	}
}

// TestChangeOnlyFlakeIsOwnedByTheLaneRegistrar (U11b F-2, the coordinator's
// ruling): a known flake found on main while a batch of changes alone is
// diagnosed is owned by the person who registered the lane; when the
// registrar is not a person the flake is recorded pending, never promoted,
// and the batch proceeds on the classification instead of looping.
func TestChangeOnlyFlakeIsOwnedByTheLaneRegistrar(t *testing.T) {
	t.Parallel()
	for _, person := range []bool{true, false} {
		red := knownRed()
		store, ledger := flakeBedWith(t, func(record *Record) {
			change := changeMemberUnit()
			change.State = UnitJoined
			change.Closure = &adapter.Closure{Tree: "tip-tree", Changed: []string{"unrelated-notes"}}
			record.Units, record.PrefixTrees = []Unit{change}, []string{"tip-tree"}
		}, red)
		script := &flakeScript{base: []DiagnosticResult{{AttemptID: "base-red", Groups: []RedGroup{red}}, {AttemptID: "base-green",
			Evidence: []GroupEvidence{{ID: red.ID, Status: "passed", LogPath: "logs/base-green.log"}}}}, tip: passedAt("identity-"+red.ID, red)}
		seams := flakeSeams(script, ledger, map[string]string{red.ID: "class-attempt", "fake-other": "tip-attempt"})
		seams.LaneOwner = func() (string, bool) {
			if person {
				return "Wido", true
			}
			return "landing owner of /lane", false
		}
		if err := DiagnoseRed(store, testBatchID, "owner", []RedGroup{red}, "", flakeNow, seams); err != nil {
			t.Fatalf("person=%v: %v", person, err)
		}
		record := load(t, store)
		if person {
			if len(ledger.promotions) != 1 || ledger.promotions[0].Owner != "Wido" || ledger.promotions[0].OwnerMachine != "m1e" || record.State != StateLanding {
				t.Fatalf("promotions=%+v state=%s", ledger.promotions, record.State)
			}
			continue
		}
		if len(ledger.promotions) != 0 || len(ledger.pendings) == 0 || record.State == StateDiagnosing {
			t.Fatalf("not a person: promotions=%+v pendings=%d state=%s", ledger.promotions, len(ledger.pendings), record.State)
		}
	}
}

// TestHeldRefusalEjectsTheGoalMemberItNames (U11b N-9): a held refusal of a
// goal member's commit at the push ejects that member with the refusal as
// its reason and reopens the rest, instead of retrying the step every tick.
func TestHeldRefusalEjectsTheGoalMemberItNames(t *testing.T) {
	t.Parallel()
	bed := newLandingBed(t)
	store := NewStore(bed.root, nil)
	strictReassembly(t, &store, expectedAssembly(bed.base, []string{"goal-a"}, []string{"chain-a"}, []string{testCommit(103)}))
	must(t, store.Create(bed.record))
	var events []string
	seams := greenLandSeams(&events)
	seams.Held = func(string, string) error {
		return &HeldCommitRefusal{Commit: "commit-goal-b", Cause: errors.New("held refused: goal-item-not-held: commit-goal-b: goal goal-b is released")}
	}
	must(t, LandSeries(store, testBatchID, "owner", time.Unix(4, 0), seams))
	record := load(t, store)
	if record.State != StateOpen || record.Units[0].State != UnitJoined || record.Units[1].State != UnitReturnPending ||
		!strings.Contains(record.Units[1].Failure, "held refused") || !strings.Contains(record.Units[1].Failure, "goal-item-not-held") {
		t.Fatalf("record=%+v", record)
	}
}

func stackedChange(parent Unit) Unit {
	child := NewChangeUnit(ChangeMember{Commit: "c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0", Parent: parent.Change.Commit, AskedBy: "m1e+human", Subject: "record: more"},
		"/seats/m1e/metasystem", "m1e", "human", []string{"records/notes.md"}, nil)
	child.State = UnitJoined
	return child
}

// TestStackedChangeInAnotherBatchIsRefused (U11b B-1a): a change stacked on
// a live change of another batch is refused at the join, naming the parent
// and its batch; nothing is assembled or written.
func TestStackedChangeInAnotherBatchIsRefused(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	base := testCommit(101)
	store := NewStore(root, nil)
	strictReassembly(t, &store)
	parent := changeMemberUnit()
	parent.State = UnitJoined
	proving := Record{Schema: 1, BatchID: "01j5x00000000000000000ba71", State: StateProving, TipTree: testCommit(104),
		Units: []Unit{parent}, History: []HistoryEntry{{At: ten.Format(time.RFC3339Nano), Verb: "join", Detail: parent.GoalID + " joined"}},
		batchRecordFields: batchRecordFields{BaseTree: base, PrefixTrees: []string{testCommit(104)}}}
	must(t, store.Create(proving))
	child := stackedChange(parent)
	child.State = UnitJoining
	_, err := JoinChange(store, ChangeJoin{Unit: child, BaseTree: base, NewID: "01j5x00000000000000000ba72", Actor: "m1e+human", At: ten})
	if err == nil || !strings.Contains(err.Error(), "change "+child.GoalID+" is stacked on change "+parent.GoalID+", which is in batch 01j5x00000000000000000ba71 (proving); run the same command after "+parent.GoalID+" lands") {
		t.Fatalf("join=%v", err)
	}
	var stacked *StackedChangeRefusal
	if !errors.As(err, &stacked) {
		t.Fatalf("not typed: %v", err)
	}
	if records, _ := store.Records(); len(records) != 1 {
		t.Fatalf("records=%d", len(records))
	}
}

// TestParentChangeLeavingTakesItsChildren (U11b B-1b): when a change leaves
// its batch, every live change stacked on it leaves in the same step with
// the parent's reason; reassembly never keeps a child without its parent.
func TestParentChangeLeavingTakesItsChildren(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	base := testCommit(101)
	store := NewStore(root, nil)
	strictReassembly(t, &store, expectedAssembly(base, []string{"goal-a"}, []string{"chain-a"}, []string{testCommit(103)}))
	record := openBatchWithGoalMember(base)
	parent := changeMemberUnit()
	parent.State = UnitJoined
	child := stackedChange(parent)
	grandchild := NewChangeUnit(ChangeMember{Commit: "d0d0d0d0d0d0d0d0d0d0d0d0d0d0d0d0d0d0d0d0", Parent: child.Change.Commit, AskedBy: "m1e+human"}, "/s", "m1e", "human", nil, nil)
	grandchild.State = UnitJoined
	record.Units = append(record.Units, parent, child, grandchild)
	record.PrefixTrees = []string{testCommit(103), testCommit(104), testCommit(105), testCommit(106)}
	must(t, store.Create(record))
	must(t, ReassembleSurvivorsWithReturns(store, testBatchID, "owner", ten, []ReturnDecision{{GoalID: parent.GoalID, Outcome: UnitEjected, Reason: "EJECTED: TestNotes failed"}}))
	after := load(t, store)
	for _, unit := range after.Units[2:] {
		if unit.State != UnitReturnPending || unit.Outcome != UnitEjected || !strings.Contains(unit.Failure, "EJECTED: TestNotes failed") {
			t.Fatalf("stacked %s stayed or lost the reason: %+v", unit.GoalID, unit)
		}
	}
	if !strings.Contains(after.Units[2].Failure, "its parent change "+parent.GoalID+" left the batch") || after.Units[0].State != UnitJoined {
		t.Fatalf("child=%+v goal=%+v", after.Units[2], after.Units[0])
	}
}

// TestSeriesHeldRefusalHoldsTheBatch (U11b N-b): a held refusal about the
// series or the lane's configuration holds the batch with the refusal as its
// visible reason and ejects nobody.
func TestSeriesHeldRefusalHoldsTheBatch(t *testing.T) {
	t.Parallel()
	bed := newLandingBed(t)
	store := NewStore(bed.root, nil)
	strictReassembly(t, &store)
	must(t, store.Create(bed.record))
	seams := greenLandSeams(&[]string{})
	seams.Held = func(string, string) error {
		return &HeldSeriesRefusal{Cause: errors.New("held refused: endpoint-mismatch: c1: this landing pushes origin refs/heads/main; the goal ledger is transport refs/heads/goals")}
	}
	must(t, LandSeries(store, testBatchID, "owner", time.Unix(4, 0), seams))
	record := load(t, store)
	if record.State != StateLanding || record.Units[0].State != UnitJoined || record.Units[1].State != UnitJoined || !strings.Contains(HoldReason(record), "endpoint-mismatch") {
		t.Fatalf("state=%s hold=%q units=%+v", record.State, HoldReason(record), record.Units)
	}
}
