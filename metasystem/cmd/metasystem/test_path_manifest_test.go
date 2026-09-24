package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/pathpattern"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestGLEPathMovedInputAttributionUsesManifestPatterns(t *testing.T) {
	t.Parallel()
	manifest := []string{"metasystem/cmd/metasystem/landing_batch*.go", "metasystem/internal/proofrun/**"}
	for _, path := range []string{"metasystem/cmd/metasystem/landing_batch_land.go", "metasystem/cmd/metasystem/landing_batch_new.go", "metasystem/internal/proofrun/test_build.go"} {
		if !testInputManifestContains(manifest, path) {
			t.Errorf("missed moved input %s", path)
		}
	}
	if testInputManifestContains(manifest, "metasystem/cmd/metasystem/other.go") {
		t.Fatal("unrelated path attributed")
	}
	if !testInputManifestContains([]string{"fixtures"}, "fixtures/case.txt") {
		t.Fatal("legacy exact-directory input did not attribute its child")
	}
}

type deliverySnapshotFact struct {
	t            *testing.T
	candidate    string
	declarations []string
	tree         string
	err          error
	wantCalls    int
	calls        int
}

func (fact *deliverySnapshotFact) snapshot(candidateTree string, declarations []string) (string, error) {
	fact.t.Helper()
	fact.calls++
	if fact.calls > fact.wantCalls || candidateTree != fact.candidate || !slices.Equal(declarations, fact.declarations) {
		fact.t.Fatalf("snapshot call %d: candidate=%q declarations=%v; want candidate=%q declarations=%v and %d calls",
			fact.calls, candidateTree, declarations, fact.candidate, fact.declarations, fact.wantCalls)
	}
	return fact.tree, fact.err
}

func (fact *deliverySnapshotFact) assertConsumed() {
	fact.t.Helper()
	if fact.calls != fact.wantCalls {
		fact.t.Fatalf("snapshot calls = %d, want %d", fact.calls, fact.wantCalls)
	}
}

func TestGLEPathRootGoPackageExpansionPlansWithCompleteInputs(t *testing.T) {
	const candidate = "candidate-root-tree"
	const selected = "go-affected/root"
	root := t.TempDir()
	contract := testpolicy.Contract{Groups: []testpolicy.Group{{
		ID: selected, Packages: []string{"."}, Inputs: []string{"go.mod", "*", "*/**"},
	}}}
	plan := testpolicy.Plan{SelectedGroups: []string{selected}}
	previous := prepareTestingForCommand
	defer func() { prepareTestingForCommand = previous }()
	prepareTestingForCommand = func(testingSelectionRequest) (testingPreparation, error) {
		return testingPreparation{ProjectRoot: root, CandidateTree: candidate, EffectiveContract: contract, Plan: plan}, nil
	}
	status, stdout, _ := captureCommandOutput(t, true, true, func() int {
		return runTestPlan([]string{"--root", root, "--purpose", "diagnostic", "--json"})
	})
	var output testingPlanOutput
	if err := json.Unmarshal([]byte(stdout), &output); status != 0 || err != nil ||
		!slices.Equal(output.Plan.SelectedGroups, []string{selected}) ||
		len(output.Groups) != 1 || output.Groups[0].ID != selected {
		t.Fatalf("public root-package JSON plan = status %d output %q: %v", status, stdout, err)
	}
	fact := &deliverySnapshotFact{
		t: t, candidate: candidate, declarations: []string{"*", "*/**", "go.mod", "metasystem.conf", "testing.json"},
		tree: "working-root-tree", wantCalls: 1,
	}
	err := checkDeliveryInputParityWith(candidate, "", "testing.json", contract, plan, fact.snapshot)
	fact.assertConsumed()
	if err == nil || err.Error() != "delivery candidate differs from relevant working-tree inputs: candidate=candidate-root-tree working=working-root-tree" {
		t.Fatalf("root package input closure did not refuse nested input: %v", err)
	}
}

