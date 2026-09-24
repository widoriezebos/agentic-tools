package goal

import (
	"fmt"
	"strings"
	"testing"
)

// publishGoal drives one full transaction for fixture setup.
func publishGoal(t *testing.T, root, opid, id string, extra []*GoalFile) PublishResult {
	t.Helper()
	files := vTree(vRoot(), append([]*GoalFile{vGoal(id, StateQueued)}, extra...), nil)
	var changes []Change
	for p, content := range files {
		changes = append(changes, Change{Path: p, Content: content})
	}
	res, err := Publish(endpointFor(root), PublishRequest{
		Opid: opid, Machine: "mac-" + id, Lineage: "l1",
		Intent: testIntentFor("open"), Message: "goal open " + id,
		Mutate: func(tip string) ([]Change, error) { return changes, nil },
	})
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("fixture publish %s: %+v %v", id, res, err)
	}
	return res
}

func publishGoalAtEndpoint(t *testing.T, e Endpoint, opid, id string, extra []*GoalFile) PublishResult {
	t.Helper()
	files := vTree(vRoot(), append([]*GoalFile{vGoal(id, StateQueued)}, extra...), nil)
	changes := make([]Change, 0, len(files))
	for path, content := range files {
		changes = append(changes, Change{Path: path, Content: content})
	}
	res, err := Publish(e, PublishRequest{
		Opid: opid, Machine: "mac-" + id, Lineage: "l1",
		Intent: testIntentFor("open"), Message: "goal open " + id,
		Mutate: func(string) ([]Change, error) { return changes, nil },
	})
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("fixture publish %s: %+v %v", id, res, err)
	}
	return res
}

func TestTwoClonesConvergeByFetchAlone(t *testing.T) {
	t.Parallel()
	a, b := fakeGoalEndpointPair(t)

	// Different-goal mutations from two clones both publish;
	// each machine observes the other via goal fetch — a read-side
	// advance with no own mutation needed — and the projections
	// converge.
	publishGoalAtEndpoint(t, a, "op-alpha", "alpha", nil)
	// B publishes on A's canonical tip, carrying both goals forward.
	filesB := vTree(vRoot(), []*GoalFile{vGoal("beta", StateQueued)}, nil)
	resB, err := Publish(b, PublishRequest{
		Opid: "op-beta", Machine: "mac-b", Lineage: "l1",
		Intent: testIntentFor("open"), Message: "goal open beta",
		Mutate: func(string) ([]Change, error) {
			return []Change{{Path: goalsPrefix + "beta.md", Content: filesB[goalsPrefix+"beta.md"]}}, nil
		},
	})
	if err != nil || resB.Outcome != OutcomeConfirmed {
		t.Fatalf("B publishes: %+v %v", resB, err)
	}

	// A advances by FETCH ONLY and sees both goals.
	resA, err := FetchAdvance(a)
	if err != nil || !resA.Advanced {
		t.Fatalf("A's read-side advance: %+v %v", resA, err)
	}
	filesAtA, err := readCommitGoals(a, resA.Tip)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"alpha", "beta"} {
		if _, ok := filesAtA[goalsPrefix+id+".md"]; !ok {
			t.Fatalf("A must observe %s after the fetch: %v", id, len(filesAtA))
		}
	}
	// B advances too; the projections converge on the same tip.
	resB2, err := FetchAdvance(b)
	if err != nil {
		t.Fatalf("B's read-side advance: %v", err)
	}
	if resB2.Tip != resA.Tip {
		t.Fatalf("the projections converge: %s vs %s", resB2.Tip, resA.Tip)
	}
}

func TestRewoundBranchRefusesUntilRepair(t *testing.T) {
	t.Parallel()
	a, _ := fakeGoalEndpointPair(t)
	client := a.Repository.(*fakeGoalRepository)
	seedTip := canonicalTipForTransactionTest(t, a)
	publishGoalAtEndpoint(t, a, "op-r", "goal-r", nil)
	if _, err := FetchAdvance(a); err != nil {
		t.Fatal(err)
	}
	before := acceptedTipForEndpoint(t, a)

	// Branch surgery rewinds the canonical branch behind the
	// accepted tip.
	client.store.mu.Lock()
	client.store.canonical = seedTip
	client.store.mu.Unlock()
	_, err := FetchAdvance(a)
	if err == nil || !strings.Contains(err.Error(), "rewound") || !strings.Contains(err.Error(), "repair --accept-remote") {
		t.Fatalf("a rewind refuses by name and points at the deliberate path: %v", err)
	}
	if after := acceptedTipForEndpoint(t, a); after != before {
		t.Fatalf("the projection stays pinned: %s vs %s", after, before)
	}
}

