package goal

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// The F17 fold, Go half: TRUE concurrency, not sequenced turns. Two
// clones publish at the same wall-clock moment; the CAS decides, the
// loser retries on the winner's tip or names the winner, and nothing
// is ever lost or doubled. (The shell half drives the CLI verbs
// end-to-end in scripts/agents/goal-cli-fixtures.sh.)

func TestConcurrentPublishesBothLandThroughTheCAS(t *testing.T) {
	_, a, b := twoClones(t)
	seedLedger(t, a)

	// Two DIFFERENT goals, two clones, one starting gun: exactly one
	// push wins the first CAS and the other retries on the advanced
	// tip inside its own publish loop — both confirm.
	var start, done sync.WaitGroup
	start.Add(1)
	results := make([]PublishResult, 2)
	errs := make([]error, 2)
	for i, leg := range []struct {
		root, ulid, machine, id string
	}{
		{a, "01J5X00000000000000000RA10", "mac-a", "race-a"},
		{b, "01J5X00000000000000000RB10", "mac-b", "race-b"},
	} {
		done.Add(1)
		go func(slot int, root, ulid, machine, id string) {
			defer done.Done()
			start.Wait()
			results[slot], errs[slot] = Open(verbReq(root, ulid, machine), id, "Raced open "+id, "main", "Go.")
		}(i, leg.root, leg.ulid, leg.machine, leg.id)
	}
	start.Done()
	done.Wait()

	for i := range results {
		if errs[i] != nil || results[i].Outcome != OutcomeConfirmed {
			t.Fatalf("racer %d: %+v %v", i, results[i], errs[i])
		}
	}
	// Convergence: one fetch from either clone sees BOTH goals.
	adv, err := FetchAdvance(endpointFor(a))
	if err != nil {
		t.Fatal(err)
	}
	tree, err := loadTree(a, adv.Tip)
	if err != nil {
		t.Fatal(err)
	}
	if tree.Live["race-a"] == nil || tree.Live["race-b"] == nil {
		t.Fatalf("both racers landed, neither lost: %v", sortedGoalIds(tree.Live))
	}
}

func TestConcurrentSameGoalRaceNamesOneWinner(t *testing.T) {
	_, a, b := twoClones(t)
	seedLedger(t, a)

	// The SAME goal id from both clones: exactly one confirms; the
	// other loses TO THE WINNER BY NAME — never a silent overwrite,
	// never a double create.
	var start, done sync.WaitGroup
	start.Add(1)
	results := make([]PublishResult, 2)
	errs := make([]error, 2)
	ulids := []string{"01J5X00000000000000000RS10", "01J5X00000000000000000RS20"}
	for i, leg := range []struct {
		root, machine string
	}{{a, "mac-a"}, {b, "mac-b"}} {
		done.Add(1)
		go func(slot int, root, machine string) {
			defer done.Done()
			start.Wait()
			results[slot], errs[slot] = Open(verbReq(root, ulids[slot], machine), "contested", "Raced create.", "main", "Go.")
		}(i, leg.root, leg.machine)
	}
	start.Done()
	done.Wait()

	confirmed, lost := -1, -1
	for i := range results {
		if errs[i] != nil {
			t.Fatalf("racer %d errored instead of classifying: %v", i, errs[i])
		}
		switch results[i].Outcome {
		case OutcomeConfirmed:
			confirmed = i
		case OutcomeLost:
			lost = i
		}
	}
	if confirmed == -1 || lost == -1 {
		t.Fatalf("exactly one winner and one named loss: %+v", results)
	}
	winnerOpid := Opid(ulids[confirmed], []string{"mac-a", "mac-b"}[confirmed], "lin-1")
	if !strings.Contains(results[lost].Detail, winnerOpid) {
		t.Fatalf("the loser names the winner: %q vs %q", results[lost].Detail, winnerOpid)
	}
	// The contested goal exists ONCE, under the winner's opid.
	adv, err := FetchAdvance(endpointFor(a))
	if err != nil {
		t.Fatal(err)
	}
	tree, err := loadTree(a, adv.Tip)
	if err != nil {
		t.Fatal(err)
	}
	f := tree.Live["contested"]
	if f == nil || f.History[0].Opid != winnerOpid {
		t.Fatalf("the winner's create stands alone: %+v", f)
	}
}

