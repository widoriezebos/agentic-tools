package lock

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func alive(Identity) Liveness   { return Alive }
func dead(Identity) Liveness    { return Dead }
func unknown(Identity) Liveness { return Unknown }

type fakeClock struct {
	mu    sync.Mutex
	now   time.Time
	slept time.Duration
}

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Sleep(duration time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(duration)
	c.slept += duration
}

func (c *fakeClock) Slept() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.slept
}

func (c *fakeClock) options(wait, poll time.Duration, probe Probe) Options {
	return Options{Wait: wait, Poll: poll, Now: c.Now, Sleep: c.Sleep, Probe: probe}
}

type contentionState int

const (
	contentionRunning contentionState = iota
	contentionBlocked
	contentionHolding
	contentionDone
)

type contentionSchedule struct {
	mu     sync.Mutex
	ready  *sync.Cond
	states []contentionState
}

func newContentionSchedule(workers int) *contentionSchedule {
	schedule := &contentionSchedule{states: make([]contentionState, workers)}
	schedule.ready = sync.NewCond(&schedule.mu)
	return schedule
}

func (s *contentionSchedule) sleep(worker int) func(time.Duration) {
	return func(time.Duration) {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.states[worker] = contentionBlocked
		s.ready.Broadcast()
		for s.states[worker] == contentionBlocked {
			s.ready.Wait()
		}
	}
}

func (s *contentionSchedule) waitUntilOthersBlocked(worker int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.states[worker] = contentionHolding
	for {
		allBlocked := true
		for other, state := range s.states {
			if other != worker && state != contentionBlocked && state != contentionDone {
				allBlocked = false
				break
			}
		}
		if allBlocked {
			return
		}
		s.ready.Wait()
	}
}

func (s *contentionSchedule) finishAndWakeNext(worker int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.states[worker] = contentionDone
	for other, state := range s.states {
		if state == contentionBlocked {
			s.states[other] = contentionRunning
			break
		}
	}
	s.ready.Broadcast()
}

func lockPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "registry.lock.d")
}

func TestAcquireReleaseRoundTrip(t *testing.T) {
	path := lockPath(t)
	self := Identity{Pid: 100, PidStartedAt: 1, Tag: "t"}
	held, err := Acquire(path, self, Options{Wait: time.Second, Probe: alive})
	if err != nil {
		t.Fatal(err)
	}
	holder, err := Holder(path)
	if err != nil || holder != self {
		t.Fatalf("lock does not name its holder: %+v %v", holder, err)
	}
	if err := held.Release(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("release left the lock in place")
	}
}

// SLC-R8-001: the lock is born owning. While N acquirers race, an
// observer polling the lock path must never see a lock directory
// without a readable owner file — there is no ownerless window.
func TestNoOwnerlessWindowUnderContention(t *testing.T) {
	path := lockPath(t)
	const workerCount = 8
	clock := newFakeClock()
	schedule := newContentionSchedule(workerCount)
	var ownerless atomic.Int64
	stop := make(chan struct{})
	var observer sync.WaitGroup
	observer.Add(1)
	go func() {
		defer observer.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			if info, err := os.Stat(path); err == nil && info.IsDir() {
				if _, err := os.ReadFile(filepath.Join(path, "owner.json")); os.IsNotExist(err) {
					// Distinguish a mid-release rename (lock gone on
					// re-check) from a genuinely ownerless lock.
					if again, err2 := os.Stat(path); err2 == nil && again.IsDir() {
						if _, err3 := os.ReadFile(filepath.Join(path, "owner.json")); os.IsNotExist(err3) {
							ownerless.Add(1)
						}
					}
				}
			}
		}
	}()

	var workers sync.WaitGroup
	var heldConcurrently atomic.Int64
	var maxConcurrent atomic.Int64
	for i := 0; i < workerCount; i++ {
		workers.Add(1)
		go func(worker int) {
			defer workers.Done()
			self := Identity{Pid: int64(1000 + worker), PidStartedAt: int64(worker + 1)}
			opts := Options{
				Wait: 10 * time.Second, Poll: time.Millisecond,
				Now: clock.Now, Sleep: schedule.sleep(worker), Probe: alive,
			}
			held, err := Acquire(path, self, opts)
			if err != nil {
				t.Errorf("worker %d: %v", worker, err)
				schedule.finishAndWakeNext(worker)
				return
			}
			if now := heldConcurrently.Add(1); now > maxConcurrent.Load() {
				maxConcurrent.Store(now)
			}
			if heldConcurrently.Load() == 1 {
				schedule.waitUntilOthersBlocked(worker)
			} else {
				t.Errorf("worker %d acquired while another worker held the lock", worker)
			}
			if err := held.Release(); err != nil {
				t.Errorf("worker %d release: %v", worker, err)
			}
			heldConcurrently.Add(-1)
			schedule.finishAndWakeNext(worker)
		}(i)
	}
	workers.Wait()
	close(stop)
	observer.Wait()
	if got := ownerless.Load(); got != 0 {
		t.Fatalf("observed %d ownerless lock states — acquisition has a window", got)
	}
	if maxConcurrent.Load() != 1 {
		t.Fatalf("two holders entered concurrently (max %d)", maxConcurrent.Load())
	}
}

