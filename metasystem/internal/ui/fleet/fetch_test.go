package fleet

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/snapshot"
)

// The fetch owner's four rules, driven by an injected clock and a fake
// connection signal. Nothing here waits on the wall: the owner's whole policy
// is in Consider, and a test decides when that is called and what the clock
// says when it is.

// driver is one owner with everything it did written down.
type driver struct {
	owner     *Owner
	mu        sync.Mutex
	at        time.Time
	connected bool
	attempts  int
	announced int
	written   []Metadata
	fail      error
	// entered and release let a test hold one attempt inside the owner while
	// another caller tries to start a second.
	entered chan struct{}
	release chan struct{}
}

func newDriver() *driver {
	drive := &driver{at: now, connected: true}
	drive.owner = &Owner{
		Attempt: func() error {
			drive.mu.Lock()
			drive.attempts++
			entered, release, fail := drive.entered, drive.release, drive.fail
			drive.mu.Unlock()
			if entered != nil {
				entered <- struct{}{}
				<-release
			}
			return fail
		},
		Connected: func() bool {
			drive.mu.Lock()
			defer drive.mu.Unlock()
			return drive.connected
		},
		Announce: func() {
			drive.mu.Lock()
			defer drive.mu.Unlock()
			drive.announced++
		},
		Record: func(state Metadata) error {
			drive.mu.Lock()
			defer drive.mu.Unlock()
			drive.written = append(drive.written, state)
			return nil
		},
		Now: func() time.Time {
			drive.mu.Lock()
			defer drive.mu.Unlock()
			return drive.at
		},
	}
	return drive
}

func (d *driver) tick(forward time.Duration) {
	d.mu.Lock()
	d.at = d.at.Add(forward)
	d.mu.Unlock()
}

func (d *driver) counts() (int, int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.attempts, d.announced
}

func TestNoAttemptWhileNoBrowserIsConnected(t *testing.T) {
	t.Parallel()
	drive := newDriver()
	drive.connected = false

	testutil.Expect(t, "a disconnected server does not fetch", drive.owner.Consider(), false)
	drive.tick(time.Hour)
	testutil.Expect(t, "however long it waits", drive.owner.Consider(), false)
	attempts, _ := drive.counts()
	testutil.Expect(t, "and nothing reached the transport", attempts, 0)

	drive.mu.Lock()
	drive.connected = true
	drive.mu.Unlock()
	testutil.Expect(t, "a browser arriving is what starts one", drive.owner.Consider(), true)
}

func TestAMinutePassesBetweenAttemptStarts(t *testing.T) {
	t.Parallel()
	drive := newDriver()

	testutil.Expect(t, "the first attempt runs", drive.owner.Consider(), true)
	drive.tick(59 * time.Second)
	testutil.Expect(t, "a second inside the minute does not", drive.owner.Consider(), false)
	drive.tick(time.Second)
	testutil.Expect(t, "one at the minute does", drive.owner.Consider(), true)
	attempts, _ := drive.counts()
	testutil.Expect(t, "so two attempts were made", attempts, 2)
}

// A failure counts against the minute exactly as a success does: a remote
// that refuses in a millisecond must not become a fetch every millisecond.
func TestAFailureCountsAgainstTheMinuteTheSameWay(t *testing.T) {
	t.Parallel()
	drive := newDriver()
	drive.fail = errors.New("presence fetch: the remote refused")

	testutil.Expect(t, "the attempt runs and fails", drive.owner.Consider(), true)
	drive.tick(30 * time.Second)
	testutil.Expect(t, "and the next one waits its minute", drive.owner.Consider(), false)
	testutil.Expect(t, "the failure is kept apart from the success",
		drive.owner.State(), Copy{AttemptedAt: seat.FormatTime(now), FailedAt: seat.FormatTime(now),
			Problem: "presence fetch: the remote refused"})
	testutil.Expect(t, "and this server has never succeeded", drive.owner.Succeeded(), false)
}

