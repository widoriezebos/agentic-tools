package steward

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
)

// A staged landing that crashed after its push, before it released its set
// (disk-lifetimes Part B 3.6, U6d), is finished by the checkout pass once
// the remote-tracking ref it was pushed to holds its commit; one whose
// commit is not there (the push never happened) releases nothing.
func TestCheckoutPassFinishesAStagedLandingThatCrashedAfterItsPush(t *testing.T) {
	t.Parallel()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	fake := &releaseGit{refs: map[string]string{"HEAD": "base"}}
	pushed := false
	git := func(ctx context.Context, dir string, args ...string) ([]byte, error) {
		if len(args) > 3 && args[0] == "merge-base" && args[3] == "refs/remotes/origin/main" && !pushed {
			return nil, errors.New("not an ancestor")
		}
		return fake.run(ctx, dir, args...)
	}
	registry := diskstore.CheckoutRegistry(root)
	workspace, err := diskstore.ObtainWorkspace(context.Background(), diskstore.WorkspaceRequest{Registry: registry, Control: root, GitRoot: root,
		Owner: diskstore.Owner{Kind: diskstore.OwnerGoal, Ref: "g"}, Name: "slice", CopyOf: "c1", Now: now, Entropy: rand.Reader, Git: git})
	if err != nil {
		t.Fatal(err)
	}
	landedPath := filepath.Join(root, "artifacts", "agents", "landing-intent", "g", "staged-c1", "landed.json")
	if err := os.MkdirAll(filepath.Dir(landedPath), 0o700); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(map[string]any{"Subject": "c1", "Staged": true, "Endpoint": "refs/remotes/origin/main", "Branch": "main",
		"ReleaseSet": diskstore.ReleaseSet{Tip: "c1", Stores: []diskstore.ReleaseEntry{{ID: workspace.Record.ID, Path: workspace.Record.Path, State: diskstore.ReleasePending}}}})
	if err := os.WriteFile(landedPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	class := LandingReleaseSets{Installation: root, StateRoot: root, GitRoot: root, Git: git}
	pass := func() diskstore.Report {
		report, err := diskstore.RunPass(context.Background(), diskstore.PassOptions{Kind: "checkout", Name: root, Registry: registry, Mode: diskstore.ModeApply,
			Now: now, Clock: func() time.Time { return now }, Classes: []diskstore.Class{class},
			CensusReader: &diskstore.CensusReader{Pids: func() ([]int64, error) { return nil, nil }}})
		if err != nil {
			t.Fatal(err)
		}
		return report
	}
	if report := pass(); len(report.Actions) != 0 {
		t.Fatalf("an unpushed staged landing releases nothing: %+v", report)
	}
	if _, err := os.Stat(workspace.Record.Path); err != nil {
		t.Fatalf("the workspace of an unpushed landing is kept: %v", err)
	}
	pushed = true
	if report := pass(); len(report.Actions) != 1 {
		t.Fatalf("a pushed staged landing's set is finished: %+v", report)
	}
	if _, err := os.Stat(workspace.Record.Path); !os.IsNotExist(err) {
		t.Fatalf("the recorded workspace is released: %v", err)
	}
	var after struct {
		Landing    string
		Swept      bool
		ReleaseSet diskstore.ReleaseSet
	}
	written, _ := os.ReadFile(landedPath)
	if json.Unmarshal(written, &after) != nil || after.Landing != "c1" || !after.Swept || !after.ReleaseSet.Finished() {
		t.Fatalf("the staged record says landed and released: %s", written)
	}
}
