package landpath

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
)

func epochOf(value int64) *int64 { return &value }

// TestCommitStampsTrailersAndConsumesProof: a passing human commit consumes
// the retained proof for the exact index tree, stamps Machine, provenance,
// Goal-Item and Goal-Revision, removes its wrapper token and weighs itself.
func TestCommitStampsTrailersAndConsumesProof(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.expect(b.commit(CommitRequest{Goal: "g1", GoalSet: true, Chain: "j1", LandedBy: "m1e"}), 0)
	for _, line := range []string{"Machine: m1+human", "Landing-Provenance: chain=j1 change=abc", "Landing-Provenance-Verdict: pass bar=area",
		"Landed-By: m1e", "Goal-Item: g1", "Goal-Revision: 7"} {
		if countExact(b.git.message, line) != 1 {
			t.Fatalf("message lacks %q:\n%s", line, b.git.message)
		}
	}
	for _, want := range []string{"brain-fence", "require-holder", "with-held", "token pid=100 start=1700000000", "verify tree=t1 goal=g1", "observe judge= goal=g1 chain=j1", "remove worktree-commit-token.json", "weight"} {
		if !b.log.has(want) {
			t.Fatalf("owner calls lack %q: %v", want, b.log.calls)
		}
	}
}

// TestCommitRefusesBeforeAnyEffect covers every refusal the boundary makes
// before it commits, each with its exit status and text.
func TestCommitRefusesBeforeAnyEffect(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		setup   func(b *bed)
		request CommitRequest
		status  int
		text    string
	}{
		{"brain fenced", func(b *bed) {
			b.owners.BrainFence = func(string, string) (string, error) { return "land refused: this checkout is declared the brain", nil }
		}, CommitRequest{Chain: "j1"}, 2, "declared the brain"},
		{"brain fence failed", func(b *bed) {
			b.owners.BrainFence = func(string, string) (string, error) { return "", fmt.Errorf("broken") }
		}, CommitRequest{Chain: "j1"}, 1, "land refused: brain fence failed"},
		{"not the holder", func(b *bed) {
			b.owners.RequireHolder = func(string, int64, *int64) (*int64, error) { return nil, fmt.Errorf("OWNED-ELSEWHERE: held by x") }
		}, CommitRequest{}, 1, "OWNED-ELSEWHERE"},
		{"agent without lineage", func(b *bed) { b.epoch = epochOf(3) }, CommitRequest{}, 2,
			"agent commit refused: the lease holder has a claim epoch but no owner lineage"},
		{"carried without its facts", nil, CommitRequest{Carried: "op1", Goal: "g1", GoalSet: true}, 2,
			"commit refused: --carried requires --goal, --ledger-tip, --carried-by, and --carried-past"},
		{"goal not kebab", nil, CommitRequest{Goal: "Bad_Goal", GoalSet: true}, 2,
			"commit refused: --goal must be a lowercase kebab identifier of at most 100 characters"},
		{"message typed Goal-Item", func(b *bed) { b.writeMessage("x\n\ngoal-item: g1\n") }, CommitRequest{}, 2,
			"commit refused: Goal-Item and carried trailers are stamped by the wrapper, never typed"},
		{"message typed Machine", func(b *bed) { b.writeMessage("x\n\nMachine: m9+x\n") }, CommitRequest{}, 2,
			"commit refused: Machine is stamped by the wrapper, never typed"},
		{"message typed Carry", func(b *bed) { b.writeMessage("x\n\nCarry: op\n") }, CommitRequest{}, 1,
			"commit refused: Goal-Item and carried trailers are stamped by the wrapper, never typed"},
		{"message unreadable", nil, CommitRequest{MessageFile: "/nonexistent/message"}, 2,
			"commit refused: commit message file is not readable: /nonexistent/message"},
		{"start time unreadable", func(b *bed) {
			b.owners.StartedAt = func(int64) (int64, error) { return 0, fmt.Errorf("gone") }
		}, CommitRequest{}, 1, "agent commit wrapper refused: wrapper process start time is unreadable"},
		{"session trailer domain", func(b *bed) { b.writeMessageAt("claude.ac/x", "x\n") }, CommitRequest{}, 2,
			"commit refused: the session trailer says claude.ac — the domain is claude.ai"},
		{"unmerged index", func(b *bed) { b.git.on("write-tree", func(GitCall) GitResult { return failed(128, "unmerged\n") }) }, CommitRequest{}, 1,
			"agent commit refused: the index cannot be proved as a tree (unmerged entries?)"},
		{"no testing contract", func(b *bed) { b.owners.ConfValue = func(string, string) string { return "" } }, CommitRequest{}, 1,
			"agent commit refused: testing.contract is required in committed metasystem.conf"},
		{"proof missing", func(b *bed) {
			b.owners.Verify = func(_ VerifyRequest, _, stderr io.Writer) int {
				fmt.Fprintln(stderr, "missing required proof")
				return 1
			}
		}, CommitRequest{}, 1, "agent commit refused: required shared testing proof is missing or insufficient"},
		{"unbound working-tree bytes", func(b *bed) {
			b.git.on("diff --no-renames --name-only -z --", func(GitCall) GitResult { return ok("internal/x.go\x00") })
		}, CommitRequest{}, 1, "  internal/x.go\nstage, stash, or remove them"},
		{"staged gitlink", func(b *bed) {
			b.git.on("ls-files -s -z", func(GitCall) GitResult { return ok("160000 abc 0\tvendor/sub\x00") })
		}, CommitRequest{}, 1, "a staged gitlink inside the proof scope"},
		{"critical symlink", func(b *bed) {
			b.git.on("ls-files -s -z", func(GitCall) GitResult { return ok("120000 abc 0\tinternal/a.go\x00120000 abc 0\tskills/x\x00") })
		}, CommitRequest{}, 1, "a critical proof input is a symlink"},
		{"hidden entry", func(b *bed) {
			b.git.on("ls-files -v -z", func(GitCall) GitResult { return ok("S cmd/a.go\x00H ok.go\x00") })
		}, CommitRequest{}, 1, "assume-unchanged or skip-worktree entries hide proof inputs"},
		{"index moved during proof", func(b *bed) {
			trees := []string{"t1\n", "t2\n"}
			b.git.on("write-tree", func(GitCall) GitResult { tree := trees[0]; trees = trees[1:]; return ok(tree) })
		}, CommitRequest{}, 1, "agent commit refused: the index or a gate input moved while the proof ran; re-stage and retry"},
		{"no machine nickname", func(b *bed) { b.git.machine = "" }, CommitRequest{}, 2,
			"commit refused: no machine nickname is enrolled"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b := newBed(t)
			if c.setup != nil {
				c.setup(b)
			}
			b.expect(b.commit(c.request), c.status, c.text)
			if len(b.git.called("commit")) != 0 {
				t.Fatalf("a refused boundary committed: %v", b.git.calls)
			}
		})
	}
}

