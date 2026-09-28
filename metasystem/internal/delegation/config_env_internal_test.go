package delegation

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// TestConfigGetReadsTheRequestConfigurationNotTheProcess: a request carrying
// configuration (a critic read's selected-installation roster) is resolved
// in this process from the request, above the checkout's files, and one
// request's configuration never reaches another (VOA-14).
func TestConfigGetReadsTheRequestConfigurationNotTheProcess(t *testing.T) {
	t.Parallel()
	const key = "delegation.test-only-carried"
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte(key+"=5\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	carried := &session{root: root, stderr: &bytes.Buffer{}, request: Request{ConfigEnv: []string{config.EnvName(key) + "=9"}}}
	if got, err := carried.configGet(key, "3"); err != nil || got != "9" {
		t.Fatalf("configGet with request configuration = %q, %v; want 9", got, err)
	}
	plain := &session{root: root, stderr: &bytes.Buffer{}}
	if got, err := plain.configGet(key, "3"); err != nil || got != "5" {
		t.Fatalf("configGet without request configuration = %q, %v; want the checkout's 5", got, err)
	}
}

// TestOwnerAdapterHandsItsConfigurationToTheAdapterProcess: the adapter
// processes a lifecycle starts receive the request's configuration in their
// environment explicitly.
func TestOwnerAdapterHandsItsConfigurationToTheAdapterProcess(t *testing.T) {
	t.Parallel()
	engine := filepath.Join(t.TempDir(), "engine")
	if err := testexec.WriteFile(engine, []byte("#!/bin/sh\nprintf '%s' \"$METASYSTEM_RUNTIMES\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	adapter := ownerAdapter{root: t.TempDir(), engine: engine, env: []string{"METASYSTEM_RUNTIMES=codex"}}
	out, err := adapter.run(context.Background(), "fake", "signature")
	if err != nil || strings.TrimSpace(out) != "codex" {
		t.Fatalf("adapter environment = %q, %v; want codex", out, err)
	}
}

// The lifecycle's environment lookup (Config.LookupEnv, the evidence root's
// environment) is what a request's carried configuration lies over: one
// lookup answers every configuration read of a request.
func TestConfigLookupLiesOverTheLifecycleEnvironment(t *testing.T) {
	t.Parallel()
	base := func(name string) (string, bool) {
		if name == "HOME" {
			return "/from-lifecycle", true
		}
		return "", false
	}
	carried := &session{l: &Lifecycle{lookupEnv: base}, request: Request{ConfigEnv: []string{"HOME=/from-request"}}}
	if got, _ := carried.configLookup()("HOME"); got != "/from-request" {
		t.Fatalf("carried HOME = %q", got)
	}
	plain := &session{l: &Lifecycle{lookupEnv: base}}
	if got, _ := plain.configLookup()("HOME"); got != "/from-lifecycle" {
		t.Fatalf("lifecycle HOME = %q", got)
	}
}
