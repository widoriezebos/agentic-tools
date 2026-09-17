//go:build !batchtest

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestBatchVerbsUnavailableWithoutFilesystemWrites(t *testing.T) {
	root := t.TempDir()
	t.Setenv("METASYSTEM_BATCH_CAPABILITIES", strings.Join([]string{
		string(recordStoreHistoryAndProberSeam), string(trunkRedLedgerOwner),
	}, ","))
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
		"TestBatchProofUnionUsesRealMemberRiskSelection",
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
	t.Setenv("METASYSTEM_BATCH_CAPABILITIES", string(recordStoreHistoryAndProberSeam))
	command := exec.Command("go", "test", "-count=1", "-run",
		"^TestBatchProductionRegistryContainsOnlyBuiltUnits$", "./cmd/metasystem")
	command.Dir = "../.."
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("runtime input registered a batch capability: %v\n%s", err, output)
	}
}

func TestBatchProductionRegistryContainsOnlyBuiltUnits(t *testing.T) {
	if os.Getenv("GO_WANT_BATCH_RUNTIME_INPUT_HELPER") != "1" {
		return
	}
	want := map[batchCapability]bool{
		ownerVerbAndTick: true, productionSupervisorTakeover: true,
		proofPlanningAndTipLaunch: true, revisionBoundAdmissionAndDiagnosticHeadroom: true,
		freshBaseDiagnosis: true, ejectionAndRedScheduling: true, prefixReceipts: true,
		landingTransportHelpers: true, atomicSeriesAndRecovery: true, waitStatusDocsAndInventory: true,
	}
	if len(compiledBatchCapabilities) != len(want) {
		t.Fatalf("production registered %d batch capabilities, want %d", len(compiledBatchCapabilities), len(want))
	}
	for capability := range compiledBatchCapabilities {
		if !want[capability] {
			t.Fatalf("production registered unbuilt capability %s", capability)
		}
	}
}