func (b *bed) writeMessage(text string) {
	b.t.Helper()
	if err := os.WriteFile(b.messageFile(), []byte(text), 0o644); err != nil {
		b.t.Fatal(err)
	}
}

// writeMessageAt writes the message at a path containing name and makes it
// the bed's message file.
func (b *bed) writeMessageAt(name, text string) {
	b.t.Helper()
	path := filepath.Join(b.root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		b.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		b.t.Fatal(err)
	}
	b.messagePath = path
}

// TestCommitAgentRefusalNamesCauseAndExits: every would-refuse code an
// agent commit meets prints its cause, the staged paths and the lawful
// exits; an attested landing's refusal exits 3.
func TestCommitAgentRefusalNamesCauseAndExits(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"evaluator-unavailable":               "the landing evaluator failed or returned an incomplete decision",
		"path-unclassified":                   "the landing contains an unclassified path",
		"ledger-path-not-goal-verb":           "ledger paths change only through goal verbs",
		"runtime-path-refused":                "runtime paths cannot be landed",
		"exact-revert-record-refused":         "exact revert cannot delete or truncate records",
		"goal-item-not-held":                  "the Goal-Item is not held by this machine and lineage",
		"goal-revision-moved":                 "the Goal-Item's claim revision moved since this chain was dispatched",
		"goal-binding-missing":                "this landing names no goal and the ledger is not Goal-free",
		"goal-binding-mismatch":               "the chain was dispatched under a different goal than --goal names",
		"record-not-owned":                    "the staged record is not owned by this landing",
		"register-carriage-policy-unreadable": "the base path-class policy is unreadable",
		"register-carriage-not-append-only":   "register carriage rewrote or deleted existing record bytes",
		"something-else":                      "agent commit refused: landing verdict would-refuse code=something-else",
	}
	for code, text := range cases {
		t.Run(code, func(t *testing.T) {
			b := newBed(t)
			b.epoch = epochOf(4)
			b.observed = landing.Observation{Mode: "refuse", RefusesAgent: true, Code: code, Provenance: "none change=x",
				VerdictTrailer: "would-refuse code=" + code, Refusal: "detail of " + code}
			b.git.on("diff --cached --name-only -z --", func(GitCall) GitResult { return ok("a b.go\x00") })
			b.expect(b.commit(CommitRequest{OwnerLineage: "L", Chain: "j1"}), 1, text, "staged paths:\n  a\\ b.go\n", "lawful classification exits:")
			if len(b.git.called("commit")) != 0 {
				t.Fatal("a refused agent commit was recorded")
			}
		})
	}
	b := newBed(t)
	b.epoch = epochOf(4)
	b.observed = landing.Observation{Mode: "refuse", RefusesAgent: true, Code: "attested-x", Provenance: "p", VerdictTrailer: "would-refuse code=attested-x"}
	b.expect(b.commit(CommitRequest{OwnerLineage: "L", Attested: "c1"}), 3, "landing verdict would-refuse code=attested-x")
}

