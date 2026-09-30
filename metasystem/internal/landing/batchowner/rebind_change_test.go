package batchowner

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/ownercall"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
)

const rebindBatchID = "01j5x00000000000000000rb01"

// rebindLedgerTree commits one claimed goal file, handed over to the batch
// by a seat, into a fresh repository and returns the root and its tree.
func rebindLedgerTree(t *testing.T, goalID string) (string, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "plans", "goals"), 0o755); err != nil {
		t.Fatal(err)
	}
	file := &goal.GoalFile{Id: goalID, State: goal.StateClaimed, Intent: "land " + goalID, Origin: goal.OriginHuman, OpenedAt: "2026-09-17T10:00:00Z", Revision: 2,
		Claimed: &goal.ClaimRecord{Machine: "seat", Lineage: "seat-lineage", At: "2026-09-17T10:00:00Z", Revision: 2, AccountingRevision: 1,
			HandedOver: goal.HandedOver{FromMachine: "seat", FromLineage: "seat-lineage", FromEpoch: 1, Batch: rebindBatchID}},
		History: []goal.HistoryLine{{At: "2026-09-17T10:00:00Z", Opid: "01J5X0000000000000000000BY-human-1a2b3c4d", Verb: "open", Actor: "human:wido", Keep: -1},
			{At: "2026-09-17T10:00:00Z", Opid: "01J5X0000000000000000000B1-seat-1a2b3c4d", Verb: "claim", Actor: "seat+seat-lineage", Keep: -1}}}
	if err := os.WriteFile(filepath.Join(root, "plans", "goals", goalID+".md"), goal.RenderFile(file), 0o644); err != nil {
		t.Fatal(err)
	}
	script := `set -eu; cd "$1"; git init -q -b main; git config user.name Test; git config user.email test@example.com; git add plans; git commit -qm ledger`
	command := exec.Command("bash", "-c", script, "rebind-fixture", root)
	command.Env = gittree.ScrubbedEnviron()
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("fixture: %v: %s", err, out)
	}
	tree := exec.Command("git", "-C", root, "rev-parse", "HEAD^{tree}")
	tree.Env = gittree.ScrubbedEnviron()
	out, err := tree.Output()
	if err != nil {
		t.Fatal(err)
	}
	if _, problems := goal.ParseFile(goal.RenderFile(file)); len(problems) != 0 {
		t.Fatalf("goal fixture is invalid: %v", problems)
	}
	return root, strings.TrimSpace(string(out))
}

// A change member holds no ledger claim: its authority is its own commit.
// The real rebind, reading the real ledger at a real tree, hands the goal
// member over to the owner and passes the change member by; before the
// guard, the change's absent goal file failed every owner tick.
func TestRebindBatchClaimsSkipsChangeMembersWithTheRealLedger(t *testing.T) {
	t.Parallel()
	root, tree := rebindLedgerTree(t, "goal-a")
	change := batch.NewChangeUnit(batch.ChangeMember{Commit: strings.Repeat("5", 40), Parent: strings.Repeat("4", 40), AskedBy: "seat+seat-lineage", Subject: "a change"},
		root, "seat", "seat-lineage", []string{"a.go"}, nil)
	change.State = batch.UnitJoined
	store := batch.NewStore(root, nil)
	if err := store.Create(batch.Record{Schema: 1, BatchID: rebindBatchID, State: batch.StateOpen, Units: []batch.Unit{
		change,
		{GoalID: "goal-a", Chain: "chain-a", Claim: batch.Claim{Machine: "seat", Lineage: "seat-lineage", Epoch: 1, Revision: 2, AccountingRevision: 1}, State: batch.UnitJoined},
	}}); err != nil {
		t.Fatal(err)
	}
	var handed []ownercall.HandoverRequest
	err := RebindBatchClaims(root, rebindBatchID, tree, "landing-machine", 3, batch.ReadReturnLedgerGoal, func(request ownercall.HandoverRequest) error {
		handed = append(handed, request)
		return nil
	})
	if err != nil {
		t.Fatalf("rebind of a batch holding a change failed: %v", err)
	}
	if len(handed) != 1 || handed[0].GoalID != "goal-a" || handed[0].TargetMachine != "landing-machine" || handed[0].TargetLineage != LandingOwnerLineage ||
		handed[0].TargetEpoch != 3 || handed[0].Batch != rebindBatchID {
		t.Fatalf("handovers=%+v, want only goal-a to the owner at epoch 3", handed)
	}

	// A batch of changes alone rebinds nothing and never fails.
	onlyChange := "01j5x00000000000000000rc01"
	if err := store.Create(batch.Record{Schema: 1, BatchID: onlyChange, State: batch.StateOpen, Units: []batch.Unit{change}}); err != nil {
		t.Fatal(err)
	}
	handed = nil
	if err := RebindBatchClaims(root, onlyChange, tree, "landing-machine", 3, batch.ReadReturnLedgerGoal, func(request ownercall.HandoverRequest) error {
		handed = append(handed, request)
		return nil
	}); err != nil || len(handed) != 0 {
		t.Fatalf("change-only rebind handovers=%+v error=%v", handed, err)
	}
}
