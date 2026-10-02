package testenv

import (
	"strings"
	"testing"
)

// A go command run under the namespace's HOME starts no telemetry writer:
// the go command's telemetry sidecar is a detached process that outlives
// the command and writes under the namespace's config dir, where it raced
// the namespace's removal ("unlinkat ...: directory not empty", seen in
// TestANestedChildUnderAReplacedHomeResolvesTheOuterPairThroughTheContext
// under load). The namespace records telemetry as off before any test runs.
func TestTheNamespaceTurnsGoTelemetryOff(t *testing.T) {
	t.Parallel()
	output, err := Go("env", "GOTELEMETRY").Output()
	if err != nil {
		t.Fatal(err)
	}
	if mode := strings.TrimSpace(string(output)); mode != "off" {
		t.Fatalf("go telemetry under the test namespace is %q, want off", mode)
	}
}
