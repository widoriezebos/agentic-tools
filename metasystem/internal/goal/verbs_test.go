package goal

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func testBudget() Budget {
	return Budget{ElapsedLimit: "4h", AttemptLimit: 4, ReservedJobMinutesLimit: 240, ActiveJobLimit: 2}
}

func verbReq(root, ulid, machine string) VerbRequest {
	return VerbRequest{
		Endpoint: endpointFor(root),
		Actor:    Actor{Machine: machine, Lineage: "lin-1"},
		Ulid:     ulid,
		Now:      time.Date(2026, 8, 20, 22, 0, 0, 0, time.UTC),
	}
}

// seedLedger publishes a root record so the verbs act on a lawful
// tree (migration owns real bootstrap; fixtures seed directly).
func seedLedger(t *testing.T, root string) {
	t.Helper()
	files := vTree(vRoot(), nil, nil)
	res, err := Publish(endpointFor(root), PublishRequest{
		Opid: "op-seed-" + root[len(root)-6:], Machine: "mac-seed", Lineage: "l1",
		Intent: testIntentFor("migrate"), Message: "seed root record",
		Mutate: func(tip string) ([]Change, error) {
			return []Change{{Path: goalsPrefix + "backlog.md", Content: files[goalsPrefix+"backlog.md"]}}, nil
		},
	})
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("seed: %+v %v", res, err)
	}
}

func TestOpenClaimDoneLifecycle(t *testing.T) {
	_, a, _ := twoClones(t)
	seedLedger(t, a)

	res, err := Open(verbReq(a, "01J5X0000000000000000000D0", "mac-a"), "build-it", "Build the thing.", "main", "Start with the walls.")
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", res, err)
	}
	res, err = Claim(verbReq(a, "01J5X0000000000000000000D1", "mac-a"), "build-it", testBudget())
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("claim: %+v %v", res, err)
	}
	res, err = Done(verbReq(a, "01J5X0000000000000000000D2", "mac-a"), "build-it", "Built and verified.")
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("done: %+v %v", res, err)
	}

	// The archive carries the record with the full history; the live
	// set is empty; every touched write bumped Revision exactly once.
	t2, err := loadTree(a, res.Tip)
	if err != nil {
		t.Fatal(err)
	}
	if len(t2.Live) != 0 {
		t.Fatalf("done moves the file out of the live set: %v", sortedGoalIds(t2.Live))
	}
	archived, ok := t2.Done["build-it"]
	if !ok {
		t.Fatal("done lands in the archive")
	}
	if archived.Conclude != "Built and verified." || archived.Revision != 3 || len(archived.History) != 3 {
		t.Fatalf("the archived record carries the whole lawful history: rev=%d hist=%d conclude=%q",
			archived.Revision, len(archived.History), archived.Conclude)
	}
	verbs := []string{archived.History[0].Verb, archived.History[1].Verb, archived.History[2].Verb}
	if strings.Join(verbs, ",") != "open,claim,done" {
		t.Fatalf("history names the verbs in order: %v", verbs)
	}
}

func TestBudgetedClaimRevisionLaws(t *testing.T) {
	_, a := oneClone(t)
	seedLedger(t, a)

	t.Run("claim requires the complete budget and binds its revision", func(t *testing.T) {
		if res, err := Open(verbReq(a, "01J5X00000000000000000H100", "mac-a"), "budgeted", "Bounded work.", "main", "Start."); err != nil || res.Outcome != OutcomeConfirmed {
			t.Fatalf("open: %+v %v", res, err)
		}
		missing, err := Claim(verbReq(a, "01J5X00000000000000000H110", "mac-a"), "budgeted")
		if err != nil || missing.Outcome != OutcomeRejected || !strings.Contains(missing.Detail, "complete tuple") {
			t.Fatalf("claim without the tuple did not refuse by remedy: %+v %v", missing, err)
		}
		res, err := Claim(verbReq(a, "01J5X00000000000000000H120", "mac-a"), "budgeted", testBudget())
		if err != nil || res.Outcome != OutcomeConfirmed {
			t.Fatalf("budgeted claim: %+v %v", res, err)
		}
		tree, err := loadTree(a, res.Tip)
		if err != nil {
			t.Fatal(err)
		}
		f := tree.Live["budgeted"]
		if f.Budget == nil || f.Claimed == nil || f.Claimed.Revision != f.Revision || f.Claimed.Revision != 2 {
			t.Fatalf("claim did not bind the complete tuple to its revision: %+v", f)
		}
		if rendered := string(RenderFile(f)); !strings.Contains(rendered, "- Budget: elapsedLimit=4h attemptLimit=4 reservedJobMinutesLimit=240 activeJobLimit=2") ||
			!strings.Contains(rendered, " revision=2") {
			t.Fatalf("the human-readable record lacks budget binding:\n%s", rendered)
		}
	})

	t.Run("set-budget starts the new revision elapsed clock", func(t *testing.T) {
		claimReq := verbReq(a, "01J5X00000000000000000H200", "mac-b")
		res, err := OpenClaim(claimReq, "rebudget", "Bounded work.", "main", "Start.", testBudget())
		if err != nil || res.Outcome != OutcomeConfirmed {
			t.Fatalf("open-claim: %+v %v", res, err)
		}
		next, err := NewBudget("8h", 6, 360, 3)
		if err != nil {
			t.Fatal(err)
		}
		setReq := verbReq(a, "01J5X00000000000000000H210", "mac-b")
		setReq.Now = claimReq.Now.Add(30 * time.Minute)
		res, err = SetBudget(setReq, "rebudget", next)
		if err != nil || res.Outcome != OutcomeConfirmed {
			t.Fatalf("set-budget: %+v %v", res, err)
		}
		tree, err := loadTree(a, res.Tip)
		if err != nil {
			t.Fatal(err)
		}
		f := tree.Live["rebudget"]
		if f.Revision != 2 || f.Claimed.Revision != 2 || f.Claimed.At != setReq.stamp() || *f.Budget != next ||
			f.History[len(f.History)-1].Verb != "set-budget" {
			t.Fatalf("set-budget did not establish a fresh bound revision: %+v", f)
		}
	})
}

