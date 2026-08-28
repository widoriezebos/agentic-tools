package goal

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The end-of-turn verdict on a converted checkout: this machine's
// claimed goal has the floor — it blocks once with the next step, and
// repeats as display, exactly the legacy Current-goal contract.
func TestTurnVerdictConvertedClaimHasTheFloor(t *testing.T) {
	root := servingBed(t, "bed-m1", map[string]*GoalFile{
		"ship-it": {
			Id: "ship-it", State: "claimed", Intent: "Ship the whole thing", Origin: "main",
			NextStep: "Land it in pieces.", OpenedAt: "2026-08-23T00:00:00Z", Revision: 2,
			Claimed: &ClaimRecord{Machine: "bed-m1", Lineage: "coordinator", At: "2026-08-23T01:00:00Z"},
		},
	})
	store := &Store{Root: root}
	first, err := store.TurnVerdict(ScanResult{}, "world-session", "", "main-1")
	if err != nil {
		t.Fatal(err)
	}
	if !first.ShouldBlock || !strings.Contains(first.Display, "Land it in pieces") {
		t.Fatalf("the claimed goal must block once with its next step: %+v", first)
	}
	second, err := store.TurnVerdict(ScanResult{}, "world-session", "", "main-1")
	if err != nil {
		t.Fatal(err)
	}
	if second.ShouldBlock || !strings.Contains(second.Display, "ship-it") {
		t.Fatalf("the repeat surfaces as display, not a block: %+v", second)
	}
}

// A queue nobody here claimed prods once toward promotion, naming the
// oldest queued goal.
func TestTurnVerdictConvertedQueueProdsOnce(t *testing.T) {
	root := servingBed(t, "bed-m1", map[string]*GoalFile{
		"older": {
			Id: "older", State: "queued", Intent: "First in line", Origin: "main",
			NextStep: "Work this first.", OpenedAt: "2026-08-20T00:00:00Z", Revision: 1,
		},
		"newer": {
			Id: "newer", State: "queued", Intent: "Second", Origin: "main",
			NextStep: "Work this later.", OpenedAt: "2026-08-22T00:00:00Z", Revision: 1,
		},
	})
	store := &Store{Root: root}
	v, err := store.TurnVerdict(ScanResult{}, "queue-session", "", "main-1")
	if err != nil {
		t.Fatal(err)
	}
	if !v.ShouldBlock || !strings.Contains(v.Display, "older") {
		t.Fatalf("the oldest queued goal must be named in the prod: %+v", v)
	}
}

// A fresh goal-free declaration on the root record is the all-clear.
func TestTurnVerdictConvertedFreshFreeIsAllClear(t *testing.T) {
	root := servingBed(t, "bed-m1", nil)
	scan, err := ScanDigest(root)
	if err != nil {
		t.Fatal(err)
	}
	rewriteBedRoot(t, root, &RootRecord{
		Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "1",
		SyncMode: SyncLocal, Revision: 2,
		Free: &FreeRecord{Declared: "2026-08-23T00:00:00Z", Origin: "human", Digest: scan},
	})
	store := &Store{Root: root}
	v, err := store.TurnVerdict(ScanResult{}, "free-session", "", "main-1")
	if err != nil {
		t.Fatal(err)
	}
	if v.ShouldBlock || !strings.Contains(v.Display, "NOTHING LEFT TO WORK ON") {
		t.Fatalf("a fresh declaration is the all-clear: %+v", v)
	}
}

// rewriteBedRoot replaces the bed's root record and re-accepts the tree.
func rewriteBedRoot(t *testing.T, root string, record *RootRecord) {
	t.Helper()
	abs := filepath.Join(root, "plans", "goals", "backlog.md")
	if err := os.WriteFile(abs, RenderRoot(record), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"add", "plans/goals/backlog.md"},
		{"commit", "-q", "-m", "root rewrite"},
		{"update-ref", AcceptedRef, "HEAD"},
	} {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}