func TestOnlyOneAttemptIsEverInFlight(t *testing.T) {
	t.Parallel()
	drive := newDriver()
	drive.entered = make(chan struct{})
	drive.release = make(chan struct{})

	started := make(chan bool, 1)
	go func() { started <- drive.owner.Consider() }()
	<-drive.entered
	// The clock is well past the minute, so only the in-flight rule can
	// refuse this one.
	drive.tick(10 * time.Minute)
	testutil.Expect(t, "a second caller is refused while one is in flight", drive.owner.Consider(), false)
	close(drive.release)
	testutil.Expect(t, "and the first one finished", <-started, true)
	attempts, _ := drive.counts()
	testutil.Expect(t, "exactly one attempt reached the transport", attempts, 1)
}

func TestEveryAttemptAnnouncesAndWritesItsMetadata(t *testing.T) {
	t.Parallel()
	drive := newDriver()

	drive.owner.Consider()
	drive.tick(time.Minute)
	drive.mu.Lock()
	drive.fail = errors.New("presence fetch: the remote refused")
	drive.mu.Unlock()
	drive.owner.Consider()

	attempts, announced := drive.counts()
	testutil.Expect(t, "two attempts were made", attempts, 2)
	testutil.Expect(t, "and each one announced, success or failure", announced, 2)
	drive.mu.Lock()
	written := append([]Metadata(nil), drive.written...)
	drive.mu.Unlock()
	testutil.Require(t, "each one wrote the metadata file", len(written), 2)
	testutil.Expect(t, "the first success is named", written[0].SucceededAt, seat.FormatTime(now))
	testutil.Expect(t, "the failure names itself", written[1].FailedAt, seat.FormatTime(now.Add(time.Minute)))
	testutil.Expect(t, "and preserves the success before it", written[1].SucceededAt, seat.FormatTime(now))
	testutil.Expect(t, "every row names the namespace the interface fetches into",
		written[1].Namespace, seat.UINamespace)
}

// A success that brought nothing is a success with an empty copy, which is a
// different thing from a server that has never fetched.
func TestANeverFetchedServerIsToldFromOneThatFetchedNothing(t *testing.T) {
	t.Parallel()
	drive := newDriver()

	testutil.Expect(t, "before the first attempt nothing is known",
		drive.owner.State(), Copy{})
	testutil.Expect(t, "and the page reads the tick's copy", drive.owner.Succeeded(), false)

	drive.owner.Consider()
	testutil.Expect(t, "after a fetch that brought nothing the attempt is recorded",
		drive.owner.State(), Copy{AttemptedAt: seat.FormatTime(now), SucceededAt: seat.FormatTime(now)})
	testutil.Expect(t, "and the page reads the interface's own namespace", drive.owner.Succeeded(), true)
}

func TestTheOwnerRunsOnTheTicksItIsGiven(t *testing.T) {
	t.Parallel()
	drive := newDriver()
	ticks := make(chan time.Time)
	ctx, stop := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		drive.owner.Run(ctx, ticks)
	}()

	ticks <- now
	drive.tick(time.Minute)
	ticks <- drive.at
	stop()
	<-done

	attempts, _ := drive.counts()
	testutil.Expect(t, "one attempt per tick that the minute allowed", attempts, 2)
}

/* ------------------------------------------------------------- the watch -- */

func TestTheWatchIsBothTheConnectionSignalAndTheEvent(t *testing.T) {
	t.Parallel()
	watch := NewWatch()

	testutil.Expect(t, "no browser, no connection", watch.Connected(), false)
	signals, leave := watch.Join()
	testutil.Expect(t, "one open stream is a connection", watch.Connected(), true)

	watch.Announce()
	select {
	case <-signals:
	default:
		t.Fatal("the open stream was not told that an attempt finished")
	}

	// A second announcement before the first was drained asks for the same
	// single re-read, so it neither blocks nor queues.
	watch.Announce()
	watch.Announce()
	testutil.Expect(t, "a pending signal is not queued twice", len(signals), 1)

	leave()
	testutil.Expect(t, "and the last browser leaving ends the connection", watch.Connected(), false)
}

/* ---------------------------------------------------------- the metadata -- */

