package main

// Adapter tests drive the public discard command with real linked worktrees;
// all job records, store records and object stores belong to the fixture.
import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

type diskOrphanBed struct {
	*diskBed
	worktree, quarantine, commit string
	proof                        dispatchcore.DelegateWorkspaceProof
}

func diskOrphanWrite(t *testing.T, path, content string) {
	t.Helper()
	helmMust(t, os.MkdirAll(filepath.Dir(path), 0o755), os.WriteFile(path, []byte(content), 0o644))
}
func diskOrphanGit(t *testing.T, dir string, env []string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(gittree.ScrubbedEnviron("GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1", "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com"), env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}
func newDiskOrphanBed(t *testing.T, registered bool) *diskOrphanBed {
	t.Helper()
	b := &diskOrphanBed{diskBed: newDiskBed(t)}
	diskOrphanGit(t, b.root, nil, "init", "-q", "-b", "main")
	diskOrphanWrite(t, filepath.Join(b.root, "README"), "base\n")
	diskOrphanWrite(t, filepath.Join(b.root, ".gitignore"), "metasystem/artifacts/\n")
	diskOrphanGit(t, b.root, nil, "add", "README", ".gitignore")
	diskOrphanGit(t, b.root, nil, "commit", "-q", "-m", "base")
	b.worktree = filepath.Join(b.inst, "artifacts", "agents", "worktrees", "orphan-1")
	diskOrphanGit(t, b.root, nil, "worktree", "add", "-q", "-b", "agent/orphan-1", b.worktree)
	id, err := diskstore.ReadGitIdentity(b.worktree)
	helmMust(t, err)
	b.quarantine = filepath.Join(id.Gitdir, diskstore.QuarantineName)
	helmMust(t, os.MkdirAll(b.quarantine, 0o755), diskstore.AddAlternate(filepath.Join(b.root, ".git", "objects"), b.quarantine))
	diskOrphanWrite(t, filepath.Join(b.worktree, "work.txt"), "orphan work\n")
	diskOrphanGit(t, b.worktree, b.env(), "add", "work.txt")
	diskOrphanGit(t, b.worktree, b.env(), "commit", "-q", "-m", "orphan work")
	b.commit = diskOrphanGit(t, b.worktree, nil, "rev-parse", "HEAD")
	b.proof = dispatchcore.DelegateWorkspaceProof{Repo: b.inst, GitRoot: b.root, Git: steward.ExecWorkspaceGit, Now: diskNow, Stage: "01K2Z7Q3M8XW1V0P9D4J6S5R2T"}
	b.owners.disk.discarder = func(string, time.Time) (diskstore.Discarder, error) { return b.proof, nil }
	if registered {
		_, err = diskstore.AdmitDelegate(context.Background(), diskstore.CheckoutRegistry(b.inst), b.inst, b.root, b.worktree, steward.ExecWorkspaceGit, diskNow)
		helmMust(t, err)
	}
	return b
}
func (b *diskOrphanBed) env() []string {
	return []string{"GIT_OBJECT_DIRECTORY=" + b.quarantine, "GIT_ALTERNATE_OBJECT_DIRECTORIES=" + filepath.Join(b.root, ".git", "objects")}
}
func (b *diskOrphanBed) tips(t *testing.T) []string {
	t.Helper()
	tree := diskOrphanGit(t, b.worktree, nil, "rev-parse", "HEAD^{tree}")
	base := diskOrphanGit(t, b.root, nil, "rev-parse", "main")
	commit := func(message string) string {
		return diskOrphanGit(t, b.worktree, b.env(), "commit-tree", tree, "-p", base, "-m", message)
	}
	branchLog, headLog, head := commit("branch reflog only"), commit("HEAD reflog only"), commit("detached HEAD")
	diskOrphanGit(t, b.worktree, nil, "update-ref", "--create-reflog", "refs/heads/agent/orphan-1", branchLog)
	diskOrphanGit(t, b.worktree, nil, "update-ref", "refs/heads/agent/orphan-1", b.commit)
	diskOrphanGit(t, b.worktree, nil, "checkout", "-q", "--detach", b.commit)
	diskOrphanGit(t, b.worktree, nil, "update-ref", "--create-reflog", "HEAD", headLog)
	diskOrphanGit(t, b.worktree, nil, "update-ref", "HEAD", head)
	return []string{b.commit, branchLog, headLog, head}
}
func (b *diskOrphanBed) assertKept(t *testing.T) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(b.worktree, "work.txt")); err != nil {
		t.Fatalf("workspace removed: %v", err)
	}
	out, err := os.ReadFile(filepath.Join(b.root, ".git", "objects", "info", "alternates"))
	helmMust(t, err)
	if !strings.Contains(string(out), b.quarantine) {
		t.Fatalf("alternate removed: %s", out)
	}
}

