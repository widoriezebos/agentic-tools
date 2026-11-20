package steward

import (
	"errors"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/tickrequest"
)

func TestRunnerRequestsPreserveHealthRemedyAttempts(t *testing.T) {
	t.Parallel()
	for _, role := range []HealthRole{RoleCapabilitySnapshots, RoleLedgerAttention} {
		t.Run(string(role), func(t *testing.T) {
			var baseline []time.Duration
			for _, busy := range []bool{false, true} {
				loop := newHelmLoop(t)
				b := newHealthBedAt(t, loop.root, EnrollmentFixture, "")
				b.writeFile("metasystem.conf", []byte("metasystem.runtimes=claude\nlaunch.seat.runtime=claude\n"))
				b.lookPath = func(string) (string, error) { return "/fake-path/claude", nil }
				if err := saveLedgerAttentionState(b.root, ledgerAttentionState{
					RemoteTip: "moved", DiffedTip: "moved", ExaminedTip: "base",
					MovedAt: b.base.Add(-time.Hour).Format(time.RFC3339Nano),
				}); err != nil {
					t.Fatal(err)
				}
				now := b.base
				var attempts []time.Duration
				fail := func() error {
					attempts = append(attempts, now.Sub(b.base))
					return errors.New("fixture remedy unavailable")
				}
				healthDeps := tickHealthDependencies{
					now: func() time.Time { return now }, lookPath: b.lookPath,
					deliver:       func(string, string) error { return nil },
					probeRuntime:  func(string, string) error { return fail() },
					examineLedger: func(string, time.Time) error { return fail() },
					evaluate: func(root, _ string, at time.Time, _ identity.Prober, _ bool) ([]RoleVerdict, SpendObservation) {
						if role == RoleLedgerAttention {
							return []RoleVerdict{checkLedgerAttention(root, at)}, SpendObservation{}
						}
						status, _ := capabilitySnapshotStatus(root, root, at, b.lookPath)
						return []RoleVerdict{status}, SpendObservation{}
					},
				}
				requests, scheduled := 0, 0
				deps := idleRunnerDependencies(&now)
				deps.Tick = func(root string, cfg TickConfig, _ WorkerCensus) (TickResult, error) {
					if cfg.RequestDriven {
						requests++
					} else {
						scheduled++
					}
					healthDeps.requestDriven = cfg.RequestDriven
					var result TickResult
					if err := completeTickHealthWithDependencies(root, &result, b.generation, b.runner, now, healthDeps); err != nil {
						t.Fatal(err)
					}
					if cfg.RequestDriven && scheduled < healthFailureLimit {
						record, err := loadHealthRecord(HealthRecordPath(root))
						if err != nil || !strings.Contains(record.Verdict.Line(), "skipped until scheduled tick") {
							t.Errorf("request cycle did not record the skipped remedy: %s, %v", record.Verdict.Line(), err)
						}
						narration, err := os.ReadFile(NarrationPath(root))
						if err != nil || !strings.Contains(string(narration), "skipped until scheduled tick") {
							t.Fatalf("request cycle did not narrate the skipped remedy: %v", err)
						}
					}
					return result, nil
				}
				deps.Sleep = func(d time.Duration) {
					before := now
					now = now.Add(d)
					if busy && before.Unix() != now.Unix() {
						if err := tickrequest.Write(loop.root, "launch-ended:fixture", now); err != nil {
							t.Fatal(err)
						}
					}
					if now.Sub(b.base) >= 3001*time.Second {
						loop.stop(t)
					}
				}
				if err := runLoopWithDependencies(loop.root, fakeCensus{}, nil, 600*time.Second, TickConfig{Now: now}, deps); err != nil {
					t.Fatal(err)
				}
				if scheduled != 6 || busy && requests != 40 || !busy && requests != 0 {
					t.Fatalf("busy=%t scheduled=%d requests=%d", busy, scheduled, requests)
				}
				record, err := loadHealthRecord(HealthRecordPath(loop.root))
				if err != nil || record.Verdict.Roles[0].FailureEscalation != AutoHealEnded || record.State.FailureCounts[role] != healthFailureLimit {
					t.Fatalf("scheduled observations did not end healing: %+v, %v", record, err)
				}
				component := "capability-probe-claude"
				if role == RoleLedgerAttention {
					component = "ledger-examination"
				}
				remedy, err := loadComponentEvidence(ComponentEvidencePath(loop.root, component))
				if err != nil || remedy.AttemptSeq != int64(len(attempts)) {
					t.Fatalf("remedy attempt record: %+v, %v", remedy, err)
				}
				if !busy {
					baseline = attempts
					if len(baseline) != healthFailureLimit-1 {
						t.Fatalf("baseline attempts=%v", baseline)
					}
				} else if !reflect.DeepEqual(attempts, baseline) {
					t.Fatalf("requests changed remedy attempts: got %v want %v", attempts, baseline)
				}
			}
		})
	}
}

func TestRequestTickContinuationPreservesHealthRemedyPatience(t *testing.T) {
	t.Parallel()
	b := newHealthBed(t, EnrollmentFixture, "")
	b.writeFile("metasystem.conf", []byte("metasystem.runtimes=claude\n"))
	b.lookPath = func(string) (string, error) { return "/fake-path/claude", nil }
	attention := newAttentionPolicyBed(t)
	attention.root, attention.now = b.root, b.base
	f := newTickContinuationFixture(t, attention, false)
	f.cfg.RequestDriven = true
	attempts := 0
	f.cfg.ProbeRuntime = func(string, string) error {
		attempts++
		return errors.New("fixture login unavailable")
	}
	f.dependencies.health = tickHealthDependencies{evaluate: b.evaluate, now: f.cfg.now,
		lookPath: b.lookPath, deliver: func(string, string) error { return nil }}
	result, completed, err := f.runAs(t, b.runner, b.generation)
	if err != nil || !completed {
		t.Fatalf("request continuation: completed=%t err=%v", completed, err)
	}
	if attempts != 0 || result.Health.State.FailureCounts[RoleCapabilitySnapshots] != 0 {
		t.Fatalf("request spent health patience or attempted repair: attempts=%d health=%s", attempts, result.Health.Line())
	}
	if !strings.Contains(result.Health.Line(), "skipped until scheduled tick") {
		t.Fatalf("request did not explain the skipped remedy: %s", result.Health.Line())
	}
}

// The command bed runs this same fixture in a child with its own PATH.
var remedyHealthRoot = flag.String("remedy-health-root", "", "write the failed-probe fixture at this root")
var remedyHealthObservations = flag.Int("remedy-health-observations", 1, "number of failed remedy observations in the command fixture")

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
	for observation := 1; observation < *remedyHealthObservations && err == nil && completed; observation++ {
		deps := f.dependencies.health
		deps.probeRuntime = f.cfg.ProbeRuntime
		err = completeTickHealthWithDependencies(root, &result, b.generation, b.runner, b.base, deps)
	}
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
	root := *remedyHealthRoot
	if root == "" {
		root = canonicalPath(t.TempDir())
	}
	b := newHealthBedAt(t, canonicalPath(root), EnrollmentFixture, "")
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
	for observation := 0; observation < *remedyHealthObservations; observation++ {
		if err := completeTickHealthWithDependencies(b.root, &result, b.generation, b.runner, b.base, deps); err != nil {
			t.Fatal(err)
		}
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
