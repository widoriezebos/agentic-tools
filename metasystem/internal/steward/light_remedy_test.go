package steward

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testprovider"
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
				deps.ProviderHome = testprovider.Home(loop.root)
				deps.Tick = func(root string, cfg TickConfig, _ WorkerCensus) (TickResult, error) {
					scheduled++
					var result TickResult
					if err := completeTickHealthWithDependencies(root, &result, b.generation, b.runner, now, healthDeps); err != nil {
						t.Fatal(err)
					}
					return result, nil
				}
				deps.Light = func(root string, cfg TickConfig, census WorkerCensus) (TickResult, error) {
					requests++
					return runLightDecisions(root, cfg, census)
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
