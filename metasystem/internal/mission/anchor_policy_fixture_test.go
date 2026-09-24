package mission

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testgit"
)

// Each fact is a declared raw anchor position. Its ledger bytes and state hash
// are copied before publication, so later file changes cannot change Git's
// declared answer.
type anchorPolicyFact struct {
	commit, tree, blob           string
	stateHash, ledger, ledgerSHA string
	cycle                        int64
	parent                       *anchorPolicyFact
}

type anchorPolicyBed struct {
	t                   *testing.T
	repo, state, ledger string
	fact                *anchorPolicyFact
	usedIndexDirs       map[string]bool
}

func newAnchorPolicyBed(t *testing.T) *anchorPolicyBed {
	t.Helper()
	b := &anchorPolicyBed{t: t, repo: t.TempDir(), usedIndexDirs: make(map[string]bool)}
	b.ledger = filepath.Join(b.repo, "ledger.md")
	writeText(t, b.ledger, oneCycleLedger)
	contract := filepath.Join(b.repo, "mission-demo.contract.md")
	writeText(t, contract, "```mission\ncandidate.branch=feature-x\nstream.alpha=Do alpha\n```\n")
	b.state = filepath.Join(b.repo, "state.json")
	if err := InitStateWithBaseline(b.state, contract, b.ledger, "", "feature-x", strings.Repeat("b", 40), testAdmissionOrigins()); err != nil {
		t.Fatal(err)
	}
	_, hash, _ := VerifyStateShape(b.state)
	doc, _ := readStateDoc(b.state)
	doc["ledger"].(map[string]any)["cycles"] = 1
	src := b.state + ".src"
	if err := atomicWriteJSON(src, doc); err != nil {
		t.Fatal(err)
	}
	if err := WriteState(b.state, src, hash); err != nil {
		t.Fatal(err)
	}
	b.publish("c", "")
	return b
}

func newStagnationAnchorPolicyBed(t *testing.T) (*anchorPolicyBed, string) {
	t.Helper()
	b := newAnchorPolicyBed(t)
	_, hash, _ := VerifyStateShape(b.state)
	doc, _ := readStateDoc(b.state)
	doc["status"] = "parked"
	doc["parkReason"] = "stop-loss"
	doc["waitingList"] = []any{"stop-loss"}
	src := b.state + ".src"
	if err := atomicWriteJSON(src, doc); err != nil {
		t.Fatal(err)
	}
	if err := WriteState(b.state, src, hash); err != nil {
		t.Fatal(err)
	}
	b.publish("d", "")
	asksDir := filepath.Join(filepath.Dir(b.state), "asks")
	writeAsk(t, asksDir, "stop-loss", StopLossKindStagnation)
	return b, asksDir
}

func (b *anchorPolicyBed) nextFact(id string) *anchorPolicyFact {
	b.t.Helper()
	doc, err := readStateDoc(b.state)
	if err != nil {
		b.t.Fatal(err)
	}
	data, err := os.ReadFile(b.ledger)
	if err != nil {
		b.t.Fatal(err)
	}
	integrity := doc["integrity"].(map[string]any)
	cycles, _ := intValue(doc["ledger"].(map[string]any)["cycles"])
	blob := sha256Hex(string(data))[:40]
	return &anchorPolicyFact{
		commit: strings.Repeat(id, 40), tree: sha256Hex("tree:" + blob)[:40], blob: blob,
		stateHash: integrity["hash"].(string), ledger: string(data), ledgerSHA: sha256Hex(string(data)),
		cycle: cycles, parent: b.fact,
	}
}

func (b *anchorPolicyBed) publish(id, identity string) {
	b.t.Helper()
	next := b.nextFact(id)
	q := b.queue()
	q.expectPublication(next, identity)
	got, err := anchorWritePinnedWithOperations(q.operations(), b.state, b.repo, b.ledger, identity, "", "")
	if err != nil || got != next.commit {
		b.t.Fatalf("anchor publication: commit=%q err=%v", got, err)
	}
	b.fact = next
}

type anchorPolicyQueue struct {
	b         *anchorPolicyBed
	calls     []testgit.Expectation
	indexDirs map[string]bool
}

type anchorPolicyExit int

func (e anchorPolicyExit) Error() string { return fmt.Sprintf("git exit %d", int(e)) }

func (b *anchorPolicyBed) queue() *anchorPolicyQueue {
	return &anchorPolicyQueue{b: b, indexDirs: make(map[string]bool)}
}

