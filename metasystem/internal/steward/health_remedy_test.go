package steward

import (
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// The command bed runs this same fixture in a child with its own PATH.
var remedyHealthRoot = flag.String("remedy-health-root", "", "write the failed-probe fixture at this root")

func TestTickFailedCapabilityProbeKeepsRunnerAlive(t *testing.T) {
	t.Parallel()
	root := *remedyHealthRoot
	if root == "" {
		root = canonicalPath(t.TempDir())
	}
	root = canonicalPath(root)
	b := newHealthBedAt(t, root, EnrollmentFixture, "")
	b.writeFile("metasystem.conf", []byte("metasystem.runtimes=claude\n"))
	b.lookPath = func(string) (string, error) { return filepath.Join(root, "fake-path", "claude"), nil }
	// An incomplete tick cannot borrow a still-fresh earlier success.
	b.startRunner(b.runner, b.base.Add(-24*time.Hour))
	attention := newAttentionPolicyBed(t)
	attention.root, attention.now = root, b.base
	f := newTickContinuationFixture(t, attention, false)
	f.cfg.ProbeRuntime = func(string, string) error { return errors.New("fixture login unavailable") }
	f.dependencies.health = tickHealthDependencies{evaluate: b.evaluate, now: f.cfg.now,
		lookPath: b.lookPath, deliver: func(string, string) error { return nil }}
	result, completed, err := f.runAs(t, b.runner, b.generation)
	if !completed {
		// Persist the same failed completion as RunTick's defer so the public
		// command sees the failed producer when the regression is restored.
		attempt, readErr := loadComponentEvidence(ComponentEvidencePath(root, "steward-tick"))
		if readErr != nil {
			t.Fatal(readErr)
		}
		if _, writeErr := completeComponentAttempt(root, "steward-tick", b.generation, attempt.AttemptSeq,
			ComponentError, "TICK_FAILED", "tick did not complete", nil, b.base); writeErr != nil {
			t.Fatal(writeErr)
		}
	}
	view := previewHealthAtWithEvaluation(root, root, b.base, b.probe, b.evaluate)
	for _, role := range view.Roles {
		if role.Role == RoleStewardRunner && role.Status != HealthAlive ||
			role.Role == RoleCapabilitySnapshots && (role.Status != HealthDead || !strings.Contains(role.Reason, "fixture login unavailable")) {
			t.Errorf("failed probe health: %+v", role)
		}
	}
	if err != nil || !completed || !strings.Contains(result.Health.Line(), "fixture login unavailable") {
		t.Fatalf("probe stopped the tick: completed=%t err=%v health=%s", completed, err, result.Health.Line())
	}
	record, err := loadComponentEvidence(ComponentEvidencePath(root, "steward-tick"))
	if err != nil || record.Result != ComponentOK || record.Outcome != "PASS_COMPLETE" || !record.LastSuccess.Equal(b.base) {
		t.Fatalf("tick completion: %+v %v", record, err)
	}
}

func TestTickLedgerExaminationFailureStaysOnItsRole(t *testing.T) {
	t.Parallel()
	b := newHealthBed(t, EnrollmentFixture, "")
	state := ledgerAttentionState{RemoteTip: "moved", DiffedTip: "moved", ExaminedTip: "base",
		MovedAt: b.base.Add(-time.Hour).Format(time.RFC3339Nano)}
	if err := saveLedgerAttentionState(b.root, state); err != nil {
		t.Fatal(err)
	}
	deps := tickHealthDependencies{now: func() time.Time { return b.base }, deliver: func(string, string) error { return nil },
		evaluate: func(root, _ string, now time.Time, _ identity.Prober, _ bool) ([]RoleVerdict, SpendObservation) {
			return []RoleVerdict{checkLedgerAttention(root, now)}, SpendObservation{}
		}, examineLedger: func(string, time.Time) error { return errors.New("fixture ledger unreadable") }}
	var result TickResult
	if err := completeTickHealthWithDependencies(b.root, &result, b.generation, b.runner, b.base, deps); err != nil {
		t.Fatal(err)
	}
	role := checkLedgerAttention(b.root, b.base)
	if role.Status != HealthDead || !strings.Contains(role.Reason, "fixture ledger unreadable") || !strings.Contains(result.Health.Line(), "fixture ledger unreadable") {
		t.Fatalf("examination failure was lost: %+v %s", role, result.Health.Line())
	}
}

func TestTickFailedProbeWithPartialSnapshotStaysRed(t *testing.T) {
	t.Parallel()
	b := newHealthBed(t, EnrollmentFixture, "")
	b.writeFile("metasystem.conf", []byte("metasystem.runtimes=claude\n"))
	b.lookPath = func(string) (string, error) { return "/fake-path/claude", nil }
	deps := tickHealthDependencies{evaluate: b.evaluate, now: func() time.Time { return b.base }, lookPath: b.lookPath,
		deliver: func(string, string) error { return nil }, probeRuntime: func(string, string) error {
			b.writeJSON("artifacts/agents/capabilities/claude-partial.json", map[string]any{
				"runtime": "claude", "capturedAt": b.base.Format(time.RFC3339Nano)})
			return errors.New("fixture snapshot durability unknown")
		}}
	var result TickResult
	if err := completeTickHealthWithDependencies(b.root, &result, b.generation, b.runner, b.base, deps); err != nil {
		t.Fatal(err)
	}
	role, _ := capabilitySnapshotStatus(b.root, b.root, b.base, b.lookPath)
	if role.Status != HealthDead || !strings.Contains(role.Reason, "fixture snapshot durability unknown") {
		t.Fatalf("a partial snapshot hid the failed probe: %+v", role)
	}
	b.writeJSON("artifacts/agents/capabilities/claude-partial.json", map[string]any{
		"runtime": "claude", "capturedAt": b.base.Add(time.Second).Format(time.RFC3339Nano)})
	if role, _ := capabilitySnapshotStatus(b.root, b.root, b.base.Add(time.Second), b.lookPath); role.Status != HealthAlive {
		t.Fatalf("a later snapshot did not recover the role: %+v", role)
	}
}

func TestTickRemedyEvidenceWriteFailureStillFails(t *testing.T) {
	t.Parallel()
	b := newHealthBed(t, EnrollmentFixture, "")
	b.writeFile("metasystem.conf", []byte("metasystem.runtimes=claude\n"))
	b.lookPath = func(string) (string, error) { return "/fake-path/claude", nil }
	deps := tickHealthDependencies{evaluate: b.evaluate, now: func() time.Time { return b.base }, lookPath: b.lookPath,
		deliver: func(string, string) error { return nil }, probeRuntime: func(string, string) error {
			path := ComponentEvidencePath(b.root, "capability-probe-claude")
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(path, 0o755); err != nil {
				t.Fatal(err)
			}
			return errors.New("fixture login unavailable")
		}}
	var result TickResult
	if err := completeTickHealthWithDependencies(b.root, &result, b.generation, b.runner, b.base, deps); err == nil {
		t.Fatal("a remedy result that could not be recorded completed the tick")
	}
}
