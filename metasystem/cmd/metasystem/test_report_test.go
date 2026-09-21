package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestTestReportReadsValidatedResultWithoutExecutingProof(t *testing.T) {
	zero, admissionMaximum := 0, 0
	digest := strings.Repeat("a", 64)
	result := proofrun.TestResult{SchemaVersion: proofrun.TestResultSchemaVersion, AttemptID: "attempt-1",
		WorkerPolicyVersion: proofrun.TestWorkerPolicyVersion, Workers: 1, AdmissionMaximum: &admissionMaximum,
		Purpose: testpolicy.PurposeDiagnostic, RequestedMode: testpolicy.ModeAuto,
		RequiredMode: testpolicy.ModeAuto, ExecutedMode: testpolicy.ModeCanary,
		ProjectRoot: "/project", BaseCommit: strings.Repeat("b", 40), CandidateTree: strings.Repeat("c", 40),
		ContractDigest: digest, BaseContractDigest: digest, PolicyEngineDigest: digest,
		BehaviorPolicyDigest: digest, PlanDigest: digest, SelectedGroups: []string{"unit"},
		LaunchCounts: proofrun.LaunchCounts{Test: 1, CountsComplete: true},
		Cost:         proofrun.TestCost{DeclaredTargetMS: 2000, ActualDurationMS: 700, ExecutionDurationMS: 650},
		Groups: []proofrun.GroupResult{{ID: "unit", Kind: "unit", InputManifest: []string{"src/app.go"}, IdentityVersion: proofrun.GroupExecutionIdentityVersion,
			ExecutionIdentity: digest, Status: "passed", NativeLaunched: true, CollectionComplete: true,
			NativeExitStatus: &zero, DurationMS: 600}}}
	result.RecomputeDelivery()
	path := filepath.Join(t.TempDir(), "result.json")
	if err := writePrivateJSON(path, result); err != nil {
		t.Fatal(err)
	}
	output, code := captureStdout(t, func() int { return runTestReport([]string{"--result", path, "--expensive-ms", "500"}) })
	if code != 0 {
		t.Fatalf("test report refused valid retained result: %s", output)
	}
	var report proofrun.TestCostSummary
	if err := json.Unmarshal([]byte(output), &report); err != nil || report.NativeTestGroups != 1 ||
		report.NativeExpensiveGroups != 1 || report.WaitDurationMS != nil || report.MetadataPreparationDurationMS != nil {
		t.Fatalf("test report output = %s, error = %v", output, err)
	}
	result.LaunchCounts.CountsComplete = false
	if err := writePrivateJSON(path, result); err != nil {
		t.Fatal(err)
	}
	_, code = captureStdout(t, func() int { return runTestReport([]string{"--result", path, "--expensive-ms", "500"}) })
	if code == 0 {
		t.Fatal("test report accepted an invalid retained result")
	}
}
