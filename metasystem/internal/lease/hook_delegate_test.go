package lease

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

func hookExact(t *testing.T, pid int64) identity.Exact {
	t.Helper()
	exact, state, err := (identity.KernelProber{}).Probe(pid)
	if err != nil || state != identity.Alive {
		t.Fatalf("probe pid %d = %s, %v", pid, state, err)
	}
	return exact
}

func writeHookJob(t *testing.T, root, job string, exact identity.Exact, custody bool) {
	t.Helper()
	ref := exact.Ref()
	process := map[string]any{
		"pid": exact.Pid, "pidStartedAt": ref.StartedAtSec,
	}
	if ref.StartedAtUnixMicro > 0 {
		process["pidStartedAtExactMicro"] = ref.StartedAtUnixMicro
	}
	if ref.StartTicks > 0 {
		process["pidStartTicks"] = ref.StartTicks
		process["bootId"] = ref.BootID
	}
	record := map[string]any{"jobId": job}
	if custody {
		record["custodyProcesses"] = []any{process}
	} else {
		for key, value := range process {
			record[key] = value
		}
	}
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "artifacts", "agents", "jobs", job+".json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestHookDelegateVerifiesProviderChildOrRecordedAdapterAncestor(t *testing.T) {
	root := t.TempDir()
	installation := t.TempDir()
	self := hookExact(t, int64(os.Getpid()))
	writeHookJob(t, root, "job-ancestor", self, false)

	child := exec.Command("sleep", "30")
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = child.Process.Kill()
		_ = child.Wait()
	})
	result, err := HookDelegate(root, installation, "job-ancestor", int64(child.Process.Pid))
	if err != nil || !result.Delegate || result.JobID != "job-ancestor" || result.MatchedPID != self.Pid {
		t.Fatalf("ancestor custody = %+v, %v", result, err)
	}

	writeHookJob(t, root, "job-child", hookExact(t, int64(child.Process.Pid)), true)
	result, err = HookDelegate(root, installation, "job-child", int64(child.Process.Pid))
	if err != nil || !result.Delegate || result.MatchedPID != int64(child.Process.Pid) {
		t.Fatalf("direct child custody = %+v, %v", result, err)
	}
}

func TestHookDelegateRejectsStaleForgedAndNonJobEvidence(t *testing.T) {
	root := t.TempDir()
	installation := t.TempDir()
	self := hookExact(t, int64(os.Getpid()))
	stale := self
	stale.StartedAt = stale.StartedAt.AddDate(0, 0, -1)
	if stale.StartTicks > 0 {
		stale.StartTicks++
	}
	writeHookJob(t, root, "job-stale", stale, false)
	result, err := HookDelegate(root, installation, "job-stale", int64(os.Getpid()))
	if err != nil || result.Delegate {
		t.Fatalf("stale identity was accepted: %+v, %v", result, err)
	}

	mission := filepath.Join(root, "artifacts", "agents", "missions", "m1", "state.json")
	if err := os.MkdirAll(filepath.Dir(mission), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mission, []byte(`{"host":{"pid":1,"pidStartedAt":1}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err = HookDelegate(root, installation, "", int64(os.Getpid()))
	if err != nil || result.Delegate {
		t.Fatalf("mission host custody was accepted: %+v, %v", result, err)
	}
	if _, err := HookDelegate(root, installation, "../job-stale", int64(os.Getpid())); err == nil {
		t.Fatal("a traversal-shaped job hint was accepted")
	}
}

func TestHookDelegateFailsClosedOnUnreadableOrMalformedJobEvidence(t *testing.T) {
	root := t.TempDir()
	installation := t.TempDir()
	path := filepath.Join(root, "artifacts", "agents", "jobs", "job-bad.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"jobId":`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := HookDelegate(root, installation, "job-bad", int64(os.Getpid())); err == nil {
		t.Fatal("malformed job evidence did not fail closed")
	}
}

func TestHookDelegateToleratesDisappearingRecordOnlyDuringUnhintedScan(t *testing.T) {
	root := t.TempDir()
	installation := t.TempDir()
	self := hookExact(t, int64(os.Getpid()))
	const job = "job-disappears"
	path := filepath.Join(root, "artifacts", "agents", "jobs", job+".json")
	originalReadFile := hookDelegateReadFile
	t.Cleanup(func() { hookDelegateReadFile = originalReadFile })
	hookDelegateReadFile = func(candidate string) ([]byte, error) {
		if candidate == path {
			if err := os.Remove(candidate); err != nil && !os.IsNotExist(err) {
				return nil, err
			}
		}
		return os.ReadFile(candidate)
	}

	writeHookJob(t, root, job, self, true)
	result, err := HookDelegate(root, installation, "", int64(os.Getpid()))
	if err != nil || result.Delegate {
		t.Fatalf("unhinted disappearance = %+v, %v", result, err)
	}

	writeHookJob(t, root, job, self, true)
	if _, err := HookDelegate(root, installation, job, int64(os.Getpid())); err == nil {
		t.Fatal("explicitly hinted disappearing job record did not fail closed")
	}
}
