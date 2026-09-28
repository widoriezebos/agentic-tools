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
		if group.ID != "refusal-register-standard" && group.ID != "fast-static-build" && group.ID != "policy-canary" && group.ID != "adapter-canary" && group.ID != "command-interface-smoke" && group.ID != "verb-ratchet" && group.ID != "delegation-layering" && group.Phase != "acceptance" {
			t.Fatalf("group %s entered admission outside the reviewed static reproof and canary floor", group.ID)
		}
	}
	for _, id := range []string{"refusal-register-standard", "fast-static-build", "policy-canary", "adapter-canary", "command-interface-smoke", "verb-ratchet", "delegation-layering"} {
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
// retiredWithDeletedVerb names legacy mandatory tests (group/test) whose only
// subject was an internal verb deleted for having no caller
// (plans/designs/verbs-object-action.md 6.1). Each maps to that verb; nothing
// else may leave the legacy floor this way.
var retiredWithDeletedVerb = map[string]string{
	"context-standard/TestRuntimeContextSampleVerb":  "runtime context-sample",
	"launch-standard/TestPackCheckVerbPrintsOneLine": "launch pack-check",
	"launch-standard/TestUnitRunPrintsOneLine":       "unit run",
	"launch-standard/TestUnitFamilyIsRegistered":     "unit run",
}

// retiredWithDeletedScript names legacy mandatory tests (group/test) whose only
// subject was a script deleted by its provider transition (6.4) and maps each to
// the test that now carries the behavior, which must be in the contract.
var retiredWithDeletedScript = map[string]string{
	"proof-standard/TestGoGateCopiedRootStopsAfterOneUnauthorizedRelaunch":                "TestGateRefusesAnUnauthorizedRelaunchedChild",
	"proof-standard/TestGoGateRelaunchUsesBuiltEngineForWorkerAuthorization":              "TestGateRelaunchesAStandaloneRunUnderItsRetainedProofOwner",
	"batch-buildcd-standard/TestGoGateFastModeRunsParallelRatchetBesideDependencyRatchet": "TestStaticRatchetsRefuseBeforeAnyToolRuns",
}

// providerTransitions names legacy groups whose command moved to a new
// provider under the two-step transition (plans/designs/verbs-object-action.md
// 6.4): the group keeps its id so a candidate's proof runs the base's command,
// and holds exactly the new provider's definition. Nothing else may leave the
// legacy definition this way.
var providerTransitions = map[string]func(testpolicy.Group) bool{
	// go-gate.sh --fast became the Go bootstrap's static leaf.
	"fast-static-build": func(group testpolicy.Group) bool {
		return group.Adapter == "command" && slices.Equal(group.Argv, []string{"go", "run", "./cmd/devgate", "static"}) &&
			slices.Contains(group.Obligations, "gate-integrity")
	},
	// witness-gate-fixtures.sh became the devgate witness and arming tests.
	"section/witness-gate-fixtures": func(group testpolicy.Group) bool {
		return group.Adapter == "go" && slices.Equal(group.Packages, []string{"cmd/devgate"}) && len(group.Tests) > 2 &&
			slices.Contains(group.Obligations, "test-execution-integrity")
	},
}

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
	// A retired fixture section may leave the contract only when every one of
	// its scenarios was ported to named Go tests that a replacement group,
	// itself on the cadence, discovers. Only these exact sections qualify.
	// Empty tests means the replacement group's own named tests are the
	// port.
	retiredSections := map[string]struct {
		replacement string
		tests       []string
	}{
		// second-session.sh moved into internal/seat/launch (verbs-object-action U3).
		"section/second-session-fixtures": {replacement: "launch-machine-standard", tests: []string{
			"TestTheManifestIsTheAdaptersDeclaredContract",
			"TestSecondSessionCreatesAnIsolatedArmedWorktree",
			"TestSecondSessionMintsANameAndRefusesUnlawfulOnes",
			"TestSecondSessionStopsWhenArmingFails",
		}},
		// goal-cli-fixtures.sh moved into the goal CLI Go tests (verbs-object-action U7b part 2).
		"section/goal-cli-fixtures": {replacement: "goal-cli-standard"},
	}
	currentGroups := map[string]testpolicy.Group{}
	for _, group := range current.Groups {
		currentGroups[group.ID] = group
	}
	// retiredName is a legacy selection reference after its section's port:
	// the replacement group once the section left the contract.
	retiredNames := func(names []string) []string {
		out := make([]string, 0, len(names))
		for _, name := range names {
			if retired, ok := retiredSections[name]; ok {
				if _, still := currentGroups[name]; !still {
					name = retired.replacement
				}
			}
			out = append(out, name)
		}
		return out
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
		containsAll(old.ID+" standard", now.Standard, retiredNames(old.Standard))
		containsAll(old.ID+" deep", now.Deep, retiredNames(old.Deep))
		containsAll(old.ID+" critical", now.Critical, retiredNames(old.Critical))
		containsAll(old.ID+" cross-cutting", now.CrossCutting, retiredNames(old.CrossCutting))
	}
	containsAll("always canary", current.Always.Canary, previous.Always.Canary)
	containsAll("always standard", current.Always.Standard, retiredNames(previous.Always.Standard))
	containsAll("unknown", current.Unknown, previous.Unknown)
	requiredCadence := make([]string, 0, len(previous.Cadence))
	for _, name := range previous.Cadence {
		if retired, ok := retiredSections[name]; ok {
			if _, still := currentGroups[name]; !still {
				name = retired.replacement
			}
		}
		requiredCadence = append(requiredCadence, name)
	}
	containsAll("cadence", current.Cadence, requiredCadence)
	for _, old := range previous.Groups {
		now, ok := currentGroups[old.ID]
		if retired, isRetired := retiredSections[old.ID]; !ok && isRetired {
			replacement, present := currentGroups[retired.replacement]
			if !present {
				t.Errorf("retired section %s has no replacement group %s", old.ID, retired.replacement)
				continue
			}
			probe := replacement
			if len(retired.tests) != 0 {
				if probe.Tests, err = json.Marshal(retired.tests); err != nil {
					t.Fatal(err)
				}
			} else if string(probe.Tests) == `"all"` || len(probe.Tests) == 0 {
				t.Errorf("retired section %s: replacement %s names no ported tests", old.ID, retired.replacement)
				continue
			}
			probeContract := testpolicy.Contract{SchemaVersion: current.SchemaVersion, Groups: []testpolicy.Group{probe}}
			if err := proofrun.CheckNativeDiscovery(t.Context(), projectRoot, installation, probeContract, os.Environ()); err != nil {
				t.Errorf("retired section %s: replacement %s does not discover its ported tests: %v", old.ID, retired.replacement, err)
			}
			continue
		}
		if !ok {
			t.Errorf("legacy host group %s disappeared", old.ID)
			continue
		}
		if transitioned, moved := providerTransitions[old.ID]; moved {
			if !transitioned(now) {
				t.Errorf("legacy group %s left its definition without holding its new provider's", old.ID)
			}
			continue
		}
		requiredInputs := old.Inputs
		if old.ID == "hook-start-audit-standard" {
			// U4 retired the shell hook bed and the sourced degraded-form
			// renderer; both moved into internal/hooks, which the group names.
			// Only these exact retired inputs may be replaced, and only by it.
			retired := map[string]bool{
				"metasystem/scripts/agents/supervision-hook-fixtures.sh": true,
				"metasystem/scripts/agents/stop-degraded-forms.sh":       true,
			}
			requiredInputs = nil
			for _, input := range old.Inputs {
				if !retired[input] {
					requiredInputs = append(requiredInputs, input)
				}
			}
			requiredInputs = append(requiredInputs, "metasystem/internal/hooks/**")
		}
		containsAll(old.ID+" inputs", now.Inputs, requiredInputs)
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
			if verb, retired := retiredWithDeletedVerb[old.ID+"/"+name]; retired {
				t.Logf("%s %s retired with the deleted verb %s", old.ID, name, verb)
				continue
			}
			if replacement, retired := retiredWithDeletedScript[old.ID+"/"+name]; retired {
				carried := false
				for _, group := range current.Groups {
					var names []string
					if json.Unmarshal(group.Tests, &names) == nil && slices.Contains(names, replacement) {
						carried = true
					}
				}
				if !carried {
					t.Errorf("%s %s retired with its deleted script, but %s is not in the contract", old.ID, name, replacement)
				}
				continue
			}
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
			if old.ID == "landing-command-standard" && name == "TestProductionJoinGateSelectsPackagesAgainstTheLandingRoot" {
				// The replacement tests retain deleted-package selection and the
				// configured contract and project-root guarantees for this group.
				requiredNames = []string{
					"TestDeletedGoPackagesSelectNearestExistingDirectory",
					"TestLandingBatchProtectedTestsUseConfiguredContractAndProjectCWD",
				}
			}
			if old.ID == "test-environment-standard" && name == "TestAgedSweepRemovesFixtureRecordFiles" {
				// Age alone no longer authorizes deletion: a sweep may remove
				// sidecars only after an authenticated dead home and a settled
				// inherited lease and custodian, and must retain failed settlements.
				requiredNames = []string{
					"TestSweepKeepsSidecarsWithoutAuthenticatedDeadHome",
					"TestRegistrySidecarCleanupWaitsForInheritedLeaseAndCustodian",
					"TestRegistrySidecarCleanupRetainsFailedSettlement",
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
