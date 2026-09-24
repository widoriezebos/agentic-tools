package landing

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

func TestAdvanceLeavesRegistersUntouched(t *testing.T) {
	f := newAdvanceFixture(t)
	appendAdvanceFile(t, f.peer, "product.txt", "peer\n")
	runAdvanceGit(t, f.peer, "add", "--", "product.txt")
	runAdvanceGit(t, f.peer, "commit", "-qm", "peer product")
	runAdvanceGit(t, f.peer, "push", "-q", "origin", "main")
	landing := f.commitLocalLanding()
	appendAdvanceFile(t, f.root, "memory/receipts.log", "receipt=local\n")
	appendAdvanceFile(t, f.root, "records/narrator-digest.log", "digest=local\n")
	receiptsBefore, err := os.ReadFile(filepath.Join(f.root, "memory", "receipts.log"))
	if err != nil {
		t.Fatal(err)
	}
	digestBefore, err := os.ReadFile(filepath.Join(f.root, "records", "narrator-digest.log"))
	if err != nil {
		t.Fatal(err)
	}
	f.fetch()

	var output bytes.Buffer
	if err := Advance(f.root, "refs/remotes/origin/main", &output, &output); err != nil {
		t.Fatal(err)
	}
	rebased := runAdvanceGit(t, f.root, "rev-parse", "HEAD")
	if rebased == landing {
		t.Fatal("advance did not move the landing commit")
	}
	for path, want := range map[string][]byte{
		"memory/receipts.log":         receiptsBefore,
		"records/narrator-digest.log": digestBefore,
	} {
		got, err := os.ReadFile(filepath.Join(f.root, filepath.FromSlash(path)))
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("register %s changed: %q, error %v", path, got, err)
		}
	}
	status := strings.Split(advanceGitOutput(t, f.root, "status", "--porcelain"), "\n")
	sort.Strings(status)
	wantStatus := []string{" M memory/receipts.log", " M records/narrator-digest.log"}
	if strings.Join(status, "\n") != strings.Join(wantStatus, "\n") {
		t.Fatalf("status = %q, want %q", status, wantStatus)
	}
	if got := runAdvanceGit(t, f.root, "stash", "list"); got != "" {
		t.Fatalf("advance changed stash list: %s", got)
	}
	f.assertOneWorktree()
	output.Reset()
	if err := Advance(f.root, "refs/remotes/origin/main", &output, &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "advance: up to date with refs/remotes/origin/main") {
		t.Fatalf("second advance output = %q", output.String())
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

func TestAdvanceRefusesContendedRegister(t *testing.T) {
	f := newAdvanceFixture(t)
	appendAdvanceFile(t, f.peer, "records/narrator-digest.log", "digest=peer\n")
	runAdvanceGit(t, f.peer, "add", "--", "records/narrator-digest.log")
	runAdvanceGit(t, f.peer, "commit", "-qm", "peer digest")
	runAdvanceGit(t, f.peer, "push", "-q", "origin", "main")
	landing := f.commitLocalLanding()
	appendAdvanceFile(t, f.root, "records/narrator-digest.log", "digest=local\n")
	wantBytes, err := os.ReadFile(filepath.Join(f.root, "records", "narrator-digest.log"))
	if err != nil {
		t.Fatal(err)
	}
	f.fetch()

	err = Advance(f.root, "refs/remotes/origin/main", &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "advance refused: advance-register-contended: records/narrator-digest.log") ||
		!regexp.MustCompile(`[0-9a-f]{40,64}`).MatchString(err.Error()) {
		t.Fatalf("contended advance error = %v", err)
	}
	if got := runAdvanceGit(t, f.root, "rev-parse", "HEAD"); got != landing {
		t.Fatalf("contended advance moved HEAD from %s to %s", landing, got)
	}
	gotBytes, err := os.ReadFile(filepath.Join(f.root, "records", "narrator-digest.log"))
	if err != nil || !bytes.Equal(gotBytes, wantBytes) {
		t.Fatalf("contended advance changed digest: %q, error %v", gotBytes, err)
	}
	if got := advanceGitOutput(t, f.root, "status", "--porcelain"); got != " M records/narrator-digest.log" {
		t.Fatalf("contended status = %q", got)
	}
	f.assertOneWorktree()

	runAdvanceGit(t, f.root, "add", "--", "records/narrator-digest.log")
	runAdvanceGit(t, f.root, "commit", "-qm", "carry digest")
	if err := Advance(f.root, "refs/remotes/origin/main", &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("repair advance: %v", err)
	}
	if status := runAdvanceGit(t, f.root, "status", "--porcelain"); status != "" {
		t.Fatalf("repair status = %q", status)
	}
	committed := runAdvanceGit(t, f.root, "show", "HEAD:records/narrator-digest.log")
	lines := strings.Split(committed, "\n")
	if len(lines) != 3 || lines[0] != "digest=seed" || countLine(lines, "digest=peer") != 1 || countLine(lines, "digest=local") != 1 {
		t.Fatalf("repaired digest lines = %v", lines)
	}
}

func TestAdvanceRefusals(t *testing.T) {
	t.Run("unstaged drift", func(t *testing.T) {
		f := newAdvanceFixture(t)
		writeAdvanceFile(t, f.peer, "product.txt", "peer\n")
		runAdvanceGit(t, f.peer, "add", "--", "product.txt")
		runAdvanceGit(t, f.peer, "commit", "-qm", "peer")
		runAdvanceGit(t, f.peer, "push", "-q", "origin", "main")
		landing := f.commitLocalLanding()
		writeAdvanceFile(t, f.root, "product.txt", "dirty\n")
		f.fetch()
		err := Advance(f.root, "refs/remotes/origin/main", &bytes.Buffer{}, &bytes.Buffer{})
		assertAdvanceRefusal(t, err, "advance-unstaged-drift")
		if got := runAdvanceGit(t, f.root, "rev-parse", "HEAD"); got != landing {
			t.Fatalf("refusal moved HEAD to %s", got)
		}
		content, _ := os.ReadFile(filepath.Join(f.root, "product.txt"))
		if string(content) != "dirty\n" {
			t.Fatalf("refusal changed product to %q", content)
		}
	})

	t.Run("rebase conflict", func(t *testing.T) {
		f := newAdvanceFixture(t)
		writeAdvanceFile(t, f.peer, "product.txt", "peer\n")
		runAdvanceGit(t, f.peer, "add", "--", "product.txt")
		runAdvanceGit(t, f.peer, "commit", "-qm", "peer")
		runAdvanceGit(t, f.peer, "push", "-q", "origin", "main")
		writeAdvanceFile(t, f.root, "product.txt", "landing\n")
		runAdvanceGit(t, f.root, "add", "--", "product.txt")
		runAdvanceGit(t, f.root, "commit", "-qm", "landing")
		landing := runAdvanceGit(t, f.root, "rev-parse", "HEAD")
		f.fetch()
		err := Advance(f.root, "refs/remotes/origin/main", &bytes.Buffer{}, &bytes.Buffer{})
		assertAdvanceRefusal(t, err, "advance-rebase-conflict")
		if got := runAdvanceGit(t, f.root, "rev-parse", "HEAD"); got != landing {
			t.Fatalf("conflict moved HEAD to %s", got)
		}
		f.assertOneWorktree()
	})

	t.Run("register removed", func(t *testing.T) {
		f := newAdvanceFixture(t)
		runAdvanceGit(t, f.peer, "rm", "-q", "--", "records/narrator-digest.log")
		runAdvanceGit(t, f.peer, "commit", "-qm", "remove digest")
		runAdvanceGit(t, f.peer, "push", "-q", "origin", "main")
		f.commitLocalLanding()
		appendAdvanceFile(t, f.root, "records/narrator-digest.log", "digest=local\n")
		f.fetch()
		err := Advance(f.root, "refs/remotes/origin/main", &bytes.Buffer{}, &bytes.Buffer{})
		assertAdvanceRefusal(t, err, "advance-register-removed")
	})

	t.Run("staged path", func(t *testing.T) {
		f := newAdvanceFixture(t)
		writeAdvanceFile(t, f.root, "product.txt", "staged\n")
		runAdvanceGit(t, f.root, "add", "--", "product.txt")
		err := Advance(f.root, "refs/remotes/origin/main", &bytes.Buffer{}, &bytes.Buffer{})
		assertAdvanceRefusal(t, err, "advance-index-not-empty")
	})

	t.Run("detached head", func(t *testing.T) {
		f := newAdvanceFixture(t)
		runAdvanceGit(t, f.root, "checkout", "-q", "--detach")
		err := Advance(f.root, "refs/remotes/origin/main", &bytes.Buffer{}, &bytes.Buffer{})
		assertAdvanceRefusal(t, err, "advance-not-on-branch")
	})

	t.Run("head moved", func(t *testing.T) {
		f := newAdvanceFixture(t)
		appendAdvanceFile(t, f.peer, "product.txt", "peer\n")
		runAdvanceGit(t, f.peer, "add", "--", "product.txt")
		runAdvanceGit(t, f.peer, "commit", "-qm", "peer")
		runAdvanceGit(t, f.peer, "push", "-q", "origin", "main")
		landing := f.commitLocalLanding()
		f.fetch()
		advanceBeforeSwap = func() {
			runAdvanceGit(t, f.root, "commit", "--allow-empty", "-qm", "move head")
		}
		t.Cleanup(func() { advanceBeforeSwap = nil })
		err := Advance(f.root, "refs/remotes/origin/main", &bytes.Buffer{}, &bytes.Buffer{})
		assertAdvanceRefusal(t, err, "advance-head-moved")
		if !strings.Contains(err.Error(), landing) {
			t.Fatalf("head-moved refusal does not name landing commit %s: %v", landing, err)
		}
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
func dirtyFastForwardFixture(t *testing.T, conflict bool) *advanceFixture {
	f := newAdvanceFixture(t)
	appendAdvanceFile(t, f.root, "memory/receipts.log", "receipt=staged\n")
	runAdvanceGit(t, f.root, "add", "memory/receipts.log")
	appendAdvanceFile(t, f.root, "memory/receipts.log", "receipt=working\n")
	appendAdvanceFile(t, f.root, "records/narrator-digest.log", "digest=working\n")
	appendAdvanceFile(t, f.peer, "memory/receipts.log", "receipt=landed\n")
	appendAdvanceFile(t, f.peer, "records/narrator-digest.log", "digest=landed\n")
	writeAdvanceFile(t, f.peer, "landed.txt", "landed\n")
	if conflict {
		writeAdvanceFile(t, f.peer, "product.txt", "landed product\n")
	}
	runAdvanceGit(t, f.peer, "add", ".")
	runAdvanceGit(t, f.peer, "commit", "-qm", "landed")
	runAdvanceGit(t, f.peer, "push", "-q", "origin", "main")
	f.fetch()
	writeAdvanceFile(t, f.root, "product.txt", "local product\n")
	writeAdvanceFile(t, f.root, "untracked.txt", "keep me\n")
	return f
}
func fastForwardState(t *testing.T, root string) map[string]string {
	index, _ := (gittree.Workspace{Dir: root}).GitPath("index")
	return map[string]string{"HEAD": runAdvanceGit(t, root, "rev-parse", "HEAD"), "index": readFastForwardFile(t, filepath.Dir(index), filepath.Base(index)), "memory/receipts.log": readFastForwardFile(t, root, "memory/receipts.log"), "records/narrator-digest.log": readFastForwardFile(t, root, "records/narrator-digest.log"), "product.txt": readFastForwardFile(t, root, "product.txt")}
}
func TestFastForwardPreservesRegisters(t *testing.T) {
	f := dirtyFastForwardFixture(t, false)
	tip := runAdvanceGit(t, f.root, "rev-parse", "refs/remotes/origin/main")
	realWrite, wroteRecovery := writeFastForwardRecovery, false
	writeFastForwardRecovery = func(path, text, anchor string) (bool, error) {
		wroteRecovery = true
		return realWrite(path, text, anchor)
	}
	err := FastForwardPreservingRegisters(context.Background(), f.root, "refs/remotes/origin/main")
	writeFastForwardRecovery = realWrite
	if err != nil || !wroteRecovery {
		t.Fatalf("fast-forward error = %v, wrote recovery = %t", err, wroteRecovery)
	}
	for path, want := range map[string]string{"memory/receipts.log": "receipt=seed\nreceipt=landed\nreceipt=staged\nreceipt=working\n", "records/narrator-digest.log": "digest=seed\ndigest=landed\ndigest=working\n"} {
		if got := readFastForwardFile(t, f.root, path); got != want {
			t.Fatalf("%s = %q, want %q", path, got, want)
		}
	}
	if got := runAdvanceGit(t, f.root, "rev-parse", "HEAD"); got != tip {
		t.Fatalf("HEAD = %s, want %s", got, tip)
	}
	if _, err := os.Stat(filepath.Join(f.root, ".git", fastForwardRecoveryName)); !os.IsNotExist(err) {
		t.Fatalf("completed recovery file error = %v", err)
	}
}
func TestFastForwardFailuresRestoreRegisters(t *testing.T) {
	for _, name := range []string{"merge", "restore", "write", "leftover"} {
		t.Run(name, func(t *testing.T) {
			f := dirtyFastForwardFixture(t, name == "merge")
			before, recovery := fastForwardState(t, f.root), filepath.Join(f.root, ".git", fastForwardRecoveryName)
			realWrite := writeFastForwardRecovery
			if name == "write" {
				writeFastForwardRecovery = func(string, string, string) (bool, error) { return false, os.ErrPermission }
			}
			if name == "leftover" {
				writeAdvanceFile(t, filepath.Dir(recovery), filepath.Base(recovery), "leftover\n")
			}
			if name == "restore" {
				_ = os.Chmod(filepath.Join(f.root, "records"), 0o555)
			}
			err := FastForwardPreservingRegisters(context.Background(), f.root, "refs/remotes/origin/main")
			writeFastForwardRecovery = realWrite
			_ = os.Chmod(filepath.Join(f.root, "records"), 0o755)
			if err == nil || !strings.Contains(err.Error(), recovery) {
				t.Fatalf("%s error = %v, want recovery path", name, err)
			}
			for key, want := range before {
				if got := fastForwardState(t, f.root)[key]; got != want {
					t.Fatalf("%s changed %s", name, key)
				}
			}
		})
	}
}
func TestFastForwardAppendFailureKeepsLandedHeadAndUnrelatedFiles(t *testing.T) {
	f := dirtyFastForwardFixture(t, false)
	tip := runAdvanceGit(t, f.root, "rev-parse", "refs/remotes/origin/main")
	recovery := filepath.Join(f.root, ".git", fastForwardRecoveryName)
	realAppend := appendRegister
	appendRegister = func(string, []byte) error { return os.ErrPermission }
	err := FastForwardPreservingRegisters(context.Background(), f.root, "refs/remotes/origin/main")
	appendRegister = realAppend
	for _, want := range []string{recovery, "memory/receipts.log", "records/narrator-digest.log"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("append error = %v, want %s", err, want)
		}
	}
	for path, want := range map[string]string{"product.txt": "local product\n", "untracked.txt": "keep me\n"} {
		if got := readFastForwardFile(t, f.root, path); got != want {
			t.Errorf("%s = %q, want %q", path, got, want)
		}
	}
	if got := runAdvanceGit(t, f.root, "rev-parse", "HEAD"); got != tip {
		t.Errorf("HEAD = %s, want landed tip %s", got, tip)
	}
	if got := advanceGitOutput(t, f.root, "diff", "--cached", "--name-only", "--", ".", ":(exclude)memory/receipts.log", ":(exclude)records/narrator-digest.log"); got != "" {
		t.Errorf("cached non-register paths = %q", got)
	}
	recoveryBytes, readErr := os.ReadFile(recovery)
	if readErr != nil || strings.Count(string(recoveryBytes), "working-sha256 ") != 2 || !strings.Contains(string(recoveryBytes), `suffix-bytes "receipt=staged\nreceipt=working\n"`) || !strings.Contains(string(recoveryBytes), `suffix-bytes "digest=working\n"`) {
		t.Errorf("recovery = %q, error %v", recoveryBytes, readErr)
	}
}
func TestFastForwardRefusesNonAppendRegisters(t *testing.T) {
	for _, test := range []struct{ name, local, landed string }{{"local rewrite", "receipt=rewritten\n", ""}, {"staged rewrite", "receipt=rewritten\n", ""}, {"landed rewrite", "receipt=seed\nreceipt=local\n", "receipt=rewritten\n"}} {
		t.Run(test.name, func(t *testing.T) {
			f := newAdvanceFixture(t)
			if test.local != "" {
				writeAdvanceFile(t, f.root, "memory/receipts.log", test.local)
			}
			if test.name == "staged rewrite" {
				runAdvanceGit(t, f.root, "add", "memory/receipts.log")
			}
			if test.landed != "" {
				writeAdvanceFile(t, f.peer, "memory/receipts.log", test.landed)
			} else {
				appendAdvanceFile(t, f.peer, "product.txt", "landed\n")
			}
			runAdvanceGit(t, f.peer, "add", ".")
			runAdvanceGit(t, f.peer, "commit", "-qm", "landed change")
			runAdvanceGit(t, f.peer, "push", "-q", "origin", "main")
			f.fetch()
			err := FastForwardPreservingRegisters(context.Background(), f.root, "refs/remotes/origin/main")
			if err == nil || !strings.Contains(err.Error(), "TEST_POLICY_ENGINE_REQUIRED") || !strings.Contains(err.Error(), "memory/receipts.log") {
				t.Fatalf("non-append error = %v", err)
			}
			if got := readFastForwardFile(t, f.root, "memory/receipts.log"); got != test.local {
				t.Fatalf("refusal changed bytes from %q to %q", test.local, got)
			}
		})
	}
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
