package steward

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testprovider"
)

func itemsProgress(t *testing.T, root string, now *time.Time) *Progress {
	t.Helper()
	self, _ := progressProbe(*now)
	return newProgress(root, self, 1, func() time.Time { return *now }, atomicfile.WriteVolatile)
}

func TestEachNotificationCountsAfterItsSend(t *testing.T) {
	t.Parallel()
	for _, fails := range []bool{false, true} {
		t.Run(map[bool]string{false: "delivered", true: "transport error"}[fails], func(t *testing.T) {
			root, intent := prepareRevivalHandoff(t, "5000000000000099")
			for _, n := range []PendingNotification{
				{Nonce: "a-legacy", DeliveryOwner: legacyHealthDeliveryOwner},
				{Nonce: handoffNoticeNonce(intent.Nonce), Message: "handoff"},
				{Nonce: "z-plain", Message: "plain"},
			} {
				if err := QueueNotification(root, n); err != nil {
					t.Fatal(err)
				}
			}
			now := claimQuotaClock
			p := itemsProgress(t, root, &now)
			p.Begin("notifications-pending")
			calls := 0
			delivered, err := deliverPendingWithDependencies(root, pendingDeliveryDependencies{progress: p,
				funnel: noticeFunnel{decider: allowEveryNotice}, transport: func(string, string) error {
					r := readProgress(t, root)
					if r.Done != calls+1 || r.State != "waiting" || r.Reason != "external" || r.Deadline == nil || !r.Deadline.Equal(now.Add(notifyTimeout)) {
						t.Fatalf("send in progress: %+v, previous sends=%d", r, calls)
					}
					now = now.Add(time.Second)
					calls++
					if fails {
						return errors.New("transport failed")
					}
					return nil
				}})
			want := 3
			if fails {
				want = 2
			}
			r := readProgress(t, root)
			if (err != nil) != fails || r.Done != want || r.State != "working" || delivered != calls-boolInt(fails) {
				t.Fatalf("delivered=%d err=%v progress=%+v", delivered, err, r)
			}
		})
	}
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func TestTheLoopHandsItsPublisherToEachMultiItemDuty(t *testing.T) {
	t.Parallel()
	loop := newHelmLoop(t)
	now := claimQuotaClock
	deps := idleRunnerDependencies(&now)
	deps.Tick = func(string, TickConfig, WorkerCensus) (TickResult, error) { return TickResult{}, nil }
	var writes []RunnerProgress
	deps.progressWrite = recordProgress(t, &writes)
	item := func(name string, callback func()) {
		t.Helper()
		if callback == nil {
			t.Fatalf("%s received no item", name)
		}
		if r := readProgress(t, loop.root); r.Operation != name || r.Done != 0 {
			t.Fatalf("before item: %+v", r)
		}
		callback()
		if r := readProgress(t, loop.root); r.Done != 1 {
			t.Fatalf("after item: %+v", r)
		}
	}
	deps.DeliverPending = func(_ string, p *Progress) (int, error) {
		if p == nil {
			t.Fatal("no publisher")
		}
		item("notifications-pending", p.Item)
		return 0, nil
	}
	deps.Channel = func(_ context.Context, _ string, callback func()) (int, error) {
		item("channel-pending", callback)
		return 0, nil
	}
	deps.SettleReplies = func(_ context.Context, _ string, callback func()) (int, error) {
		item("question-replies", callback)
		return 0, nil
	}
	deps.SweepDisk = func(_ string, _ time.Time, _ bool, callback func()) error { item("disk-sweep", callback); return nil }
	deps.Sleep = func(d time.Duration) { now = now.Add(d); loop.stop(t) }
	if err := runLoopWithDependencies(loop.root, fakeCensus{}, nil, time.Second, TickConfig{ProviderHome: testprovider.Home(loop.root)}, deps); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"notifications-pending", "channel-pending", "question-replies", "disk-sweep"} {
		var last RunnerProgress
		for _, r := range writes {
			if r.Operation == name {
				last = r
			}
		}
		if last.Done != 2 {
			t.Fatalf("%s completion: %+v", name, last)
		}
	}
}

func populatedItemSources(t *testing.T, root string, p *Progress) SituationSources {
	t.Helper()
	src := situationHostSources(t, situationRegistry(t, t.TempDir(), t.TempDir()))
	src.Progress = p
	src.Machine = func(string) (string, error) { return "m1e", nil }
	src.Processes = identity.FixedProcessTable{{Pid: 21}, {Pid: 22}, {Pid: 23}}
	pids := 0
	src.Argv = func(int64) ([]string, bool) {
		if r := readProgress(t, root); r.Done != 8+pids {
			t.Fatalf("pid in progress: %+v, previous pids=%d", r, pids)
		}
		pids++
		if pids == 3 {
			return []string{"go", "test", "./..."}, true
		}
		return nil, false
	}
	holders := 0
	src.Holder = func(string) (SeatHolder, error) {
		if r := readProgress(t, root); r.Done != 11+holders {
			t.Fatalf("holder in progress: %+v, previous holders=%d", r, holders)
		}
		holders++
		return SeatHolder{}, nil
	}
	src.ClaimLaneState = claimQuotaLane(t, root, nil)
	src.Waiting = func(string) ([]string, error) { return nil, nil }
	src.BoardHome = func() (string, error) { return t.TempDir(), nil }
	src.Launches = func() ([]launch.Record, error) { return nil, nil }
	src.Units = func(string, string, goal.Projection, time.Time) ([]UnitStage, error) { return nil, nil }
	return src
}