func TestDeathOnlyTakeover(t *testing.T) {
	path := lockPath(t)
	if _, err := Acquire(path, Identity{Pid: 1, PidStartedAt: 1}, Options{Wait: time.Second, Probe: alive}); err != nil {
		t.Fatal(err)
	}
	successor := Identity{Pid: 2, PidStartedAt: 2}
	held, err := Acquire(path, successor, Options{Wait: time.Second, Probe: dead})
	if err != nil {
		t.Fatalf("takeover of a proven-dead holder failed: %v", err)
	}
	holder, _ := Holder(path)
	if holder != successor {
		t.Fatalf("lock does not name the successor: %+v", holder)
	}
	_ = held
}

func TestUnknownNeverAuthorizesTakeover(t *testing.T) {
	path := lockPath(t)
	if _, err := Acquire(path, Identity{Pid: 1, PidStartedAt: 1}, Options{Wait: time.Second, Probe: alive}); err != nil {
		t.Fatal(err)
	}
	clock := newFakeClock()
	const wait = 150 * time.Millisecond
	const poll = 10 * time.Millisecond
	_, err := Acquire(path, Identity{Pid: 2, PidStartedAt: 2}, clock.options(wait, poll, unknown))
	var holderErr *HolderError
	if !errors.As(err, &holderErr) || holderErr.State != Unknown {
		t.Fatalf("unknown liveness must wait out and fail naming the unproven holder: %v", err)
	}
	if holder, _ := Holder(path); holder.Pid != 1 {
		t.Fatal("the unproven holder lost its lock")
	}
	if got, want := clock.Slept(), wait+poll; got != want {
		t.Fatalf("logical wait = %v, want %v", got, want)
	}
}

func TestLiveHolderKeepsTheLock(t *testing.T) {
	path := lockPath(t)
	if _, err := Acquire(path, Identity{Pid: 1, PidStartedAt: 1}, Options{Wait: time.Second, Probe: alive}); err != nil {
		t.Fatal(err)
	}
	clock := newFakeClock()
	_, err := Acquire(path, Identity{Pid: 2, PidStartedAt: 2},
		clock.options(100*time.Millisecond, 10*time.Millisecond, alive))
	var holderErr *HolderError
	if !errors.As(err, &holderErr) || holderErr.Holder.Pid != 1 || holderErr.State != Alive {
		t.Fatalf("live holder must be named in the refusal: %v", err)
	}
}

// REG-4: a lock directory with no owner file is garbage by
// construction and takeable after the bounded window.
func TestOwnerlessDirectoryIsGarbageAfterWindow(t *testing.T) {
	path := lockPath(t)
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	clock := newFakeClock()
	const wait = 150 * time.Millisecond
	held, err := Acquire(path, Identity{Pid: 3, PidStartedAt: 3},
		clock.options(wait, 10*time.Millisecond, alive))
	if err != nil {
		t.Fatalf("garbage lock not taken: %v", err)
	}
	if got := clock.Slept(); got != wait {
		t.Fatalf("garbage takeover followed a logical wait of %v, want %v", got, wait)
	}
	if holder, err := Holder(path); err != nil || holder.Pid != 3 {
		t.Fatalf("garbage takeover did not publish the successor: %+v %v", holder, err)
	}
	if err := held.Release(); err != nil {
		t.Fatal(err)
	}
}