func TestTheMetadataFileRoundTripsForTheTool(t *testing.T) {
	t.Parallel()
	checkout := t.TempDir()

	absent, present, err := LoadMetadata(checkout)
	testutil.Require(t, "an absent file is not an error", err, nil)
	testutil.Expect(t, "and says it is absent", present, false)
	testutil.Expect(t, "naming the namespace all the same", absent.Namespace, seat.UINamespace)

	written := Metadata{Namespace: seat.UINamespace, AttemptedAt: seat.FormatTime(now),
		SucceededAt: seat.FormatTime(now)}
	testutil.Require(t, "the file is written", SaveMetadata(checkout, written), nil)
	testutil.Expect(t, "beside the steward's own agent files",
		filepath.ToSlash(MetadataPath(checkout)[len(checkout)+1:]), "artifacts/agents/ui/presence-fetch.json")

	read, found, err := LoadMetadata(checkout)
	testutil.Require(t, "and read back", err, nil)
	testutil.Expect(t, "as present", found, true)
	testutil.Expect(t, "with the attempt it recorded", read.AttemptedAt, written.AttemptedAt)
	testutil.Expect(t, "and the schema this build writes", read.Schema, MetadataSchema)
}

func TestATornMetadataFileIsAnErrorRatherThanAnEmptyReading(t *testing.T) {
	t.Parallel()
	checkout := t.TempDir()
	path := MetadataPath(checkout)
	testutil.Require(t, "the directory is made", os.MkdirAll(filepath.Dir(path), 0o755), nil)
	testutil.Require(t, "and a torn file written", os.WriteFile(path, []byte("{not json"), 0o644), nil)

	_, present, err := LoadMetadata(checkout)
	testutil.Expect(t, "the file is there", present, true)
	testutil.Expect(t, "and unreadable rather than empty", err != nil, true)
}

/* ------------------------------------------- beside the loop, not inside -- */

// The presence fetch is the fleet package's own call and never the snapshot
// loop's. The loop's Fetch returns one error, and a failure there blanks the
// ledger tip and backs the loop off toward five minutes; a presence remote
// that is down must cost the fleet page its freshness and the ledger nothing.
//
// So the two run side by side here, over one clock, with the presence fetch
// failing throughout: the ledger loop still advances, and neither waits on
// the other.
func TestThePresenceFetchAndTheLedgerLoopRunSideBySide(t *testing.T) {
	t.Parallel()
	drive := newDriver()
	drive.fail = errors.New("presence fetch: the remote refused")

	holder := snapshot.New(t.TempDir(), func() time.Time { return now })
	timers := &handTimers{armed: make(chan *handTimer, 64)}
	advances := make(chan struct{}, 8)
	ctx, stop := context.WithCancel(context.Background())
	loopDone := make(chan struct{})
	go func() {
		defer close(loopDone)
		holder.Run(ctx, func(goal.Endpoint) (goal.AdvanceResult, error) {
			advances <- struct{}{}
			return goal.AdvanceResult{}, errors.New("this bed has no ledger, which is not what this test is about")
		}, timers)
	}()

	ran := 0
	for round := 0; round < 3; round++ {
		if drive.owner.Consider() {
			ran++
		}
		drive.tick(time.Minute)
		(<-timers.armed).fire()
		<-advances
	}
	testutil.Expect(t, "every presence attempt ran, none of them waiting on the loop", ran, 3)
	stop()
	(<-timers.armed).fire()
	<-loopDone

	attempts, announced := drive.counts()
	testutil.Expect(t, "every presence attempt ran while the loop was ticking", attempts, 3)
	testutil.Expect(t, "and every one announced", announced, 3)
	testutil.Expect(t, "the presence failure is the presence copy's own",
		drive.owner.State().Problem, "presence fetch: the remote refused")
	testutil.Expect(t, "and the ledger loop's own failure stayed the ledger loop's",
		holder.Observe().Fetch.Outcome, snapshot.OutcomeFailed)
}

// handTimers is the loop's clock, driven by this test: the loop arms a
// deadline and waits, and this test decides when it comes.
type handTimers struct {
	armed chan *handTimer
}

func (h *handTimers) NewTimer(time.Duration) snapshot.Timer {
	timer := &handTimer{channel: make(chan time.Time, 1)}
	h.armed <- timer
	return timer
}

type handTimer struct{ channel chan time.Time }

func (h *handTimer) C() <-chan time.Time { return h.channel }
func (h *handTimer) Stop()               {}
func (h *handTimer) fire()               { h.channel <- now }
