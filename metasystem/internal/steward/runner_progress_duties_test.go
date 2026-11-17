package steward

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/registry"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testprovider"
)

type progressBridge struct{ step func() }

func (b progressBridge) Step() string { b.step(); return "" }
func (progressBridge) Close()         {}

func TestEachLoopDutyPublishesItsStartAndItsEnd(t *testing.T) {
	t.Parallel()
	for _, atHelm := range []bool{false, true} {
		t.Run(map[bool]string{false: "seat", true: "helm"}[atHelm], func(t *testing.T) {
			t.Parallel()
			loop := newHelmLoop(t)
			now := claimQuotaClock
			deps := idleRunnerDependencies(&now)
			_, deps.Self = progressProbe(now)
			var writes []RunnerProgress
			deps.progressWrite = recordProgress(t, &writes)
			check := func(name string, external bool) {
				t.Helper()
				r := readProgress(t, loop.root)
				if r.Operation != name || r.Done != 0 {
					t.Fatalf("inside %s: %+v", name, r)
				}
				if external {
					if r.State != "waiting" || r.Reason != "external" || r.Deadline == nil || !r.Deadline.Equal(now.Add(15*time.Second)) {
						t.Fatalf("inside %s: %+v", name, r)
					}
				} else if r.State != "working" {
					t.Fatalf("inside %s: %+v", name, r)
				}
				now = now.Add(time.Millisecond)
			}
			deps.ExamineLedger = func(_ string, _ time.Time, p *Progress) error {
				if p == nil {
					t.Fatal("no publisher")
				}
				check("examine-ledger", false)
				return nil
			}
			deps.Tick = func(string, TickConfig, WorkerCensus) (TickResult, error) {
				check("tick", false)
				return TickResult{Decision: Decision{Action: ActRevive}, Seat: &SeatSelection{}}, nil
			}
			deps.SweepDisk = func(string, time.Time, bool, func()) error { check("disk-sweep", false); return nil }
			deps.Bridge = func(string) bridgeStepper { return progressBridge{func() { check("bridge", false) }} }
			deps.SettleReplies = func(context.Context, string, func()) (int, error) {
				check("question-replies", true)
				return 0, errors.New("reply transport failed")
			}
			deps.StartSeat = func(string, TickConfig, WorkerCensus, SeatSelection) (SeatRecord, error) {
				check("seat-start", false)
				return SeatRecord{}, nil
			}
			deps.Resumable = func(string) (string, bool, error) { return "prepared", true, nil }
			deps.DeliverPending = func(string, *Progress) (int, error) { check("notifications-pending", false); return 0, nil }
			deps.Channel = func(context.Context, string, func()) (int, error) { check("channel-pending", true); return 0, nil }
			deps.TrimCaches = func(context.Context, string, TickConfig) error { check("cache-trim", false); return nil }
			cfg := TickConfig{Now: now, ProviderHome: testprovider.Home(loop.root),
				Situation:            func(string, time.Time, ProgressItems) Situation { check("situation", false); return Situation{} },
				FollowDeployedEngine: func() (bool, bool, error) { check("engine-follow", false); return false, false, nil },
				CompletedBoundary:    func(string, time.Time) error { check("unit-boundary", false); return nil },
				RearmAtBoundary:      func(*Progress) (bool, error) { check("rearm", false); return false, nil },
				KeepLandingLane:      func() lane.AgentRun { check("landing-keeper", false); return lane.AgentRun{} },
			}
			if atHelm {
				takeHelmFixture(t, loop.root)
			}
			deps.Sleep = func(d time.Duration) { now = now.Add(d); loop.stop(t) }
			if err := runLoopWithDependencies(loop.root, fakeCensus{}, func(*Progress) error { check("revive", false); return nil }, time.Second, cfg, deps); err != nil {
				t.Fatal(err)
			}
			want := []string{"examine-ledger", "tick", "disk-sweep", "bridge", "question-replies", "situation", "engine-follow", "landing-keeper"}
			if !atHelm {
				want = append(want[:7], "unit-boundary", "rearm", "landing-keeper", "seat-start", "revive", "seat-recovery-requests", "notifications-pending", "channel-pending", "cache-trim")
			}
			want = append(want, "wait")
			var got []string
			for i := 0; i < len(writes); {
				first := writes[i]
				j := i + 1
				for j < len(writes) && writes[j].Operation == first.Operation {
					j++
				}
				got = append(got, first.Operation)
				if first.State != "working" || first.Done != 0 || !first.ProgressAt.Equal(first.At) {
					t.Fatalf("begin: %+v", first)
				}
				if first.Operation != "wait" {
					last := writes[j-1]
					if last.Done != 1 || last.State != "working" {
						t.Fatalf("completion: %+v", last)
					}
				}
				i = j
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("operations=%v want=%v", got, want)
			}
		})
	}
}

