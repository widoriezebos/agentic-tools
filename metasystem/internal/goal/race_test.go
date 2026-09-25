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

// The two Open races wait until both clients reach their first publication.
// Each client still uses the fake store's real compare-and-swap result.
type openRaceRepository struct {
	*fakeGoalRepository
	firstPublishes *sync.WaitGroup
	firstPublish   sync.Once
	mu             sync.Mutex
	captures       int
	firstCaptures  int
	refusals       int
}

func (r *openRaceRepository) Capture(opid string) (string, error) {
	tip, err := r.fakeGoalRepository.Capture(opid)
	r.mu.Lock()
	r.captures++
	r.mu.Unlock()
	return tip, err
}

func (r *openRaceRepository) Publish(parent, commit string) (CASOutcome, error) {
	r.firstPublish.Do(func() {
		r.mu.Lock()
		r.firstCaptures = r.captures
		r.mu.Unlock()
		r.firstPublishes.Done()
		r.firstPublishes.Wait()
	})
	outcome, err := r.fakeGoalRepository.Publish(parent, commit)
	if outcome == CASRefused {
		r.mu.Lock()
		r.refusals++
		r.mu.Unlock()
	}
	return outcome, err
}

func (r *openRaceRepository) counts() (refusals, captures, firstCaptures int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.refusals, r.captures, r.firstCaptures
}

func TestConcurrentPublishesBothLandThroughTheCAS(t *testing.T) {
	t.Parallel()
	a, b := fakeGoalEndpointPair(t)
	var firstPublishes sync.WaitGroup
	firstPublishes.Add(2)
	clients := [2]*openRaceRepository{
		{fakeGoalRepository: a.Repository.(*fakeGoalRepository), firstPublishes: &firstPublishes},
		{fakeGoalRepository: b.Repository.(*fakeGoalRepository), firstPublishes: &firstPublishes},
	}
	a.Repository, b.Repository = clients[0], clients[1]

	// Two DIFFERENT goals, two clones, one starting gun: exactly one
	// push wins the first CAS and the other retries on the advanced
	// tip inside its own publish loop — both confirm.
	var start, done sync.WaitGroup
	start.Add(1)
	results := make([]PublishResult, 2)
	errs := make([]error, 2)
	for i, leg := range []struct {
		endpoint          Endpoint
		ulid, machine, id string
	}{
		{a, "01J5X00000000000000000RA10", "mac-a", "race-a"},
		{b, "01J5X00000000000000000RB10", "mac-b", "race-b"},
	} {
		done.Add(1)
		go func(slot int, endpoint Endpoint, ulid, machine, id string) {
			defer done.Done()
			start.Wait()
			results[slot], errs[slot] = Open(verbReqFor(endpoint, ulid, machine), id, "Raced open "+id, "main", "Go.")
		}(i, leg.endpoint, leg.ulid, leg.machine, leg.id)
	}
	start.Done()
	done.Wait()

	for i := range results {
		if errs[i] != nil || results[i].Outcome != OutcomeConfirmed {
			t.Fatalf("racer %d: %+v %v", i, results[i], errs[i])
		}
	}
	refusals := 0
	for i, client := range clients {
		refused, captures, firstCaptures := client.counts()
		refusals += refused
		if firstCaptures == 0 || (refused == 1 && captures <= firstCaptures) {
			t.Fatalf("racer %d did not recapture after a refused first publish: refusals=%d captures=%d before first publish=%d", i, refused, captures, firstCaptures)
		}
	}
	if refusals != 1 {
		t.Fatalf("the two Opens require one actual stale-parent refusal; got %d", refusals)
	}
	// Convergence: one fetch from either clone sees BOTH goals.
	adv, err := FetchAdvance(a)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := loadTreeFor(a, adv.Tip)
	if err != nil {
		t.Fatal(err)
	}
	if tree.Live["race-a"] == nil || tree.Live["race-b"] == nil {
		t.Fatalf("both racers landed, neither lost: %v", sortedGoalIds(tree.Live))
	}
}