func TestLabelVerbWritesCanonicalWholeFields(t *testing.T) {
	_, a, _ := twoClones(t)
	seedLedger(t, a)

	res, err := Open(verbReq(a, "01J5X00000000000000000Q500", "mac-a"), "labeled", "Grouped work.", "main", "Go.", "zeta", "alpha", "zeta")
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open with labels: %+v %v", res, err)
	}
	tree, err := loadTree(a, res.Tip)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(tree.Live["labeled"].Labels, ","); got != "alpha,zeta" {
		t.Fatalf("open stores a sorted, deduplicated field: %q", got)
	}

	labels, err := ApplyLabelDelta(tree.Live["labeled"].Labels, []string{"beta", "alpha"}, []string{"zeta"})
	if err != nil {
		t.Fatal(err)
	}
	res, err = Edit(verbReq(a, "01J5X00000000000000000Q510", "mac-a"), "labeled", EditFields{Labels: &labels})
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("edit labels: %+v %v", res, err)
	}
	tree, err = loadTree(a, res.Tip)
	if err != nil {
		t.Fatal(err)
	}
	edited := tree.Live["labeled"]
	if got := strings.Join(edited.Labels, ","); got != "alpha,beta" {
		t.Fatalf("edit replaces the whole canonical field: %q", got)
	}
	if last := edited.History[len(edited.History)-1]; last.Verb != "edit" || len(last.Targets) != 1 {
		t.Fatalf("label changes use the existing field-agnostic edit history grammar: %+v", last)
	}

	unchanged, err := ApplyLabelDelta(edited.Labels, []string{"alpha"}, []string{"absent"})
	if err != nil {
		t.Fatal(err)
	}
	res, err = Edit(verbReq(a, "01J5X00000000000000000Q520", "mac-a"), "labeled", EditFields{Labels: &unchanged})
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("an equal final set follows the shipped edit behavior: %+v %v", res, err)
	}
	if _, err := ApplyLabelDelta(edited.Labels, []string{"alpha"}, []string{"alpha"}); err == nil || !strings.Contains(err.Error(), "both --label and --unlabel") {
		t.Fatalf("a contradictory edit refuses by name: %v", err)
	}
	firstFinal, err := ApplyLabelDelta(edited.Labels, []string{"first"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	lastFinal, err := ApplyLabelDelta(edited.Labels, []string{"last"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res, err = Edit(verbReq(a, "01J5X00000000000000000Q525", "mac-a"), "labeled", EditFields{Labels: &firstFinal}); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("first whole-field publisher: %+v %v", res, err)
	}
	if res, err = Edit(verbReq(a, "01J5X00000000000000000Q526", "mac-a"), "labeled", EditFields{Labels: &lastFinal}); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("last whole-field publisher: %+v %v", res, err)
	}
	tree, err = loadTree(a, res.Tip)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(tree.Live["labeled"].Labels, ","); got != "alpha,beta,last" {
		t.Fatalf("the last publisher replaces the whole field instead of set-merging: %q", got)
	}
	if _, err := Open(verbReq(a, "01J5X00000000000000000Q530", "mac-a"), "bad-label", "Bad.", "main", "Stop.", "Bad_Label"); err == nil || !strings.Contains(err.Error(), labelRe.String()) {
		t.Fatalf("open refuses the one grammar: %v", err)
	}
}

func TestOpenClaimCarriesLabels(t *testing.T) {
	_, a, _ := twoClones(t)
	seedLedger(t, a)
	res, err := OpenClaim(verbReq(a, "01J5X00000000000000000Q540", "mac-a"), "held-label", "Claimed at creation.", "main", "Go.", testBudget(), "custody")
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open --claim with labels: %+v %v", res, err)
	}
	tree, err := loadTree(a, res.Tip)
	if err != nil {
		t.Fatal(err)
	}
	if got := tree.Live["held-label"]; got.State != StateClaimed || strings.Join(got.Labels, ",") != "custody" {
		t.Fatalf("the atomic open and claim carries labels: %+v", got)
	}
}

func TestSameGoalClaimRaceOneWinnerNamed(t *testing.T) {
	_, a, b := twoClones(t)
	seedLedger(t, a)
	if res, err := Open(verbReq(a, "01J5X0000000000000000000D3", "mac-a"), "contested", "One goal, two machines.", "main", "Race."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", res, err)
	}

	// A claims first; B's claim rebuilds on the new tip, reads the
	// standing claim, and classifies the loss naming the winner.
	resA, err := Claim(verbReq(a, "01J5X0000000000000000000D4", "mac-a"), "contested", testBudget())
	if err != nil || resA.Outcome != OutcomeConfirmed {
		t.Fatalf("A claims: %+v %v", resA, err)
	}
	resB, err := Claim(verbReq(b, "01J5X0000000000000000000D5", "mac-b"), "contested", testBudget())
	if err != nil || resB.Outcome != OutcomeLost {
		t.Fatalf("B loses the claim race: %+v %v", resB, err)
	}
	if !strings.Contains(resB.Detail, "mac-a") {
		t.Fatalf("the loser names the winner's operation: %s", resB.Detail)
	}
}

func TestClaimRefusalsAreNamed(t *testing.T) {
	_, a, _ := twoClones(t)
	seedLedger(t, a)
	if res, err := Open(verbReq(a, "01J5X0000000000000000000D6", "mac-a"), "dep", "The blocker.", "main", "First."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open dep: %+v %v", res, err)
	}
	// A goal blocked by an open dependency refuses the claim by name.
	res, err := Open(verbReq(a, "01J5X0000000000000000000D7", "mac-a"), "eager", "Wants to run early.", "main", "Second.")
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open eager: %+v %v", res, err)
	}
	// Wire the edge by hand for the fixture (no verb writes this edge):
	// publish an updated file carrying BlockedBy.
	t2, err := loadTree(a, res.Tip)
	if err != nil {
		t.Fatal(err)
	}
	eager := t2.Live["eager"]
	eager.Blocked = []string{"dep"}
	eager.Revision++
	if resw, errw := Publish(endpointFor(a), PublishRequest{
		Opid: "op-wire-edge", Machine: "mac-a", Lineage: "l1",
		Intent: testIntentFor("edit"), Message: "wire edge",
		Mutate: func(tip string) ([]Change, error) {
			return []Change{{Path: livePath("eager"), Content: RenderFile(eager)}}, nil
		},
		Validate: func(commit string) error { return ValidateCommit(a, commit) },
	}); errw != nil || resw.Outcome != OutcomeConfirmed {
		t.Fatalf("wire edge: %+v %v", resw, errw)
	}

	// Refusals are REJECTED results, journaled by name — not Go
	// errors: the engine's contract for a definite rejection.
	res2, err := Claim(verbReq(a, "01J5X0000000000000000000D8", "mac-a"), "eager", testBudget())
	if err != nil || res2.Outcome != OutcomeRejected || !strings.Contains(res2.Detail, "blocked by dep") {
		t.Fatalf("claiming past an open blocker rejects by name: %+v %v", res2, err)
	}
	res3, err := Claim(verbReq(a, "01J5X0000000000000000000DE", "mac-a"), "ghost", testBudget())
	if err != nil || res3.Outcome != OutcomeRejected || !strings.Contains(res3.Detail, "ghost") {
		t.Fatalf("claiming a goal that does not exist rejects naming it: %+v %v", res3, err)
	}
}

