package goal

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// strandEntry journals an operation as a dead foreign owner left it.
func strandEntry(t *testing.T, root string, opid string, phase Phase, intent Intent) {
	t.Helper()
	strandEntryAt(t, root, opid, "mac-a", phase, intent)
}

// strandEntryAt strands an entry under an explicit machine — the
// cross-clone recovery legs need the entry's identity to be the
// OTHER clone's.
func strandEntryAt(t *testing.T, root, opid, machine string, phase Phase, intent Intent) {
	t.Helper()
	if intent.Verb == "claim" || intent.Verb == "open-claim" || intent.Verb == "steal" {
		if intent.Args == nil {
			intent.Args = map[string]string{}
		}
		if intent.Args["claimEpoch"] == "" {
			intent.Args["claimEpoch"] = "1"
		}
	}
	if _, err := CreateEntry(root, opid, machine, "lin-1", intent); err != nil {
		t.Fatal(err)
	}
	if phase == PhasePushed {
		if err := MarkPushed(root, opid, "sometip", 1, time.Now().Add(-time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	cmd := spawnForeignOwner(t, root, opid)
	_ = cmd.Process.Kill()
	_, _ = cmd.Process.Wait()
}

func TestRecoveryCompletesADeadOwnersOpen(t *testing.T) {
	_, a, _ := twoClones(t)
	seedLedger(t, a)
	// A dead owner's CREATED open: never pushed, fully rebuildable.
	opid := Opid("01J5X00000000000000000Q000", "mac-a", "lin-1")
	strandEntry(t, a, opid, PhaseCreated, Intent{
		Verb: "open", Targets: []string{"orphaned"},
		Args: map[string]string{"intent": "The dead owner's work.", "origin": "main", "next": "Continue.", "labels": "custody,recovery"},
	})
	// The pushed-block gate: a CREATED stranded entry does not block,
	// but recovery still completes it (dead owner + created =
	// complete, R10-M04).
	reports, err := Recover(endpointFor(a))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, rep := range reports {
		if rep.Opid == opid {
			found = true
			if rep.Action != ActionComplete || !strings.Contains(rep.Detail, "confirmed") {
				t.Fatalf("the dead owner's open completes: %+v", rep)
			}
		}
	}
	if !found {
		t.Fatalf("recovery visits the stranded entry: %+v", reports)
	}
	// The work LANDED under the ORIGINAL opid.
	p, err := Project(endpointFor(a), true, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	orphaned := p.Tree.Live["orphaned"]
	if orphaned == nil || orphaned.Intent != "The dead owner's work." || strings.Join(orphaned.Labels, ",") != "custody,recovery" {
		t.Fatal("the recovered open is on the canonical branch")
	}
	if orphaned.History[0].Opid != opid {
		t.Fatalf("the recovered history carries the ORIGINAL opid: %s", orphaned.History[0].Opid)
	}
	// The entry is terminal confirmed.
	entry, err := ReadEntry(a, opid)
	if err != nil || entry.Phase != PhaseTerminal || entry.Outcome != OutcomeConfirmed {
		t.Fatalf("the recovered entry confirms: %+v %v", entry, err)
	}
}

func TestRecoveryCompletesOpenClaimWithLabels(t *testing.T) {
	_, a, _ := twoClones(t)
	seedLedger(t, a)
	opid := Opid("01J5X00000000000000000Q005", "mac-a", "lin-1")
	strandEntry(t, a, opid, PhaseCreated, Intent{
		Verb: "open-claim", Targets: []string{"recovered-claim"},
		Args: mergeIntentArgs(map[string]string{
			"intent": "The dead owner's claimed work.", "origin": "main",
			"next": "Continue.", "labels": "custody,recovery",
		}, budgetIntentArgs(testBudget())),
	})
	if _, err := Recover(endpointFor(a)); err != nil {
		t.Fatal(err)
	}
	p, err := Project(endpointFor(a), true, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	f := p.Tree.Live["recovered-claim"]
	if f == nil || f.State != StateClaimed || f.Claimed == nil || strings.Join(f.Labels, ",") != "custody,recovery" {
		t.Fatalf("recovery carries the atomic claim and labels: %+v", f)
	}
}

func TestRecoveryUnblocksAStrandedPush(t *testing.T) {
	_, a, _ := twoClones(t)
	seedLedger(t, a)
	// A dead owner's PUSHED claim on a goal that exists: the block
	// clears through recovery, not through waiting.
	if res, err := Open(verbReq(a, "01J5X00000000000000000Q010", "mac-a"), "claimable", "Work.", "main", "Go."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", res, err)
	}
	opid := Opid("01J5X00000000000000000Q020", "mac-a", "lin-1")
	strandEntry(t, a, opid, PhasePushed, Intent{
		Verb: "claim", Targets: []string{"claimable"}, Args: budgetIntentArgs(testBudget()),
	})

	// The stranded push blocks ordinary mutations…
	_, err := Open(verbReq(a, "01J5X00000000000000000Q030", "mac-a"), "blocked-out", "Waits.", "main", "Go.")
	if err == nil || !strings.Contains(err.Error(), opid) {
		t.Fatalf("a pushed entry blocks by name: %v", err)
	}
	// …until recovery completes the dead owner's claim.
	reports, err := Recover(endpointFor(a))
	if err != nil {
		t.Fatal(err)
	}
	var clear bool
	for _, rep := range reports {
		if rep.Opid == opid && rep.Action == ActionComplete {
			clear = true
		}
	}
	if !clear {
		t.Fatalf("recovery completes the stranded push: %+v", reports)
	}
	p, err := Project(endpointFor(a), true, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	claimed := p.Tree.Live["claimable"]
	if claimed == nil || claimed.State != StateClaimed || claimed.Claimed.Machine != "mac-a" {
		t.Fatalf("the recovered claim landed: %+v", claimed)
	}
	// The clone mutates again.
	if res, err := Open(verbReq(a, "01J5X00000000000000000Q040", "mac-a"), "unblocked", "Flows.", "main", "Go."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("the clone mutates after recovery: %+v %v", res, err)
	}
}

func TestRecoveryClosesUnrebuildableVerbsByName(t *testing.T) {
	_, a, _ := twoClones(t)
	seedLedger(t, a)
	opid := Opid("01J5X00000000000000000Q050", "mac-a", "lin-1")
	strandEntry(t, a, opid, PhaseCreated, Intent{Verb: "reconcile", Targets: []string{"x"},
		Args: map[string]string{"by": "wido", "rows": "1"}})
	reports, err := Recover(endpointFor(a))
	if err != nil {
		t.Fatal(err)
	}
	for _, rep := range reports {
		if rep.Opid == opid {
			if !strings.Contains(rep.Detail, "not rebuildable") {
				t.Fatalf("the unrebuildable verb closes by name: %+v", rep)
			}
		}
	}
	entry, err := ReadEntry(a, opid)
	if err != nil || entry.Phase != PhaseTerminal || entry.Outcome != OutcomeRejected {
		t.Fatalf("the entry closes rejected, never wedges: %+v %v", entry, err)
	}
}

func TestRecoveryRebuildsParkAndEdit(t *testing.T) {
	_, a, _ := twoClones(t)
	seedLedger(t, a)
	if res, err := Open(verbReq(a, "01J5X00000000000000000Q100", "mac-a"), "target", "Work.", "main", "Go."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", res, err)
	}
	// A dead owner's park, with its reason in the stored intent.
	parkOpid := Opid("01J5X00000000000000000Q110", "mac-a", "lin-1")
	strandEntry(t, a, parkOpid, PhaseCreated, Intent{
		Verb: "park", Targets: []string{"target"},
		Args: map[string]string{"because": "recovered pause", "by": "wido"},
	})
	if _, err := Recover(endpointFor(a)); err != nil {
		t.Fatal(err)
	}
	p, err := Project(endpointFor(a), true, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	parked := p.Tree.Live["target"]
	if parked.State != StateParked || parked.Parked.Because != "recovered pause" {
		t.Fatalf("the recovered park carries its reason: %+v", parked.Parked)
	}

	// A dead owner's edit, deltas in the stored intent feeding recovery.
	// The park above was a HUMAN act, so lifting it is one too — the
	// rebuild runs the REAL verb, which enforces exactly that.
	// The by= arg here is what a live human-directed unpark
	// journals itself (intentArgs stamps it), so
	// this stranded shape is the real crash shape, not a fabrication.
	unparkOpid := Opid("01J5X00000000000000000Q120", "mac-a", "lin-1")
	strandEntry(t, a, unparkOpid, PhaseCreated, Intent{Verb: "unpark", Targets: []string{"target"},
		Args: map[string]string{"by": "wido"}})
	if _, err := Recover(endpointFor(a)); err != nil {
		t.Fatal(err)
	}
	editOpid := Opid("01J5X00000000000000000Q130", "mac-a", "lin-1")
	strandEntry(t, a, editOpid, PhaseCreated, Intent{
		Verb: "edit", Targets: []string{"target"},
		Deltas: []FieldDelta{
			{Target: "target", Field: "next", New: "Recovered next step."},
			{Target: "target", Field: "blockedBy", New: ""},
		},
	})
	if _, err := Recover(endpointFor(a)); err != nil {
		t.Fatal(err)
	}
	p, err = Project(endpointFor(a), true, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	edited := p.Tree.Live["target"]
	if edited.State != StateQueued || edited.NextStep != "Recovered next step." {
		t.Fatalf("the recovered unpark and edit landed in order: %+v", edited)
	}
	carries := func(f *GoalFile, opid string) bool {
		for _, h := range f.History {
			if h.Opid == opid {
				return true
			}
		}
		return false
	}
	if !carries(edited, editOpid) || !carries(edited, unparkOpid) {
		t.Fatal("each recovery carries its original opid")
	}
}

func TestRecoveryConfirmsASightedPushWithoutRebuilding(t *testing.T) {
	_, a, _ := twoClones(t)
	seedLedger(t, a)
	// The operation LANDED (its opid is canonical) but the owner
	// died between the push and the confirm: recovery confirms on
	// sight — no rebuild, no second commit.
	res, err := Open(verbReq(a, "01J5X00000000000000000Q200", "mac-a"), "landed", "Landed work.", "main", "Go.")
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", res, err)
	}
	opid := Opid("01J5X00000000000000000Q200", "mac-a", "lin-1")
	// Rewind the entry's belief to pushed with a dead foreign owner.
	entry, err := ReadEntry(a, opid)
	if err != nil {
		t.Fatal(err)
	}
	entry.Phase = PhasePushed
	entry.Outcome = ""
	entry.TerminalAt = ""
	if err := writeEntry(a, entry); err != nil {
		t.Fatal(err)
	}
	cmd := spawnForeignOwner(t, a, opid)
	_ = cmd.Process.Kill()
	_, _ = cmd.Process.Wait()

	tipBefore := mustGit(t, a, "ls-remote", "origin", "refs/heads/main")
	reports, err := Recover(endpointFor(a))
	if err != nil {
		t.Fatal(err)
	}
	confirmed := false
	for _, rep := range reports {
		if rep.Opid == opid && rep.Action == ActionConfirm {
			confirmed = true
		}
	}
	if !confirmed {
		t.Fatalf("a sighted push confirms without rebuilding: %+v", reports)
	}
	if tipAfter := mustGit(t, a, "ls-remote", "origin", "refs/heads/main"); tipAfter != tipBefore {
		t.Fatal("confirmation on sight moves nothing on the canonical branch")
	}
}

func TestRecoveryHandlesOwnEntriesAndDoneRebuild(t *testing.T) {
	_, a, _ := twoClones(t)
	seedLedger(t, a)
	// The test process OWNS these entries: recovery abandons its own
	// never-pushed work and expires its own overdue push.
	ownCreated := Opid("01J5X00000000000000000Q300", "mac-a", "lin-1")
	if _, err := CreateEntry(a, ownCreated, "mac-a", "lin-1", testIntentFor("claim")); err != nil {
		t.Fatal(err)
	}
	ownPushed := Opid("01J5X00000000000000000Q310", "mac-a", "lin-1")
	if _, err := CreateEntry(a, ownPushed, "mac-a", "lin-1", testIntentFor("claim")); err != nil {
		t.Fatal(err)
	}
	if err := MarkPushed(a, ownPushed, "sometip", 1, time.Now().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	reports, err := Recover(endpointFor(a))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]RecoveryAction{}
	for _, rep := range reports {
		got[rep.Opid] = rep.Action
	}
	if got[ownCreated] != ActionAbandonOwn {
		t.Fatalf("the owner abandons its own never-pushed work: %v", got[ownCreated])
	}
	if got[ownPushed] != ActionExpireOwn {
		t.Fatalf("the owner expires its own overdue push: %v", got[ownPushed])
	}

	// A dead owner's DONE rebuild, conclusion in the stored intent.
	if res, err := Open(verbReq(a, "01J5X00000000000000000Q320", "mac-a"), "finish-me", "Work.", "main", "Go."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", res, err)
	}
	doneOpid := Opid("01J5X00000000000000000Q330", "mac-a", "lin-1")
	strandEntry(t, a, doneOpid, PhaseCreated, Intent{
		Verb: "done", Targets: []string{"finish-me"},
		Args: map[string]string{"conclusion": "Recovered conclusion."},
	})
	if _, err := Recover(endpointFor(a)); err != nil {
		t.Fatal(err)
	}
	p, err := Project(endpointFor(a), true, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	archived := p.Tree.Done["finish-me"]
	if archived == nil || archived.Conclude != "Recovered conclusion." {
		t.Fatalf("the recovered done archived with its conclusion: %+v", archived)
	}
}

func TestRecoveryRunsTheRealVerbSemanticsAcrossAnArc(t *testing.T) {
	_, a, b := twoClones(t)
	seedLedger(t, a)
	for i, id := range []string{"rv-one", "rv-two"} {
		ulid := fmt.Sprintf("01J5X00000000000000000Q%d40", i)
		if res, err := Open(verbReq(a, ulid, "mac-a"), id, "Arc "+id, "main", "Go."); err != nil || res.Outcome != OutcomeConfirmed {
			t.Fatalf("open %s: %+v %v", id, res, err)
		}
		if res, err := SetArc(verbReq(a, ulid[:len(ulid)-1]+"5", "mac-a"), id, "rv-arc"); err != nil || res.Outcome != OutcomeConfirmed {
			t.Fatalf("set-arc %s: %+v %v", id, res, err)
		}
	}
	// A dead owner's CASCADE claim: the rebuilt transaction must
	// claim BOTH members — a hand-copy would split the arc.
	claimOpid := Opid("01J5X00000000000000000Q160", "mac-a", "lin-1")
	strandEntry(t, a, claimOpid, PhaseCreated, Intent{
		Verb: "claim", Targets: []string{"rv-one"},
		Args: mergeIntentArgs(map[string]string{"cascade": "arc"}, budgetIntentArgs(testBudget())),
	})
	if _, err := Recover(endpointFor(a)); err != nil {
		t.Fatal(err)
	}
	p, err := Project(endpointFor(a), true, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"rv-one", "rv-two"} {
		f := p.Tree.Live[id]
		if f.State != StateClaimed || f.Claimed == nil || f.Claimed.Machine != "mac-a" {
			t.Fatalf("the recovered claim cascades across the arc: %s %+v", id, f.Claimed)
		}
		if f.History[len(f.History)-1].Opid != claimOpid {
			t.Fatalf("the cascade carries the ENTRY's opid: %s", f.History[len(f.History)-1].Opid)
		}
	}

	// A dead owner's STEAL from the other machine cannot replay. The
	// journal's by string is intent evidence, not human authority.
	stealOpid := Opid("01J5X00000000000000000Q170", "mac-b", "lin-1")
	strandEntryAt(t, b, stealOpid, "mac-b", PhaseCreated, Intent{
		Verb: "steal", Targets: []string{"rv-one"},
		Args: map[string]string{"by": "wido"},
	})
	reports, err := Recover(endpointFor(b))
	if err != nil {
		t.Fatal(err)
	}
	p, err = Project(endpointFor(b), true, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	stolen := p.Tree.Live["rv-one"]
	if stolen.Claimed == nil || stolen.Claimed.Machine != "mac-a" {
		t.Fatalf("journal text reassigned a human-reserved claim: %+v", stolen.Claimed)
	}
	entry, err := ReadEntry(b, stealOpid)
	if err != nil || entry.Outcome != OutcomeRejected {
		t.Fatalf("the unauthorized steal journal did not close rejected: entry=%+v err=%v reports=%+v", entry, err, reports)
	}
}

func TestRecoveryRefusesAStrandedOriginRewrite(t *testing.T) {
	_, a, _ := twoClones(t)
	seedLedger(t, a)
	if res, err := Open(verbReq(a, "01J5X00000000000000000Q200", "mac-a"), "prov", "Work.", "main", "Go."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", res, err)
	}
	// A pre-fold journal's origin delta: recovery must refuse it by
	// name, never replay it — origin is immutable through recovery
	// exactly as it is through the live verbs.
	opid := Opid("01J5X00000000000000000Q210", "mac-a", "lin-1")
	strandEntry(t, a, opid, PhaseCreated, Intent{
		Verb: "edit", Targets: []string{"prov"},
		Deltas: []FieldDelta{{Target: "prov", Field: "origin", New: "main"}},
	})
	reports, err := Recover(endpointFor(a))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, rep := range reports {
		if rep.Opid == opid {
			found = true
			if !strings.Contains(rep.Detail, "Origin") || !strings.Contains(rep.Detail, "immutable") {
				t.Fatalf("the origin rewrite refuses by name: %+v", rep)
			}
		}
	}
	if !found {
		t.Fatalf("recovery visits the stranded origin rewrite: %+v", reports)
	}
}

func TestRecoveryNeverPromotesAHumanStringFromJournaledIntent(t *testing.T) {
	_, a, _ := twoClones(t)
	seedLedger(t, a)
	if res, err := Open(verbReq(a, "01J5X00000000000000000Q300", "mac-a"), "hand-held", "Work.", "main", "Go."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", res, err)
	}
	parkReq := verbReq(a, "01J5X00000000000000000Q310", "mac-a")
	parkReq.Actor.Human = "wido"
	if res, err := Park(parkReq, "hand-held", "waiting on review"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("park: %+v %v", res, err)
	}
	// The stranded intent is exactly what the live verb journals. The
	// by field is deliberately retained in it, but recovery must not
	// convert that string into Actor.Human.
	editReq := verbReq(a, "01J5X00000000000000000Q320", "mac-a")
	editReq.Actor.Human = "wido"
	next := "Recovered by the human's own hand."
	labels := []string{"recovered", "custody"}
	liveReq, err := editRequest(editReq, "hand-held", EditFields{NextStep: &next, Labels: &labels})
	if err != nil {
		t.Fatal(err)
	}
	strandEntryAt(t, a, liveReq.Opid, "mac-a", PhaseCreated, liveReq.Intent)
	if _, err := Recover(endpointFor(a)); err != nil {
		t.Fatal(err)
	}
	p, err := Project(endpointFor(a), true, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	f := p.Tree.Live["hand-held"]
	if f == nil || f.NextStep == next || len(f.Labels) != 0 {
		t.Fatalf("journal text authorized a human-only edit: %+v", f)
	}
	entry, err := ReadEntry(a, liveReq.Opid)
	if err != nil || entry.Outcome != OutcomeRejected {
		t.Fatalf("the human-shaped journal did not close rejected: entry=%+v err=%v", entry, err)
	}
}
