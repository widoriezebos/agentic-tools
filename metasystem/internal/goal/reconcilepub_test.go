package goal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func humanReconcileReq(root, ulid string) VerbRequest {
	return VerbRequest{
		Endpoint: endpointFor(root),
		Actor:    Actor{Machine: "mac-a", Lineage: "lin-1", Human: "wido"},
		Ulid:     ulid,
		Now:      time.Date(2026, 8, 21, 1, 0, 0, 0, time.UTC),
	}
}

func TestReconcilePublishesHandEditsUnderTheHuman(t *testing.T) {
	a, _ := reconcileBed(t)
	editFile(t, a, goalsPrefix+"editable.md", func(f *GoalFile) {
		f.Intent = "Reconciled intent."
		f.NextStep = "Reconciled next."
	})
	res, err := Reconcile(humanReconcileReq(a, "01J5X00000000000000000P000"))
	if err != nil {
		t.Fatal(err)
	}
	if res.Publish.Outcome != OutcomeConfirmed || len(res.Rows) != 1 || res.Rows[0].Verb != "edit" {
		t.Fatalf("one edit row publishes: %+v", res)
	}
	tree, err := loadTree(a, res.Publish.Tip)
	if err != nil {
		t.Fatal(err)
	}
	f := tree.Live["editable"]
	if f.Intent != "Reconciled intent." {
		t.Fatalf("the hand edit landed: %+v", f)
	}
	last := f.History[len(f.History)-1]
	if last.Actor != "human:wido" || last.Verb != "edit" {
		t.Fatalf("the history attributes the human (all actor H): %+v", last)
	}
	// The refresh rematerialized: the checkout file carries the
	// SYNTHESIZED revision and history, and the base advanced.
	onDisk, err := os.ReadFile(filepath.Join(a, "plans", "goals", "editable.md"))
	if err != nil {
		t.Fatal(err)
	}
	parsed, problems := ParseFile(onDisk)
	if len(problems) > 0 || parsed.Revision != f.Revision {
		t.Fatalf("the checkout carries the published synthesis: %v %+v", problems, parsed)
	}
	rec, exists, _ := ReadBase(a)
	if !exists || rec.Commit != res.Publish.Commit || rec.RefreshDue {
		t.Fatalf("the base advanced to the published commit: %+v", rec)
	}
	// A second reconcile with no edits maps zero rows and publishes
	// nothing — consecutive sessions without a pull work from the
	// advanced base.
	res2, err := Reconcile(humanReconcileReq(a, "01J5X00000000000000000P010"))
	if err != nil {
		t.Fatal(err)
	}
	if len(res2.Rows) != 0 || res2.Publish.Outcome != "" {
		t.Fatalf("a clean checkout reconciles to nothing: %+v", res2)
	}
}

func TestReconcileObservesRawLabelsAndPublishesCanonicalLabels(t *testing.T) {
	a, _ := reconcileBed(t)
	editFile(t, a, goalsPrefix+"editable.md", func(f *GoalFile) {
		f.Labels = []string{"zeta", "alpha", "zeta"}
	})
	onDisk, err := os.ReadFile(filepath.Join(a, "plans", "goals", "editable.md"))
	if err != nil {
		t.Fatal(err)
	}
	parsed, problems := ParseFile(onDisk)
	if len(problems) != 0 || strings.Join(parsed.Labels, ",") != "zeta,alpha,zeta" {
		t.Fatalf("parse preserves the raw hand edit before reconcile: labels=%v problems=%v", parsed.Labels, problems)
	}

	res, err := Reconcile(humanReconcileReq(a, "01J5X00000000000000000P020"))
	if err != nil || res.Publish.Outcome != OutcomeConfirmed {
		t.Fatalf("reconcile labels: %+v %v", res, err)
	}
	tree, err := loadTree(a, res.Publish.Tip)
	if err != nil {
		t.Fatal(err)
	}
	got := tree.Live["editable"]
	if labels := strings.Join(got.Labels, ","); labels != "alpha,zeta" {
		t.Fatalf("reconcile republishes the canonical label field: %q", labels)
	}
	last := got.History[len(got.History)-1]
	if last.Verb != "edit" || last.Actor != "human:wido" {
		t.Fatalf("the existing edit history grammar records the reconciliation: %+v", last)
	}
}

