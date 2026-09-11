package landing

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
)

func TestCreateTestReceiptIgnoresLiveWorkspaceMotion(t *testing.T) {
	f := newObserveFixture(t)
	f.write("product.txt", "candidate\n")
	f.git("add", "--", "product.txt")
	candidate, err := (gittree.Workspace{Dir: f.root}).StagedTree()
	if err != nil {
		t.Fatal(err)
	}
	worktreesBefore := f.git("worktree", "list", "--porcelain")
	t.Setenv("LANDING_RECEIPT_LIVE_ROOT", f.root)
	t.Setenv("GOTOOLCHAIN", "receipt-fixture")

	receipt, err := CreateTestReceipt(
		f.root,
		candidate,
		`test "$GOTOOLCHAIN" = receipt-fixture && printf '%s\n' 'digest=during-receipt' >> "$LANDING_RECEIPT_LIVE_ROOT/records/narrator-digest.log" && sleep 0.1`,
		io.Discard,
		io.Discard,
	)
	if err != nil {
		t.Fatalf("receipt rejected live workspace motion: %v", err)
	}
	if receipt.ExitStatus != 0 {
		t.Fatalf("receipt exit status = %d, want zero", receipt.ExitStatus)
	}
	wantBinding := TestReceiptBinding{
		IndexTreeBefore: candidate, WorktreeTreeBefore: candidate,
		IndexTreeAfter: candidate, WorktreeTreeAfter: candidate,
	}
	if receipt.Tree != candidate || receipt.Binding != wantBinding {
		t.Fatalf("receipt = %+v, want tree and all bindings %s", receipt, candidate)
	}
	digest, err := os.ReadFile(filepath.Join(f.root, "records", "narrator-digest.log"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(digest), "digest=during-receipt\n") {
		t.Fatalf("live narrator digest did not retain the command's append:\n%s", digest)
	}
	if got := f.git("worktree", "list", "--porcelain"); got != worktreesBefore {
		t.Fatalf("temporary receipt worktree remained after success:\n%s", got)
	}
}

func TestCreateTestReceiptRefusesIsolatedCandidateMotion(t *testing.T) {
	f := newObserveFixture(t)
	f.write("product.txt", "candidate\n")
	f.git("add", "--", "product.txt")
	candidate, err := (gittree.Workspace{Dir: f.root}).StagedTree()
	if err != nil {
		t.Fatal(err)
	}
	worktreesBefore := f.git("worktree", "list", "--porcelain")

	_, err = CreateTestReceipt(
		f.root,
		candidate,
		`printf '%s\n' changed-by-command > product.txt`,
		io.Discard,
		io.Discard,
	)
	if err == nil || !strings.Contains(err.Error(), "test receipt refused: the candidate changed while the command ran") {
		t.Fatalf("candidate-changing command error = %v", err)
	}
	if _, statErr := os.Stat(TestReceiptPath(f.root, candidate)); !os.IsNotExist(statErr) {
		t.Fatalf("refused receipt remains available: %v", statErr)
	}
	liveProduct, readErr := os.ReadFile(filepath.Join(f.root, "product.txt"))
	if readErr != nil || string(liveProduct) != "candidate\n" {
		t.Fatalf("candidate-changing command reached the live product: bytes=%q error=%v", liveProduct, readErr)
	}
	if got := f.git("worktree", "list", "--porcelain"); got != worktreesBefore {
		t.Fatalf("temporary receipt worktree remained after refusal:\n%s", got)
	}
}

func TestCreateTestReceiptRemovesIsolatedWorktreeAfterCommandFailure(t *testing.T) {
	f := newObserveFixture(t)
	candidate, err := (gittree.Workspace{Dir: f.root}).StagedTree()
	if err != nil {
		t.Fatal(err)
	}
	worktreesBefore := f.git("worktree", "list", "--porcelain")

	receipt, err := CreateTestReceipt(f.root, candidate, "exit 23", io.Discard, io.Discard)
	if err != nil {
		t.Fatalf("record failing command: %v", err)
	}
	if receipt.ExitStatus != 23 {
		t.Fatalf("receipt exit status = %d, want 23", receipt.ExitStatus)
	}
	if got := f.git("worktree", "list", "--porcelain"); got != worktreesBefore {
		t.Fatalf("temporary receipt worktree remained after command failure:\n%s", got)
	}
}

func TestCreateTestReceiptChecksOutWholeTreeAtRepositoryRoot(t *testing.T) {
	f := newAdoptedObserveFixture(t)
	if err := os.Remove(filepath.Join(f.root, "product.txt")); err != nil {
		t.Fatal(err)
	}
	f.write("candidate.txt", "candidate\n")
	f.git("add", "-A", "--", ".")
	candidate, err := (gittree.Workspace{Dir: f.root}).StagedTree()
	if err != nil {
		t.Fatal(err)
	}

	receipt, err := CreateTestReceipt(
		f.root,
		candidate,
		`test ! -e product.txt && test "$(cat candidate.txt)" = candidate`,
		io.Discard,
		io.Discard,
	)
	if err != nil {
		t.Fatalf("receipt against repository-root candidate: %v", err)
	}
	if receipt.Tree != candidate || receipt.Binding.IndexTreeAfter != candidate || receipt.Binding.WorktreeTreeAfter != candidate {
		t.Fatalf("receipt = %+v, want candidate %s", receipt, candidate)
	}
}

func TestCreateTestReceiptRemovesIsolatedWorktreeAfterSignal(t *testing.T) {
	if os.Getenv("LANDING_RECEIPT_SIGNAL_HELPER") == "1" {
		_, err := CreateTestReceipt(
			os.Getenv("LANDING_RECEIPT_SIGNAL_ROOT"),
			os.Getenv("LANDING_RECEIPT_SIGNAL_TREE"),
			`printf '%s\n' "$PWD" > "$LANDING_RECEIPT_SIGNAL_PROBE"; exec sleep 5`,
			io.Discard,
			io.Discard,
		)
		if err == nil || !strings.Contains(err.Error(), "landing test receipt interrupted by") {
			t.Fatalf("signaled receipt error = %v", err)
		}
		return
	}

	f := newObserveFixture(t)
	candidate, err := (gittree.Workspace{Dir: f.root}).StagedTree()
	if err != nil {
		t.Fatal(err)
	}
	worktreesBefore := f.git("worktree", "list", "--porcelain")
	probe := filepath.Join(t.TempDir(), "isolated-root")
	helper := exec.Command(os.Args[0], "-test.run=^TestCreateTestReceiptRemovesIsolatedWorktreeAfterSignal$")
	helper.Env = gittree.ScrubbedEnviron(
		"LANDING_RECEIPT_SIGNAL_HELPER=1",
		"LANDING_RECEIPT_SIGNAL_ROOT="+f.root,
		"LANDING_RECEIPT_SIGNAL_TREE="+candidate,
		"LANDING_RECEIPT_SIGNAL_PROBE="+probe,
	)
	var output bytes.Buffer
	helper.Stdout = &output
	helper.Stderr = &output
	if err := helper.Start(); err != nil {
		t.Fatal(err)
	}
	helperWaited := false
	t.Cleanup(func() {
		if !helperWaited {
			_ = helper.Process.Signal(os.Interrupt)
			_ = helper.Wait()
		}
	})
	// The helper is a second copy of this test binary doing real git work;
	// under a loaded box its probe takes longer than a quiet box's few
	// seconds, so the bound is one only a hang trips (2026-09-11: ten
	// seconds failed inside the pooled battery). And the helper's output is
	// read only after the helper has been waited for: its stdout copier
	// writes the buffer until then, which the race detector reported.
	deadline := time.Now().Add(90 * time.Second)
	var isolatedRoot string
	for time.Now().Before(deadline) {
		data, readErr := os.ReadFile(probe)
		if readErr == nil {
			isolatedRoot = strings.TrimSpace(string(data))
			break
		}
		if !os.IsNotExist(readErr) {
			t.Fatal(readErr)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if isolatedRoot == "" {
		_ = helper.Process.Signal(os.Interrupt)
		_ = helper.Wait()
		helperWaited = true
		t.Fatalf("signal helper did not expose its isolated root:\n%s", output.String())
	}
	resolvedTemp, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	resolvedIsolatedRoot, err := filepath.EvalSymlinks(isolatedRoot)
	if err != nil {
		t.Fatal(err)
	}
	relativeToTemp, err := filepath.Rel(resolvedTemp, resolvedIsolatedRoot)
	if err != nil || relativeToTemp == ".." || strings.HasPrefix(relativeToTemp, ".."+string(filepath.Separator)) {
		t.Fatalf("isolated root %q is not under the system temporary directory", isolatedRoot)
	}
	if err := helper.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	waitErr := helper.Wait()
	helperWaited = true
	if waitErr != nil {
		t.Fatalf("signal helper failed: %v\n%s", waitErr, output.String())
	}
	if _, err := os.Stat(isolatedRoot); !os.IsNotExist(err) {
		t.Fatalf("signaled receipt left isolated root %s: %v", isolatedRoot, err)
	}
	if got := f.git("worktree", "list", "--porcelain"); got != worktreesBefore {
		t.Fatalf("temporary receipt worktree remained after signal:\n%s", got)
	}
}

func TestReceiptCommandBoundResolution(t *testing.T) {
	conf := filepath.Join(t.TempDir(), "metasystem.conf")
	if got := receiptCommandBound(conf); got.Limit != 40*time.Minute || got.Key != "landing.receipt-bound-min" {
		t.Fatalf("absent receipt bound = %+v", got)
	}
	if err := os.WriteFile(conf, []byte("landing.receipt-bound-min=17\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := receiptCommandBound(conf); got.Limit != 17*time.Minute || got.Key != "landing.receipt-bound-min" {
		t.Fatalf("configured receipt bound = %+v", got)
	}
	for _, malformed := range []string{"0", "nonsense"} {
		if err := os.WriteFile(conf, []byte("landing.receipt-bound-min="+malformed+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := receiptCommandBound(conf); got.Limit != 40*time.Minute {
			t.Fatalf("malformed receipt bound %q did not fall back: %+v", malformed, got)
		}
	}
}
