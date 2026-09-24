package landing

import (
	"context"
	"crypto/sha1"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
)

const (
	ffHead    = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	ffTip     = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	ffReceipt = "memory/receipts.log"
	ffDigest  = "records/narrator-digest.log"
)

type ffStep struct {
	bridge bool
	args   []string
	out    []byte
	err    error
	effect func()
}

type ffFixture struct {
	t     *testing.T
	root  string
	steps []ffStep
	next  int
	head  string
	index []byte
}

func newFFFixture(t *testing.T) *ffFixture {
	t.Helper()
	f := &ffFixture{t: t, root: t.TempDir(), head: ffHead, index: []byte{0, 'o', 'p', 'a', 'q', 'u', 'e', 255}}
	writeAdvanceFile(t, f.root, ".git/index", string(f.index))
	writeAdvanceFile(t, f.root, ffReceipt, "receipt=seed\nreceipt=staged\nreceipt=working\n")
	writeAdvanceFile(t, f.root, ffDigest, "digest=seed\ndigest=working\n")
	writeAdvanceFile(t, f.root, "product.txt", "local product\n")
	writeAdvanceFile(t, f.root, "untracked.txt", "keep me\n")
	f.add(false, []string{"rev-parse", "--show-toplevel"}, f.root+"\n", nil, nil)
	f.add(false, []string{"rev-parse", "--show-prefix"}, "", nil, nil)
	f.add(false, []string{"rev-parse", "--git-path", fastForwardRecoveryName}, filepath.Join(f.root, ".git", fastForwardRecoveryName)+"\n", nil, nil)
	return f
}

func (f *ffFixture) add(bridge bool, args []string, out string, err error, effect func()) {
	f.steps = append(f.steps, ffStep{bridge: bridge, args: args, out: []byte(out), err: err, effect: effect})
}

func (f *ffFixture) planHead() {
	f.add(false, []string{"rev-parse", "--verify", "--quiet", "HEAD^{commit}"}, ffHead+"\n", nil, nil)
	f.add(false, []string{"rev-parse", "--git-path", "index"}, filepath.Join(f.root, ".git/index")+"\n", nil, nil)
}

func ffBlob(content string) string {
	header := fmt.Sprintf("blob %d\x00", len(content))
	sum := sha1.Sum(append([]byte(header), content...))
	return fmt.Sprintf("%x", sum)
}

func (f *ffFixture) planRegister(path, before, landed, staged string) {
	for _, item := range []struct{ revision, content string }{{ffHead, before}, {ffTip, landed}} {
		oid := ffBlob(item.content)
		f.add(false, []string{"--literal-pathspecs", "ls-tree", "-r", "-z", "--full-tree", item.revision, "--", path},
			"100644 blob "+oid+"\t"+path+"\x00", nil, nil)
		f.add(false, []string{"cat-file", "blob", oid}, item.content, nil, nil)
	}
	f.add(true, []string{"show", ":" + path}, staged, nil, nil)
}

func (f *ffFixture) planDirtyReads() {
	f.planHead()
	f.planRegister(ffReceipt, "receipt=seed\n", "receipt=seed\nreceipt=landed\n", "receipt=seed\nreceipt=staged\n")
	f.planRegister(ffDigest, "digest=seed\n", "digest=seed\ndigest=landed\n", "digest=seed\n")
}

func (f *ffFixture) effectFiles(index string, receipt, digest string) func() {
	return func() {
		writeAdvanceFile(f.t, f.root, ".git/index", index)
		writeAdvanceFile(f.t, f.root, ffReceipt, receipt)
		writeAdvanceFile(f.t, f.root, ffDigest, digest)
	}
}

func (f *ffFixture) planRestore(err error) {
	f.add(true, []string{"restore", "--staged", "--worktree", "--", ffReceipt, ffDigest}, "", err,
		f.effectFiles("restored-index", "receipt=seed\n", "digest=seed\n"))
}

func (f *ffFixture) planMerge(err error) {
	f.add(true, []string{"merge", "--ff-only", ffTip}, "", err, func() {
		if err == nil {
			f.effectFiles("landed-index", "receipt=seed\nreceipt=landed\n", "digest=seed\ndigest=landed\n")()
			f.head = ffTip
		}
	})
}

