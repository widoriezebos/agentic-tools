package landpath

// Ports of scripts/agents/static-reproof-fixtures.sh and of the commit.sh and
// land.sh legs of scripts/agents/path-class-fixtures.sh that exercise
// behavior the Go commit boundary keeps: the unbound-input closure, staged
// symlinks and hidden entries, the trailer postconditions, the evaluator's
// fallback, and forwarding the landing's declarations to the evaluator. The
// retired contract-off branch (go-gate --fast, the proof-built evaluator,
// internal audit metasystem) has no port.

import (
	"io"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
)

// reproofObserve replaces the live judge's observation and records every
// request it is asked.
func reproofObserve(b *bed, observe func(ObserveRequest) (landing.Observation, int)) *[]ObserveRequest {
	var requests []ObserveRequest
	live := b.owners.Live
	b.owners.Live = func() Judge {
		judge := live()
		judge.Observe = func(request ObserveRequest) (landing.Observation, int) {
			requests = append(requests, request)
			return observe(request)
		}
		return judge
	}
	return &requests
}

// TestReproofHumanStampsTheEvaluatorsRefusingVerdict ports the human legs of
// TestRealCommitWrapperStampsParseableObservation (missing-declaration, and
// the human chain-open landing): a human commit is sovereign, lands, and
// stamps the evaluator's provenance and would-refuse verdict byte for byte,
// without a Goal-Revision because the verdict is not a pass.
func TestReproofHumanStampsTheEvaluatorsRefusingVerdict(t *testing.T) {
	t.Parallel()
	provenance := "none change=" + strings.Repeat("0", 64)
	for _, code := range []string{"missing-declaration", "chain-open"} {
		t.Run(code, func(t *testing.T) {
			b := newBed(t)
			b.observed = landing.Observation{Mode: "refuse", RefusesAgent: true, Code: code, Provenance: provenance,
				VerdictTrailer: "would-refuse code=" + code, GoalRevision: 1}
			b.expect(b.commit(CommitRequest{HeldEpoch: "human", Goal: "fx", GoalSet: true, Chain: "fixture-chain", OwnerLineage: "human"}), 0)
			for _, line := range []string{"Landing-Provenance: " + provenance, "Landing-Provenance-Verdict: would-refuse code=" + code, "Goal-Item: fx"} {
				if countExact(b.git.message, line) != 1 {
					t.Fatalf("message lacks %q:\n%s", line, b.git.message)
				}
			}
			if strings.Contains(b.git.message, "Goal-Revision:") {
				t.Fatalf("a would-refuse verdict stamped a Goal-Revision:\n%s", b.git.message)
			}
		})
	}
}

// TestReproofAgentLandsAnObserveModeException ports the mechanical-chain
// leg: an observe-mode would-refuse the evaluator does not hold against
// agents (chain-not-design-bearing) lands an agent commit with its verdict.
func TestReproofAgentLandsAnObserveModeException(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.epoch = epochOf(1)
	b.observed = landing.Observation{Mode: "observe", Code: "chain-not-design-bearing", Provenance: "chain=mechanical-chain change=abc",
		VerdictTrailer: "would-refuse code=chain-not-design-bearing", GoalRevision: 1}
	b.expect(b.commit(CommitRequest{Goal: "fx", GoalSet: true, Chain: "mechanical-chain", OwnerLineage: "human"}), 0)
	if countExact(b.git.message, "Landing-Provenance-Verdict: would-refuse code=chain-not-design-bearing") != 1 ||
		strings.Contains(b.git.message, "Goal-Revision:") {
		t.Fatalf("mechanical exception message:\n%s", b.git.message)
	}
}