func TestReleaseIsOwnerOrHuman(t *testing.T) {
	_, a, b := twoClones(t)
	seedLedger(t, a)
	if res, err := Open(verbReq(a, "01J5X0000000000000000000D9", "mac-a"), "held", "Held by A.", "main", "Work."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", res, err)
	}
	if res, err := Claim(verbReq(a, "01J5X0000000000000000000DA", "mac-a"), "held", testBudget()); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("claim: %+v %v", res, err)
	}
	// B's agent cannot release A's claim: a rejection by name.
	resF, err := Release(verbReq(b, "01J5X0000000000000000000DB", "mac-b"), "held")
	if err != nil || resF.Outcome != OutcomeRejected || !strings.Contains(resF.Detail, "human act") {
		t.Fatalf("a foreign release rejects as a human act: %+v %v", resF, err)
	}
	// B under a human can.
	humanReq := verbReq(b, "01J5X0000000000000000000DC", "mac-b")
	humanReq.Actor.Human = "wido"
	res, err := Release(humanReq, "held")
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("a human-directed foreign release proceeds: %+v %v", res, err)
	}
	t2, err := loadTree(b, res.Tip)
	if err != nil {
		t.Fatal(err)
	}
	released := t2.Live["held"]
	if released.State != StateQueued || released.Claimed != nil {
		t.Fatalf("the release returns the goal to the queue: %+v", released)
	}
	last := released.History[len(released.History)-1]
	if last.Actor != "human:wido" {
		t.Fatalf("the history attributes the HUMAN authority: %+v", last)
	}
}

func TestOpenClearsGoalFreeInTheSameCommit(t *testing.T) {
	_, a, _ := twoClones(t)
	// Seed a root record WITH a Goal-free declaration.
	root := vRoot()
	root.Free = &FreeRecord{Declared: "2026-08-20T11:00:00Z", Origin: "main", Digest: strings.Repeat("cd", 32)}
	files := vTree(root, nil, nil)
	if res, err := Publish(endpointFor(a), PublishRequest{
		Opid: "op-seed-free", Machine: "mac-seed", Lineage: "l1",
		Intent: testIntentFor("migrate"), Message: "seed free root",
		Mutate: func(tip string) ([]Change, error) {
			return []Change{{Path: goalsPrefix + "backlog.md", Content: files[goalsPrefix+"backlog.md"]}}, nil
		},
	}); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("seed: %+v %v", res, err)
	}

	res, err := Open(verbReq(a, "01J5X0000000000000000000DD", "mac-a"), "revival", "Work resumes.", "main", "Go.")
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open under Goal-free: %+v %v", res, err)
	}
	t2, err := loadTree(a, res.Tip)
	if err != nil {
		t.Fatal(err)
	}
	if t2.Root.Free != nil {
		t.Fatal("open clears the Goal-free declaration in the same commit")
	}
	if _, ok := t2.Live["revival"]; !ok {
		t.Fatal("the opened goal is live")
	}
}

func TestParkUnparkCycle(t *testing.T) {
	_, a, b := twoClones(t)
	seedLedger(t, a)
	if res, err := Open(verbReq(a, "01J5X0000000000000000000E0", "mac-a"), "pausable", "Pausable work.", "main", "Go."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", res, err)
	}
	// Park without a reason refuses up front.
	if _, err := Park(verbReq(a, "01J5X0000000000000000000E1", "mac-a"), "pausable", " "); err == nil {
		t.Fatal("park needs its reason")
	}
	res, err := Park(verbReq(a, "01J5X0000000000000000000E2", "mac-a"), "pausable", "waiting on the vendor")
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("park: %+v %v", res, err)
	}
	t2, err := loadTree(a, res.Tip)
	if err != nil {
		t.Fatal(err)
	}
	parked := t2.Live["pausable"]
	if parked.State != StateParked || parked.Parked == nil || parked.Parked.Because != "waiting on the vendor" {
		t.Fatalf("the park carries its reason: %+v", parked.Parked)
	}

	// An agent parking ANOTHER machine's claim rejects; a human
	// park records the displaced claimant.
	if res, err := Unpark(verbReq(a, "01J5X0000000000000000000E3", "mac-a"), "pausable"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("unpark: %+v %v", res, err)
	}
	if res, err := Claim(verbReq(a, "01J5X0000000000000000000E4", "mac-a"), "pausable", testBudget()); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("claim: %+v %v", res, err)
	}
	resB, err := Park(verbReq(b, "01J5X0000000000000000000E5", "mac-b"), "pausable", "b wants it stopped")
	if err != nil || resB.Outcome != OutcomeRejected || !strings.Contains(resB.Detail, "human act") {
		t.Fatalf("an agent parking another's claim rejects: %+v %v", resB, err)
	}
	humanReq := verbReq(b, "01J5X0000000000000000000E6", "mac-b")
	humanReq.Actor.Human = "wido"
	resH, err := Park(humanReq, "pausable", "operator stop")
	if err != nil || resH.Outcome != OutcomeConfirmed {
		t.Fatalf("the human park proceeds: %+v %v", resH, err)
	}
	t3, err := loadTree(b, resH.Tip)
	if err != nil {
		t.Fatal(err)
	}
	displaced := t3.Live["pausable"]
	if displaced.Parked == nil || !strings.HasPrefix(displaced.Parked.Displaced, "mac-a+lin-1@") {
		t.Fatalf("the displaced claimant is recorded: %+v", displaced.Parked)
	}
	last := displaced.History[len(displaced.History)-1]
	if last.Displaced == "" || last.Actor != "human:wido" {
		t.Fatalf("the history line carries displaced= under the human: %+v", last)
	}
}

