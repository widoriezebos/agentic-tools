package main

import (
	"errors"
	"strings"
	"testing"

	"context"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/adapter/supervisor"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"os"
	"path/filepath"
)

// The build's goal worktree is a registered store (Part B U5e): recorded
// before git makes it, with nothing inside the tree; the verb holds its
// record lock shared while it runs, so the sweeper's critical section
// cannot open; a worktree whose release has begun is gone, and the verb
// refuses as input.
func TestGoalWorktreePreparationRegistersAndEntersTheStore(t *testing.T) {
	c := newConnectionBed(t)
	owners := c.connectionOwners()
	layout, err := owners.resolver.ResolveLayout(c.root())
	if err != nil {
		t.Fatal(err)
	}
	inv := &intentInvocation{owners: owners, layout: layout, cwd: c.root()}
	path, problem := inv.prepareGoalWorktree(c.id)
	if problem != nil || path != c.worktree {
		t.Fatalf("prepare = %q %+v", path, problem)
	}
	registry := diskstore.CheckoutRegistry(layout.InstallationRoot)
	records, err := diskstore.FindLinkedWorktrees(registry, diskstore.GoalWorktreeClass, diskstore.Owner{Kind: diskstore.OwnerGoal, Ref: c.id})
	if err != nil || len(records) != 1 || records[0].Path != c.worktree || records[0].State != diskstore.StateAccepted || records[0].Identity.Gitdir == "" {
		t.Fatalf("records = %+v, %v", records, err)
	}
	if status := connectionGit(t, c.worktree, "status", "--porcelain=v1", "--untracked-files=all"); status != "" {
		t.Fatalf("registration dirtied the worktree: %q", status)
	}
	var held *diskstore.HeldError
	if _, err := registry.TryCritical(records[0].ID); !errors.As(err, &held) {
		t.Fatalf("while the verb runs the sweeper's section = %v, want held", err)
	}
	inv.leaveStores()
	critical, err := registry.TryCritical(records[0].ID)
	if err != nil {
		t.Fatalf("after the verb the section opens: %v", err)
	}
	releasing := critical.Record()
	releasing.State = diskstore.StateReleasing
	if err := critical.Write(releasing); err != nil {
		t.Fatal(err)
	}
	_ = critical.Release()
	again := &intentInvocation{owners: owners, layout: layout, cwd: c.root()}
	if _, problem := again.prepareGoalWorktree(c.id); problem == nil || problem.Outcome != intentRefused || !strings.Contains(problem.Summary, "being released") {
		t.Fatalf("a worktree being released = %+v, want an input refusal", problem)
	}
	again.leaveStores()
}

// goal done's sweep enters the registered worktree's critical section
// (Part B 3.2 "Goal"): while an engine verb is inside it the sweep does not
// run, and goal done says the steward's disk pass retries it.
func TestGoalDoneSweepWaitsForAVerbInsideTheWorktree(t *testing.T) {
	c := newConnectionBed(t)
	owners := c.connectionOwners()
	layout, err := owners.resolver.ResolveLayout(c.root())
	if err != nil {
		t.Fatal(err)
	}
	inside := &intentInvocation{owners: owners, layout: layout, cwd: c.root()}
	if _, problem := inside.prepareGoalWorktree(c.id); problem != nil {
		t.Fatalf("prepare = %+v", problem)
	}
	defer inside.leaveStores()
	swept := false
	err = sweepGoalWorktrees(layout.InstallationRoot, c.id, func(context.Context) error { swept = true; return nil })
	if err == nil || swept || !strings.Contains(err.Error(), "retries the sweep") {
		t.Fatalf("a sweep past a verb inside the worktree = %v, swept %v", err, swept)
	}
}

// What the build's isolation copies into a new goal worktree is recorded as
// the engine's own, with its digest (Round D3 F-1); a file that was there
// before the copy is not.
func TestGoalWorktreePreparationRecordsTheCopiedLocalConfiguration(t *testing.T) {
	manifest, _ := supervisor.LocalConfigManifest(supervisor.Deps{})
	if len(manifest) == 0 {
		t.Skip("no runtime declares local configuration")
	}
	c := newConnectionBed(t)
	c.isolate = func(_ func(string, string) error, _, destination string) error {
		target := filepath.Join(destination, manifest[0])
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		return os.WriteFile(target, []byte("{}\n"), 0o600)
	}
	owners := c.connectionOwners()
	layout, err := owners.resolver.ResolveLayout(c.root())
	if err != nil {
		t.Fatal(err)
	}
	inv := &intentInvocation{owners: owners, layout: layout, cwd: c.root()}
	path, problem := inv.prepareGoalWorktree(c.id)
	if problem != nil {
		t.Fatalf("prepare = %+v", problem)
	}
	inv.leaveStores()
	registry := diskstore.CheckoutRegistry(layout.InstallationRoot)
	records, err := diskstore.FindLinkedWorktrees(registry, diskstore.GoalWorktreeClass, diskstore.Owner{Kind: diskstore.OwnerGoal, Ref: c.id})
	if err != nil || len(records) != 1 || records[0].Path != path {
		t.Fatalf("records = %+v, %v", records, err)
	}
	content, err := registry.ReadEngineContent(records[0].ID)
	if err != nil || len(content.Files) != 1 || content.Files[0].Path != filepath.FromSlash(manifest[0]) || content.Files[0].SHA256 == "" {
		t.Fatalf("engine content = %+v, %v", content, err)
	}
}