func TestConcurrentSameGoalRaceNamesOneWinner(t *testing.T) {
	t.Parallel()
	a, b := fakeGoalEndpointPair(t)
	var firstPublishes sync.WaitGroup
	firstPublishes.Add(2)
	clients := [2]*openRaceRepository{
		{fakeGoalRepository: a.Repository.(*fakeGoalRepository), firstPublishes: &firstPublishes},
		{fakeGoalRepository: b.Repository.(*fakeGoalRepository), firstPublishes: &firstPublishes},
	}
	a.Repository, b.Repository = clients[0], clients[1]

	// The SAME goal id from both clones: exactly one confirms; the
	// other loses TO THE WINNER BY NAME — never a silent overwrite,
	// never a double create.
	var start, done sync.WaitGroup
	start.Add(1)
	results := make([]PublishResult, 2)
	errs := make([]error, 2)
	ulids := []string{"01J5X00000000000000000RS10", "01J5X00000000000000000RS20"}
	for i, leg := range []struct {
		endpoint Endpoint
		machine  string
	}{{a, "mac-a"}, {b, "mac-b"}} {
		done.Add(1)
		go func(slot int, endpoint Endpoint, machine string) {
			defer done.Done()
			start.Wait()
			results[slot], errs[slot] = Open(verbReqFor(endpoint, ulids[slot], machine), "contested", "Raced create.", "main", "Go.")
		}(i, leg.endpoint, leg.machine)
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
	refusals := 0
	for i, client := range clients {
		refused, captures, firstCaptures := client.counts()
		refusals += refused
		if firstCaptures == 0 || (refused == 1 && captures <= firstCaptures) {
			t.Fatalf("racer %d did not recapture after a refused first publish: refusals=%d captures=%d before first publish=%d", i, refused, captures, firstCaptures)
		}
	}
	if refusals != 1 {
		t.Fatalf("the contested Opens require one actual stale-parent refusal; got %d", refusals)
	}
	winnerOpid := Opid(ulids[confirmed], []string{"mac-a", "mac-b"}[confirmed], "lin-1")
	if !strings.Contains(results[lost].Detail, winnerOpid) {
		t.Fatalf("the loser names the winner: %q vs %q", results[lost].Detail, winnerOpid)
	}
	// The contested goal exists ONCE, under the winner's opid.
	adv, err := FetchAdvance(a)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := loadTreeFor(a, adv.Tip)
	if err != nil {
		t.Fatal(err)
	}
	f := tree.Live["contested"]
	if f == nil || f.History[0].Opid != winnerOpid {
		t.Fatalf("the winner's create stands alone: %+v", f)
	}
}

func TestConcurrentSplitMembersClaimIndependently(t *testing.T) {
	t.Parallel()
	a, b := fakeGoalEndpointPair(t)
	if res, err := Open(verbReqFor(a, "01J5X00000000000000000CJ00", "mac-a"), "concurrent-parent", "Independent work.", OriginMain, "Split it."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open parent: %+v %v", res, err)
	}
	members := []MemberDraft{
		{ID: "concurrent-one", Intent: "First independent member.", NextStep: "Work one."},
		{ID: "concurrent-two", Intent: "Second independent member.", NextStep: "Work two."},
	}
	if res, err := Split(verbReqFor(a, "01J5X00000000000000000CJ10", "mac-a"), "concurrent-parent", members, mainRatification("concurrent-parent", members), nil); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("split: %+v %v", res, err)
	}
	approveGoalForTest(t, verbReqFor(a, "01J5X00000000000000000CJ15", "mac-a"), "concurrent-one", testBudget())
	approveGoalForTest(t, verbReqFor(a, "01J5X00000000000000000CJ16", "mac-a"), "concurrent-two", testBudget())
	if _, err := FetchAdvance(b); err != nil {
		t.Fatal(err)
	}

	var start, done sync.WaitGroup
	start.Add(1)
	results := make([]PublishResult, 2)
	errs := make([]error, 2)
	for index, leg := range []struct {
		endpoint          Endpoint
		id, machine, ulid string
	}{{a, "concurrent-one", "mac-a", "01J5X00000000000000000CJ20"}, {b, "concurrent-two", "mac-b", "01J5X00000000000000000CJ30"}} {
		done.Add(1)
		go func(slot int, endpoint Endpoint, id, machine, ulid string) {
			defer done.Done()
			start.Wait()
			results[slot], errs[slot] = Claim(verbReqFor(endpoint, ulid, machine), id)
		}(index, leg.endpoint, leg.id, leg.machine, leg.ulid)
	}
	start.Done()
	done.Wait()
	for index := range results {
		if errs[index] != nil || results[index].Outcome != OutcomeConfirmed {
			t.Fatalf("independent claimant %d: %+v %v", index, results[index], errs[index])
		}
	}
	advanced, err := FetchAdvance(a)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := loadTreeFor(a, advanced.Tip)
	if err != nil || tree.Live["concurrent-one"].Claimed.Machine != "mac-a" || tree.Live["concurrent-two"].Claimed.Machine != "mac-b" {
		t.Fatalf("both independent claims did not converge: %+v %v", tree, err)
	}
	if err := validateCommitFor(a, advanced.Tip); err != nil {
		t.Fatalf("mixed claimant tree did not validate: %v", err)
	}
	if res, err := Open(verbReqFor(a, "01J5X00000000000000000CJ40", "mac-a"), "quota-bystander", "Unrelated claim.", OriginMain, "Wait."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open bystander: %+v %v", res, err)
	}
	approveGoalForTest(t, verbReqFor(a, "01J5X00000000000000000CJ45", "mac-a"), "quota-bystander", testBudget())
	quota, err := Claim(verbReqFor(a, "01J5X00000000000000000CJ50", "mac-a"), "quota-bystander")
	if err != nil || quota.Outcome != OutcomeRejected || !strings.Contains(quota.Detail, "quota") {
		t.Fatalf("mixed arc weakened the per-machine quota: %+v %v", quota, err)
	}
}