func TestReopenGuardsClaimedDependents(t *testing.T) {
	_, a, _ := twoClones(t)
	seedLedger(t, a)
	if res, err := Open(verbReq(a, "01J5X0000000000000000000E7", "mac-a"), "base", "The base.", "main", "First."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open base: %+v %v", res, err)
	}
	if res, err := Done(verbReq(a, "01J5X0000000000000000000E8", "mac-a"), "base", "Base shipped."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("done base: %+v %v", res, err)
	}
	if res, err := Open(verbReq(a, "01J5X0000000000000000000E9", "mac-a"), "tower", "On the base.", "main", "Second."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open tower: %+v %v", res, err)
	}
	blocked := []string{"base"}
	if res, err := Edit(verbReq(a, "01J5X0000000000000000000EA", "mac-a"), "tower", EditFields{Blocked: &blocked}); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("edit edge: %+v %v", res, err)
	}
	if res, err := Claim(verbReq(a, "01J5X0000000000000000000EB", "mac-a"), "tower", testBudget()); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("claim tower: %+v %v", res, err)
	}
	// Reopening the base under the claimed dependent refuses.
	res, err := Reopen(verbReq(a, "01J5X0000000000000000000EC", "mac-a"), "base")
	if err != nil || res.Outcome != OutcomeRejected || !strings.Contains(res.Detail, "tower") {
		t.Fatalf("reopen under a claimed dependent rejects naming it: %+v %v", res, err)
	}
	// Release the dependent; the reopen proceeds and the archive
	// entry moves back queued.
	if res, err := Release(verbReq(a, "01J5X0000000000000000000ED", "mac-a"), "tower"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("release: %+v %v", res, err)
	}
	res, err = Reopen(verbReq(a, "01J5X0000000000000000000EE", "mac-a"), "base")
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("reopen: %+v %v", res, err)
	}
	t2, err := loadTree(a, res.Tip)
	if err != nil {
		t.Fatal(err)
	}
	if _, archived := t2.Done["base"]; archived {
		t.Fatal("reopen removes the archive entry")
	}
	back := t2.Live["base"]
	if back == nil || back.State != StateQueued || back.Conclude != "" {
		t.Fatalf("the reopened goal is queued without its old conclusion: %+v", back)
	}
}

func TestDeclareFreeExclusivityAndRenewal(t *testing.T) {
	_, a, _ := twoClones(t)
	seedLedger(t, a)
	if res, err := Open(verbReq(a, "01J5X0000000000000000000EF", "mac-a"), "open-one", "Work.", "main", "Go."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", res, err)
	}
	// Declaring over a queued goal rejects by name.
	res, err := DeclareFree(verbReq(a, "01J5X0000000000000000000F0", "mac-a"), "main", strings.Repeat("ef", 32))
	if err != nil || res.Outcome != OutcomeRejected || !strings.Contains(res.Detail, "open-one") {
		t.Fatalf("declare-free over queued rejects: %+v %v", res, err)
	}
	// Park it; the declaration coexists with parked.
	if res, err := Park(verbReq(a, "01J5X0000000000000000000F1", "mac-a"), "open-one", "later"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("park: %+v %v", res, err)
	}
	res, err = DeclareFree(verbReq(a, "01J5X0000000000000000000F2", "mac-a"), "main", strings.Repeat("ef", 32))
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("declare-free over parked: %+v %v", res, err)
	}
	// Renewal with the same digest is idempotent IN EFFECT: the
	// fresh operation finds the declaration standing and abandons —
	// never a confirmed entry whose opid is nowhere (F8: the opid
	// is the journal's truth).
	res, err = DeclareFree(verbReq(a, "01J5X0000000000000000000F3", "mac-a"), "main", strings.Repeat("ef", 32))
	if err != nil || res.Outcome != OutcomeAbandoned || !strings.Contains(res.Detail, "already stands") {
		t.Fatalf("a fresh renewal abandons honestly: %+v %v", res, err)
	}
}

func TestEditAcceptsAMultiKilobyteIntent(t *testing.T) {
	_, a, _ := twoClones(t)
	seedLedger(t, a)
	if res, err := Open(verbReq(a, "01J5X0000000000000000000F4", "mac-a"), "verbose", "Short.", "main", "Go."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", res, err)
	}
	// Prose caps are REMOVED: witnessed by a
	// multi-kilobyte intent, not a 500-byte one.
	big := strings.Repeat("A thorough paragraph of intent. ", 150) // ~4.8KB
	res, err := Edit(verbReq(a, "01J5X0000000000000000000F5", "mac-a"), "verbose", EditFields{Intent: &big})
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("a multi-kilobyte intent is lawful: %+v %v", res, err)
	}
	t2, err := loadTree(a, res.Tip)
	if err != nil {
		t.Fatal(err)
	}
	if len(t2.Live["verbose"].Intent) < 4000 {
		t.Fatalf("the intent round-trips whole: %d bytes", len(t2.Live["verbose"].Intent))
	}
}

func TestStealNeedsItsHumanAndRecordsIt(t *testing.T) {
	_, a, b := twoClones(t)
	seedLedger(t, a)
	if res, err := OpenClaim(verbReq(a, "01J5X0000000000000000000F6", "mac-a"), "wanted", "Wanted work.", "main", "Go.", testBudget()); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open --claim: %+v %v", res, err)
	}
	// Steal without a human refuses before anything happens.
	if _, err := Steal(verbReq(b, "01J5X0000000000000000000F7", "mac-b"), "wanted"); err == nil ||
		!strings.Contains(err.Error(), "--by") {
		t.Fatalf("steal without --by refuses: %v", err)
	}
	humanReq := verbReq(b, "01J5X0000000000000000000F8", "mac-b")
	humanReq.Actor.Human = "wido"
	res, err := Steal(humanReq, "wanted")
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("the attributed steal proceeds: %+v %v", res, err)
	}
	t2, err := loadTree(b, res.Tip)
	if err != nil {
		t.Fatal(err)
	}
	stolen := t2.Live["wanted"]
	if stolen.Claimed == nil || stolen.Claimed.Machine != "mac-b" {
		t.Fatalf("the claim moved to the stealing pair: %+v", stolen.Claimed)
	}
	last := stolen.History[len(stolen.History)-1]
	if last.Verb != "steal" || last.Actor != "human:wido" {
		t.Fatalf("the history line records the human authority (R7-08): %+v", last)
	}
}

