package validate

import (
	"os"
	"path/filepath"
	"testing"
)

// dispatch-fixtures.sh lines 4285-4312: the review protects trusted plans/
// state whether the delegate left it untracked, committed it on the
// worktree branch, or committed it and changed it again, even when the
// round's boundary declares it; and the agent control plane refuses
// delegate-created files. Real Git is the claim: the reviewed diff is the
// worktree's snapshot against its base, which committed history reaches.
func TestConformanceIntegrationProtectsPlansAcrossCommittedAndUncommittedStates(t *testing.T) {
	t.Parallel()
	f := newConformanceFixture(t)
	appendFile(t, filepath.Join(f.worktree, "source.txt"), "untracked change\n")
	f.writeImplementer("", "source.txt", "plans/delegate.md")
	if err := os.MkdirAll(filepath.Join(f.worktree, "plans"), 0o755); err != nil {
		t.Fatal(err)
	}
	appendFile(t, filepath.Join(f.worktree, "plans", "delegate.md"), "delegate plan\n")
	expectConformance(t, f, "review", 1, "trusted plans/ state changed: plans/delegate.md")

	f.commitWorktree()
	expectConformance(t, f, "review", 1, "trusted plans/ state changed: plans/delegate.md")

	appendFile(t, filepath.Join(f.worktree, "plans", "delegate.md"), "uncommitted change\n")
	expectConformance(t, f, "review", 1, "trusted plans/ state changed: plans/delegate.md")

	if err := os.MkdirAll(filepath.Join(f.worktree, "artifacts", "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	appendFile(t, filepath.Join(f.worktree, "artifacts", "agents", "tamper"), "tamper\n")
	expectConformance(t, f, "review", 1, "agent control plane contains delegate-created files")
}