func TestTornTipRefusesNamingFileAndRule(t *testing.T) {
	t.Parallel()
	a, b := fakeGoalEndpointPair(t)
	publishGoalAtEndpoint(t, a, "op-t", "goal-t", nil)
	if _, err := FetchAdvance(a); err != nil {
		t.Fatal(err)
	}
	before := acceptedTipForEndpoint(t, a)

	// A hand edit lands on the canonical branch without a recomputed
	// digest — the exact accidental-edit shape the guard exists for.
	files, err := b.Repository.Files(before, goalsPrefix+"goal-t.md")
	if err != nil {
		t.Fatal(err)
	}
	torn := strings.Replace(
		string(files[goalsPrefix+"goal-t.md"]),
		"Do the thing", "hand-edited without a digest", 1)
	commit, err := b.Repository.Build("tamper", before, []Change{{Path: goalsPrefix + "goal-t.md", Content: []byte(torn)}}, "tamper")
	if err != nil {
		t.Fatal(err)
	}
	if outcome, err := b.Repository.Publish(before, commit); err != nil || outcome != CASLanded {
		t.Fatalf("torn tip setup: %s %v", outcome, err)
	}

	_, err = FetchAdvance(a)
	if err == nil || !strings.Contains(err.Error(), "goal-t.md") || !strings.Contains(err.Error(), "Integrity") {
		t.Fatalf("the refusal names the file and the rule: %v", err)
	}
	if after := acceptedTipForEndpoint(t, a); after != before {
		t.Fatalf("the projection stays at the accepted tree: %s vs %s", after, before)
	}
}

func TestForeignLedgerRefusesByName(t *testing.T) {
	t.Parallel()
	a, c := fakeGoalEndpointPair(t)
	seedTip := canonicalTipForTransactionTest(t, a)
	publishGoalAtEndpoint(t, a, "op-f", "goal-f", nil)
	if _, err := FetchAdvance(a); err != nil {
		t.Fatal(err)
	}
	before := acceptedTipForEndpoint(t, a)

	// A second root in the same store is disconnected from A's accepted
	// commit, which remains readable while the canonical tip changes.
	client := a.Repository.(*fakeGoalRepository)
	foreignClient := c.Repository.(*fakeGoalRepository)
	client.store.mu.Lock()
	client.store.serial++
	foreignSeedTip := fmt.Sprintf("%040x", client.store.serial)
	client.store.commits[foreignSeedTip] = fakeGoalCommit{
		parent: "", files: map[string][]byte{}, at: client.store.commits[seedTip].at,
	}
	client.store.canonical = foreignSeedTip
	foreignClient.accepted = ""
	client.store.mu.Unlock()
	foreignRoot := vRoot()
	foreignRoot.Identity = "01J5XFFFFFFFFFFFFFFFFFFFFF"
	foreignFiles := vTree(foreignRoot, []*GoalFile{vGoal("foreign", StateQueued)}, nil)
	var changes []Change
	for p, content := range foreignFiles {
		changes = append(changes, Change{Path: p, Content: content})
	}
	res, err := Publish(c, PublishRequest{
		Opid: "op-foreign", Machine: "mac-c", Lineage: "l1",
		Intent: testIntentFor("open"), Message: "goal open foreign",
		Mutate: func(tip string) ([]Change, error) { return changes, nil },
	})
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("foreign world publish: %+v %v", res, err)
	}
	if descendant, err := client.IsAncestor(before, res.Tip); err != nil || descendant {
		t.Fatalf("foreign canonical history must be disconnected from A's accepted tip: descendant=%t err=%v", descendant, err)
	}
	if descendant, err := client.IsAncestor(seedTip, res.Tip); err != nil || descendant {
		t.Fatalf("foreign canonical history must be disconnected from A's original seed: descendant=%t err=%v", descendant, err)
	}
	acceptedFiles, err := client.Files(before, goalsPrefix+"goal-f.md")
	if err != nil || len(acceptedFiles[goalsPrefix+"goal-f.md"]) == 0 {
		t.Fatalf("A's accepted commit must remain readable: files=%v err=%v", acceptedFiles, err)
	}

	_, err = FetchAdvance(a)
	if err == nil || !strings.Contains(err.Error(), "foreign ledger") {
		t.Fatalf("a foreign ledger refuses by name: %v", err)
	}
	if after := acceptedTipForEndpoint(t, a); after != before {
		t.Fatalf("the foreign ledger moved A's accepted tip: %s vs %s", after, before)
	}
}

func TestMutationsRunTheAcceptanceGates(t *testing.T) {
	t.Parallel()
	a, _ := fakeGoalEndpointPair(t)
	client := a.Repository.(*fakeGoalRepository)
	seed := canonicalTipForTransactionTest(t, a)
	publishGoalAtEndpoint(t, a, "op-gate", "gated", nil)
	if _, err := FetchAdvance(a); err != nil {
		t.Fatal(err)
	}
	// Branch surgery rewinds the canonical branch; a MUTATION must
	// refuse exactly as the read side does — never build on a world
	// this clone would not accept (F5).
	client.store.mu.Lock()
	client.store.canonical = seed
	client.store.mu.Unlock()
	_, err := Open(verbReqFor(a, "01J5X00000000000000000G000", "mac-a"), "onto-rewind", "Must refuse.", "main", "No.")
	if err == nil || !strings.Contains(err.Error(), "rewound") {
		t.Fatalf("a mutation onto a rewound branch refuses by name: %v", err)
	}
	// The journal closed the attempt honestly.
	entry, readErr := ReadEntry(a.Root, Opid("01J5X00000000000000000G000", "mac-a", "lin-1"))
	if readErr != nil || entry.Phase != PhaseTerminal || entry.Outcome != OutcomeAbandoned {
		t.Fatalf("the refused mutation abandons its entry: %+v %v", entry, readErr)
	}
}
