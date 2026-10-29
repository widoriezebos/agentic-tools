package lease

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegatecustody"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

func hookExact(t *testing.T, pid int64) identity.Exact {
	t.Helper()
	exact, state, err := (identity.KernelProber{}).Probe(pid)
	if err != nil || state != identity.Alive {
		t.Fatalf("probe pid %d = %s, %v", pid, state, err)
	}
	return exact
}

func TestHookDelegateOwnsAUnitLaunchsProcesses(t *testing.T) {
	child, _ := readyChild(t)
	self := hookExact(t, int64(os.Getpid())).Ref()
	for _, row := range []string{"supervisor", "child", "completed", "absent", "unreadable", "failed"} {
		t.Run(row, func(t *testing.T) {
			root, installation := t.TempDir(), t.TempDir()
			store := launch.Store{Root: t.TempDir()}
			job := "unit-reader"
			status := "pending"
			if row == "failed" {
				status = "failed"
			}
			path := filepath.Join(installation, "artifacts/agents/jobs", job+".json")
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			data, _ := json.Marshal(map[string]any{"jobId": job, "operationId": "unit-launch:" + job, "unitRun": "unit", "status": status})
			if err := os.WriteFile(path, data, 0o644); err != nil {
				t.Fatal(err)
			}
			if row != "absent" {
				record := launch.Record{ID: job, State: launch.Running, Supervisor: &self}
				if row == "completed" {
					record.State = launch.Completed
				}
				if row == "child" {
					ref := hookExact(t, child).Ref()
					record.Supervisor, record.Child = nil, &ref
				}
				if err := store.Create(record); err != nil {
					t.Fatal(err)
				}
				if row == "unreadable" {
					dir, _ := store.StateDir(job)
					if err := os.WriteFile(filepath.Join(dir, "record.json"), []byte("{"), 0o600); err != nil {
						t.Fatal(err)
					}
				}
			}
			owned, err := delegatecustody.Read(root, "", nil)
			if err != nil {
				t.Fatal(err)
			}
			probe, err := fixtureProbe(installation)
			if err != nil {
				t.Fatal(err)
			}
			got, err := hookDelegateWith(owned, installation, "", child, probe, store)
			if row == "unreadable" {
				if err == nil {
					t.Fatal("an unreadable launch did not refuse the hook")
				}
				return
			}
			want := row == "supervisor" || row == "child"
			if err != nil || got.Delegate != want || (want && got.JobID != job) {
				t.Fatalf("unit process custody = %+v, %v; delegate wanted %v", got, err, want)
			}
		})
	}
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

	childPID, _ := readyChild(t)
	result, err := HookDelegate(root, installation, "job-ancestor", childPID)
	if err != nil || !result.Delegate || result.JobID != "job-ancestor" || result.MatchedPID != self.Pid {
		t.Fatalf("ancestor custody = %+v, %v", result, err)
	}

	writeHookJob(t, root, "job-child", hookExact(t, childPID), true)
	result, err = HookDelegate(root, installation, "job-child", childPID)
	if err != nil || !result.Delegate || result.MatchedPID != childPID {
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
	reads := 0
	hookDelegateReadFile = func(candidate string) ([]byte, error) {
		if candidate == path {
			reads++
			if reads == 2 {
				if err := os.Remove(candidate); err != nil {
					return nil, err
				}
			}
		}
		return os.ReadFile(candidate)
	}
	writeHookJob(t, root, job, self, true)
	// Both scans read the same directory on this layout, and the second scan
	// must preserve the ordinary scan's disappearance rule.
	result, err = HookDelegate(root, root, "", int64(os.Getpid()))
	if err != nil || !result.Delegate || reads != 2 {
		t.Fatalf("disappearance during the added scan = %+v, %v; reads=%d", result, err, reads)
	}
}
