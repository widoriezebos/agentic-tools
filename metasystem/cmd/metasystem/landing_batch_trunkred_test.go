package main

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
)

func TestLedgerTrunkRedOwnerRecordsIdempotently(t *testing.T) {
	root := syncedClaimedGoalFixture(t)
	const machine, lineage = "mac-landing", "landing-lineage"
	ownerValue, err := newLedgerTrunkRedOwner(root, machine, lineage)
	if err != nil || !isLedgerTrunkRedOwner(ownerValue) {
		t.Fatalf("construct owner: %T %v", ownerValue, err)
	}
	owner := ownerValue.(*ledgerTrunkRedOwner)
	owner.now = func() time.Time { return time.Date(2026, 9, 17, 11, 0, 0, 0, time.UTC) }
	red := batch.TrunkRed{BatchID: "batch-1", AttemptID: "attempt-1", BaseCommit: "base-1", BaseTree: "tree-1",
		SeenAt: time.Date(2026, 9, 17, 10, 0, 0, 0, time.UTC), Joiners: []batch.Claim{{Machine: machine}},
		Groups: []batch.RedGroup{{ID: "fast", Status: "failed", Failures: []batch.Failure{{Report: "report", Classname: "Class", Name: "Test", Status: "failed", Reason: "red"}}}}}
	opid := goal.Opid("01J5X0000000000000000000V1", machine, lineage)
	refs, err := owner.Record(opid, red)
	if err != nil || len(refs) != 1 || refs[0].Group != "fast" {
		t.Fatalf("first record: %+v %v", refs, err)
	}
	afterFirst := goalSyncMutationGit(t, root, "rev-list", "--count", goal.LocalLedgerBranch)
	refs, err = owner.Record(opid, red)
	if err != nil || len(refs) != 1 || goalSyncMutationGit(t, root, "rev-list", "--count", goal.LocalLedgerBranch) != afterFirst {
		t.Fatalf("idempotent replay: %+v %v", refs, err)
	}

	entry, err := goal.ReadEntry(root, opid)
	if err != nil {
		t.Fatal(err)
	}
	entry.Phase, entry.Outcome, entry.Evidence, entry.TerminalAt = goal.PhasePushed, "", "", ""
	writeAdapterJournalEntry(t, root, entry)
	refs, err = owner.Record(opid, red)
	if err != nil || len(refs) != 1 || goalSyncMutationGit(t, root, "rev-list", "--count", goal.LocalLedgerBranch) != afterFirst {
		t.Fatalf("pushed recovery replay: %+v %v", refs, err)
	}

	red2 := red
	red2.BatchID, red2.AttemptID, red2.BaseCommit = "batch-2", "attempt-2", "base-2"
	red2.SeenAt = red.SeenAt.Add(time.Hour)
	opid2 := goal.Opid("01J5X0000000000000000000V2", machine, lineage)
	intent := adapterRecordIntent(red2)
	created, err := goal.CreateEntry(root, opid2, machine, lineage, intent)
	if err != nil {
		t.Fatal(err)
	}
	created.Owner = goal.OwnerIdentity{Pid: 999999999, PidStartedAt: 1}
	writeAdapterJournalEntry(t, root, created)
	refs, err = owner.Record(opid2, red2)
	if err != nil || len(refs) != 1 {
		t.Fatalf("created recovery: %+v %v", refs, err)
	}
	open, err := owner.Open()
	if err != nil || len(open) != 1 || len(open[0].Holds) != 2 || open[0].LastBaseCommit != "base-2" {
		t.Fatalf("open projection: %+v %v", open, err)
	}

	red3 := red
	red3.BatchID, red3.AttemptID = "batch-3", "attempt-3"
	opid3 := goal.Opid("01J5X0000000000000000000V3", machine, lineage)
	if _, err := goal.CreateEntry(root, opid3, machine, lineage, adapterRecordIntent(red3)); err != nil {
		t.Fatal(err)
	}
	if err := goal.MarkTerminal(root, opid3, goal.OutcomeAbandoned, "owner stopped before publication"); err != nil {
		t.Fatal(err)
	}
	beforeFailed := goalSyncMutationGit(t, root, "rev-list", "--count", goal.LocalLedgerBranch)
	_, err = owner.Record(opid3, red3)
	var failed *batch.TrunkRedRecordFailed
	if !errors.As(err, &failed) || failed.Outcome != string(goal.OutcomeAbandoned) || failed.Evidence != "owner stopped before publication" ||
		goalSyncMutationGit(t, root, "rev-list", "--count", goal.LocalLedgerBranch) != beforeFailed {
		t.Fatalf("abandoned mapping: failure=%+v err=%v", failed, err)
	}
	if _, err := owner.Record("old-opid", red); err == nil {
		t.Fatal("an opid outside the landing owner's form was accepted")
	}

	ownOpid := goal.Opid("01J5X0000000000000000000V4", machine, lineage)
	ownRequest, err := owner.request(ownOpid)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := goal.OwnTrunkRed(ownRequest, goal.TrunkRedOwnArgs{Entry: refs[0].ID, Goal: "standing-validation", Branch: "fix/red",
		BranchCommit: "1111111111111111111111111111111111111111"}); err != nil {
		t.Fatal(err)
	}
	clearOpid := goal.Opid("01J5X0000000000000000000V5", machine, lineage)
	greenCommit := goalSyncMutationGit(t, root, "rev-parse", "HEAD")
	greenTree := goalSyncMutationGit(t, root, "rev-parse", "HEAD^{tree}")
	if err := owner.Clear(clearOpid, refs[0], batch.Green{AttemptID: "green-1", BaseCommit: greenCommit, BaseTree: greenTree, Group: "fast"}); err != nil {
		t.Fatal(err)
	}
	closedOpid := goal.Opid("01J5X0000000000000000000V6", machine, lineage)
	if err := owner.Clear(closedOpid, refs[0], batch.Green{AttemptID: "green-2", BaseCommit: greenCommit, BaseTree: greenTree, Group: "fast"}); !batch.IsTrunkRedClosed(err) {
		t.Fatalf("second clear error=%v, want typed already-closed classification", err)
	}
	if open, err := owner.Open(); err != nil || len(open) != 0 {
		t.Fatalf("clear left open entries: %+v %v", open, err)
	}
	projection, err := goal.Project(owner.endpoint, false, owner.now())
	if err != nil || len(projection.Tree.TrunkRed) != 1 || projection.Tree.TrunkRed[0].Closed == nil || projection.Tree.TrunkRed[0].FixBranch.State != goal.TrunkRedBranchOpen {
		t.Fatalf("clear with absent fix commit: entries=%+v error=%v", projection.Tree.TrunkRed, err)
	}
}

