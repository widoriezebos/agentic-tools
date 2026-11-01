package steward

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostload"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

// waitBed is the wake bed read through the shared page: with hold, a build
// runs and the lane holds a proof beside it; an ask to this seat is due at
// the deadline given. Every read samples a new load and a new process start.
type waitBed struct {
	t                   *testing.T
	root, home, install string
	store               launch.Store
	ask                 board.Message
	src                 SituationSources
	clock               time.Time
	sleeps, reads       int
}

func newWaitBed(t *testing.T, hold bool, deadline time.Duration) *waitBed {
	t.Helper()
	root, laneHome, laneInstall := situationRecordsBed(t)
	b := &waitBed{t: t, root: root, home: t.TempDir(), install: laneInstall, store: launch.Store{Root: filepath.Join(t.TempDir(), "launch")}, clock: situationNow}
	if hold {
		if err := b.store.Create(launch.Record{ID: "host-build", Kind: "build", State: launch.Running}); err != nil {
			t.Fatal(err)
		}
		b.held(situationNow)
	}
	b.ask = composePublish(t, b.home, board.Request{Kind: board.KindAsk, From: board.Sender{Machine: "m1x"}, To: board.Address{Machine: "m1e"}, Text: "Is it built?", Deadline: deadline}, situationNow)
	manager := &launch.Manager{Store: b.store}
	prober := &situationProber{starts: map[int64]time.Time{}, calls: map[int64]int{}}
	b.src = situationHostSources(t, situationRegistry(t, root))
	b.src.Load = func(at time.Time) hostload.Sample {
		b.reads++
		prober.starts[101] = situationNow.Add(-time.Duration(b.reads) * time.Hour)
		return hostload.Sample{At: at.Format(time.RFC3339), Load1m: float64(b.reads), Available: true}
	}
	b.src.Processes = identity.FixedProcessTable{{Pid: 101, Group: 101, Session: 101, Parent: 1}}
	b.src.Argv = func(int64) ([]string, bool) { return []string{"go", "test", "./..."}, true }
	b.src.Cwd = func(int64) (string, bool) { return root, true }
	b.src.Prober = prober
	b.src.LaneHome = func() (string, error) { return laneHome, nil }
	b.src.HostWork = manager.HostWork
	b.src.LaneStatus = composeLane(t, laneInstall, manager.HostWork, lane.OwnerView{State: "running"})
	b.src.ClaimLaneState = claimQuotaLane(t, root, nil)
	b.src.BoardHome = func() (string, error) { return b.home, nil }
	b.src.Launches = manager.EffectiveList
	return b
}

// held writes the lane's held line, as its runner does.
func (b *waitBed) held(at time.Time) {
	b.t.Helper()
	if err := plain.RecordIntervention(b.install, plain.Intervention{At: at.Format(time.RFC3339Nano), Row: "proof-beside-builds", Subject: "main", Outcome: "held: 1 builds and 0 unit checks run"}); err != nil {
		b.t.Fatal(err)
	}
}

// page is the waiter's page: the shared page within fresh, read at at.
func (b *waitBed) page(at time.Time, fresh time.Duration) (Composition, error) {
	read := func(now time.Time) Situation {
		src := b.src
		src.Now = func() time.Time { return now }
		return readSituation(b.t, b.root, src)
	}
	record, err := SituationPage(b.root, read, at, fresh)
	return record.Composition, err
}

// wait reads the start page now and waits up to bound; event runs after each
// sleep at the clock it advanced to.
func (b *waitBed) wait(bound time.Duration, event func(at time.Time)) (Composition, string, error) {
	b.t.Helper()
	start, err := b.page(b.clock, SituationFresh)
	if err != nil {
		b.t.Fatal(err)
	}
	sleep := func(_ context.Context, d time.Duration) error {
		if b.sleeps++; b.sleeps > int(bound/SituationPoll)+1 {
			b.t.Fatalf("a wait of %v slept %d times", bound, b.sleeps)
		}
		b.clock = b.clock.Add(d)
		if event != nil {
			event(b.clock)
		}
		return nil
	}
	return WaitSituation(context.Background(), b.page, start, func() time.Time { return b.clock }, sleep, bound, SituationPoll)
}

// once runs act the first time the clock reaches at.
func once(at time.Time, act func(time.Time)) func(time.Time) {
	done := false
	return func(now time.Time) {
		if !done && !now.Before(at) {
			done = true
			act(now)
		}
	}
}

