package validate

import (
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/pathclass"
)

// Ports of scripts/agents/conformance-fixtures.sh. The instruction and
// testing-contract checks read the committed files; the conformance cases the
// in-package fixtures did not already assert are driven on the owner with Git
// stubbed (the raw fixture) or without Git at all, except the ignored-local
// case, whose claim is Git's own ignore rules.

func conformanceBedSource(t *testing.T, relative string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(relative)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestConformanceBedInstructionsCarryBoundedRead(t *testing.T) {
	t.Parallel()
	const boundedRead = "equest the smallest section you need, read a large file through a bounded view, and open a reference only through its path."
	for _, instruction := range []string{
		"internal/protocol/templates/brief.md", "internal/protocol/roles/design-critic.md",
		"internal/protocol/roles/code-critic.md", "internal/protocol/roles/implementer.md", "docs/orchestration.md",
	} {
		if !strings.Contains(conformanceBedSource(t, instruction), boundedRead) {
			t.Errorf("instruction file %s does not carry the bounded-read sentence", instruction)
		}
	}
}

func TestConformanceBedBriefReservesReceiptsForTheSeat(t *testing.T) {
	t.Parallel()
	const line = "Leave `metasystem/memory/receipts.log` unchanged; the seat writes the receipt at landing."
	if !slices.Contains(strings.Split(conformanceBedSource(t, "internal/protocol/templates/brief.md"), "\n"), line) {
		t.Fatal("builder brief template does not reserve receipts for the seat")
	}
}

func TestConformanceBedInstructionsHandOffOverTheBound(t *testing.T) {
	t.Parallel()
	if !strings.Contains(conformanceBedSource(t, "internal/protocol/roles/steward-continuation.md"), "metasystem session handoff --root <installation> --verify") {
		t.Error("steward-continuation role does not verify a named context handoff")
	}
	if !strings.Contains(conformanceBedSource(t, "docs/orchestration.md"), "metasystem session handoff --root") {
		t.Error("orchestration instructions do not hand off a context over the bound")
	}
}

// conformanceBedGroup is the testing.json group that runs the instruction
// checks above; the context-budget surface must select it and a change to
// docs/orchestration.md must admit it. The group switch that retires
// conformance-fixtures.sh renames it to the group that runs TestConformanceBed*.
const conformanceBedGroup = "conformance-bed-standard"

func TestConformanceBedContextTestingContract(t *testing.T) {
	t.Parallel()
	var contract struct {
		Surfaces []struct {
			ID       string   `json:"id"`
			Standard []string `json:"standard"`
		} `json:"surfaces"`
		Groups []struct {
			ID       string          `json:"id"`
			Inputs   []string        `json:"inputs"`
			Packages []string        `json:"packages"`
			Tests    json.RawMessage `json:"tests"`
		} `json:"groups"`
	}
	if err := json.Unmarshal([]byte(conformanceBedSource(t, "testing.json")), &contract); err != nil {
		t.Fatal(err)
	}
	const orchestration = "metasystem/docs/orchestration.md"
	groups := map[string]int{}
	for index, group := range contract.Groups {
		groups[group.ID] = index
	}
	contextIndex, ok := groups["context-standard"]
	if !ok {
		t.Fatal("testing.json has no context-standard group")
	}
	context := contract.Groups[contextIndex]
	if !slices.Contains(context.Inputs, orchestration) {
		t.Error("context-standard inputs omit docs/orchestration.md")
	}
	if !slices.Contains(context.Packages, "internal/goal") {
		t.Error("context-standard packages omit internal/goal")
	}
	var tests []string
	if err := json.Unmarshal(context.Tests, &tests); err != nil {
		t.Fatalf("context-standard tests are not a list: %v", err)
	}
	for _, name := range []string{
		"TestHandoffWritesAVerifiedStateFile", "TestHandoffManifestPreservesOverflow", "TestHandoffRefusals",
		"TestHandoffRetriesNonceCollisionWithoutOverwriting", "TestHandoffCleansDirectoryPublicationFailure",
		"TestHandoffPublicationIsExclusiveAndReverified", "TestLiveHandoffUsesNoExpiryClock",
		"TestLiveHandoffReadDoesNotWaitOrWrite", "TestHandoffAcceptsOnlyTheActiveContinuation",
		"TestHandoffIgnoresConcurrentUnrelatedRecords", "TestSecondHandoffSupersedesTheFirst",
		"TestConcurrentHandoffsDoNotCross", "TestCancelHandoffReportsItsExactPartialOutcome",
		"TestVerifyHandoffStateUsesExactLifecycleRecord", "TestHandoffRejectsInvalidLiveAuthority",
		"TestContextPruneKeepsLiveHandoffs", "TestContextPruneSerializesWithConsumption",
		"TestContextPruneStopsOnUsageError", "TestContextPruneDefaultAgeComposesWithUsageFloor",
		"TestContextPruneRefusesRedirectedHandoffTrees", "TestContextPruneRechecksBeforeRemoval",
		"TestContextPruneReportsRemovalBeforeSyncFailure", "TestContextPruneRetainsDamageAndRefusesBadBounds",
		"TestHandoffAndDiagnosticsHaveSeparateLifetimes", "TestTurnVerdictAllowsTheStopUnderARecordedHandoff",
		"TestHandoffAllowanceCarriesFrozenFacts", "TestContextHandoffVerb", "TestContextVerifyAndCancel",
		"TestContextVerbUsage",
	} {
		if !slices.Contains(tests, name) {
			t.Errorf("context-standard tests omit %s", name)
		}
	}
	var budget []string
	for _, surface := range contract.Surfaces {
		if surface.ID == "context-budget" {
			budget = surface.Standard
		}
	}
	for _, group := range []string{"context-standard", "context-foundations-standard", "supervision-bed-standard", conformanceBedGroup} {
		if !slices.Contains(budget, group) {
			t.Errorf("context-budget standard selection omits %s", group)
		}
	}
	conformanceIndex, ok := groups[conformanceBedGroup]
	if !ok || !slices.Contains(contract.Groups[conformanceIndex].Inputs, orchestration) {
		t.Errorf("%s inputs omit docs/orchestration.md", conformanceBedGroup)
	}
}

// conformanceBedChain writes the implementer chain impl (round 1) and its
// follow-up impl-r2 (round 2, whose prompt enumerates F-9) and returns the
// run a merge critique reads.
func conformanceBedChain(t *testing.T, implementerModel string) (*conformanceFixture, *conformanceRun) {
	t.Helper()
	f := &conformanceFixture{t: t, controller: t.TempDir()}
	implementer := map[string]any{
		"jobId": "impl", "role": "implementer", "round": 1, "parentJob": nil,
		"status": "completed", "effectiveModel": implementerModel,
	}
	f.writeJSON("artifacts/agents/jobs/impl.json", implementer)
	f.writeFollowUp()
	return f, &conformanceRun{root: f.controller, rootJob: "impl", record: implementer}
}

// conformanceBedCritic writes the closed code-critic chain critic reviewing
// impl on tree with the given material finding and exhaustion items.
func conformanceBedCritic(f *conformanceFixture, material string, exhaustions []any, model string) {
	f.writeJSON("artifacts/agents/jobs/critic.json", map[string]any{
		"jobId": "critic", "role": "code-critic", "round": 1, "parentJob": nil,
		"reviews": "impl", "status": "completed", "effectiveModel": model,
		"chainClosed": true, "critiqueExhaustions": exhaustions,
	})
	findings := []any{}
	if material != "" {
		findings = []any{map[string]any{"id": material, "severity": "high", "material": true,
			"claim": "fixture material finding remains open", "evidence": "fixture evidence"}}
	}
	f.writeJSON("artifacts/agents/critic/rounds/1/return.json", map[string]any{
		"jobId": "critic", "round": 1, "reviewedTree": "tree",
		"findings": findings, "verdictMaterialCount": len(findings),
	})
}

func conformanceBedMerge(t *testing.T, r *conformanceRun, independence string, wantCode int, wants ...string) {
	t.Helper()
	r.out, r.errs = nil, nil
	out, errs, code := r.mergeCritique("", "tree", "fake", independence)
	joined := strings.Join(append(append([]string{}, out...), errs...), "\n")
	if code != wantCode {
		t.Fatalf("merge critique code=%d, want %d:\n%s", code, wantCode, joined)
	}
	for _, want := range wants {
		if !strings.Contains(joined, want) {
			t.Fatalf("merge critique lacks %q:\n%s", want, joined)
		}
	}
}

func TestConformanceBedDispositionedMaterialStillRefuses(t *testing.T) {
	t.Parallel()
	f, r := conformanceBedChain(t, "shared-model")
	conformanceBedCritic(f, "F-7", []any{}, "critic-model")
	dispositions := filepath.Join(f.controller, "artifacts", "agents", "critic", "rounds", "1", "dispositions.md")
	if err := os.WriteFile(dispositions, []byte("| Finding id | Disposition | Reasoning and evidence | Amendment |\n| --- | --- | --- | --- |\n| F-7 | accepted | fixture disposition | fixture amendment |\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	conformanceBedMerge(t, r, "", 1, "still has material findings despite any dispositions: F-7")
}

func TestConformanceBedExhaustionSuccessorRefusals(t *testing.T) {
	t.Parallel()
	t.Run("successor prompt omits the open finding", func(t *testing.T) {
		t.Parallel()
		f, r := conformanceBedChain(t, "shared-model")
		conformanceBedCritic(f, "F-10", []any{map[string]any{"round": 1, "openFindingIds": []any{"F-10"}, "successorJobId": "impl-r2"}}, "critic-model")
		conformanceBedMerge(t, r, "", 1, "prompt does not enumerate open findings: F-10")
	})
	t.Run("successor is another critic", func(t *testing.T) {
		t.Parallel()
		f, r := conformanceBedChain(t, "shared-model")
		items := []any{map[string]any{"round": 1, "openFindingIds": []any{"F-9"}, "successorJobId": "other-critic"}}
		conformanceBedCritic(f, "F-9", items, "critic-model")
		f.writeJSON("artifacts/agents/jobs/other-critic.json", map[string]any{
			"jobId": "other-critic", "role": "code-critic", "round": 1, "parentJob": nil,
			"reviews": "unrelated", "status": "completed", "effectiveModel": "critic-model",
			"chainClosed": true, "critiqueExhaustions": items,
		})
		conformanceBedMerge(t, r, "", 1, "is not an implementer follow-up in the reviewed implementation chain")
	})
}

func TestConformanceBedSameModelNamesBothJobsAndRemedies(t *testing.T) {
	t.Parallel()
	f, r := conformanceBedChain(t, "shared-model")
	conformanceBedCritic(f, "", []any{}, "shared-model")
	conformanceBedMerge(t, r, "", 1,
		"implementer job 'impl' uses effective model 'shared-model'",
		"code-critic chain 'critic' uses effective model 'shared-model'",
		"dispatch a critic on a different model", "declare independence=session-only")
	conformanceBedMerge(t, r, "session-only", 0, "independence=session-only recorded in gate evidence")
}

// The cumulative boundary: each round declares only its own files, and the
// follow-up's review compares the changed paths with the union through its
// round.
func TestConformanceBedCumulativeBoundaryUnionsRounds(t *testing.T) {
	t.Parallel()
	f := &conformanceFixture{t: t, controller: t.TempDir()}
	f.writeJSON("artifacts/agents/jobs/impl.json", map[string]any{"jobId": "impl", "role": "implementer", "round": 1, "parentJob": nil})
	f.writeJSON("artifacts/agents/jobs/impl-r2.json", map[string]any{"jobId": "impl-r2", "role": "implementer", "round": 2, "parentJob": "impl"})
	f.writeJSON("artifacts/agents/impl/rounds/1/return.json", map[string]any{"jobId": "impl", "round": 1, "diffBoundary": []any{"source.txt"}})
	f.writeJSON("artifacts/agents/impl/rounds/2/return.json", map[string]any{"jobId": "impl-r2", "round": 2, "diffBoundary": []any{"docs/note.md"}})
	r := &conformanceRun{root: f.controller, job: "impl-r2", rootJob: "impl", roundText: "2", workspace: t.TempDir()}

	violations := r.cumulativeBoundaryViolations([]string{"docs/note.md", "extra.txt", "source.txt"})
	joined := strings.Join(violations, "\n")
	if len(violations) != 1 || !strings.Contains(joined, "some implementation round must declare every changed path") ||
		!strings.Contains(joined, "'extra.txt'") || strings.Contains(joined, "source.txt") || strings.Contains(joined, "docs/note.md") {
		t.Fatalf("cumulative boundary violations = %q", violations)
	}
	if violations := r.cumulativeBoundaryViolations([]string{"docs/note.md", "source.txt"}); len(violations) != 0 {
		t.Fatalf("the union of both rounds' declarations was refused: %q", violations)
	}
}

func TestConformanceBedWaiverRefusals(t *testing.T) {
	t.Parallel()
	manifest, err := pathclass.Parse([]byte(conformanceBedSource(t, "internal/pathclass/path-classes.txt")))
	if err != nil {
		t.Fatal(err)
	}
	waiver := map[string]any{"class": "prose-under-30"}
	t.Run("tracked script", func(t *testing.T) {
		t.Parallel()
		r := &conformanceRun{root: t.TempDir(), rootJob: "impl", workspace: t.TempDir()}
		resolution := manifest.Resolve(pathclass.Install, "scripts/tool.sh")
		resolution.Mode = pathclass.Adopted
		_, errs, code := r.mergeWaiver(waiver, []waiverPath{{projectPath: "scripts/tool.sh", resolution: resolution}}, "1\t0\tscripts/tool.sh\n")
		if code != 1 || !strings.Contains(strings.Join(errs, "\n"), "prose-under-30 touches a path that is never waivable: ['scripts/tool.sh']") {
			t.Fatalf("tracked script waiver: code=%d errs=%q", code, errs)
		}
	})
	t.Run("agent control plane", func(t *testing.T) {
		t.Parallel()
		workspace := t.TempDir()
		tamper := filepath.Join(workspace, "artifacts", "agents", "tamper")
		if err := os.MkdirAll(filepath.Dir(tamper), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(tamper, []byte("tamper\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		r := &conformanceRun{root: t.TempDir(), rootJob: "impl", workspace: workspace}
		prose := pathclass.Resolution{Class: pathclass.Outside, Mode: pathclass.Adopted}
		_, errs, code := r.mergeWaiver(waiver, []waiverPath{{projectPath: "docs/note.md", resolution: prose}}, "1\t0\tdocs/note.md\n")
		if code != 1 || !strings.Contains(strings.Join(errs, "\n"), "agent control plane contains delegate-created files") {
			t.Fatalf("control plane waiver: code=%d errs=%q", code, errs)
		}
	})
}

func conformanceBedDigest(t *testing.T, path string) [32]byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return sha256.Sum256(data)
}

// A merge, refused as stale or accepted, never rewrites the critic's review
// evidence.
func TestConformanceBedMergeLeavesReviewEvidenceUntouched(t *testing.T) {
	f := newRawConformanceFixture(t)
	appendFile(t, filepath.Join(f.worktree, "source.txt"), "changed\n")
	f.writeImplementer("", "source.txt")
	expectConformance(t, f, "review", 0, "reviewedTree=")
	round := filepath.Join(f.controller, "artifacts", "agents", "impl", "rounds", "1")
	diff, review := conformanceBedDigest(t, filepath.Join(round, "diff.patch")), conformanceBedDigest(t, filepath.Join(round, "review.json"))
	reviewedTree := f.reviewedTree()
	f.commitWorktree()
	unchanged := func(stage string) {
		t.Helper()
		if conformanceBedDigest(t, filepath.Join(round, "diff.patch")) != diff || conformanceBedDigest(t, filepath.Join(round, "review.json")) != review {
			t.Fatalf("the %s merge rewrote the stored review evidence", stage)
		}
	}
	f.writeCritic(strings.Repeat("0", 40), "", "", "critic-model")
	expectConformance(t, f, "merge", 1, "is stale")
	unchanged("refused")
	f.writeCritic(reviewedTree, "", "", "critic-model")
	expectConformance(t, f, "merge", 0, "merge critique accepted")
	unchanged("accepted")
}

// TestConformanceReviewGitAdapterExcludesIgnoredLocalFile keeps Git's own
// ignore rules as the claim: an ignored local configuration file is outside
// both the temporary review index and the declared boundary.
func TestConformanceReviewGitAdapterExcludesIgnoredLocalFile(t *testing.T) {
	t.Parallel()
	f := newConformanceFixture(t)
	appendFile(t, filepath.Join(f.worktree, "source.txt"), "changed\n")
	if err := os.WriteFile(filepath.Join(f.worktree, "local.conf"), []byte("role.code-critic.runtime=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	f.writeImplementer("", "source.txt")
	expectConformance(t, f, "review", 0, "reviewedTree=")
	patch, err := os.ReadFile(filepath.Join(f.controller, "artifacts", "agents", "impl", "rounds", "1", "diff.patch"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(patch), "local.conf") || !strings.Contains(string(patch), "source.txt") {
		t.Fatalf("the review artifact did not exclude the ignored local configuration file:\n%s", patch)
	}
}
