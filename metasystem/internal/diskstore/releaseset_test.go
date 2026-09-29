package diskstore

import (
	"context"
	"testing"
)

// ancestry answers merge-base --is-ancestor from a table of (ancestor, tip)
// pairs; every other pair is not an ancestor.
func (g *fakeWorkspaceGit) withAncestry(pairs ...[2]string) WorkspaceGit {
	return func(ctx context.Context, dir string, args ...string) ([]byte, error) {
		if len(args) == 4 && args[0] == "merge-base" && args[1] == "--is-ancestor" {
			for _, pair := range pairs {
				if pair[0] == args[2] && pair[1] == args[3] {
					return nil, nil
				}
			}
			return nil, gitNotFound{}
		}
		return g.run(ctx, dir, args...)
	}
}

// A landing's release set is the goal's accepted, clean copies whose branch
// tip and worktree HEAD both lie in the selected tip; a copy whose work lies
// beyond it (a --through prefix), a dirty copy, a plain workspace and
// another goal's workspace are not in it. Running the set releases each
// id and marks it released, kept with the reason, or absent; a repeat
// finishes only what is still pending and writes nothing new (3.6,
// revision 4d; R22; Round B3 ruling).
func TestReleaseSetSelectsContainedCopiesAndRunsOnce(t *testing.T) {
	t.Parallel()
	bed := newWorkspaceBed(t)
	inside := bed.obtain(goalG, "inside", "base1")
	later := bed.obtain(goalG, "later", "base1")
	beyond := bed.obtain(goalG, "beyond", "base2")
	dirty := bed.obtain(goalG, "dirty", "base1")
	bed.obtain(goalG, "plain", "")
	bed.obtain(Owner{Kind: OwnerGoal, Ref: "other"}, "inside", "base1")
	bed.git.refs["refs/heads/workspace/goal-g/beyond"] = "far"
	bed.git.dirty[dirty.Record.Path] = " M x\n"
	git := bed.git.withAncestry([2]string{"base1", "tip"})
	set, err := SelectReleaseSet(context.Background(), bed.registry, bed.git.gitRoot, goalG.Ref, "tip", git)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, entry := range set.Stores {
		ids[entry.ID] = entry.State == ReleasePending
	}
	if len(ids) != 2 || !ids[inside.Record.ID] || !ids[later.Record.ID] || ids[beyond.Record.ID] || ids[dirty.Record.ID] {
		t.Fatalf("the set holds the clean contained copies only: %+v", set)
	}
	bed.git.dirty[later.Record.Path] = "?? new\n"
	request := WorkspaceReleaseRequest{Registry: bed.registry, GitRoot: bed.git.gitRoot, Git: git, Census: emptyCensus(), By: "landing", Now: testNow}
	if changed := RunReleaseSet(context.Background(), request, &set); !changed {
		t.Fatal("the first run changes the set")
	}
	states := map[string]string{}
	for _, entry := range set.Stores {
		states[entry.ID] = entry.State
	}
	if states[inside.Record.ID] != ReleaseReleased || states[later.Record.ID] != ReleaseKept || !set.Finished() {
		t.Fatalf("states = %v", states)
	}
	if record, _ := bed.registry.Load(beyond.Record.ID); record.State != StateAccepted {
		t.Fatal("a workspace outside the set is untouched")
	}
	if changed := RunReleaseSet(context.Background(), request, &set); changed {
		t.Fatal("a finished set changes nothing on a repeat")
	}
	gone := ReleaseSet{Tip: "tip", Stores: []ReleaseEntry{{ID: "01K00000000000000000000000", State: ReleasePending}}}
	RunReleaseSet(context.Background(), request, &gone)
	if gone.Stores[0].State != ReleaseAbsent {
		t.Fatalf("a missing store is absent: %+v", gone)
	}
}

// A store whose release is pending (a verb holds it) stays pending, and the
// set stays unfinished for the next retry.
func TestReleaseSetKeepsAPendingStoreForTheRetry(t *testing.T) {
	t.Parallel()
	bed := newWorkspaceBed(t)
	workspace := bed.obtain(goalG, "inside", "base1")
	git := bed.git.withAncestry([2]string{"base1", "tip"})
	set, err := SelectReleaseSet(context.Background(), bed.registry, bed.git.gitRoot, goalG.Ref, "tip", git)
	if err != nil || len(set.Stores) != 1 {
		t.Fatalf("set = %+v %v", set, err)
	}
	entrant, err := bed.registry.Enter(workspace.Record.ID)
	if err != nil {
		t.Fatal(err)
	}
	request := WorkspaceReleaseRequest{Registry: bed.registry, GitRoot: bed.git.gitRoot, Git: git, Census: emptyCensus(), By: "landing", Now: testNow}
	RunReleaseSet(context.Background(), request, &set)
	if set.Finished() || set.Stores[0].State != ReleasePending {
		t.Fatalf("a held store stays pending: %+v", set)
	}
	_ = entrant.Leave()
	RunReleaseSet(context.Background(), request, &set)
	if !set.Finished() || set.Stores[0].State != ReleaseReleased {
		t.Fatalf("the retry releases it: %+v", set)
	}
}
