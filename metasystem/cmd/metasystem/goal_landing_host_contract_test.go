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
	installation, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	projectRoot := filepath.Dir(installation)
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
		nowSelectsAll := string(now.Tests) == `"all"`
		var nowTests []string
		if len(now.Tests) != 0 && !nowSelectsAll {
			if err := json.Unmarshal(now.Tests, &nowTests); err != nil {
				t.Fatal(err)
			}
		}
		var automaticallyCovered []string
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
			if nowSelectsAll {
				automaticallyCovered = append(automaticallyCovered, requiredNames...)
			} else {
				containsAll(old.ID+" tests", nowTests, requiredNames)
			}
		}
		if len(automaticallyCovered) != 0 {
			probe := now
			probe.Tests, err = json.Marshal(automaticallyCovered)
			if err != nil {
				t.Fatal(err)
			}
			probeContract := testpolicy.Contract{SchemaVersion: current.SchemaVersion, Groups: []testpolicy.Group{probe}}
			if err := proofrun.CheckNativeDiscovery(t.Context(), projectRoot, installation, probeContract, os.Environ()); err != nil {
				t.Errorf("%s automatic test selection dropped mandatory legacy names: %v", old.ID, err)
			}
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

func TestGitAdapterGoManifestInventoryMatchesNativeGoList(t *testing.T) {
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
