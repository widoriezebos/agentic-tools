package steward

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/registry"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testprovider"
)

func TestAStoppedRunnerBeginsNoFurtherDuty(t *testing.T) {
	t.Parallel()
	duties := []string{"tick", "disk-sweep", "bridge", "question-replies", "situation", "engine-follow", "unit-boundary", "rearm", "landing-keeper", "seat-start", "revive", "seat-recovery-requests", "notifications-pending", "channel-pending"}
	for _, duty := range duties {
		for _, signal := range []bool{false, true} {
			t.Run(duty+map[bool]string{false: "/file", true: "/signal"}[signal], func(t *testing.T) {
				t.Parallel()
				loop := newHelmLoop(t)
				now := claimQuotaClock
				fake := newFakeStopSignals()
				stopped := false
				stopAt := func(operation string) {
					t.Helper()
					if operation != duty || stopped {
						return
					}
					stopped = true
					if signal {
						relay := receiveWithin(t, fake.signals, "signal relay")
						relay <- syscall.SIGTERM
						receiveWithin(t, fake.reported, "drain report")
					} else {
						loop.stop(t)
					}
				}
				deps := idleRunnerDependencies(&now)
				_, deps.Self = progressProbe(now)
				deps.StopSignals = fake.source(t)
				var writes []RunnerProgress
				write := recordProgress(t, &writes)
				deps.progressWrite = func(path, contents string) error {
					err := write(path, contents)
					r := writes[len(writes)-1]
					if r.Operation == "seat-recovery-requests" && r.State == "working" && r.Done == 0 {
						stopAt(r.Operation)
					}
					return err
				}
				deps.Tick = func(string, TickConfig, WorkerCensus) (TickResult, error) {
					stopAt("tick")
					return TickResult{Decision: Decision{Action: ActRevive}, Seat: &SeatSelection{}}, nil
				}
				deps.SweepDisk = func(string, time.Time, bool, func()) error { stopAt("disk-sweep"); return nil }
				deps.Bridge = func(string) bridgeStepper { return progressBridge{func() { stopAt("bridge") }} }
				deps.SettleReplies = func(context.Context, string, func()) (int, error) { stopAt("question-replies"); return 0, nil }
				deps.StartSeat = func(string, TickConfig, WorkerCensus, SeatSelection) (SeatRecord, error) {
					stopAt("seat-start")
					return SeatRecord{}, nil
				}
				deps.Resumable = func(string) (string, bool, error) { return "prepared", true, nil }
				deps.DeliverPending = func(string, *Progress) (int, error) { stopAt("notifications-pending"); return 0, nil }
				deps.Channel = func(context.Context, string, func()) (int, error) { stopAt("channel-pending"); return 0, nil }
				deps.TrimCaches = func(context.Context, string, TickConfig) error { t.Fatal("cache trim began after stop"); return nil }
				deps.Sleep = func(time.Duration) { t.Fatal("the stopped runner slept") }
				cfg := TickConfig{Now: now, ProviderHome: testprovider.Home(loop.root),
					Situation:            func(string, time.Time, ProgressItems) Situation { stopAt("situation"); return Situation{} },
					FollowDeployedEngine: func() (bool, bool, error) { stopAt("engine-follow"); return false, false, nil },
					CompletedBoundary:    func(string, time.Time) error { stopAt("unit-boundary"); return nil },
					RearmAtBoundary:      func(*Progress) (bool, error) { stopAt("rearm"); return false, nil },
					KeepLandingLane:      func() lane.AgentRun { stopAt("landing-keeper"); return lane.AgentRun{} },
				}
				if err := runLoopWithDependencies(loop.root, fakeCensus{}, func(*Progress) error { stopAt("revive"); return nil }, time.Second, cfg, deps); err != nil {
					t.Fatal(err)
				}
				last := writes[len(writes)-1]
				if last.Operation != duty || last.State != "working" || last.Done != 1 || last.OKTicks != 1 {
					t.Fatalf("drain must finish %s without beginning another duty: %+v", duty, writes)
				}
				receiveWithin(t, fake.stopped, "signal relay stop")
				select {
				case code := <-fake.exits:
					t.Fatalf("hard exit: %d", code)
				default:
				}
				log, err := os.ReadFile(runnerLogPath(loop.root))
				if err != nil || strings.Count(string(log), "tick 1:") != 1 {
					t.Fatalf("tick timing: %s %v", log, err)
				}
			})
		}
	}
}

