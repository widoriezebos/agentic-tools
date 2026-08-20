package steward

import (
	"os/exec"
	"testing"

	"encoding/json"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"os"
	"path/filepath"
)

func TestOwnedLiveProcessesCountAsLive(t *testing.T) {
	v := census.Verdict{Verdict: "SUCCESS", Inventory: []census.InventoryItem{
		{Class: "CUSTODY"}, {Class: "ANNOUNCED"},
	}}
	w := workersFromVerdict(v)
	if w.Live != 2 || !w.CensusComplete || w.Untracked != 0 {
		t.Fatalf("custody and announced are live under a complete scan: %+v", w)
	}
}

func TestUntrackedProcessBlocksADeathProof(t *testing.T) {
	v := census.Verdict{Verdict: "SUCCESS", Inventory: []census.InventoryItem{{Class: "UNTRACKED"}}}
	w := workersFromVerdict(v)
	if w.Untracked != 1 || w.Live != 0 {
		t.Fatalf("an unaccounted live process must block the proof: %+v", w)
	}
}

func TestFailedCensusIsAnIncompleteScan(t *testing.T) {
	v := census.Verdict{Verdict: "CENSUS-FAILED"}
	w := workersFromVerdict(v)
	if w.CensusComplete {
		t.Fatalf("a failed census proves nothing: %+v", w)
	}
}

func TestUnknownInventoryClassIsUnprovable(t *testing.T) {
	v := census.Verdict{Verdict: "SUCCESS", Inventory: []census.InventoryItem{{Class: "SOMETHING-NEW"}}}
	w := workersFromVerdict(v)
	if w.Unprovable != 1 {
		t.Fatalf("an unrecognized class must not silently vanish: %+v", w)
	}
}

// gitConfig sets a repo-local key for fixtures.
func gitConfig(root, key, value string) ([]byte, error) {
	return exec.Command("git", "-C", root, "config", key, value).CombinedOutput()
}

func TestDiagnosticsBlockADeathProof(t *testing.T) {
	v := census.Verdict{Verdict: "SUCCESS", Diagnostics: []string{"pid 123: probe failed"}}
	w := workersFromVerdict(v)
	if w.Unprovable == 0 {
		t.Fatalf("an unaccounted process cannot be silently dropped: %+v", w)
	}
}

func TestRecordedRunnersAndRunsCountAsWorkers(t *testing.T) {
	root := t.TempDir()
	write := func(rel string, body map[string]any) {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		data, _ := json.Marshal(body)
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// A live monitored run: this test process itself.
	self := int64(os.Getpid())
	exact, _, err := identity.KernelProber{}.Probe(self)
	if err != nil {
		t.Fatal(err)
	}
	write("artifacts/agents/runs/live-run.json", map[string]any{
		"pid": self, "pidStartedAt": exact.StartedAt.Unix(),
	})
	// A gate whose pid was reused: recorded start disagrees.
	write("artifacts/agents/supervision/gate-runs/stale.json", map[string]any{
		"pid": self, "pidStartedAt": exact.StartedAt.Unix() - 9999,
	})
	// A malformed record blocks the proof rather than vanishing.
	if err := os.WriteFile(filepath.Join(root, "artifacts/agents/runs/torn.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	live, unprovable := supplementWorkers(root)
	if live != 1 {
		t.Fatalf("the live monitored run must count as a worker: live=%d", live)
	}
	if unprovable != 1 {
		t.Fatalf("the torn record must block the proof: unprovable=%d", unprovable)
	}
}

func TestDrainingRunWithDeadLeaderBlocksTheProof(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "artifacts/agents/runs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	self := int64(os.Getpid())
	exact, _, err := identity.KernelProber{}.Probe(self)
	if err != nil {
		t.Fatal(err)
	}
	// Draining stamps endedAt and exitCode while descendants may
	// still work; a leader whose pid was reused is doubt, not proof.
	rec, _ := json.Marshal(map[string]any{
		"pid": self, "pidStartedAt": exact.StartedAt.Unix() - 9999,
		"status": "draining", "endedAt": "2026-08-20T00:00:00Z", "exitCode": 0,
	})
	if err := os.WriteFile(filepath.Join(dir, "drain.json"), rec, 0o644); err != nil {
		t.Fatal(err)
	}
	live, unprovable := supplementWorkers(root)
	if live != 0 || unprovable != 1 {
		t.Fatalf("a draining record's timestamps are not a terminal proof: live=%d unprovable=%d", live, unprovable)
	}
}

func TestClosedRunWithDeadLeaderDoesNotBlockTheProof(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "artifacts/agents/runs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	self := int64(os.Getpid())
	exact, _, err := identity.KernelProber{}.Probe(self)
	if err != nil {
		t.Fatal(err)
	}
	rec, _ := json.Marshal(map[string]any{
		"pid": self, "pidStartedAt": exact.StartedAt.Unix() - 9999,
		"status": "green", "endedAt": "2026-08-20T00:00:00Z", "exitCode": 0,
	})
	if err := os.WriteFile(filepath.Join(dir, "closed.json"), rec, 0o644); err != nil {
		t.Fatal(err)
	}
	live, unprovable := supplementWorkers(root)
	if live != 0 || unprovable != 0 {
		t.Fatalf("a green run is closed; its dead leader proves nothing open: live=%d unprovable=%d", live, unprovable)
	}
}
