package candidateengine

import (
	"strings"
	"testing"
)

// TestCandidateEngineBuildEnvironmentIsPinnedWithoutDroppingProofCustody:
// every build-owned variable is pinned and the stamp set, while the proof
// custody variables the build does not own pass through.
func TestCandidateEngineBuildEnvironmentIsPinnedWithoutDroppingProofCustody(t *testing.T) {
	t.Parallel()
	stamp := strings.Repeat("a", 40)
	environment := BuildEnvironment([]string{
		"PATH=/fixture/bin", "GOFLAGS=-mod=vendor", "GOWORK=/foreign/workspace", "GOTOOLCHAIN=auto",
		"GOEXPERIMENT=fieldtrack", "GOENV=/foreign/goenv", "CGO_ENABLED=1", "GOAMD64=v4", "GOARM64=v9.5", "GOARM=5",
		"METASYSTEM_PROOF_CONTROL_ROOT=/proof", "METASYSTEM_PROOF_ATTEMPT=proof-attempt",
	}, stamp)
	values := map[string]string{}
	for _, entry := range environment {
		name, value, _ := strings.Cut(entry, "=")
		values[name] = value
	}
	want := map[string]string{"CGO_ENABLED": "0", "GOAMD64": "v1", "GOARM64": "v8.0", "GOARM": "7",
		"GOENV": "off", "GOEXPERIMENT": "", "GOFLAGS": "-mod=readonly", "GOTOOLCHAIN": "local", "GOWORK": "off", "METASYSTEM_BUILD_STAMP": stamp,
		"METASYSTEM_PROOF_CONTROL_ROOT": "/proof", "METASYSTEM_PROOF_ATTEMPT": "proof-attempt"}
	for name, value := range want {
		if values[name] != value {
			t.Fatalf("candidate build environment %s=%q, want %q: %v", name, values[name], value, environment)
		}
	}
}