func TestPruneKeepsTheClosureAndTheNewest(t *testing.T) {
	_, a, _ := twoClones(t)
	seedLedger(t, a)
	// Three archived goals of increasing age: ancient (chained under
	// a live blocker edge), middle, fresh. One live goal depends on
	// a done goal that itself depends on ancient — the done-to-done
	// chain the closure must follow.
	mk := func(ulid, id, opened string, blocked []string) {
		if res, err := Open(verbReq(a, ulid, "mac-a"), id, "Work "+id, "main", "Go."); err != nil || res.Outcome != OutcomeConfirmed {
			t.Fatalf("open %s: %+v %v", id, res, err)
		}
		if blocked != nil {
			if res, err := Edit(verbReq(a, ulid[:len(ulid)-1]+"E", "mac-a"), id, EditFields{Blocked: &blocked}); err != nil || res.Outcome != OutcomeConfirmed {
				t.Fatalf("edge %s: %+v %v", id, res, err)
			}
		}
		if res, err := Done(verbReq(a, ulid[:len(ulid)-1]+"D", "mac-a"), id, "Done "+id); err != nil || res.Outcome != OutcomeConfirmed {
			t.Fatalf("done %s: %+v %v", id, res, err)
		}
	}
	mk("01J5X00000000000000000A000", "ancient", "", nil)
	mk("01J5X00000000000000000A010", "middle", "", []string{"ancient"})
	mk("01J5X00000000000000000A020", "fresh", "", nil)
	// The live goal depends on middle: closure = middle + ancient.
	if res, err := Open(verbReq(a, "01J5X00000000000000000A030", "mac-a"), "alive", "Live work.", "main", "Go."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open alive: %+v %v", res, err)
	}
	edge := []string{"middle"}
	if res, err := Edit(verbReq(a, "01J5X00000000000000000A040", "mac-a"), "alive", EditFields{Blocked: &edge}); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("edge alive: %+v %v", res, err)
	}

	// keep=0: only the closure survives — fresh dies, ancient and
	// middle stay reachable, nothing dangles by construction.
	res, err := Prune(verbReq(a, "01J5X00000000000000000A050", "mac-a"), 0)
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("prune: %+v %v", res, err)
	}
	t2, err := loadTree(a, res.Tip)
	if err != nil {
		t.Fatal(err)
	}
	if _, kept := t2.Done["ancient"]; !kept {
		t.Fatal("the closure follows done-to-done edges: ancient stays")
	}
	if _, kept := t2.Done["middle"]; !kept {
		t.Fatal("the live blocker's target stays")
	}
	if _, gone := t2.Done["fresh"]; gone {
		t.Fatal("outside the closure with keep=0, fresh dies")
	}
	// The root record carries the opid line with the literal keep.
	last := t2.Root.History[len(t2.Root.History)-1]
	if last.Verb != "prune" || last.Keep != 0 {
		t.Fatalf("the root history carries prune keep=0: %+v", last)
	}
	// A prune replay is idempotent on the rebuilt tip.
	res2, err := Prune(verbReq(a, "01J5X00000000000000000A050", "mac-a"), 0)
	if err != nil || res2.Outcome != OutcomeConfirmed || res2.Detail != "idempotent" {
		t.Fatalf("the prune replay classifies idempotent: %+v %v", res2, err)
	}
}

func TestArcClaimsAsOneUnit(t *testing.T) {
	_, a, b := twoClones(t)
	seedLedger(t, a)
	// Two members of one arc, plus a bystander.
	for i, id := range []string{"arc-one", "arc-two", "solo"} {
		ulid := fmt.Sprintf("01J5X00000000000000000B0%d0", i)
		if res, err := Open(verbReq(a, ulid, "mac-a"), id, "Work "+id, "main", "Go."); err != nil || res.Outcome != OutcomeConfirmed {
			t.Fatalf("open %s: %+v %v", id, res, err)
		}
	}
	arc := "covenant-patience"
	for i, id := range []string{"arc-one", "arc-two"} {
		ulid := fmt.Sprintf("01J5X00000000000000000B1%d0", i)
		res, err := Publish(endpointFor(a), PublishRequest{
			Opid: Opid(ulid, "mac-a", "lin-1"), Machine: "mac-a", Lineage: "lin-1",
			Intent: testIntentFor("edit"), Message: "wire arc " + id,
			Mutate: func(tip string) ([]Change, error) {
				t2, err := loadTree(a, tip)
				if err != nil {
					return nil, err
				}
				f := t2.Live[id]
				f.Arc = arc
				touch(f, verbReq(a, ulid, "mac-a"), "edit", []string{id})
				return []Change{{Path: livePath(id), Content: RenderFile(f)}}, nil
			},
			Validate: func(commit string) error { return ValidateCommit(a, commit) },
		})
		if err != nil || res.Outcome != OutcomeConfirmed {
			t.Fatalf("wire arc %s: %+v %v", id, res, err)
		}
	}

	// The covenant-patience two-clone race: one winner takes BOTH
	// members; the loser names the winner.
	resA, err := ClaimArc(verbReq(a, "01J5X00000000000000000B200", "mac-a"), "arc-one", testBudget())
	if err != nil || resA.Outcome != OutcomeConfirmed {
		t.Fatalf("A claims the arc: %+v %v", resA, err)
	}
	resB, err := ClaimArc(verbReq(b, "01J5X00000000000000000B210", "mac-b"), "arc-two", testBudget())
	if err != nil || resB.Outcome != OutcomeLost {
		t.Fatalf("B loses the whole cascade: %+v %v", resB, err)
	}
	t2, err := loadTree(a, resA.Tip)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"arc-one", "arc-two"} {
		m := t2.Live[id]
		if m.State != StateClaimed || m.Claimed == nil || m.Claimed.Machine != "mac-a" {
			t.Fatalf("one claimant holds every member: %s %+v", id, m.Claimed)
		}
		last := m.History[len(m.History)-1]
		if len(last.Targets) != 2 {
			t.Fatalf("the cascade's history line names the whole set: %+v", last)
		}
	}
	if t2.Live["solo"].State != StateQueued {
		t.Fatal("the bystander is untouched")
	}
	// The quota holds: two claimed members, one arc, one machine —
	// the tree validates (arc counts once).
	if problems := ValidateTree(t2); len(problems) != 0 {
		t.Fatalf("one arc under one claimant counts once against the quota: %v", problems)
	}

	// Release cascades back as one unit.
	resR, err := ReleaseArc(verbReq(a, "01J5X00000000000000000B220", "mac-a"), "arc-one")
	if err != nil || resR.Outcome != OutcomeConfirmed {
		t.Fatalf("release cascade: %+v %v", resR, err)
	}
	t3, err := loadTree(a, resR.Tip)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"arc-one", "arc-two"} {
		if t3.Live[id].State != StateQueued || t3.Live[id].Claimed != nil {
			t.Fatalf("the release returns every member: %s %+v", id, t3.Live[id])
		}
	}
}

