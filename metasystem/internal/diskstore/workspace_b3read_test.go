package diskstore

// Witnesses of the B3 read of the workspace release (F1 to F6), each a
// real-git claim: what git keeps reachable, and what a release removes.

import (
	"context"
	"crypto/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type realRepo struct {
	t        *testing.T
	repo     string
	base     string
	control  string
	registry Registry
}

func newRealRepo(t *testing.T) *realRepo {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	root := realDir(t)
	r := &realRepo{t: t, repo: filepath.Join(root, "repo")}
	if err := os.MkdirAll(r.repo, 0o700); err != nil {
		t.Fatal(err)
	}
	r.git(r.repo, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(r.repo, ".gitignore"), []byte("metasystem/artifacts/\n*.env\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	r.git(r.repo, "add", ".gitignore")
	r.git(r.repo, "commit", "-q", "-m", "base")
	r.base = r.git(r.repo, "rev-parse", "HEAD")
	r.control = filepath.Join(r.repo, "metasystem")
	r.registry = CheckoutRegistry(r.control)
	return r
}

func (r *realRepo) git(dir string, args ...string) string {
	r.t.Helper()
	out, err := realWorkspaceGit(context.Background(), dir, args...)
	if err != nil {
		r.t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

func (r *realRepo) request(name, copyOf string) WorkspaceRequest {
	return WorkspaceRequest{Registry: r.registry, Control: r.control, GitRoot: r.repo, Owner: goalG, Name: name, CopyOf: copyOf,
		Now: testNow, Entropy: rand.Reader, Git: realWorkspaceGit}
}

func (r *realRepo) obtain(name, copyOf string) Workspace {
	r.t.Helper()
	workspace, err := ObtainWorkspace(context.Background(), r.request(name, copyOf))
	if err != nil {
		r.t.Fatal(err)
	}
	return workspace
}

func (r *realRepo) release(id string) WorkspaceRelease {
	r.t.Helper()
	outcome, err := ReleaseWorkspace(context.Background(), WorkspaceReleaseRequest{Registry: r.registry, GitRoot: r.repo, ID: id, Git: realWorkspaceGit,
		Census: &UseCensus{Taken: true}, By: "t", Now: testNow, IgnoredReleaseBytes: 1 << 20})
	if err != nil {
		r.t.Fatal(err)
	}
	return outcome
}

func (r *realRepo) reachable(sha string) bool {
	return r.git(r.repo, "for-each-ref", "--contains", sha) != ""
}

// F1: a path that exists with no record is never claimed: the request
// fails naming disk show, leaves no reserved record, and a repeat never
// removes what is there.
func TestWorkspaceRealGitNeverClaimsAnUnrecordedPath(t *testing.T) {
	t.Parallel()
	r := newRealRepo(t)
	path := WorkspacePath(r.control, goalG, "bed")
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "precious.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		_, err := ObtainWorkspace(context.Background(), r.request("bed", ""))
		if err == nil || !strings.Contains(err.Error(), "metasystem disk show") || strings.Contains(err.Error(), "repeat") {
			t.Fatalf("an unrecorded path is refused naming disk show: %v", err)
		}
	}
	if _, err := os.Stat(filepath.Join(path, "precious.txt")); err != nil {
		t.Fatalf("the unrecorded content survives: %v", err)
	}
	if records, _ := r.registry.Inventory(); len(records) != 0 {
		t.Fatalf("a refused request leaves no record: %+v", records)
	}
}

// F1: a reserved record whose path holds content it cannot prove it made
// (no marker naming it) is refused, never removed.
func TestWorkspaceInterruptedCreationRemovesOnlyWhatItProves(t *testing.T) {
	t.Parallel()
	r := newRealRepo(t)
	record, err := r.registry.Register(Registration{Path: WorkspacePath(r.control, goalG, "bed"), Class: WorkspaceClass, Owner: goalG,
		Lifetime: LifetimeOwner, CapKind: CapTarget, Layout: LayoutPlain, Reservation: mustKey(t, goalG, "bed")}, testNow, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	writeReservation(t, r.registry, goalG, "bed", record.ID)
	if err := os.MkdirAll(record.Path, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(record.Path, "precious.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ObtainWorkspace(context.Background(), r.request("bed", "")); err == nil || !strings.Contains(err.Error(), "metasystem disk show") {
		t.Fatalf("unprovable content is refused: %v", err)
	}
	if _, err := os.Stat(filepath.Join(record.Path, "precious.txt")); err != nil {
		t.Fatalf("unprovable content survives: %v", err)
	}
}

// F2: a record already releasing is archived before anything is removed.
func TestWorkspaceRealGitReleasingRecordIsArchivedFirst(t *testing.T) {
	t.Parallel()
	r := newRealRepo(t)
	workspace := r.obtain("r", r.base)
	r.git(workspace.Record.Path, "commit", "-q", "--allow-empty", "-m", "unique work")
	unique := r.git(workspace.Record.Path, "rev-parse", "HEAD")
	critical, err := r.registry.TryCritical(workspace.Record.ID)
	if err != nil {
		t.Fatal(err)
	}
	record := critical.Record()
	record.State = StateReleasing
	if err := critical.Write(record); err != nil {
		t.Fatal(err)
	}
	_ = critical.Release()
	if outcome := r.release(workspace.Record.ID); !outcome.Done {
		t.Fatalf("release: %+v", outcome)
	}
	if !r.reachable(unique) {
		t.Fatalf("the unique commit %s is archived though the record was releasing", unique)
	}
}

// F3: a commit at the worktree's own detached HEAD is archived too and
// counted.
func TestWorkspaceRealGitArchivesADetachedHead(t *testing.T) {
	t.Parallel()
	r := newRealRepo(t)
	workspace := r.obtain("d", r.base)
	r.git(workspace.Record.Path, "checkout", "-q", "--detach")
	r.git(workspace.Record.Path, "commit", "-q", "--allow-empty", "-m", "detached work")
	detached := r.git(workspace.Record.Path, "rev-parse", "HEAD")
	outcome := r.release(workspace.Record.ID)
	if !outcome.Done || outcome.Unique != 1 || !r.reachable(detached) {
		t.Fatalf("the detached commit is archived and counted: %+v reachable=%v", outcome, r.reachable(detached))
	}
}

// F3: a commit left only in the worktree's reflog (reset away) is archived.
func TestWorkspaceRealGitArchivesReflogTips(t *testing.T) {
	t.Parallel()
	r := newRealRepo(t)
	workspace := r.obtain("l", r.base)
	r.git(workspace.Record.Path, "commit", "-q", "--allow-empty", "-m", "reset away")
	lost := r.git(workspace.Record.Path, "rev-parse", "HEAD")
	r.git(workspace.Record.Path, "reset", "-q", "--hard", r.base)
	if outcome := r.release(workspace.Record.ID); !outcome.Done || !r.reachable(lost) {
		t.Fatalf("a reflog tip is archived: %+v reachable=%v", outcome, r.reachable(lost))
	}
}

// F4: a workspace whose work did not land is not in the landing's set; one
// whose branch tip and HEAD both lie in the landed tip is.
func TestReleaseSetRealGitTakesOnlyLandedWork(t *testing.T) {
	t.Parallel()
	r := newRealRepo(t)
	r.git(r.repo, "commit", "-q", "--allow-empty", "-m", "slice1 landed")
	landed := r.git(r.repo, "rev-parse", "HEAD")
	unlanded := r.obtain("slice2", r.base)
	r.git(unlanded.Record.Path, "commit", "-q", "--allow-empty", "-m", "slice2 work, not landed")
	contained := r.obtain("slice1", r.base)
	r.obtain("plain", "")
	set, err := SelectReleaseSet(context.Background(), r.registry, r.repo, goalG.Ref, landed, realWorkspaceGit)
	if err != nil {
		t.Fatal(err)
	}
	if len(set.Stores) != 1 || set.Stores[0].ID != contained.Record.ID {
		t.Fatalf("only the contained copy is selected: %+v", set.Stores)
	}
}

// F5: a copy's temporary directory is out of every workspace name's reach:
// releasing copy x never touches plain workspace x.tmp.
func TestWorkspaceRealGitCopyTmpNeverCollidesWithANamedWorkspace(t *testing.T) {
	t.Parallel()
	r := newRealRepo(t)
	plain := r.obtain("x.tmp", "")
	if err := os.WriteFile(filepath.Join(plain.Record.Path, "output.bin"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	workspace := r.obtain("x", r.base)
	if strings.HasPrefix(workspace.Tmp, plain.Record.Path) || workspace.Tmp == plain.Record.Path {
		t.Fatalf("tmp %s lies in workspace %s", workspace.Tmp, plain.Record.Path)
	}
	if outcome := r.release(workspace.Record.ID); !outcome.Done {
		t.Fatalf("release: %+v", outcome)
	}
	if _, err := os.Stat(filepath.Join(plain.Record.Path, "output.bin")); err != nil {
		t.Fatalf("plain workspace x.tmp keeps its content: %v", err)
	}
}

// F6: ignored content is content: a little is released with the copy, more
// than disk.workspace-ignored-release-mib keeps it naming the size and the
// person's discard.
func TestWorkspaceRealGitKeepsLargeIgnoredContent(t *testing.T) {
	t.Parallel()
	r := newRealRepo(t)
	small := r.obtain("small", r.base)
	if err := os.WriteFile(filepath.Join(small.Record.Path, "local.env"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if outcome := r.release(small.Record.ID); !outcome.Done {
		t.Fatalf("a little ignored content is released: %+v", outcome)
	}
	large := r.obtain("large", r.base)
	evidence := filepath.Join(large.Record.Path, "metasystem", "artifacts", "evidence")
	if err := os.MkdirAll(evidence, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evidence, "proof.log"), make([]byte, 2<<20), 0o600); err != nil {
		t.Fatal(err)
	}
	outcome := r.release(large.Record.ID)
	if !outcome.Kept || !strings.Contains(outcome.Reason, "ignored") || !strings.Contains(outcome.Reason, "proof.log") || !strings.Contains(outcome.Command, "--discard") {
		t.Fatalf("large ignored content keeps the copy: %+v", outcome)
	}
	if _, err := os.Stat(filepath.Join(evidence, "proof.log")); err != nil {
		t.Fatalf("the ignored evidence survives: %v", err)
	}
}

// N1: an archive never overwrites an existing archive ref.
func TestWorkspaceRealGitArchiveNeverOverwrites(t *testing.T) {
	t.Parallel()
	r := newRealRepo(t)
	first := r.obtain("n", r.base)
	r.git(first.Record.Path, "commit", "-q", "--allow-empty", "-m", "first")
	one := r.git(first.Record.Path, "rev-parse", "HEAD")
	r.release(first.Record.ID)
	second := r.obtain("n", r.base)
	r.git(second.Record.Path, "commit", "-q", "--allow-empty", "-m", "second")
	two := r.git(second.Record.Path, "rev-parse", "HEAD")
	r.release(second.Record.ID)
	if !r.reachable(one) || !r.reachable(two) {
		t.Fatalf("both archives stay: %v %v", r.reachable(one), r.reachable(two))
	}
}

func mustKey(t *testing.T, owner Owner, name string) string {
	t.Helper()
	key, err := ReservationKey(owner, name)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func writeReservation(t *testing.T, registry Registry, owner Owner, name, id string) {
	t.Helper()
	entry := registry.ReservationPath(mustKey(t, owner, name))
	if err := os.MkdirAll(filepath.Dir(entry), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(entry, []byte(`{"record":"`+id+`"}`), 0o600); err != nil {
		t.Fatal(err)
	}
}

// N6: a name is one path element and one branch component.
func TestValidWorkspaceNameFollowsBranchNameRules(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]bool{"bed": true, "slice-2.b_x": true, "x.tmp": true, "": false, ".hidden": false, "-x": false,
		"a..b": false, "a/b": false, "a b": false, "x.lock": false, "end.": false, "a~1": false, strings.Repeat("a", 101): false} {
		if got := ValidWorkspaceName(name); got != want {
			t.Errorf("ValidWorkspaceName(%q) = %v, want %v", name, got, want)
		}
	}
}

// F5: a copy's temporary directory is removed only when its marker names
// the record; content without it is refused and kept.
func TestCopyTmpIsRemovedOnlyWithItsMarker(t *testing.T) {
	t.Parallel()
	bed := newWorkspaceBed(t)
	workspace := bed.obtain(goalG, "bed", "abc123")
	if err := os.Remove(filepath.Join(workspace.Tmp, MarkerName)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace.Tmp, "foreign"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if outcome := bed.release(workspace.Record.ID, emptyCensus(), nil); outcome.Done {
		t.Fatalf("an unmarked tmp with content stops the release: %+v", outcome)
	}
	if _, err := os.Stat(filepath.Join(workspace.Tmp, "foreign")); err != nil {
		t.Fatalf("the unmarked content survives: %v", err)
	}
}

// N3: the census of a release is taken inside the critical section, never
// for a workspace a verb holds.
func TestWorkspaceReleaseTakesTheCensusInsideTheSection(t *testing.T) {
	t.Parallel()
	bed := newWorkspaceBed(t)
	workspace := bed.obtain(goalG, "", "")
	taken := 0
	request := WorkspaceReleaseRequest{Registry: bed.registry, GitRoot: bed.git.gitRoot, ID: workspace.Record.ID, Git: bed.git.run, By: "t", Now: testNow,
		TakeCensus: func() *UseCensus { taken++; return emptyCensus() }}
	entrant, err := bed.registry.Enter(workspace.Record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if outcome, _ := ReleaseWorkspace(context.Background(), request); !outcome.Pending || taken != 0 {
		t.Fatalf("a held workspace takes no census: %+v taken=%d", outcome, taken)
	}
	_ = entrant.Leave()
	if outcome, _ := ReleaseWorkspace(context.Background(), request); !outcome.Done || taken != 1 {
		t.Fatalf("the release takes one census inside its section: %+v taken=%d", outcome, taken)
	}
}

// N5: a landing record another writer holds is pending, never read.
func TestLandingRecordLockIsExclusive(t *testing.T) {
	t.Parallel()
	path := filepath.Join(realDir(t), "landed.json")
	release, err := LockLandingRecord(path, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := LockLandingRecord(path, false); err != ErrRecordHeld {
		t.Fatalf("a held record: %v", err)
	}
	release()
	again, err := LockLandingRecord(path, false)
	if err != nil {
		t.Fatal(err)
	}
	again()
}