type progressSeatLauncher struct {
	fakeSeatLauncher
	inspect func()
}

func (l *progressSeatLauncher) StartSeat(s SeatLaunchSpec) error {
	l.inspect()
	return l.fakeSeatLauncher.StartSeat(s)
}

func progressSeatConfig(t *testing.T) (string, TickConfig) {
	t.Helper()
	root := canonicalPath(claimQuotaTickBed(t, nil, claimQuotaReady("ready-r", 1, "")))
	home := testprovider.Register(t, root)
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("launch.seat.runtime=claude\n"), 0644); err != nil {
		t.Fatal(err)
	}
	launcher := &progressSeatLauncher{inspect: func() {
		r := readProgress(t, root)
		if r.Operation != "seat-start" || r.State != "working" {
			t.Fatalf("launcher: %+v", r)
		}
	}}
	cfg := TickConfig{Now: claimQuotaClock, Seat: launcher, ProviderHome: home, WorkStateRoot: t.TempDir(), AskHome: t.TempDir(), ClaimLaneState: claimQuotaLane(t, root, nil)}
	cfg.Situation = func(string, time.Time, ProgressItems) Situation { return Situation{} }
	return root, cfg
}

func TestAWaitForArbitrationIsPublished(t *testing.T) {
	// The arbitration hook is shared package state.
	for _, duty := range []string{"seat-start", "examine-ledger", "revive", "seat-recovery-requests"} {
		t.Run(duty, func(t *testing.T) {
			loop := newHelmLoop(t)
			root := loop.root
			cfg := TickConfig{Now: claimQuotaClock, ProviderHome: testprovider.Home(root)}
			if duty == "seat-start" {
				root, cfg = progressSeatConfig(t)
				loop.root = root
			}
			now := cfg.Now
			deps := idleRunnerDependencies(&now)
			_, deps.Self = progressProbe(now)
			deps.Tick = func(string, TickConfig, WorkerCensus) (TickResult, error) {
				return TickResult{Decision: Decision{Action: ActRevive}}, nil
			}
			deps.Sleep = func(d time.Duration) { now = now.Add(d); loop.stop(t) }
			lockedRoot, wantDone := root, 0
			if duty == "seat-recovery-requests" {
				other := t.TempDir()
				cfg.Seat = &fakeSeatLauncher{}
				cfg.RecoveryRegistry = situationRegistry(t, root, other)
				seats, err := registry.HostCheckouts(cfg.RecoveryRegistry)
				if err != nil || len(seats) != 2 {
					t.Fatalf("seats=%v %v", seats, err)
				}
				lockedRoot, wantDone = seats[1].Path, 1
			}
			held, err := AcquireArbitration(lockedRoot)
			if err != nil {
				t.Fatal(err)
			}
			defer held.Release()
			previous := beforeArbitrationWait
			seen := false
			beforeArbitrationWait = func() {
				r := readProgress(t, root)
				if r.Operation != duty || r.State != "waiting" || r.Reason != "arbitration" || r.Deadline != nil || r.Done != wantDone {
					t.Errorf("queued: %+v", r)
				}
				seen = true
				held.Release()
			}
			defer func() { beforeArbitrationWait = previous }()
			var revive func(*Progress) error
			switch duty {
			case "seat-start":
				deps.Tick = func(string, TickConfig, WorkerCensus) (TickResult, error) {
					return TickResult{Decision: Decision{Action: ActRevive}, Seat: &SeatSelection{Goal: "ready-r"}}, nil
				}
				cfg.Situation = func(string, time.Time, ProgressItems) Situation {
					if readProgress(t, root).Operation == "seat-start" {
						if arb, err := TryAcquireArbitration(root); err != ErrArbitrationHeld {
							if arb != nil {
								arb.Release()
							}
							t.Errorf("recheck arbitration: %v", err)
						}
					}
					return Situation{}
				}
			case "examine-ledger":
				deps.ExamineLedger = examineLedgerMove
			case "revive":
				revive = func(p *Progress) error {
					cfg.Progress = p
					_, err := CompleteRevival(root, cfg, fakeCensus{}, "absent", nil, nil)
					return err
				}
			}
			if err := runLoopWithDependencies(root, deadCensus(), revive, time.Second, cfg, deps); err != nil {
				t.Fatal(err)
			}
			if !seen {
				t.Fatal("did not queue on arbitration")
			}
			if duty == "seat-start" && len(cfg.Seat.(*progressSeatLauncher).starts) != 1 {
				t.Fatal("production seat start did not launch")
			}
		})
	}
}