func TestMemberDoneLeavesSiblingClaimed(t *testing.T) {
	_, a, _ := twoClones(t)
	seedLedger(t, a)
	for i, id := range []string{"pair-one", "pair-two"} {
		ulid := fmt.Sprintf("01J5X00000000000000000B3%d0", i)
		if res, err := Open(verbReq(a, ulid, "mac-a"), id, "Paired "+id, "main", "Go."); err != nil || res.Outcome != OutcomeConfirmed {
			t.Fatalf("open %s: %+v %v", id, res, err)
		}
		wireULID := fmt.Sprintf("01J5X00000000000000000B4%d0", i)
		res, err := Publish(endpointFor(a), PublishRequest{
			Opid:    Opid(wireULID, "mac-a", "lin-1"),
			Machine: "mac-a", Lineage: "lin-1",
			Intent: testIntentFor("edit"), Message: "wire pair " + id,
			Mutate: func(tip string) ([]Change, error) {
				t2, err := loadTree(a, tip)
				if err != nil {
					return nil, err
				}
				f := t2.Live[id]
				f.Arc = "the-pair"
				touch(f, verbReq(a, wireULID, "mac-a"), "edit", []string{id})
				return []Change{{Path: livePath(id), Content: RenderFile(f)}}, nil
			},
			Validate: func(commit string) error { return ValidateCommit(a, commit) },
		})
		if err != nil || res.Outcome != OutcomeConfirmed {
			t.Fatalf("wire %s: %+v %v", id, res, err)
		}
	}
	if res, err := ClaimArc(verbReq(a, "01J5X00000000000000000B500", "mac-a"), "pair-one", testBudget()); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("claim arc: %+v %v", res, err)
	}
	// Concluding ONE member archives it and leaves the sibling
	// claimed — the arc survives.
	res, err := Done(verbReq(a, "01J5X00000000000000000B510", "mac-a"), "pair-one", "First member shipped.")
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("done member: %+v %v", res, err)
	}
	t2, err := loadTree(a, res.Tip)
	if err != nil {
		t.Fatal(err)
	}
	if _, archived := t2.Done["pair-one"]; !archived {
		t.Fatal("the concluded member is archived")
	}
	sibling := t2.Live["pair-two"]
	if sibling.State != StateClaimed || sibling.Claimed == nil || sibling.Claimed.Machine != "mac-a" {
		t.Fatalf("the sibling stays claimed; the arc survives: %+v", sibling)
	}
}

func TestParkCascadePinsOneAcknowledgment(t *testing.T) {
	_, a, b := twoClones(t)
	seedLedger(t, a)
	for i, id := range []string{"casc-one", "casc-two"} {
		if res, err := Open(verbReq(a, fmt.Sprintf("01J5X00000000000000000C0%d0", i), "mac-a"), id, "Cascade "+id, "main", "Go."); err != nil || res.Outcome != OutcomeConfirmed {
			t.Fatalf("open %s: %+v %v", id, res, err)
		}
		wireULID := fmt.Sprintf("01J5X00000000000000000C1%d0", i)
		res, err := Publish(endpointFor(a), PublishRequest{
			Opid:    Opid(wireULID, "mac-a", "lin-1"),
			Machine: "mac-a", Lineage: "lin-1",
			Intent: testIntentFor("edit"), Message: "wire " + id,
			Mutate: func(tip string) ([]Change, error) {
				t2, err := loadTree(a, tip)
				if err != nil {
					return nil, err
				}
				f := t2.Live[id]
				f.Arc = "the-cascade"
				touch(f, verbReq(a, wireULID, "mac-a"), "edit", []string{id})
				return []Change{{Path: livePath(id), Content: RenderFile(f)}}, nil
			},
			Validate: func(commit string) error { return ValidateCommit(a, commit) },
		})
		if err != nil || res.Outcome != OutcomeConfirmed {
			t.Fatalf("wire %s: %+v %v", id, res, err)
		}
	}
	if res, err := ClaimArc(verbReq(a, "01J5X00000000000000000C200", "mac-a"), "casc-one", testBudget()); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("claim arc: %+v %v", res, err)
	}
	// The human parks A's whole claimed arc from machine B: the
	// displaced pair is recorded ONCE and rides each touched line.
	humanReq := verbReq(b, "01J5X00000000000000000C210", "mac-b")
	humanReq.Actor.Human = "wido"
	res, err := ParkArc(humanReq, "casc-one", "operator hold")
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("human park cascade: %+v %v", res, err)
	}
	t2, err := loadTree(b, res.Tip)
	if err != nil {
		t.Fatal(err)
	}
	pairs := map[string]bool{}
	for _, id := range []string{"casc-one", "casc-two"} {
		m := t2.Live[id]
		if m.State != StateParked {
			t.Fatalf("the cascade parks every member: %s is %s", id, m.State)
		}
		if m.Parked.Displaced != "" {
			pairs[m.Parked.Displaced] = true
		}
	}
	if len(pairs) != 1 {
		t.Fatalf("ONE displaced pair across the cascade (R10-M06): %v", pairs)
	}
	// The displaced agent cannot lift the human's pause itself.
	res, err = UnparkArc(verbReq(a, "01J5X00000000000000000C215", "mac-a"), "casc-one")
	if err != nil || res.Outcome != OutcomeRejected || !strings.Contains(res.Detail, "human's pause") {
		t.Fatalf("an agent lifting a human's park refuses by name: %+v %v", res, err)
	}
	// The displaced pair's next History-appending publication
	// piggybacks ONE automatic root-record ack line answering the
	// displacement — both goals on the one line.
	res, err = Open(verbReq(a, "01J5X00000000000000000C218", "mac-a"), "casc-after", "Displaced pair keeps working.", "main", "Go.")
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("the displaced pair's next publication: %+v %v", res, err)
	}
	tAck, err := loadTree(a, res.Tip)
	if err != nil {
		t.Fatal(err)
	}
	var acks []HistoryLine
	for _, h := range tAck.Root.History {
		if h.Ack {
			acks = append(acks, h)
		}
	}
	if len(acks) != 1 || len(acks[0].Targets) != 2 || acks[0].Displaced == "" ||
		!strings.HasPrefix(acks[0].Displaced, "mac-a+lin-1@") {
		t.Fatalf("one ack line answers the pair's displacement across the cascade: %+v", acks)
	}
	// Acknowledged once: the pair's NEXT publication does not re-ack.
	stillGoing := "Still going."
	res, err = Edit(verbReq(a, "01J5X00000000000000000C219", "mac-a"), "casc-after", EditFields{NextStep: &stillGoing})
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("edit after ack: %+v %v", res, err)
	}
	tAgain, err := loadTree(a, res.Tip)
	if err != nil {
		t.Fatal(err)
	}
	ackCount := 0
	for _, h := range tAgain.Root.History {
		if h.Ack {
			ackCount++
		}
	}
	if ackCount != 1 {
		t.Fatalf("an answered displacement never re-acks: %d lines", ackCount)
	}
	// The human lifts the pause; the whole arc restores.
	humanUnpark := verbReq(a, "01J5X00000000000000000C220", "mac-a")
	humanUnpark.Actor.Human = "wido"
	res, err = UnparkArc(humanUnpark, "casc-one")
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("unpark cascade: %+v %v", res, err)
	}
	t3, err := loadTree(a, res.Tip)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"casc-one", "casc-two"} {
		if t3.Live[id].State != StateQueued {
			t.Fatalf("unpark restores the whole arc: %s is %s", id, t3.Live[id].State)
		}
	}
}

