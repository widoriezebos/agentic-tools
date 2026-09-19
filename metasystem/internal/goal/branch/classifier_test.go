package branch_test

import (
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

func TestGoalBranchPushUsesLedgerFailureClassification(t *testing.T) {
	for index, output := range []string{
		"stale info",
		"[rejected] main -> main (fetch first)",
		"[remote rejected] main -> main (failed to update ref: cannot lock ref but expected abc)",
		"connection closed without a result",
	} {
		bin := t.TempDir()
		script := filepath.Join(bin, "git")
		if err := testexec.WriteFile(script, []byte("#!/bin/sh\nprintf '%s\\n' \"$PUSH_FAILURE\" >&2\nexit 1\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", bin)
		t.Setenv("PUSH_FAILURE", output)
		got, _ := (branch.GitPushTransport{}).Push("repo", "origin", "refs/heads/main", "old", "new")
		want := branch.CASOutcome(goal.ClassifyPushFailure(output))
		if got != want {
			t.Fatalf("case %d output %q classified %s, want shared %s", index, output, got, want)
		}
	}
}
