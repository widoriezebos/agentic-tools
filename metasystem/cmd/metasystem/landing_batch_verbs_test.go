//go:build !batchtest

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
)

func TestBatchVerbsUnavailableWithoutFilesystemWrites(t *testing.T) {
	root := t.TempDir()
	missing := requiredBatchCapabilities[0]
	delete(compiledBatchCapabilities, missing)
	defer func() { compiledBatchCapabilities[missing] = struct{}{} }()
	commands := [][]string{
		{"landing", "batch", "join"}, {"landing", "batch", "status"},
		{"landing", "batch", "withdraw"}, {"landing", "batch", "owner"},
		{"landing", "batch", "tick"}, {"landing", "batch", "wait"},
		{"goal", "handover"},
	}
	for _, command := range commands {
		args := append(append([]string{}, command...), "--root", root)
		stderr, code := captureStderr(t, func() int { return dispatch(args) })
		if code != 1 || !strings.Contains(stderr, "BATCH_UNAVAILABLE") {
			t.Errorf("%v = code %d, stderr %q", command, code, stderr)
		}
		entries, err := os.ReadDir(root)
		if err != nil || len(entries) != 0 {
			t.Fatalf("%v touched the filesystem: entries=%v err=%v", command, entries, err)
		}
	}
}

func TestBatchTaggedCapabilityWitnessExecutesInProof(t *testing.T) {
	names := []string{
		"TestBatchCapabilitiesGate", "TestGoalHandoverRequiresCompleteInputs", "TestGoalHandoverTargetAuthenticationFailsClosed",
		"TestBatchJoinSpawnsOneOwner", "TestBatchOwnerHoldsTheLease", "TestBatchOwnerWiringBound",
		"TestBatchOwnerInspectionUsesInjectedProberAndKeepsReadErrorsUnknown", "TestBatchProductionReturnTargetClassifiesCustody",
		"TestBatchProofCoversTheUnion", "TestBatchProofAcceptsReusableSuccess", "TestBatchProofRearmsBaseBeforePlanningEvenWhenTreeMatches",
		"TestBatchSupervisorTakeoverRebindsJoinedClaims", "TestGoalHandoverTargetRootFlagFlows", "TestLandingBatchJoinVerbPublishesOutsideFlock",
		"TestBatchProductionReturnAndForwardHandoverArguments", "TestBatchTrunkRedLedgerOwnerCapability",
		"TestBatchProofUnionUsesRealMemberRiskSelection", "TestBatchProofDurableUnionRefusalSkipsSecondRearm",
		"TestBatchProofRefusalTransitions",
	}
	command := exec.Command("go", "test", "-count=1", "-tags", "batchtest",
		"-json", "-run", "^("+strings.Join(names, "|")+")$", "./cmd/metasystem")
	command.Dir = "../.."
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("tagged batch capability witness: %v\n%s", err, output)
	}
	if err := requireTaggedBatchPasses(output, names); err != nil {
		t.Fatal(err)
	}
}

func requireTaggedBatchPasses(output []byte, names []string) error {
	passed := map[string]bool{}
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		var event struct {
			Action string `json:"Action"`
			Test   string `json:"Test"`
		}
		if json.Unmarshal(scanner.Bytes(), &event) == nil && event.Action == "pass" && event.Test != "" {
			passed[event.Test] = true
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	for _, name := range names {
		if !passed[name] {
			return fmt.Errorf("tagged batch capability witness %s did not report pass", name)
		}
	}
	return nil
}

func TestBatchTaggedCapabilityWitnessRejectsMissingTest(t *testing.T) {
	line := []byte(`{"Action":"pass","Test":"TestPresent"}` + "\n")
	if err := requireTaggedBatchPasses(line, []string{"TestPresent", "TestMissing"}); err == nil || !strings.Contains(err.Error(), "TestMissing") {
		t.Fatalf("missing tagged witness error=%v", err)
	}
}

func TestBatchRuntimeInputsCannotRegisterCapability(t *testing.T) {
	t.Setenv("GO_WANT_BATCH_RUNTIME_INPUT_HELPER", "1")
	t.Setenv("METASYSTEM_BATCH_CAPABILITIES", "runtimeOnlyCapability")
	command := exec.Command("go", "test", "-count=1", "-run",
		"^TestBatchProductionRegistryComplete$", "./cmd/metasystem")
	command.Dir = "../.."
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("runtime input registered a batch capability: %v\n%s", err, output)
	}
}

func TestBatchProductionRegistryComplete(t *testing.T) {
	want := []batchCapability{
		recordStoreHistoryAndProberSeam, chainReaderIdentityUniquenessAndTransport, joinGate, assemblyConflictCeilingAndSeal,
		handedOverClaimCardinality, fieldCompleteHandover, serializedJoinPublication, crashSafeTerminalReturn,
		censusOnTheInjectedProber, boundedRolloutConfiguration, fifoLockAndStartRule, ownerVerbAndTick,
		productionSupervisorTakeover, registerPreservingFastForward, proofPlanningAndTipLaunch,
		revisionBoundAdmissionAndDiagnosticHeadroom, freshBaseDiagnosis, ejectionAndRedScheduling, trunkRedLedgerOwner,
		prefixReceipts, landingTransportHelpers, atomicSeriesAndRecovery, waitStatusDocsAndInventory,
	}
	if !slices.Equal(requiredBatchCapabilities[:], want) {
		t.Fatalf("required batch capability registry=%v, want literal production inventory %v", requiredBatchCapabilities, want)
	}
	if len(compiledBatchCapabilities) != len(requiredBatchCapabilities) {
		t.Fatalf("production registered %d batch capabilities, want %d", len(compiledBatchCapabilities), len(requiredBatchCapabilities))
	}
	for _, capability := range requiredBatchCapabilities {
		if _, ok := compiledBatchCapabilities[capability]; !ok {
			t.Fatalf("production did not register required capability %s", capability)
		}
	}
	root := syncedClaimedGoalFixture(t)
	productionOwner, err := productionBatchLedgerOwner(root)
	if err != nil || !isLedgerTrunkRedOwner(productionOwner) {
		t.Fatalf("production trunk-red owner type %T error=%v, want ledger adapter", productionOwner, err)
	}
}