func TestAcceptedRefCASHoldsUnderRaceAndNeverRewinds(t *testing.T) {
	_, a, b := twoClones(t)
	seedLedger(t, a)
	baseline, err := FetchAdvance(endpointFor(a))
	if err != nil {
		t.Fatal(err)
	}

	// The remote advances past this clone's accepted ref...
	res, err := Open(verbReq(b, "01J5X00000000000000000AR10", "mac-b"), "advancer", "Moves the tip.", "main", "Go.")
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open on b: %+v %v", res, err)
	}
	// ...and TWO concurrent advances in one clone race the accepted
	// ref's own CAS: both return clean, the ref lands on the new tip
	// exactly once.
	var start, done sync.WaitGroup
	start.Add(1)
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		done.Add(1)
		go func(slot int) {
			defer done.Done()
			start.Wait()
			_, errs[slot] = FetchAdvance(endpointFor(a))
		}(i)
	}
	start.Done()
	done.Wait()
	for i, raceErr := range errs {
		if raceErr != nil {
			t.Fatalf("concurrent advance %d: %v", i, raceErr)
		}
	}
	tipOut := strings.TrimSpace(mustGit(t, a, "rev-parse", AcceptedRef))
	if tipOut != res.Tip {
		t.Fatalf("the raced advances land on the canonical tip once: %s vs %s", short(tipOut), short(res.Tip))
	}

	// Forward-only under pressure: an explicit advance BACK to the
	// baseline must not move the ref — a rewind is never a race
	// outcome.
	_ = AdvanceAccepted(a, baseline.Tip)
	if got := strings.TrimSpace(mustGit(t, a, "rev-parse", AcceptedRef)); got != tipOut {
		t.Fatalf("the accepted ref never rewinds: %s vs %s", short(got), short(tipOut))
	}
}

func TestConcurrentSameFieldReconcileNeverSilentlyOverwrites(t *testing.T) {
	_, a, b := twoClones(t)
	seedLedger(t, a)
	res, err := Open(verbReq(a, "01J5X00000000000000000CF10", "mac-a"), "contested-field", "Original intent.", "main", "Go.")
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", res, err)
	}
	materialize(t, a, res.Tip)

	// The hand edit in clone A, captured against the base...
	editablePath := filepath.Join(a, "plans", "goals", "contested-field.md")
	edited, err := os.ReadFile(editablePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(editablePath, []byte(strings.Replace(string(edited), "Original intent.", "Hand-edited intent.", 1)), 0o644); err != nil {
		t.Fatal(err)
	}

	// ...races clone B's verb edit of the SAME field.
	competitor := "Competitor intent."
	var start, done sync.WaitGroup
	start.Add(1)
	var reconcileRes ReconcileResult
	var reconcileErr, editErr error
	var editRes PublishResult
	done.Add(2)
	go func() {
		defer done.Done()
		start.Wait()
		req := verbReq(a, "01J5X00000000000000000CF20", "mac-a")
		req.Actor.Human = "wido"
		reconcileRes, reconcileErr = Reconcile(req)
	}()
	go func() {
		defer done.Done()
		start.Wait()
		editRes, editErr = Edit(verbReq(b, "01J5X00000000000000000CF30", "mac-b"), "contested-field", EditFields{Intent: &competitor})
	}()
	start.Done()
	done.Wait()

	if editErr != nil || editRes.Outcome != OutcomeConfirmed {
		t.Fatalf("the verb edit lands: %+v %v", editRes, editErr)
	}
	// The certified invariant: the hand edit NEVER silently erases
	// the committed verb edit. Whichever way the race falls, the
	// verb edit's value survives on the final tree — the reconcile
	// either lost the race and REJECTED with the field named, or won
	// it and was lawfully overwritten by the retrying verb edit.
	adv, err := FetchAdvance(endpointFor(a))
	if err != nil {
		t.Fatal(err)
	}
	tree, err := loadTree(a, adv.Tip)
	if err != nil {
		t.Fatal(err)
	}
	final := tree.Live["contested-field"]
	if final.Intent != competitor {
		t.Fatalf("the committed verb edit survives the race: %q (reconcile: %+v %v)", final.Intent, reconcileRes.Publish, reconcileErr)
	}
	if reconcileErr == nil && reconcileRes.Publish.Outcome == OutcomeRejected &&
		!strings.Contains(reconcileRes.Publish.Detail, "intent") {
		t.Fatalf("a rejected reconcile names the field: %+v", reconcileRes.Publish)
	}
}