// TestReproofForwardsDeclarationsToTheEvaluator ports the agent legs whose
// verdicts the evaluator decides (orphaned --revert-of, conflicting
// declarations, chain-open, the behavior floor): the boundary forwards every
// declaration, the goal and the machine+lineage actor unchanged, and refuses
// an agent with the evaluator's code, the staged path and the lawful exits.
func TestReproofForwardsDeclarationsToTheEvaluator(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		request CommitRequest
		code    string
	}{
		{"orphaned revert", CommitRequest{RevertOf: strings.Repeat("0", 40)}, "conflicting-declarations"},
		{"chain open", CommitRequest{Chain: "fixture-chain"}, "chain-open"},
		{"conflict", CommitRequest{Chain: "fixture-chain", DirectFix: "exact-revert"}, "conflicting-declarations"},
		{"behavior floor", CommitRequest{DirectFix: "register-carriage"}, "direct-fix-floor-refused"},
		{"missing declaration", CommitRequest{}, "missing-declaration"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b := newBed(t)
			b.epoch = epochOf(1)
			b.git.on("diff --cached --name-only -z --", func(GitCall) GitResult { return ok("README\x00") })
			requests := reproofObserve(b, func(ObserveRequest) (landing.Observation, int) {
				return landing.Observation{Mode: "refuse", RefusesAgent: true, Code: c.code, Provenance: "none change=x",
					VerdictTrailer: "would-refuse code=" + c.code}, 0
			})
			request := c.request
			request.Goal, request.GoalSet, request.OwnerLineage = "fx", true, "human"
			b.expect(b.commit(request), 1, "would-refuse code="+c.code, "staged paths:\n  README\n", "--chain <root-job-id>", "fix the Change-Class classification")
			if len(*requests) != 1 {
				t.Fatalf("observed %d times", len(*requests))
			}
			got := (*requests)[0]
			if got.Chain != c.request.Chain || got.RevertOf != c.request.RevertOf || got.DirectFix != c.request.DirectFix ||
				got.Goal != "fx" || got.Actor != "m1+human" || got.Tree != "t1" || got.Judge != "" {
				t.Fatalf("observe request %+v", got)
			}
			if len(b.git.called("commit")) != 0 {
				t.Fatal("a refused agent commit was recorded")
			}
		})
	}
}

// TestReproofVendoredInstallationObservesItsSubtree ports the vendored leg:
// an installation below the repository top level is observed at its own
// subtree, and an unclassified path's refusal carries the evaluator's
// base-manifest detail.
func TestReproofVendoredInstallationObservesItsSubtree(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.epoch = epochOf(1)
	b.git.prefix = "metasystem/"
	b.git.on("rev-parse t1:metasystem", func(GitCall) GitResult { return ok("sub1\n") })
	b.git.on("diff --cached --name-only -z --", func(GitCall) GitResult { return ok("README\x00") })
	detail := "path README has no class in the engine's path-class policy (internal/pathclass/path-classes.txt); no classified ancestor"
	requests := reproofObserve(b, func(ObserveRequest) (landing.Observation, int) {
		return landing.Observation{Mode: "refuse", RefusesAgent: true, Code: "path-unclassified", Provenance: "none change=x",
			VerdictTrailer: "would-refuse code=path-unclassified", Refusal: detail}, 0
	})
	b.expect(b.commit(CommitRequest{Goal: "fx", GoalSet: true, DirectFix: "register-carriage", OwnerLineage: "human"}), 1,
		"would-refuse code=path-unclassified", detail,
		"classify every named path in the engine's path-class policy (internal/pathclass/path-classes.txt, engine source), rebuild the engine, then retry")
	if len(*requests) != 1 || (*requests)[0].Tree != "sub1" {
		t.Fatalf("vendored observation %+v", *requests)
	}
}

// TestReproofEvaluatorFailureRefusesAgentAndStampsHuman ports leg 3a: an
// evaluator that exits nonzero decides nothing, even when it printed a
// complete observation; the agent is refused as evaluator-unavailable with
// the staged path, the repair and the exits, and a human lands the same tree
// with the fallback stamp.
func TestReproofEvaluatorFailureRefusesAgentAndStampsHuman(t *testing.T) {
	t.Parallel()
	failing := func(ObserveRequest) (landing.Observation, int) { return passingObservation(), 72 }
	b := newBed(t)
	b.epoch = epochOf(1)
	b.git.on("diff --cached --name-only -z --", func(GitCall) GitResult { return ok("README\x00") })
	reproofObserve(b, failing)
	b.expect(b.commit(CommitRequest{OwnerLineage: "fixture-lineage"}), 1, "landing evaluator failed",
		"would-refuse code=evaluator-unavailable", "  README\n", "restore or rebuild the proof-built landing evaluator",
		"--chain <root-job-id>", "fix the Change-Class classification")
	if len(b.git.called("commit")) != 0 {
		t.Fatal("evaluator failure created an agent commit")
	}

	b = newBed(t)
	reproofObserve(b, failing)
	b.expect(b.commit(CommitRequest{HeldEpoch: "human"}), 0)
	if countExact(b.git.message, "Landing-Provenance-Verdict: would-refuse code=evaluator-unavailable") != 1 ||
		countExact(b.git.message, "Landing-Provenance: none change=unknown") != 1 {
		t.Fatalf("human fallback stamp:\n%s", b.git.message)
	}
}

