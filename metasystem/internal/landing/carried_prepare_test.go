package landing

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
)

// carriedStageFixture is a template checkout: the installation nests at
// metasystem/ beside the design page, its product includes a top-level
// benchmark script outside the prefix, and origin is a bare repository. The
// real Git index, worktree and refs are the claims of these tests.
type carriedStageFixture struct {
	*advanceFixture
	top     string
	subject CarriedSubject
}

const carriedFixtureLedgerPath = "metasystem/memory/receipts.log"

func newCarriedStageFixture(t *testing.T) *carriedStageFixture {
	t.Helper()
	base := t.TempDir()
	seed := filepath.Join(base, "seed")
	remote := filepath.Join(base, "origin.git")
	top := filepath.Join(base, "local")
	peer := filepath.Join(base, "peer")
	if err := os.MkdirAll(seed, 0o755); err != nil {
		t.Fatal(err)
	}
	classes, err := os.ReadFile(filepath.Join("..", "..", "scripts", "agents", "path-classes.txt"))
	if err != nil {
		t.Fatal(err)
	}
	runAdvanceGit(t, seed, "init", "-q", "-b", "main")
	configureAdvanceGit(t, seed)
	writeAdvanceFile(t, seed, "development/metasystem-design.md", "# design\n")
	writeAdvanceFile(t, seed, "benchmark/run.sh", "seed\n")
	writeAdvanceFile(t, seed, "metasystem/.gitignore", "artifacts/\n")
	writeAdvanceFile(t, seed, "metasystem/product.txt", "one\ntwo\nthree\n")
	writeAdvanceFile(t, seed, "metasystem/scripts/agents/path-classes.txt", string(classes)+"install:product.txt behavior\ninstall:added.txt behavior\n")
	writeAdvanceFile(t, seed, carriedFixtureLedgerPath, "1|1970-01-01T00:00:00Z|RECEIPT|type=seed|outcome=shipped\n")
	writeAdvanceFile(t, seed, "metasystem/records/narrator-digest.log", "digest=seed\n")
	runAdvanceGit(t, seed, "add", ".")
	runAdvanceGit(t, seed, "commit", "-qm", "seed")
	runAdvanceGit(t, base, "init", "--bare", "-q", remote)
	runAdvanceGit(t, remote, "symbolic-ref", "HEAD", "refs/heads/main")
	runAdvanceGit(t, seed, "remote", "add", "origin", remote)
	runAdvanceGit(t, seed, "push", "-q", "-u", "origin", "main")
	runAdvanceGit(t, base, "clone", "-q", remote, top)
	runAdvanceGit(t, base, "clone", "-q", remote, peer)
	configureAdvanceGit(t, top)
	configureAdvanceGit(t, peer)
	f := &carriedStageFixture{advanceFixture: &advanceFixture{t: t, root: filepath.Join(top, "metasystem"), peer: peer, remote: remote}, top: top}
	f.fetch()
	f.subject = f.compose(map[string]string{"metasystem/product.txt": "one\nTWO\nthree\n", "benchmark/run.sh": "candidate\n", "metasystem/added.txt": "new\n"})
	return f
}

// compose is the candidate a goal's composition would produce on the
// fetched endpoint: the endpoint tree with the given files replaced,
// projected as the landing owner projects it.
func (f *carriedStageFixture) compose(files map[string]string) CarriedSubject {
	f.t.Helper()
	endpoint := runAdvanceGit(f.t, f.top, "rev-parse", "refs/remotes/origin/main")
	index := filepath.Join(f.t.TempDir(), "index")
	scratch := func(args ...string) string {
		f.t.Helper()
		return runAdvanceGitEnv(f.t, f.top, []string{"GIT_INDEX_FILE=" + index}, args...)
	}
	scratch("read-tree", endpoint+"^{tree}")
	for path, content := range files {
		blob := runAdvanceGitInput(f.t, f.top, content, "hash-object", "-w", "--stdin")
		scratch("update-index", "--add", "--cacheinfo", "100644,"+blob+","+path)
	}
	candidate := scratch("write-tree")
	endpointTree := runAdvanceGit(f.t, f.top, "rev-parse", endpoint+"^{tree}")
	endpointWorkspace, err := ProjectWorkspaceTree(f.root, endpointTree)
	if err != nil {
		f.t.Fatal(err)
	}
	workspace, err := ProjectWorkspaceTree(f.root, candidate)
	if err != nil {
		f.t.Fatal(err)
	}
	return CarriedSubject{Goal: "carried-goal", Endpoint: endpoint, EndpointWorkspace: endpointWorkspace, Workspace: workspace}
}

