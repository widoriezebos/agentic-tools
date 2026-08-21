package goal

import (
	"strings"
	"testing"
	"time"
)

// strandEntry journals an operation as a dead foreign owner left it.
func strandEntry(t *testing.T, root string, opid string, phase Phase, intent Intent) {
	t.Helper()
	if _, err := CreateEntry(root, opid, "mac-a", "lin-1", intent); err != nil {
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
		Args: map[string]string{"intent": "The dead owner's work.", "origin": "main", "next": "Continue."},
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
	if orphaned == nil || orphaned.Intent != "The dead owner's work." {
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

func TestRecoveryUnblocksAStrandedPush(t *testing.T) {
	_, a, _ := twoClones(t)
	seedLedger(t, a)
	// A dead owner's PUSHED claim on a goal that exists: the block
	// clears through recovery, not through waiting.
	if res, err := Open(verbReq(a, "01J5X00000000000000000Q010", "mac-a"), "claimable", "Work.", "main", "Go."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", res, err)
	}
	opid := Opid("01J5X00000000000000000Q020", "mac-a", "lin-1")
	strandEntry(t, a, opid, PhasePushed, Intent{Verb: "claim", Targets: []string{"claimable"}})

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

	// A dead owner's edit, deltas in the stored intent (the F7
	// completeness fix feeding recovery).
	unparkOpid := Opid("01J5X00000000000000000Q120", "mac-a", "lin-1")
	strandEntry(t, a, unparkOpid, PhaseCreated, Intent{Verb: "unpark", Targets: []string{"target"}})
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
	if !hasOpid(edited, editOpid) || !hasOpid(edited, unparkOpid) {
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
