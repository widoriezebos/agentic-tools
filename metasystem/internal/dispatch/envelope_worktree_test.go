package dispatch

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Issue #5 (quarantine design): a worktree envelope gains EXACTLY three
// derived git roots — the worktree's git dir and the agent ref namespace
// with its reflog — and never the shared object store (loose objects go
// to the worktree-local quarantine) nor main's ref. The acceptance probe
// commits UNDER THE QUARANTINE CONTRACT (GIT_OBJECT_DIRECTORY routed,
// alternates-linked) and proves every .git write stayed inside the
// granted roots while main's ref never moved.
func TestWorktreeEnvelopeGrantsGitRoots(t *testing.T) {
	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")
	os.MkdirAll(repo, 0o755)
	git := func(workdir string, env []string, args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", workdir}, args...)...)
		if env != nil {
			cmd.Env = append(os.Environ(), env...)
		}
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git(repo, nil, "init", "-q", "-b", "main")
	os.WriteFile(filepath.Join(repo, "f.txt"), []byte("base\n"), 0o644)
	git(repo, nil, "add", ".")
	git(repo, nil, "-c", "user.name=t", "-c", "user.email=t@x", "commit", "-qm", "base")
	worktree := filepath.Join(dir, "wt")
	git(repo, nil, "worktree", "add", "-q", "-b", "agent/job-1", worktree)

	// The engine's worktree-dispatch quarantine (dispatch.sh): private
	// object store inside the worktree's PRIVATE GIT DIR — outside the
	// shippable projection — alternates-linked.
	quarantine := filepath.Join(repo, ".git", "worktrees", "wt", "objects-quarantine")
	os.MkdirAll(quarantine, 0o755)
	commonObjects := filepath.Join(repo, ".git", "objects")
	os.MkdirAll(filepath.Join(commonObjects, "info"), 0o755)
	os.WriteFile(filepath.Join(commonObjects, "info", "alternates"), []byte(quarantine+"\n"), 0o644)

	source := filepath.Join(dir, "preset.json")
	os.WriteFile(source, []byte(`{"readRoots":["."],"writeRoots":["<worktree>"],"network":"deny","approvals":"deny","tools":"runtime-default"}`), 0o644)
	output := filepath.Join(dir, "envelope.json")
	if err := ExpandPermissions(source, repo, worktree, true, "workspace", "", output); err != nil {
		t.Fatalf("expand: %v", err)
	}
	data, _ := os.ReadFile(output)
	var envelope map[string]any
	json.Unmarshal(data, &envelope)
	var got []string
	for _, r := range envelope["writeRoots"].([]any) {
		got = append(got, r.(string))
	}
	want := []string{
		resolvePath(worktree),
		resolvePath(filepath.Join(repo, ".git", "worktrees", "wt")),
		resolvePath(filepath.Join(repo, ".git", "refs", "heads", "agent")),
		resolvePath(filepath.Join(repo, ".git", "logs", "refs", "heads", "agent")),
	}
	sort.Strings(got)
	sort.Strings(want)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("write roots are not EXACTLY the granted set:\ngot  %v\nwant %v", got, want)
	}

	// The acceptance probe under the quarantine contract.
	mainRefBefore := git(repo, nil, "rev-parse", "refs/heads/main")
	before := gitStateInventory(t, repo)
	os.WriteFile(filepath.Join(worktree, "g.txt"), []byte("new\n"), 0o644)
	quarantineEnv := []string{
		"GIT_OBJECT_DIRECTORY=" + quarantine,
		"GIT_ALTERNATE_OBJECT_DIRECTORIES=" + commonObjects,
		"GIT_CONFIG_COUNT=2",
		"GIT_CONFIG_KEY_0=maintenance.auto", "GIT_CONFIG_VALUE_0=false",
		"GIT_CONFIG_KEY_1=gc.auto", "GIT_CONFIG_VALUE_1=0",
	}
	git(worktree, quarantineEnv, "add", "g.txt")
	git(worktree, quarantineEnv, "-c", "user.name=t", "-c", "user.email=t@x", "commit", "-qm", "delegate round")
	after := gitStateInventory(t, repo)
	granted := want
	for path := range after {
		if before[path] == after[path] {
			continue
		}
		inside := false
		for _, root := range granted {
			if strings.HasPrefix(path+"/", root+"/") || path == root {
				inside = true
				break
			}
		}
		if !inside {
			t.Fatalf("commit wrote outside the granted roots: %s", path)
		}
	}
	if git(repo, nil, "rev-parse", "refs/heads/main") != mainRefBefore {
		t.Fatal("a worktree commit moved main")
	}
	// Forced-threshold maintenance proof: even with the
	// REPOSITORY config demanding gc at every object, the quarantine
	// env's maintenance.auto/gc.auto override wins and packed-refs
	// never appears from a delegate commit.
	git(repo, nil, "config", "gc.auto", "1")
	os.WriteFile(filepath.Join(worktree, "h.txt"), []byte("more\n"), 0o644)
	git(worktree, quarantineEnv, "add", "h.txt")
	git(worktree, quarantineEnv, "-c", "user.name=t", "-c", "user.email=t@x", "commit", "-qm", "second round")
	if _, err := os.Stat(filepath.Join(repo, ".git", "packed-refs")); !os.IsNotExist(err) {
		t.Fatal("a delegate commit triggered maintenance: packed-refs written outside the grant")
	}
	if out := git(repo, quarantineEnv, "fsck", "--no-progress"); strings.Contains(out, "error") {
		t.Fatalf("fsck unhappy: %s", out)
	}
	// The delegate's commit is readable from the MAIN repo through the
	// alternates link — conformance and merge depend on exactly this.
	git(repo, nil, "cat-file", "-e", git(worktree, quarantineEnv, "rev-parse", "HEAD"))
}

