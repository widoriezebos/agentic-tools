package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/pathpattern"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
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

func TestGLEPathRootGoPackageExpansionPlansWithCompleteInputs(t *testing.T) {
	root := t.TempDir()
	writeTestingFixtureFile(t, filepath.Join(root, "go.mod"), []byte("module example.invalid/root\n\ngo 1.27\n"), 0o644)
	writeTestingFixtureFile(t, filepath.Join(root, "root.go"), []byte("package root\nconst Value = 1\n"), 0o644)
	writeTestingFixtureFile(t, filepath.Join(root, "root_test.go"), []byte("package root\nimport \"testing\"\nfunc TestRoot(t *testing.T) {}\n"), 0o644)
	testingFixtureGit(t, root, "init", "-q")
	testingFixtureGit(t, root, "add", ".")
	testingFixtureGit(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "base")
	workspace := gittree.Workspace{Dir: root}
	base, err := workspace.HeadTree()
	if err != nil {
		t.Fatal(err)
	}
	writeTestingFixtureFile(t, filepath.Join(root, "root.go"), []byte("package root\nconst Value = 2\n"), 0o644)
	testingFixtureGit(t, root, "add", "root.go")
	candidate := strings.TrimSpace(testingFixtureGit(t, root, "write-tree"))
	contract, err := testpolicy.Load(filepath.Join("..", "..", "testing.json"))
	if err != nil {
		t.Fatal(err)
	}
	for index := range contract.Groups {
		if contract.Groups[index].ID == "go-affected" {
			contract.Groups[index].CWD = "."
			contract.Groups[index].Inputs = []string{"go.mod"}
		}
	}
	expanded, err := proofrun.ExpandGoPackageGroups(contract, root, base, candidate)
	if err != nil {
		t.Fatal(err)
	}
	var selected string
	for _, group := range expanded.Groups {
		if strings.HasPrefix(group.ID, "go-affected/") && slices.Equal(group.Packages, []string{"."}) {
			selected = group.ID
			if !slices.Contains(group.Inputs, "*") || !slices.Contains(group.Inputs, "*/**") {
				t.Fatalf("root package inputs are incomplete: %v", group.Inputs)
			}
		}
	}
	if selected == "" {
		t.Fatal("root Go package was not expanded")
	}
	plan := testpolicy.Plan{SelectedGroups: []string{selected}}
	previous := prepareTestingForCommand
	defer func() { prepareTestingForCommand = previous }()
	prepareTestingForCommand = func(testingSelectionRequest) (testingPreparation, error) {
		return testingPreparation{ProjectRoot: root, CandidateTree: candidate, EffectiveContract: expanded, Plan: plan}, nil
	}
	status, stdout, _ := captureCommandOutput(t, true, true, func() int {
		return runTestPlan([]string{"--root", root, "--purpose", "diagnostic", "--json"})
	})
	if status != 0 || !strings.Contains(stdout, selected) {
		t.Fatalf("public root-package plan = status %d output %q", status, stdout)
	}
	writeTestingFixtureFile(t, filepath.Join(root, "nested", "new-input.txt"), []byte("unstaged\n"), 0o644)
	if err := checkDeliveryInputParity(workspace, candidate, "", "testing.json", expanded, plan); err == nil || !strings.Contains(err.Error(), "delivery candidate differs") {
		t.Fatalf("root package input closure missed nested unstaged file: %v", err)
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

func TestGLEPathPublicDeliveryPlanRejectsUnstagedSelectedGoPackageSource(t *testing.T) {
	root := t.TempDir()
	for name, body := range map[string]string{
		"metasystem/go.mod":                "module example.invalid/host\n\ngo 1.27\n",
		"metasystem/newpkg/newpkg.go":      "package newpkg\nconst Value = 1\n",
		"metasystem/newpkg/newpkg_test.go": "package newpkg\nimport \"testing\"\nfunc TestValue(t *testing.T) {}\n",
		"metasystem/metasystem.conf":       "testing.contract=testing.json\n",
		"metasystem/testing.json":          "{}\n",
	} {
		writeTestingFixtureFile(t, filepath.Join(root, name), []byte(body), 0o644)
	}
	testingFixtureGit(t, root, "init", "-q")
	testingFixtureGit(t, root, "add", ".")
	testingFixtureGit(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "base")
	workspace := gittree.Workspace{Dir: root}
	base, err := workspace.HeadTree()
	if err != nil {
		t.Fatal(err)
	}
	writeTestingFixtureFile(t, filepath.Join(root, "metasystem/newpkg/newpkg.go"), []byte("package newpkg\nconst Value = 2\n"), 0o644)
	testingFixtureGit(t, root, "add", "metasystem/newpkg/newpkg.go")
	candidate := strings.TrimSpace(testingFixtureGit(t, root, "write-tree"))
	contract, err := testpolicy.Load(filepath.Join("..", "..", "testing.json"))
	if err != nil {
		t.Fatal(err)
	}
	expanded, err := proofrun.ExpandGoPackageGroups(contract, root, base, candidate)
	if err != nil {
		t.Fatal(err)
	}
	selected := ""
	for _, group := range expanded.Groups {
		if len(group.Packages) == 1 && group.Packages[0] == "./newpkg" && strings.HasPrefix(group.ID, "go-affected/") {
			selected = group.ID
			break
		}
	}
	if selected == "" {
		t.Fatal("changed Go package was not selected")
	}
	plan := testpolicy.Plan{SelectedGroups: []string{selected}}
	writeTestingFixtureFile(t, filepath.Join(root, "metasystem/newpkg/unstaged.go"), []byte("package newpkg\nconst Unstaged = 1\n"), 0o644)
	legacy := expanded
	legacy.Groups = append([]testpolicy.Group(nil), expanded.Groups...)
	for i := range legacy.Groups {
		if legacy.Groups[i].ID == selected {
			legacy.Groups[i].Inputs = []string{"metasystem/go.mod", "metasystem/go.sum"}
		}
	}
	if err := checkDeliveryInputParity(workspace, candidate, "metasystem/", "testing.json", legacy, plan); err != nil {
		t.Fatalf("old manifest unexpectedly detected the untracked package source: %v", err)
	}
	previous := prepareTestingForCommand
	defer func() { prepareTestingForCommand = previous }()
	prepareTestingForCommand = func(testingSelectionRequest) (testingPreparation, error) {
		return testingPreparation{}, checkDeliveryInputParity(workspace, candidate, "metasystem/", "testing.json", expanded, plan)
	}
	status, _, stderr := captureCommandOutput(t, true, true, func() int {
		return runTestPlan([]string{"--root", root, "--purpose", "delivery"})
	})
	if status != 1 || !strings.Contains(stderr, "delivery candidate differs from relevant working-tree inputs") {
		t.Fatalf("public delivery plan accepted untracked selected-package source: status=%d stderr=%q", status, stderr)
	}
}
