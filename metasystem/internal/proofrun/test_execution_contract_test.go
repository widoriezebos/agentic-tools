package proofrun

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestGLEPrerequisiteFailureBlocksOnlyDependents(t *testing.T) {
	root := t.TempDir()
	runTestResultGit(t, root, "init", "-q", "-b", "main")
	runTestResultGit(t, root, "config", "user.name", "fixture")
	runTestResultGit(t, root, "config", "user.email", "fixture@example.invalid")
	writeTestResultScript(t, root, "first", "failed", 24)
	writeTestResultScript(t, root, "second", "failed", 25)
	writeTestResultScript(t, root, "third", "passed", 0)
	writeTestResultScript(t, root, "deep", "passed", 0)
	runTestResultGit(t, root, "add", ".")
	runTestResultGit(t, root, "commit", "-qm", "fixture")
	tree := runTestResultGit(t, root, "rev-parse", "HEAD^{tree}")
	contract, plan := stopFixtureContract(testpolicy.PurposeDelivery)
	contract.SchemaVersion = testpolicy.ExecutionContractSchemaVersion
	for index := range contract.Groups {
		contract.Groups[index].Phase = "acceptance"
		contract.Groups[index].EnvironmentMode = "inherit"
		contract.Groups[index].Inputs = []string{"scripts/" + contract.Groups[index].ID + ".sh"}
	}
	contract.Groups[3].Requires = []string{"first"}
	if err := contract.Validate(); err != nil {
		t.Fatal(err)
	}
	result, status, err := RunTestPlan(context.Background(), TestRunRequest{ProjectRoot: root, CandidateTree: tree, BaseCommit: "HEAD", PolicyBaseCommit: "HEAD",
		Contract: contract, Plan: plan, AttemptID: "attempt", LogRoot: filepath.Join(root, "artifacts", "logs"), Concurrency: 2,
		CandidateEngineDigest: strings.Repeat("a", 64)})
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]GroupResult{}
	for _, group := range result.Groups {
		byID[group.ID] = group
	}
	if status == 0 || result.Delivery.Sufficient || result.LaunchCounts.Test != 3 || result.StoppedAtFirstFailure {
		t.Fatalf("independent failures were not collected: status=%d counts=%+v delivery=%+v", status, result.LaunchCounts, result.Delivery)
	}
	if byID["first"].Status != "failed" || byID["second"].Status != "failed" || byID["third"].Status != "passed" {
		t.Fatalf("independent verdicts: %+v", byID)
	}
	if byID["deep"].Status != "blocked" || !reflect.DeepEqual(byID["deep"].BlockingGroups, []string{"first"}) || byID["deep"].NativeLaunched {
		t.Fatalf("dependent launched despite failed prerequisite: %+v", byID["deep"])
	}
	if err := ValidateTestResult(result); err != nil {
		t.Fatalf("blocked result was not valid retained evidence: %v", err)
	}
}

func TestGLEPrerequisiteCannotBeHiddenByCachedDependent(t *testing.T) {
	t.Parallel()
	contract := testpolicy.Contract{SchemaVersion: testpolicy.ExecutionContractSchemaVersion, Groups: []testpolicy.Group{
		{ID: "harness"}, {ID: "consumer", Requires: []string{"harness"}}, {ID: "independent"},
	}}
	result := TestResult{Groups: []GroupResult{
		{ID: "harness", Status: "failed"},
		{ID: "consumer", Kind: "unit", Status: "reused", ReuseAttempt: "prior", CollectionComplete: true},
		{ID: "independent", Kind: "unit", Status: "reused", ReuseAttempt: "prior", CollectionComplete: true},
	}, LaunchCounts: LaunchCounts{ReusedTest: 2}}
	blockUnsatisfiedPrerequisites(&result, contract)
	if result.Groups[1].Status != "blocked" || result.Groups[2].Status != "reused" || result.LaunchCounts.ReusedTest != 1 {
		t.Fatalf("cached dependent hid failed harness or independent pass was lost: %+v", result)
	}
	repaired := TestResult{Groups: []GroupResult{
		{ID: "harness", Status: "passed"},
		{ID: "consumer", Kind: "unit", Status: "reused"},
		{ID: "independent", Kind: "unit", Status: "reused"},
	}, LaunchCounts: LaunchCounts{ReusedTest: 2}}
	blockUnsatisfiedPrerequisites(&repaired, contract)
	if repaired.Groups[1].Status != "reused" || repaired.Groups[2].Status != "reused" {
		t.Fatalf("repair discarded compatible passes: %+v", repaired.Groups)
	}
}

