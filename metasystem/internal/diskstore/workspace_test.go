package diskstore

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// fakeWorkspaceGit is one test's git: linked worktrees are directories with
// a .git file, refs a map, status and the unique-commit count set by the
// test. It never runs git.
type fakeWorkspaceGit struct {
	mu        sync.Mutex
	gitRoot   string
	refs      map[string]string
	dirty     map[string]string
	unique    string
	calls     []string
	failAdd   bool
	worktrees map[string]string
}

func (g *fakeWorkspaceGit) branchOf(worktree string) string { return g.worktrees[worktree] }

func newFakeWorkspaceGit(gitRoot string) *fakeWorkspaceGit {
	return &fakeWorkspaceGit{gitRoot: gitRoot, refs: map[string]string{"HEAD": "c0ffee"}, dirty: map[string]string{}, unique: "0", worktrees: map[string]string{}}
}

func (g *fakeWorkspaceGit) run(_ context.Context, dir string, args ...string) ([]byte, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.calls = append(g.calls, strings.Join(args, " "))
	switch {
	case len(args) == 6 && args[0] == "worktree" && args[1] == "add" && args[2] == "-b":
		if g.failAdd {
			return nil, errors.New("fatal: interrupted")
		}
		branch, path, sha := args[3], args[4], args[5]
		gitdir := filepath.Join(g.gitRoot, ".git", "worktrees", filepath.Base(path))
		if err := os.MkdirAll(gitdir, 0o700); err != nil {
			return nil, err
		}
		if err := os.MkdirAll(path, 0o700); err != nil {
			return nil, err
		}
		if err := os.WriteFile(filepath.Join(path, ".git"), []byte("gitdir: "+gitdir+"\n"), 0o600); err != nil {
			return nil, err
		}
		g.refs["refs/heads/"+branch] = sha
		g.worktrees[path] = branch
		return nil, nil
	case len(args) >= 3 && args[0] == "worktree" && args[1] == "remove":
		return nil, os.RemoveAll(args[len(args)-1])
	case len(args) == 2 && args[0] == "worktree" && args[1] == "prune":
		return nil, nil
	case len(args) == 3 && args[0] == "branch" && args[1] == "-D":
		if _, ok := g.refs["refs/heads/"+args[2]]; !ok {
			return nil, fmt.Errorf("error: branch '%s' not found", args[2])
		}
		delete(g.refs, "refs/heads/"+args[2])
		return nil, nil
	case len(args) >= 2 && args[0] == "status":
		return []byte(g.dirty[dir]), nil
	case len(args) == 4 && args[0] == "rev-parse" && args[1] == "--verify" && args[2] == "-q":
		ref := args[3]
		if ref == "HEAD" && dir != g.gitRoot {
			ref = "refs/heads/" + g.branchOf(dir)
		}
		if sha, ok := g.refs[ref]; ok {
			return []byte(sha + "\n"), nil
		}
		return nil, errors.New("exit status 1")
	case len(args) == 4 && args[0] == "update-ref" && args[3] == "":
		if _, exists := g.refs[args[1]]; exists {
			return nil, errors.New("fatal: reference already exists")
		}
		g.refs[args[1]] = args[2]
		return nil, nil
	case len(args) >= 2 && args[0] == "log" && args[1] == "-g":
		return nil, nil
	case len(args) == 4 && args[0] == "merge-base" && args[1] == "--is-ancestor":
		if args[2] == args[3] {
			return nil, nil
		}
		return nil, errors.New("exit status 1")
	case len(args) >= 3 && args[0] == "rev-list" && args[1] == "--count":
		return []byte(g.unique + "\n"), nil
	}
	return nil, fmt.Errorf("fake git: unexpected %q in %s", args, dir)
}

type workspaceBed struct {
	t        *testing.T
	control  string
	registry Registry
	git      *fakeWorkspaceGit
}

func newWorkspaceBed(t *testing.T) *workspaceBed {
	t.Helper()
	root := realDir(t)
	control := filepath.Join(root, "repo", "metasystem")
	if err := os.MkdirAll(control, 0o700); err != nil {
		t.Fatal(err)
	}
	return &workspaceBed{t: t, control: control, registry: CheckoutRegistry(control), git: newFakeWorkspaceGit(filepath.Join(root, "repo"))}
}

