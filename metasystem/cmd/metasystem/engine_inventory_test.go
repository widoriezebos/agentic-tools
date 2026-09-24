package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gopackages"
)

var engineBindingStandardTests = []string{
	"TestAmbientTrustedPolicyDecisionCannotBypassRetainedEngine",
	"TestBaseMovedToAnEngineChangeReArmsOnce",
	"TestBaseMovedUnderTheRunRestartsPreparationOnce",
	"TestCandidateBuiltCommitPassesDispatchSkewPreflight",
	"TestCandidateEngineArtifactReuseValidatesBytesAndBuildInputs",
	"TestCandidateEngineBuildEnvironmentIsPinnedWithoutDroppingProofCustody",
	"TestCandidateEngineBuildFailureCannotFallBackToPolicyEngine",
	"TestCandidateEngineIsBuiltFromCandidateTreeAndBindsExecutionIdentity",
	"TestCandidateEngineNativeGitSerializationPinsHeadSourceAndCleanup",
	"TestCandidateEngineNativePhysicalCommandOriginIncludesEventHeldColdBuild",
	"TestCandidateEngineTrimpathIsReproducibleAcrossMaterializationDirectories",
	"TestCauseOutcomesRenderDeclaredCommandCount",
	"TestDecisionMismatchNamesTheField",
	"TestEngineBindingWitnessInventory",
	"TestEngineRefusalsNameTheirCause",
	"TestFastForwardBlockerRemediesPreserveCheckout",
	"TestFailedCandidateEngineBuildCannotFillArtifactCache",
	"TestGoalLandingGoManifestSelectionCoversCurrentPackages",
	"TestFrozenNegativeProbeResponseIsLegacyOnlyForActualInvalidResult",
	"TestFrozenPolicyProbeRefusalNamesResultPathPredicate",
	"TestFrozenPublicVersionOneCorpusNormalizesSupportedSourceLayouts",
	"TestFrozenPublicVersionOneCorpusRunsAllSixCasesThroughFirstTransitionWorker",
	"TestFrozenPublicVersionOneProtectionCorpusIsComplete",
	"TestFrozenPublicVersionOneSelectionProbesRunAgainstCandidateExecutable",
	"TestFrozenWorkerProbesTraverseActiveEmptyLegacyAndForgedReuse",
	"TestFrozenWorkerProbePathsCanonicalizeRootOnce",
	"TestFrozenWorkerProbeReaderAcceptsCandidateGroupFields",
	"TestLandedRearmDecidesFromTheThreeFacts",
	"TestLandedRearmFactsFailOnARepositoryFailureInsteadOfJudgingHeadUnlanded",
	"TestLandedRearmFastForwardsRebuildsAndReArms",
	"TestLandedRearmJudgmentIsProgressBounded",
	"TestLandedRearmRebuildWaitsForHostSlotAndClearsCustody",
	"TestLandedRearmReadsTheCheckoutAgainstItsRemote",
	"TestLandedRearmRefusesARepositoryFailureReadingTheLandingRefWithGitDetail",
	"TestLandedRearmRefusesARepositoryFailureResolvingCheckoutHeadWithGitDetail",
	"TestLandedRearmRefusesAFastForwardBlockedByDirtyLedgers",
	"TestLandedRearmRefusesAncestorPathCollisionsBeforeFastForward",
	"TestLandedRearmRefusesAStalledTipCompareByName",
	"TestNonDescendantPolicyBaseRefusesDecisionMismatch",
	"TestPolicyChildNeverFetchesOrReArms",
	"TestProtectedCoverageFloorCannotFallOrDisappear",
	"TestPublicTestingPlanAmbientTrustedPolicyDecisionCannotBypassRetainedEngineNativeGit",
	"TestPublishTestingResultSpillsOverTheBound",
	"TestPublishTestingResultUnderTheBoundIsUnchanged",
	"TestRefusalQuotesPathFacts",
	"TestRunOwnerSurvivesTestingWorkerFilters",
	"TestSecondBaseMoveRefusesBaseMoved",
	"TestTestListCheckPlanAndVerifyWithoutLaunching",
	"TestTestPlanReArmsOnALandedEngine",
	"TestTestRunRearmsOnALandedEngine",
	"TestTestingPlanAdoptsCandidateFallbackOnlyWhenBaseHasNone",
	"TestTestingCommandAdmissionSamplesAfterPreparationAndAtForcedFallback",
	"TestTestingSelectionCarriesDeliveryAllGroupsOnlyForExecution",
	"TestTestWorkerBuildIdentityCompatibilityDoorIsPolicyProbeOnly",
	"TestTrustedPolicyEngineIsRequiredWithoutBuildingDuringReadOnlySelection",
	"TestUnenrolledLinkedWorktreeNamesItsMainCheckout",
	"TestUnexplainedPolicyFieldRefusesDecisionMismatch",
	"TestUnprintableBlockerPathsAreCountedNotRendered",
	"TestVerifyDoesNotKeyLegacyCandidateDigestByWholeTreeReceipt",
	"TestVerifyRecoversCandidateDigestFromNewestSufficientAttempt",
	"TestVerifySamplesFreshnessAfterRetainedProofRevalidation",
	"TestDiagnosticsReadersFollowTheCandidatePair",
	"TestBatchPrefixProofControlRootAcceptsALinkedLandingWorktree",
	"TestBatchPrefixProofControlRootMustOwnLinkedExecution",
	"TestBatchPrefixTestingControlRootRetainsAttemptOutsideExecution",
	"TestLinkedWorktreeMainInstallationPreservesInstallationSubdirectory",
	"TestTestRunKeepsProofRecordsUnderTheControlRoot",
}