func (q *anchorPolicyQueue) output(stdout string, args ...string) {
	q.calls = append(q.calls, testgit.Expectation{
		Call:   testgit.Call{Dir: q.b.repo, Args: args},
		Result: testgit.Result{Stdout: []byte(stdout)},
	})
}

func (q *anchorPolicyQueue) try(stdout string, code int, args ...string) {
	var err error
	if code != 0 {
		err = anchorPolicyExit(code)
	}
	q.calls = append(q.calls, testgit.Expectation{
		Call:   testgit.Call{Dir: q.b.repo, Args: args},
		Result: testgit.Result{Stdout: []byte(stdout), Err: err},
	})
}

func (q *anchorPolicyQueue) expectTip(f *anchorPolicyFact) {
	ref := stateAnchorRef("demo")
	if f == nil {
		q.output("", "for-each-ref", "--format=%(refname)", ref)
		return
	}
	q.output(ref+"\n", "for-each-ref", "--format=%(refname)", ref)
	message := fmt.Sprintf("mission(demo): anchor cycle %d\n\nMission-Id: demo\nMission-State-Hash: %s\nMission-Ledger-SHA256: %s\nMission-Ledger-Path: ledger.md\nMission-Cycle: %d\n\n", f.cycle, f.stateHash, f.ledgerSHA, f.cycle)
	q.output(f.commit+"\x1f"+message, "log", "-1", "--format=%H%x1f%B", ref)
}

func (q *anchorPolicyQueue) expectTree(f *anchorPolicyFact) {
	listing := func(f *anchorPolicyFact) string { return "100644 blob " + f.blob + "\tledger.md\x00" }
	q.try(listing(f), 0, "ls-tree", "-r", "--full-tree", "-z", f.commit)
	parents := f.commit
	if f.parent != nil {
		parents += " " + f.parent.commit
	}
	q.try(parents+"\n", 0, "rev-list", "--parents", "-n", "1", f.commit)
	if f.parent != nil {
		q.try(listing(f.parent), 0, "ls-tree", "-r", "--full-tree", "-z", f.parent.commit)
	}
}

func (q *anchorPolicyQueue) expectVerify(f *anchorPolicyFact) {
	q.expectTip(f)
	q.try("", 0, "merge-base", "--is-ancestor", f.commit, stateAnchorRef("demo"))
	q.try(f.ledger, 0, "show", f.commit+":ledger.md")
	q.expectTree(f)
}

func (q *anchorPolicyQueue) expectPrefix(f *anchorPolicyFact) {
	q.expectTip(f)
	q.try(f.ledger, 0, "show", f.commit+":ledger.md")
	q.try("", 0, "merge-base", "--is-ancestor", f.commit, stateAnchorRef("demo"))
	q.expectTree(f)
}

func (q *anchorPolicyQueue) expectLag(f *anchorPolicyFact, readBlob bool) {
	q.expectTip(f)
	if readBlob {
		q.try(f.ledger, 0, "show", f.commit+":ledger.md")
	}
}

func (q *anchorPolicyQueue) expectPark(f *anchorPolicyFact, readBlob bool) {
	q.output(".git\n", "rev-parse", "--git-dir")
	q.expectLag(f, readBlob)
}

func (q *anchorPolicyQueue) expectPublication(next *anchorPolicyFact, identity string) {
	q.output("feature-x\n", "branch", "--show-current")
	q.expectTip(next.parent)
	q.calls = append(q.calls, testgit.Expectation{
		Call:   testgit.Call{Dir: q.b.repo, Args: []string{"hash-object", "-w", "--no-filters", "--stdin"}, Stdin: []byte(next.ledger)},
		Result: testgit.Result{Stdout: []byte(next.blob + "\n")},
	})
	q.index("", "read-tree", "--empty")
	q.index("", "update-index", "--add", "--cacheinfo", "100644,"+next.blob+",ledger.md")
	q.index(next.tree+"\n", "write-tree")
	if next.parent == nil {
		q.try("", 128, "rev-parse", "--verify", stateAnchorRef("demo"))
	} else {
		q.try(next.parent.commit+"\n", 0, "rev-parse", "--verify", stateAnchorRef("demo"))
	}
	author := identity
	if author == "" {
		author = "metasystem"
	}
	message := fmt.Sprintf("mission(demo): anchor cycle %d\n\nMission-Id: demo\nMission-State-Hash: %s\nMission-Ledger-SHA256: %s\nMission-Ledger-Path: ledger.md\nMission-Cycle: %d", next.cycle, next.stateHash, next.ledgerSHA, next.cycle)
	args := []string{"commit-tree", next.tree, "-m", message}
	if next.parent != nil {
		args = append(args, "-p", next.parent.commit)
	}
	q.calls = append(q.calls, testgit.Expectation{
		Call: testgit.Call{Dir: q.b.repo, Args: args, Env: []string{
			"GIT_AUTHOR_NAME=" + author, "GIT_AUTHOR_EMAIL=" + author + "@example.invalid",
			"GIT_COMMITTER_NAME=metasystem", "GIT_COMMITTER_EMAIL=metasystem@example.invalid",
		}},
		Result: testgit.Result{Stdout: []byte(next.commit + "\n")},
	})
	update := []string{"update-ref", stateAnchorRef("demo"), next.commit, ""}
	if next.parent != nil {
		update[3] = next.parent.commit
	}
	q.output("", update...)
}

