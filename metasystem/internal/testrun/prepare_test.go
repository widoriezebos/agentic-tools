package testrun

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testgit"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestBaseMovedUnderTheRunRestartsPreparationOnce(t *testing.T) {
	calls, state := 0, &PreparationState{}
	var prepared Preparation
	var prepareErr error
	status, _, stderr := runOnOwnStreams(func(stdout, stderr io.Writer) int {
		prepared, prepareErr = prepareWith(SelectionRequest{LandedRearm: true, Preparation: state, Notes: stderr}, func(request SelectionRequest) (Preparation, error) {
			calls++
			if !request.LandedRearm {
				return Preparation{}, errors.New("restart skipped landed re-arm entry")
			}
			if calls == 1 {
				return Preparation{}, &baseMove{ours: "old-base", engine: "new-base"}
			}
			return Preparation{PolicyBaseCommit: "new-base"}, nil
		})
		if prepareErr != nil {
			return 1
		}
		return 0
	})
	if status != 0 || prepareErr != nil || calls != 2 || prepared.PolicyBaseCommit != "new-base" ||
		!state.restarted || os.Getenv("METASYSTEM_PREPARATION_RESTARTED") != "" || !strings.Contains(stderr, "restarting preparation once") {
		t.Fatalf("authenticated move did not restart once from the landed re-arm entry: status=%d calls=%d prepared=%+v err=%v restarted=%t stderr=%q",
			status, calls, prepared, prepareErr, state.restarted, stderr)
	}
}

type policyBaseMoveFixture struct {
	root    string
	parents map[string]string
	refText string
	commits map[string]string
	git     *testgit.Stub
}

func newPolicyBaseMoveFixture(t *testing.T, parents map[string]string, tip string, calls ...[]string) *policyBaseMoveFixture {
	t.Helper()
	root := t.TempDir()
	copyParents := make(map[string]string, len(parents))
	for commit, parent := range parents {
		if !policyBaseFixtureSHA(commit) || parent != "" && !policyBaseFixtureSHA(parent) {
			t.Fatalf("invalid fixture commit or parent: %q %q", commit, parent)
		}
		copyParents[commit] = parent
	}
	if _, exists := copyParents[tip]; !exists {
		t.Fatalf("landing tip is absent from fixture ancestry: %q", tip)
	}
	expected := make([]testgit.Expectation, 0, len(calls))
	for _, call := range calls {
		expected = append(expected, testgit.Expectation{Call: testgit.Call{Dir: root, Args: call}})
	}
	return &policyBaseMoveFixture{
		root: root, parents: copyParents, refText: "refs/remotes/origin/main",
		commits: map[string]string{"refs/remotes/origin/main": tip}, git: testgit.New(t, expected...),
	}
}

func policyBaseFixtureSHA(value string) bool {
	if len(value) != 40 {
		return false
	}
	for _, digit := range value {
		if digit < '0' || digit > '9' && digit < 'a' || digit > 'f' {
			return false
		}
	}
	return true
}

func (fixture *policyBaseMoveFixture) readers() policyBaseMoveReaders {
	return policyBaseMoveReaders{
		isAncestor: func(root, ours, engine string) (bool, error) {
			if result := fixture.git.Run(testgit.Call{Dir: root, Args: []string{"ancestry", ours, engine}}); result.Err != nil {
				return false, result.Err
			}
			if !policyBaseFixtureSHA(ours) || !policyBaseFixtureSHA(engine) {
				return false, fmt.Errorf("invalid ancestry commit")
			}
			if _, exists := fixture.parents[ours]; !exists {
				return false, fmt.Errorf("unknown ancestor commit %s", ours)
			}
			for commit := engine; commit != ""; commit = fixture.parents[commit] {
				if _, exists := fixture.parents[commit]; !exists {
					return false, fmt.Errorf("unknown descendant commit %s", commit)
				}
				if commit == ours {
					return true, nil
				}
			}
			return false, nil
		},
		localLandingRef: func(root string) (string, error) {
			result := fixture.git.Run(testgit.Call{Dir: root, Args: []string{"landing-ref"}})
			return fixture.refText, result.Err
		},
		commitAtRef: func(root, ref string) (string, error) {
			result := fixture.git.Run(testgit.Call{Dir: root, Args: []string{"commit-at-ref", ref}})
			if result.Err != nil {
				return "", result.Err
			}
			commit, exists := fixture.commits[ref]
			if !exists || !policyBaseFixtureSHA(commit) {
				return "", fmt.Errorf("unknown landing ref or invalid commit %q", ref)
			}
			return commit, nil
		},
	}
}