func TestASeatStartStopsBeforeItsLaunchRecord(t *testing.T) {
	t.Parallel()
	for _, duringLaunch := range []bool{false, true} {
		t.Run(map[bool]string{false: "recheck", true: "launch"}[duringLaunch], func(t *testing.T) {
			t.Parallel()
			root, cfg := progressSeatConfig(t)
			loop := &helmLoop{root: root}
			now := cfg.Now
			before, err := LoadEvidence(EvidencePath(root))
			if err != nil {
				t.Fatal(err)
			}
			rechecked := false
			cfg.Situation = func(string, time.Time, ProgressItems) Situation {
				if readProgress(t, root).Operation == "seat-start" {
					rechecked = true
					held, err := TryAcquireArbitration(root)
					if held != nil {
						held.Release()
					}
					if err != ErrArbitrationHeld {
						t.Fatalf("recheck arbitration: %v", err)
					}
					if !duringLaunch {
						loop.stop(t)
					}
				}
				return Situation{}
			}
			launcher := cfg.Seat.(*progressSeatLauncher)
			inspect := launcher.inspect
			launcher.inspect = func() { inspect(); loop.stop(t) }
			deps := idleRunnerDependencies(&now)
			_, deps.Self = progressProbe(now)
			fake := newFakeStopSignals()
			deps.StopSignals = fake.source(t)
			var writes []RunnerProgress
			deps.progressWrite = recordProgress(t, &writes)
			deps.Tick = func(string, TickConfig, WorkerCensus) (TickResult, error) {
				return TickResult{Decision: Decision{Action: ActRevive}, Seat: &SeatSelection{Goal: "ready-r"}}, nil
			}
			deps.Sleep = func(time.Duration) { t.Fatal("stopped seat start slept") }
			if err := runLoopWithDependencies(root, deadCensus(), nil, time.Second, cfg, deps); err != nil {
				t.Fatal(err)
			}
			files, err := os.ReadDir(seatsDir(root))
			if err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			records, err := readSeatRecords(root)
			if err != nil || !rechecked || len(launcher.starts) != boolInt(duringLaunch) || len(records) != boolInt(duringLaunch) || len(files) != 2*boolInt(duringLaunch) {
				t.Fatalf("rechecked=%v starts=%v records=%v files=%v err=%v", rechecked, launcher.starts, records, files, err)
			}
			after, err := LoadEvidence(EvidencePath(root))
			if err != nil || after.AbnormalCount != before.AbnormalCount {
				t.Fatalf("abnormal reservations: before=%+v after=%+v err=%v", before, after, err)
			}
			pending, err := PendingNotifications(root)
			if err != nil {
				t.Fatal(err)
			}
			for _, n := range pending {
				if n.Nonce == "seat-start-failure" {
					t.Fatalf("a drained seat start queued failure: %+v", n)
				}
			}
			last := writes[len(writes)-1]
			if last.Operation != "seat-start" || last.Done == 0 {
				t.Fatalf("a duty began after seat start: %+v", writes)
			}
		})
	}
}

func TestNotificationsAndRecoveryStopAfterTheCurrentItem(t *testing.T) {
	t.Parallel()
	t.Run("notifications", func(t *testing.T) {
		t.Parallel()
		root, now := t.TempDir(), claimQuotaClock
		for _, nonce := range []string{"a", "b", "c"} {
			if err := QueueNotification(root, PendingNotification{Nonce: nonce, Message: nonce}); err != nil {
				t.Fatal(err)
			}
		}
		stopping, sends := false, 0
		p := itemsProgress(t, root, &now)
		p.stopping = func() bool { return stopping }
		p.Begin("notifications-pending")
		delivered, err := deliverPendingWithDependencies(root, pendingDeliveryDependencies{progress: p,
			funnel: noticeFunnel{decider: allowEveryNotice}, transport: func(string, string) error { sends++; stopping = true; return nil }})
		pending, readErr := PendingNotifications(root)
		if err != nil || readErr != nil || delivered != 1 || sends != 1 || len(pending) != 2 || readProgress(t, root).Done != 1 {
			t.Fatalf("delivered=%d sends=%d pending=%v errors=%v %v progress=%+v", delivered, sends, pending, err, readErr, readProgress(t, root))
		}
	})
	t.Run("recovery", func(t *testing.T) {
		t.Parallel()
		root, cfg, seats := drainRecoveryConfig(t)
		now, stopping, waits := claimQuotaClock, false, 0
		var writes []RunnerProgress
		write := recordProgress(t, &writes)
		self, _ := progressProbe(now)
		cfg.Progress = newProgress(root, self, 1, func() time.Time { return now }, func(path, contents string) error {
			err := write(path, contents)
			if writes[len(writes)-1].State == "waiting" {
				waits++
				stopping = true
			}
			return err
		})
		cfg.Progress.stopping = func() bool { return stopping }
		cfg.Progress.Begin("seat-recovery-requests")
		err := ReconcileRecoveryRequests(root, cfg, fakeCensus{})
		if err == nil || !strings.Contains(err.Error(), seats[0].Path) || strings.Contains(err.Error(), seats[1].Path) || waits != 1 || readProgress(t, root).Done != 1 {
			t.Fatalf("recovery=%v waits=%d writes=%+v", err, waits, writes)
		}
	})
}

