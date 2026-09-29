package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
)

// landReleaseGit is the workspace fake with ancestry: every commit named in
// contained is an ancestor of every tip.
type landReleaseGit struct {
	*workspaceGit
	contained map[string]bool
}

func (g landReleaseGit) run(ctx context.Context, dir string, args ...string) ([]byte, error) {
	if len(args) == 4 && args[0] == "merge-base" && args[1] == "--is-ancestor" {
		if g.contained[args[2]] || args[2] == args[3] {
			return nil, nil
		}
		return nil, cmdGitNotFound{}
	}
	return g.workspaceGit.run(ctx, dir, args...)
}

func (b *deliveryBed) doWithDisk(disk diskOwners, args ...string) (int, intentResult) {
	b.t.Helper()
	owners := b.intentBed.owners()
	owners.delivery = b.owners
	owners.connection = b.connection
	owners.disk = disk
	return b.runJSON(owners, args...)
}

func landWorkspace(t *testing.T, root string, git diskstore.WorkspaceGit, name, copyOf string) diskstore.Record {
	t.Helper()
	workspace, err := diskstore.ObtainWorkspace(context.Background(), diskstore.WorkspaceRequest{Registry: diskstore.CheckoutRegistry(root),
		Control: root, GitRoot: root, Owner: diskstore.Owner{Kind: diskstore.OwnerGoal, Ref: "standing-validation"}, Name: name, CopyOf: copyOf,
		Now: diskNow, Entropy: rand.Reader, Git: git})
	if err != nil {
		t.Fatal(err)
	}
	return workspace.Record
}

func landedRecords(t *testing.T, root string) []intentLanded {
	t.Helper()
	paths, _ := filepath.Glob(filepath.Join(root, "artifacts", "agents", "landing-intent", "standing-validation", "*", "landed.json"))
	var records []intentLanded
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var landed intentLanded
		if err := json.Unmarshal(data, &landed); err != nil {
			t.Fatal(err)
		}
		records = append(records, landed)
	}
	return records
}

// The hand route records the release set in landed.json before its push
// and releases exactly those workspaces once the merged branch is swept; a
// landing whose sweep failed releases them when the retry sweeps; a replay
// after a new workspace was made for the next slice releases nothing new
// and records no second set (3.6 revision 4d, DL4C-11, R22, U6d).
func TestIntentLandByHandReleasesTheRecordedWorkspaces(t *testing.T) {
	t.Parallel()
	b := newDeliveryBed(t)
	owners := &landingOwners{status: readBranch(2, "reader-record", "reader-record")}
	owners.install(b)
	git := landReleaseGit{workspaceGit: &workspaceGit{refs: map[string]string{"HEAD": "base"}, dirty: map[string]string{}}, contained: map[string]bool{"sliceone": true}}
	disk := diskOwners{git: git.run, now: func() time.Time { return diskNow }, census: func() *diskstore.UseCensus { return &diskstore.UseCensus{Taken: true} },
		person: func(string) (string, error) { return "Wido", nil }}
	root := b.root()
	inside := landWorkspace(t, root, git.run, "slice", "sliceone")
	beyond := landWorkspace(t, root, git.run, "later", "notyet")

	owners.sweepErr = []error{errors.New("sweep: remote refused the branch deletion")}
	code, result := b.doWithDisk(disk, "work", "land", "standing-validation")
	expectOutcome(t, "pushed, unswept", code, result, intentPartial)
	records := landedRecords(t, root)
	if len(records) != 1 || records[0].ReleaseSet == nil || len(records[0].ReleaseSet.Stores) != 1 || records[0].ReleaseSet.Stores[0].ID != inside.ID ||
		records[0].ReleaseSet.Stores[0].State != diskstore.ReleasePending {
		t.Fatalf("the set is recorded before the push and waits for the sweep: %+v", records)
	}
	if _, err := os.Stat(inside.Path); err != nil {
		t.Fatalf("nothing is released before the merged branch is swept: %v", err)
	}

	code, result = b.doWithDisk(disk, "work", "land", "standing-validation")
	expectOutcome(t, "resumed sweep", code, result, intentConfirmed)
	if _, err := os.Stat(inside.Path); !os.IsNotExist(err) {
		t.Fatalf("the recorded workspace is released with the sweep: %v", err)
	}
	if _, err := os.Stat(beyond.Path); err != nil {
		t.Fatalf("a workspace whose work did not land stays: %v", err)
	}
	records = landedRecords(t, root)
	if len(records) != 1 || records[0].ReleaseSet.Stores[0].State != diskstore.ReleaseReleased || !records[0].ReleaseSet.Finished() {
		t.Fatalf("the set records the release: %+v", records)
	}

	git.contained["nextslice"] = true
	next := landWorkspace(t, root, git.run, "next", "nextslice")
	code, result = b.doWithDisk(disk, "work", "land", "standing-validation")
	expectOutcome(t, "landed and branch gone", code, result, intentUnchanged)
	if _, err := os.Stat(next.Path); err != nil {
		t.Fatalf("a replay never discovers the next slice's workspace: %v", err)
	}
	if records := landedRecords(t, root); len(records) != 1 || len(records[0].ReleaseSet.Stores) != 1 {
		t.Fatalf("no second set: %+v", records)
	}
}

// A set store an engine verb holds stays pending, and the next work land of
// the goal finishes it before anything else.
func TestIntentLandFinishesAnUnfinishedReleaseSet(t *testing.T) {
	t.Parallel()
	b := newDeliveryBed(t)
	owners := &landingOwners{status: readBranch(2, "reader-record", "reader-record")}
	owners.install(b)
	git := landReleaseGit{workspaceGit: &workspaceGit{refs: map[string]string{"HEAD": "base"}, dirty: map[string]string{}}, contained: map[string]bool{"sliceone": true}}
	disk := diskOwners{git: git.run, now: func() time.Time { return diskNow }, census: func() *diskstore.UseCensus { return &diskstore.UseCensus{Taken: true} }}
	root := b.root()
	inside := landWorkspace(t, root, git.run, "slice", "sliceone")
	entrant, err := diskstore.CheckoutRegistry(root).Enter(inside.ID)
	if err != nil {
		t.Fatal(err)
	}
	code, result := b.doWithDisk(disk, "work", "land", "standing-validation")
	expectOutcome(t, "landed", code, result, intentConfirmed)
	if records := landedRecords(t, root); len(records) != 1 || records[0].ReleaseSet.Finished() || !strings.Contains(records[0].ReleaseSet.Stores[0].Reason, "record lock is held") {
		t.Fatalf("a held store stays pending with its reason: %+v", records)
	}
	_ = entrant.Leave()
	b.doWithDisk(disk, "work", "land", "standing-validation")
	if _, err := os.Stat(inside.Path); !os.IsNotExist(err) {
		t.Fatalf("the next work land finishes the set: %v", err)
	}
	if records := landedRecords(t, root); !records[0].ReleaseSet.Finished() {
		t.Fatalf("finished: %+v", records)
	}
}
