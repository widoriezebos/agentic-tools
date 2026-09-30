package testrun

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"slices"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/refusal"
)

const testWorkerProtocol = "metasystem.test-worker"

// ErrWorkerPolicyUnsupported refuses a pinned engine whose test workers
// this engine cannot drive.
var ErrWorkerPolicyUnsupported error = &refusal.Coded{Code: "TEST_WORKER_POLICY_UNSUPPORTED", Reason: "the pinned engine cannot run this engine's test workers"}

type WorkerCapabilities struct {
	SchemaVersion                 int    `json:"schemaVersion"`
	Protocol                      string `json:"protocol"`
	ProtocolVersion               int    `json:"protocolVersion"`
	TestResultSchemaVersion       int    `json:"testResultSchemaVersion"`
	GroupExecutionIdentityVersion int    `json:"groupExecutionIdentityVersion"`
	WorkerPolicyVersion           int    `json:"workerPolicyVersion"`
	// ScratchEnvironmentPolicies are the descriptor policies the worker reads;
	// absent means v1 only.
	ScratchEnvironmentPolicies []string `json:"scratchEnvironmentPolicies,omitempty"`
	// TestResultSchemaVersions are the result schemas the worker writes on
	// request; absent means testResultSchemaVersion only. The compared
	// testResultSchemaVersion stays the worker-policy schema, so a frontend
	// that predates the list still talks to this worker (and gets that schema).
	TestResultSchemaVersions []int `json:"testResultSchemaVersions,omitempty"`
}

func CurrentWorkerCapabilities() WorkerCapabilities {
	return WorkerCapabilities{SchemaVersion: 1, Protocol: testWorkerProtocol,
		ProtocolVersion: proofrun.TestWorkerProtocolVersion, TestResultSchemaVersion: proofrun.WorkerPolicyTestResultSchemaVersion,
		TestResultSchemaVersions:      proofrun.TestResultSchemaVersions,
		GroupExecutionIdentityVersion: proofrun.GroupExecutionIdentityVersion, WorkerPolicyVersion: proofrun.TestWorkerPolicyVersion,
		ScratchEnvironmentPolicies: proofrun.ScratchEnvironmentPolicies}
}

func WriteWorkerCapabilities(writer io.Writer) error {
	return json.NewEncoder(writer).Encode(CurrentWorkerCapabilities())
}

// RequireWorkerCapabilities refuses a worker whose six known
// capabilities differ from this frontend's and answers what it reports. A
// field this frontend does not know is tolerated, so a newer worker may
// advertise more (scratch environment policies) without being refused.
func RequireWorkerCapabilities(ctx context.Context, engine string, environment []string) (WorkerCapabilities, error) {
	command := exec.CommandContext(ctx, engine, "test", "worker-capabilities")
	command.Env = Environment(environment)
	data, err := command.CombinedOutput()
	if err != nil {
		return WorkerCapabilities{}, fmt.Errorf("%w: %s did not answer the worker handshake (%v); install a matching engine release first", ErrWorkerPolicyUnsupported,
			engine, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	var capabilities WorkerCapabilities
	if err := decoder.Decode(&capabilities); err != nil {
		return WorkerCapabilities{}, fmt.Errorf("%w: %s gave an unreadable handshake answer (%v); install a matching engine release first", ErrWorkerPolicyUnsupported, engine, err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return WorkerCapabilities{}, fmt.Errorf("%w: %s added data after its handshake answer; install a matching engine release first", ErrWorkerPolicyUnsupported, engine)
	}
	want := CurrentWorkerCapabilities()
	if !sameKnownWorkerCapabilities(capabilities, want) {
		return WorkerCapabilities{}, fmt.Errorf("%w: %s drives workers differently from this engine; install a matching engine release first",
			ErrWorkerPolicyUnsupported, engine)
	}
	return capabilities, nil
}

// The scratch environment policy is negotiated with the destination worker
// through its worker capabilities (disk-lifetimes A5, DL4A-01): a launcher
// writes the highest policy both sides list, so a worker that lists no
// policies (every worker before A5.2) gets v1 exactly as before.

// sameKnownWorkerCapabilities compares the six capabilities every
// generation reports; optional advertised lists are negotiated, not equal.
func sameKnownWorkerCapabilities(got, want WorkerCapabilities) bool {
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

// chooseTestResultSchema asks for the execution-record schema only from a
// worker whose recorded capabilities list it; zero leaves the request field
// out, which every worker reads as the worker-policy schema. An unchecked
// preparation asks only when this engine is its own worker.
func chooseTestResultSchema(prepared Preparation) int {
	schemas := prepared.WorkerResultSchemas
	if !prepared.WorkerCapabilitiesChecked && prepared.FirstTestingTransition {
		schemas = CurrentWorkerCapabilities().TestResultSchemaVersions
	}
	if slices.Contains(schemas, proofrun.TestResultSchemaVersion) {
		return proofrun.TestResultSchemaVersion
	}
	return 0
}

// workerScratchPolicies are the policies prepared's worker reads: the
// ones its capability check recorded, this engine's own when it is its own
// worker, else a fresh capability read; an unreadable answer is v1 only.
func workerScratchPolicies(ctx context.Context, prepared Preparation) []string {
	if prepared.WorkerCapabilitiesChecked {
		return prepared.WorkerScratchPolicies
	}
	if prepared.FirstTestingTransition {
		return CurrentWorkerCapabilities().ScratchEnvironmentPolicies
	}
	capabilities, err := ReadWorkerCapabilities(ctx, prepared, "")
	if err != nil {
		return nil
	}
	return capabilities.ScratchEnvironmentPolicies
}

// ReadWorkerCapabilities checks the capabilities of the engine that
// runs prepared's worker: self on the first testing transition, else the
// trusted destination engine.
func ReadWorkerCapabilities(ctx context.Context, prepared Preparation, self string) (WorkerCapabilities, error) {
	engine := prepared.PolicyEngine
	if prepared.FirstTestingTransition {
		engine = self
	}
	return RequireWorkerCapabilities(ctx, engine, prepared.Environment)
}

// PrepareScratch binds request to scratch under the negotiated policy.
func PrepareScratch(ctx context.Context, request *proofrun.TestRunRequest, scratch *proofrun.ScratchRun, prepared Preparation) error {
	request.BindScratch(scratch, nil)
	policy := chooseScratchEnvironmentPolicy(workerScratchPolicies(ctx, prepared))
	return proofrun.PrepareScratchEnvironmentFor(request, scratch, policy)
}
