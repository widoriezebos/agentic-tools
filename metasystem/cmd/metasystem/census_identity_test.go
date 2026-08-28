package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

func writeAliveIdentity(t *testing.T, path string, ref identity.Ref) {
	t.Helper()
	value := map[string]any{"pid": ref.Pid, "pidStartedAt": ref.StartedAtSec}
	switch ref.Mode() {
	case identity.CompareDarwinMicroseconds:
		value["pidStartedAtExactMicro"] = ref.StartedAtUnixMicro
	case identity.CompareLinuxTicksBootID:
		value["pidStartTicks"] = ref.StartTicks
		value["bootId"] = ref.BootID
	}
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestProcAliveUsesNativeExactIdentityAndLegacyFallback(t *testing.T) {
	t.Setenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE", "")
	pid := int64(os.Getpid())
	exact, state, err := (identity.KernelProber{}).Probe(pid)
	if err != nil || state != identity.Alive {
		t.Fatalf("probe self: state=%s err=%v", state, err)
	}
	root := t.TempDir()
	path := filepath.Join(root, "identity.json")
	ref := exact.Ref()
	writeAliveIdentity(t, path, ref)
	if code := runCensusAlive([]string{"--identity-file", path, "--root", root}); code != 0 {
		t.Fatalf("native exact identity returned exit %d", code)
	}

	recycled := ref
	if runtime.GOOS == "linux" {
		recycled.StartTicks++
	} else {
		recycled.StartedAtUnixMicro++
	}
	writeAliveIdentity(t, path, recycled)
	if code := runCensusAlive([]string{"--identity-file", path, "--root", root}); code != 1 {
		t.Fatalf("same-second recycled identity returned exit %d", code)
	}

	if code := runCensusAlive([]string{
		"--pid", strconv.FormatInt(pid, 10),
		"--start-time", strconv.FormatInt(exact.StartedAt.Unix(), 10),
		"--root", root,
	}); code != 0 {
		t.Fatalf("legacy seconds identity returned exit %d", code)
	}
}