// TestReproofUnboundInputsAreNamed ports legs 4, 6 and 9: a diverged, an
// untracked and an ignored projected input each refuse naming the path, and
// paths the LANDING projection drops do not.
func TestReproofUnboundInputsAreNamed(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name, listing, path string
	}{
		{"diverged", "diff --no-renames --name-only -z --", "internal/red/red.go"},
		{"untracked", "ls-files --others --exclude-standard --full-name -z", "internal/red/stray.go"},
		{"ignored", "ls-files --others -i --exclude-standard --full-name -z", "internal/red/generated.go"},
	} {
		t.Run(c.name, func(t *testing.T) {
			b := newBed(t)
			b.git.on(c.listing, func(GitCall) GitResult { return ok("artifacts/agents/x.log\x00" + c.path + "\x00") })
			b.owners.SelectLanding = func(paths []string, _ string) ([]string, error) {
				var kept []string
				for _, path := range paths {
					if !strings.HasPrefix(path, "artifacts/") {
						kept = append(kept, path)
					}
				}
				return kept, nil
			}
			b.expect(b.commit(CommitRequest{HeldEpoch: "human"}), 1, "not what the commit would record", "  "+c.path+"\n")
			if strings.Contains(b.stderr.String(), "artifacts/agents/x.log") || len(b.git.called("commit")) != 0 {
				t.Fatalf("projection or commit: %s %v", b.stderr.String(), b.git.calls)
			}
		})
	}
	b := newBed(t)
	b.git.on("ls-files --others --exclude-standard --full-name -z", func(GitCall) GitResult { return ok("artifacts/agents/x.log\x00") })
	b.owners.SelectLanding = func([]string, string) ([]string, error) { return nil, nil }
	b.expect(b.commit(CommitRequest{HeldEpoch: "human"}), 0)
}

// TestReproofStagedDirectorySymlinksRefuse ports leg 11: a whole projected
// directory replaced by one staged symlink (docs, internal/gaterun) refuses
// like a critical leaf, in a vendored installation too; a symlink outside
// the proof inputs does not.
func TestReproofStagedDirectorySymlinksRefuse(t *testing.T) {
	t.Parallel()
	for _, prefix := range []string{"", "metasystem/"} {
		b := newBed(t)
		b.git.prefix = prefix
		b.git.on("rev-parse t1:metasystem", func(GitCall) GitResult { return ok("sub1\n") })
		b.git.on("ls-files -s -z", func(GitCall) GitResult {
			return ok("120000 abc 0\t" + prefix + "docs\x00120000 abc 0\t" + prefix + "internal/gaterun\x00120000 abc 0\t" + prefix + "skills/linked\x00")
		})
		b.expect(b.commit(CommitRequest{HeldEpoch: "human"}), 1, "a critical proof input is a symlink",
			"  "+prefix+"docs\n", "  "+prefix+"internal/gaterun\n")
		if strings.Contains(b.stderr.String(), "skills/linked") {
			t.Fatalf("a non-critical symlink was refused: %s", b.stderr.String())
		}
	}
	b := newBed(t)
	b.git.on("ls-files -s -z", func(GitCall) GitResult { return ok("120000 abc 0\tskills/linked\x00100644 abc 0\tREADME\x00") })
	b.expect(b.commit(CommitRequest{HeldEpoch: "human"}), 0)
}

// TestReproofAssumeUnchangedEntriesRefuse: an assume-unchanged entry (a
// lowercase ls-files -v tag) hides divergence exactly like skip-worktree.
func TestReproofAssumeUnchangedEntriesRefuse(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.git.on("ls-files -v -z", func(GitCall) GitResult { return ok("h internal/a.go\x00H README\x00") })
	b.expect(b.commit(CommitRequest{HeldEpoch: "human"}), 1, "assume-unchanged or skip-worktree entries hide proof inputs", "  internal/a.go\n")
	if strings.Contains(b.stderr.String(), "README") {
		t.Fatalf("a plain tracked entry was named: %s", b.stderr.String())
	}
}

