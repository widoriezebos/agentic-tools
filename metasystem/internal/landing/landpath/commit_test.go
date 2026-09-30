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
		}, CommitRequest{Chain: "j1"}, 1, "this checkout's role couldn't be read"},
		{"not the holder", func(b *bed) {
			b.owners.RequireHolder = func(string, int64, *int64) (*int64, error) { return nil, fmt.Errorf("OWNED-ELSEWHERE: held by x") }
		}, CommitRequest{}, 1, "OWNED-ELSEWHERE"},
		{"agent without lineage", func(b *bed) { b.epoch = epochOf(3) }, CommitRequest{}, 2,
			"this agent shell doesn't say which session it is"},
		{"carried without its facts", nil, CommitRequest{Carried: "op1", Goal: "g1", GoalSet: true}, 2,
			"--carried requires --goal, --ledger-tip, --carried-by, and --carried-past"},
		{"goal not kebab", nil, CommitRequest{Goal: "Bad_Goal", GoalSet: true}, 2,
			"\"Bad_Goal\" is not a goal id"},
		{"message typed Goal-Item", func(b *bed) { b.writeMessage("x\n\ngoal-item: g1\n") }, CommitRequest{}, 2,
			"the commit message types a line the landing adds itself (Goal-Item:)"},
		{"message typed Machine", func(b *bed) { b.writeMessage("x\n\nMachine: m9+x\n") }, CommitRequest{}, 2,
			"the commit message types a line the landing adds itself (Machine:)"},
		{"message typed Carry", func(b *bed) { b.writeMessage("x\n\nCarry: op\n") }, CommitRequest{}, 1,
			"the commit message types a line the landing adds itself (Carry:)"},
		{"message unreadable", nil, CommitRequest{MessageFile: "/nonexistent/message"}, 2,
			"the commit message file can't be read, so nothing was committed\nneeded first: check the file named by --message, then repeat this command\n"},
		{"start time unreadable", func(b *bed) {
			b.owners.StartedAt = func(int64) (int64, error) { return 0, fmt.Errorf("gone") }
		}, CommitRequest{}, 1, "this process couldn't be read"},
		{"session trailer domain", func(b *bed) { b.writeMessageAt("claude.ac/x", "x\n") }, CommitRequest{}, 2,
			"the commit message's session link says claude.ac; the domain is claude.ai"},
		{"unmerged index", func(b *bed) { b.git.on("write-tree", func(GitCall) GitResult { return failed(128, "unmerged\n") }) }, CommitRequest{}, 1,
			"the staged files still hold merge conflicts"},
		{"no testing contract", func(b *bed) { b.owners.ConfValue = func(string, string) string { return "" } }, CommitRequest{}, 1,
			"metasystem.conf names no test contract (testing.contract)"},
		{"proof missing", func(b *bed) {
			b.owners.Verify = func(_ VerifyRequest, _, stderr io.Writer) int {
				fmt.Fprintln(stderr, "missing required proof")
				return 1
			}
		}, CommitRequest{}, 1, "the change's tests haven't passed on this checkout yet"},
		{"unbound working-tree bytes", func(b *bed) {
			b.git.on("diff --no-renames --name-only -z --", func(GitCall) GitResult { return ok("internal/x.go\x00") })
		}, CommitRequest{}, 1, "files on disk differ from the staged change: internal/x.go\nrun: git status --short  (stage, stash or remove them"},
		{"staged gitlink", func(b *bed) {
			b.git.on("ls-files -s -z", func(GitCall) GitResult { return ok("160000 abc 0\tvendor/sub\x00") })
		}, CommitRequest{}, 1, "a staged folder is a nested Git checkout the commit can't record: vendor/sub"},
		{"critical symlink", func(b *bed) {
			b.git.on("ls-files -s -z", func(GitCall) GitResult { return ok("120000 abc 0\tinternal/a.go\x00120000 abc 0\tskills/x\x00") })
		}, CommitRequest{}, 1, "a staged file the tests read is a symlink, which the landing refuses: internal/a.go"},
		{"hidden entry", func(b *bed) {
			b.git.on("ls-files -v -z", func(GitCall) GitResult { return ok("S cmd/a.go\x00H ok.go\x00") })
		}, CommitRequest{}, 1, "Git is told to ignore changes to a file the tests read: cmd/a.go"},
		{"index moved during proof", func(b *bed) {
			trees := []string{"t1\n", "t2\n"}
			b.git.on("write-tree", func(GitCall) GitResult { tree := trees[0]; trees = trees[1:]; return ok(tree) })
		}, CommitRequest{}, 1, "the staged files changed while the landing checked them"},
		{"no machine nickname", func(b *bed) { b.git.machine = "" }, CommitRequest{}, 2,
			"this machine has no name yet, so nothing was committed\nrun: git config metasystem.goal.machine NAME  (then repeat this command)\n"},
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
		"evaluator-unavailable":               "the landing check crashed or gave no answer",
		"path-unclassified":                   "the change has files no landing rule covers yet",
		"ledger-path-not-goal-verb":           "the change edits goal files, which change only through goal commands",
		"runtime-path-refused":                "the change includes files the running system writes",
		"exact-revert-record-refused":         "a revert may not delete or shorten records",
		"goal-item-not-held":                  "goal G is not claimed by this session",
		"goal-revision-moved":                 "goal G was claimed again after this work started",
		"goal-binding-missing":                "this change names no goal, and landings here need one",
		"goal-binding-mismatch":               "this work was started for another goal than G",
		"record-not-owned":                    "the change edits a record another goal owns",
		"register-carriage-policy-unreadable": "the landing rules on this branch can't be read",
		"register-carriage-not-append-only":   "the change rewrites or deletes lines of an append-only record",
		"something-else":                      "the landing check refused this change, so nothing was committed",
	}
	for code, text := range cases {
		t.Run(code, func(t *testing.T) {
			b := newBed(t)
			b.epoch = epochOf(4)
			b.observed = landing.Observation{Mode: "refuse", RefusesAgent: true, Code: code, Provenance: "none change=x",
				VerdictTrailer: "would-refuse code=" + code, Refusal: "detail of " + code}
			b.git.on("diff --cached --name-only -z --", func(GitCall) GitResult { return ok("a b.go\x00") })
			b.expect(b.commit(CommitRequest{OwnerLineage: "L", Chain: "j1"}), 1, text, "verdict: would-refuse code="+code, "staged paths:\n  a\\ b.go\n", "an agent's change also lands when")
			if len(b.git.called("commit")) != 0 {
				t.Fatal("a refused agent commit was recorded")
			}
		})
	}
	b := newBed(t)
	b.epoch = epochOf(4)
	b.observed = landing.Observation{Mode: "refuse", RefusesAgent: true, Code: "attested-x", Provenance: "p", VerdictTrailer: "would-refuse code=attested-x"}
	b.expect(b.commit(CommitRequest{OwnerLineage: "L", Attested: "c1"}), 3, "verdict: would-refuse code=attested-x")
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
	b.expect(b.commit(CommitRequest{OwnerLineage: "L"}), 1, "the landing check crashed or gave no answer")
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
		}, "the commit was undone: it recorded other files than the ones checked"},
		{"machine", func(b *bed) {
			b.git.on("log -1 --format=%B", func(GitCall) GitResult { return ok(b.git.message + "\nMachine: m2+x\n") })
		}, "expected exactly one Machine trailer, found 2"},
		{"goal item", func(b *bed) {
			b.git.on("log -1 --format=%B", func(GitCall) GitResult { return ok(b.git.message + "\nGoal-Item: g1\n") })
		}, "the commit was undone: its message didn't end up with exactly one Goal-Item: line"},
		{"stray carried", func(b *bed) {
			b.git.on("log -1 --format=%B", func(GitCall) GitResult { return ok(b.git.message + "\nCarry: op\n") })
		}, "carried-trailer postcondition: Carry must be absent"},
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
		{"detached", func(b *bed) { b.git.branch = "" }, "committed, but not pushed: this checkout isn't on a branch"},
		{"fetch", func(b *bed) { b.git.on("fetch", func(GitCall) GitResult { return failed(1, "") }) }, "committed, but not pushed: origin couldn't be reached"},
		{"push", func(b *bed) { b.git.on("push", func(GitCall) GitResult { return failed(1, "") }) }, "committed, but origin refused the push"},
		{"transport", func(b *bed) {
			b.git.on("remote", func(GitCall) GitResult { return ok("transport\n") })
			b.owners.SyncTransport = func(string, string, io.Writer, io.Writer) int { return 1 }
		}, "pushed to origin, but the transport copy couldn't be updated"},
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
	b.expect(b.commit(request), 3, "the deciding observation does not bind the requested word, refusal, and ledger")

	b = newBed(t)
	b.observed = carriedObservation
	b.owners.Live = func() Judge {
		judge := live()
		judge.VerifyCarried = func(string, string, string) ([]byte, int) { return []byte("not json"), 1 }
		return judge
	}
	b.expect(b.commit(request), 3, "the test results for this change can't be read", "test verify gave no structured delivery result")
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
	b.expect(b.commit(request), 3, "neither this metasystem nor one built from HEAD could judge the landing")
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

