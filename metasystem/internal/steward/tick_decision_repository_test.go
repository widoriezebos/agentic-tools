package steward

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// decisionTickRepository keeps accepted goal bytes separate from the real
// evidence, intent, and notification stores used by the decision tests.
type decisionTickRepository struct {
	t     *testing.T
	root  string
	head  string
	files map[string][]byte
}

func newDecisionTickRepository(t *testing.T) *decisionTickRepository {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	writeStewardRecord(t, ledgerAttentionStatePath(root), map[string]any{
		"schema": ledgerAttentionStateSchema, "lastOutcome": "local",
	})
	rootRecord := &goal.RootRecord{
		Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "1",
		SyncMode: goal.SyncLocal, Revision: 1,
	}
	file := &goal.GoalFile{
		Id: "fix-it", State: goal.StateClaimed, Intent: "Repair the thing", Origin: "main",
		NextStep: "Repair it.", OpenedAt: "2026-08-23T00:00:00Z", Revision: 2,
		Claimed: &goal.ClaimRecord{Machine: "bed-m1", Lineage: "coordinator", At: "2026-08-23T01:00:00Z"},
		History: bedHistory("fix-it", "claim"),
	}
	return &decisionTickRepository{t: t, root: root, head: "1111111111111111111111111111111111111111",
		files: map[string][]byte{
			"plans/goals/backlog.md": goal.RenderRoot(rootRecord),
			"plans/goals/fix-it.md":  goal.RenderFile(file),
		}}
}

func (b *decisionTickRepository) declareProgress() {
	b.head = "2222222222222222222222222222222222222222"
}

func (b *decisionTickRepository) declareGoalFree() {
	rootRecord := &goal.RootRecord{
		Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "1",
		SyncMode: goal.SyncLocal, Revision: 2,
		Free: &goal.FreeRecord{Declared: "2026-08-23T02:00:00Z", Origin: "main", Digest: "abc123"},
	}
	b.files = map[string][]byte{"plans/goals/backlog.md": goal.RenderRoot(rootRecord)}
}

func (b *decisionTickRepository) acceptedTip() string {
	sum := sha256.New()
	_, _ = sum.Write(b.files["plans/goals/backlog.md"])
	_, _ = sum.Write(b.files["plans/goals/fix-it.md"])
	return hex.EncodeToString(sum.Sum(nil))
}

func (b *decisionTickRepository) currentMarks() Marks {
	b.t.Helper()
	type reply struct {
		args  []string
		value string
	}
	replies := []reply{
		{[]string{"rev-parse", "HEAD"}, b.head},
		{[]string{"rev-parse", "--verify", "--quiet", "refs/metasystem/goals/accepted"}, b.acceptedTip()},
	}
	readCount := 0
	marks, err := currentMarksWithReader(b.root, func(root string, args ...string) ([]byte, error) {
		if root != b.root || readCount >= len(replies) {
			b.t.Fatalf("unexpected mark read: root=%q args=%v", root, args)
		}
		want := replies[readCount]
		readCount++
		if !reflect.DeepEqual(args, want.args) {
			b.t.Fatalf("mark read %d: args=%v, want %v", readCount, args, want.args)
		}
		return []byte(want.value + "\n"), nil
	})
	if err != nil || readCount != len(replies) {
		b.t.Fatalf("mark replies: consumed %d of %d: %v", readCount, len(replies), err)
	}
	return marks
}

func (b *decisionTickRepository) openWorkDependencies() (openWorkDependencies, func()) {
	b.t.Helper()
	routed := false
	readCount := 0
	dependencies := openWorkDependencies{
		NewWorld: func(root string) bool {
			if root != b.root || routed || readCount != 0 {
				b.t.Fatalf("unexpected goal world route: root=%q routed=%t reads=%d", root, routed, readCount)
			}
			routed = true
			return true
		},
		ReadClaimableBudgetedWork: func(root string, now time.Time) (goal.ClaimableBudgetedWork, error) {
			if root != b.root || !routed || readCount != 0 || now.IsZero() {
				b.t.Fatalf("unexpected accepted goal read: root=%q routed=%t reads=%d time=%s", root, routed, readCount, now)
			}
			readCount++
			tree, problems := goal.ParseTreeFiles(b.files)
			if len(problems) != 0 {
				b.t.Fatalf("declared accepted goal files did not parse: %v", problems)
			}
			return goal.ClaimableWorkFromProjection(goal.Projection{
				Root: b.root, Tree: tree, Horizon: goal.ApprovalHorizon{Now: now},
			}, "bed-m1", identity.KernelProber{})
		},
	}
	return dependencies, func() {
		if !routed || readCount != 1 {
			b.t.Fatalf("accepted goal replies: routed=%t consumed=%d of 1", routed, readCount)
		}
	}
}

func (b *decisionTickRepository) tickN(cfg TickConfig, census WorkerCensus, n int) TickResult {
	b.t.Helper()
	var last TickResult
	for i := 0; i < n; i++ {
		path := EvidencePath(b.root)
		prev, err := LoadEvidence(path)
		if err != nil {
			b.t.Fatal(err)
		}
		marks := b.currentMarks()
		dependencies, checkReads := b.openWorkDependencies()
		last, err = decideTickWithDependencies(b.root, cfg, census, prev, marks, dependencies)
		checkReads()
		if err != nil {
			b.t.Fatal(err)
		}
		if err := SaveEvidence(b.root, path, last.Evidence); err != nil {
			b.t.Fatal(err)
		}
	}
	return last
}