func (f *ffFixture) pop(bridge bool, args []string) ffStep {
	f.t.Helper()
	if f.next >= len(f.steps) {
		f.t.Fatalf("unexpected or repeated raw operation %q", args)
	}
	step := f.steps[f.next]
	if step.bridge != bridge || !slices.Equal(step.args, args) {
		f.t.Fatalf("raw operation %d = bridge %t %q, want bridge %t %q", f.next, bridge, args, step.bridge, step.args)
	}
	f.next++
	if step.effect != nil {
		step.effect()
	}
	return step
}

func (f *ffFixture) raw(request gittree.RawRequest) gittree.RawResult {
	f.t.Helper()
	pins := []string{"-C", f.root, "-c", "core.fileMode=true", "-c", "diff.noprefix=false", "-c", "diff.mnemonicPrefix=false",
		"-c", "apply.ignoreWhitespace=no", "-c", "core.logAllRefUpdates=false", "-c", "core.useReplaceRefs=false",
		"-c", "gc.auto=0", "-c", "maintenance.auto=false"}
	if request.Dir != f.root || len(request.Args) < len(pins) || !slices.Equal(request.Args[:len(pins)], pins) ||
		request.Stdin != nil || !slices.Equal(request.Env, gittree.ScrubbedEnviron()) {
		f.t.Fatalf("raw workspace request = %+v", request)
	}
	args := request.Args[len(pins):]
	if request.Operation != "git "+strings.Join(args, " ") {
		f.t.Fatalf("raw operation label = %q", request.Operation)
	}
	step := f.pop(false, args)
	if step.err != nil {
		return gittree.RawResult{Err: step.err}
	}
	return gittree.RawResult{Stdout: step.out}
}

func (f *ffFixture) git(ctx context.Context, root string, args ...string) ([]byte, error) {
	f.t.Helper()
	if ctx.Err() != nil || root != f.root {
		f.t.Fatalf("raw bridge ctx/root = %v %q", ctx.Err(), root)
	}
	step := f.pop(true, args)
	return step.out, step.err
}

func (f *ffFixture) run() error {
	f.t.Helper()
	err := fastForwardPreservingRegistersWith(context.Background(), f.root, ffTip, gittree.Workspace{Dir: f.root, RawSource: f.raw}, f.git)
	if f.next != len(f.steps) {
		f.t.Fatalf("raw operations consumed %d of %d", f.next, len(f.steps))
	}
	return err
}

func (f *ffFixture) recovery() string { return filepath.Join(f.root, ".git", fastForwardRecoveryName) }

func (f *ffFixture) assertUnrelated() {
	f.t.Helper()
	for path, want := range map[string]string{"product.txt": "local product\n", "untracked.txt": "keep me\n"} {
		if got := readFastForwardFile(f.t, f.root, path); got != want {
			f.t.Fatalf("%s = %q, want %q", path, got, want)
		}
	}
}

func TestFastForwardPreservesRegisters(t *testing.T) {
	f := newFFFixture(t)
	f.planDirtyReads()
	f.planRestore(nil)
	f.planMerge(nil)
	realWrite := writeFastForwardRecovery
	var recoveryTextSeen string
	writeFastForwardRecovery = func(path, text, anchor string) (bool, error) {
		recoveryTextSeen = text
		return realWrite(path, text, anchor)
	}
	defer func() { writeFastForwardRecovery = realWrite }()
	if err := f.run(); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{ffReceipt: "receipt=seed\nreceipt=landed\nreceipt=staged\nreceipt=working\n", ffDigest: "digest=seed\ndigest=landed\ndigest=working\n"} {
		if got := readFastForwardFile(t, f.root, path); got != want {
			t.Fatalf("%s = %q, want %q", path, got, want)
		}
	}
	if f.head != ffTip || readFastForwardFile(t, f.root, ".git/index") != "landed-index" {
		t.Fatal("raw merge did not land head/index")
	}
	if _, err := os.Stat(f.recovery()); !os.IsNotExist(err) {
		t.Fatalf("completed recovery file: %v", err)
	}
	if strings.Count(recoveryTextSeen, "working-sha256 ") != 2 || !strings.Contains(recoveryTextSeen, `suffix-bytes "receipt=staged\nreceipt=working\n"`) || !strings.Contains(recoveryTextSeen, `suffix-bytes "digest=working\n"`) {
		t.Fatalf("recovery text = %q", recoveryTextSeen)
	}
	f.assertUnrelated()
}

