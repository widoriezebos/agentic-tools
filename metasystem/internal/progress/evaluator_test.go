package progress

import (
	"os"
	"path/filepath"
	"testing"
)

// Each declared root gets its own launch-time standing: a contained root keeps
// liveness beside an outside or excluded sibling, a missing tail inside the
// worktree is still contained, a delegate worktree under artifacts/agents is
// its own product, and a shared checkout makes every root attribution-only.
func TestCaptureProductRootScopesFixesEachRootsLaunchStanding(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	workspace := filepath.Join(base, "worktree")
	inside := filepath.Join(workspace, "product")
	missingTail := filepath.Join(workspace, "later", "future")
	excluded := filepath.Join(workspace, "artifacts", "agents", "private")
	outside := filepath.Join(base, "shared-product")
	for _, dir := range []string{inside, excluded, outside} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	scopes, err := CaptureProductRootScopes(LaunchModeWorktree, workspace, []string{inside, missingTail, excluded, outside})
	if err != nil {
		t.Fatal(err)
	}
	want := []ProductRootScope{
		{Path: inside, Standing: StandingLiveness, Reason: ReasonContainedAtLaunch},
		{Path: missingTail, Standing: StandingLiveness, Reason: ReasonContainedAtLaunch},
		{Path: excluded, Standing: StandingAttributionOnly, Reason: ReasonExcludedAtLaunch},
		{Path: outside, Standing: StandingAttributionOnly, Reason: ReasonOutsideWorktreeAtLaunch},
	}
	if len(scopes) != len(want) {
		t.Fatalf("worktree scopes = %+v, want %+v", scopes, want)
	}
	for i := range want {
		if scopes[i] != want[i] {
			t.Fatalf("worktree scope %d = %+v, want %+v", i, scopes[i], want[i])
		}
	}

	delegate := filepath.Join(base, "repository", "artifacts", "agents", "worktrees", "job-a")
	if err := os.MkdirAll(delegate, 0o755); err != nil {
		t.Fatal(err)
	}
	scopes, err = CaptureProductRootScopes(LaunchModeWorktree, delegate, []string{delegate})
	if err != nil || len(scopes) != 1 || scopes[0].Standing != StandingLiveness {
		t.Fatalf("delegate worktree scopes = %+v, %v; want liveness", scopes, err)
	}

	scopes, err = CaptureProductRootScopes(LaunchModeSharedCheckout, workspace, []string{inside})
	if err != nil || len(scopes) != 1 || scopes[0] != (ProductRootScope{Path: inside, Standing: StandingAttributionOnly, Reason: ReasonSharedCheckout}) {
		t.Fatalf("shared-checkout scopes = %+v, %v; want attribution-only", scopes, err)
	}
}

func TestCaptureProductRootScopesRefusesAnUnboundLaunch(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	for name, call := range map[string]func() error{
		"unknown mode": func() error { _, err := CaptureProductRootScopes("elsewhere", workspace, nil); return err },
		"no workspace": func() error { _, err := CaptureProductRootScopes(LaunchModeWorktree, "", nil); return err },
		"relative root": func() error {
			_, err := CaptureProductRootScopes(LaunchModeWorktree, workspace, []string{"product"})
			return err
		},
		"empty root": func() error {
			_, err := CaptureProductRootScopes(LaunchModeWorktree, workspace, []string{""})
			return err
		},
	} {
		if call() == nil {
			t.Fatalf("%s was accepted", name)
		}
	}
}
