package batchowner

import (
	"context"
	"crypto/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

// Round D3 N9: a batch member's release set is read from the same registry
// the goal's workspaces are recorded in (the state root of its
// installation), as the hand route reads it, even where the state root is
// not the installation (an adopted installation's is its repository).
func TestTheBatchRouteReadsTheSeatsStoreRegistry(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	repo, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) string {
		command := exec.Command("git", append([]string{"-C", repo}, args...)...)
		command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1", "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
		out, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q", "-b", "main")
	installation := filepath.Join(repo, "tool")
	if err := os.MkdirAll(filepath.Join(installation, "scripts", "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installation, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".gitignore"), []byte("artifacts/\ntool/artifacts/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", ".")
	git("commit", "-q", "-m", "base")
	tip := git("rev-parse", "HEAD")
	control := steward.StoreControl(installation)
	if control == installation {
		t.Fatalf("the fixture's state root is its installation; the witness proves nothing")
	}
	workspace, err := diskstore.ObtainWorkspace(context.Background(), diskstore.WorkspaceRequest{Registry: diskstore.CheckoutRegistry(control),
		Control: control, GitRoot: repo, Owner: diskstore.Owner{Kind: diskstore.OwnerGoal, Ref: "g"}, Name: "slice", CopyOf: tip,
		Now: time.Now().UTC(), Entropy: rand.Reader, Git: steward.ExecWorkspaceGit})
	if err != nil {
		t.Fatal(err)
	}
	set, err := SelectMemberReleaseSet(installation, "g", tip)
	if err != nil || set == nil || len(set.Stores) != 1 || set.Stores[0].ID != workspace.Record.ID {
		t.Fatalf("the member's set = %+v, %v; want the workspace recorded at the state root %s", set, err, control)
	}
}

// Round D3 N6: a seat's retry sees only its own landed members' unfinished
// sets, and the engine registers it with the steward.
func TestUnfinishedSeatReleaseSetsAreTheSeatsOwn(t *testing.T) {
	t.Parallel()
	lane := t.TempDir()
	pending := &diskstore.ReleaseSet{Tip: "t", Stores: []diskstore.ReleaseEntry{{ID: "01K6WORKSPACE0000000000000", State: diskstore.ReleasePending}}}
	store := batch.NewStore(lane, nil)
	claim := batch.Claim{Machine: "m", Lineage: "l", Epoch: 1, Revision: 1, AccountingRevision: 1}
	if err := store.Create(batch.Record{Schema: 1, BatchID: "01j5x00000000000000000cc01", BaseTree: "b", TipTree: "b", State: batch.StateOpen, Units: []batch.Unit{
		{GoalID: "mine", Chain: "c1", SeatRoot: "/seat/a", P6Done: true, ReleaseSet: pending, State: batch.UnitJoined, Claim: claim},
		{GoalID: "theirs", Chain: "c2", SeatRoot: "/seat/b", P6Done: true, ReleaseSet: pending, State: batch.UnitJoined, Claim: claim},
	}}); err != nil {
		t.Fatal(err)
	}
	if ids, err := UnfinishedSeatReleaseSets(lane, "/seat/a"); err != nil || len(ids) != 1 {
		t.Fatalf("seat a = %v, %v", ids, err)
	}
	if ids, err := UnfinishedSeatReleaseSets(lane, "/seat/c"); err != nil || len(ids) != 0 {
		t.Fatalf("seat c = %v, %v", ids, err)
	}
}
