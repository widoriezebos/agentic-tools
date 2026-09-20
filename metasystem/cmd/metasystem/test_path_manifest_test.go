package main

import (
	"encoding/json"
	"os"
	"path/filepath"
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
	root := t.TempDir()
	for name, body := range map[string]string{
		"metasystem.conf": "testing.contract=testing.json\n",
		"testing.json":    "{}\n",
		"src/a.go":        "candidate\n",
		"src/other.txt":   "candidate\n",
	} {
		writeTestingFixtureFile(t, filepath.Join(root, name), []byte(body), 0o644)
	}
	testingFixtureGit(t, root, "init")
	testingFixtureGit(t, root, "add", ".")
	testingFixtureGit(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-m", "candidate")
	workspace := gittree.Workspace{Dir: root}
	tree, err := workspace.HeadTree()
	if err != nil {
		t.Fatal(err)
	}
	contract := testpolicy.Contract{Groups: []testpolicy.Group{{ID: "app", Inputs: []string{"src/*.go"}}}}
	plan := testpolicy.Plan{SelectedGroups: []string{"app"}}
	writeTestingFixtureFile(t, filepath.Join(root, "src", "other.txt"), []byte("unrelated edit\n"), 0o644)
	if err := checkDeliveryInputParity(workspace, tree, "", "testing.json", contract, plan); err != nil {
		t.Fatalf("unrelated working edit refused delivery: %v", err)
	}
	writeTestingFixtureFile(t, filepath.Join(root, "src", "a.go"), []byte("dirty matching edit\n"), 0o644)
	previous := prepareTestingForCommand
	defer func() { prepareTestingForCommand = previous }()
	prepareTestingForCommand = func(testingSelectionRequest) (testingPreparation, error) {
		return testingPreparation{}, checkDeliveryInputParity(workspace, tree, "", "testing.json", contract, plan)
	}
	status, _, stderr := captureCommandOutput(t, true, true, func() int {
		return runTestPlan([]string{"--root", root, "--purpose", "delivery"})
	})
	if status != 1 || !strings.Contains(stderr, "delivery candidate differs from relevant working-tree inputs") {
		t.Fatalf("public delivery plan accepted dirty wildcard input: status=%d stderr=%q", status, stderr)
	}
}