func (f *carriedStageFixture) stage() (CarriedStaged, error) {
	return StageCarriedCandidate(f.request())
}

func (f *carriedStageFixture) request() CarriedStage {
	return CarriedStage{Root: f.root, Upstream: "refs/remotes/origin/main", Subject: f.subject, Exception: "missing-declaration", Opid: "WORD-1",
		Now: func() time.Time { return time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC) }}
}

// checkout is every byte a refusal must preserve: HEAD, the index (staged
// entries, including unmerged ones) and the worktree's status and diff.
func (f *carriedStageFixture) checkout() string {
	f.t.Helper()
	return strings.Join([]string{
		runAdvanceGit(f.t, f.top, "rev-parse", "HEAD"),
		advanceGitOutput(f.t, f.top, "ls-files", "--stage"),
		f.status(),
		advanceGitOutput(f.t, f.top, "diff", "--binary"),
	}, "\n---\n")
}

// status is the whole checkout's status, ignored files included, less the
// checkout mutation lock's own file, which every lock holder creates.
func (f *carriedStageFixture) status() string {
	f.t.Helper()
	lock, err := filepath.Rel(f.top, lease.LockPath(f.root))
	if err != nil {
		f.t.Fatal(err)
	}
	lines := []string{}
	for _, line := range strings.Split(advanceGitOutput(f.t, f.top, "status", "--porcelain=v1", "--untracked-files=all", "--ignored"), "\n") {
		if line != "!! "+filepath.ToSlash(lock) {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

func (f *carriedStageFixture) goalRows(ref string) []string {
	f.t.Helper()
	rows := []string{}
	for _, line := range strings.Split(advanceGitOutput(f.t, f.top, "show", ref+":"+carriedFixtureLedgerPath), "\n") {
		if strings.Contains(line, "|goal=carried-goal|") {
			rows = append(rows, line)
		}
	}
	return rows
}

func assertCarriedRefusal(t *testing.T, err error, code string) {
	t.Helper()
	var refusal *CarriedRefusal
	if !errors.As(err, &refusal) || refusal.Code != code {
		t.Fatalf("refusal = %v, want %s", err, code)
	}
}

// TestCarriedStageAppliesTheWholeCandidateWithItsReceipt: the candidate's
// installation and top-level product reach the index and worktree exactly,
// the one truthful RECEIPT row is appended behind every existing row, and
// the exact repeat changes nothing.
func TestCarriedStageAppliesTheWholeCandidateWithItsReceipt(t *testing.T) {
	t.Parallel()
	f := newCarriedStageFixture(t)
	appendAdvanceFile(t, f.root, "memory/receipts.log", "2|2026-09-26T11:00:00Z|RECEIPT|type=other|outcome=shipped|note=hook row\n")
	staged, err := f.stage()
	if err != nil || !staged.Applied || staged.Receipt != "pass:staged-existing" && staged.Receipt != "pass:appended" {
		t.Fatalf("stage = %+v, %v", staged, err)
	}
	for path, want := range map[string]string{"metasystem/product.txt": "one\nTWO\nthree\n", "benchmark/run.sh": "candidate\n", "metasystem/added.txt": "new\n"} {
		if got := advanceGitOutput(t, f.top, "show", ":"+path); got+"\n" != want {
			t.Fatalf("index %s = %q, want %q", path, got, want)
		}
	}
	if drift := advanceGitOutput(t, f.top, "status", "--porcelain=v1", "--untracked-files=all"); strings.Contains(drift, " M ") || strings.Contains(drift, "??") {
		t.Fatalf("worktree disagrees with the staged candidate:\n%s", drift)
	}
	ledger := advanceGitOutput(t, f.top, "show", ":"+carriedFixtureLedgerPath)
	if !strings.HasPrefix(ledger, "1|1970-01-01T00:00:00Z|RECEIPT|type=seed") || !strings.Contains(ledger, "note=hook row") {
		t.Fatalf("staged ledger lost existing rows:\n%s", ledger)
	}
	before := f.checkout()
	again, err := f.stage()
	if err != nil || again.Applied || f.checkout() != before {
		t.Fatalf("exact repeat = %+v, %v; checkout changed=%v", again, err, f.checkout() != before)
	}
	if rows := f.goalRows(""); len(rows) != 1 || !strings.Contains(rows[0], "type=implement|outcome=reworked") ||
		!strings.Contains(rows[0], "verify=skipped") || !strings.Contains(rows[0], "WORD-1") || strings.Contains(rows[0], "pending") {
		t.Fatalf("goal receipt rows = %v", rows)
	}
}

// TestCarriedStageReusesAnAppendAfterACrash: a crash after receipt.Add but
// before the ledger was staged leaves the row in the worktree; the retry
// stages that row instead of writing a second one.
func TestCarriedStageReusesAnAppendAfterACrash(t *testing.T) {
	t.Parallel()
	f := newCarriedStageFixture(t)
	if _, err := f.stage(); err != nil {
		t.Fatal(err)
	}
	runAdvanceGit(t, f.top, "reset", "-q", "--", carriedFixtureLedgerPath)
	staged, err := f.stage()
	if err != nil || staged.Applied || staged.Receipt != "pass:staged-existing" {
		t.Fatalf("retry = %+v, %v", staged, err)
	}
	if rows := f.goalRows(""); len(rows) != 1 {
		t.Fatalf("goal receipt rows after the retry = %v", rows)
	}
}

// TestCarriedStagePreservesCheckout: every checkout state the candidate does
// not own refuses before any byte, index entry or ref moves.
func TestCarriedStagePreservesCheckout(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name    string
		prepare func(*carriedStageFixture)
		code    string
	}{
		{name: "unstaged installation file", code: "carried-checkout-dirty", prepare: func(f *carriedStageFixture) {
			writeAdvanceFile(f.t, f.root, "scripts/agents/path-classes.txt", "edited\n")
		}},
		{name: "untracked beside the installation", code: "carried-checkout-dirty", prepare: func(f *carriedStageFixture) {
			writeAdvanceFile(f.t, f.top, "notes.txt", "a person's scratch\n")
		}},
		{name: "unrelated staged file", code: "carried-index-not-candidate", prepare: func(f *carriedStageFixture) {
			writeAdvanceFile(f.t, f.top, "benchmark/run.sh", "other\n")
			runAdvanceGit(f.t, f.top, "add", "benchmark/run.sh")
		}},
		{name: "unmerged index", code: "carried-index-unmerged", prepare: func(f *carriedStageFixture) {
			blob := runAdvanceGit(f.t, f.top, "rev-parse", "HEAD:benchmark/run.sh")
			runAdvanceGitInput(f.t, f.top, "0 0000000000000000000000000000000000000000\tbenchmark/run.sh\n100644 "+blob+" 1\tbenchmark/run.sh\n100644 "+blob+" 2\tbenchmark/run.sh\n100644 "+blob+" 3\tbenchmark/run.sh\n", "update-index", "--index-info")
		}},
		{name: "ignored file where the candidate adds one", code: "carried-patch-conflict", prepare: func(f *carriedStageFixture) {
			writeAdvanceFile(f.t, f.top, ".git/info/exclude", "metasystem/added.txt\n")
			writeAdvanceFile(f.t, f.root, "added.txt", "a person's own file\n")
		}},
		{name: "main product behind the endpoint", code: "carried-main-behind", prepare: func(f *carriedStageFixture) {
			writeAdvanceFile(f.t, f.peer, "metasystem/product.txt", "moved\n")
			runAdvanceGit(f.t, f.peer, "commit", "-qam", "origin product")
			runAdvanceGit(f.t, f.peer, "push", "-q", "origin", "main")
			f.fetch()
			f.subject = f.compose(map[string]string{"metasystem/product.txt": "candidate\n"})
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			f := newCarriedStageFixture(t)
			test.prepare(f)
			before := f.checkout()
			_, err := f.stage()
			assertCarriedRefusal(t, err, test.code)
			if after := f.checkout(); after != before {
				t.Fatalf("refusal changed the checkout:\nbefore=%s\nafter=%s", before, after)
			}
		})
	}
}

// TestCarriedStageRefusesAHeldCheckoutLock: staging takes the checkout
// mutation lock every mutating owner shares; a holder makes it refuse with
// nothing touched. Serial: the bounded wait is read from the environment.
func TestCarriedStageRefusesAHeldCheckoutLock(t *testing.T) {
	t.Setenv("METASYSTEM_LEASE_LOCK_WAIT_SEC", "0")
	f := newCarriedStageFixture(t)
	release, err := lease.LockBounded(lease.LockPath(f.root), "carried stage test holder")
	if err != nil {
		t.Fatal(err)
	}
	before := f.checkout()
	_, err = f.stage()
	assertCarriedRefusal(t, err, "carried-checkout-locked")
	if f.checkout() != before {
		t.Fatal("a refused stage changed the checkout")
	}
	release()
	if staged, err := f.stage(); err != nil || !staged.Applied {
		t.Fatalf("stage after release = %+v, %v", staged, err)
	}
}

// TestCarriedAdvanceSeparatesABehindMainFromMovedProduct: main that is only
// behind the endpoint the candidate was composed on advances through the
// Advance owner; origin product that moved after composition, or local
// commits origin lacks, refuse with HEAD unchanged.
func TestCarriedAdvanceSeparatesABehindMainFromMovedProduct(t *testing.T) {
	t.Parallel()
	publish := func(f *carriedStageFixture, content string) {
		writeAdvanceFile(t, f.peer, "benchmark/run.sh", content)
		runAdvanceGit(t, f.peer, "commit", "-qam", "origin product "+content)
		runAdvanceGit(t, f.peer, "push", "-q", "origin", "main")
		f.fetch()
	}
	t.Run("behind", func(t *testing.T) {
		f := newCarriedStageFixture(t)
		publish(f, "newer\n")
		f.subject = f.compose(map[string]string{"metasystem/product.txt": "candidate\n"})
		if err := CarriedAdvance(f.root, "refs/remotes/origin/main", f.subject); err != nil {
			t.Fatal(err)
		}
		if head, origin := runAdvanceGit(t, f.top, "rev-parse", "HEAD"), runAdvanceGit(t, f.top, "rev-parse", "refs/remotes/origin/main"); head != origin {
			t.Fatalf("main %s was not advanced to origin %s", head, origin)
		}
		if _, err := f.stage(); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("origin product moved after composition", func(t *testing.T) {
		f := newCarriedStageFixture(t)
		publish(f, "newer\n")
		before := f.checkout()
		assertCarriedRefusal(t, CarriedAdvance(f.root, "refs/remotes/origin/main", f.subject), "carried-origin-moved")
		_, err := f.stage()
		assertCarriedRefusal(t, err, "carried-origin-moved")
		if f.checkout() != before {
			t.Fatal("a moved origin changed the checkout")
		}
	})
	t.Run("local commits origin lacks", func(t *testing.T) {
		f := newCarriedStageFixture(t)
		publish(f, "newer\n")
		f.subject = f.compose(map[string]string{"metasystem/product.txt": "candidate\n"})
		writeAdvanceFile(t, f.root, "product.txt", "local\n")
		runAdvanceGit(t, f.top, "commit", "-qam", "local product")
		before := f.checkout()
		assertCarriedRefusal(t, CarriedAdvance(f.root, "refs/remotes/origin/main", f.subject), "carried-local-diverged")
		if f.checkout() != before {
			t.Fatal("a diverged main changed the checkout")
		}
	})
}

// TestCarriedSubjectRetention: a subject binds to one exception once, and a
// lost response adopts only the one composition of its workspace.
func TestCarriedSubjectRetention(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	id := func(c string) string { return strings.Repeat(c, 40) }
	first := CarriedSubject{Goal: "g", Endpoint: id("a"), EndpointWorkspace: id("b"), Workspace: id("c")}
	if err := RetainCarriedSubject(dir, first); err != nil {
		t.Fatal(err)
	}
	if err := RetainCarriedSubject(dir, first); err != nil {
		t.Fatalf("retaining the same subject again: %v", err)
	}
	if got, err := RetainedCarriedSubject(dir, "g", id("c")); err != nil || got != first {
		t.Fatalf("retained = %+v, %v", got, err)
	}
	if _, err := RetainedCarriedSubject(dir, "g", id("d")); err == nil {
		t.Fatal("an unknown workspace was adopted")
	} else {
		assertCarriedRefusal(t, err, "carried-subject-missing")
	}
	second := first
	second.Endpoint, second.EndpointWorkspace = id("e"), id("f")
	if err := RetainCarriedSubject(dir, second); err != nil {
		t.Fatal(err)
	}
	_, err := RetainedCarriedSubject(dir, "g", id("c"))
	assertCarriedRefusal(t, err, "carried-subject-ambiguous")
	if err := BindCarriedSubject(dir, "W", first); err != nil {
		t.Fatal(err)
	}
	assertCarriedRefusal(t, BindCarriedSubject(dir, "W", second), "carried-subject-ambiguous")
	if bound, present, err := BoundCarriedSubject(dir, "W"); err != nil || !present || bound != first {
		t.Fatalf("bound = %+v %v %v", bound, present, err)
	}
}

func runAdvanceGitEnv(t *testing.T, root string, env []string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	cmd.Env = append(gittree.ScrubbedEnviron(), env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git -C %s %v: %v\n%s", root, args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func runAdvanceGitInput(t *testing.T, root, input string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	cmd.Env = gittree.ScrubbedEnviron()
	cmd.Stdin = strings.NewReader(input)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git -C %s %v: %v\n%s", root, args, err, out)
	}
	return strings.TrimSpace(string(out))
}
