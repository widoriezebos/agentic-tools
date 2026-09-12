package steward

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// attemptProbe answers liveness per pid for the proof-attempts role; every
// pid was started at second 100, like the records the test writes.
type attemptProbe map[int64]identity.Liveness

func (p attemptProbe) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	state, ok := p[pid]
	if !ok {
		return identity.Exact{}, identity.Dead, nil
	}
	if state == identity.Unknown {
		return identity.Exact{}, identity.Unknown, nil
	}
	return identity.Exact{Pid: pid, StartedAt: time.Unix(100, 0)}, state, nil
}

func writeAttemptRecord(t *testing.T, dir, id, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, id+".json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCheckProofAttemptsNamesADeadLauncher(t *testing.T) {
	repoRoot := t.TempDir()
	attempts := filepath.Join(repoRoot, "metasystem", "artifacts", "agents", "proof-runs", "attempts")
	if err := os.MkdirAll(attempts, 0o755); err != nil {
		t.Fatal(err)
	}
	// No attempts at all: alive.
	if role := checkProofAttempts(repoRoot, attemptProbe{}); role.Role != RoleProofAttempts || role.Status != HealthAlive {
		t.Fatalf("empty checkout = %+v", role)
	}
	// A terminal attempt is skipped whatever its launcher reads; a live one
	// with a live launcher is fine; a live one with a dead launcher is the
	// incident, named with its pid; a live one without launcher evidence is
	// unknown.
	writeAttemptRecord(t, attempts, "proof-done-1", `{"attemptId":"proof-done-1","launcher":{"pid":501,"pidStartedAt":100},"terminal":{"result":"success"}}`)
	writeAttemptRecord(t, attempts, "proof-live-1", `{"attemptId":"proof-live-1","launcher":{"pid":502,"pidStartedAt":100}}`)
	probe := attemptProbe{501: identity.Dead, 502: identity.Alive}
	if role := checkProofAttempts(repoRoot, probe); role.Status != HealthAlive {
		t.Fatalf("live launcher = %+v", role)
	}
	writeAttemptRecord(t, attempts, "proof-hung-1", `{"attemptId":"proof-hung-1","launcher":{"pid":503,"pidStartedAt":100}}`)
	probe[503] = identity.Dead
	role := checkProofAttempts(repoRoot, probe)
	if role.Status != HealthDead || !strings.Contains(role.Reason, "proof-hung-1 (launcher pid 503)") || !strings.Contains(role.Remedy, "reaper") {
		t.Fatalf("dead launcher = %+v", role)
	}
	if strings.Contains(role.Reason, "proof-done-1") || strings.Contains(role.Reason, "proof-live-1") {
		t.Fatalf("the incident named attempts that are not hung: %+v", role)
	}
	if err := os.Remove(filepath.Join(attempts, "proof-hung-1.json")); err != nil {
		t.Fatal(err)
	}
	writeAttemptRecord(t, attempts, "proof-blind-1", `{"attemptId":"proof-blind-1"}`)
	if role := checkProofAttempts(repoRoot, probe); role.Status != HealthUnknown || !strings.Contains(role.Reason, "proof-blind-1") {
		t.Fatalf("missing launcher evidence = %+v", role)
	}
}