// SLC-R8-001's paused-acquirer proof: an acquirer that populated its
// private directory and paused resumes into a FAILED rename when the
// lock was won meanwhile — never into a shared lock.
func TestPausedAcquirerResumesIntoFailedRename(t *testing.T) {
	path := lockPath(t)
	private := path + ".acquire-paused"
	if err := os.Mkdir(private, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(private, "owner.json"), []byte(`{"pid":9,"pidStartedAt":9}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Acquire(path, Identity{Pid: 1, PidStartedAt: 1}, Options{Wait: time.Second, Probe: alive}); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(private, path); err == nil {
		t.Fatal("the paused acquirer's rename succeeded into a held lock")
	}
	if holder, _ := Holder(path); holder.Pid != 1 {
		t.Fatalf("holder changed under a failed rename: %+v", holder)
	}
}

func TestReleaseRefusesAfterTakeover(t *testing.T) {
	path := lockPath(t)
	original, err := Acquire(path, Identity{Pid: 1, PidStartedAt: 1}, Options{Wait: time.Second, Probe: alive})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Acquire(path, Identity{Pid: 2, PidStartedAt: 2}, Options{Wait: time.Second, Probe: dead}); err != nil {
		t.Fatal(err)
	}
	if err := original.Release(); err == nil {
		t.Fatal("a displaced holder released its successor's lock (SLC-F-001 shape)")
	}
	if holder, _ := Holder(path); holder.Pid != 2 {
		t.Fatal("successor lost its lock to a displaced release")
	}
}

func TestUnreadableOwnerIsUninspectableAlive(t *testing.T) {
	path := lockPath(t)
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	ownerPath := filepath.Join(path, "owner.json")
	if err := os.WriteFile(ownerPath, []byte(`{"pid":1,"pidStartedAt":1}`), 0o000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(ownerPath, 0o644)
	clock := newFakeClock()
	_, err := Acquire(path, Identity{Pid: 2, PidStartedAt: 2},
		clock.options(120*time.Millisecond, 10*time.Millisecond, dead))
	var holderErr *HolderError
	if !errors.As(err, &holderErr) || holderErr.State != Unknown {
		t.Fatalf("an unreadable owner must refuse as unproven, got %v", err)
	}
}

func TestAcquireRequiresProbe(t *testing.T) {
	if _, err := Acquire(lockPath(t), Identity{Pid: 1, PidStartedAt: 1}, Options{Wait: time.Second}); err == nil {
		t.Fatal("acquisition without a liveness probe must refuse")
	}
}

func TestTransferNamedKeepsTheLockAndChangesOnlyTheOwner(t *testing.T) {
	path := lockPath(t)
	first := Identity{Pid: 41, PidStartedAt: 410, Tag: "first", Label: "suite"}
	second := Identity{Pid: 42, PidStartedAt: 420, Tag: "same-chain", Label: "dispatch"}
	if _, err := Acquire(path, first, Options{Wait: time.Second, Probe: alive}); err != nil {
		t.Fatal(err)
	}
	if err := TransferNamed(path, Identity{Pid: 99}, second, nil); err == nil {
		t.Fatal("a process that did not own the lock transferred it")
	}
	if err := TransferNamed(path, first, second, nil); err != nil {
		t.Fatalf("transfer: %v", err)
	}
	holder, err := Holder(path)
	if err != nil || holder != second {
		t.Fatalf("holder after transfer: %+v err=%v", holder, err)
	}
	if err := ReleaseNamed(path, second, nil); err != nil {
		t.Fatal(err)
	}
}

func TestTakeoverRaceHasOneWinner(t *testing.T) {
	path := lockPath(t)
	if _, err := Acquire(path, Identity{Pid: 1, PidStartedAt: 1}, Options{Wait: time.Second, Probe: alive}); err != nil {
		t.Fatal(err)
	}
	winners := make(chan Identity, 4)
	var group sync.WaitGroup
	for i := 0; i < 4; i++ {
		group.Add(1)
		go func(worker int) {
			defer group.Done()
			self := Identity{Pid: int64(50 + worker), PidStartedAt: int64(worker + 1)}
			probe := func(holder Identity) Liveness {
				if holder.Pid == 1 {
					return Dead // the original holder is provably dead
				}
				return Alive // a fellow taker is alive; wait behind it
			}
			clock := newFakeClock()
			if _, err := Acquire(path, self, clock.options(300*time.Millisecond, time.Millisecond, probe)); err == nil {
				winners <- self
			}
		}(i)
	}
	group.Wait()
	close(winners)
	var won []Identity
	for winner := range winners {
		won = append(won, winner)
	}
	if len(won) != 1 {
		t.Fatalf("takeover race must have exactly one winner, got %d (%v)", len(won), won)
	}
	if holder, _ := Holder(path); fmt.Sprintf("%d", holder.Pid) == "1" {
		t.Fatal("nobody took over a provably dead holder")
	}
}

type refusingCodec struct{}

func (refusingCodec) Encode(Identity) ([]byte, error) { return nil, errors.New("no encoding") }
func (refusingCodec) Decode(data []byte) (Identity, error) {
	return identityJSON{}.Decode(data)
}

func TestTransferNamedSurfacesCodecAndAbsenceFailures(t *testing.T) {
	path := lockPath(t)
	if err := TransferNamed(path, Identity{Pid: 7}, Identity{Pid: 8}, nil); err == nil {
		t.Fatal("transferring an absent lock succeeded")
	}
	first := Identity{Pid: 41, PidStartedAt: 410, Tag: "first"}
	if _, err := Acquire(path, first, Options{Wait: time.Second, Probe: alive}); err != nil {
		t.Fatal(err)
	}
	if err := TransferNamed(path, first, Identity{Pid: 42}, refusingCodec{}); err == nil {
		t.Fatal("a codec that cannot encode still transferred the owner")
	}
	holder, err := Holder(path)
	if err != nil || holder != first {
		t.Fatalf("failed transfer disturbed the holder: %+v err=%v", holder, err)
	}
	if err := ReleaseNamed(path, first, nil); err != nil {
		t.Fatal(err)
	}
}

func TestAcquireSurfacesOwnerEncodingFailure(t *testing.T) {
	path := lockPath(t)
	if _, err := Acquire(path, Identity{Pid: 9, PidStartedAt: 90}, Options{Wait: 50 * time.Millisecond, Probe: alive, Codec: refusingCodec{}}); err == nil {
		t.Fatal("an unencodable owner acquired the lock")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("failed acquisition left lock state behind")
	}
}