func drainRecoveryConfig(t *testing.T) (string, TickConfig, []registry.HostCheckout) {
	t.Helper()
	loop := newHelmLoop(t)
	cfg := TickConfig{Now: claimQuotaClock, ProviderHome: testprovider.Home(loop.root), Seat: &fakeSeatLauncher{}, RecoveryRegistry: situationRegistry(t, loop.root, t.TempDir())}
	seats, err := registry.HostCheckouts(cfg.RecoveryRegistry)
	if err != nil || len(seats) != 2 {
		t.Fatalf("registry: %v %v", seats, err)
	}
	for _, seat := range seats {
		if err := os.MkdirAll(seatsDir(seat.Path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(seatRecordPath(seat.Path, "unreadable"), []byte("{"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return loop.root, cfg, seats
}

func TestAPersonsReviveAndRecoveryNeverDrain(t *testing.T) {
	t.Parallel()
	root, cfg, seats := drainRecoveryConfig(t)
	(&helmLoop{root: root}).stop(t)
	err := ReconcileRecoveryRequests(root, cfg, fakeCensus{})
	if err == nil || !strings.Contains(err.Error(), seats[0].Path) || !strings.Contains(err.Error(), seats[1].Path) || cfg.Progress.Stopping() || (&Progress{}).Stopping() {
		t.Fatalf("manual recovery did not reach both seats: %v", err)
	}
	file := claimQuotaHeld("held")
	file.Claimed.Lineage = SeatLineage
	root = canonicalPath(claimQuotaTickBed(t, nil, file))
	bin := filepath.Join(canonicalPath(t.TempDir()), "engine")
	if err := testexec.WriteFile(bin, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	digest, err := installDigest(bin)
	if err != nil {
		t.Fatal(err)
	}
	if err := MintIdentity(RepoIdentityPath(root), InstallIdentity{RepoIdentity: root, Generation: 1, InstallPath: bin, InstallDigest: digest}); err != nil {
		t.Fatal(err)
	}
	if _, err := humanauthority.Enroll(root, 20, stewardPersonTree{}, "Wido", claimQuotaClock); err != nil {
		t.Fatal(err)
	}
	person, err := humanauthority.Prove(root, 20, stewardPersonTree{}, claimQuotaClock)
	if err != nil {
		t.Fatal(err)
	}
	last := SeatRecord{Schema: 1, LaunchID: "failed", Goal: file.Id, Held: true, ApprovalOpid: approvalOpid(file), StartedAt: claimQuotaClock.Add(-time.Hour).Format(time.RFC3339)}
	if err := writeSeatRecord(root, last); err != nil {
		t.Fatal(err)
	}
	launcher := &fakeSeatLauncher{states: map[string]SeatLaunchState{last.LaunchID: {Found: true, Terminal: true, State: "failed"}}}
	cfg = TickConfig{Now: claimQuotaClock, ProviderHome: testprovider.Register(t, root), Seat: launcher, Prober: deadProber{}, WorkStateRoot: t.TempDir()}
	(&helmLoop{root: root}).stop(t)
	record, err := ReviveSeat(root, cfg, deadCensus(), last.LaunchID, person, situationRegistry(t, root))
	if err != nil || record.LaunchID == "" || len(launcher.starts) != 1 {
		t.Fatalf("manual revive=%+v starts=%v err=%v", record, launcher.starts, err)
	}
}