// TestCommitIncompleteObservationRefusesAgentAndAdmitsHuman: an evaluator
// that returns no complete decision is evaluator-unavailable; a human
// commit is sovereign and lands with that verdict, an agent's is refused.
func TestCommitIncompleteObservationRefusesAgentAndAdmitsHuman(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.observed = landing.Observation{Mode: "observe"}
	b.expect(b.commit(CommitRequest{}), 0)
	if countExact(b.git.message, "Landing-Provenance-Verdict: would-refuse code=evaluator-unavailable") != 1 {
		t.Fatalf("human commit verdict:\n%s", b.git.message)
	}
	b = newBed(t)
	b.epoch = epochOf(2)
	b.observed = landing.Observation{Mode: "observe"}
	b.expect(b.commit(CommitRequest{OwnerLineage: "L"}), 1, "the landing evaluator failed or returned an incomplete decision")
	if b.log.count("require-holder epoch=true") != 1 {
		t.Fatalf("the agent epoch was not re-proved under the lease: %v", b.log.calls)
	}
}

// TestCommitPostconditionRollsBack: a commit that recorded other bytes or
// other trailers than proved is rolled back softly and refused.
func TestCommitPostconditionRollsBack(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		setup func(b *bed)
		text  string
	}{
		{"tree", func(b *bed) {
			b.git.on("rev-parse HEAD^{tree}", func(GitCall) GitResult { return ok("other\n") })
		}, "the commit recorded a tree the static re-proof never judged"},
		{"machine", func(b *bed) {
			b.git.on("log -1 --format=%B", func(GitCall) GitResult { return ok(b.git.message + "\nMachine: m2+x\n") })
		}, "expected exactly one Machine trailer, found 2"},
		{"goal item", func(b *bed) {
			b.git.on("log -1 --format=%B", func(GitCall) GitResult { return ok(b.git.message + "\nGoal-Item: g1\n") })
		}, "did not contain exactly one byte-exact Goal-Item stamped by --goal; the commit was rolled back"},
		{"stray carried", func(b *bed) {
			b.git.on("log -1 --format=%B", func(GitCall) GitResult { return ok(b.git.message + "\nCarry: op\n") })
		}, "failed the carried-trailer postcondition (Carry must be absent)"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b := newBed(t)
			c.setup(b)
			b.expect(b.commit(CommitRequest{Goal: "g1", GoalSet: true}), 1, c.text)
			if len(b.git.called("reset --soft h0")) != 1 {
				t.Fatalf("not rolled back: %v", b.git.calls)
			}
		})
	}
	b := newBed(t)
	b.git.on("rev-parse --verify --quiet HEAD", func(GitCall) GitResult { return failed(1, "") })
	b.git.on("rev-parse HEAD^{tree}", func(GitCall) GitResult { return ok("other\n") })
	b.expect(b.commit(CommitRequest{}), 1)
	if len(b.git.called("update-ref -d HEAD")) != 1 {
		t.Fatalf("unborn rollback: %v", b.git.calls)
	}
}