func TestDetachReleasesWithoutSplittingTheQuota(t *testing.T) {
	_, a, _ := twoClones(t)
	seedLedger(t, a)
	for i, id := range []string{"det-one", "det-two"} {
		if res, err := Open(verbReq(a, fmt.Sprintf("01J5X00000000000000000D0%d0", i), "mac-a"), id, "Det "+id, "main", "Go."); err != nil || res.Outcome != OutcomeConfirmed {
			t.Fatalf("open %s: %+v %v", id, res, err)
		}
		if res, err := SetArc(verbReq(a, fmt.Sprintf("01J5X00000000000000000D1%d0", i), "mac-a"), id, "det-arc"); err != nil || res.Outcome != OutcomeConfirmed {
			t.Fatalf("set-arc %s: %+v %v", id, res, err)
		}
	}
	if res, err := ClaimArc(verbReq(a, "01J5X00000000000000000D200", "mac-a"), "det-one", testBudget()); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("claim arc: %+v %v", res, err)
	}
	// Detaching a claimed member releases it: the quota never
	// splits one claim into two independent ones.
	res, err := Detach(verbReq(a, "01J5X00000000000000000D210", "mac-a"), "det-two")
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("detach: %+v %v", res, err)
	}
	t2, err := loadTree(a, res.Tip)
	if err != nil {
		t.Fatal(err)
	}
	freed := t2.Live["det-two"]
	if freed.Arc != "" || freed.State != StateQueued || freed.Claimed != nil {
		t.Fatalf("the departing member releases arcless: %+v", freed)
	}
	kept := t2.Live["det-one"]
	if kept.State != StateClaimed || kept.Claimed == nil {
		t.Fatalf("the remaining member keeps the claim: %+v", kept)
	}
	if problems := ValidateTree(t2); len(problems) != 0 {
		t.Fatalf("the tree stays lawful after the detach: %v", problems)
	}
}

func TestQueuedJoinsClaimedArcUnderTheClaimantOnly(t *testing.T) {
	_, a, b := twoClones(t)
	seedLedger(t, a)
	if res, err := OpenClaim(verbReq(a, "01J5X00000000000000000D300", "mac-a"), "anchor", "The anchor.", "main", "Go.", testBudget()); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open --claim anchor: %+v %v", res, err)
	}
	if res, err := SetArc(verbReq(a, "01J5X00000000000000000D310", "mac-a"), "anchor", "join-arc"); err == nil && res.Outcome == OutcomeConfirmed {
		t.Fatalf("set-arc on a CLAIMED goal must refuse (queued moves only): %+v", res)
	}
	// Release the standing claim first — the validator refuses a
	// second independent claim (the quota is one per machine), which
	// the first draft of this test proved by accident.
	if res, err := Release(verbReq(a, "01J5X00000000000000000D315", "mac-a"), "anchor"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("release anchor: %+v %v", res, err)
	}
	// Re-anchor properly: open a queued member, arc it, claim the arc.
	if res, err := Open(verbReq(a, "01J5X00000000000000000D320", "mac-a"), "anchor2", "Anchor two.", "main", "Go."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open anchor2: %+v %v", res, err)
	}
	if res, err := SetArc(verbReq(a, "01J5X00000000000000000D330", "mac-a"), "anchor2", "join-arc"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("set-arc anchor2: %+v %v", res, err)
	}
	if res, err := ClaimArc(verbReq(a, "01J5X00000000000000000D340", "mac-a"), "anchor2", testBudget()); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("claim join-arc: %+v %v", res, err)
	}
	// A stranger cannot move a queued goal into the claimed arc.
	if res, err := Open(verbReq(b, "01J5X00000000000000000D350", "mac-b"), "joiner", "Wants in.", "main", "Go."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open joiner: %+v %v", res, err)
	}
	if res, err := SetBudget(verbReq(b, "01J5X00000000000000000D355", "mac-b"), "joiner", testBudget()); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("budget joiner: %+v %v", res, err)
	}
	res, err := SetArc(verbReq(b, "01J5X00000000000000000D360", "mac-b"), "joiner", "join-arc")
	if err != nil || res.Outcome != OutcomeRejected || !strings.Contains(res.Detail, "stranger") {
		t.Fatalf("a stranger's move into a claimed arc rejects: %+v %v", res, err)
	}
	// The claimant's own move auto-claims the joiner under the
	// standing claim — the arc stays one unit.
	res, err = SetArc(verbReq(a, "01J5X00000000000000000D370", "mac-a"), "joiner", "join-arc")
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("the claimant's move proceeds: %+v %v", res, err)
	}
	t2, err := loadTree(a, res.Tip)
	if err != nil {
		t.Fatal(err)
	}
	joined := t2.Live["joiner"]
	if joined.Arc != "join-arc" || joined.State != StateClaimed || joined.Claimed == nil || joined.Claimed.Machine != "mac-a" {
		t.Fatalf("the joiner auto-claims under the standing claimant: %+v", joined)
	}
	if problems := ValidateTree(t2); len(problems) != 0 {
		t.Fatalf("the joined arc stays lawful: %v", problems)
	}
}

func TestFreshNoOpsAbandonHonestly(t *testing.T) {
	_, a, b := twoClones(t)
	seedLedger(t, a)
	if res, err := OpenClaim(verbReq(a, "01J5X00000000000000000F900", "mac-a"), "held-fast", "Held.", "main", "Go.", testBudget()); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open --claim: %+v %v", res, err)
	}
	// A fresh claim of an already-ours goal abandons (F8): its opid
	// is nowhere, so confirmed would be a lie.
	res, err := Claim(verbReq(a, "01J5X00000000000000000F910", "mac-a"), "held-fast", testBudget())
	if err != nil || res.Outcome != OutcomeAbandoned || !strings.Contains(res.Detail, "not by this operation") {
		t.Fatalf("claim-already-ours abandons: %+v %v", res, err)
	}
	// A fresh steal of an already-ours goal abandons the same way.
	humanReq := verbReq(a, "01J5X00000000000000000F920", "mac-a")
	humanReq.Actor.Human = "wido"
	res, err = Steal(humanReq, "held-fast")
	if err != nil || res.Outcome != OutcomeAbandoned {
		t.Fatalf("steal-already-ours abandons: %+v %v", res, err)
	}
	// Detach without an arc abandons with its reason.
	res, err = Detach(verbReq(b, "01J5X00000000000000000F930", "mac-b"), "held-fast")
	if err != nil || res.Outcome != OutcomeAbandoned || !strings.Contains(res.Detail, "not in an arc") {
		t.Fatalf("detach-without-arc abandons: %+v %v", res, err)
	}
	// Every abandoned entry is journaled with its reason — no
	// confirmed entry whose opid is nowhere.
	for _, ulid := range []string{"01J5X00000000000000000F910", "01J5X00000000000000000F920"} {
		entry, err := ReadEntry(a, Opid(ulid, "mac-a", "lin-1"))
		if err != nil || entry.Outcome != OutcomeAbandoned {
			t.Fatalf("the no-op journals abandoned: %+v %v", entry, err)
		}
	}
}