func TestBaseMovedToAnEngineChangeReArmsOnce(t *testing.T) {
	for _, test := range []struct {
		name           string
		contractChange bool
	}{
		{name: "engine path only"},
		{name: "engine path and testing contract", contractChange: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			old, moved := strings.Repeat("a", 40), strings.Repeat("b", 40)
			fixture := newPolicyBaseMoveFixture(t, map[string]string{old: "", moved: old}, moved,
				[]string{"ancestry", old, moved}, []string{"landing-ref"}, []string{"commit-at-ref", "refs/remotes/origin/main"})
			decision := PlanOutput{CandidateTree: "candidate", PolicyBaseCommit: moved, BaseContractDigest: "old-digest"}
			if test.contractChange {
				decision.BaseContractDigest = "new-digest"
			}
			calls, rearms, state := 0, 0, &PreparationState{}
			prepared, err := prepareWith(SelectionRequest{LandedRearm: true, Preparation: state}, func(request SelectionRequest) (Preparation, error) {
				calls++
				if calls == 1 {
					return Preparation{}, compareTrustedPolicyDecisionWithReaders("candidate", old, "old-digest", decision, fixture.root, fixture.readers())
				}
				if request.LandedRearm {
					rearms++
				}
				return Preparation{PolicyBaseCommit: moved}, nil
			})
			if err != nil || calls != 2 || rearms != 1 || prepared.PolicyBaseCommit != moved || !state.restarted {
				t.Fatalf("descendant move did not succeed after exactly one restart and re-arm: calls=%d rearms=%d prepared=%+v restarted=%t err=%v", calls, rearms, prepared, state.restarted, err)
			}
		})
	}
}

func TestSecondBaseMoveRefusesBaseMoved(t *testing.T) {
	calls := 0
	_, err := prepareWith(SelectionRequest{}, func(SelectionRequest) (Preparation, error) {
		calls++
		if calls == 1 {
			return Preparation{}, &baseMove{ours: "base-a", engine: "base-b"}
		}
		return Preparation{}, &baseMove{ours: "base-b", engine: "base-c"}
	})
	if err == nil || calls != 2 || !strings.Contains(fullRefusal(err), "cause=base-moved ours=base-b engine=base-c restarts=1") {
		t.Fatalf("second move did not refuse with the guarded base-moved cause: calls=%d err=%v", calls, err)
	}
}

// TestPreparationRestartAllowanceIsInvocationLocal is the VOA-14 witness:
// two consecutive preparations in one resident process, each with one base
// movement, are both admitted; two movements within one invocation, across
// its preparations, are still refused; and nothing reaches the process
// environment.
func TestPreparationRestartAllowanceIsInvocationLocal(t *testing.T) {
	onceMoved := func(calls *int) preparationAttempt {
		return func(SelectionRequest) (Preparation, error) {
			*calls++
			if *calls == 1 {
				return Preparation{}, &baseMove{ours: "base-a", engine: "base-b"}
			}
			return Preparation{PolicyBaseCommit: "base-b"}, nil
		}
	}
	runOnOwnStreams(func(stdout, stderr io.Writer) int {
		for invocation := 1; invocation <= 2; invocation++ {
			calls := 0
			request := SelectionRequest{Preparation: &PreparationState{}, Notes: stderr}
			if prepared, err := prepareWith(request, onceMoved(&calls)); err != nil || calls != 2 || prepared.PolicyBaseCommit != "base-b" {
				t.Fatalf("invocation %d in the same process: calls=%d prepared=%+v err=%v", invocation, calls, prepared, err)
			}
		}
		return 0
	})
	if value, set := os.LookupEnv("METASYSTEM_PREPARATION_RESTARTED"); set {
		t.Fatalf("preparation state leaked into the process environment: %q", value)
	}
	shared := &PreparationState{}
	first, second := 0, 0
	runOnOwnStreams(func(stdout, stderr io.Writer) int {
		if _, err := prepareWith(SelectionRequest{Preparation: shared, Notes: stderr}, onceMoved(&first)); err != nil {
			t.Fatalf("first preparation of the invocation: %v", err)
		}
		_, err := prepareWith(SelectionRequest{Preparation: shared, Notes: stderr}, onceMoved(&second))
		if err == nil || second != 1 || !strings.Contains(fullRefusal(err), "cause=base-moved") {
			t.Fatalf("a second movement within one invocation was admitted: calls=%d err=%v", second, err)
		}
		return 0
	})
}