func (q *anchorPolicyQueue) index(stdout string, args ...string) {
	q.calls = append(q.calls, testgit.Expectation{
		Call:   testgit.Call{Dir: q.b.repo, Args: args},
		Result: testgit.Result{Stdout: []byte(stdout)},
		Check: func(call testgit.Call) error {
			if len(call.Env) != 1 || len(call.Stdin) != 0 || !strings.HasPrefix(call.Env[0], "GIT_INDEX_FILE=") {
				return fmt.Errorf("unexpected index environment or stdin: env=%q stdin=%q", call.Env, call.Stdin)
			}
			path := strings.TrimPrefix(call.Env[0], "GIT_INDEX_FILE=")
			dir := filepath.Dir(path)
			if filepath.Base(path) != "index" || !strings.HasPrefix(filepath.Base(dir), "metasystem-anchor.") || filepath.Dir(dir) != os.TempDir() {
				return fmt.Errorf("index is not in a fresh temporary anchor directory: %q", path)
			}
			if _, err := os.Stat(dir); err != nil {
				return fmt.Errorf("index directory absent: %w", err)
			}
			if len(q.indexDirs) == 0 {
				if q.b.usedIndexDirs[dir] {
					return fmt.Errorf("index directory reused across publications: %q", dir)
				}
				q.b.usedIndexDirs[dir] = true
				q.indexDirs[dir] = true
			}
			if !q.indexDirs[dir] || len(q.indexDirs) != 1 {
				return fmt.Errorf("index path changed within publication: %q", path)
			}
			return nil
		},
	})
}

func (q *anchorPolicyQueue) operations() anchorOperations {
	stub := testgit.New(q.b.t, q.calls...)
	return anchorOperations{
		gitOutput: func(repo string, args ...string) (string, error) {
			r := stub.Run(testgit.Call{Dir: repo, Args: args})
			return string(r.Stdout), r.Err
		},
		gitTry: func(repo string, args ...string) (string, int) {
			r := stub.Run(testgit.Call{Dir: repo, Args: args})
			if r.Err == nil {
				return string(r.Stdout), 0
			}
			if e, ok := r.Err.(anchorPolicyExit); ok {
				return string(r.Stdout), int(e)
			}
			return string(r.Stdout), -1
		},
		gitStdinOutput: func(repo string, stdin []byte, args ...string) (string, error) {
			r := stub.Run(testgit.Call{Dir: repo, Args: args, Stdin: stdin})
			return string(r.Stdout), r.Err
		},
		gitEnvOutput: func(repo string, env []string, args ...string) (string, error) {
			r := stub.Run(testgit.Call{Dir: repo, Args: args, Env: env})
			return string(r.Stdout), r.Err
		},
	}
}

func TestAnchorPolicyRefusesLedgerMoveWithoutStateWrite(t *testing.T) {
	b := newAnchorPolicyBed(t)
	second := b.nextFact("d")
	q := b.queue()
	q.expectPublication(second, "")
	if _, err := anchorWritePinnedWithOperations(q.operations(), b.state, b.repo, b.ledger, "", "", ""); err != nil {
		t.Fatalf("an identical re-anchor must stay allowed: %v", err)
	}
	b.fact = second
	writeText(t, b.ledger, strings.Replace(oneCycleLedger, "observed=score=0.5", "observed=score=0.9", 1))
	q = b.queue()
	q.output("feature-x\n", "branch", "--show-current")
	q.expectTip(b.fact)
	if _, err := anchorWritePinnedWithOperations(q.operations(), b.state, b.repo, b.ledger, "", "", ""); err == nil ||
		!strings.Contains(err.Error(), "ledger bytes changed without a state write") {
		t.Fatalf("a ledger move without a state write must refuse, got %v", err)
	}
}
