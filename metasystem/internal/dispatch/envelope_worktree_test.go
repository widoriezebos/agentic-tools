package dispatch

import (
	"encoding/json"
	"fmt"
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

// Metadata errors and branch guards refuse root derivation before any grant.
func TestWorktreeGitRootRefusals(t *testing.T) {
	dir := t.TempDir()
	worktree := filepath.Join(dir, "wt")
	common := filepath.Join(dir, ".git")
	gitDir := filepath.Join(common, "worktrees", "wt")
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct{ name, branch, reason string }{
		{"bare branch", "topic", "agent/ branch"},
		{"main branch", "main", "agent/ branch"},
		{"detached", "", "own branch"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			facts := worktreeFacts(t, worktree, gitDir, common, tc.branch)
			if _, err := worktreeGitWriteRootsWithMetadata(worktree, facts); err == nil || !strings.Contains(err.Error(), tc.reason) {
				t.Fatalf("branch %q must refuse: %v", tc.branch, err)
			}
		})
	}
	for _, tc := range []struct{ name, kind, reason string }{
		{"git dir", "git-dir", "worktree git dir unreadable"},
		{"common dir", "common-dir", "worktree common git dir unreadable"},
		{"branch", "branch", "own branch"},
	} {
		t.Run(tc.name+" read failure", func(t *testing.T) {
			reads := []gitFactRead{{kind: "git-dir", workspace: worktree, value: gitDir}, {kind: "common-dir", workspace: worktree, value: common}, {kind: "branch", workspace: worktree, value: "agent/j"}}
			for i := range reads {
				if reads[i].kind == tc.kind {
					reads[i].err = fmt.Errorf("declared read failure")
					reads = reads[:i+1]
					break
				}
			}
			if _, err := worktreeGitWriteRootsWithMetadata(worktree, declaredGitFacts(t, reads...)); err == nil || !strings.Contains(err.Error(), tc.reason) {
				t.Fatalf("%s error must refuse: %v", tc.kind, err)
			}
		})
	}
	absent := filepath.Join(dir, "absent")
	if _, err := worktreeGitWriteRootsWithMetadata(absent, declaredGitFacts(t, gitFactRead{kind: "git-dir", workspace: absent, err: fmt.Errorf("missing worktree")})); err == nil || !strings.Contains(err.Error(), "worktree git dir unreadable") {
		t.Fatalf("nonexistent worktree must refuse at metadata read: %v", err)
	}
}

// Filesystem guards still operate on real directories and refuse invalid layouts.
func TestWorktreeGitRootNegativeGuards(t *testing.T) {
	dir := t.TempDir()
	worktree := filepath.Join(dir, "wt")
	common := filepath.Join(dir, ".git")
	gitDir := filepath.Join(common, "worktrees", "wt")
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(common, "reftable"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := worktreeGitWriteRootsWithMetadata(worktree, declaredGitFacts(t,
		gitFactRead{kind: "git-dir", workspace: worktree, value: gitDir},
		gitFactRead{kind: "common-dir", workspace: worktree, value: common},
	)); err == nil || !strings.Contains(err.Error(), "reftable") {
		t.Fatalf("reftable repository not refused: %v", err)
	}
	if err := os.RemoveAll(filepath.Join(common, "reftable")); err != nil {
		t.Fatal(err)
	}

	// A file in the namespace path makes MkdirAll fail on every test UID.
	logs := filepath.Join(common, "logs", "refs")
	if err := os.MkdirAll(filepath.Dir(logs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(logs, []byte("blocked"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := worktreeGitWriteRootsWithMetadata(worktree, worktreeFacts(t, worktree, gitDir, common, "agent/j")); err == nil || !strings.Contains(err.Error(), "reflog namespace") {
		t.Fatalf("uncreatable reflog namespace did not fail closed: %v", err)
	}
}
