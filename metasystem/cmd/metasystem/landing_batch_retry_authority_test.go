package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
)

func TestGLEBatchMovedRetryRejectsRevisedMemberBeforePublication(t *testing.T) {
	const id = "01j5x00000000000000000ba21"
	root, peer, origin, baseCommit, baseTree, tip := movedBatchGitFixture(t)
	observed := time.Now().UTC().Truncate(time.Second)
	opened := observed.Add(-time.Minute).Format(time.RFC3339)
	budget, err := goal.NewBudget("100h", 10, 1000, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	file := &goal.GoalFile{Id: "goal-a", State: goal.StateClaimed, Intent: "land goal-a", Origin: goal.OriginHuman,
		OpenedAt: opened, Revision: 2, Budget: &budget,
		Claimed: &goal.ClaimRecord{Machine: "landing", Lineage: "owner", At: opened, Revision: 2,
			AccountingRevision: 2, EpisodeAt: opened, EpisodeRevision: 2,
			HandedOver: goal.HandedOver{FromMachine: "seat", FromLineage: "seat-lineage", FromEpoch: 1, Batch: id}},
		History: []goal.HistoryLine{
			{At: opened, Opid: "01J5X0000000000000000000BY-human-1a2b3c4d", Verb: "open", Actor: "human:wido", Keep: -1},
			{At: opened, Opid: "01J5X0000000000000000000B1-landing-1a2b3c4d", Verb: "claim", Actor: "landing+owner", Keep: -1},
		},
	}
	runBatchFixtureGit(t, root, "checkout", "-q", "-B", "accepted-ledger", baseCommit)
	runBatchFixtureGit(t, root, "rm", "-q", "plans/goals/ledger.md")
	rootFile := filepath.Join(root, "plans", "goals", "backlog.md")
	goalFile := filepath.Join(root, "plans", "goals", "goal-a.md")
	if err := os.MkdirAll(filepath.Dir(rootFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rootFile, goal.RenderRoot(&goal.RootRecord{
		Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "1", SyncMode: goal.SyncRemote, Revision: 1,
	}), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(goalFile, goal.RenderFile(file), 0o644); err != nil {
		t.Fatal(err)
	}
	runBatchFixtureGit(t, root, "add", "plans/goals")
	runBatchFixtureGit(t, root, "commit", "-qm", "accepted goal claim")
	runBatchFixtureGit(t, root, "update-ref", goal.AcceptedRef, "HEAD")
	// The production authority reader fetches the canonical endpoint before
	// every publication boundary. Make this claim an actual origin ancestor.
	runBatchFixtureGit(t, root, "push", "-q", "origin", "HEAD:refs/heads/main")
	runBatchFixtureGit(t, root, "checkout", "-q", "landing/"+id)
	runBatchFixtureGit(t, peer, "fetch", "-q", "origin", "main")
	runBatchFixtureGit(t, peer, "checkout", "-q", "-B", "main", "origin/main")
	if err := os.MkdirAll(filepath.Join(peer, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(peer, "docs", "trunk.txt"), []byte("moved\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runBatchFixtureGit(t, peer, "add", "docs/trunk.txt")
	runBatchFixtureGit(t, peer, "commit", "-qm", "move disjoint trunk path")
	runBatchFixtureGit(t, peer, "push", "-q", "origin", "main")
	movedCommit := strings.TrimSpace(runBatchFixtureGit(t, peer, "rev-parse", "HEAD"))
	candidateTree := strings.TrimSpace(runBatchFixtureGit(t, root, "rev-parse", tip+"^{tree}"))
	claim := batch.Claim{Machine: "seat", Lineage: "seat-lineage", Epoch: 1, Revision: 2, AccountingRevision: 2}
	record := batch.Record{Schema: 1, BatchID: id, State: batch.StateLanding, BaseTree: baseTree,
		PrefixTrees: []string{candidateTree}, TipTree: candidateTree,
		Units:   []batch.Unit{{GoalID: "goal-a", Chain: "chain-a", Claim: claim, State: batch.UnitJoined}},
		Seal:    map[string]batch.Claim{"goal-a": {Revision: 2, AccountingRevision: 2}},
		Proof:   &batch.Proof{Status: "green", AttemptID: "tip-proof", SelectedGroups: []string{"selected"}, InputManifests: map[string][]string{"selected": {"source/**"}}},
		Landing: &batch.LandingProgress{Base: baseTree, BranchTip: tip, Commits: map[string]string{}},
		History: []batch.HistoryEntry{{At: observed.Format(time.RFC3339Nano), To: batch.StateLanding, Actor: "owner"}},
	}
	store := batch.NewStore(root, nil)
	if err := store.Create(record); err != nil {
		t.Fatal(err)
	}
	if err := authorizeBatchMember(root, record, record.Units[0]); err != nil {
		t.Fatalf("initial live claim was not authorized: %v", err)
	}
	originalChild, originalVerify, originalPush := batchChildRunner, batchVerifyRebasedSeries, batchMovedEndpointPush
	t.Cleanup(func() {
		batchChildRunner, batchVerifyRebasedSeries, batchMovedEndpointPush = originalChild, originalVerify, originalPush
	})
	batchChildRunner = func(string, string, ...string) error { return nil }
	verified, pushed, firstAttempt := 0, 0, 0
	revisedCommit := ""
	batchVerifyRebasedSeries = func(_ string, _ batch.Record, trees []string) error {
		verified++
		if len(trees) != 1 || trees[0] == candidateTree {
			t.Fatalf("rebased prefix was not replanned: %v", trees)
		}
		return nil
	}
	batchMovedEndpointPush = func(root, id, base, tip string) error {
		pushed++
		return originalPush(root, id, base, tip)
	}
	seams := batchLandSeams(root, id, record, baseCommit, "owner")
	seams.Prepare = func(string) error { return nil }
	seams.Apply = func(batch.Unit) error { return nil }
	seams.AppendReceipt = func(batch.Unit, batch.PrefixReceipt) error { return nil }
	seams.Commit = func(batch.Unit, batch.PrefixReceipt) (string, error) { return tip, nil }
	seams.Held = func(string, string) error { return nil }
	seams.VerifySeries = func([]batch.Unit, map[string]string) error {
		return authorizeBatchMember(root, record, record.Units[0])
	}
	seams.Push = func(string, string) error {
		firstAttempt++
		if firstAttempt != 1 {
			t.Fatal("unexpected second ordinary endpoint push")
		}
		runBatchFixtureGit(t, root, "checkout", "-q", "accepted-ledger")
		runBatchFixtureGit(t, root, "fetch", "-q", "origin", "main")
		runBatchFixtureGit(t, root, "rebase", "-q", "origin/main")
		file.Revision, file.Claimed.Revision, file.Claimed.AccountingRevision, file.Claimed.EpisodeRevision = 3, 3, 3, 3
		file.Claimed.At, file.Claimed.EpisodeAt = observed.Format(time.RFC3339), observed.Format(time.RFC3339)
		file.History = append(file.History, goal.HistoryLine{At: observed.Format(time.RFC3339),
			Opid: "01J5X0000000000000000000B2-landing-1a2b3c4d", Verb: "revise", Actor: "landing+owner", Keep: -1})
		if err := os.WriteFile(goalFile, goal.RenderFile(file), 0o644); err != nil {
			t.Fatal(err)
		}
		runBatchFixtureGit(t, root, "add", "plans/goals/goal-a.md")
		runBatchFixtureGit(t, root, "commit", "-qm", "revise handed-over goal")
		runBatchFixtureGit(t, root, "update-ref", goal.AcceptedRef, "HEAD")
		revisedCommit = strings.TrimSpace(runBatchFixtureGit(t, root, "rev-parse", "HEAD"))
		runBatchFixtureGit(t, root, "push", "-q", "origin", "HEAD:refs/heads/main")
		runBatchFixtureGit(t, root, "checkout", "-q", "landing/"+id)
		return batch.LandLandingBranch(root, id, baseCommit, tip)
	}
	err = batch.LandSeries(store, id, "owner", observed, seams)
	var refused *batch.PrefixRevisionRefusal
	if !errors.As(err, &refused) {
		t.Fatalf("revised member refusal=%T %v; retry pushes=%d, endpoint=%s", err, err, pushed,
			strings.TrimSpace(runBatchFixtureGit(t, origin, "rev-parse", "refs/heads/main")))
	}
	landed, err := store.Load(id)
	if err != nil {
		t.Fatal(err)
	}
	mainTip := strings.TrimSpace(runBatchFixtureGit(t, origin, "rev-parse", "refs/heads/main"))
	if verified != 1 || firstAttempt != 1 || pushed != 0 || revisedCommit == "" || mainTip != revisedCommit || movedCommit == revisedCommit || landed.State != batch.StateDissolved ||
		len(landed.Units) != 1 || landed.Units[0].State != batch.UnitReturnPending || landed.Units[0].Outcome != batch.UnitEjected || landed.Landing != nil {
		t.Fatalf("retry escaped refusal: verified=%d initialPush=%d retryPush=%d origin=%s record=%+v", verified, firstAttempt, pushed, mainTip, landed)
	}
}