func (b *workspaceBed) request(owner Owner, name, copyOf string) WorkspaceRequest {
	return WorkspaceRequest{Registry: b.registry, Control: b.control, GitRoot: b.git.gitRoot, Owner: owner, Name: name, CopyOf: copyOf,
		CapBytes: 8 << 30, Now: testNow, Entropy: rand.Reader, Git: b.git.run}
}

func (b *workspaceBed) obtain(owner Owner, name, copyOf string) Workspace {
	b.t.Helper()
	workspace, err := ObtainWorkspace(context.Background(), b.request(owner, name, copyOf))
	if err != nil {
		b.t.Fatal(err)
	}
	return workspace
}

func (b *workspaceBed) release(id string, census *UseCensus, discard *Discard) WorkspaceRelease {
	b.t.Helper()
	outcome, err := ReleaseWorkspace(context.Background(), WorkspaceReleaseRequest{Registry: b.registry, GitRoot: b.git.gitRoot, ID: id,
		Git: b.git.run, Census: census, Discard: discard, By: "test", Now: testNow})
	if err != nil {
		b.t.Fatal(err)
	}
	return outcome
}

func emptyCensus() *UseCensus { return &UseCensus{Taken: true} }

var goalG = Owner{Kind: OwnerGoal, Ref: "g"}

func (b *workspaceBed) records() []Record {
	records, _ := b.registry.Inventory()
	return records
}

// A plain workspace is a marker directory registered before it received a
// byte, with its temporary directory inside; its release removes exactly
// the recorded path and marks the record released (3.6, U6b).
func TestPlainWorkspaceIsAMarkerDirectoryAndReleasesExactlyItsPath(t *testing.T) {
	t.Parallel()
	bed := newWorkspaceBed(t)
	workspace := bed.obtain(goalG, "", "")
	want := filepath.Join(bed.control, "artifacts", "agents", "workspaces", "goal-g", "default")
	if workspace.Record.Path != want || !workspace.Created || workspace.Record.State != StateAccepted || workspace.Record.Layout != LayoutPlain {
		t.Fatalf("workspace = %+v", workspace)
	}
	if err := Revalidate(workspace.Record); err != nil {
		t.Fatalf("the plain workspace carries its marker: %v", err)
	}
	if workspace.Tmp != filepath.Join(want, "tmp") {
		t.Fatalf("tmp = %s", workspace.Tmp)
	}
	sibling := filepath.Join(filepath.Dir(want), "other")
	if err := os.MkdirAll(sibling, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace.Tmp, "bed.log"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	outcome := bed.release(workspace.Record.ID, emptyCensus(), nil)
	if !outcome.Done {
		t.Fatalf("release: %+v", outcome)
	}
	if _, err := os.Stat(want); !os.IsNotExist(err) {
		t.Fatalf("the workspace is gone: %v", err)
	}
	if _, err := os.Stat(sibling); err != nil {
		t.Fatalf("only the recorded path goes: %v", err)
	}
	record, _ := bed.registry.Load(workspace.Record.ID)
	if record.State != StateReleased || record.ReleasedBy != "test" {
		t.Fatalf("record = %+v", record)
	}
}

// A copy is a linked worktree on workspace/<owner>/<name>; a clean copy's
// release archives its branch tip under refs/archive/<owner>/<name>/<branch>,
// counts the commits nothing else contains, and removes the worktree and
// the branch (3.6 revision 4c, 3.12).
func TestCopyWorkspaceReleaseArchivesTheTipThenRemovesWorktreeAndBranch(t *testing.T) {
	t.Parallel()
	bed := newWorkspaceBed(t)
	workspace := bed.obtain(goalG, "bed", "abc123")
	if workspace.Record.Layout != LayoutCopy || workspace.Record.CopyOf != "abc123" || workspace.Record.Identity.Gitdir == "" {
		t.Fatalf("copy record = %+v", workspace.Record)
	}
	if workspace.Tmp != filepath.Join(bed.control, "artifacts", "agents", "workspaces", ".metasystem-tmp", workspace.Record.ID) {
		t.Fatalf("a copy's tmp lies in the reserved .metasystem-tmp, keyed by its record: %s", workspace.Tmp)
	}
	branch := "refs/heads/workspace/goal-g/bed"
	if bed.git.refs[branch] != "abc123" {
		t.Fatalf("refs = %v", bed.git.refs)
	}
	bed.git.refs[branch] = "d00d"
	bed.git.unique = "1"
	outcome := bed.release(workspace.Record.ID, emptyCensus(), nil)
	if !outcome.Done || outcome.Archive != "refs/archive/goal-g/bed/workspace/goal-g/bed" || outcome.Unique != 1 {
		t.Fatalf("release: %+v", outcome)
	}
	if bed.git.refs[outcome.Archive] != "d00d" {
		t.Fatalf("the archive ref holds the old tip: %v", bed.git.refs)
	}
	if _, ok := bed.git.refs[branch]; ok {
		t.Fatal("the branch is deleted after the archive")
	}
	for _, path := range []string{workspace.Record.Path, workspace.Tmp} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("%s is gone: %v", path, err)
		}
	}
	record, _ := bed.registry.Load(workspace.Record.ID)
	if !strings.Contains(strings.Join(record.Notes, "\n"), "1 commit") {
		t.Fatalf("the record counts the archived commits: %v", record.Notes)
	}
}

