package main

import (
	"context"
	"slices"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

// The scratch environment policy is negotiated with the destination worker
// through its worker capabilities (disk-lifetimes A5, DL4A-01): a launcher
// writes the highest policy both sides list, so a worker that lists no
// policies (every worker before A5.2) gets v1 exactly as before.

// sameKnownTestingWorkerCapabilities compares the six capabilities every
// generation reports; optional advertised lists are negotiated, not equal.
func sameKnownTestingWorkerCapabilities(got, want testingWorkerCapabilities) bool {
	return got.SchemaVersion == want.SchemaVersion && got.Protocol == want.Protocol &&
		got.ProtocolVersion == want.ProtocolVersion && got.TestResultSchemaVersion == want.TestResultSchemaVersion &&
		got.GroupExecutionIdentityVersion == want.GroupExecutionIdentityVersion && got.WorkerPolicyVersion == want.WorkerPolicyVersion
}

// chooseScratchEnvironmentPolicy is the policy this frontend writes for a
// worker listing workerPolicies: its own when the worker reads it, else v1.
func chooseScratchEnvironmentPolicy(workerPolicies []string) string {
	if slices.Contains(workerPolicies, proofrun.ScratchEnvironmentPolicy) {
		return proofrun.ScratchEnvironmentPolicy
	}
	return proofrun.ScratchEnvironmentPolicyV1
}

// testingWorkerScratchPolicies are the policies prepared's worker reads: the
// ones its capability check recorded, this engine's own when it is its own
// worker, else a fresh capability read; an unreadable answer is v1 only.
func testingWorkerScratchPolicies(ctx context.Context, prepared testingPreparation) []string {
	if prepared.WorkerCapabilitiesChecked {
		return prepared.WorkerScratchPolicies
	}
	if prepared.FirstTestingTransition {
		return currentTestingWorkerCapabilities().ScratchEnvironmentPolicies
	}
	capabilities, err := readTestingWorkerCapabilities(ctx, prepared, "")
	if err != nil {
		return nil
	}
	return capabilities.ScratchEnvironmentPolicies
}

// readTestingWorkerCapabilities checks the capabilities of the engine that
// runs prepared's worker: self on the first testing transition, else the
// trusted destination engine.
func readTestingWorkerCapabilities(ctx context.Context, prepared testingPreparation, self string) (testingWorkerCapabilities, error) {
	engine := prepared.PolicyEngine
	if prepared.FirstTestingTransition {
		engine = self
	}
	return requireTestingWorkerCapabilities(ctx, engine, prepared.Environment)
}

// prepareTestingScratch binds request to scratch under the negotiated policy.
func prepareTestingScratch(ctx context.Context, request *proofrun.TestRunRequest, scratch *proofrun.ScratchRun, prepared testingPreparation) error {
	request.BindScratch(scratch, nil)
	policy := chooseScratchEnvironmentPolicy(testingWorkerScratchPolicies(ctx, prepared))
	return proofrun.PrepareScratchEnvironmentFor(request, scratch, policy)
}