// TestCommitGitFailureAndWeightSkip: git's own refusal is the boundary's
// status; weight bookkeeping never refuses a concluded commit.
func TestCommitGitFailureAndWeightSkip(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.git.on("commit", func(GitCall) GitResult { return failed(1, "pre-commit guard: refusing\n") })
	b.expect(b.commit(CommitRequest{}), 1, "pre-commit guard: refusing")
	b = newBed(t)
	b.owners.WeightAdd = func(string, string, string, string, []byte, io.Writer, io.Writer) int { return 1 }
	b.expect(b.commit(CommitRequest{}), 0, "validation-weight bookkeeping skipped (non-fatal)")
}

// TestCommitPushLandsOriginThenTransport: --push fetches, checks held goals
// against origin, pushes origin, then mirrors transport.
func TestCommitPushLandsOriginThenTransport(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.git.on("remote", func(GitCall) GitResult { return ok("origin\ntransport\n") })
	b.expect(b.commit(CommitRequest{Push: true}), 0)
	if !b.log.has("held base=refs/remotes/origin/main") || !b.log.has("transport main") || len(b.git.called("push origin main")) != 1 {
		t.Fatalf("push: %v %v", b.log.calls, b.git.calls)
	}
	for _, c := range []struct {
		name  string
		setup func(b *bed)
		text  string
	}{
		{"detached", func(b *bed) { b.git.branch = "" }, "landing push refused: HEAD is not on a branch"},
		{"fetch", func(b *bed) { b.git.on("fetch", func(GitCall) GitResult { return failed(1, "") }) }, "origin could not be fetched; the commit stands locally"},
		{"push", func(b *bed) { b.git.on("push", func(GitCall) GitResult { return failed(1, "") }) }, "landing push failed at origin; the commit stands locally"},
		{"transport", func(b *bed) {
			b.git.on("remote", func(GitCall) GitResult { return ok("transport\n") })
			b.owners.SyncTransport = func(string, string, io.Writer, io.Writer) int { return 1 }
		}, "landing push failed at transport with origin already pushed"},
	} {
		t.Run(c.name, func(t *testing.T) {
			b := newBed(t)
			c.setup(b)
			b.expect(b.commit(CommitRequest{Push: true}), 1, c.text)
		})
	}
}