// gitStateInventory maps every file under the repo's .git to its mtime+size
// signature, for asserting which paths a commit touched.
func gitStateInventory(t *testing.T, repo string) map[string]string {
	t.Helper()
	inventory := map[string]string{}
	root := filepath.Join(repo, ".git")
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		inventory[resolvePath(path)] = info.ModTime().String() + "|" + string(rune(info.Size()))
		return nil
	})
	return inventory
}

// Refusals: a detached worktree, a worktree on main, and a bare-named
// branch all refuse root derivation by name.
func TestWorktreeGitRootRefusals(t *testing.T) {
	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")
	os.MkdirAll(repo, 0o755)
	git := func(workdir string, args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", workdir}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git(repo, "init", "-q", "-b", "main")
	os.WriteFile(filepath.Join(repo, "f.txt"), []byte("base\n"), 0o644)
	git(repo, "add", ".")
	git(repo, "-c", "user.name=t", "-c", "user.email=t@x", "commit", "-qm", "base")

	bare := filepath.Join(dir, "wt-bare")
	git(repo, "worktree", "add", "-q", "-b", "topic", bare)
	if _, err := worktreeGitWriteRoots(bare); err == nil || !strings.Contains(err.Error(), "agent/ branch") {
		t.Fatalf("bare-named branch must refuse: %v", err)
	}

	detached := filepath.Join(dir, "wt-detached")
	git(repo, "worktree", "add", "-q", "--detach", detached)
	if _, err := worktreeGitWriteRoots(detached); err == nil || !strings.Contains(err.Error(), "own branch") {
		t.Fatalf("detached worktree must refuse: %v", err)
	}

	if _, err := worktreeGitWriteRoots(filepath.Join(dir, "absent")); err == nil {
		t.Fatal("nonexistent worktree must refuse")
	}
}

// The negative guards prove themselves — a reftable
// repository refuses by name, and a reflog namespace that cannot be
// created fails the expansion closed instead of losing a delegate cycle.
func TestWorktreeGitRootNegativeGuards(t *testing.T) {
	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")
	os.MkdirAll(repo, 0o755)
	git := func(workdir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", workdir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	git(repo, "init", "-q", "-b", "main")
	os.WriteFile(filepath.Join(repo, "f.txt"), []byte("base\n"), 0o644)
	git(repo, "add", ".")
	git(repo, "-c", "user.name=t", "-c", "user.email=t@x", "commit", "-qm", "base")
	worktree := filepath.Join(dir, "wt")
	git(repo, "worktree", "add", "-q", "-b", "agent/j", worktree)

	// A reftable marker refuses by name.
	os.MkdirAll(filepath.Join(repo, ".git", "reftable"), 0o755)
	if _, err := worktreeGitWriteRoots(worktree); err == nil || !strings.Contains(err.Error(), "reftable") {
		t.Fatalf("reftable repository not refused: %v", err)
	}
	os.RemoveAll(filepath.Join(repo, ".git", "reftable"))

	// An uncreatable reflog namespace fails closed.
	logs := filepath.Join(repo, ".git", "logs")
	os.MkdirAll(logs, 0o755)
	os.RemoveAll(filepath.Join(logs, "refs"))
	if err := os.Chmod(logs, 0o555); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(logs, 0o755)
	if _, err := worktreeGitWriteRoots(worktree); err == nil || !strings.Contains(err.Error(), "reflog namespace") {
		t.Fatalf("uncreatable reflog namespace did not fail closed: %v", err)
	}
}