// TestReproofGoalDeclarationRefusals ports the exit-2 legs of
// TestCommitWrapperStampsGoalItemTrailer the Go boundary still receives: an
// empty goal and a commit message read from standard input.
func TestReproofGoalDeclarationRefusals(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.epoch = epochOf(1)
	b.expect(b.commit(CommitRequest{Goal: "", GoalSet: true, DirectFix: "register-carriage", OwnerLineage: "L"}), 2,
		"commit refused: --goal must be a lowercase kebab identifier of at most 100 characters")
	b = newBed(t)
	b.epoch = epochOf(1)
	b.expect(b.commit(CommitRequest{Goal: "fx", GoalSet: true, DirectFix: "register-carriage", OwnerLineage: "L", MessageFile: "-"}), 2,
		"commit refused: -F - is an unscannable commit message source")
	if len(b.git.called("commit")) != 0 || b.log.has("token") {
		t.Fatalf("a refused declaration reached an effect: %v", b.log.calls)
	}
}

// TestReproofChangedGoalItemRollsBack ports the hook-changed leg: a
// commit-msg hook that rewrites the stamped Goal-Item leaves one Goal-Item
// that is not byte-exact, and the commit is rolled back softly.
func TestReproofChangedGoalItemRollsBack(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.epoch = epochOf(1)
	b.git.on("log -1 --format=%B", func(GitCall) GitResult {
		return ok(strings.ReplaceAll(b.git.message, "Goal-Item: fx", "Goal-Item: victim") + "\n")
	})
	b.expect(b.commit(CommitRequest{Goal: "fx", GoalSet: true, DirectFix: "register-carriage", OwnerLineage: "L"}), 1,
		"final commit message did not contain exactly one byte-exact Goal-Item")
	if len(b.git.called("reset --soft h0")) != 1 {
		t.Fatalf("not rolled back softly: %v", b.git.calls)
	}
}

// TestReproofLandForwardsGoalToEvaluator ports TestLandForwardsGoalToEvaluator:
// the landing carries --goal and --direct-fix through the commit boundary to
// the evaluator under the seat's lineage, stamping Goal-Item and the pass;
// a foreign lineage's goal-item-not-held refusal leaves no commit.
func TestReproofLandForwardsGoalToEvaluator(t *testing.T) {
	t.Parallel()
	land := func(b *bed, lineage string) int {
		b.git.stagedEmpty = true
		b.git.on("add --", func(GitCall) GitResult { b.git.stagedEmpty = false; return ok("") })
		return b.land(LandRequest{Pathspecs: []string{"plans/fx-note.md"}, Goal: "fx", GoalSet: true, DirectFix: "register-carriage",
			SkipTransport: true, OwnerLineage: lineage})
	}
	b := newBed(t)
	b.epoch = epochOf(1)
	requests := reproofObserve(b, func(request ObserveRequest) (landing.Observation, int) {
		return landing.Observation{Mode: "observe", Code: "register-carriage", Provenance: "direct-fix class=register-carriage change=abc",
			VerdictTrailer: "pass bar=b", GoalRevision: 1}, 0
	})
	b.expect(land(b, "L"), 0)
	if len(*requests) != 1 || (*requests)[0].Goal != "fx" || (*requests)[0].DirectFix != "register-carriage" || (*requests)[0].Actor != "m1+L" {
		t.Fatalf("observe requests %+v", *requests)
	}
	for _, line := range []string{"Goal-Item: fx", "Landing-Provenance-Verdict: pass bar=b"} {
		if countExact(b.git.message, line) != 1 {
			t.Fatalf("landed message lacks %q:\n%s", line, b.git.message)
		}
	}

	b = newBed(t)
	b.epoch = epochOf(1)
	reproofObserve(b, func(request ObserveRequest) (landing.Observation, int) {
		if request.Actor != "m1+other" {
			t.Fatalf("foreign actor %q", request.Actor)
		}
		return landing.Observation{Mode: "refuse", RefusesAgent: true, Code: "goal-item-not-held", Provenance: "none change=x",
			VerdictTrailer: "would-refuse code=goal-item-not-held"}, 0
	})
	b.owners.Held = func(string, string, string, string, string, io.Writer, io.Writer) int {
		t.Fatal("a refused landing reached held")
		return 1
	}
	b.expect(land(b, "other"), 1, "goal-item-not-held")
	if len(b.git.called("commit")) != 0 {
		t.Fatal("a foreign lineage committed")
	}
}
