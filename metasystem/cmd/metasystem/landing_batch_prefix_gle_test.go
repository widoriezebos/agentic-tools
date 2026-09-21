package main

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestGLEBatchEveryPrefixHasApplicablePolicyProof(t *testing.T) {
	t.Parallel()
	units := []batch.Unit{
		{GoalID: "goal-a", SelectedGroups: []string{"first"}},
		{GoalID: "goal-b", SelectedGroups: []string{"second"}},
		{GoalID: "goal-c", SelectedGroups: []string{"third"}},
	}
	paths := map[string][]string{
		"tree-1": {"a.txt"},
		"tree-2": {"a.txt", "b.txt"},
		"tree-3": {"a.txt", "b.txt", "c.txt"},
	}
	planner := func(_, _, tree string, mode testpolicy.Mode, admitted []string) (testingPlanOutput, error) {
		contract := testpolicy.Contract{Surfaces: []testpolicy.Surface{
			{ID: "a", Paths: []string{"a.txt"}, Standard: []string{"first"}},
			{ID: "b", Paths: []string{"b.txt"}, Standard: []string{"second"}},
		}, Groups: []testpolicy.Group{{ID: "first"}, {ID: "second"}}}
		if tree == "tree-3" {
			contract.Surfaces = append(contract.Surfaces, testpolicy.Surface{ID: "c", Paths: []string{"c.txt"}, Standard: []string{"third"}})
			contract.Groups = append(contract.Groups, testpolicy.Group{ID: "third"})
		}
		selected, err := testpolicy.Select(contract, testpolicy.SelectionRequest{ChangedPaths: paths[tree], RequestedMode: mode, Purpose: testpolicy.PurposeDelivery,
			BatchRequirements: admitted})
		return testingPlanOutput{PolicyBaseCommit: "base-commit", CandidateTree: tree, ContractDigest: "candidate-policy-" + tree,
			BaseContractDigest: "base-policy", Plan: selected, Groups: contract.Groups}, err
	}
	for index, want := range [][]string{{"first"}, {"first", "second"}, {"first", "second", "third"}} {
		tree := []string{"tree-1", "tree-2", "tree-3"}[index]
		decision, err := planPrefixDecisionWith("", units[:index+1], tree, planner)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(decision.Groups, want) {
			t.Fatalf("prefix %d selected %v, want %v", index+1, decision.Groups, want)
		}
	}
}

func TestGLEBatchSupplementalFreshnessUsesTheSelectedPlan(t *testing.T) {
	t.Parallel()
	maxAge := int64(60_000)
	contract := testpolicy.Contract{Groups: []testpolicy.Group{
		{ID: "floor"}, {ID: "prerequisite"},
		{ID: "admitted", Requires: []string{"prerequisite"}, Freshness: "episode", FreshnessMaxAgeMS: &maxAge},
		{ID: "unselected-fresh", Freshness: "episode", FreshnessMaxAgeMS: &maxAge},
	}, Always: testpolicy.Always{Canary: []string{"floor"}}}
	planner := func(_, _, tree string, mode testpolicy.Mode, admitted []string) (testingPlanOutput, error) {
		selected, err := testpolicy.Select(contract, testpolicy.SelectionRequest{RequestedMode: mode, Purpose: testpolicy.PurposeDelivery,
			BatchRequirements: admitted})
		groups := []testpolicy.Group{}
		for _, group := range contract.Groups {
			if slices.Contains(selected.SelectedGroups, group.ID) {
				groups = append(groups, group)
			}
		}
		return testingPlanOutput{PolicyBaseCommit: "base", CandidateTree: tree, ContractDigest: "contract", Plan: selected, Groups: groups}, err
	}
	plain, err := planPrefixDecisionWith("", []batch.Unit{{GoalID: "goal-a"}}, "tree", planner)
	if err != nil || plain.FreshRequired {
		t.Fatalf("unselected fresh group required episode: decision=%+v err=%v", plain, err)
	}
	selected, err := planPrefixDecisionWith("", []batch.Unit{{GoalID: "goal-a", SelectedGroups: []string{"admitted"}}}, "tree", planner)
	if err != nil || !selected.FreshRequired || selected.FreshMaxAgeMS != maxAge ||
		!slices.Equal(selected.Groups, []string{"admitted", "floor", "prerequisite"}) {
		t.Fatalf("supplemental fresh decision=%+v err=%v", selected, err)
	}
}