func TestDiskDiscardFreshOrphanArchivesEveryIndependentTipAndAbsorbs(t *testing.T) {
	t.Parallel()
	for _, registered := range []bool{false, true} {
		t.Run(map[bool]string{false: "legacy", true: "registered"}[registered], func(t *testing.T) {
			t.Parallel()
			b := newDiskOrphanBed(t, registered)
			tips := b.tips(t)
			diskOrphanWrite(t, filepath.Join(b.worktree, ".gitmodules"), "")
			expected := map[string][]byte{}
			helmMust(t, filepath.WalkDir(b.quarantine, func(path string, entry os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				rel, _ := filepath.Rel(b.quarantine, path)
				if entry.IsDir() || len(filepath.Dir(rel)) != 2 || len(entry.Name()) != 38 {
					return nil
				}
				expected[rel], err = os.ReadFile(path)
				return err
			}))
			code, out := b.run("disk", "clean", "--discard", b.worktree, "--reason", "abandoned fixture", "--json")
			if code != 0 || !strings.Contains(out, `"done": true`) {
				t.Fatalf("discard=%d: %s", code, out)
			}
			if _, err := os.Stat(b.worktree); !os.IsNotExist(err) {
				t.Fatalf("worktree still exists: %v", err)
			}
			if _, err := os.Stat(b.quarantine); !os.IsNotExist(err) {
				t.Fatalf("quarantine still exists: %v", err)
			}
			refs := strings.Fields(diskOrphanGit(t, b.root, nil, "for-each-ref", "--format=%(refname)", "refs/archive/"))
			for _, tip := range tips {
				reachable := false
				for _, ref := range refs {
					if diskOrphanGit(t, b.root, nil, "rev-parse", ref) == tip {
						reachable = true
						break
					}
				}
				if !reachable {
					t.Errorf("independent tip %s is not archived: %v", tip, refs)
				}
				diskOrphanGit(t, b.root, nil, "rev-list", "--objects", tip)
			}
			if len(expected) == 0 {
				t.Fatal("fixture recorded no objects")
			}
			for rel, want := range expected {
				got, err := os.ReadFile(filepath.Join(b.root, ".git", "objects", rel))
				if err != nil || !bytes.Equal(got, want) {
					t.Errorf("common object %s: %v", rel, err)
				}
			}
			records, bad := diskstore.CheckoutRegistry(b.inst).Inventory()
			if len(bad) != 0 || len(records) != 1 || records[0].State != diskstore.StateReleased || !strings.Contains(strings.Join(records[0].Notes, " "), "abandoned fixture") {
				t.Fatalf("release not journaled: %+v %+v", records, bad)
			}
		})
	}
}

func TestDiskDiscardOrphanFailuresPreventRemoval(t *testing.T) {
	t.Parallel()
	for _, failure := range []string{"archive", "HEAD reflog", "branch reflog", "live user"} {
		t.Run(failure, func(t *testing.T) {
			t.Parallel()
			b := newDiskOrphanBed(t, false)
			b.tips(t)
			injected := false
			b.proof.Git = func(ctx context.Context, dir string, args ...string) ([]byte, error) {
				if len(args) > 0 && args[0] == "update-ref" && failure == "archive" || len(args) > 3 && args[0] == "log" && (failure == "HEAD reflog" && args[3] == "HEAD" || failure == "branch reflog" && strings.HasPrefix(args[3], "refs/heads/")) {
					injected = true
					return nil, errors.New("fixture " + failure + " failed")
				}
				return steward.ExecWorkspaceGit(ctx, dir, args...)
			}
			want := "fixture " + failure + " failed"
			if failure == "live user" {
				want = "in use by pid 77"
				b.census.Processes = []diskstore.CensusProcess{{Pid: 77, UID: 501, Command: "fixture holder", Cwd: b.worktree}}
			}
			code, out := b.run("disk", "clean", "--discard", b.worktree, "--reason", "fixture", "--json")
			if code != 0 || !strings.Contains(out, want) || strings.Contains(out, `"done": true`) {
				t.Fatalf("discard=%d: %s", code, out)
			}
			if failure != "live user" && !injected {
				t.Fatal("failure seam was not reached")
			}
			b.assertKept(t)
		})
	}
}

