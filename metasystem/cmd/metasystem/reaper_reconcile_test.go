package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

// The reaper's tick reconciles proof attempts on the installation root: an
// attempt whose launcher pid no longer exists is terminal after one pass,
// with its cause, through the same function the component wires
// (goal hung-proof-attempts-end-at-their-deadline).
func TestReaperTickReconcilesAnAttemptWhoseLauncherIsGone(t *testing.T) {
	repo := t.TempDir()
	root := filepath.Join(repo, "metasystem")
	for _, dir := range []string{filepath.Join(root, "scripts", "agents"), filepath.Join(root, "artifacts", "agents", "jobs")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	conf := filepath.Join(root, "metasystem.conf")
	if err := os.WriteFile(conf, []byte("dispatch.cap-max=120\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"coverage-ratchet.json", "coverage-ratchet-linux.json"} {
		if err := os.WriteFile(filepath.Join(root, "scripts", "agents", name), []byte(`{"floors":{"internal/proofrun":1},"exempt":{}}`), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	identity, err := proofrun.BuildProofIdentity(root, conf, "full", "reaper-wiring", []string{"gate"}, behaviorsurface.SupportedVersion)
	if err != nil {
		t.Fatal(err)
	}
	// A pid that no such process holds: the kernel prober answers Dead.
	launcher := proofrun.ProcessIdentity{Pid: 1<<30 - 7, PidStartedAt: 100}
	attempt, result, err := proofrun.ReserveLocked(proofrun.AdmissionRequest{ControlRoot: root, ExecutionRoot: root, GoalID: "goal-a",
		GoalRevision: 3, AccountingRevision: 2, ReservedMinutes: 4, Identity: identity, Launcher: launcher, Now: time.Now().UTC()})
	if err != nil || result.Disposition != proofrun.DispositionExecuted {
		t.Fatalf("reserve = %+v, %+v, %v", attempt, result, err)
	}
	setupReaper(repo, root)()
	after, err := proofrun.ReadAttempt(root, attempt.AttemptID)
	if err != nil || after.Terminal == nil || after.Terminal.Result != proofrun.TerminalFailed {
		t.Fatalf("attempt after the reaper tick = %+v, %v", after.Terminal, err)
	}
	live, err := proofrun.LiveAttempts(root, nil)
	if err != nil || len(live) != 0 {
		t.Fatalf("live attempts after the tick = %+v, %v", live, err)
	}
}