func TestReconcileConflictNamesGoalAndField(t *testing.T) {
	a, tip := reconcileBed(t)
	_ = tip
	// The hand edit parks; a competitor concludes the goal on the
	// canonical branch first — the row's before-predicate fails on
	// the FETCHED tip and refuses as a named conflict.
	editFile(t, a, goalsPrefix+"editable.md", func(f *GoalFile) {
		f.State = StateParked
		f.Parked = &ParkRecord{By: "human:wido", At: "2026-08-21T01:00:00Z", Because: "pausing"}
	})
	if res, err := Done(verbReq(a, "01J5X00000000000000000P020", "mac-a"), "editable", "Concluded before the reconcile."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("competitor done: %+v %v", res, err)
	}
	res, err := Reconcile(humanReconcileReq(a, "01J5X00000000000000000P030"))
	if err != nil || res.Publish.Outcome != OutcomeRejected {
		t.Fatalf("the conflict rejects: %+v %v", res, err)
	}
	if !strings.Contains(res.Publish.Detail, "editable") || !strings.Contains(res.Publish.Detail, "state") {
		t.Fatalf("the conflict names the goal and the field: %s", res.Publish.Detail)
	}
}

func TestReconcileWithoutAHumanRefuses(t *testing.T) {
	a, _ := reconcileBed(t)
	req := humanReconcileReq(a, "01J5X00000000000000000P040")
	req.Actor.Human = ""
	if _, err := Reconcile(req); err == nil || !strings.Contains(err.Error(), "--by") {
		t.Fatalf("reconcile names its human: %v", err)
	}
}