func TestGoalLandingGoManifestSelectionCoversCurrentPackages(t *testing.T) {
	module, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := os.ReadFile(filepath.Join(module, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	baseManifest := append(bytes.Clone(manifest), []byte("\n// base manifest inventory probe\n")...)
	const (
		baseTree      = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		candidateTree = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		oldBlob       = "cccccccccccccccccccccccccccccccccccccccc"
		newBlob       = "dddddddddddddddddddddddddddddddddddddddd"
	)
	type reply struct {
		args   []string
		output []byte
	}
	ls := func(tree string) []string {
		return []string{"--literal-pathspecs", "ls-tree", "-r", "-z", "--full-tree", tree, "--", "go.mod"}
	}
	entry := func(blob string) []byte {
		return []byte(fmt.Sprintf("100644 blob %s\tgo.mod\x00", blob))
	}
	replies := []reply{
		{ls(baseTree), entry(oldBlob)},
		{[]string{"cat-file", "blob", oldBlob}, baseManifest},
		{ls(candidateTree), entry(newBlob)},
		{[]string{"cat-file", "blob", newBlob}, manifest},
		{[]string{"diff", "--name-only", "-z", "--no-renames", "--no-ext-diff", "--no-textconv", "--ignore-submodules=none", baseTree, candidateTree, "--"}, []byte("go.mod\x00")},
		{ls(baseTree), entry(oldBlob)},
		{ls(candidateTree), entry(newBlob)},
		{ls(candidateTree), entry(newBlob)},
		{[]string{"cat-file", "blob", newBlob}, manifest},
	}
	pins := []string{"-C", module, "-c", "core.fileMode=true", "-c", "diff.noprefix=false", "-c", "diff.mnemonicPrefix=false", "-c", "apply.ignoreWhitespace=no", "-c", "core.logAllRefUpdates=false", "-c", "core.useReplaceRefs=false", "-c", "gc.auto=0", "-c", "maintenance.auto=false"}
	next, opened, closed := 0, 0, 0
	workspace := gittree.Workspace{Dir: module, RawSource: func(request gittree.RawRequest) gittree.RawResult {
		if next >= len(replies) {
			t.Fatalf("unexpected raw Git call: %v", request.Args)
		}
		want := replies[next]
		next++
		if request.Dir != module || !slices.Equal(request.Args, append(slices.Clone(pins), want.args...)) ||
			request.Operation != "git "+strings.Join(want.args, " ") || request.Stdin != nil ||
			!slices.Equal(request.Env, gittree.ScrubbedEnviron()) {
			t.Fatalf("raw Git call %d = %+v; want args %v", next, request, want.args)
		}
		return gittree.RawResult{Stdout: want.output}
	}}
	goFlags := "-mod=readonly -buildvcs=false"
	if inherited := os.Getenv("GOFLAGS"); inherited != "" {
		goFlags = inherited + " " + goFlags
	}
	environment := make([]string, 0, len(os.Environ())+1)
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "GOFLAGS=") {
			environment = append(environment, entry)
		}
	}
	environment = append(environment, "GOFLAGS="+goFlags)
	selection, err := gopackages.SelectWithWorkspaceSnapshot(workspace, baseTree, candidateTree, nil, environment, func(tree string) (string, func() error, error) {
		opened++
		current, readErr := os.ReadFile(filepath.Join(module, "go.mod"))
		if tree != candidateTree || opened != 1 || readErr != nil || !bytes.Equal(current, manifest) {
			t.Fatalf("snapshot tree=%q opens=%d manifest error=%v", tree, opened, readErr)
		}
		return module, func() error {
			closed++
			if closed != 1 {
				t.Fatal("snapshot closed more than once")
			}
			return nil
		}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if next != len(replies) || opened != 1 || closed != 1 {
		t.Fatalf("raw/snapshot closure: calls=%d/%d opened=%d closed=%d", next, len(replies), opened, closed)
	}
	if !slices.Equal(selection.Changed, []string{"./..."}) {
		t.Fatalf("manifest change selected %v, want ./...", selection.Changed)
	}
	command := exec.Command("go", "list", "./...")
	command.Dir, command.Env = module, environment
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("native go list: %v: %s", err, output)
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

func TestEngineBindingWitnessInventory(t *testing.T) {
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, statErr := os.Stat(filepath.Join(root, "go.mod")); statErr == nil {
			break
		}
		parent := filepath.Dir(root)
		if parent == root {
			t.Fatal("could not find metasystem module root")
		}
		root = parent
	}
	files := []string{
		"cmd/metasystem/test_test.go",
		"cmd/metasystem/rearm_on_landed_test.go",
		"cmd/metasystem/engine_refusal_test.go",
		"cmd/metasystem/engine_inventory_test.go",
		"internal/enginecause/cause_test.go",
	}
	actual := map[string]bool{}
	for _, name := range files {
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, filepath.FromSlash(name)), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if ok && function.Recv == nil && function.Name.Name != "TestMain" && strings.HasPrefix(function.Name.Name, "Test") {
				if function.Name.Name == "TestLandedRearmScriptedGitProcess" {
					// The independently selected parent scan excludes this test because it only runs as a subprocess helper.
					continue
				}
				actual[function.Name.Name] = true
			}
		}
	}
	declared := map[string]bool{}
	for _, name := range engineBindingStandardTests {
		if declared[name] {
			t.Errorf("engine-binding-standard declares %s twice", name)
		}
		declared[name] = true
	}
	for name := range actual {
		if !declared[name] {
			t.Errorf("engine-binding-standard does not declare %s", name)
		}
	}
	for name := range declared {
		if !actual[name] {
			t.Errorf("engine-binding-standard declares missing test %s", name)
		}
	}
}