func TestLedgerTrunkRedOwnerClearClassifiesFixCommit(t *testing.T) {
	tests := []struct {
		name        string
		merged      bool
		unreadable  bool
		packed      bool
		commitGraph bool
		branchState string
	}{
		{name: "merged", merged: true, branchState: goal.TrunkRedBranchMerged},
		{name: "present but not merged", branchState: goal.TrunkRedBranchOpen},
		{name: "unreadable", merged: true, unreadable: true},
		{name: "unreadable pack", merged: true, unreadable: true, packed: true},
		{name: "unreadable pack commit graph", merged: true, unreadable: true, packed: true, commitGraph: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := syncedClaimedGoalFixture(t)
			const machine, lineage = "mac-landing", "landing-lineage"
			ownerValue, err := newLedgerTrunkRedOwner(root, machine, lineage)
			if err != nil {
				t.Fatal(err)
			}
			owner := ownerValue.(*ledgerTrunkRedOwner)
			owner.now = func() time.Time { return time.Date(2026, 9, 17, 11, 0, 0, 0, time.UTC) }

			tree := goalSyncMutationGit(t, root, "rev-parse", "HEAD^{tree}")
			fixCommit := goalSyncMutationGit(t, root, "commit-tree", tree, "-m", "fix commit")
			greenArgs := []string{"commit-tree", tree, "-m", "green commit for " + test.name}
			if test.merged {
				greenArgs = append(greenArgs, "-p", fixCommit)
			}
			greenCommit := goalSyncMutationGit(t, root, greenArgs...)

			red := batch.TrunkRed{BatchID: "batch-clear", AttemptID: "attempt-red", BaseCommit: fixCommit, BaseTree: tree,
				SeenAt: time.Date(2026, 9, 17, 10, 0, 0, 0, time.UTC), Joiners: []batch.Claim{{Machine: machine}},
				Groups: []batch.RedGroup{{ID: "fast", Status: "failed", Failures: []batch.Failure{{Report: "report", Classname: "Class", Name: "Test", Status: "failed", Reason: "red"}}}}}
			recordOpid := goal.Opid("01J5X0000000000000000000X1", machine, lineage)
			refs, err := owner.Record(recordOpid, red)
			if err != nil || len(refs) != 1 {
				t.Fatalf("record: refs=%+v error=%v", refs, err)
			}
			ownOpid := goal.Opid("01J5X0000000000000000000X2", machine, lineage)
			ownRequest, err := owner.request(ownOpid)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := goal.OwnTrunkRed(ownRequest, goal.TrunkRedOwnArgs{Entry: refs[0].ID, Goal: "standing-validation", Branch: "fix/red", BranchCommit: fixCommit}); err != nil {
				t.Fatal(err)
			}

			if test.unreadable {
				if test.packed {
					prefix := filepath.Join(root, ".git", "objects", "pack", "pack")
					command := exec.Command("git", "-C", root, "pack-objects", prefix)
					command.Env = gittree.ScrubbedEnviron()
					command.Stdin = strings.NewReader(fixCommit + "\n")
					output, err := command.Output()
					if err != nil {
						t.Fatal(err)
					}
					indexPath := prefix + "-" + strings.TrimSpace(string(output)) + ".idx"
					if err := os.Remove(filepath.Join(root, ".git", "objects", fixCommit[:2], fixCommit[2:])); err != nil {
						t.Fatal(err)
					}
					if test.commitGraph {
						goalSyncMutationGit(t, root, "update-ref", "refs/probe/green", greenCommit)
						goalSyncMutationGit(t, root, "commit-graph", "write", "--reachable")
					}
					if err := os.Chmod(indexPath, 0); err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() { _ = os.Chmod(indexPath, 0o600) })
				} else {
					objectPath := filepath.Join(root, ".git", "objects", fixCommit[:2], fixCommit[2:])
					if err := os.Chmod(objectPath, 0); err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() { _ = os.Chmod(objectPath, 0o600) })
				}
			}
			clearOpid := goal.Opid("01J5X0000000000000000000X3", machine, lineage)
			err = owner.Clear(clearOpid, refs[0], batch.Green{AttemptID: "attempt-green", BaseCommit: greenCommit, BaseTree: tree, Group: "fast"})
			if test.unreadable {
				open, openErr := owner.Open()
				if err == nil || !strings.Contains(err.Error(), fixCommit) || openErr != nil || len(open) != 1 {
					t.Fatalf("unreadable fix commit: clear error=%v open=%+v open error=%v", err, open, openErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			projection, err := goal.Project(owner.endpoint, false, owner.now())
			if err != nil || len(projection.Tree.TrunkRed) != 1 || projection.Tree.TrunkRed[0].Closed == nil || projection.Tree.TrunkRed[0].FixBranch.State != test.branchState {
				t.Fatalf("clear: entries=%+v error=%v", projection.Tree.TrunkRed, err)
			}
		})
	}
}

