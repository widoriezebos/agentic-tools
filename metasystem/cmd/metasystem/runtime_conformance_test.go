package main

import (
	"os"
	"path/filepath"
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

// The adapter/host capability FLAGS are backed by executable seam
// files (code critique finding 7): a declaration cannot claim an
// adapter or launcher that does not exist.
func TestCapabilityFlagsBackedByExecutables(t *testing.T) {
	root := "../.."
	for _, declaration := range runtimes.All() {
		if declaration.HasAdapter {
			path := filepath.Join(root, "scripts", "agents", "adapters", declaration.Name+".sh")
			if info, err := os.Stat(path); err != nil || info.Mode()&0o111 == 0 {
				t.Errorf("%s declares an adapter but %s is not an executable file", declaration.Name, path)
			}
		}
		if declaration.HasHostLauncher {
			path := filepath.Join(root, "scripts", "agents", "hosts", declaration.Name+".sh")
			if info, err := os.Stat(path); err != nil || info.Mode()&0o111 == 0 {
				t.Errorf("%s declares a host launcher but %s is not an executable file", declaration.Name, path)
			}
		}
	}
}

// The ACP seam joins both ways: every declared expectation has an
// adapter-owned dialect, every registered dialect has a
// declaration, and every dialect covers the full tools ordinal —
// a missing grade would become a silent default at dispatch time.
func TestACPExpectationsMatchDialects(t *testing.T) {
	declared := map[string]bool{}
	for _, name := range runtimes.Names() {
		declaration, _ := runtimes.Lookup(name)
		if declaration.ExpectedACP != nil {
			declared[name] = true
			if _, err := adapter.ACPDialectFor(name); err != nil {
				t.Fatalf("%s declares ACP but registers no dialect: %v", name, err)
			}
		}
	}
	for _, name := range adapter.ACPDialectList() {
		if !declared[name] {
			t.Fatalf("%s registers an ACP dialect without a registry declaration", name)
		}
		dialect, err := adapter.ACPDialectFor(name)
		if err != nil {
			t.Fatal(err)
		}
		for _, grade := range []string{"read-only", "runtime-default"} {
			if dialect.ModeForTools[grade] == "" {
				t.Fatalf("%s dialect does not cover tools=%s", name, grade)
			}
		}
	}
	if !declared["devin"] {
		t.Fatal("devin must declare the first ACP increment (D79)")
	}
}
