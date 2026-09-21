package testpolicy

import (
	"encoding/json"
	"strings"
	"testing"
)

func workerPointer(value int) *int { return &value }

func commandWorkerContract(workers *int) Contract {
	contract := executionFixtureContract()
	group := &contract.Groups[0]
	group.Adapter = "command"
	group.Packages, group.Tests = nil, nil
	group.Argv, group.Format = []string{"sh", "scripts/check.sh"}, "junit-xml"
	group.Kind = "component"
	group.Outputs, group.Reports = []string{"reports"}, []string{"reports"}
	group.ExpectedTests = []ExpectedTest{{Report: "reports/result.xml", Name: "check"}}
	group.Resources.Workers = workers
	return contract
}

func TestWorkerResourceDeclarationPreservesOmittedZeroAndPositive(t *testing.T) {
	t.Parallel()
	for _, specimen := range []struct {
		name    string
		workers *int
		want    string
	}{
		{name: "omitted"},
		{name: "whole-attempt", workers: workerPointer(0), want: `"workers":0`},
		{name: "declared-share", workers: workerPointer(4096), want: `"workers":4096`},
	} {
		t.Run(specimen.name, func(t *testing.T) {
			t.Parallel()
			contract := commandWorkerContract(specimen.workers)
			if err := contract.Validate(); err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(contract)
			if err != nil || specimen.want != "" && !strings.Contains(string(encoded), specimen.want) || specimen.want == "" && strings.Contains(string(encoded), `"workers"`) {
				t.Fatalf("encoded contract=%s err=%v", encoded, err)
			}
			decoded, err := Decode(encoded)
			if err != nil {
				t.Fatal(err)
			}
			got := decoded.Groups[0].Resources.Workers
			if specimen.workers == nil && got != nil || specimen.workers != nil && (got == nil || *got != *specimen.workers) {
				t.Fatalf("workers round trip=%v want=%v", got, specimen.workers)
			}
		})
	}
}

func TestWorkerResourceDeclarationRejectsInvalidOwnersAndOverride(t *testing.T) {
	t.Parallel()
	negative := commandWorkerContract(workerPointer(-1))
	if err := negative.Validate(); err == nil || !strings.Contains(err.Error(), "workers must be zero or a positive") {
		t.Fatalf("negative workers error=%v", err)
	}
	goGroup := executionFixtureContract()
	goGroup.Groups[0].Resources.Workers = workerPointer(2)
	if err := goGroup.Validate(); err == nil || !strings.Contains(err.Error(), "command and section") {
		t.Fatalf("Go workers declaration error=%v", err)
	}
	legacy := commandWorkerContract(workerPointer(2))
	legacy.SchemaVersion = SchemaVersion
	legacy.Groups[0].Phase, legacy.Groups[0].EnvironmentMode = "", ""
	if err := legacy.Validate(); err == nil || !strings.Contains(err.Error(), "require schemaVersion") {
		t.Fatalf("legacy workers declaration error=%v", err)
	}
	reserved := commandWorkerContract(nil)
	reserved.Groups[0].Env = map[string]string{TestWorkersEnvironment: "999"}
	if err := reserved.Validate(); err == nil || !strings.Contains(err.Error(), "reserved by the test runner") {
		t.Fatalf("reserved environment error=%v", err)
	}
}

func TestProtectedWorkerDeclarationKeepsTheLargerAllowance(t *testing.T) {
	t.Parallel()
	base := commandWorkerContract(workerPointer(2))
	candidate := cloneContract(base)
	candidate.Groups[0].Resources.Workers = workerPointer(5)
	merged := ProtectedContract(base, candidate)
	if merged.Groups[0].Resources.Workers == nil || *merged.Groups[0].Resources.Workers != 5 {
		t.Fatalf("merged workers=%v", merged.Groups[0].Resources.Workers)
	}
	candidate.Groups[0].Resources.Workers = workerPointer(0)
	merged = ProtectedContract(base, candidate)
	if merged.Groups[0].Resources.Workers == nil || *merged.Groups[0].Resources.Workers != 0 {
		t.Fatalf("whole-attempt declaration was not retained: %v", merged.Groups[0].Resources.Workers)
	}
}

func TestNamedGoGroupsMayUseTheNativeShardOwner(t *testing.T) {
	t.Parallel()
	contract := executionFixtureContract()
	contract.Groups[0].Tests = json.RawMessage(`["TestOne","TestTwo"]`)
	contract.Groups[0].Shards = 2
	if err := contract.Validate(); err != nil {
		t.Fatalf("named Go shards were refused: %v", err)
	}
	contract.Groups[0].Coverage = true
	if err := contract.Validate(); err == nil || !strings.Contains(err.Error(), "coverage requires tests=all") {
		t.Fatalf("named sharding weakened the coverage completeness rule: %v", err)
	}
}
