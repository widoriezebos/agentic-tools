package main

import (
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/adapter"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/host"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
	usageseam "github.com/widoriezebos/agentic-tools/metasystem/internal/usage"
)

// The registry's capability EXPECTATIONS and the owner tables'
// registered capabilities must agree both ways (agnosticism audit:
// expected-but-unregistered fails, registered-but-undeclared fails) —
// this is what keeps the pure-data registry and the seam-local
// behavior tables from drifting apart.
func TestCapabilityDeclarationsMatchRegistrations(t *testing.T) {
	registered := map[string]map[string]bool{}
	note := func(runtime, capability string) {
		if registered[runtime] == nil {
			registered[runtime] = map[string]bool{}
		}
		registered[runtime][capability] = true
	}
	for _, runtime := range host.DeliveryRecollectorList() {
		note(runtime, runtimes.CapDeliveryRecollection)
	}
	for _, runtime := range usageseam.RecovererList() {
		note(runtime, runtimes.CapUsageRecovery)
	}
	for _, key := range adapter.SelftestProbeList() {
		note(strings.SplitN(key, "/", 2)[0], runtimes.CapSelfTestProbe)
	}

	expected := map[string]map[string]bool{}
	for _, declaration := range runtimes.All() {
		for _, capability := range declaration.ExpectedCapabilities {
			if expected[declaration.Name] == nil {
				expected[declaration.Name] = map[string]bool{}
			}
			expected[declaration.Name][capability] = true
		}
	}

	for runtime, capabilities := range expected {
		for capability := range capabilities {
			if !registered[runtime][capability] {
				t.Errorf("%s declares %s but no seam file registered it", runtime, capability)
			}
		}
	}
	for runtime, capabilities := range registered {
		for capability := range capabilities {
			if !expected[runtime][capability] {
				t.Errorf("%s registered %s but the registry does not declare it", runtime, capability)
			}
		}
	}
}