// TestCommitCarriedStampsJudgeBatteryAndLedger: a carried commit is decided
// by the live judge, binds the word, refusal and ledger, and stamps the
// carried trailers; a red battery names its missing and failing groups.
func TestCommitCarriedStampsJudgeBatteryAndLedger(t *testing.T) {
	t.Parallel()
	request := CommitRequest{Goal: "g1", GoalSet: true, Carried: "op1", LedgerTip: "L1", CarriedBy: "human:wido", CarriedPast: "group:unit", HeldEpoch: "human"}
	carriedObservation := landing.Observation{Mode: "observe", Code: "human-carried", Provenance: "carried opid=op1 past=group:unit ledger=L1 x",
		VerdictTrailer: "pass carried", GoalRevision: 3}
	b := newBed(t)
	b.observed = carriedObservation
	live := b.owners.Live
	b.owners.Live = func() Judge {
		judge := live()
		judge.VerifyCarried = func(string, string, string) ([]byte, int) {
			return []byte(`{"delivery":{"sufficient":false,"missingGroups":["a","b"],"failingGroups":[]}}`), 1
		}
		return judge
	}
	b.expect(b.commit(request), 0)
	for _, line := range []string{"Carry: op1", "Carried-By: human:wido", "Carried-Tree: workspace=w-t1 project=t1", "Carried-Past: group:unit",
		"Carried-Battery: red missing=a,b failing=-", "Carried-Ledger: L1", "Goal-Item: g1"} {
		if countExact(b.git.message, line) != 1 {
			t.Fatalf("carried message lacks %q:\n%s", line, b.git.message)
		}
	}
	if !strings.Contains(b.git.message, "Carried-Judge: live sha256=") || b.log.has("verify ") || b.log.has("with-held") {
		t.Fatalf("carried judge or lease: %s %v", b.git.message, b.log.calls)
	}

	b = newBed(t)
	b.observed = landing.Observation{Mode: "refuse", Code: "conflicting-declarations", Refusal: "two declarations", Provenance: "p", VerdictTrailer: "would-refuse x"}
	b.expect(b.commit(request), 3, "conflicting-declarations: two declarations")

	b = newBed(t)
	b.observed = landing.Observation{Mode: "observe", Code: "human-carried", Provenance: "carried opid=other past=group:unit ledger=L1 x", VerdictTrailer: "pass"}
	b.expect(b.commit(request), 3, "carried landing asks: the deciding observation does not bind the requested word, refusal, and ledger")

	b = newBed(t)
	b.observed = carriedObservation
	b.owners.Live = func() Judge {
		judge := live()
		judge.VerifyCarried = func(string, string, string) ([]byte, int) { return []byte("not json"), 1 }
		return judge
	}
	b.expect(b.commit(request), 3, "test verify failed: no structured delivery result")
}

// TestCommitCarriedFallsBackToBaseJudge: when the live engine cannot decide
// a carried landing, the base engine built from HEAD decides, and the judge
// trailer names its tree, digest and the live failure.
func TestCommitCarriedFallsBackToBaseJudge(t *testing.T) {
	t.Parallel()
	request := CommitRequest{Goal: "g1", GoalSet: true, Carried: "op1", LedgerTip: "L1", CarriedBy: "human:wido", CarriedPast: "group:unit", HeldEpoch: "human"}
	b := newBed(t)
	b.observed = landing.Observation{Mode: "observe", Code: "evaluator-crashed"}
	cleaned := false
	b.owners.BuildBaseJudge = func(string, string, io.Writer) (Judge, func(), error) {
		return Judge{Digest: "d1",
			Observe: func(r ObserveRequest) (landing.Observation, int) {
				if r.Judge != "base" || r.LiveFailure != "evaluator-crashed" {
					t.Fatalf("base request %+v", r)
				}
				return landing.Observation{Mode: "observe", Code: "human-carried", Provenance: "c opid=op1 past=group:unit ledger=L1 x", VerdictTrailer: "pass", GoalRevision: 3}, 0
			},
			Workspace:     func(_, tree string) (string, error) { return "bw-" + tree, nil },
			VerifyCarried: func(string, string, string) ([]byte, int) { return []byte(`{"delivery":{"sufficient":true}}`), 0 },
		}, func() { cleaned = true }, nil
	}
	b.expect(b.commit(request), 0)
	if countExact(b.git.message, "Carried-Judge: base tree=t1 sha256=d1 live-failure=evaluator-crashed") != 1 ||
		countExact(b.git.message, "Carried-Tree: workspace=bw-t1 project=t1") != 1 || !cleaned {
		t.Fatalf("base judge message:\n%s", b.git.message)
	}
	b = newBed(t)
	b.observed = landing.Observation{}
	b.expect(b.commit(request), 3, "no live or base judge decided; the base judge build failed")
}

// TestCommitRetiredComposerRefusalNamesTheFix: a pre-commit composer from
// before the engine guard refuses every commit without naming a fix; the
// commit path's refusal names the command that re-enrolls it (rule H1).
func TestCommitRetiredComposerRefusalNamesTheFix(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.git.on("commit", func(GitCall) GitResult {
		return failed(1, "pre-commit: the metasystem ledger guard is missing at /r/metasystem/scripts/agents/pre-commit-guard.sh; refusing to commit without the fence\n")
	})
	b.expect(b.commit(CommitRequest{}), 1, "re-enroll it with: metasystem system setup")
}
