package diskstore

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type cloneBed struct {
	*realRepo
	clone string
}

// newCloneBed is a checkout and, beside it, a clone of it holding every
// kind of row: three branches (one not in the checkout), a unique commit at
// a detached HEAD, a commit only a tag reaches, a stash entry, a linked
// worktree outside the clone, a staged-only version of a file, and an
// initialized submodule with a commit of its own.
func newCloneBed(t *testing.T) *cloneBed {
	t.Helper()
	r := newRealRepo(t)
	root := filepath.Dir(r.repo)
	b := &cloneBed{realRepo: r, clone: filepath.Join(root, "clone")}
	r.git(root, "clone", "-q", r.repo, b.clone)
	r.git(b.clone, "config", "user.name", "t")
	r.git(b.clone, "config", "user.email", "t@example.com")
	r.git(b.clone, "branch", "side")
	r.git(b.clone, "checkout", "-q", "-b", "unpublished")
	r.git(b.clone, "commit", "-q", "--allow-empty", "-m", "only in the clone")
	r.git(b.clone, "commit", "-q", "--allow-empty", "-m", "tagged")
	r.git(b.clone, "tag", "only-tag")
	r.git(b.clone, "reset", "-q", "--hard", "HEAD~1")
	if err := os.WriteFile(filepath.Join(b.clone, "file.txt"), []byte("one\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	r.git(b.clone, "add", "file.txt")
	r.git(b.clone, "commit", "-q", "-m", "file")
	if err := os.WriteFile(filepath.Join(b.clone, "file.txt"), []byte("stashed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	r.git(b.clone, "stash", "-q")
	sub := filepath.Join(root, "subsrc")
	r.git(root, "init", "-q", "-b", "main", sub)
	r.git(sub, "commit", "-q", "--allow-empty", "-m", "sub base")
	r.git(b.clone, "-c", "protocol.file.allow=always", "submodule", "add", "-q", sub, "lib")
	r.git(b.clone, "commit", "-q", "-m", "submodule")
	r.git(filepath.Join(b.clone, "lib"), "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "sub unique")
	r.git(b.clone, "add", "lib")
	r.git(b.clone, "commit", "-q", "-m", "bump submodule")
	r.git(b.clone, "worktree", "add", "-q", "--detach", filepath.Join(root, "clone-wt"))
	r.git(filepath.Join(root, "clone-wt"), "commit", "-q", "--allow-empty", "-m", "detached in the linked worktree")
	if err := os.WriteFile(filepath.Join(b.clone, "file.txt"), []byte("staged only\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	r.git(b.clone, "add", "file.txt")
	if err := os.WriteFile(filepath.Join(b.clone, "file.txt"), []byte("one\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return b
}

func (b *cloneBed) release(discard *Discard) CloneRelease {
	b.t.Helper()
	outcome, err := ReleaseClone(context.Background(), CloneReleaseRequest{Registry: b.registry, GitRoot: b.repo, Path: b.clone,
		Git: realWorkspaceGit, Census: &UseCensus{Taken: true}, Discard: discard, Now: testNow})
	if err != nil {
		b.t.Fatal(err)
	}
	return outcome
}

// Every row of the clone is archived in the checkout's common store,
// verified there, and the clone and its linked worktree are removed; a
// repeat succeeds and names nothing new (U6d-2, R22).
func TestCloneReleaseRealGitArchivesEveryRowThenRemoves(t *testing.T) {
	t.Parallel()
	b := newCloneBed(t)
	wanted := map[string]string{
		"unpublished branch":  b.git(b.clone, "rev-parse", "refs/heads/unpublished~0"),
		"tag-only commit":     b.git(b.clone, "rev-parse", "only-tag^{commit}"),
		"stash entry":         b.git(b.clone, "rev-parse", "stash@{0}"),
		"linked worktree":     b.git(filepath.Join(filepath.Dir(b.clone), "clone-wt"), "rev-parse", "HEAD"),
		"submodule's own one": b.git(filepath.Join(b.clone, "lib"), "rev-parse", "HEAD"),
		"staged-only blob":    b.git(b.clone, "rev-parse", ":file.txt"),
	}
	// The index differs from HEAD by a staged change: the clone is dirty
	// until a person discards; the discard archives the index anyway.
	if outcome := b.release(nil); !outcome.Kept || !strings.Contains(outcome.Command, "--discard") {
		t.Fatalf("a dirty clone is kept naming the discard: %+v", outcome)
	}
	if _, err := os.Stat(b.clone); err != nil {
		t.Fatalf("the kept clone stays: %v", err)
	}
	outcome := b.release(&Discard{By: "Wido", At: testNow, Reason: "fixture"})
	if !outcome.Done {
		t.Fatalf("release: %+v", outcome)
	}
	for what, sha := range wanted {
		if _, err := realWorkspaceGit(context.Background(), b.repo, "cat-file", "-e", sha); err != nil {
			t.Errorf("the %s (%s) is not in the common store: %v", what, sha, err)
		}
		if what != "staged-only blob" && !b.reachable(sha) {
			t.Errorf("the %s (%s) is reachable from no archive ref", what, sha)
		}
	}
	for _, gone := range []string{b.clone, filepath.Join(filepath.Dir(b.clone), "clone-wt")} {
		if _, err := os.Stat(gone); !os.IsNotExist(err) {
			t.Errorf("%s is removed: %v", gone, err)
		}
	}
	refs := b.git(b.repo, "for-each-ref", "refs/archive/")
	if again := b.release(nil); !again.Done || !again.Already {
		t.Fatalf("a repeat succeeds: %+v", again)
	}
	if b.git(b.repo, "for-each-ref", "refs/archive/") != refs {
		t.Fatal("a repeat makes no second archive")
	}
}

// A release interrupted after its mapping resumes from it and never makes
// a second archive name; a gitlink without a git directory keeps the clone
// until a person discards, and the discard is persisted.
func TestCloneReleaseRealGitResumesFromItsMapping(t *testing.T) {
	t.Parallel()
	r := newRealRepo(t)
	root := filepath.Dir(r.repo)
	clone := filepath.Join(root, "clone")
	r.git(root, "clone", "-q", r.repo, clone)
	r.git(clone, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "unique")
	unique := r.git(clone, "rev-parse", "HEAD")
	r.git(clone, "update-index", "--add", "--cacheinfo", "160000,"+unique+",vendored")
	if err := os.WriteFile(filepath.Join(clone, ".gitmodules"), []byte("[submodule \"vendored\"]\n\tpath = vendored\n\turl = ../nowhere\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	r.git(clone, "add", ".gitmodules")
	if err := os.Mkdir(filepath.Join(clone, "vendored"), 0o700); err != nil {
		t.Fatal(err)
	}
	r.git(clone, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "-m", "gitlink")
	mapping, err := InventoryClone(context.Background(), realWorkspaceGit, clone, cloneArchiveBase(clone), testNow, CloneMapping{})
	if err != nil {
		t.Fatal(err)
	}
	blockers := cloneBlockers(context.Background(), realWorkspaceGit, clone, mapping, 1<<20)
	if len(blockers) == 0 || !strings.Contains(strings.Join(blockers, "\n"), "vendored: a gitlink without a git directory") {
		t.Fatalf("a gitlink without a git directory keeps the clone: %v", blockers)
	}
	if err := writeCloneMapping(CloneMappingPath(r.registry, clone), mapping); err != nil {
		t.Fatal(err)
	}
	request := CloneReleaseRequest{Registry: r.registry, GitRoot: r.repo, Path: clone, Git: realWorkspaceGit, Census: &UseCensus{Taken: true}, Now: testNow}
	if outcome, err := ReleaseClone(context.Background(), request); err != nil || !outcome.Kept {
		t.Fatalf("kept: %+v %v", outcome, err)
	}
	request.Discard = &Discard{By: "Wido", At: testNow, Reason: "the vendored gitlink is published elsewhere"}
	outcome, err := ReleaseClone(context.Background(), request)
	if err != nil || !outcome.Done || outcome.Base != mapping.Base {
		t.Fatalf("resumed: %+v %v", outcome, err)
	}
	if saved, _, _ := readCloneMapping(CloneMappingPath(r.registry, clone)); saved.Discard == nil || saved.Discard.Reason == "" {
		t.Fatalf("the person's discard is persisted: %+v", saved)
	}
	if !r.reachable(unique) {
		t.Fatal("the unique commit is archived")
	}
	if names := r.git(r.repo, "for-each-ref", "--format=%(refname)", "refs/archive/"); strings.Contains(names, "@") {
		t.Fatalf("no second archive name: %s", names)
	}
}

// Only a clone of this project beside the checkout is released this way.
func TestCloneReleaseRefusesWhatIsNotAClone(t *testing.T) {
	t.Parallel()
	r := newRealRepo(t)
	root := filepath.Dir(r.repo)
	other := filepath.Join(root, "other")
	r.git(root, "init", "-q", "-b", "main", other)
	r.git(other, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "unrelated")
	for _, path := range []string{other, r.repo, filepath.Join(r.repo, "metasystem")} {
		if _, err := ReleaseClone(context.Background(), CloneReleaseRequest{Registry: r.registry, GitRoot: r.repo, Path: path,
			Git: realWorkspaceGit, Census: &UseCensus{Taken: true}, Now: testNow}); err == nil && path != filepath.Join(r.repo, "metasystem") {
			t.Errorf("%s is refused", path)
		}
	}
	if _, err := os.Stat(other); err != nil {
		t.Fatalf("an unrelated repository is never removed: %v", err)
	}
}

// Work committed after a release was kept is archived by the next one: the
// inventory is taken afresh, never from the earlier mapping.
func TestCloneReleaseRealGitArchivesWorkAddedAfterAKeep(t *testing.T) {
	t.Parallel()
	r := newRealRepo(t)
	root := filepath.Dir(r.repo)
	clone := filepath.Join(root, "clone")
	r.git(root, "clone", "-q", r.repo, clone)
	if err := os.WriteFile(filepath.Join(clone, "work.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	request := CloneReleaseRequest{Registry: r.registry, GitRoot: r.repo, Path: clone, Git: realWorkspaceGit, Census: &UseCensus{Taken: true}, Now: testNow}
	if outcome, err := ReleaseClone(context.Background(), request); err != nil || !outcome.Kept {
		t.Fatalf("the dirty clone is kept: %+v %v", outcome, err)
	}
	r.git(clone, "add", "work.txt")
	r.git(clone, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "-m", "committed after the keep")
	committed := r.git(clone, "rev-parse", "HEAD")
	if outcome, err := ReleaseClone(context.Background(), request); err != nil || !outcome.Done {
		t.Fatalf("release: %+v %v", outcome, err)
	}
	if !r.reachable(committed) {
		t.Fatal("the work committed after the keep is archived")
	}
}

// An armed checkout of the host is never released as a clone.
func TestCloneReleaseRefusesAnArmedCheckout(t *testing.T) {
	t.Parallel()
	r := newRealRepo(t)
	root := filepath.Dir(r.repo)
	seat := filepath.Join(root, "seat")
	r.git(root, "clone", "-q", r.repo, seat)
	_, err := ReleaseClone(context.Background(), CloneReleaseRequest{Registry: r.registry, GitRoot: r.repo, Path: seat, Git: realWorkspaceGit,
		Census: &UseCensus{Taken: true}, Now: testNow, Protected: []string{seat}})
	if err == nil || !strings.Contains(err.Error(), "armed checkout") {
		t.Fatalf("an armed checkout is refused: %v", err)
	}
	if _, err := os.Stat(seat); err != nil {
		t.Fatal("the armed checkout stays")
	}
}

// Round B3-2 R4: a hook, a configuration beyond a clone's defaults, an
// info/exclude line, LFS objects and a worktree whose directory is gone
// each keep the clone, named; a repository beside it whose alternates read
// from it refuses the release outright.
func TestCloneReleaseKeepsWhatTheArchiveCannotCarry(t *testing.T) {
	t.Parallel()
	r := newRealRepo(t)
	root := filepath.Dir(r.repo)
	clone := filepath.Join(root, "clone")
	r.git(root, "clone", "-q", r.repo, clone)
	r.git(clone, "config", "alias.st", "status")
	r.git(clone, "worktree", "add", "-q", "--detach", filepath.Join(root, "gone-wt"))
	for path, content := range map[string]string{
		filepath.Join(clone, ".git", "hooks", "pre-commit"):             "#!/bin/sh\n",
		filepath.Join(clone, ".git", "info", "exclude"):                 "# comment\nscratch/\n",
		filepath.Join(clone, ".git", "lfs", "objects", "ab", "cd", "x"): "blob",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.RemoveAll(filepath.Join(root, "gone-wt")); err != nil {
		t.Fatal(err)
	}
	request := CloneReleaseRequest{Registry: r.registry, GitRoot: r.repo, Path: clone, Git: realWorkspaceGit, Census: &UseCensus{Taken: true}, Now: testNow}
	outcome, err := ReleaseClone(context.Background(), request)
	if err != nil || !outcome.Kept {
		t.Fatalf("kept: %+v %v", outcome, err)
	}
	mapping, _, _ := readCloneMapping(CloneMappingPath(r.registry, clone))
	joined := strings.Join(mapping.Blockers, "\n")
	for _, want := range []string{"hook pre-commit", "configuration alias.st", "info/exclude line scratch/", "LFS objects", "gone-wt: a worktree of the clone whose directory is gone"} {
		if !strings.Contains(joined, want) {
			t.Errorf("blockers lack %q:\n%s", want, joined)
		}
	}
	reader := filepath.Join(root, "reader")
	r.git(root, "init", "-q", reader)
	if err := os.WriteFile(filepath.Join(reader, ".git", "objects", "info", "alternates"), []byte(filepath.Join(clone, ".git", "objects")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	request.Discard = &Discard{By: "Wido", At: testNow, Reason: "fixture"}
	if _, err := ReleaseClone(context.Background(), request); err == nil || !strings.Contains(err.Error(), "alternates") {
		t.Fatalf("a repository reading through the clone refuses the release: %v", err)
	}
	if _, err := os.Stat(clone); err != nil {
		t.Fatal("the clone stays")
	}
}