// A dirty copy is kept, naming work land and the person's discard; with a
// person's discard it is archived and released and the record names who.
func TestDirtyCopyIsKeptUntilAPersonDiscards(t *testing.T) {
	t.Parallel()
	bed := newWorkspaceBed(t)
	workspace := bed.obtain(goalG, "bed", "abc123")
	bed.git.dirty[workspace.Record.Path] = " M file.go\n"
	outcome := bed.release(workspace.Record.ID, emptyCensus(), nil)
	if outcome.Done || !outcome.Kept || !strings.Contains(outcome.Command, "metasystem work land g") ||
		!strings.Contains(outcome.Command, "metasystem work workspace g --release --discard --name bed") {
		t.Fatalf("dirty release: %+v", outcome)
	}
	if _, err := os.Stat(workspace.Record.Path); err != nil {
		t.Fatalf("the dirty copy stays: %v", err)
	}
	outcome = bed.release(workspace.Record.ID, emptyCensus(), &Discard{By: "Wido", At: testNow, Reason: "scratch"})
	if !outcome.Done {
		t.Fatalf("discarded release: %+v", outcome)
	}
	record, _ := bed.registry.Load(workspace.Record.ID)
	if record.AuthorizedDiscard == nil || record.AuthorizedDiscard.By != "Wido" {
		t.Fatalf("the record names the person's discard: %+v", record)
	}
}

// The checkout-use proof: a live process inside makes the release pending
// naming it (retried once it ended); a census not taken or incomplete is
// pending with disk clean --release.
func TestWorkspaceInUseIsKeptAndAnIncompleteCensusIsPending(t *testing.T) {
	t.Parallel()
	bed := newWorkspaceBed(t)
	workspace := bed.obtain(goalG, "", "")
	inside := &UseCensus{Taken: true, Processes: []CensusProcess{{Pid: 42, UID: 501, Command: "go test", Cwd: filepath.Join(workspace.Record.Path, "tmp")}}}
	if outcome := bed.release(workspace.Record.ID, inside, nil); !outcome.Pending || !strings.Contains(outcome.Reason, "pid 42") {
		t.Fatalf("in use: %+v", outcome)
	}
	incomplete := &UseCensus{Taken: true, Unreadable: []CensusGap{{Pid: 7, Reason: "denied"}}}
	outcome := bed.release(workspace.Record.ID, incomplete, nil)
	if !outcome.Pending || !strings.Contains(outcome.Command, "metasystem disk clean --release "+workspace.Record.ID) {
		t.Fatalf("incomplete census: %+v", outcome)
	}
	if outcome := bed.release(workspace.Record.ID, nil, nil); !outcome.Pending {
		t.Fatalf("no census: %+v", outcome)
	}
	if _, err := os.Stat(workspace.Record.Path); err != nil {
		t.Fatalf("a kept workspace stays: %v", err)
	}
}

