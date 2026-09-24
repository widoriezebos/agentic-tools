package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gopackages"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestGoalLandingHostContractRetainsPriorGroupsAndGoGate(t *testing.T) {
	t.Parallel()
	contract, err := testpolicy.Load(filepath.Join("..", "..", "testing.json"))
	if err != nil {
		t.Fatal(err)
	}
	if contract.SchemaVersion != testpolicy.ExecutionContractSchemaVersion {
		t.Fatalf("schema = %d", contract.SchemaVersion)
	}
	ids := map[string]testpolicy.Group{}
	for _, group := range contract.Groups {
		ids[group.ID] = group
		if group.Phase == "" || group.EnvironmentMode != "inherit" {
			t.Fatalf("group %s lost phase or inherited environment", group.ID)
		}
		if group.ID != "refusal-register-standard" && group.ID != "fast-static-build" && group.ID != "policy-canary" && group.ID != "adapter-canary" && group.ID != "command-interface-smoke" && group.Phase != "acceptance" {
			t.Fatalf("group %s entered admission outside the reviewed static reproof and canary floor", group.ID)
		}
	}
	for _, id := range []string{"refusal-register-standard", "fast-static-build", "policy-canary", "adapter-canary", "command-interface-smoke"} {
		if ids[id].Phase != "admission" {
			t.Fatalf("%s must be admission", id)
		}
	}
	if ids["go-affected"].PackageSelection != "changed-and-consumers" || string(ids["go-affected"].Tests) != `"all"` {
		t.Fatal("host Go selector must discover every affected package test")
	}
	if !slices.Contains(contract.Always.Standard, "go-batchtest") ||
		!slices.Equal(ids["go-batchtest"].BuildTags, []string{"batchtest"}) ||
		!slices.Equal(ids["go-batchtest"].Packages, []string{"cmd/metasystem"}) {
		t.Fatal("host batch-tag tests disappeared from standard acceptance")
	}
	weakened := contract
	weakened.Groups = append([]testpolicy.Group(nil), contract.Groups...)
	for index := range weakened.Groups {
		if weakened.Groups[index].ID == "go-affected" {
			weakened.Groups[index].PackageSelection = ""
			weakened.Groups[index].Packages = []string{"internal/testpolicy"}
			weakened.Groups[index].Tests = []byte(`["TestContractRejectsMissingNoopAndUnknownFields"]`)
		}
	}
	protected := testpolicy.ProtectedContract(contract, weakened)
	for _, group := range protected.Groups {
		if group.ID == "go-affected" && (group.PackageSelection != "changed-and-consumers" || len(group.Packages) != 0 || string(group.Tests) != `"all"`) {
			t.Fatal("candidate policy narrowed protected package selector")
		}
	}
	assertLegacyHostContractCoverage(t, contract)
}