func TestFastForwardFailuresRestoreRegisters(t *testing.T) {
	for _, name := range []string{"merge", "restore", "write", "leftover"} {
		t.Run(name, func(t *testing.T) {
			f := newFFFixture(t)
			if name == "leftover" {
				writeAdvanceFile(t, f.root, ".git/"+fastForwardRecoveryName, "leftover\n")
			} else {
				f.planDirtyReads()
			}
			if name == "restore" {
				f.planRestore(os.ErrPermission)
			}
			if name == "merge" {
				f.planRestore(nil)
				f.planMerge(os.ErrPermission)
			}
			before := map[string]string{".git/index": readFastForwardFile(t, f.root, ".git/index"), ffReceipt: readFastForwardFile(t, f.root, ffReceipt), ffDigest: readFastForwardFile(t, f.root, ffDigest), "product.txt": readFastForwardFile(t, f.root, "product.txt")}
			realWrite := writeFastForwardRecovery
			if name == "write" {
				writeFastForwardRecovery = func(string, string, string) (bool, error) { return false, os.ErrPermission }
			}
			err := f.run()
			writeFastForwardRecovery = realWrite
			if err == nil || !strings.Contains(err.Error(), f.recovery()) {
				t.Fatalf("%s error = %v, want recovery path", name, err)
			}
			for path, want := range before {
				if got := readFastForwardFile(t, f.root, path); got != want {
					t.Fatalf("%s changed %s: %q", name, path, got)
				}
			}
			if f.head != ffHead {
				t.Fatal("failed fast-forward moved HEAD")
			}
			f.assertUnrelated()
		})
	}
}

func TestFastForwardAppendFailureKeepsLandedHeadAndUnrelatedFiles(t *testing.T) {
	f := newFFFixture(t)
	f.planDirtyReads()
	f.planRestore(nil)
	f.planMerge(nil)
	realAppend := appendRegister
	appendRegister = func(string, []byte) error { return os.ErrPermission }
	defer func() { appendRegister = realAppend }()
	err := f.run()
	for _, want := range []string{f.recovery(), ffReceipt, ffDigest} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("append error = %v, want %s", err, want)
		}
	}
	if f.head != ffTip || readFastForwardFile(t, f.root, ".git/index") != "landed-index" {
		t.Fatal("append failure lost landed head/index")
	}
	if got := readFastForwardFile(t, f.root, ffReceipt); got != "receipt=seed\nreceipt=landed\n" {
		t.Fatalf("receipt = %q", got)
	}
	if got := readFastForwardFile(t, f.root, ffDigest); got != "digest=seed\ndigest=landed\n" {
		t.Fatalf("digest = %q", got)
	}
	f.assertUnrelated()
	recoveryBytes, readErr := os.ReadFile(f.recovery())
	if readErr != nil || strings.Count(string(recoveryBytes), "working-sha256 ") != 2 || !strings.Contains(string(recoveryBytes), `suffix-bytes "receipt=staged\nreceipt=working\n"`) || !strings.Contains(string(recoveryBytes), `suffix-bytes "digest=working\n"`) {
		t.Errorf("recovery = %q, error %v", recoveryBytes, readErr)
	}
}

func TestFastForwardRefusesNonAppendRegisters(t *testing.T) {
	for _, tc := range []struct{ name, working, staged, landed string }{
		{"local rewrite", "receipt=rewritten\n", "receipt=seed\n", "receipt=seed\n"},
		{"staged rewrite", "receipt=rewritten\n", "receipt=rewritten\n", "receipt=seed\n"},
		{"landed rewrite", "receipt=seed\nreceipt=local\n", "receipt=seed\n", "receipt=rewritten\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFFFixture(t)
			writeAdvanceFile(t, f.root, ffReceipt, tc.working)
			f.planHead()
			f.planRegister(ffReceipt, "receipt=seed\n", tc.landed, tc.staged)
			err := f.run()
			if err == nil || !strings.Contains(err.Error(), "TEST_POLICY_ENGINE_REQUIRED") || !strings.Contains(err.Error(), ffReceipt) {
				t.Fatalf("non-append error = %v", err)
			}
			if got := readFastForwardFile(t, f.root, ffReceipt); got != tc.working {
				t.Fatalf("refusal changed bytes from %q to %q", tc.working, got)
			}
			if got := readFastForwardFile(t, f.root, ".git/index"); got != string(f.index) {
				t.Fatal("refusal changed opaque index")
			}
			if f.head != ffHead {
				t.Fatal("refusal moved HEAD")
			}
			if _, err := os.Stat(f.recovery()); !os.IsNotExist(err) {
				t.Fatalf("refusal recovery file: %v", err)
			}
			f.assertUnrelated()
		})
	}
}

