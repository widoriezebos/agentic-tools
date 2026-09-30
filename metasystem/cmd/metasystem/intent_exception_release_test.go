package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

// exceptionReleaseBed is the real carried delivery bed with two of the
// goal's workspaces made from its branch tip: one clean copy whose work the
// goal branch holds, and one with a commit the goal branch does not have.
type exceptionReleaseBed struct {
	*carriedDeliveryBed
	stateRoot, gitRoot string
	landed, unlanded   diskstore.Record
}

func newExceptionReleaseBed(t *testing.T) *exceptionReleaseBed {
	t.Helper()
	b := &exceptionReleaseBed{carriedDeliveryBed: newCarriedDeliveryBed(t)}
	f := b.f
	// The use census is taken and names no holder; git is the real git.
	b.owners.disk = diskOwners{census: func() *diskstore.UseCensus { return &diskstore.UseCensus{Taken: true} }}
	b.stateRoot = steward.StoreControl(f.mainRoot)
	b.gitRoot = goalSyncMutationGit(t, f.mainRoot, "rev-parse", "--show-toplevel")
	obtain := func(name string) diskstore.Record {
		workspace, err := diskstore.ObtainWorkspace(context.Background(), diskstore.WorkspaceRequest{Registry: diskstore.CheckoutRegistry(b.stateRoot),
			Control: b.stateRoot, GitRoot: b.gitRoot, Owner: diskstore.Owner{Kind: diskstore.OwnerGoal, Ref: "standing-validation"}, Name: name,
			CopyOf: f.branchTip, Now: time.Now().UTC(), Entropy: rand.Reader, Git: steward.ExecWorkspaceGit})
		if err != nil {
			t.Fatal(err)
		}
		return workspace.Record
	}
	b.landed, b.unlanded = obtain("slice"), obtain("beyond")
	writeTestingFixtureFile(t, filepath.Join(b.unlanded.Path, "unlanded.txt"), []byte("work the goal branch does not have\n"), 0o644)
	goalSyncMutationGit(t, b.unlanded.Path, "add", "unlanded.txt")
	goalSyncMutationGit(t, b.unlanded.Path, "-c", "user.name=builder", "-c", "user.email=builder@example.invalid", "commit", "-qm", "unlanded work")
	return b
}

// record reads the exception's landing record.
func (b *exceptionReleaseBed) record(opid string) intentLanded {
	b.t.Helper()
	path := filepath.Join(b.f.mainRoot, "artifacts", "agents", "landing-intent", "standing-validation", "exception-"+opid, "landed.json")
	data, err := os.ReadFile(path)
	if err != nil {
		b.t.Fatalf("the exception's landing record: %v", err)
	}
	var landed intentLanded
	if err := json.Unmarshal(data, &landed); err != nil {
		b.t.Fatal(err)
	}
	return landed
}

// released says the landed workspace is gone with its tip archived, and the
// unlanded one stays.
func (b *exceptionReleaseBed) released() {
	b.t.Helper()
	if _, err := os.Stat(b.landed.Path); !os.IsNotExist(err) {
		b.t.Fatalf("the clean workspace whose work landed is not released: %v", err)
	}
	archived := goalSyncMutationGit(b.t, b.gitRoot, "for-each-ref", "--format=%(objectname)", "refs/archive/")
	if !strings.Contains(archived, b.f.branchTip) {
		b.t.Fatalf("the released workspace's tip %s is not archived: %q", b.f.branchTip, archived)
	}
	if _, err := os.Stat(b.unlanded.Path); err != nil {
		b.t.Fatalf("a workspace with unlanded commits is kept: %v", err)
	}
}

// TestExceptionLandingReleasesItsLandedWorkspaces: the exception route
// (work land G --exception, then --using-exception) records the goal's
// release set in its own landing record before the push and releases the
// goal's clean workspace whose work landed once the push succeeded; a
// workspace with commits the landing does not contain is never in the set
// (disk-lifetimes Part B 3.6, R22, U6d).
func TestExceptionLandingReleasesItsLandedWorkspaces(t *testing.T) {
	b := newExceptionReleaseBed(t)
	_, result := b.land("standing-validation", "--exception", "missing-declaration", "--reason", "flaky host", "--by", "Wido", "--upgrade-goals")
	opid, _ := carriedResultData(result)["exception"].(string)
	if result.Outcome != intentPartial || opid == "" {
		t.Fatalf("first request: %+v", result)
	}
	selected := b.record(opid)
	if selected.ReleaseSet == nil || selected.ReleaseSet.Tip != b.f.branchTip || len(selected.ReleaseSet.Stores) != 1 ||
		selected.ReleaseSet.Stores[0].ID != b.landed.ID || selected.Landing != "" || selected.Swept {
		t.Fatalf("the set is recorded at selection with only the landed workspace, nothing landed yet: %+v", selected)
	}
	if _, err := os.Stat(b.landed.Path); err != nil {
		t.Fatalf("nothing is released before the push: %v", err)
	}
	code, landed := b.shown(b.provePublicly(result))
	if code != 0 || landed.Outcome != intentConfirmed {
		t.Fatalf("the exception did not land: %d %+v", code, landed)
	}
	b.released()
	after := b.record(opid)
	if carried := b.carried(opid); after.Landing != carried || !after.Swept || after.Subject != carried || !after.ReleaseSet.Finished() ||
		after.ReleaseSet.Stores[0].State != diskstore.ReleaseReleased {
		t.Fatalf("the record says landed as the carried commit and released: %+v", after)
	}
	if !strings.Contains(landed.Summary, "1 released") {
		t.Fatalf("the landing's summary names its workspaces: %q", landed.Summary)
	}
}