func TestLedgerTrunkRedOwnerClearRefusesChangedProjection(t *testing.T) {
	assertLedgerTrunkRedClearRefusesChange(t, "ownership and branch", func(replacementCommit string) goal.TrunkRedOwnArgs {
		return goal.TrunkRedOwnArgs{Goal: "standing-validation", Branch: "fix/reassigned",
			BranchCommit: replacementCommit, To: "mac-reassigned", By: "Wido"}
	}, "mac-reassigned", true)
}

func TestLedgerTrunkRedOwnerClearRefusesChangedOwner(t *testing.T) {
	assertLedgerTrunkRedClearRefusesChange(t, "ownership", func(string) goal.TrunkRedOwnArgs {
		return goal.TrunkRedOwnArgs{Goal: "standing-validation", To: "mac-reassigned", By: "Wido"}
	}, "mac-reassigned", false)
}

func TestLedgerTrunkRedOwnerClearRefusesChangedBranch(t *testing.T) {
	assertLedgerTrunkRedClearRefusesChange(t, "branch", func(replacementCommit string) goal.TrunkRedOwnArgs {
		return goal.TrunkRedOwnArgs{Goal: "standing-validation", Branch: "fix/reassigned", BranchCommit: replacementCommit}
	}, "mac-landing", true)
}