// This adapter witnesses the native merge's HEAD/index move; the policy cases
// above exercise register decisions and recovery against strict raw facts.
func TestFastForwardNativeHeadIndexAdapter(t *testing.T) {
	t.Parallel()
	f := newAdvanceFixture(t)
	appendAdvanceFile(t, f.root, ffReceipt, "receipt=staged\n")
	runAdvanceGit(t, f.root, "add", ffReceipt)
	appendAdvanceFile(t, f.root, ffReceipt, "receipt=working\n")
	appendAdvanceFile(t, f.peer, ffReceipt, "receipt=landed\n")
	writeAdvanceFile(t, f.peer, "landed.txt", "landed\n")
	runAdvanceGit(t, f.peer, "add", ".")
	runAdvanceGit(t, f.peer, "commit", "-qm", "landed")
	runAdvanceGit(t, f.peer, "push", "-q", "origin", "main")
	f.fetch()
	tip := runAdvanceGit(t, f.root, "rev-parse", "refs/remotes/origin/main")
	if err := FastForwardPreservingRegisters(context.Background(), f.root, tip); err != nil {
		t.Fatal(err)
	}
	if got := runAdvanceGit(t, f.root, "rev-parse", "HEAD"); got != tip {
		t.Fatalf("HEAD = %s, want %s", got, tip)
	}
	if got := advanceGitOutput(t, f.root, "show", ":"+ffReceipt); got != "receipt=seed\nreceipt=landed" {
		t.Fatalf("index receipt = %q", got)
	}
	if got := advanceGitOutput(t, f.root, "diff", "--cached", "--name-only"); got != "" {
		t.Fatalf("cached paths = %q", got)
	}
	if got := readFastForwardFile(t, f.root, ffReceipt); got != "receipt=seed\nreceipt=landed\nreceipt=staged\nreceipt=working\n" {
		t.Fatalf("working receipt = %q", got)
	}
	t.Run("merge refusal restores opaque index", func(t *testing.T) {
		f := newAdvanceFixture(t)
		appendAdvanceFile(t, f.root, ffReceipt, "receipt=staged\n")
		runAdvanceGit(t, f.root, "add", ffReceipt)
		appendAdvanceFile(t, f.root, ffReceipt, "receipt=working\n")
		writeAdvanceFile(t, f.root, "product.txt", "local product\n")
		writeAdvanceFile(t, f.peer, "product.txt", "landed product\n")
		runAdvanceGit(t, f.peer, "add", "product.txt")
		runAdvanceGit(t, f.peer, "commit", "-qm", "landed product")
		runAdvanceGit(t, f.peer, "push", "-q", "origin", "main")
		f.fetch()
		beforeIndex := readFastForwardFile(t, f.root, ".git/index")
		beforeHead := runAdvanceGit(t, f.root, "rev-parse", "HEAD")
		err := FastForwardPreservingRegisters(context.Background(), f.root, "refs/remotes/origin/main")
		if err == nil || !strings.Contains(err.Error(), "fast-forward failed; recovery file remains") {
			t.Fatalf("merge refusal = %v", err)
		}
		if got := readFastForwardFile(t, f.root, ".git/index"); got != beforeIndex {
			t.Fatal("merge refusal changed opaque index")
		}
		if got := readFastForwardFile(t, f.root, ffReceipt); got != "receipt=seed\nreceipt=staged\nreceipt=working\n" {
			t.Fatalf("restored register = %q", got)
		}
		if got := runAdvanceGit(t, f.root, "rev-parse", "HEAD"); got != beforeHead {
			t.Fatalf("HEAD = %s, want %s", got, beforeHead)
		}
		if got := readFastForwardFile(t, f.root, "product.txt"); got != "local product\n" {
			t.Fatalf("local product = %q", got)
		}
	})
}