// The repeat contract of 3.6 (R-129, DL2-20, DL3B-07).
func TestWorkspaceRepeatContract(t *testing.T) {
	t.Parallel()
	bed := newWorkspaceBed(t)
	first := bed.obtain(goalG, "bed", "abc123")
	again := bed.obtain(goalG, "bed", "abc123")
	if again.Created || again.Record.ID != first.Record.ID || len(bed.records()) != 1 {
		t.Fatalf("a repeat returns the same workspace and writes nothing: %+v", again)
	}
	_, err := ObtainWorkspace(context.Background(), bed.request(goalG, "bed", "beef"))
	var conflict *WorkspaceConflict
	if !errors.As(err, &conflict) || !strings.Contains(err.Error(), "--release") || !strings.Contains(err.Error(), "--name") {
		t.Fatalf("a different --copy-of under the same name: %v", err)
	}
	if outcome := bed.release(first.Record.ID, emptyCensus(), nil); !outcome.Done {
		t.Fatalf("release: %+v", outcome)
	}
	before := snapshotTree(t, bed.registry.Dir)
	if outcome := bed.release(first.Record.ID, emptyCensus(), nil); !outcome.Done || !outcome.Already {
		t.Fatalf("a second release succeeds: %+v", outcome)
	}
	if after := snapshotTree(t, bed.registry.Dir); after != before {
		t.Fatal("a second release writes nothing")
	}
	recreated := bed.obtain(goalG, "bed", "beef")
	if !recreated.Created || recreated.Record.ID == first.Record.ID || recreated.Record.Path != first.Record.Path {
		t.Fatalf("a create after release is a new record at the same path: %+v", recreated)
	}
	session := bed.obtain(Owner{Kind: OwnerSession, Ref: "s1"}, "bed", "")
	other := bed.obtain(Owner{Kind: OwnerSession, Ref: "s2"}, "bed", "")
	if session.Record.ID == other.Record.ID || session.Record.Path == other.Record.Path {
		t.Fatal("the same name under two session owners is two workspaces")
	}
}

// Two concurrent first requests serialize on the reservation lock and yield
// one workspace and one record.
func TestConcurrentFirstRequestsYieldOneWorkspace(t *testing.T) {
	t.Parallel()
	bed := newWorkspaceBed(t)
	var wait sync.WaitGroup
	ids := make([]string, 4)
	for index := range ids {
		wait.Add(1)
		go func() {
			defer wait.Done()
			workspace, err := ObtainWorkspace(context.Background(), bed.request(goalG, "", ""))
			if err != nil {
				t.Error(err)
				return
			}
			ids[index] = workspace.Record.ID
		}()
	}
	wait.Wait()
	for _, id := range ids {
		if id != ids[0] {
			t.Fatalf("ids = %v", ids)
		}
	}
	if records := bed.records(); len(records) != 1 {
		t.Fatalf("one record: %d", len(records))
	}
}

// A creation that fails removes what it made and its reservation, so the
// next request creates anew; a creation interrupted by a crash (a reserved
// record, nothing made yet) is discarded and recreated by the next
// request, never returned as it is.
func TestInterruptedCreationIsDiscardedAndRecreated(t *testing.T) {
	t.Parallel()
	bed := newWorkspaceBed(t)
	bed.git.failAdd = true
	if _, err := ObtainWorkspace(context.Background(), bed.request(goalG, "bed", "abc123")); err == nil || !strings.Contains(err.Error(), "nothing is left") {
		t.Fatalf("the failed creation reports it and what it left: %v", err)
	}
	for _, record := range bed.records() {
		if record.State != StateReleased {
			t.Fatalf("a failed creation leaves no reserved record: %+v", record)
		}
	}
	if _, found, _ := FindWorkspace(bed.registry, goalG, "bed"); found {
		t.Fatal("a failed creation leaves no reservation")
	}
	bed.git.failAdd = false
	crashed, err := bed.registry.Register(Registration{Path: WorkspacePath(bed.control, goalG, "bed"), Git: true, Class: WorkspaceClass, Owner: goalG,
		Checkout: bed.control, Lifetime: LifetimeOwner, CapKind: CapTarget, Layout: LayoutCopy, Reservation: mustKey(t, goalG, "bed"), CopyOf: "abc123"}, testNow, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	writeReservation(t, bed.registry, goalG, "bed", crashed.ID)
	workspace := bed.obtain(goalG, "bed", "abc123")
	if !workspace.Created || workspace.Record.ID == crashed.ID || workspace.Record.State != StateAccepted {
		t.Fatalf("recreated: %+v", workspace)
	}
	if old, _ := bed.registry.Load(crashed.ID); old.State != StateReleased {
		t.Fatalf("the crashed record is released: %+v", old)
	}
}

// A release while a verb holds the store's record lock is pending, never a
// wait and never a removal.
func TestWorkspaceReleaseWithAHeldRecordLockIsPending(t *testing.T) {
	t.Parallel()
	bed := newWorkspaceBed(t)
	workspace := bed.obtain(goalG, "", "")
	entrant, err := bed.registry.Enter(workspace.Record.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer entrant.Leave()
	if outcome := bed.release(workspace.Record.ID, emptyCensus(), nil); !outcome.Pending {
		t.Fatalf("held: %+v", outcome)
	}
}
