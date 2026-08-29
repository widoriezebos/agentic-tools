package supervise

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

func writeWatcherRepairState(t *testing.T, root string, generation, pid, started int64, tag string) {
	t.Helper()
	if err := os.MkdirAll(SupervisionDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	document := stateDocument{Generation: generation, Components: map[string]stateComponent{
		string(Watcher): {Pid: pid, PidStartedAt: started, InstanceTag: tag},
	}}
	data, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(SupervisionDir(root), "state.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestWatcherRepairRequestIsGenerationFencedAndCompletesExactlyOnce(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	writeWatcherRepairState(t, root, 7, 47001, 101, "owner-watcher-7")
	if err := RequestWatcherRestart(root, "recorded pid is dead", now); err != nil {
		t.Fatal(err)
	}
	if err := RequestWatcherRestart(root, "duplicate observation", now.Add(time.Minute)); err != nil {
		t.Fatalf("a duplicate request was not idempotent: %v", err)
	}
	previous := Held{
		Component: Watcher, Tag: "owner-watcher-7", Generation: 7,
		Identity: identity.Ref{Pid: 47001, StartedAtSec: 101},
	}
	repairs := &DiskWatcherRepairs{Root: root}
	requested, err := repairs.WatcherRestartRequested(previous)
	if err != nil || !requested {
		t.Fatalf("the exact pending generation was not actionable: requested=%v err=%v", requested, err)
	}
	foreign := previous
	foreign.Generation++
	if requested, err := repairs.WatcherRestartRequested(foreign); err == nil || requested || !strings.Contains(err.Error(), "current watcher") {
		t.Fatalf("a request crossed the generation fence: requested=%v err=%v", requested, err)
	}
	replacement := Held{Component: Watcher, Tag: "owner-watcher-7-replacement", Generation: 7, Identity: identity.Ref{Pid: 47002, StartedAtSec: 102}}
	if err := repairs.CompleteWatcherRestart(foreign, replacement); err == nil {
		t.Fatal("a foreign generation completed the pending repair")
	}
	if err := repairs.CompleteWatcherRestart(previous, replacement); err != nil {
		t.Fatal(err)
	}
	request, err := loadWatcherRestartRequest(root)
	if err != nil || !request.Completed || request.Replacement == nil || request.Replacement.Pid != replacement.Identity.Pid {
		t.Fatalf("replacement identity was not recorded: request=%+v err=%v", request, err)
	}
	if requested, err := repairs.WatcherRestartRequested(previous); err != nil || requested {
		t.Fatalf("a completed request remained actionable: requested=%v err=%v", requested, err)
	}
	if err := repairs.CompleteWatcherRestart(previous, replacement); err == nil {
		t.Fatal("a completed request was completed twice")
	}
}

func TestEndingWatcherRepairIsMissingSafeAndPersistsTheBreakerReason(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 29, 13, 0, 0, 0, time.UTC)
	if err := EndWatcherRestart(root, "healing ended", now); err != nil {
		t.Fatalf("ending a missing request was not idempotent: %v", err)
	}
	request := WatcherRestartRequest{
		Schema: 1, Generation: 3, Pid: 41, PidStartedAt: 100, InstanceTag: "watcher-3",
		RequestedAt: now.Add(-time.Minute), Reason: "watcher is stale",
	}
	if err := saveWatcherRestartRequest(root, request); err != nil {
		t.Fatal(err)
	}
	if err := EndWatcherRestart(root, "automatic healing ended", now); err != nil {
		t.Fatal(err)
	}
	ended, err := loadWatcherRestartRequest(root)
	if err != nil || !ended.Completed || ended.EndReason != "automatic healing ended" || !ended.CompletedAt.Equal(now) {
		t.Fatalf("ended request lost its reason: request=%+v err=%v", ended, err)
	}
	if err := EndWatcherRestart(root, "replacement reason", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	again, err := loadWatcherRestartRequest(root)
	if err != nil || again.EndReason != ended.EndReason || !again.CompletedAt.Equal(ended.CompletedAt) {
		t.Fatalf("ending twice changed durable history: request=%+v err=%v", again, err)
	}
}

func TestWatcherRepairRefusesUnusableStateAndMalformedRequests(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	if err := RequestWatcherRestart(root, "missing state", now); err == nil || !strings.Contains(err.Error(), "read supervision state") {
		t.Fatalf("missing state did not refuse repair: %v", err)
	}
	if err := os.MkdirAll(SupervisionDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(SupervisionDir(root), "state.json")
	if err := os.WriteFile(statePath, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := RequestWatcherRestart(root, "malformed state", now); err == nil || !strings.Contains(err.Error(), "read supervision state") {
		t.Fatalf("malformed state did not refuse repair: %v", err)
	}
	if err := os.WriteFile(statePath, []byte(`{"generation":1,"components":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := RequestWatcherRestart(root, "missing watcher", now); err == nil || !strings.Contains(err.Error(), "no exact watcher") {
		t.Fatalf("incomplete watcher state did not refuse repair: %v", err)
	}
	requestPath := watcherRestartRequestPath(root)
	if err := os.WriteFile(requestPath, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadWatcherRestartRequest(root); err == nil || !strings.Contains(err.Error(), "malformed") {
		t.Fatalf("malformed watcher request was accepted: %v", err)
	}
	if err := os.WriteFile(requestPath, []byte(`{"schema":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadWatcherRestartRequest(root); err == nil || !strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("incomplete watcher request was accepted: %v", err)
	}
}

func TestWatcherRepairHealthFenceRecognizesRoleEscalationAndMalformedEvidence(t *testing.T) {
	root := t.TempDir()
	healthDir := filepath.Join(root, "artifacts", "agents", "steward")
	if err := os.MkdirAll(healthDir, 0o755); err != nil {
		t.Fatal(err)
	}
	healthPath := filepath.Join(healthDir, "health.json")
	if err := os.WriteFile(healthPath, []byte(`{
  "state":{"failureCounts":{"repo-watcher":4}},
  "verdict":{"roles":[{"role":"repo-watcher","failureEscalation":"AUTO_HEAL_ENDED"}]}
}`), 0o600); err != nil {
		t.Fatal(err)
	}
	ended, err := watcherHealingEnded(root)
	if err != nil || !ended {
		t.Fatalf("role escalation did not fence watcher repair: ended=%v err=%v", ended, err)
	}
	if err := os.WriteFile(healthPath, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := watcherHealingEnded(root); err == nil || !strings.Contains(err.Error(), "health breaker") {
		t.Fatalf("malformed health evidence was accepted: %v", err)
	}
}

func TestWatcherRepairAdapterTreatsMissingRequestAndHealthAsNoRepair(t *testing.T) {
	root := t.TempDir()
	held := Held{Component: Watcher, Tag: "watcher-1", Generation: 1, Identity: identity.Ref{Pid: 41, StartedAtSec: 100}}
	requested, err := (&DiskWatcherRepairs{Root: root}).WatcherRestartRequested(held)
	if err != nil || requested {
		t.Fatalf("a missing request became actionable: requested=%v err=%v", requested, err)
	}
	ended, err := watcherHealingEnded(root)
	if err != nil || ended {
		t.Fatalf("missing health evidence ended healing: ended=%v err=%v", ended, err)
	}
}

func TestCompletedWatcherRequestIsReplacedByANewGeneration(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 29, 15, 0, 0, 0, time.UTC)
	writeWatcherRepairState(t, root, 2, 42, 101, "watcher-2")
	completed := WatcherRestartRequest{
		Schema: 1, Generation: 1, Pid: 41, PidStartedAt: 100, InstanceTag: "watcher-1",
		RequestedAt: now.Add(-time.Hour), Reason: "old failure", Completed: true, CompletedAt: now.Add(-30 * time.Minute),
	}
	if err := saveWatcherRestartRequest(root, completed); err != nil {
		t.Fatal(err)
	}
	if err := RequestWatcherRestart(root, "new generation failed", now); err != nil {
		t.Fatal(err)
	}
	request, err := loadWatcherRestartRequest(root)
	if err != nil || request.Completed || request.Generation != 2 || request.Pid != 42 {
		t.Fatalf("new generation did not replace completed request: request=%+v err=%v", request, err)
	}
}

func TestWatcherRepairReadFailuresRemainRefusals(t *testing.T) {
	t.Run("end malformed request", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(SupervisionDir(root), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(watcherRestartRequestPath(root), []byte("{"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := EndWatcherRestart(root, "healing ended", time.Now()); err == nil {
			t.Fatal("a malformed request was ended as if it were valid")
		}
	})
	t.Run("adapter malformed request", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(SupervisionDir(root), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(watcherRestartRequestPath(root), []byte("{"), 0o600); err != nil {
			t.Fatal(err)
		}
		if requested, err := (&DiskWatcherRepairs{Root: root}).WatcherRestartRequested(Held{}); err == nil || requested {
			t.Fatalf("a malformed request became actionable: requested=%v err=%v", requested, err)
		}
	})
	t.Run("health path is unreadable", func(t *testing.T) {
		root := t.TempDir()
		healthPath := filepath.Join(root, "artifacts", "agents", "steward", "health.json")
		if err := os.MkdirAll(healthPath, 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := watcherHealingEnded(root); err == nil || !strings.Contains(err.Error(), "health breaker") {
			t.Fatalf("an unreadable health breaker was accepted: %v", err)
		}
	})
	t.Run("complete missing request", func(t *testing.T) {
		if err := (&DiskWatcherRepairs{Root: t.TempDir()}).CompleteWatcherRestart(Held{}, Held{}); !os.IsNotExist(err) {
			t.Fatalf("missing completion request lost its cause: %v", err)
		}
	})
}
