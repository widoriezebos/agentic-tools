package landing

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/contractmerge"
)

type advanceFixture struct {
	t      *testing.T
	root   string
	peer   string
	remote string
}

func newAdvanceFixture(t *testing.T) *advanceFixture {
	t.Helper()
	base := t.TempDir()
	seed := filepath.Join(base, "seed")
	remote := filepath.Join(base, "origin.git")
	root := filepath.Join(base, "local")
	peer := filepath.Join(base, "peer")
	if err := os.MkdirAll(seed, 0o755); err != nil {
		t.Fatal(err)
	}
	runAdvanceGit(t, seed, "init", "-q", "-b", "main")
	configureAdvanceGit(t, seed)
	writeAdvanceFile(t, seed, ".gitignore", "artifacts/\n")
	writeAdvanceFile(t, seed, ".gitattributes", "memory/receipts.log merge=union\nrecords/narrator-digest.log merge=union\n")
	writeAdvanceFile(t, seed, "product.txt", "seed\n")
	writeAdvanceFile(t, seed, "scripts/agents/go-gate.sh", "seed\n")
	writeAdvanceFile(t, seed, "memory/receipts.log", "receipt=seed\n")
	writeAdvanceFile(t, seed, "records/narrator-digest.log", "digest=seed\n")
	runAdvanceGit(t, seed, "add", ".")
	runAdvanceGit(t, seed, "commit", "-qm", "seed")
	runAdvanceGit(t, base, "init", "--bare", "-q", remote)
	runAdvanceGit(t, remote, "symbolic-ref", "HEAD", "refs/heads/main")
	runAdvanceGit(t, seed, "remote", "add", "origin", remote)
	runAdvanceGit(t, seed, "push", "-q", "-u", "origin", "main")
	runAdvanceGit(t, base, "clone", "-q", remote, root)
	runAdvanceGit(t, base, "clone", "-q", remote, peer)
	configureAdvanceGit(t, root)
	configureAdvanceGit(t, peer)
	return &advanceFixture{t: t, root: root, peer: peer, remote: remote}
}

func configureAdvanceGit(t *testing.T, root string) {
	t.Helper()
	runAdvanceGit(t, root, "config", "user.name", "advance fixture")
	runAdvanceGit(t, root, "config", "user.email", "advance@example.invalid")
}

func runAdvanceGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	cmd.Env = gittree.ScrubbedEnviron()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git -C %s %v: %v\n%s", root, args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func advanceGitOutput(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	cmd.Env = gittree.ScrubbedEnviron()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git -C %s %v: %v\n%s", root, args, err, out)
	}
	return strings.TrimSuffix(string(out), "\n")
}

