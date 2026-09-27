package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// TestHookDelegateOwnerStatusContract proves the runtime hook's custody
// owner binding keeps the status contract the hook decides on: the exact
// custody JSON with 0, and 1 for a narrowed record that is absent.
func TestHookDelegateOwnerStatusContract(t *testing.T) {
	root := t.TempDir()
	installation := t.TempDir()
	exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("probe current process = %s, %v", state, err)
	}
	ref := exact.Ref()
	record := map[string]any{"jobId": "job-cli", "pid": exact.Pid, "pidStartedAt": ref.StartedAtSec}
	if ref.StartedAtUnixMicro > 0 {
		record["pidStartedAtExactMicro"] = ref.StartedAtUnixMicro
	}
	if ref.StartTicks > 0 {
		record["pidStartTicks"] = ref.StartTicks
		record["bootId"] = ref.BootID
	}
	data, _ := json.Marshal(record)
	path := filepath.Join(root, "artifacts", "agents", "jobs", "job-cli.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	owners := hookOwners{diagnostics: io.Discard}
	output, code := owners.HookDelegate(root, installation, "job-cli", os.Getpid())
	if code != 0 || !strings.Contains(output, `"delegate":true`) || !strings.HasSuffix(output, "\n") {
		t.Fatalf("positive query = status %d output %q", code, output)
	}
	if !regexp.MustCompile(`^\{"delegate":true,"jobId":"job-cli","matchedPid":[1-9][0-9]*,"comparisonMode":"(darwin-microseconds|linux-ticks-boot-id|legacy-seconds)"\}$`).MatchString(strings.TrimSuffix(output, "\n")) {
		t.Fatalf("positive query is not the exact custody shape the start boundary accepts: %q", output)
	}
	if _, code := owners.HookDelegate(root, installation, "job-absent", os.Getpid()); code != 1 {
		t.Fatalf("missing narrowed record status = %d, want 1", code)
	}
	if _, code := owners.HookDelegate(t.TempDir(), installation, "", os.Getpid()); code != 3 {
		t.Fatalf("query against a registry no job owns = %d, want 3", code)
	}
}