// laneProofBed is a seat whose only retained proof covers the admission
// groups: the full delivery plan is unproved locally.
func laneProofBed(t *testing.T, staged string) *bed {
	b := newBed(t)
	b.owners.Verify = func(request VerifyRequest, _, stderr io.Writer) int {
		b.log.add("verify scope=%s", request.Scope)
		if request.Scope == ProofFull {
			fmt.Fprintln(stderr, "missing required proof")
			return 1
		}
		return 0
	}
	b.owners.SelectCode = func(paths []string, _ string) ([]string, error) {
		var code []string
		for _, path := range paths {
			if strings.HasSuffix(path, ".go") {
				code = append(code, path)
			}
		}
		return code, nil
	}
	b.git.on("diff --cached --no-renames --name-only -z --", func(GitCall) GitResult { return ok(staged) })
	return b
}

// A change joining the landing lane is proved by the lane: a records-only
// change needs no local proof, a code change only its admission groups;
// without a lane the full delivery proof is still required. Each refusal
// names which of the three rules it applied.
func TestCommitLocalProofFollowsTheLane(t *testing.T) {
	t.Parallel()
	records := "memory/receipts.log\x00records/narrator-digest.log\x00"
	b := laneProofBed(t, records)
	b.expect(b.commit(CommitRequest{LaneJoin: true}), 0)
	if !b.log.has("verify scope=none") || len(b.git.called("commit")) != 1 {
		t.Fatalf("records-only lane change: %v", b.log.calls)
	}

	code := "memory/receipts.log\x00cmd/metasystem/main.go\x00"
	b = laneProofBed(t, code)
	b.expect(b.commit(CommitRequest{LaneJoin: true}), 0)
	if !b.log.has("verify scope=admission") || len(b.git.called("commit")) != 1 {
		t.Fatalf("code lane change: %v", b.log.calls)
	}

	b = laneProofBed(t, records)
	b.expect(b.commit(CommitRequest{}), 1, "the change's tests haven't passed on this checkout yet",
		"without a landing lane the whole test plan must pass on this checkout")
	if !b.log.has("verify scope=full") || len(b.git.called("commit")) != 0 {
		t.Fatalf("no-lane change: %v", b.log.calls)
	}

	for _, c := range []struct {
		staged, scope, text string
	}{
		{records, "none", "files on disk differ from the staged change, so its test results don't apply to it"},
		{code, "admission", "files on disk differ from the staged change, so its test results don't apply to it"},
	} {
		b = laneProofBed(t, c.staged)
		b.owners.Verify = func(request VerifyRequest, _, stderr io.Writer) int {
			b.log.add("verify scope=%s", request.Scope)
			fmt.Fprintln(stderr, "delivery candidate differs from relevant working-tree inputs")
			return 1
		}
		b.expect(b.commit(CommitRequest{LaneJoin: true}), 1, c.text)
		if !b.log.has("verify scope="+c.scope) || len(b.git.called("commit")) != 0 {
			t.Fatalf("%s refusal: %v", c.scope, b.log.calls)
		}
	}
}

// The landing driver's commit-only step is the lane join: its commit
// boundary applies the lane's proof rule.
func TestCommitOnlyLandingJoinsTheLaneAtTheBoundary(t *testing.T) {
	t.Parallel()
	if !(&driver{request: LandRequest{CommitOnly: true}}).commitRequest().LaneJoin {
		t.Fatal("a commit-only landing did not ask the boundary for the lane's proof rule")
	}
	if (&driver{request: LandRequest{}}).commitRequest().LaneJoin {
		t.Fatal("a landing without the lane asked for the lane's proof rule")
	}
}