func TestDiskDiscardOrphanProtectsIndependentStores(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"nested directory", "nested git file", "submodule history", "registered descendant"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			b := newDiskOrphanBed(t, false)
			want := ""
			switch kind {
			case "nested directory":
				nested := filepath.Join(b.worktree, "nested")
				helmMust(t, os.MkdirAll(nested, 0o755))
				diskOrphanGit(t, nested, nil, "init", "-q", "-b", "main")
				diskOrphanWrite(t, filepath.Join(nested, "unique"), "independent history\n")
				diskOrphanGit(t, nested, nil, "add", ".")
				diskOrphanGit(t, nested, nil, "commit", "-q", "-m", "unique")
				want = "nested repository"
			case "nested git file":
				diskOrphanWrite(t, filepath.Join(b.worktree, "nested", ".git"), "gitdir: elsewhere\n")
				want = "nested repository"
			case "submodule history":
				diskOrphanWrite(t, filepath.Join(filepath.Dir(b.quarantine), "modules", "sub", "history"), "unique history")
				want = "submodule history"
			case "registered descendant":
				registry := diskstore.CheckoutRegistry(b.inst)
				child, err := registry.Register(diskstore.Registration{Path: filepath.Join(b.worktree, "child"), Class: "fixture", Owner: diskstore.Owner{Kind: diskstore.OwnerProcess, Ref: "child"}, Lifetime: diskstore.LifetimeOwner, CapKind: diskstore.CapTarget}, diskNow, rand.Reader)
				helmMust(t, err)
				helmMust(t, os.MkdirAll(child.Path, 0o755), diskstore.WriteMarker(child))
				_, err = registry.Accept(child.ID)
				helmMust(t, err)
				want = "independently registered descendant"
			}
			before := diskSnapshot(t, b.worktree)
			code, out := b.run("disk", "clean", "--discard", b.worktree, "--reason", "fixture", "--json")
			if code != 0 || !strings.Contains(out, want) || strings.Contains(out, `"done": true`) {
				t.Fatalf("discard=%d: %s", code, out)
			}
			b.assertKept(t)
			if len(diskSnapshotDiff(before, diskSnapshot(t, b.worktree))) != 0 {
				t.Fatal("protected content changed")
			}
		})
	}
}

func TestDiskDiscardPathAdmissionAndReason(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"reason missing", "hand made", "other repository", "copied link"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			b := newDiskOrphanBed(t, false)
			target := b.worktree
			if kind == "hand made" {
				target = filepath.Join(b.root, "hand-made")
				diskOrphanGit(t, b.root, nil, "worktree", "add", "-q", "--detach", target)
			}
			if kind == "copied link" {
				target = filepath.Join(b.inst, "artifacts", "agents", "worktrees", "copied-link")
				link, err := os.ReadFile(filepath.Join(b.worktree, ".git"))
				helmMust(t, err)
				diskOrphanWrite(t, filepath.Join(target, ".git"), string(link))
			}
			if kind == "other repository" {
				other := t.TempDir()
				diskOrphanGit(t, other, nil, "init", "-q", "-b", "main")
				diskOrphanGit(t, other, nil, "commit", "--allow-empty", "-q", "-m", "other")
				target = filepath.Join(b.inst, "artifacts", "agents", "worktrees", "other")
				diskOrphanGit(t, other, nil, "worktree", "add", "-q", "--detach", target)
			}
			args := []string{"disk", "clean", "--discard", target, "--json"}
			if kind != "reason missing" {
				args = append(args, "--reason", "fixture")
			}
			code, out := b.run(args...)
			if code != 2 {
				t.Fatalf("admission=%d: %s", code, out)
			}
			records, bad := diskstore.CheckoutRegistry(b.inst).Inventory()
			if len(records) != 0 || len(bad) != 0 {
				t.Fatalf("refusal registered a store: %+v %+v", records, bad)
			}
			if _, err := os.Stat(target); err != nil {
				t.Fatalf("refusal removed target: %v", err)
			}
			b.assertKept(t)
		})
	}
}