// The pin: only the named machine may claim a pinned goal — every
// other machine, and even a human steal onto one, refuses by name.
// Pinning itself is a human act, refuses over a foreign claim, and
// "-" clears it.
func TestMachinePinning(t *testing.T) {
	_, a, b := twoClones(t)
	seedLedger(t, a)
	if res, err := Open(verbReq(a, "01J5X0000000000000000000P1", "mac-a"), "gpu-work", "Needs the big machine.", "main", "Train."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", res, err)
	}

	// An agent cannot pin: pinning directs machines, so it names its
	// human.
	if _, err := SetPin(verbReq(a, "01J5X0000000000000000000P2", "mac-a"), "gpu-work", "mac-b"); err == nil ||
		!strings.Contains(err.Error(), "--by") {
		t.Fatalf("agent set-pin refuses: %v", err)
	}
	pinReq := verbReq(a, "01J5X0000000000000000000P3", "mac-a")
	pinReq.Actor.Human = "wido"
	pinRes, err := SetPin(pinReq, "gpu-work", "mac-b")
	if err != nil || pinRes.Outcome != OutcomeConfirmed {
		t.Fatalf("the attributed pin lands: %+v %v", pinRes, err)
	}
	pinnedTree, err := loadTree(a, pinRes.Tip)
	if err != nil {
		t.Fatal(err)
	}
	pinnedFile := pinnedTree.Live["gpu-work"]
	if pinnedFile == nil || pinnedFile.Pinned != "mac-b" {
		t.Fatalf("the file carries the pin: %+v", pinnedFile)
	}
	lastLine := pinnedFile.History[len(pinnedFile.History)-1]
	if lastLine.Verb != "set-pin" || lastLine.Actor != "human:wido" {
		t.Fatalf("the pin's history line names its human: %+v", lastLine)
	}

	// The wrong machine's claim rejects naming both machines.
	res, err := Claim(verbReq(a, "01J5X0000000000000000000P4", "mac-a"), "gpu-work", testBudget())
	if err != nil || res.Outcome != OutcomeRejected ||
		!strings.Contains(res.Detail, "pinned to machine mac-b") || !strings.Contains(res.Detail, "mac-a") {
		t.Fatalf("a foreign claim rejects by name: %+v %v", res, err)
	}
	// The pinned machine claims normally.
	if res, err := Claim(verbReq(b, "01J5X0000000000000000000P5", "mac-b"), "gpu-work", testBudget()); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("the pinned machine claims: %+v %v", res, err)
	}

	// Re-pinning a goal claimed elsewhere refuses: the human decides
	// between waiting, releasing, and stealing first.
	repin := verbReq(a, "01J5X0000000000000000000P6", "mac-a")
	repin.Actor.Human = "wido"
	if res, err := SetPin(repin, "gpu-work", "mac-a"); err != nil || res.Outcome != OutcomeRejected ||
		!strings.Contains(res.Detail, "claimed by machine mac-b") {
		t.Fatalf("re-pin over a foreign claim rejects: %+v %v", res, err)
	}

	// Even a human steal honors the pin: the stealing machine is not
	// the pinned one.
	stealReq := verbReq(a, "01J5X0000000000000000000P7", "mac-a")
	stealReq.Actor.Human = "wido"
	if res, err := Steal(stealReq, "gpu-work"); err != nil || res.Outcome != OutcomeRejected ||
		!strings.Contains(res.Detail, "honors the pin") {
		t.Fatalf("a steal onto a foreign machine rejects: %+v %v", res, err)
	}

	// While pinned to mac-b, the goal is invisible to mac-a's frontier
	// and ready on mac-b's.
	frontierTree, err := loadTree(a, pinRes.Tip)
	if err != nil {
		t.Fatal(err)
	}
	frontierA := Next(Projection{Tree: frontierTree}, "mac-a")
	for _, id := range frontierA.Ready {
		if id == "gpu-work" {
			t.Fatal("a foreign-pinned goal must not be mac-a's ready work")
		}
	}
	frontierB := Next(Projection{Tree: frontierTree}, "mac-b")
	readyOnB := false
	for _, id := range frontierB.Ready {
		if id == "gpu-work" {
			readyOnB = true
		}
	}
	if !readyOnB {
		t.Fatal("the pinned machine's frontier must list the goal ready")
	}

	// A released pin TRANSFERS whole: mac-b's pin moves to mac-a, and
	// mac-a — refused before — now claims.
	if res, err := Release(verbReq(b, "01J5X0000000000000000000P8", "mac-b"), "gpu-work"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("release: %+v %v", res, err)
	}
	moveReq := verbReq(a, "01J5X0000000000000000000P9", "mac-a")
	moveReq.Actor.Human = "wido"
	if res, err := SetPin(moveReq, "gpu-work", "mac-a"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("the transfer lands: %+v %v", res, err)
	}
	if res, err := Claim(verbReq(a, "01J5X0000000000000000000PA", "mac-a"), "gpu-work", testBudget()); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("the newly pinned machine claims: %+v %v", res, err)
	}

	// Clearing with "-" reopens the field: after release, any machine
	// claims again.
	if res, err := Release(verbReq(a, "01J5X0000000000000000000PB", "mac-a"), "gpu-work"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("release for the clear: %+v %v", res, err)
	}
	clearReq := verbReq(a, "01J5X0000000000000000000PC", "mac-a")
	clearReq.Actor.Human = "wido"
	if res, err := SetPin(clearReq, "gpu-work", "-"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("the clear lands: %+v %v", res, err)
	}
	if res, err := Claim(verbReq(b, "01J5X0000000000000000000PD", "mac-b"), "gpu-work", testBudget()); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("an unpinned goal claims anywhere: %+v %v", res, err)
	}
}