func TestGLEBatchFencedTipAdmissionNamesMemberForReassembly(t *testing.T) {
	root, tree := batchPrefixReceiptTestRoot(t)
	stub := filepath.Join(t.TempDir(), "fenced-test-run")
	if err := testexec.WriteFile(stub, []byte("#!/bin/sh\nprintf 'CANDIDATE_GOAL_REFUSED state=fenced\\n' >&2\nexit 78\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	previous := batchTipProofExecutable
	batchTipProofExecutable = func() (string, error) { return stub, nil }
	t.Cleanup(func() { batchTipProofExecutable = previous })
	_, err := launchBatchTipProof(batchProofLaunch{Root: root, GoalID: "goal-c", Tree: tree, Mode: testpolicy.ModeAuto,
		ResultPath: filepath.Join(t.TempDir(), "result.json")})
	var refusal *batchProofAdmissionRefusal
	if !errors.As(err, &refusal) || refusal.kind != "fenced" {
		t.Fatalf("tip fence classified as %T %v", err, err)
	}
	if proofrun.ExitAdmissionRefused != 78 {
		t.Fatal("test stub exit no longer matches admission refusal")
	}
}

func TestGLEBatchRebasedPrefixTreesNamesEveryBoundary(t *testing.T) {
	t.Parallel()
	root, _ := batchPrefixReceiptTestRoot(t)
	base := runReceiptGit(t, root, "rev-parse", "HEAD")
	var expected []string
	for _, name := range []string{"a", "b-one", "b-two", "c"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
		runReceiptGit(t, root, "add", name)
		runReceiptGit(t, root, "commit", "-qm", name)
		expected = append(expected, runReceiptGit(t, root, "rev-parse", "HEAD^{tree}"))
	}
	tip := runReceiptGit(t, root, "rev-parse", "HEAD")
	units := []batch.Unit{{GoalID: "a"}, {GoalID: "b", Builds: []batch.BranchBuild{{Commit: "one"}, {Commit: "two"}}}, {GoalID: "c"}}
	trees, err := rebasedPrefixTrees(root, base, tip, units)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(trees, []string{expected[0], expected[2], expected[3]}) {
		t.Fatalf("prefix trees=%v, want selected cumulative trees", trees)
	}
	if _, err := rebasedPrefixTrees(root, base, tip, units[:2]); err == nil {
		t.Fatal("incomplete unit inventory accepted a rebased range")
	}
}

func TestGLEBatchDeliverySupplementKeepsPolicyFloor(t *testing.T) {
	t.Parallel()
	parsed, _, status := parseTestingSelection("test verify", []string{"--root", "/synthetic", "--goal", "goal-a", "--tree", "tree-a", "--purpose", "delivery", "--batch-prefix", "--batch-requirements", `{"groups":["admitted"]}`}, false)
	if status != 0 || !parsed.BatchPrefixReceipt || !slices.Equal(parsed.BatchRequirements, []string{"admitted"}) {
		t.Fatalf("batch parse=%+v status=%d", parsed, status)
	}
	contract := testpolicy.Contract{Always: testpolicy.Always{Canary: []string{"protected"}},
		Groups: []testpolicy.Group{{ID: "protected"}, {ID: "admitted"}}}
	plan, err := testpolicy.Select(contract, testpolicy.SelectionRequest{RequestedMode: parsed.Mode, Purpose: parsed.Purpose,
		BatchRequirements: parsed.BatchRequirements})
	if err != nil || !slices.Equal(plan.SelectedGroups, []string{"admitted", "protected"}) {
		t.Fatalf("delivery groups=%v err=%v", plan.SelectedGroups, err)
	}
	if !slices.Contains(plan.RequiredGroups, "admitted") {
		t.Fatalf("admitted member obligation is selected but not required: %+v", plan)
	}
	for _, status := range []string{"not-run", "failed"} {
		result := proofrun.TestResult{RequiredGroups: plan.RequiredGroups,
			Groups: []proofrun.GroupResult{{ID: "protected", Status: "passed"}, {ID: "admitted", Status: status}}}
		result.RecomputeDelivery()
		if result.Delivery.Sufficient {
			t.Fatalf("retained consumer certified %s admitted group", status)
		}
	}
	if _, err := testpolicy.Select(contract, testpolicy.SelectionRequest{RequestedMode: testpolicy.ModeAuto, Purpose: testpolicy.PurposeDelivery, Groups: []string{"admitted"}}); err == nil {
		t.Fatal("unmarked --groups promoted diagnostic selection to delivery")
	}
}

func TestGLEBatchRequirementsTransportIsStrict(t *testing.T) {
	t.Parallel()
	base := []string{"--root", "/synthetic", "--purpose", "delivery", "--batch-prefix"}
	valid := append(slices.Clone(base), "--batch-requirements", `{"groups":["a","b"]}`)
	request, _, code := parseTestingSelection("test plan", valid, false)
	if code != 0 || !slices.Equal(request.BatchRequirements, []string{"a", "b"}) || len(request.Groups) != 0 {
		t.Fatalf("typed requirements=%+v code=%d", request, code)
	}
	for name, args := range map[string][]string{
		"unknown field":    append(slices.Clone(base), "--batch-requirements", `{"groups":["a"],"plan":{"requiredGroups":[]}}`),
		"duplicate field":  append(slices.Clone(base), "--batch-requirements", `{"groups":["a"],"groups":["b"]}`),
		"wrong case":       append(slices.Clone(base), "--batch-requirements", `{"Groups":["a"]}`),
		"null groups":      append(slices.Clone(base), "--batch-requirements", `{"groups":null}`),
		"duplicate group":  append(slices.Clone(base), "--batch-requirements", `{"groups":["a","a"]}`),
		"trailing content": append(slices.Clone(base), "--batch-requirements", `{"groups":["a"]} {}`),
		"no prefix":        {"--root", "/synthetic", "--purpose", "delivery", "--batch-requirements", `{"groups":["a"]}`},
		"diagnostic":       {"--root", "/synthetic", "--purpose", "diagnostic", "--batch-prefix", "--batch-requirements", `{"groups":["a"]}`},
		"old groups":       append(slices.Clone(base), "--groups", "a"),
		"mixed groups":     append(slices.Clone(valid), "--groups", "a"),
	} {
		t.Run(name, func(t *testing.T) {
			code, _, stderr := captureCommandOutput(t, false, true, func() int {
				_, _, status := parseTestingSelection("test plan", args, false)
				return status
			})
			if code != 2 || stderr == "" {
				t.Fatalf("invalid batch requirements %v: code=%d stderr=%q", args, code, stderr)
			}
		})
	}
	if diagnostic, _, code := parseTestingSelection("test plan", []string{"--root", "/synthetic", "--mode", "canary", "--purpose", "diagnostic", "--groups", "a"}, false); code != 0 || !slices.Equal(diagnostic.Groups, []string{"a"}) {
		t.Fatalf("diagnostic --groups no longer works: %+v code=%d", diagnostic, code)
	}
}

func TestGLEBatchRequirementsFollowLegacyTrustedPolicyFloor(t *testing.T) {
	t.Parallel()
	engine := filepath.Join(t.TempDir(), "old-policy-engine")
	script := `#!/bin/sh
for arg in "$@"; do
  case "$arg" in --batch-requirements|--batch-prefix|--groups) exit 93;; esac
done
printf '%s\n' '{"schemaVersion":1,"candidateTree":"candidate","plan":{"purpose":"delivery","requiredGroups":["floor"],"selectedGroups":["floor"],"stages":[{"id":"canary","groups":["floor"]}]}}'
`
	if err := testexec.WriteFile(engine, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	request := testingSelectionRequest{Mode: testpolicy.ModeAuto, Purpose: testpolicy.PurposeDelivery,
		BatchPrefixReceipt: true, BatchRequirements: []string{"admitted"}}
	trusted, err := planWithTrustedPolicyEngine(engine, trustedPolicyFloorRequest(request), t.TempDir(), "candidate")
	if err != nil {
		t.Fatalf("legacy trusted policy child could not supply its floor: %v", err)
	}
	contract := testpolicy.Contract{Groups: []testpolicy.Group{{ID: "floor"}, {ID: "dependency"}, {ID: "admitted", Requires: []string{"dependency"}}}}
	plan, err := testpolicy.WithBatchRequirements(contract, trusted.Plan, request.BatchRequirements)
	if err != nil || !slices.Equal(plan.RequiredGroups, []string{"admitted", "dependency", "floor"}) ||
		!slices.Equal(plan.SelectedGroups, plan.RequiredGroups) {
		t.Fatalf("trusted floor lost when composing batch requirements: plan=%+v err=%v", plan, err)
	}
}

func TestGLEBatchTipRetainsEveryAdmittedMemberObligation(t *testing.T) {
	t.Parallel()
	units := []batch.Unit{{GoalID: "a", SelectedGroups: []string{"accepted-a"}}, {GoalID: "b", SelectedGroups: []string{"accepted-b"}}}
	plan, err := planBatchMemberUnion("", "tree", units, testpolicy.ModeAuto,
		func(_, _, _ string, _ testpolicy.Mode) (testpolicy.Plan, error) {
			return testpolicy.Plan{SelectedGroups: []string{"protected-floor"}, RequiredMode: testpolicy.ModeStandard, ExecutedMode: testpolicy.ModeStandard}, nil
		})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(plan.SelectedGroups, []string{"accepted-a", "accepted-b", "protected-floor"}) {
		t.Fatalf("tip omitted admitted union: %v", plan.SelectedGroups)
	}
}