func TestReconcileAppliesEveryRowKind(t *testing.T) {
	a, _ := reconcileBed(t)

	// Hand-park the existing goal AND hand-create a new one in the
	// same session: two rows, one commit, one opid.
	editFile(t, a, goalsPrefix+"editable.md", func(f *GoalFile) {
		f.State = StateParked
		f.Parked = &ParkRecord{By: "human:wido", At: "2026-08-21T01:30:00Z", Because: "hand pause"}
	})
	created := &GoalFile{Id: "hand-new", State: StateQueued, Intent: "Created by hand.", Origin: "human", NextStep: "Begin."}
	if err := os.WriteFile(filepath.Join(a, "plans", "goals", "hand-new.md"), RenderFile(created), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := Reconcile(humanReconcileReq(a, "01J5X00000000000000000P100"))
	if err != nil || res.Publish.Outcome != OutcomeConfirmed {
		t.Fatalf("two-row session publishes: %+v %v", res, err)
	}
	tree, err := loadTree(a, res.Publish.Tip)
	if err != nil {
		t.Fatal(err)
	}
	if tree.Live["editable"].State != StateParked || tree.Live["editable"].Parked.Because != "hand pause" {
		t.Fatalf("the park row applied: %+v", tree.Live["editable"])
	}
	if tree.Live["hand-new"] == nil || tree.Live["hand-new"].Intent != "Created by hand." {
		t.Fatalf("the open row applied: %+v", tree.Live["hand-new"])
	}
	oneOpid := tree.Live["editable"].History[len(tree.Live["editable"].History)-1].Opid
	if tree.Live["hand-new"].History[0].Opid != oneOpid {
		t.Fatal("one opid across the session's whole footprint")
	}

	// Hand-unpark, then hand-done, each through its own session.
	editFile(t, a, goalsPrefix+"editable.md", func(f *GoalFile) {
		f.State = StateQueued
		f.Parked = nil
	})
	res, err = Reconcile(humanReconcileReq(a, "01J5X00000000000000000P110"))
	if err != nil || res.Publish.Outcome != OutcomeConfirmed || res.Rows[0].Verb != "unpark" {
		t.Fatalf("the unpark row applies: %+v %v", res, err)
	}
	published, _ := ReadCommitGoals(a, res.Publish.Commit)
	raw := strings.Replace(string(published[goalsPrefix+"editable.md"]), "- State: queued", "- State: done\n- Concluded: Concluded by hand.", 1)
	if err := os.WriteFile(filepath.Join(a, "plans", "goals", "editable.md"), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err = Reconcile(humanReconcileReq(a, "01J5X00000000000000000P120"))
	if err != nil || res.Publish.Outcome != OutcomeConfirmed || res.Rows[0].Verb != "done" {
		t.Fatalf("the done row applies: %+v %v", res, err)
	}
	tree, err = loadTree(a, res.Publish.Tip)
	if err != nil {
		t.Fatal(err)
	}
	if tree.Done["editable"] == nil || tree.Done["editable"].Conclude != "Concluded by hand." {
		t.Fatalf("the hand-done archived: %+v", tree.Done["editable"])
	}
	// The checkout rematerialized: the live file is gone, the
	// archive file exists.
	if _, err := os.Stat(filepath.Join(a, "plans", "goals", "editable.md")); err == nil {
		t.Fatal("the concluded live file leaves the checkout on refresh")
	}
	if _, err := os.Stat(filepath.Join(a, filepath.FromSlash(recordsGoalsRoot), "editable.md")); err != nil {
		t.Fatal("the archive file materialized")
	}
}

func TestReconcileOpenConflictOnExistingGoal(t *testing.T) {
	a, _ := reconcileBed(t)
	// A hand-created file whose id LANDS on the canonical branch
	// before the reconcile: the open row's before-predicate fails on
	// the fetched tip.
	created := &GoalFile{Id: "collide", State: StateQueued, Intent: "Mine.", Origin: "human", NextStep: "Go."}
	if err := os.WriteFile(filepath.Join(a, "plans", "goals", "collide.md"), RenderFile(created), 0o644); err != nil {
		t.Fatal(err)
	}
	if res, err := Open(verbReq(a, "01J5X00000000000000000P130", "mac-a"), "collide", "Theirs first.", "main", "Go."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("competitor open: %+v %v", res, err)
	}
	res, err := Reconcile(humanReconcileReq(a, "01J5X00000000000000000P140"))
	if err != nil || res.Publish.Outcome != OutcomeRejected || !strings.Contains(res.Publish.Detail, "collide") {
		t.Fatalf("the open conflict rejects naming the goal: %+v %v", res, err)
	}
}

func TestConcurrentFieldEditConflictsInsteadOfOverwriting(t *testing.T) {
	a, _ := reconcileBed(t)
	// The hand edit changes intent A→B; a competitor lands A→C on
	// the canonical branch first. The replay must CONFLICT naming
	// the field — never overwrite C with B (F9).
	editFile(t, a, goalsPrefix+"editable.md", func(f *GoalFile) {
		f.Intent = "Hand version B."
	})
	competitor := "Competitor version C."
	if res, err := Edit(verbReq(a, "01J5X00000000000000000P200", "mac-a"), "editable", EditFields{Intent: &competitor}); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("competitor edit: %+v %v", res, err)
	}
	res, err := Reconcile(humanReconcileReq(a, "01J5X00000000000000000P210"))
	if err != nil || res.Publish.Outcome != OutcomeRejected {
		t.Fatalf("the concurrent edit conflicts: %+v %v", res, err)
	}
	if !strings.Contains(res.Publish.Detail, "intent") || !strings.Contains(res.Publish.Detail, "Competitor version C.") {
		t.Fatalf("the conflict names the field and the fetched value: %s", res.Publish.Detail)
	}
	// The competitor's value survives untouched.
	p, err := Project(endpointFor(a), true, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if p.Tree.Live["editable"].Intent != competitor {
		t.Fatal("the fetched value is never overwritten")
	}
}

func TestHandArcMoveMapsToItsVerbs(t *testing.T) {
	a, _ := reconcileBed(t)
	// A hand-set arc maps to set-arc and replays with the base-arc
	// comparison (F11: arc IS on the closed surface).
	editFile(t, a, goalsPrefix+"editable.md", func(f *GoalFile) {
		f.Arc = "hand-formed-arc"
	})
	res, err := Reconcile(humanReconcileReq(a, "01J5X00000000000000000P220"))
	if err != nil || res.Publish.Outcome != OutcomeConfirmed || res.Rows[0].Verb != "set-arc" {
		t.Fatalf("the arc move maps and publishes: %+v %v", res, err)
	}
	p, err := Project(endpointFor(a), true, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if p.Tree.Live["editable"].Arc != "hand-formed-arc" {
		t.Fatalf("the arc landed: %+v", p.Tree.Live["editable"])
	}
	// A hand-cleared arc maps to detach.
	editFile(t, a, goalsPrefix+"editable.md", func(f *GoalFile) {
		f.Arc = ""
	})
	res, err = Reconcile(humanReconcileReq(a, "01J5X00000000000000000P230"))
	if err != nil || res.Publish.Outcome != OutcomeConfirmed || res.Rows[0].Verb != "detach" {
		t.Fatalf("the arc clear maps to detach: %+v %v", res, err)
	}
}

func TestHandOriginEditIsOutsideTheSurface(t *testing.T) {
	a, tip := reconcileBed(t)
	editFile(t, a, goalsPrefix+"editable.md", func(f *GoalFile) {
		f.Origin = "rewritten-provenance"
	})
	snap, err := CaptureSnapshot(a)
	if err != nil {
		t.Fatal(err)
	}
	_, err = MapDeltas(a, tip, snap)
	if err == nil || !strings.Contains(err.Error(), "Origin") {
		t.Fatalf("Origin is outside the closed edit surface: %v", err)
	}
}

// The reconcile cluster: state before-values bind, foreign
// claims hear every hand override, and the hand path speaks the
// same membership matrix as the verbs.

func TestHandParkOfAClaimedGoalDisplacesThePair(t *testing.T) {
	a, tip := reconcileBed(t)
	if res, err := Claim(verbReq(a, "01J5X00000000000000000RP00", "mac-b"), "editable", testBudget()); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("foreign claim: %+v %v", res, err)
	}
	adv, err := FetchAdvance(endpointFor(a))
	if err != nil {
		t.Fatal(err)
	}
	materialize(t, a, adv.Tip)
	_ = tip
	// The hand edit: State claimed -> parked with its because.
	editFile(t, a, goalsPrefix+"editable.md", func(f *GoalFile) {
		f.State = StateParked
		f.Claimed = nil
		f.Parked = &ParkRecord{By: "human:wido", At: "2026-08-21T09:00:00Z", Because: "operator hold"}
	})
	req := verbReq(a, "01J5X00000000000000000RP10", "mac-a")
	req.Actor.Human = "wido"
	res, err := Reconcile(req)
	if err != nil || res.Publish.Outcome != OutcomeConfirmed {
		t.Fatalf("the claimed hand-park is a lawful human act (R2-13): %+v %v", res.Publish, err)
	}
	tree, err := loadTree(a, res.Publish.Commit)
	if err != nil {
		t.Fatal(err)
	}
	parked := tree.Live["editable"]
	if parked.State != StateParked || !strings.HasPrefix(parked.Parked.Displaced, "mac-b+lin-1@") {
		t.Fatalf("the displaced pair is recorded in full (R2-12): %+v", parked.Parked)
	}
}

func TestHandParkAgainstQueuedConflictsWithALandedClaim(t *testing.T) {
	a, _ := reconcileBed(t)
	// The hand park is made against QUEUED...
	editFile(t, a, goalsPrefix+"editable.md", func(f *GoalFile) {
		f.State = StateParked
		f.Parked = &ParkRecord{By: "human:wido", At: "2026-08-21T09:00:00Z", Because: "pausing it"}
	})
	// ...and a claim lands meanwhile.
	if res, err := Claim(verbReq(a, "01J5X00000000000000000RQ00", "mac-b"), "editable", testBudget()); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("competing claim: %+v %v", res, err)
	}
	req := verbReq(a, "01J5X00000000000000000RQ10", "mac-a")
	req.Actor.Human = "wido"
	res, err := Reconcile(req)
	if err != nil || res.Publish.Outcome != OutcomeRejected || !strings.Contains(res.Publish.Detail, "made against queued") {
		t.Fatalf("the state before-value binds (R2-10): %+v %v", res.Publish, err)
	}
	// The competitor's claim survives.
	adv, err := FetchAdvance(endpointFor(a))
	if err != nil {
		t.Fatal(err)
	}
	tree, err := loadTree(a, adv.Tip)
	if err != nil {
		t.Fatal(err)
	}
	if tree.Live["editable"].State != StateClaimed {
		t.Fatalf("the landed claim is preserved: %+v", tree.Live["editable"])
	}
}

func TestHandJoinIntoForeignClaimedArcLandsQueued(t *testing.T) {
	a, _ := reconcileBed(t)
	// A foreign pair claims an arc; the hand moves a fresh queued
	// goal into it.
	if res, err := Open(verbReq(a, "01J5X00000000000000000RJ00", "mac-b"), "arc-seed", "Seed.", "main", "Go."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open seed: %+v %v", res, err)
	}
	if res, err := SetArc(verbReq(a, "01J5X00000000000000000RJ10", "mac-b"), "arc-seed", "held-arc"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("set-arc seed: %+v %v", res, err)
	}
	if res, err := Claim(verbReq(a, "01J5X00000000000000000RJ20", "mac-b"), "arc-seed", testBudget()); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("claim seed: %+v %v", res, err)
	}
	budgetReq := verbReq(a, "01J5X00000000000000000RJ25", "mac-a")
	budgetReq.Actor.Human = "wido"
	if res, err := SetBudget(budgetReq, "editable", testBudget()); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("budget queued join fixture: %+v %v", res, err)
	}
	adv, err := FetchAdvance(endpointFor(a))
	if err != nil {
		t.Fatal(err)
	}
	materialize(t, a, adv.Tip)
	// The hand edit: the QUEUED editable goal joins the claimed arc.
	editFile(t, a, goalsPrefix+"editable.md", func(f *GoalFile) { f.Arc = "held-arc" })
	req := verbReq(a, "01J5X00000000000000000RJ30", "mac-a")
	req.Actor.Human = "wido"
	res, err := Reconcile(req)
	if err != nil || res.Publish.Outcome != OutcomeConfirmed {
		t.Fatalf("the human join into a claimed arc is the matrix's own row (R2-13): %+v %v", res.Publish, err)
	}
	tree, err := loadTree(a, res.Publish.Commit)
	if err != nil {
		t.Fatal(err)
	}
	joined := tree.Live["editable"]
	if joined.State != StateQueued || joined.Claimed != nil {
		t.Fatalf("a foreign destination claim was inherited by the joiner: %+v", joined)
	}
	last := joined.History[len(joined.History)-1]
	if last.Displaced != "" {
		t.Fatalf("a queued join displaced a foreign claimant it did not move: %+v", last)
	}
}
