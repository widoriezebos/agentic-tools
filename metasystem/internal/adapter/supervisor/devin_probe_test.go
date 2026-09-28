package supervisor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The Devin probe, as scripts/agents/adapters/devin.sh probe was: the CLI
// version and the settings the session merges make the configuration
// identity, `devin auth status` gates it, and the snapshot carries Devin's
// transports, capabilities, unverified permissions and registry enforcement.
const devinProbeStub = `#!/bin/bash
case "$1 ${2:-}" in
  "--version ") echo "devin 2026.9.1" ;;
  "auth status") exit "$(cat "$STUB_DIR/auth-exit" 2>/dev/null || echo 0)" ;;
  *) exit 9 ;;
esac
`

func devinProbeFixture(t *testing.T) (*devinFixture, Deps) {
	t.Helper()
	f := newDevinFixture(t, "", "dispatch")
	f.stub("devin", devinProbeStub)
	if err := os.Chmod(filepath.Join(f.stubDir, "devin"), 0o755); err != nil {
		t.Fatal(err)
	}
	d := f.deps()
	d.Git = func(dir string, args ...string) (string, bool) {
		if strings.Join(args, " ") == "rev-parse --show-toplevel" {
			return f.root, true
		}
		return "", false
	}
	return f, d
}

func TestDevinProbeWritesItsSnapshot(t *testing.T) {
	t.Parallel()
	f, d := devinProbeFixture(t)
	if code := devinProbe(d, nil); code != 0 {
		t.Fatalf("probe exit %d: %s", code, f.stderr)
	}
	matches, _ := filepath.Glob(filepath.Join(f.root, "artifacts", "agents", "capabilities", "devin-*.json"))
	if len(matches) != 1 {
		t.Fatalf("snapshots %v", matches)
	}
	data, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatal(err)
	}
	var snapshot map[string]any
	if err := json.Unmarshal(data, &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot["runtime"] != "devin" || snapshot["cliVersion"] != "2026.9.1" {
		t.Fatalf("identity %v %v", snapshot["runtime"], snapshot["cliVersion"])
	}
	caps, _ := snapshot["capabilities"].(map[string]any)
	if caps["protocolServer"] != true || caps["sessionEstablishedSignal"] != false || caps["sessionEstablishedTimeoutSec"] != float64(30) || caps["nativeUsage"] != false {
		t.Fatalf("capabilities %v", caps)
	}
	if transports, _ := json.Marshal(snapshot["transports"]); string(transports) != `["file","stdout","atif","acp"]` {
		t.Fatalf("transports %s", transports)
	}
	enforcement, _ := json.Marshal(snapshot["envelopeEnforcement"])
	if !strings.Contains(string(enforcement), `"network":"notEnforced"`) || !strings.Contains(string(enforcement), `"writeRoots":"notEnforced"`) || !strings.Contains(string(enforcement), `"readRoots":"notEnforced"`) {
		t.Fatalf("enforcement %s", enforcement)
	}
	permissions, _ := json.Marshal(snapshot["permissions"])
	if string(permissions) != `{"unverified":["readRoots","writeRoots","network"]}` {
		t.Fatalf("permissions %s", permissions)
	}
}

func TestDevinProbeRefusesWithoutAuthentication(t *testing.T) {
	t.Parallel()
	f, d := devinProbeFixture(t)
	f.stub("auth-exit", "1\n")
	if code := devinProbe(d, nil); code != 1 || !strings.Contains(f.stderr.String(), "devin authentication is unavailable; run devin auth login") {
		t.Fatalf("exit %d stderr %q", code, f.stderr)
	}
	if matches, _ := filepath.Glob(filepath.Join(f.root, "artifacts", "agents", "capabilities", "devin-*.json")); len(matches) != 0 {
		t.Fatalf("an unauthenticated probe wrote %v", matches)
	}
}

// TestDevinCancelIsTheOwnedCancel: `devin cancel --job ID` is
// dispatch.sh's owned cancel, as devin.sh's cancel verb was; its status is
// the verb's.
func TestDevinCancelIsTheOwnedCancel(t *testing.T) {
	t.Parallel()
	f := newDevinFixture(t, "", "dispatch")
	code := Main([]string{"devin", "cancel", "--root", f.root, "--job", f.job}, func(string) Deps { return f.deps() })
	// The recorder answers an unknown callback 2: the verb returns it.
	if code != 2 || strings.Join(f.dispatch.verbs(), " ") != "__cancel-owned" || flagValue(f.dispatch.calls[0], "--job") != f.job {
		t.Fatalf("exit %d calls %v", code, f.dispatch.calls)
	}
}

// TestDevinPrepareLeavesTheEffectiveEnvelope: prepare (role delegate) leaves
// the effective envelope the shared comparison reads, with the working
// directory as the write boundary (devin.sh's rewrite, now Devin's choice
// inside prepare), and the launch proceeds because it is no wider than the
// request.
func TestDevinPrepareLeavesTheEffectiveEnvelope(t *testing.T) {
	t.Parallel()
	f := newDevinFixture(t, "", "dispatch")
	f.legacyHappyStubs()
	if code := f.run("dispatch"); code != 0 {
		t.Fatalf("exit %d\ncalls %v\nstderr %s\nlog %s", code, f.dispatch.verbs(), f.stderr, f.jobLog())
	}
	data, err := os.ReadFile(filepath.Join(f.roundDir, "effective-permissions.json"))
	if err != nil {
		t.Fatal(err)
	}
	var effective map[string]any
	if err := json.Unmarshal(data, &effective); err != nil {
		t.Fatal(err)
	}
	workspace, _ := filepath.EvalSymlinks(f.workspace)
	if roots, _ := json.Marshal(effective["writeRoots"]); string(roots) != `["`+workspace+`"]` {
		t.Fatalf("write boundary %s, want the working directory %s", roots, workspace)
	}
	if effective["network"] != "deny" || effective["approvals"] != "deny" || effective["tools"] != "runtime-default" {
		t.Fatalf("effective envelope %v", effective)
	}
}