func TestGLEExplicitEnvironmentAndDeclaredToolInputs(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	external := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "source.txt"), []byte("source"), 0o600); err != nil {
		t.Fatal(err)
	}
	configuration := filepath.Join(external, "tool.conf")
	if err := os.WriteFile(configuration, []byte("one"), 0o600); err != nil {
		t.Fatal(err)
	}
	tool := filepath.Join(root, "tool.sh")
	if err := testexec.WriteFile(tool, []byte("#!/bin/sh\necho tool-one\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	group := testpolicy.Group{EnvironmentMode: "explicit", Env: map[string]string{"PATH": os.Getenv("PATH"), "TOOL_HOME": external},
		Inputs: []string{"source.txt"}, ExternalInputs: []testpolicy.ExternalInput{{ID: "tool-config", Path: "${TOOL_HOME}/tool.conf"}}}
	request := TestRunRequest{Environment: []string{"PATH=/untrusted", "AMBIENT_SECRET=do-not-inherit", "TOOL_HOME=/untrusted"}}
	environment := groupTestEnvironment(request, group)
	command, err := explicitEnvironmentCommand(context.Background(), root, environment, []string{"/usr/bin/env"})
	if err != nil {
		t.Fatal(err)
	}
	output, err := command.Output()
	if err != nil || strings.Contains(string(output), "AMBIENT_SECRET") || strings.Contains(string(output), "TOOL_HOME=/untrusted") {
		t.Fatalf("explicit child inherited semantic ambient values: %q %v", output, err)
	}
	firstInput, err := digestGroupInputsWithImplicit(root, group, environment, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configuration, []byte("two"), 0o600); err != nil {
		t.Fatal(err)
	}
	secondInput, err := digestGroupInputsWithImplicit(root, group, environment, nil)
	if err != nil || firstInput == secondInput {
		t.Fatalf("declared external input did not change identity: %s %s %v", firstInput, secondInput, err)
	}
	toolDefinition := testpolicy.Tool{ID: "compiler", Executable: tool, VersionArgs: []string{"--version"}}
	firstTool, _, _, err := identifyTool(context.Background(), root, environment, toolDefinition)
	if err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(tool, []byte("#!/bin/sh\necho tool-two\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	secondTool, _, _, err := identifyTool(context.Background(), root, environment, toolDefinition)
	if err != nil || firstTool == secondTool {
		t.Fatalf("declared tool did not change identity: %s %s %v", firstTool, secondTool, err)
	}
	delete(group.Env, "TOOL_HOME")
	if _, err := digestGroupInputsWithImplicit(root, group, groupTestEnvironment(request, group), nil); err == nil || !strings.Contains(err.Error(), "TOOL_HOME is unset") {
		t.Fatalf("undeclared external locator silently inherited: %v", err)
	}
	group.Env = map[string]string{}
	empty := groupTestEnvironment(request, group)
	if empty == nil {
		t.Fatal("explicit empty environment became inherited environment")
	}
	emptyCommand, err := explicitEnvironmentCommand(context.Background(), root, empty, []string{"/usr/bin/env"})
	if err != nil {
		t.Fatal(err)
	}
	output, err = emptyCommand.Output()
	if err != nil || len(output) != 0 {
		t.Fatalf("empty explicit child environment: %q %v", output, err)
	}
}
