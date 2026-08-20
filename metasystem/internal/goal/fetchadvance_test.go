package goal

import (
	"os"
	"path/filepath"
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

func TestTwoClonesConvergeByFetchAlone(t *testing.T) {
	_, a, b := twoClones(t)

	// BGS-1: different-goal mutations from two clones both publish;
	// each machine observes the other via goal fetch — a read-side
	// advance with no own mutation needed — and the projections
	// converge.
	publishGoal(t, a, "op-alpha", "alpha", nil)
	// B's publish rebuilds on A's tip through the ordinary benign
	// retry, carrying both goals forward.
	filesB := vTree(vRoot(), []*GoalFile{vGoal("beta", StateQueued)}, nil)
	resB, err := Publish(endpointFor(b), PublishRequest{
		Opid: "op-beta", Machine: "mac-b", Lineage: "l1",
		Intent: testIntentFor("open"), Message: "goal open beta",
		Mutate: func(tip string) ([]Change, error) {
			changes := []Change{{Path: goalsPrefix + "beta.md", Content: filesB[goalsPrefix+"beta.md"]}}
			if _, catErr := gitIn(b, "cat-file", "-p", tip+":"+goalsPrefix+"backlog.md"); catErr != nil {
				changes = append(changes, Change{Path: goalsPrefix + "backlog.md", Content: filesB[goalsPrefix+"backlog.md"]})
			}
			return changes, nil
		},
	})
	if err != nil || resB.Outcome != OutcomeConfirmed {
		t.Fatalf("B publishes: %+v %v", resB, err)
	}

	// A advances by FETCH ONLY and sees both goals.
	resA, err := FetchAdvance(endpointFor(a))
	if err != nil || !resA.Advanced {
		t.Fatalf("A's read-side advance: %+v %v", resA, err)
	}
	filesAtA, err := ReadCommitGoals(a, resA.Tip)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"alpha", "beta"} {
		if _, ok := filesAtA[goalsPrefix+id+".md"]; !ok {
			t.Fatalf("A must observe %s after the fetch: %v", id, len(filesAtA))
		}
	}
	// B advances too; the projections converge on the same tip.
	resB2, err := FetchAdvance(endpointFor(b))
	if err != nil {
		t.Fatalf("B's read-side advance: %v", err)
	}
	if resB2.Tip != resA.Tip {
		t.Fatalf("the projections converge: %s vs %s", resB2.Tip, resA.Tip)
	}
}

func TestRewoundBranchRefusesUntilRepair(t *testing.T) {
	origin, a, _ := twoClones(t)
	seedTip := mustGit(t, origin, "rev-parse", "refs/heads/main")
	publishGoal(t, a, "op-r", "goal-r", nil)
	if _, err := FetchAdvance(endpointFor(a)); err != nil {
		t.Fatal(err)
	}
	before := mustGit(t, a, "rev-parse", AcceptedRef)

	// Branch surgery rewinds the canonical branch behind the
	// accepted tip.
	mustGit(t, origin, "update-ref", "refs/heads/main", seedTip)
	_, err := FetchAdvance(endpointFor(a))
	if err == nil || !strings.Contains(err.Error(), "rewound") || !strings.Contains(err.Error(), "repair --accept-remote") {
		t.Fatalf("a rewind refuses by name and points at the deliberate path: %v", err)
	}
	if after := mustGit(t, a, "rev-parse", AcceptedRef); after != before {
		t.Fatalf("the projection stays pinned: %s vs %s", after, before)
	}
}

func TestTornTipRefusesNamingFileAndRule(t *testing.T) {
	origin, a, b := twoClones(t)
	publishGoal(t, a, "op-t", "goal-t", nil)
	if _, err := FetchAdvance(endpointFor(a)); err != nil {
		t.Fatal(err)
	}
	before := mustGit(t, a, "rev-parse", AcceptedRef)

	// A hand edit lands on the canonical branch without a recomputed
	// digest — the exact accidental-edit shape the guard exists for.
	mustGit(t, b, "fetch", "-q", "origin", "main")
	mustGit(t, b, "checkout", "-q", "-B", "tamper", "origin/main")
	torn := strings.Replace(
		mustGit(t, b, "cat-file", "-p", "origin/main:"+goalsPrefix+"goal-t.md"),
		"Do the thing", "hand-edited without a digest", 1)
	writeInWorktree(t, b, goalsPrefix+"goal-t.md", torn+"\n")
	mustGit(t, b, "add", goalsPrefix+"goal-t.md")
	mustGit(t, b, "commit", "-qm", "tamper")
	mustGit(t, b, "push", "-q", "origin", "tamper:main")
	_ = origin

	_, err := FetchAdvance(endpointFor(a))
	if err == nil || !strings.Contains(err.Error(), "goal-t.md") || !strings.Contains(err.Error(), "Integrity") {
		t.Fatalf("the refusal names the file and the rule: %v", err)
	}
	if after := mustGit(t, a, "rev-parse", AcceptedRef); after != before {
		t.Fatalf("the projection stays at the accepted tree: %s vs %s", after, before)
	}
}

func TestForeignLedgerRefusesByName(t *testing.T) {
	_, a, _ := twoClones(t)
	publishGoal(t, a, "op-f", "goal-f", nil)
	if _, err := FetchAdvance(endpointFor(a)); err != nil {
		t.Fatal(err)
	}

	// A second, unrelated ledger world: same directory shapes, a
	// different adoption identity.
	foreignOrigin, c, _ := twoClones(t)
	foreignRoot := vRoot()
	foreignRoot.Identity = "01J5XFFFFFFFFFFFFFFFFFFFFF"
	foreignFiles := vTree(foreignRoot, []*GoalFile{vGoal("foreign", StateQueued)}, nil)
	var changes []Change
	for p, content := range foreignFiles {
		changes = append(changes, Change{Path: p, Content: content})
	}
	res, err := Publish(endpointFor(c), PublishRequest{
		Opid: "op-foreign", Machine: "mac-c", Lineage: "l1",
		Intent: testIntentFor("open"), Message: "goal open foreign",
		Mutate: func(tip string) ([]Change, error) { return changes, nil },
	})
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("foreign world publish: %+v %v", res, err)
	}

	// Re-pointing A's config at the foreign remote must not let the
	// fetch silently select another ledger.
	mustGit(t, a, "remote", "set-url", "origin", foreignOrigin)
	_, err = FetchAdvance(endpointFor(a))
	if err == nil || !strings.Contains(err.Error(), "foreign ledger") {
		t.Fatalf("a foreign ledger refuses by name: %v", err)
	}
}

// writeInWorktree writes one file inside a clone's worktree.
func writeInWorktree(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
