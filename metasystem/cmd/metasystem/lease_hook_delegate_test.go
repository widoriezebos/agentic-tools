package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

func TestLeaseHookDelegateCLIExitContract(t *testing.T) {
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
	output, code := captureStdout(t, func() int {
		return runLeaseHookDelegate([]string{"--root", root, "--metasystem-root", installation, "--caller-pid", stringInt64(int64(os.Getpid())), "--job", "job-cli"})
	})
	if code != 0 || !strings.Contains(output, `"delegate":true`) {
		t.Fatalf("positive query = exit %d output %q", code, output)
	}
	if _, code := captureStdout(t, func() int {
		return runLeaseHookDelegate([]string{"--root", root, "--metasystem-root", installation, "--caller-pid", stringInt64(int64(os.Getpid())), "--job", "job-absent"})
	}); code != 1 {
		t.Fatalf("missing narrowed record exit = %d, want 1", code)
	}
}

func stringInt64(value int64) string {
	return strconv.FormatInt(value, 10)
}