func TestTheWaitWakesOnlyOnWhatMatters(t *testing.T) {
	t.Parallel()
	due := 25*time.Minute + 5*time.Second
	t.Run("nothing that matters", func(t *testing.T) {
		t.Parallel()
		b := newWaitBed(t, true, due)
		page, reason, err := b.wait(10*time.Minute, nil)
		if err != nil || reason != "nothing changed in 10m" {
			t.Fatalf("a wait with only load and process starts changing: %q %v, want nothing changed in 10m", reason, err)
		}
		if b.sleeps > 20 || !b.clock.Equal(situationNow.Add(10*time.Minute)) {
			t.Fatalf("the wait slept %d times and stands at %v, want at most 20 sleeps ending at +10m", b.sleeps, b.clock.Sub(situationNow))
		}
		// One read per stale page: at the start and every two and a half
		// minutes; the polls between are file reads.
		if b.reads != 5 || !slices.Contains(page.Wake, "hold|main") {
			t.Fatalf("the wait read the situation %d times, want 5; the page's wake %v", b.reads, page.Wake)
		}

		// A clock that never moves still ends at the count.
		b.sleeps = 0
		frozen := b.clock
		start, _ := b.page(frozen, SituationFresh)
		_, reason, err = WaitSituation(context.Background(), b.page, start, func() time.Time { return frozen }, func(context.Context, time.Duration) error {
			if b.sleeps++; b.sleeps > 20 {
				t.Fatalf("a wait on a still clock slept %d times, want at most 20", b.sleeps)
			}
			return nil
		}, 10*time.Minute, SituationPoll)
		if err != nil || reason != "nothing changed in 10m" || b.sleeps != 20 {
			t.Fatalf("a wait on a still clock: %q %v after %d sleeps", reason, err, b.sleeps)
		}

		// A page that cannot be read ends the wait at once.
		polls, failed := 0, errors.New("the page cannot be read")
		_, _, err = WaitSituation(context.Background(), func(at time.Time, fresh time.Duration) (Composition, error) {
			if polls++; polls == 2 {
				return Composition{}, failed
			}
			return b.page(at, fresh)
		}, start, func() time.Time { return frozen }, func(context.Context, time.Duration) error { return nil }, 10*time.Minute, SituationPoll)
		if !errors.Is(err, failed) || polls != 2 {
			t.Fatalf("a read error at the second poll: %v after %d polls, want the error at once", err, polls)
		}
	})

	for _, c := range []struct {
		name  string
		hold  bool
		event func(b *waitBed, at time.Time)
		want  func(b *waitBed) string
	}{
		{"a new ask", true, func(b *waitBed, at time.Time) {
			b.ask = composePublish(t, b.home, board.Request{Kind: board.KindAsk, From: board.Sender{Machine: "m1x"}, To: board.Address{Machine: "m1e"}, Text: "Is it proven?"}, at)
		}, func(b *waitBed) string { return "changed: ask|" + b.ask.ID + "|open" }},
		{"a reply", true, func(b *waitBed, at time.Time) {
			composePublish(t, b.home, board.Request{Kind: board.KindReply, From: board.Sender{Machine: "m1e"}, To: board.Address{Machine: "m1x"}, Thread: b.ask.ID, Text: "Yes."}, at)
		}, func(b *waitBed) string { return "changed: ask|" + b.ask.ID + "|" + board.ThreadReplied }},
		{"a held line", false, func(b *waitBed, at time.Time) {
			if err := b.store.Create(launch.Record{ID: "host-build-2", Kind: "build", State: launch.Running}); err != nil {
				t.Error(err)
			}
			b.held(at)
		}, func(*waitBed) string { return "changed: hold|main" }},
		{"the build completed", true, func(b *waitBed, _ time.Time) {
			if _, err := b.store.Update("host-build", func(r *launch.Record) error { r.State = launch.Completed; return nil }); err != nil {
				t.Error(err)
			}
		}, func(*waitBed) string { return "changed: hold|none" }},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			b := newWaitBed(t, c.hold, due)
			events := 0
			_, reason, err := b.wait(10*time.Minute, once(situationNow.Add(3*time.Minute), func(at time.Time) { events++; c.event(b, at) }))
			if err != nil || reason != c.want(b) || events != 1 || b.clock.After(situationNow.Add(5*time.Minute)) {
				t.Fatalf("%s at +3m: woke %q at +%v (%v), want %q by +5m", c.name, reason, b.clock.Sub(situationNow), err, c.want(b))
			}
		})
	}

	t.Run("a deadline", func(t *testing.T) {
		t.Parallel()
		b := newWaitBed(t, true, due)
		page, reason, err := b.wait(30*time.Minute, nil)
		if err != nil || reason != "deadline 09:25" || !b.clock.Equal(situationNow.Add(due)) {
			t.Fatalf("the ask's deadline: woke %q at +%v (%v), want deadline 09:25 at +%v", reason, b.clock.Sub(situationNow), err, due)
		}
		if page.First.Kind != "ask" || page.First.Subject != b.ask.ID || !slices.Contains(page.Wake, "ask|"+b.ask.ID+"|"+board.ThreadExpired) {
			t.Fatalf("the deadline's page: first %+v, wake %v, want the ask expired and first", page.First, page.Wake)
		}
	})

	t.Run("a deadline after a tick saved the page", func(t *testing.T) {
		t.Parallel()
		b := newWaitBed(t, true, due)
		saved := situationNow.Add(25 * time.Minute)
		page, reason, err := b.wait(30*time.Minute, once(saved, func(at time.Time) {
			// The runner's duty: a fresh read and save at the pass's clock.
			if _, err := SituationPage(b.root, func(now time.Time) Situation {
				src := b.src
				src.Now = func() time.Time { return now }
				return readSituation(t, b.root, src)
			}, at, 0); err != nil {
				t.Error(err)
			}
		}))
		if err != nil || !strings.HasPrefix(reason, "deadline ") || !b.clock.Equal(situationNow.Add(due)) {
			t.Fatalf("woke %q at +%v (%v), want the deadline at +%v", reason, b.clock.Sub(situationNow), err, due)
		}
		if !strings.Contains(page.First.Why, "past its deadline") || page.First.Subject != b.ask.ID || slices.ContainsFunc(page.Deadlines, situationNow.Add(due).Equal) {
			t.Fatalf("the page saved at +25m was returned: first %+v, deadlines %v, want the ask past its deadline", page.First, page.Deadlines)
		}
	})

	t.Run("a deadline in the last interval", func(t *testing.T) {
		t.Parallel()
		b := newWaitBed(t, true, 45*time.Second)
		_, reason, err := b.wait(time.Minute, nil)
		if err != nil || !strings.HasPrefix(reason, "deadline ") || !b.clock.Equal(situationNow.Add(45*time.Second)) {
			t.Fatalf("a deadline at +45s in a one-minute wait: %q at +%v (%v)", reason, b.clock.Sub(situationNow), err)
		}
	})

	t.Run("a tick pass between polls", func(t *testing.T) {
		t.Parallel()
		bed := newPageBed(t)
		first := bed.now
		bed.run(1, func(int) dutyCycle { return dutyCycle{} }, nil)
		start := bed.saved().Composition
		bed.reads()
		clock, id, passReads := first, "", 0
		page := func(at time.Time, fresh time.Duration) (Composition, error) {
			record, err := SituationPage(bed.root, func(now time.Time) Situation { return bed.read(bed.root, now) }, at, fresh)
			return record.Composition, err
		}
		sleep := func(_ context.Context, d time.Duration) error {
			if clock = clock.Add(d); clock.Equal(bed.now) {
				id = bed.ask("Is it proven?")
				bed.run(1, func(int) dutyCycle { return dutyCycle{} }, nil)
				passReads, _ = bed.reads()
			}
			return nil
		}
		_, reason, err := WaitSituation(context.Background(), page, start, func() time.Time { return clock }, sleep, 10*time.Minute, SituationPoll)
		if err != nil || reason != "changed: ask|"+id+"|open" || !clock.Equal(first.Add(time.Minute)) {
			t.Fatalf("a pass saved a new ask: woke %q at %v (%v), want the ask at the pass", reason, clock, err)
		}
		if projections, loads := bed.reads(); passReads != 1 || projections != 0 || loads != 0 {
			t.Fatalf("the pass read %d times and the waiter %d and %d, want the pass once and the waiter never", passReads, projections, loads)
		}
	})
}