// TestExceptionLandingCrashAfterThePushIsFinishedByTheRetry: an exception
// landing killed after its push, before its release, leaves the set
// pending; the sweeper's retry of landing release sets finds the pushed
// commit on the remote-tracking ref and finishes it with the goal still
// open, and the same land command afterwards records no second set.
func TestExceptionLandingCrashAfterThePushIsFinishedByTheRetry(t *testing.T) {
	b := newExceptionReleaseBed(t)
	_, result := b.land("standing-validation", "--exception", "missing-declaration", "--reason", "flaky host", "--by", "Wido", "--upgrade-goals")
	opid, _ := carriedResultData(result)["exception"].(string)
	if opid == "" {
		t.Fatalf("first request: %+v", result)
	}
	proved := b.provePublicly(result)
	b.crashAt = "after-push"
	code, crashed := b.shown(proved)
	if code == 0 || !strings.Contains(string(b.lands[len(b.lands)-1].stdout), "FIXTURE-CRASH after-push") {
		t.Fatalf("the crash seam did not stop the landing after its push: %d %+v", code, crashed)
	}
	carried := b.carried(opid)
	if _, err := os.Stat(b.landed.Path); err != nil {
		t.Fatalf("the crashed landing released in process: %v", err)
	}
	if pending := b.record(opid); pending.Swept || pending.Subject != carried || pending.ReleaseSet.Finished() {
		t.Fatalf("the record names the pushed commit and waits: %+v", pending)
	}
	now := time.Now().UTC()
	class := steward.LandingReleaseSets{Installation: b.f.mainRoot, StateRoot: b.stateRoot, GitRoot: b.gitRoot, Git: steward.ExecWorkspaceGit}
	report, err := diskstore.RunPass(context.Background(), diskstore.PassOptions{Kind: "checkout", Name: b.f.mainRoot, Registry: diskstore.CheckoutRegistry(b.stateRoot),
		Mode: diskstore.ModeApply, Now: now, Clock: func() time.Time { return now }, Classes: []diskstore.Class{class},
		CensusReader: &diskstore.CensusReader{Pids: func() ([]int64, error) { return nil, nil }}})
	if err != nil || len(report.Actions) != 1 {
		t.Fatalf("the sweeper's retry finishes the pushed exception's set: %v %+v", err, report)
	}
	b.released()
	if after := b.record(opid); after.Landing != carried || !after.Swept || !after.ReleaseSet.Finished() {
		t.Fatalf("the retry records the landing and the release: %+v", after)
	}
	code, resumed := b.shown(crashed)
	if code != 0 || resumed.Outcome != intentConfirmed {
		t.Fatalf("the same land command completes the carried record: %d %+v", code, resumed)
	}
	if after := b.record(opid); len(after.ReleaseSet.Stores) != 1 || after.ReleaseSet.Stores[0].ID != b.landed.ID {
		t.Fatalf("no second set: %+v", after)
	}
}

// TestExceptionLandingCrashAfterThePushIsFinishedByTheNextLand: the same
// crash, finished instead by the next land command under the same
// exception, which completes the carried record and releases the set.
func TestExceptionLandingCrashAfterThePushIsFinishedByTheNextLand(t *testing.T) {
	b := newExceptionReleaseBed(t)
	_, result := b.land("standing-validation", "--exception", "missing-declaration", "--reason", "flaky host", "--by", "Wido", "--upgrade-goals")
	opid, _ := carriedResultData(result)["exception"].(string)
	if opid == "" {
		t.Fatalf("first request: %+v", result)
	}
	proved := b.provePublicly(result)
	b.crashAt = "after-push"
	if code, crashed := b.shown(proved); code == 0 {
		t.Fatalf("the crash seam did not stop the landing: %+v", crashed)
	} else if _, err := os.Stat(b.landed.Path); err != nil {
		t.Fatalf("the crashed landing released in process: %v", err)
	} else if code, resumed := b.shown(crashed); code != 0 || resumed.Outcome != intentConfirmed {
		t.Fatalf("the next land did not complete: %d %+v", code, resumed)
	}
	b.released()
	if after := b.record(opid); after.Landing != b.carried(opid) || !after.Swept || !after.ReleaseSet.Finished() || len(after.ReleaseSet.Stores) != 1 {
		t.Fatalf("the next land records the landing and the release: %+v", after)
	}
}