func TestReviveAndSeatStartPublishTheirExternalCalls(t *testing.T) {
	t.Parallel()
	f := newRevivalFixture(t, 1, 2)
	writeLedger(t, f.root, "# Goals\n\n## Queued goal: next-work — Repair the next thing\n- Origin: main\n- Next step: Claim it.\n")
	it := testIntent("progress-claim")
	it.Reason = "seatIdle"
	it.Goal = "next-work"
	it.ClaimNeeded = true
	it.SeatActor = &SeatActor{Machine: "machine-a", Lineage: "main-lineage"}
	if err := PrepareIntent(f.root, filepath.Join(f.root, "memory", "receipts.log"), it); err != nil {
		t.Fatal(err)
	}
	now := claimQuotaClock
	self, _ := progressProbe(now)
	var writes []RunnerProgress
	p := newProgress(f.root, self, 1, func() time.Time { return now }, recordProgress(t, &writes))
	p.Begin("revive")
	call := func(done int) {
		t.Helper()
		r := readProgress(t, f.root)
		if r.Operation != "revive" || r.State != "waiting" || r.Reason != "external" || r.Deadline != nil || r.Done != done {
			t.Fatalf("external call: %+v", r)
		}
		now = now.Add(time.Second)
	}
	out, err := f.complete(TickConfig{Progress: p}, deadCensus(), it.Nonce, func(Intent) error { call(1); return nil }, func(Intent) error { call(0); return nil })
	if err != nil || !out.Launched {
		t.Fatalf("revival=%+v %v", out, err)
	}
	for _, done := range []int{1, 2} {
		found := false
		for _, r := range writes {
			if r.Done == done && r.State == "working" {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing completion %d: %+v", done, writes)
		}
	}
	root, cfg := progressSeatConfig(t)
	p = newProgress(root, self, 1, func() time.Time { return now }, atomicfile.WriteVolatile)
	p.Begin("seat-start")
	cfg.Progress = p
	projections := 0
	cfg.GoalProjection = func(root string, at time.Time) (goal.Projection, error) {
		r := readProgress(t, root)
		if r.Operation != "seat-start" || r.State != "waiting" || r.Reason != "external" || r.Deadline != nil || r.Done != projections {
			t.Fatalf("projection: %+v", r)
		}
		projections++
		now = now.Add(time.Second)
		return defaultSeatDependencies(cfg.Seat).Project(root, at)
	}
	if _, err := startSeatWithDependencies(root, SeatSelection{Goal: "ready-r"}, seatDependenciesForConfig(cfg)); err != nil {
		t.Fatal(err)
	}
	if r := readProgress(t, root); projections == 0 || r.State != "working" || r.Done != projections {
		t.Fatalf("projections=%d final=%+v", projections, r)
	}
}