func assertLedgerTrunkRedClearRefusesChange(t *testing.T, description string,
	change func(string) goal.TrunkRedOwnArgs, wantOwner string, wantReplacementCommit bool,
) {
	t.Helper()
	owner, ref, green, replacementCommit := ledgerTrunkRedClearFixture(t)
	owner.beforeClearTransaction = func() error {
		changeOpid := goal.Opid("01J5X0000000000000000000Y3", owner.actor.Machine, owner.actor.Lineage)
		request, err := owner.request(changeOpid)
		if err != nil {
			return err
		}
		args := change(replacementCommit)
		args.Entry = ref.ID
		request.Actor.Human = args.By
		_, err = goal.OwnTrunkRed(request, args)
		return err
	}

	clearOpid := goal.Opid("01J5X0000000000000000000Y4", owner.actor.Machine, owner.actor.Lineage)
	err := owner.Clear(clearOpid, ref, green)
	if err == nil || !strings.Contains(err.Error(), "TRUNK_RED_CHANGED") {
		t.Fatalf("clear after %s change error=%v, want TRUNK_RED_CHANGED", description, err)
	}
	projection, projectErr := goal.Project(owner.endpoint, false, owner.now())
	if projectErr != nil || len(projection.Tree.TrunkRed) != 1 {
		t.Fatalf("read %s-mutated entry: entries=%+v error=%v", description, projection.Tree.TrunkRed, projectErr)
	}
	entry := projection.Tree.TrunkRed[0]
	wrongCommit := wantReplacementCommit && entry.FixBranch.Commit != replacementCommit ||
		!wantReplacementCommit && entry.FixBranch.Commit == replacementCommit
	if entry.Closed != nil || entry.Owner.Machine != wantOwner || wrongCommit {
		t.Fatalf("refused clear changed the %s-mutated entry: entries=%+v error=%v", description, projection.Tree.TrunkRed, projectErr)
	}
}

func TestLedgerTrunkRedOwnerClearAcceptsUnchangedProjection(t *testing.T) {
	owner, ref, green, _ := ledgerTrunkRedClearFixture(t)
	clearOpid := goal.Opid("01J5X0000000000000000000Y4", owner.actor.Machine, owner.actor.Lineage)
	if err := owner.Clear(clearOpid, ref, green); err != nil {
		t.Fatalf("unchanged clear: %v", err)
	}
	projection, err := goal.Project(owner.endpoint, false, owner.now())
	if err != nil || len(projection.Tree.TrunkRed) != 1 || projection.Tree.TrunkRed[0].Closed == nil ||
		projection.Tree.TrunkRed[0].FixBranch.State != goal.TrunkRedBranchMerged {
		t.Fatalf("unchanged clear result: entries=%+v error=%v", projection.Tree.TrunkRed, err)
	}
}