func TestUnexplainedPolicyFieldRefusesDecisionMismatch(t *testing.T) {
	old, moved := strings.Repeat("a", 40), strings.Repeat("b", 40)
	fixture := newPolicyBaseMoveFixture(t, map[string]string{old: "", moved: old}, moved)
	err := compareTrustedPolicyDecisionWithReaders("candidate", old, "digest", PlanOutput{
		CandidateTree: "other-candidate", PolicyBaseCommit: moved, BaseContractDigest: "other-digest",
	}, fixture.root, fixture.readers())
	if err == nil || !strings.Contains(fullRefusal(err), "cause=decision-mismatch field=candidate-tree") {
		t.Fatalf("candidate mismatch was incorrectly explained by the base move: %v", err)
	}
}

func TestNonDescendantPolicyBaseRefusesDecisionMismatch(t *testing.T) {
	common, ours, sibling := strings.Repeat("a", 40), strings.Repeat("b", 40), strings.Repeat("c", 40)
	fixture := newPolicyBaseMoveFixture(t, map[string]string{common: "", ours: common, sibling: common}, sibling,
		[]string{"ancestry", ours, sibling})
	err := compareTrustedPolicyDecisionWithReaders("candidate", ours, "digest", PlanOutput{
		CandidateTree: "candidate", PolicyBaseCommit: sibling, BaseContractDigest: "digest",
	}, fixture.root, fixture.readers())
	if err == nil || !strings.Contains(fullRefusal(err), "cause=decision-mismatch field=policy-base-commit") {
		t.Fatalf("non-descendant base was incorrectly restarted: %v", err)
	}
}

func TestTrustedPolicyEngineIsRequiredWithoutBuildingDuringReadOnlySelection(t *testing.T) {
	root := t.TempDir()
	if _, _, _, err := TrustedPolicyEngine(root, strings.Repeat("a", 40), false); err == nil || !strings.Contains(fullRefusal(err), "TEST_POLICY_ENGINE_REQUIRED") {
		t.Fatalf("missing retained policy engine was accepted: %v", err)
	}
	if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
		t.Fatalf("read-only policy selection created build inputs: entries=%v err=%v", entries, err)
	}
}