func TestGLEPathMovedBaseReopensForDiscoveredLiteral(t *testing.T) {
	t.Parallel()
	literal := pathpattern.EncodeLiteral("src/[literal].go")
	record := batch.Record{Proof: &batch.Proof{SelectedGroups: []string{"app"}, InputManifests: map[string][]string{"app": {literal}}}}
	if !batchProofInputsMoved(record, []string{"src/[literal].go"}, "metasystem") {
		t.Fatal("moved discovered literal did not reopen proof")
	}
	if batchProofInputsMoved(record, []string{"src/other.go"}, "metasystem") {
		t.Fatal("unrelated moved file reopened proof")
	}
}

func TestGLEPathPlanReportsOptionalNoMatch(t *testing.T) {
	root := t.TempDir()
	writeTestingFixtureFile(t, filepath.Join(root, "src", "real.go"), []byte("package src\n"), 0o644)
	testingFixtureGit(t, root, "init")
	testingFixtureGit(t, root, "add", ".")
	testingFixtureGit(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-m", "input")
	tree, err := (gittree.Workspace{Dir: root}).HeadTree()
	if err != nil {
		t.Fatal(err)
	}
	group := testpolicy.Group{ID: "app", Inputs: []string{"src/real.go", "src/futrue?.go"}}
	contract := testpolicy.Contract{Groups: []testpolicy.Group{group}}
	plan := testpolicy.Plan{SelectedGroups: []string{"app"}}
	unmatched, err := unmatchedTestingInputs(gittree.Workspace{Dir: root}, tree, contract, plan)
	if err != nil || len(unmatched) != 1 || unmatched[0].Group != "app" || unmatched[0].Pattern != "src/futrue?.go" {
		t.Fatalf("candidate no-match report = %v, %v", unmatched, err)
	}
	previous := prepareTestingForCommand
	defer func() { prepareTestingForCommand = previous }()
	prepareTestingForCommand = func(testingSelectionRequest) (testingPreparation, error) {
		return testingPreparation{ProjectRoot: root, CandidateTree: tree, EffectiveContract: contract, Plan: plan, UnmatchedInputs: unmatched}, nil
	}
	status, stdout, _ := captureCommandOutput(t, true, true, func() int {
		return runTestPlan([]string{"--root", root, "--purpose", "diagnostic", "--json"})
	})
	if status != 0 {
		t.Fatalf("public plan status = %d", status)
	}
	var output testingPlanOutput
	if err := json.Unmarshal([]byte(stdout), &output); err != nil || len(output.UnmatchedInputs) != 1 || output.UnmatchedInputs[0] != unmatched[0] {
		t.Fatalf("public no-match JSON = %q, %v", stdout, err)
	}
	status, _, stderr := captureCommandOutput(t, true, true, func() int {
		return runTestPlan([]string{"--root", root, "--purpose", "diagnostic"})
	})
	if status != 0 || !strings.Contains(stderr, "TEST-INPUT-NO-MATCH") || !strings.Contains(stderr, "futrue?.go") {
		t.Fatalf("public text no-match = %d, %q", status, stderr)
	}
	if _, err := os.Stat(filepath.Join(root, "src", "futrue1.go")); !os.IsNotExist(err) {
		t.Fatalf("optional input was created: %v", err)
	}
}

func TestGLEPathPublicDeliveryPlanRejectsDirtyWildcardInput(t *testing.T) {
	const candidate = "candidate-wildcard-tree"
	root := t.TempDir()
	contract := testpolicy.Contract{Groups: []testpolicy.Group{{ID: "app", Inputs: []string{"src/*.go"}}}}
	plan := testpolicy.Plan{SelectedGroups: []string{"app"}}
	declarations := []string{"metasystem.conf", "src/*.go", "testing.json"}
	unrelated := &deliverySnapshotFact{
		t: t, candidate: candidate, declarations: declarations, tree: candidate, wantCalls: 1,
	}
	if err := checkDeliveryInputParityWith(candidate, "", "testing.json", contract, plan, unrelated.snapshot); err != nil {
		t.Fatalf("unrelated working edit refused delivery: %v", err)
	}
	unrelated.assertConsumed()
	matching := &deliverySnapshotFact{
		t: t, candidate: candidate, declarations: declarations, tree: "working-wildcard-tree", wantCalls: 1,
	}
	previous := prepareTestingForCommand
	defer func() { prepareTestingForCommand = previous }()
	prepareTestingForCommand = func(testingSelectionRequest) (testingPreparation, error) {
		return testingPreparation{}, checkDeliveryInputParityWith(candidate, "", "testing.json", contract, plan, matching.snapshot)
	}
	status, _, stderr := captureCommandOutput(t, true, true, func() int {
		return runTestPlan([]string{"--root", root, "--purpose", "delivery"})
	})
	matching.assertConsumed()
	if status != 1 || !strings.Contains(stderr, "delivery candidate differs from relevant working-tree inputs") {
		t.Fatalf("public delivery plan accepted dirty wildcard input: status=%d stderr=%q", status, stderr)
	}
}

func TestGLEPathPublicDeliveryPlanRejectsUnstagedSelectedGoPackageSource(t *testing.T) {
	const candidate = "candidate-newpkg-tree"
	const selected = "go-affected/newpkg"
	root := t.TempDir()
	plan := testpolicy.Plan{SelectedGroups: []string{selected}}
	legacy := testpolicy.Contract{Groups: []testpolicy.Group{{
		ID: selected, Packages: []string{"./newpkg"}, Inputs: []string{"metasystem/go.mod", "metasystem/go.sum"},
	}}}
	complete := testpolicy.Contract{Groups: []testpolicy.Group{{
		ID: selected, Packages: []string{"./newpkg"},
		Inputs: []string{"metasystem/go.mod", "metasystem/go.sum", "metasystem/newpkg/**"},
	}}}
	oldFact := &deliverySnapshotFact{
		t: t, candidate: candidate,
		declarations: []string{"metasystem/go.mod", "metasystem/go.sum", "metasystem/metasystem.conf", "metasystem/testing.json"},
		tree:         candidate, wantCalls: 1,
	}
	if err := checkDeliveryInputParityWith(candidate, "metasystem/", "testing.json", legacy, plan, oldFact.snapshot); err != nil {
		t.Fatalf("old manifest unexpectedly detected the untracked package source: %v", err)
	}
	oldFact.assertConsumed()
	completeFact := &deliverySnapshotFact{
		t: t, candidate: candidate,
		declarations: []string{"metasystem/go.mod", "metasystem/go.sum", "metasystem/metasystem.conf", "metasystem/newpkg/**", "metasystem/testing.json"},
		tree:         "working-newpkg-tree", wantCalls: 1,
	}
	previous := prepareTestingForCommand
	defer func() { prepareTestingForCommand = previous }()
	prepareTestingForCommand = func(testingSelectionRequest) (testingPreparation, error) {
		return testingPreparation{}, checkDeliveryInputParityWith(candidate, "metasystem/", "testing.json", complete, plan, completeFact.snapshot)
	}
	status, _, stderr := captureCommandOutput(t, true, true, func() int {
		return runTestPlan([]string{"--root", root, "--purpose", "delivery"})
	})
	completeFact.assertConsumed()
	if status != 1 || !strings.Contains(stderr, "delivery candidate differs from relevant working-tree inputs") {
		t.Fatalf("public delivery plan accepted untracked selected-package source: status=%d stderr=%q", status, stderr)
	}
}

func TestDeliveryInputParityBoundary(t *testing.T) {
	t.Parallel()
	cause := errors.New("snapshot unavailable")
	cases := []struct {
		name         string
		prefix       string
		contract     testpolicy.Contract
		plan         testpolicy.Plan
		declarations []string
		workingTree  string
		snapshotErr  error
		wantCalls    int
		wantErr      string
	}{
		{
			name: "selected base package and shared script refuse changed working inputs", prefix: "metasystem/",
			contract: testpolicy.Contract{Groups: []testpolicy.Group{{
				ID: "go-affected/base", Inputs: []string{"metasystem/scripts/**", "metasystem/base/**"},
			}}},
			plan:         testpolicy.Plan{SelectedGroups: []string{"go-affected/base"}},
			declarations: []string{"metasystem/base/**", "metasystem/metasystem.conf", "metasystem/scripts/**", "metasystem/testing.json"},
			workingTree:  "working-boundary-tree", wantCalls: 1,
			wantErr: "delivery candidate differs from relevant working-tree inputs: candidate=candidate-boundary-tree working=working-boundary-tree",
		},
		{
			name: "sorted union and deduplication", prefix: "metasystem/",
			contract: testpolicy.Contract{Groups: []testpolicy.Group{
				{ID: "one", Inputs: []string{"src/*.go", "shared", "src/*.go"}},
				{ID: "two", Inputs: []string{"other", "shared"}},
				{ID: "section", Adapter: "section"},
				{ID: "command", Adapter: "command", CWD: "tools", Argv: []string{"./run.sh"}},
			}},
			plan: testpolicy.Plan{SelectedGroups: []string{"one", "two", "section", "command"}},
			declarations: []string{
				"metasystem/metasystem.conf", "metasystem/scripts/agents/validate-section-selector.sh",
				"metasystem/testing.json", "other", "shared", "src/*.go", "tools/run.sh",
			},
			workingTree: "candidate-boundary-tree", wantCalls: 1,
		},
		{
			name: "missing selected group", contract: testpolicy.Contract{Groups: []testpolicy.Group{{ID: "other"}}},
			plan: testpolicy.Plan{SelectedGroups: []string{"missing"}}, wantErr: "selected testing group missing is absent",
		},
		{
			name: "escaping relative executable", contract: testpolicy.Contract{Groups: []testpolicy.Group{{
				ID: "command", Adapter: "command", CWD: "tools", Argv: []string{"../../escape"},
			}}},
			plan: testpolicy.Plan{SelectedGroups: []string{"command"}}, wantErr: "testing group command command executable escapes the project",
		},
		{
			name: "snapshot error wrapping", contract: testpolicy.Contract{Groups: []testpolicy.Group{{ID: "app"}}},
			plan:         testpolicy.Plan{SelectedGroups: []string{"app"}},
			declarations: []string{"metasystem.conf", "testing.json"},
			snapshotErr:  cause, wantCalls: 1, wantErr: "capture relevant candidate working inputs: snapshot unavailable",
		},
		{
			name: "equal tree", contract: testpolicy.Contract{Groups: []testpolicy.Group{{ID: "app"}}},
			plan:         testpolicy.Plan{SelectedGroups: []string{"app"}},
			declarations: []string{"metasystem.conf", "testing.json"},
			workingTree:  "candidate-boundary-tree", wantCalls: 1,
		},
		{
			name: "mismatching tree", contract: testpolicy.Contract{Groups: []testpolicy.Group{{ID: "app"}}},
			plan:         testpolicy.Plan{SelectedGroups: []string{"app"}},
			declarations: []string{"metasystem.conf", "testing.json"},
			workingTree:  "working-boundary-tree", wantCalls: 1,
			wantErr: "delivery candidate differs from relevant working-tree inputs: candidate=candidate-boundary-tree working=working-boundary-tree",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			const candidate = "candidate-boundary-tree"
			fact := &deliverySnapshotFact{
				t: t, candidate: candidate, declarations: tc.declarations,
				tree: tc.workingTree, err: tc.snapshotErr, wantCalls: tc.wantCalls,
			}
			err := checkDeliveryInputParityWith(candidate, tc.prefix, "testing.json", tc.contract, tc.plan, fact.snapshot)
			fact.assertConsumed()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("parity returned unexpected error: %v", err)
				}
			} else if err == nil || err.Error() != tc.wantErr {
				t.Fatalf("parity error = %v, want %q", err, tc.wantErr)
			}
			if tc.snapshotErr != nil && !errors.Is(err, tc.snapshotErr) {
				t.Fatalf("snapshot cause was not wrapped: %v", err)
			}
		})
	}
}