func TestTheSituationCountsEachSourceAndItem(t *testing.T) {
	t.Parallel()
	root := claimQuotaTickBed(t, nil, claimQuotaHeld("held-h"))
	now := claimQuotaClock
	p := itemsProgress(t, root, &now)
	p.Begin("situation")
	src := populatedItemSources(t, root, p)
	s := readSituation(t, root, src)
	// Thirteen sources run without a red line, plus one held goal's facts,
	// three pids (including skipped processes), and two armed holders.
	if r := readProgress(t, root); r.Done != 19 || len(s.Holders) != 2 || len(s.GoalFacts) != 1 {
		t.Fatalf("sources and items: %+v holders=%d facts=%v unavailable=%v", r, len(s.Holders), s.GoalFacts, s.Unavailable)
	}
}

func TestTheSituationPublishesUnderTheSeatStartsArbitration(t *testing.T) {
	t.Parallel()
	root, cfg := progressSeatConfig(t)
	now := claimQuotaClock
	deps := idleRunnerDependencies(&now)
	deps.Tick = func(string, TickConfig, WorkerCensus) (TickResult, error) {
		return TickResult{Decision: Decision{Action: ActRevive}, Seat: &SeatSelection{Goal: "ready-r"}}, nil
	}
	rechecks := 0
	cfg.Situation = func(root string, at time.Time, progress ProgressItems) Situation {
		src := withEmptyHost(t, SituationSources{Now: func() time.Time { return at }, Progress: progress})
		src.Waiting = func(string) ([]string, error) {
			r := readProgress(t, root)
			if r.Operation == "seat-start" {
				rechecks++
				if held, err := TryAcquireArbitration(root); err != ErrArbitrationHeld {
					if held != nil {
						held.Release()
					}
					t.Fatalf("arbitration=%v", err)
				}
				if r.Done < 2 {
					t.Fatalf("sources not published under arbitration: %+v", r)
				}
			}
			return nil, nil
		}
		return readSituation(t, root, src)
	}
	deps.Sleep = func(d time.Duration) {
		now = now.Add(d)
		if err := stopRunnerLoop(root); err != nil {
			t.Fatal(err)
		}
	}
	if err := runLoopWithDependencies(root, deadCensus(), nil, time.Second, cfg, deps); err != nil {
		t.Fatal(err)
	}
	if rechecks == 0 {
		t.Fatal("no situation composed under seat start")
	}
}

func TestAPersonsSituationAndDiskCleanPublishNothing(t *testing.T) {
	t.Parallel()
	bed := newStaleBed(t)
	now := claimQuotaClock
	p := itemsProgress(t, bed.inst, &now)
	p.Begin("resident")
	before, err := os.ReadFile(runnerProgressPath(bed.inst))
	if err != nil {
		t.Fatal(err)
	}
	src := withEmptyHost(t, SituationSources{Now: func() time.Time { return now }})
	readSituation(t, bed.inst, src)
	if _, err := SweepDiskStores(context.Background(), bed.inst, bed.pass(diskstore.ModeReport, []string{bed.inst}, false)); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(runnerProgressPath(bed.inst))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("manual read changed progress: %v", err)
	}
}

func TestDiskAndCachePassesCarryTheirItems(t *testing.T) {
	t.Parallel()
	bed := newStaleBed(t)
	pass := bed.pass(diskstore.ModeReport, []string{bed.inst}, false)
	count := 0
	pass.Item = func() { count++ }
	result, err := SweepDiskStores(context.Background(), bed.inst, pass)
	if err != nil || !result.Machined || len(result.Checkout.Classes) == 0 || len(result.Machine.Classes) == 0 || count != len(result.Checkout.Classes)+len(result.Machine.Classes) {
		t.Fatalf("count=%d result=%+v err=%v", count, result, err)
	}
	now := claimQuotaClock
	p := itemsProgress(t, bed.inst, &now)
	p.Begin("cache-trim")
	state := filepath.Join(t.TempDir(), "cache-trim")
	if err := machineCacheTrimmer(func() (string, error) { return t.TempDir(), nil }, state)(context.Background(), bed.inst, TickConfig{Now: now, Progress: p}); err != nil {
		t.Fatal(err)
	}
	if r := readProgress(t, bed.inst); r.Done != 4 {
		t.Fatalf("cache items: %+v", r)
	}
}