func ledgerTrunkRedClearFixture(t *testing.T) (*ledgerTrunkRedOwner, batch.EntryRef, batch.Green, string) {
	t.Helper()
	root := syncedClaimedGoalFixture(t)
	const machine, lineage = "mac-landing", "landing-lineage"
	ownerValue, err := newLedgerTrunkRedOwner(root, machine, lineage)
	if err != nil {
		t.Fatal(err)
	}
	owner := ownerValue.(*ledgerTrunkRedOwner)
	owner.now = func() time.Time { return time.Date(2026, 9, 18, 11, 0, 0, 0, time.UTC) }
	tree := goalSyncMutationGit(t, root, "rev-parse", "HEAD^{tree}")
	fixCommit := goalSyncMutationGit(t, root, "commit-tree", tree, "-m", "fix commit")
	greenCommit := goalSyncMutationGit(t, root, "commit-tree", tree, "-p", fixCommit, "-m", "green commit")
	replacementCommit := goalSyncMutationGit(t, root, "commit-tree", tree, "-m", "replacement fix commit")
	red := batch.TrunkRed{BatchID: "batch-clear-race", AttemptID: "attempt-red", BaseCommit: fixCommit, BaseTree: tree,
		SeenAt: time.Date(2026, 9, 18, 10, 0, 0, 0, time.UTC), Joiners: []batch.Claim{{Machine: machine}},
		Groups: []batch.RedGroup{{ID: "fast", Status: "failed", Failures: []batch.Failure{{Report: "report", Classname: "Class", Name: "Test", Status: "failed", Reason: "red"}}}}}
	recordOpid := goal.Opid("01J5X0000000000000000000Y1", machine, lineage)
	refs, err := owner.Record(recordOpid, red)
	if err != nil || len(refs) != 1 {
		t.Fatalf("record clear fixture: refs=%+v error=%v", refs, err)
	}
	ownOpid := goal.Opid("01J5X0000000000000000000Y2", machine, lineage)
	ownRequest, err := owner.request(ownOpid)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := goal.OwnTrunkRed(ownRequest, goal.TrunkRedOwnArgs{Entry: refs[0].ID, Goal: "standing-validation",
		Branch: "fix/red", BranchCommit: fixCommit}); err != nil {
		t.Fatal(err)
	}
	green := batch.Green{AttemptID: "attempt-green", BaseCommit: greenCommit, BaseTree: tree, Group: "fast"}
	return owner, refs[0], green, replacementCommit
}

func adapterRecordIntent(red batch.TrunkRed) goal.Intent {
	args := goal.TrunkRedRecordArgs{Batch: red.BatchID, Attempt: red.AttemptID, BaseCommit: red.BaseCommit,
		BaseTree: red.BaseTree, SeenAt: red.SeenAt.UTC().Truncate(time.Second).Format(time.RFC3339), OwnerMachine: red.OwnerMachine()}
	for _, group := range red.Groups {
		converted := goal.TrunkRedRecordGroup{Identity: batch.TrunkRedID(group), Group: group.ID, Status: group.Status,
			NotRunReason: group.NotRunReason, LogPath: group.LogPath, LogDigest: group.LogDigest}
		for _, failure := range group.Failures {
			converted.Failures = append(converted.Failures, goal.TrunkRedFailure{Report: failure.Report, Classname: failure.Classname,
				Name: failure.Name, Status: failure.Status, Reason: failure.Reason})
		}
		if converted.Failures == nil {
			converted.Failures = []goal.TrunkRedFailure{}
		}
		args.Groups = append(args.Groups, converted)
	}
	encoded, _ := json.Marshal(args)
	return goal.Intent{Verb: "trunk-red-record", Args: map[string]string{"red": string(encoded)}}
}

func writeAdapterJournalEntry(t *testing.T, root string, entry goal.Entry) {
	t.Helper()
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "artifacts", "agents", "goal-transactions", entry.Opid+".json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