func TestSplitMemberDependencyStillOwnsClaimOrdering(t *testing.T) {
	t.Parallel()
	a, b := fakeGoalEndpointPair(t)
	if res, err := Open(verbReqFor(a, "01J5X00000000000000000CD00", "mac-a"), "dependency-parent", "Ordered work.", OriginMain, "Split it."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", res, err)
	}
	members := testMembers("dependency-parent")
	if res, err := Split(verbReqFor(a, "01J5X00000000000000000CD10", "mac-a"), "dependency-parent", members, mainRatification("dependency-parent", members), nil); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("split: %+v %v", res, err)
	}
	approveGoalForTest(t, verbReqFor(a, "01J5X00000000000000000CD15", "mac-a"), "dependency-parent-one", testBudget())
	approveGoalForTest(t, verbReqFor(a, "01J5X00000000000000000CD16", "mac-a"), "dependency-parent-two", testBudget())
	if _, err := FetchAdvance(b); err != nil {
		t.Fatal(err)
	}
	blocked, err := Claim(verbReqFor(b, "01J5X00000000000000000CD20", "mac-b"), "dependency-parent-two")
	if err != nil || blocked.Outcome != OutcomeRejected || !strings.Contains(blocked.Detail, "dependency-parent-one") {
		t.Fatalf("unmet member dependency did not refuse by name: %+v %v", blocked, err)
	}
	if res, err := Claim(verbReqFor(a, "01J5X00000000000000000CD30", "mac-a"), "dependency-parent-one"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("claim predecessor: %+v %v", res, err)
	}
	if res, err := Done(verbReqFor(a, "01J5X00000000000000000CD40", "mac-a"), "dependency-parent-one", "Predecessor done."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("complete predecessor: %+v %v", res, err)
	}
	if res, err := Claim(verbReqFor(b, "01J5X00000000000000000CD50", "mac-b"), "dependency-parent-two"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("claim after dependency completion: %+v %v", res, err)
	}
}

func TestAcceptedRefCASHoldsUnderRaceAndNeverRewinds(t *testing.T) {
	t.Parallel()
	a, b := fakeGoalEndpointPair(t)
	baseline, err := FetchAdvance(a)
	if err != nil {
		t.Fatal(err)
	}

	// The remote advances past this clone's accepted ref...
	res, err := Open(verbReqFor(b, "01J5X00000000000000000AR10", "mac-b"), "advancer", "Moves the tip.", "main", "Go.")
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
			_, errs[slot] = FetchAdvance(a)
		}(i)
	}
	start.Done()
	done.Wait()
	for i, raceErr := range errs {
		if raceErr != nil {
			t.Fatalf("concurrent advance %d: %v", i, raceErr)
		}
	}
	tipOut := acceptedTipForEndpoint(t, a)
	client := a.Repository.(*fakeGoalRepository)
	client.store.mu.Lock()
	canonical := client.store.canonical
	client.store.mu.Unlock()
	if tipOut != res.Tip || tipOut != canonical {
		t.Fatalf("the raced advances land on the canonical tip once: accepted=%s published=%s canonical=%s", short(tipOut), short(res.Tip), short(canonical))
	}

	// Forward-only under pressure: an explicit advance BACK to the
	// baseline must not move the ref — a rewind is never a race
	// outcome.
	if err := advanceAcceptedFor(a, baseline.Tip); err != nil {
		t.Fatal(err)
	}
	if got := acceptedTipForEndpoint(t, a); got != tipOut {
		t.Fatalf("the accepted ref never rewinds: %s vs %s", short(got), short(tipOut))
	}
}

func TestConcurrentSameFieldReconcileNeverSilentlyOverwrites(t *testing.T) {
	t.Parallel()
	a, b := fakeGoalEndpointPair(t)
	res, err := Open(verbReqFor(a, "01J5X00000000000000000CF10", "mac-a"), "contested-field", "Original intent.", "main", "Go.")
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", res, err)
	}
	materializeFakeReconcile(t, a, res.Tip)

	// The hand edit in clone A, captured against the base...
	editablePath := filepath.Join(a.Root, "plans", "goals", "contested-field.md")
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
		req := verbReqFor(a, "01J5X00000000000000000CF20", "mac-a")
		req.Actor.Human = "wido"
		reconcileRes, reconcileErr = reconcileForTest(t, req)
	}()
	go func() {
		defer done.Done()
		start.Wait()
		editRes, editErr = Edit(verbReqFor(b, "01J5X00000000000000000CF30", "mac-b"), "contested-field", EditFields{Intent: &competitor})
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
	adv, err := FetchAdvance(a)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := loadTreeFor(a, adv.Tip)
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
