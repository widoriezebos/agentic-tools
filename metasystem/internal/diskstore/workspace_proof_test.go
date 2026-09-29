package diskstore

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func (b *workspaceBed) sweep(ended map[string]bool, known bool) Report {
	b.t.Helper()
	proof := WorkspaceProof{GitRoot: b.git.gitRoot, Git: b.git.run, Ended: func(owner Owner) (bool, bool, string) { return ended[owner.Ref], known, "the ledger at abc" }}
	report, err := RunPass(context.Background(), PassOptions{Kind: "checkout", Name: b.control, Registry: b.registry, Mode: ModeApply,
		Now: testNow, Clock: func() time.Time { return testNow },
		Classes:      []Class{RegisteredStores{Registry: b.registry, Proofs: map[OwnerKind]OwnerProof{OwnerGoal: proof}}},
		CensusReader: &CensusReader{Pids: func() ([]int64, error) { return nil, nil }}})
	if err != nil {
		b.t.Fatal(err)
	}
	return report
}

// The sweeper releases a concluded goal's workspaces through the same
// release as the verb (the workspace row of 3.1, U6b): a clean copy is
// archived and removed, a plain one removed; an open goal's workspace is
// kept naming --release; a dirty copy of a concluded goal is kept naming
// the discard; an unknown goal state is pending; goalbranch.Sweep is never
// involved.
func TestSweeperReleasesAConcludedGoalsWorkspaces(t *testing.T) {
	t.Parallel()
	bed := newWorkspaceBed(t)
	copied := bed.obtain(goalG, "bed", "abc123")
	plain := bed.obtain(goalG, "", "")
	open := bed.obtain(Owner{Kind: OwnerGoal, Ref: "open"}, "", "")
	dirty := bed.obtain(Owner{Kind: OwnerGoal, Ref: "done-dirty"}, "bed", "abc123")
	bed.git.dirty[dirty.Record.Path] = " M x\n"

	if report := bed.sweep(nil, false); len(report.Actions) != 0 || len(report.Pending) != 4 {
		t.Fatalf("an unknown goal state releases nothing: %+v", report)
	}
	report := bed.sweep(map[string]bool{"g": true, "done-dirty": true}, true)
	if len(report.Actions) != 2 {
		t.Fatalf("the concluded goal's two workspaces are released: %+v", report)
	}
	for _, path := range []string{copied.Record.Path, plain.Record.Path} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("%s is gone: %v", path, err)
		}
	}
	if bed.git.refs["refs/archive/goal-g/bed/workspace/goal-g/bed"] != "abc123" {
		t.Fatalf("the copy's tip is archived first: %v", bed.git.refs)
	}
	var keptOpen, keptDirty bool
	for _, line := range report.Kept {
		keptOpen = keptOpen || line.Path == open.Record.Path && strings.Contains(line.Command, "metasystem work workspace open --release")
		keptDirty = keptDirty || line.Path == dirty.Record.Path && strings.Contains(line.Command, "--release --discard")
	}
	if !keptOpen || !keptDirty {
		t.Fatalf("kept lines: %+v", report.Kept)
	}
	record, _ := bed.registry.Load(copied.Record.ID)
	if record.State != StateReleased || !strings.Contains(strings.Join(record.Notes, " "), "archived") ||
		!strings.Contains(strings.Join(record.Notes, " "), "read from the ledger at abc") {
		t.Fatalf("record = %+v", record)
	}
}