func TestTestingPlanAdoptsCandidateFallbackOnlyWhenBaseHasNone(t *testing.T) {
	candidate := testFallbackContract()
	for _, test := range []struct {
		name             string
		baseFallback     bool
		expectedFallback string
	}{
		{name: "candidate-fallback-fills-empty-base", expectedFallback: "residual"},
		{name: "base-fallback-remains-protected", baseFallback: true, expectedFallback: "trusted"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("PATH", t.TempDir())
			base := candidate
			base.Surfaces = append([]testpolicy.Surface(nil), candidate.Surfaces...)
			base.Groups = append([]testpolicy.Group(nil), candidate.Groups...)
			if test.baseFallback {
				base.Fallback = "trusted"
				base.Surfaces[1].Paths = []string{"candidate-owned/**"}
				base.Surfaces = append(base.Surfaces, testpolicy.Surface{ID: "trusted", Paths: []string{}, Standard: []string{"trusted"}})
				base.Groups = append(base.Groups, testFallbackGroup("trusted", "unowned.txt"))
			} else {
				base.Fallback = ""
				base.Surfaces = append([]testpolicy.Surface(nil), candidate.Surfaces[:1]...)
				base.Groups = append([]testpolicy.Group(nil), candidate.Groups[:1]...)
			}
			baseBytes, err := json.Marshal(base)
			if err != nil {
				t.Fatal(err)
			}
			candidateBytes, err := json.Marshal(candidate)
			if err != nil {
				t.Fatal(err)
			}
			baseContract, err := testpolicy.Decode(baseBytes)
			if err != nil {
				t.Fatal(err)
			}
			candidateContract, err := testpolicy.Decode(candidateBytes)
			if err != nil {
				t.Fatal(err)
			}
			effective := protectedContractWithCandidateFallback(baseContract, candidateContract)
			if err := effective.Validate(); err != nil {
				t.Fatal(err)
			}
			if effective.Fallback != test.expectedFallback {
				t.Fatalf("effective fallback = %q, want %q", effective.Fallback, test.expectedFallback)
			}
			plan, err := testpolicy.Select(effective, testpolicy.SelectionRequest{
				ChangedPaths: []string{"unowned.txt"}, RequestedMode: testpolicy.ModeAuto, Purpose: testpolicy.PurposeDiagnostic,
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(plan.Uncertainty) != 0 || !containsString(plan.AffectedSurfaces, test.expectedFallback) || !containsString(plan.SelectedGroups, test.expectedFallback) {
				t.Fatalf("unowned path did not select the fallback without uncertainty: %+v", plan)
			}
			if test.baseFallback && containsString(plan.AffectedSurfaces, candidate.Fallback) {
				t.Fatalf("candidate fallback replaced the protected base fallback: %+v", plan)
			}
			wantGroups := []string{"app", test.expectedFallback}
			if !reflect.DeepEqual(plan.SelectedGroups, wantGroups) {
				t.Fatalf("selected groups = %v, want %v", plan.SelectedGroups, wantGroups)
			}
		})
	}
}

func testFallbackContract() testpolicy.Contract {
	return testpolicy.Contract{SchemaVersion: 1,
		ProjectRisk: testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"},
		Fallback:    "residual",
		Surfaces: []testpolicy.Surface{
			{ID: "app", Paths: []string{"owned/**"}, Standard: []string{"app"}},
			{ID: "residual", Paths: []string{}, Standard: []string{"residual"}},
		},
		Groups:  []testpolicy.Group{testFallbackGroup("app", "owned/**"), testFallbackGroup("residual", "unowned.txt")},
		Always:  testpolicy.Always{Canary: []string{"app"}},
		Unknown: []string{"app"},
		Cadence: []string{"app"},
	}
}

func testFallbackGroup(id, input string) testpolicy.Group {
	return testpolicy.Group{ID: id, Kind: "unit", Adapter: "go", CWD: ".", Inputs: []string{input}, Outputs: []string{}, Tools: []testpolicy.Tool{},
		Obligations: []string{}, Platforms: []string{"any"}, TargetMS: 1000, Packages: []string{"."}, Tests: json.RawMessage(`"all"`)}
}

func TestProtectedCoverageFloorCannotFallOrDisappear(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	root := t.TempDir()
	const baseTree = "1111111111111111111111111111111111111111"
	const loweredTree = "2222222222222222222222222222222222222222"
	const raisedTree = "3333333333333333333333333333333333333333"
	const baseOID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	const linuxOID = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	const loweredOID = "cccccccccccccccccccccccccccccccccccccccc"
	const raisedOID = "dddddddddddddddddddddddddddddddddddddddd"
	const baselinePath = "testing-coverage-floors.json"
	const linuxPath = "testing-coverage-floors-linux.json"
	const legacyPath = "scripts/agents/coverage-ratchet.json"
	const legacyLinuxPath = "scripts/agents/coverage-ratchet-linux.json"
	baseJSON := []byte(`{"floors":{"internal/app":80.0}}`)
	linuxJSON := []byte(`{"floors":{"internal/app":79.0}}`)
	loweredJSON := []byte(`{"floors":{"internal/app":79.9}}`)
	raisedJSON := []byte(`{"floors":{"internal/app":80.1,"internal/new":50.0}}`)
	type rawFact struct {
		args   []string
		output []byte
	}
	var facts []rawFact
	addFile := func(tree, path, oid string, content []byte) {
		facts = append(facts,
			rawFact{args: []string{"--literal-pathspecs", "ls-tree", "-r", "-z", "--full-tree", tree, "--", path},
				output: []byte(fmt.Sprintf("100644 blob %s\t%s\x00", oid, path))},
			rawFact{args: []string{"cat-file", "blob", oid}, output: content},
		)
	}
	addAbsent := func(tree, path string) {
		facts = append(facts, rawFact{args: []string{"--literal-pathspecs", "ls-tree", "-r", "-z", "--full-tree", tree, "--", path}})
	}
	pins := []string{"-C", root, "-c", "core.fileMode=true", "-c", "diff.noprefix=false", "-c", "diff.mnemonicPrefix=false",
		"-c", "apply.ignoreWhitespace=no", "-c", "core.logAllRefUpdates=false", "-c", "core.useReplaceRefs=false",
		"-c", "gc.auto=0", "-c", "maintenance.auto=false"}
	next := 0
	workspace := gittree.Workspace{Dir: root, RawSource: func(request gittree.RawRequest) gittree.RawResult {
		if next >= len(facts) {
			t.Fatalf("unexpected raw Git request: %v", request.Args)
		}
		fact := facts[next]
		next++
		wantArgs := append(append([]string(nil), pins...), fact.args...)
		if request.Dir != root || !reflect.DeepEqual(request.Args, wantArgs) || request.Operation != "git "+strings.Join(fact.args, " ") ||
			request.Stdin != nil || !reflect.DeepEqual(request.Env, gittree.ScrubbedEnviron()) {
			t.Fatalf("raw Git request %d = %+v, want args %v", next, request, wantArgs)
		}
		return gittree.RawResult{Stdout: fact.output}
	}}
	addFile(baseTree, baselinePath, baseOID, baseJSON)
	addFile(loweredTree, baselinePath, loweredOID, loweredJSON)
	if err := protectCoverageRatchets(workspace, baseTree, loweredTree, ""); err == nil || !strings.Contains(fullRefusal(err), "TEST_POLICY_COVERAGE_FLOOR_LOWERED") {
		t.Fatalf("lowered base floor was accepted: %v", err)
	}
	if next != len(facts) {
		t.Fatalf("lowered floor consumed %d of %d raw Git requests", next, len(facts))
	}
	facts = nil
	next = 0
	addFile(baseTree, baselinePath, baseOID, baseJSON)
	addFile(raisedTree, baselinePath, raisedOID, raisedJSON)
	addFile(baseTree, linuxPath, linuxOID, linuxJSON)
	addFile(raisedTree, linuxPath, linuxOID, linuxJSON)
	if err := protectCoverageRatchets(workspace, baseTree, raisedTree, ""); err != nil {
		t.Fatalf("raised protected floor was refused: %v", err)
	}
	if next != len(facts) {
		t.Fatalf("raised floor consumed %d of %d raw Git requests", next, len(facts))
	}

	// The landing that moves the floors beside testing.json: its base keeps
	// them at the legacy path, and the moved floors are judged against them.
	facts = nil
	next = 0
	addAbsent(baseTree, baselinePath)
	addFile(baseTree, legacyPath, baseOID, baseJSON)
	addFile(loweredTree, baselinePath, loweredOID, loweredJSON)
	if err := protectCoverageRatchets(workspace, baseTree, loweredTree, ""); err == nil || !strings.Contains(fullRefusal(err), "TEST_POLICY_COVERAGE_FLOOR_LOWERED") {
		t.Fatalf("a floor lowered while moving was accepted: %v", err)
	}
	facts = nil
	next = 0
	addAbsent(baseTree, baselinePath)
	addFile(baseTree, legacyPath, baseOID, baseJSON)
	addFile(raisedTree, baselinePath, baseOID, baseJSON)
	addAbsent(baseTree, linuxPath)
	addFile(baseTree, legacyLinuxPath, linuxOID, linuxJSON)
	addFile(raisedTree, linuxPath, linuxOID, linuxJSON)
	if err := protectCoverageRatchets(workspace, baseTree, raisedTree, ""); err != nil {
		t.Fatalf("the move refused itself: %v", err)
	}
	if next != len(facts) {
		t.Fatalf("the move consumed %d of %d raw Git requests", next, len(facts))
	}
}

// A floor leaves with its package: deleting a package takes its floors out of
// both files, and that is no lowered floor, since nothing is left to measure.
// A floor dropped while its package still has Go files stays refused.
func TestProtectedCoverageFloorLeavesWithItsPackage(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	root := t.TempDir()
	const baseTree = "1111111111111111111111111111111111111111"
	const candidateTree = "2222222222222222222222222222222222222222"
	const baseOID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	const candidateOID = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	const baselinePath = "testing-coverage-floors.json"
	const linuxPath = "testing-coverage-floors-linux.json"
	baseJSON := []byte(`{"floors":{"internal/app":80.0,"internal/gone":70.0}}`)
	candidateJSON := []byte(`{"floors":{"internal/app":80.0}}`)
	type rawFact struct {
		args   []string
		output []byte
	}
	var facts []rawFact
	addFile := func(tree, path, oid string, content []byte) {
		facts = append(facts,
			rawFact{args: []string{"--literal-pathspecs", "ls-tree", "-r", "-z", "--full-tree", tree, "--", path},
				output: []byte(fmt.Sprintf("100644 blob %s\t%s\x00", oid, path))},
			rawFact{args: []string{"cat-file", "blob", oid}, output: content},
		)
	}
	addListing := func(tree, path string, files ...string) {
		var output []byte
		for _, file := range files {
			output = append(output, []byte(fmt.Sprintf("100644 blob %s\t%s\x00", candidateOID, file))...)
		}
		facts = append(facts, rawFact{args: []string{"--literal-pathspecs", "ls-tree", "-r", "-z", "--full-tree", tree, "--", path}, output: output})
	}
	pins := []string{"-C", root, "-c", "core.fileMode=true", "-c", "diff.noprefix=false", "-c", "diff.mnemonicPrefix=false",
		"-c", "apply.ignoreWhitespace=no", "-c", "core.logAllRefUpdates=false", "-c", "core.useReplaceRefs=false",
		"-c", "gc.auto=0", "-c", "maintenance.auto=false"}
	next := 0
	workspace := gittree.Workspace{Dir: root, RawSource: func(request gittree.RawRequest) gittree.RawResult {
		if next >= len(facts) {
			t.Fatalf("unexpected raw Git request: %v", request.Args)
		}
		fact := facts[next]
		next++
		if wantArgs := append(append([]string(nil), pins...), fact.args...); !reflect.DeepEqual(request.Args, wantArgs) {
			t.Fatalf("raw Git request %d = %v, want %v", next, request.Args, wantArgs)
		}
		return gittree.RawResult{Stdout: fact.output}
	}}

	addFile(baseTree, baselinePath, baseOID, baseJSON)
	addFile(candidateTree, baselinePath, candidateOID, candidateJSON)
	addListing(candidateTree, "internal/gone")
	addListing(baseTree, linuxPath)
	addListing(baseTree, "scripts/agents/coverage-ratchet-linux.json")
	if err := protectCoverageRatchets(workspace, baseTree, candidateTree, ""); err != nil {
		t.Fatalf("a floor that left with its deleted package was refused: %v", err)
	}
	if next != len(facts) {
		t.Fatalf("the deletion consumed %d of %d raw Git requests", next, len(facts))
	}

	facts, next = nil, 0
	addFile(baseTree, baselinePath, baseOID, baseJSON)
	addFile(candidateTree, baselinePath, candidateOID, candidateJSON)
	addListing(candidateTree, "internal/gone", "internal/gone/testdata/fixture.txt", "internal/gone/gone.go")
	if err := protectCoverageRatchets(workspace, baseTree, candidateTree, ""); err == nil || !strings.Contains(fullRefusal(err), "TEST_POLICY_COVERAGE_FLOOR_LOWERED") {
		t.Fatalf("a floor dropped from a package that still exists was accepted: %v", err)
	}

	facts, next = nil, 0
	addFile(baseTree, baselinePath, baseOID, baseJSON)
	addFile(candidateTree, baselinePath, candidateOID, candidateJSON)
	addListing(candidateTree, "internal/gone", "internal/gone/sub/other.go")
	addListing(baseTree, linuxPath)
	addListing(baseTree, "scripts/agents/coverage-ratchet-linux.json")
	if err := protectCoverageRatchets(workspace, baseTree, candidateTree, ""); err != nil {
		t.Fatalf("a nested package's files kept the deleted package's floor: %v", err)
	}
}

func TestAmbientTrustedPolicyDecisionCannotBypassRetainedEngine(t *testing.T) {
	root := t.TempDir()
	t.Setenv("METASYSTEM_TRUSTED_POLICY_DECISION", "1")
	if _, _, _, err := TrustedPolicyEngine(root, strings.Repeat("a", 40), false); err == nil || !strings.Contains(fullRefusal(err), "TEST_POLICY_ENGINE_REQUIRED") {
		t.Fatalf("ambient flag bypassed retained engine authentication: %v", err)
	}
	if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
		t.Fatalf("ambient policy selection created build inputs: entries=%v err=%v", entries, err)
	}
}
