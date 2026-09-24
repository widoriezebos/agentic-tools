//go:build !batchtest

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
)

func TestBatchVerbsUnavailableWithoutFilesystemWrites(t *testing.T) {
	root := t.TempDir()
	commands := []struct {
		name string
		args []string
		want string
	}{
		{"join", []string{"landing", "batch", "join", "--root", root}, "landing batch join"},
		{"status", []string{"landing", "batch", "status", "--root", root, "unexpected"}, "landing batch status"},
		{"withdraw", []string{"landing", "batch", "withdraw", "--root", root}, "landing batch withdraw"},
		{"owner", []string{"landing", "batch", "owner", "--root", root, "unexpected"}, "landing batch owner"},
		{"tick", []string{"landing", "batch", "tick", "--root", root}, "landing batch tick"},
		{"wait", []string{"landing", "batch", "wait", "--root", root}, "landing batch wait"},
		{"handover", []string{"goal", "handover", "--root", root}, "goal handover needs"},
	}
	for _, command := range commands {
		t.Run(command.name, func(t *testing.T) {
			stderr, code := captureStderr(t, func() int { return dispatch(command.args) })
			if code != 2 || !strings.Contains(stderr, command.want) || strings.Contains(stderr, "BATCH_UNAVAILABLE") {
				t.Errorf("default command = code %d, stderr %q", code, stderr)
			}
		})
	}
	stderr, code := captureStderr(t, func() int { return runLandingBatch([]string{"unknown", "--root", root}) })
	if code != 2 || !strings.Contains(stderr, `unknown verb "unknown"`) {
		t.Fatalf("unknown verb = code %d, stderr %q", code, stderr)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("command validation touched the filesystem: entries=%v err=%v", entries, err)
	}
}

func TestBatchTaggedCapabilityWitnessExecutesInProof(t *testing.T) {
	names := []string{
		"TestGoalHandoverRequiresCompleteInputs", "TestGoalHandoverTargetAuthenticationFailsClosed",
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

func TestBatchRolloutRequiresSealReceiptDiagnosisAndRecovery(t *testing.T) {
	if productionBatchProofDependencies.seal == nil || productionBatchProofDependencies.launch == nil ||
		batchDiagnosisSeams.diagnose == nil || batchLandRecoverPush == nil {
		t.Fatal("seal, proof, diagnosis, and moved-base recovery must be bound in every build")
	}
	seams := batchLandSeams("root", "batch", batch.Record{}, "base", "actor")
	if seams.Prepare == nil || seams.AppendReceipt == nil || seams.Commit == nil || seams.Held == nil ||
		seams.PublishBranch == nil || seams.Push == nil || seams.RecoverPush == nil || seams.Cleanup == nil {
		t.Fatal("receipt composition and atomic series landing must be bound in every build")
	}
}

func TestBatchProductionRegistryComplete(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		data, readErr := os.ReadFile(file)
		if readErr != nil {
			t.Fatal(readErr)
		}
		for _, dead := range []string{"batchCapabilitiesAvailable", "compiledBatchCapabilities", "registerBatchRollout", "BATCH_UNAVAILABLE"} {
			if strings.Contains(string(data), dead) {
				t.Fatalf("production source %s still contains dead registry symbol %s", file, dead)
			}
		}
	}
	wantVerbs := []string{"join", "status", "withdraw", "owner", "tick", "wait"}
	if len(landingBatchVerbs) != len(wantVerbs) {
		t.Fatalf("default batch verbs=%v, want %v", landingBatchVerbs, wantVerbs)
	}
	for _, verb := range wantVerbs {
		if landingBatchVerbs[verb] == nil {
			t.Fatalf("default build does not bind batch verb %s", verb)
		}
	}
	root := t.TempDir()
	facts := batchConfigFacts(root, true, false)
	productionOwner, err := productionBatchLedgerOwnerWithConfig(root, facts.config)
	facts.assertConsumed(t, false)
	if err != nil || !isLedgerTrunkRedOwner(productionOwner) {
		t.Fatalf("production trunk-red owner type %T error=%v, want ledger adapter", productionOwner, err)
	}
}
