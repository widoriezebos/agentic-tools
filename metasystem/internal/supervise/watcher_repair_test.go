package supervise

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

func TestPendingWatcherRepairConsultsTheHealthBreakerBeforeActing(t *testing.T) {
	root := t.TempDir()
	held := Held{
		Component: Watcher, Tag: "owner-watcher-7", Generation: 7,
		Identity: identity.Ref{Pid: 47001, StartedAtSec: 101},
	}
	request := WatcherRestartRequest{
		Schema: 1, Generation: held.Generation, Pid: held.Identity.Pid,
		PidStartedAt: held.Identity.StartedAtSec, InstanceTag: held.Tag,
		RequestedAt: time.Now().UTC(), Reason: "recorded pid is dead",
	}
	if err := saveWatcherRestartRequest(root, request); err != nil {
		t.Fatal(err)
	}
	healthDir := filepath.Join(root, "artifacts", "agents", "steward")
	if err := os.MkdirAll(healthDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(healthDir, "health.json"), []byte(`{
  "state":{"failureCounts":{"repo-watcher":5}},
  "verdict":{"roles":[{"role":"repo-watcher","failureEscalation":"AUTO_HEAL_ENDED"}]}
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	requested, err := (&DiskWatcherRepairs{Root: root}).WatcherRestartRequested(held)
	if err != nil || requested {
		t.Fatalf("an earlier request must become unactionable when failure five ends healing: requested=%v err=%v", requested, err)
	}
}