func TestDiskDelegatePreviewAndShowBorrowing(t *testing.T) {
	t.Parallel()
	b := newDiskOrphanBed(t, true)
	b.owners.disk.pass = func(ctx context.Context, top string, pass steward.DiskPass) (steward.DiskPassResult, error) {
		pass.SkipMachine = true
		pass.Home = b.home
		pass.Processes = identity.ListedProcessTable{}
		pass.Proofs = map[diskstore.OwnerKind]diskstore.OwnerProof{diskstore.OwnerDelegate: b.proof}
		return steward.SweepDiskStores(ctx, top, pass)
	}
	before := diskSnapshot(t, filepath.Join(b.root, ".git", "objects"))
	code, out := b.run("disk", "clean", "--preview", "--json")
	if code != 0 || !strings.Contains(out, `"absorbable": true`) {
		t.Fatalf("preview=%d: %s", code, out)
	}
	if diff := diskSnapshotDiff(before, diskSnapshot(t, filepath.Join(b.root, ".git", "objects"))); len(diff) != 0 {
		t.Fatalf("preview changed objects: %v", diff)
	}
	b.assertKept(t)
	// A report-only pass supplies disk show's read-only report.
	_, err := steward.SweepDiskStores(context.Background(), b.inst, steward.DiskPass{Mode: diskstore.ModeReport, Now: diskNow, Clock: time.Now, SkipMachine: true, Home: b.home, Processes: identity.ListedProcessTable{}, Proofs: map[diskstore.OwnerKind]diskstore.OwnerProof{diskstore.OwnerDelegate: b.proof}})
	helmMust(t, err)
	code, out = b.run("disk", "show", "--verbose")
	if code != 0 || !strings.Contains(flatPage(out), "the repository borrows its objects; remove it only through metasystem disk clean") {
		t.Fatalf("borrowed show=%d: %s", code, out)
	}
	code, out = b.run("disk", "clean", "--json")
	if code != 0 || !strings.Contains(flatPage(out), "its objects are in the repository") {
		t.Fatalf("apply=%d: %s", code, out)
	}
	code, out = b.run("disk", "show", "--verbose")
	if code != 0 || !strings.Contains(flatPage(out), "its objects are in the repository") || strings.Contains(out, "git worktree remove") {
		t.Fatalf("absorbed show=%d: %s", code, out)
	}
}

func TestDiskDiscardDispatchInProgressKeepsObjects(t *testing.T) {
	t.Parallel()
	for _, registered := range []bool{false, true} {
		t.Run(map[bool]string{false: "legacy", true: "registered"}[registered], func(t *testing.T) {
			t.Parallel()
			b := newDiskOrphanBed(t, registered)
			exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
			if err != nil || state != identity.Alive || !exact.ArgvKnown || len(exact.Argv) == 0 {
				t.Fatalf("fixture must identify its live kernel command: %+v %v", exact, err)
			}
			owner, err := json.Marshal(map[string]any{"pid": os.Getpid(), "instanceTag": strings.Join(exact.Argv, " ")})
			helmMust(t, err)
			path := filepath.Join(b.inst, "artifacts", "agents", "locks", "orphan-1.d")
			diskOrphanWrite(t, filepath.Join(path, "owner.json"), string(owner))
			if !b.census.Taken || !b.census.Complete() || len(b.census.Holders(b.worktree)) != 0 {
				t.Fatal("discard fixture must have an empty, complete use census")
			}
			objects := diskSnapshot(t, filepath.Join(b.root, ".git", "objects"))
			quarantine := diskSnapshot(t, b.quarantine)
			code, out := b.run("disk", "clean", "--discard", b.worktree, "--reason", "fixture", "--json")
			if code != 0 || !strings.Contains(out, "a dispatch of this chain is in progress") || strings.Contains(out, `"done": true`) {
				t.Fatalf("live dispatch discard=%d: %s", code, out)
			}
			b.assertKept(t)
			if diff := diskSnapshotDiff(objects, diskSnapshot(t, filepath.Join(b.root, ".git", "objects"))); len(diff) != 0 {
				t.Fatalf("discard changed objects or alternates: %v", diff)
			}
			if diff := diskSnapshotDiff(quarantine, diskSnapshot(t, b.quarantine)); len(diff) != 0 {
				t.Fatalf("discard changed quarantine: %v", diff)
			}
			got, err := os.ReadFile(filepath.Join(path, "owner.json"))
			if err != nil || !bytes.Equal(got, owner) {
				t.Fatalf("discard changed launch owner: %v", err)
			}
		})
	}
}