// The frozen schema-1 fixture is independent of the detached candidate HEAD.
// It captures every old native group and mandatory selection reference.
func assertLegacyHostContractCoverage(t *testing.T, current testpolicy.Contract) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "goal_landing_legacy_testing.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != "0951d3c46e488ca28c13cb25c517bfef54de5836ca0beb8de3cad4859664717a" {
		t.Fatalf("frozen legacy host contract changed: %s", got)
	}
	previous, err := testpolicy.Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	containsAll := func(owner string, actual, required []string) {
		t.Helper()
		for _, name := range required {
			if !slices.Contains(actual, name) {
				t.Errorf("%s dropped mandatory %s", owner, name)
			}
		}
	}
	currentSurfaces := map[string]testpolicy.Surface{}
	for _, surface := range current.Surfaces {
		currentSurfaces[surface.ID] = surface
	}
	for _, old := range previous.Surfaces {
		now, ok := currentSurfaces[old.ID]
		if !ok {
			t.Errorf("legacy surface %s disappeared", old.ID)
			continue
		}
		containsAll(old.ID+" standard", now.Standard, old.Standard)
		containsAll(old.ID+" deep", now.Deep, old.Deep)
		containsAll(old.ID+" critical", now.Critical, old.Critical)
		containsAll(old.ID+" cross-cutting", now.CrossCutting, old.CrossCutting)
	}
	containsAll("always canary", current.Always.Canary, previous.Always.Canary)
	containsAll("always standard", current.Always.Standard, previous.Always.Standard)
	containsAll("unknown", current.Unknown, previous.Unknown)
	containsAll("cadence", current.Cadence, previous.Cadence)
	currentGroups := map[string]testpolicy.Group{}
	for _, group := range current.Groups {
		currentGroups[group.ID] = group
	}
	for _, old := range previous.Groups {
		now, ok := currentGroups[old.ID]
		if !ok {
			t.Errorf("legacy host group %s disappeared", old.ID)
			continue
		}
		containsAll(old.ID+" inputs", now.Inputs, old.Inputs)
		containsAll(old.ID+" packages", now.Packages, old.Packages)
		containsAll(old.ID+" obligations", now.Obligations, old.Obligations)
		var oldTests []string
		if len(old.Tests) != 0 && string(old.Tests) != `"all"` {
			if err := json.Unmarshal(old.Tests, &oldTests); err != nil {
				t.Fatal(err)
			}
		}
		var nowTests []string
		if len(now.Tests) != 0 && string(now.Tests) != `"all"` {
			if err := json.Unmarshal(now.Tests, &nowTests); err != nil {
				t.Fatal(err)
			}
		}
		for _, name := range oldTests {
			requiredNames := []string{name}
			if old.ID == "authority-standard" && name == "TestTemporaryGoalProofUsesTheRealWallClock" {
				// Temporary goal authority keeps both guarantees from the retired
				// wall-clock test: wrapper validation and deterministic time policy.
				// Only this exact legacy name in this group may use replacements.
				requiredNames = []string{
					"TestTemporaryGoalProofWrapperRejectsIncompleteWordPair",
					"TestTemporaryGoalProofEnforcesReviewExpiryAndHorizon",
				}
			}
			containsAll(old.ID+" tests", nowTests, requiredNames)
		}
		old.Inputs, now.Inputs = nil, nil
		old.Packages, now.Packages = nil, nil
		old.Tests, now.Tests = nil, nil
		old.Requires, now.Requires = nil, nil
		old.Phase, now.Phase = "", ""
		old.EnvironmentMode, now.EnvironmentMode = "", ""
		old.Resources, now.Resources = testpolicy.GroupResources{}, testpolicy.GroupResources{}
		old.Freshness, now.Freshness = "", ""
		old.FreshnessMaxAgeMS, now.FreshnessMaxAgeMS = nil, nil
		if !reflect.DeepEqual(old, now) {
			t.Errorf("legacy group %s changed its native definition outside the reviewed migration fields", old.ID)
		}
	}
}