func writeAdvanceFile(t *testing.T, root, path, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func appendAdvanceFile(t *testing.T, root, path, content string) {
	t.Helper()
	appendReceiptFixtureFile(t, filepath.Join(root, filepath.FromSlash(path)), content)
}

func (f *advanceFixture) commitLocalLanding() string {
	f.t.Helper()
	appendAdvanceFile(f.t, f.root, "scripts/agents/go-gate.sh", "landing\n")
	runAdvanceGit(f.t, f.root, "add", "--", "scripts/agents/go-gate.sh")
	runAdvanceGit(f.t, f.root, "commit", "-qm", "landing")
	return runAdvanceGit(f.t, f.root, "rev-parse", "HEAD")
}

func (f *advanceFixture) fetch() {
	f.t.Helper()
	runAdvanceGit(f.t, f.root, "fetch", "-q", "origin", "+refs/heads/main:refs/remotes/origin/main")
}

func (f *advanceFixture) assertOneWorktree() {
	f.t.Helper()
	if count := strings.Count(runAdvanceGit(f.t, f.root, "worktree", "list", "--porcelain"), "worktree "); count != 1 {
		f.t.Fatalf("registered worktrees = %d, want one", count)
	}
}

func TestAdvanceRebaseMergesTestingContractBySurface(t *testing.T) {
	root := t.TempDir()
	runAdvanceGit(t, root, "init", "-q", "-b", "main")
	configureAdvanceGit(t, root)
	fixture := func(name string) []byte {
		t.Helper()
		data, err := os.ReadFile(filepath.Join("..", "testpolicy", "contractmerge", "testdata", "history-"+name+".json"))
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	writeAdvanceFile(t, root, ".gitattributes", "metasystem/testing.json merge=metasystem-testing\n")
	writeAdvanceFile(t, root, ".gitignore", "artifacts/\n")
	writeAdvanceFile(t, root, "metasystem/testing.json", string(fixture("base")))
	runAdvanceGit(t, root, "add", ".")
	runAdvanceGit(t, root, "commit", "-qm", "base")
	base := runAdvanceGit(t, root, "rev-parse", "HEAD")
	writeAdvanceFile(t, root, "metasystem/testing.json", string(fixture("ours")))
	runAdvanceGit(t, root, "add", "metasystem/testing.json")
	runAdvanceGit(t, root, "commit", "-qm", "landing testing contract")
	landing := runAdvanceGit(t, root, "rev-parse", "HEAD")
	runAdvanceGit(t, root, "switch", "--quiet", "-c", "upstream", base)
	writeAdvanceFile(t, root, "metasystem/testing.json", string(fixture("theirs")))
	runAdvanceGit(t, root, "add", "metasystem/testing.json")
	runAdvanceGit(t, root, "commit", "-qm", "upstream testing contract")
	runAdvanceGit(t, root, "switch", "--quiet", "main")

	t.Setenv("METASYSTEM_CONTRACT_DRIVER_HELPER", "1")
	if err := Advance(root, "upstream", &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if head := runAdvanceGit(t, root, "rev-parse", "HEAD"); head == landing {
		t.Fatal("surface-aware rebase did not move the landing commit")
	}
	got, err := os.ReadFile(filepath.Join(root, "metasystem", "testing.json"))
	if err != nil {
		t.Fatal(err)
	}
	want, err := contractmerge.MergeBytes(fixture("base"), fixture("theirs"), fixture("ours"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("rebased testing contract differs from semantic merge:\n%s", got)
	}
	if status := runAdvanceGit(t, root, "status", "--porcelain=v1"); status != "" {
		t.Fatalf("surface-aware rebase left changes: %q", status)
	}
}

// Native Git proves the physical effects that the raw policy transcript cannot observe.
func TestAdvanceNativeAdapter(t *testing.T) {
	t.Run("reset preserves dirty registers", func(t *testing.T) {
		f := newAdvanceFixture(t)
		appendAdvanceFile(t, f.peer, "product.txt", "peer\n")
		runAdvanceGit(t, f.peer, "add", "--", "product.txt")
		runAdvanceGit(t, f.peer, "commit", "-qm", "peer product")
		runAdvanceGit(t, f.peer, "push", "-q", "origin", "main")
		landing := f.commitLocalLanding()
		appendAdvanceFile(t, f.root, "memory/receipts.log", "receipt=local\n")
		appendAdvanceFile(t, f.root, "records/narrator-digest.log", "digest=local\n")
		receipts, _ := os.ReadFile(filepath.Join(f.root, "memory/receipts.log"))
		digest, _ := os.ReadFile(filepath.Join(f.root, "records/narrator-digest.log"))
		stash := runAdvanceGit(t, f.root, "stash", "list")
		f.fetch()
		if err := Advance(f.root, "refs/remotes/origin/main", &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
			t.Fatal(err)
		}
		if head := runAdvanceGit(t, f.root, "rev-parse", "HEAD"); head == landing {
			t.Fatal("reset did not move HEAD")
		}
		if indexTree, headTree := runAdvanceGit(t, f.root, "write-tree"), runAdvanceGit(t, f.root, "rev-parse", "HEAD^{tree}"); indexTree != headTree {
			t.Fatalf("reset index tree = %s, HEAD tree = %s", indexTree, headTree)
		}
		for path, want := range map[string][]byte{"memory/receipts.log": receipts, "records/narrator-digest.log": digest} {
			got, err := os.ReadFile(filepath.Join(f.root, path))
			if err != nil || !bytes.Equal(got, want) {
				t.Fatalf("register %s = %q, %v", path, got, err)
			}
		}
		status := strings.Split(advanceGitOutput(t, f.root, "status", "--porcelain"), "\n")
		sort.Strings(status)
		if !reflect.DeepEqual(status, []string{" M memory/receipts.log", " M records/narrator-digest.log"}) {
			t.Fatalf("status = %q", status)
		}
		if got := runAdvanceGit(t, f.root, "stash", "list"); got != stash {
			t.Fatalf("stash changed: %q", got)
		}
		f.assertOneWorktree()
	})
	t.Run("repaired digest union", func(t *testing.T) {
		f := newAdvanceFixture(t)
		appendAdvanceFile(t, f.peer, "records/narrator-digest.log", "digest=peer\n")
		runAdvanceGit(t, f.peer, "add", "--", "records/narrator-digest.log")
		runAdvanceGit(t, f.peer, "commit", "-qm", "peer digest")
		runAdvanceGit(t, f.peer, "push", "-q", "origin", "main")
		f.commitLocalLanding()
		appendAdvanceFile(t, f.root, "records/narrator-digest.log", "digest=local\n")
		original := runAdvanceGit(t, f.root, "rev-parse", "HEAD")
		f.fetch()
		err := Advance(f.root, "refs/remotes/origin/main", &bytes.Buffer{}, &bytes.Buffer{})
		assertAdvanceRefusal(t, err, "advance-register-contended")
		if got := runAdvanceGit(t, f.root, "rev-parse", "HEAD"); got != original {
			t.Fatalf("contended advance moved HEAD to %s", got)
		}
		f.assertOneWorktree()
		runAdvanceGit(t, f.root, "add", "--", "records/narrator-digest.log")
		runAdvanceGit(t, f.root, "commit", "-qm", "carry digest")
		if err := Advance(f.root, "refs/remotes/origin/main", &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
			t.Fatal(err)
		}
		if status := runAdvanceGit(t, f.root, "status", "--porcelain"); status != "" {
			t.Fatalf("repair status = %q", status)
		}
		lines := strings.Split(runAdvanceGit(t, f.root, "show", "HEAD:records/narrator-digest.log"), "\n")
		if len(lines) != 3 || lines[0] != "digest=seed" || countLine(lines, "digest=peer") != 1 || countLine(lines, "digest=local") != 1 {
			t.Fatalf("repaired digest = %v", lines)
		}
	})
	t.Run("head race preserves actual HEAD", func(t *testing.T) {
		f := newAdvanceFixture(t)
		appendAdvanceFile(t, f.peer, "product.txt", "peer\n")
		runAdvanceGit(t, f.peer, "add", "--", "product.txt")
		runAdvanceGit(t, f.peer, "commit", "-qm", "peer")
		runAdvanceGit(t, f.peer, "push", "-q", "origin", "main")
		landing := f.commitLocalLanding()
		f.fetch()
		var moved string
		advanceBeforeSwap = func() {
			runAdvanceGit(t, f.root, "commit", "--allow-empty", "-qm", "move head")
			moved = runAdvanceGit(t, f.root, "rev-parse", "HEAD")
		}
		t.Cleanup(func() { advanceBeforeSwap = nil })
		err := Advance(f.root, "refs/remotes/origin/main", &bytes.Buffer{}, &bytes.Buffer{})
		assertAdvanceRefusal(t, err, "advance-head-moved")
		if moved == "" || moved == landing || runAdvanceGit(t, f.root, "rev-parse", "HEAD") != moved || !strings.Contains(err.Error(), landing) {
			t.Fatalf("head race = %v, moved %s", err, moved)
		}
		f.assertOneWorktree()
	})
}

// A real repository proves a contended checkout lease leaves HEAD unchanged and creates no extra worktree.
func TestAdvanceRefusesWhenCheckoutLocked(t *testing.T) {
	f := newAdvanceFixture(t)
	head := runAdvanceGit(t, f.root, "rev-parse", "HEAD")
	release, err := lease.LockBounded(lease.LockPath(f.root), "test-holder")
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	t.Setenv("METASYSTEM_LEASE_LOCK_WAIT_SEC", "0")
	err = Advance(f.root, "refs/remotes/origin/main", &bytes.Buffer{}, &bytes.Buffer{})
	assertAdvanceRefusal(t, err, "advance-checkout-locked")
	if !strings.Contains(err.Error(), "another lease-gated operation holds it") {
		t.Fatalf("lock refusal changed its existing text: %v", err)
	}
	if got := runAdvanceGit(t, f.root, "rev-parse", "HEAD"); got != head {
		t.Fatalf("lock refusal moved HEAD to %s", got)
	}
	f.assertOneWorktree()
}

func TestAdvanceRefusesCheckoutLockBeforeGit(t *testing.T) {
	t.Parallel()
	const childMarker = "METASYSTEM_TEST_ADVANCE_CHECKOUT_LOCK_CHILD"
	if os.Getenv(childMarker) == "1" {
		root := t.TempDir()
		release, err := lease.LockBounded(lease.LockPath(root), "test-holder")
		if err != nil {
			t.Fatal(err)
		}
		defer release()
		err = Advance(root, "refs/remotes/origin/main", &bytes.Buffer{}, &bytes.Buffer{})
		assertAdvanceRefusal(t, err, "advance-checkout-locked")
		if !strings.Contains(err.Error(), "another lease-gated operation holds it") {
			t.Fatalf("lock refusal changed its existing text: %v", err)
		}
		return
	}
	child := exec.Command(os.Args[0], "-test.run=^TestAdvanceRefusesCheckoutLockBeforeGit$")
	child.Env = append(os.Environ(), childMarker+"=1", "METASYSTEM_LEASE_LOCK_WAIT_SEC=0")
	if output, err := child.CombinedOutput(); err != nil {
		t.Fatalf("checkout-lock child failed: %v\n%s", err, output)
	}
}

func assertAdvanceRefusal(t *testing.T, err error, code string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), "advance refused: "+code+":") {
		t.Fatalf("advance error = %v, want %s", err, code)
	}
}

func countLine(lines []string, want string) int {
	count := 0
	for _, line := range lines {
		if line == want {
			count++
		}
	}
	return count
}
func readFastForwardFile(t *testing.T, root, path string) string {
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
func TestAppendDeltaRefusesNonAppendRegisters(t *testing.T) {
	t.Parallel()
	for _, transition := range [][2]string{{"receipt=seed", "receipt=seed\nreceipt=next\n"}, {"receipt=seed\n", "receipt=seed\nreceipt=partial"}} {
		if _, ok := appendDelta([]byte(transition[0]), true, []byte(transition[1]), true); ok {
			t.Fatalf("non-append transition was accepted: %q", transition)
		}
	}
	if _, ok := appendDelta([]byte("receipt=seed\n"), true, nil, false); ok {
		t.Fatal("a deleted register was accepted")
	}
}