func TestGoalLandingGoExpansionCoversPriorChangedAndConsumers(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	module := filepath.Join(root, "metasystem")
	write := func(path, value string) {
		t.Helper()
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(value), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("metasystem/go.mod", "module example.invalid/host\n\ngo 1.27\n")
	write("metasystem/base/base.go", "package base\nconst Value = 1\n")
	write("metasystem/base/base_test.go", "package base\nimport \"testing\"\nfunc TestBase(t *testing.T) {}\n")
	write("metasystem/consumer/consumer.go", "package consumer\n")
	write("metasystem/consumer/consumer_test.go", "package consumer\nimport _ \"example.invalid/host/base\"\n")
	write("metasystem/internal/old/old.go", "package old\n")
	write("metasystem/internal/live/live.go", "package live\n")
	write("metasystem/scripts/shared.sh", "echo original\n")
	testingFixtureGit(t, root, "init", "-q")
	testingFixtureGit(t, root, "add", ".")
	testingFixtureGit(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "base")
	base := strings.TrimSpace(testingFixtureGit(t, root, "rev-parse", "HEAD^{tree}"))
	write("metasystem/base/base.go", "package base\nconst Value = 2\n")
	testingFixtureGit(t, root, "add", ".")
	first := strings.TrimSpace(testingFixtureGit(t, root, "write-tree"))
	contract, err := testpolicy.Load(filepath.Join("..", "..", "testing.json"))
	if err != nil {
		t.Fatal(err)
	}
	firstContract, err := proofrun.ExpandGoPackageGroups(contract, root, base, first)
	if err != nil {
		t.Fatal(err)
	}
	firstA := concreteGoGroup(t, firstContract, "./base")
	firstConsumer := concreteGoGroup(t, firstContract, "./consumer")
	if firstA.ID == firstConsumer.ID {
		t.Fatal("distinct package groups share an id")
	}
	if !slices.Contains(firstA.Inputs, "metasystem/base/**") ||
		!slices.Contains(firstA.Inputs, "metasystem/scripts/**") ||
		!slices.Contains(firstConsumer.Inputs, "metasystem/consumer/**") || !slices.Contains(firstConsumer.Inputs, "metasystem/base/**") {
		t.Fatalf("concrete groups omitted package/import directories: A=%v consumer=%v", firstA.Inputs, firstConsumer.Inputs)
	}
	write("metasystem/base/unstaged-public-input.txt", "new input outside staged tree\n")
	if err := checkDeliveryInputParity(gittree.Workspace{Dir: root}, first, "metasystem/", "testing.json", firstContract,
		testpolicy.Plan{SelectedGroups: []string{firstA.ID}}); err == nil || !strings.Contains(err.Error(), "delivery candidate differs") {
		t.Fatalf("unstaged new package file did not trigger exact input parity: %v", err)
	}
	if err := os.Remove(filepath.Join(module, "base", "unstaged-public-input.txt")); err != nil {
		t.Fatal(err)
	}
	write("metasystem/added/added.go", "package added\n")
	if err := os.Remove(filepath.Join(module, "internal", "old", "old.go")); err != nil {
		t.Fatal(err)
	}
	testingFixtureGit(t, root, "add", "-A")
	second := strings.TrimSpace(testingFixtureGit(t, root, "write-tree"))
	selected, err := gopackages.Select(module, base, second)
	if err != nil {
		t.Fatal(err)
	}
	// Frozen old-gate behavior for this fixture: deletion selects the nearest
	// surviving ancestor wildcard; test-only imports make consumer depend on
	// base. The direct join-gate runner is retired after schema-2 cutover.
	if !slices.Equal(selected.Changed, []string{"./added", "./base", "./internal/..."}) ||
		!slices.Equal(selected.Dependents, []string{"./consumer"}) {
		t.Fatalf("changed/reverse package closure drifted: changed=%v dependents=%v", selected.Changed, selected.Dependents)
	}
	old, err := batch.SelectUnitPackages(module, base, second)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(selected.Changed, old.Changed) || !slices.Equal(selected.Dependents, old.Dependents) {
		t.Fatalf("new selection changed=%v dependents=%v; prior selector changed=%v dependents=%v", selected.Changed, selected.Dependents, old.Changed, old.Dependents)
	}
	priorConsumers, err := batch.ReverseDependents(module, second, selected.Changed)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(selected.Dependents, priorConsumers) {
		t.Fatalf("reverse consumers = %v, prior = %v", selected.Dependents, priorConsumers)
	}
	secondContract, err := proofrun.ExpandGoPackageGroups(contract, root, base, second)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(firstA, concreteGoGroup(t, secondContract, "./base")) {
		t.Fatal("adding package B changed reusable package A definition")
	}
	for _, pkg := range []string{"./base", "./consumer", "./added", "./internal/live"} {
		concreteGoGroup(t, secondContract, pkg)
	}
	for _, pkg := range selected.Packages {
		concreteGoGroup(t, secondContract, pkg)
	}
	for _, group := range secondContract.Groups {
		if group.PackageSelection == "" && len(group.Packages) == 1 && group.Packages[0] == "./internal/old" && strings.HasPrefix(group.ID, "go-affected/") {
			t.Fatal("deleted package remained in current test inventory")
		}
	}
	proof := proofrun.TestRunRequest{ProjectRoot: root, CandidateTree: first, Contract: firstContract,
		Plan: testpolicy.Plan{SelectedGroups: []string{firstA.ID}}, JudgeKey: "fixed-judge", BehaviorPolicyDigest: "fixed-behavior", Environment: os.Environ()}
	firstIdentities, _, _, err := proofrun.PrepareGroupExecutionIdentities(t.Context(), proof)
	if err != nil {
		t.Fatal(err)
	}
	proof.CandidateTree, proof.Contract = second, secondContract
	secondIdentities, _, _, err := proofrun.PrepareGroupExecutionIdentities(t.Context(), proof)
	if err != nil {
		t.Fatal(err)
	}
	if firstIdentities[firstA.ID] != secondIdentities[firstA.ID] {
		t.Fatal("unrelated package B invalidated package A's real execution identity")
	}
	write("metasystem/scripts/shared.sh", "echo changed\n")
	testingFixtureGit(t, root, "add", "metasystem/scripts/shared.sh")
	sharedTree := strings.TrimSpace(testingFixtureGit(t, root, "write-tree"))
	sharedContract, err := proofrun.ExpandGoPackageGroups(contract, root, base, sharedTree)
	if err != nil {
		t.Fatal(err)
	}
	proof.CandidateTree, proof.Contract = sharedTree, sharedContract
	sharedIdentities, _, _, err := proofrun.PrepareGroupExecutionIdentities(t.Context(), proof)
	if err != nil {
		t.Fatal(err)
	}
	if firstIdentities[firstA.ID] == sharedIdentities[firstA.ID] {
		t.Fatal("shared runtime script changed without invalidating package A evidence")
	}
	write("metasystem/scripts/shared.sh", "echo original\n")
	testingFixtureGit(t, root, "add", "metasystem/scripts/shared.sh")
	write("metasystem/base/base.go", "package base\nconst Value = 3\n")
	testingFixtureGit(t, root, "add", "metasystem/base/base.go")
	third := strings.TrimSpace(testingFixtureGit(t, root, "write-tree"))
	thirdContract, err := proofrun.ExpandGoPackageGroups(contract, root, base, third)
	if err != nil {
		t.Fatal(err)
	}
	proof.CandidateTree, proof.Contract = third, thirdContract
	thirdIdentities, _, _, err := proofrun.PrepareGroupExecutionIdentities(t.Context(), proof)
	if err != nil {
		t.Fatal(err)
	}
	if firstIdentities[firstA.ID] == thirdIdentities[firstA.ID] {
		t.Fatal("real package A input change retained package A's execution identity")
	}
	write("metasystem/go.mod", "module example.invalid/host\n\ngo 1.27\n// changed module manifest\n")
	testingFixtureGit(t, root, "add", "metasystem/go.mod")
	manifestTree := strings.TrimSpace(testingFixtureGit(t, root, "write-tree"))
	manifestSelection, err := gopackages.Select(module, third, manifestTree)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(manifestSelection.Packages, []string{"./added", "./base", "./consumer", "./internal/live"}) {
		t.Fatalf("module manifest change must cover every current package, got %v", manifestSelection.Packages)
	}
}

func TestGoalLandingGoAssetOnlyChangeExpandsConcretePackage(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for path, content := range map[string]string{
		"metasystem/go.mod":                "module example.invalid/assets\n\ngo 1.27\n",
		"metasystem/app/app.go":            "package app\n",
		"metasystem/app/testdata/data.txt": "original\n",
	} {
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	testingFixtureGit(t, root, "init", "-q")
	testingFixtureGit(t, root, "add", ".")
	testingFixtureGit(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "base")
	base := strings.TrimSpace(testingFixtureGit(t, root, "rev-parse", "HEAD^{tree}"))
	asset := filepath.Join(root, "metasystem", "app", "testdata", "data.txt")
	if err := os.WriteFile(asset, []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testingFixtureGit(t, root, "add", "metasystem/app/testdata/data.txt")
	tree := strings.TrimSpace(testingFixtureGit(t, root, "write-tree"))
	contract, err := testpolicy.Load(filepath.Join("..", "..", "testing.json"))
	if err != nil {
		t.Fatal(err)
	}
	expanded, err := proofrun.ExpandGoPackageGroups(contract, root, base, tree)
	if err != nil {
		t.Fatal(err)
	}
	group := concreteGoGroup(t, expanded, "./app")
	if !slices.Contains(group.Inputs, "metasystem/app/**") {
		t.Fatalf("asset-only group lacks its native input directory: %+v", group)
	}
}

func TestGoalLandingGoManifestSelectionCoversCurrentPackages(t *testing.T) {
	t.Parallel()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	workspace := gittree.Workspace{Dir: root}
	base, err := workspace.TreeOf("HEAD")
	if err != nil {
		t.Fatal(err)
	}
	detached, err := workspace.NewDetachedWorktree(base)
	if err != nil {
		t.Fatal(err)
	}
	defer detached.Close()
	module := filepath.Join(detached.Workspace().Dir, "metasystem")
	modPath := filepath.Join(module, "go.mod")
	modBytes, err := os.ReadFile(modPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(modPath, append(modBytes, []byte("\n// host inventory probe\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	testingFixtureGit(t, detached.Workspace().Dir, "add", "metasystem/go.mod")
	candidate := strings.TrimSpace(testingFixtureGit(t, detached.Workspace().Dir, "write-tree"))
	selection, err := gopackages.Select(filepath.Join(root, "metasystem"), base, candidate)
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command("go", "list", "./...")
	command.Dir = module
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("list current Go packages: %v: %s", err, output)
	}
	var want []string
	for _, imported := range strings.Fields(string(output)) {
		pkg := "."
		if imported != selection.ModulePath {
			pkg = "./" + strings.TrimPrefix(imported, selection.ModulePath+"/")
		}
		want = append(want, pkg)
	}
	slices.Sort(want)
	if !slices.Equal(selection.Packages, want) {
		t.Fatalf("current package inventory differs from native go list: selected=%v native=%v", selection.Packages, want)
	}
}

func concreteGoGroup(t *testing.T, contract testpolicy.Contract, pkg string) testpolicy.Group {
	t.Helper()
	for _, group := range contract.Groups {
		if strings.HasPrefix(group.ID, "go-affected/") && slices.Equal(group.Packages, []string{pkg}) {
			if !slices.Contains(contract.Always.Standard, group.ID) {
				t.Fatalf("%s is not selected for delivery", pkg)
			}
			return group
		}
	}
	t.Fatalf("missing concrete Go group for %s", pkg)
	return testpolicy.Group{}
}
