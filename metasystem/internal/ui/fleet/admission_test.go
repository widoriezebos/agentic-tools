package fleet

import (
	"errors"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// The connection signal and the decision that reads it are one act.
//
// Two acts — ask Connected, then take the attempt slot — leave room for the
// last browser to leave in between, which is exactly the case "no attempt
// after the last disconnect" exists to prevent. These prove the real Watch,
// not a fake: the driver in fetch_test.go decides under its own lock the same
// way, and what is asserted here is that Watch.Admit makes that possible.

// TestAdmissionIsDecidedUnderTheLockAStreamLeavesUnder: the whole of the
// property, asserted without waiting on anything.
//
// Join and the function Join hands back both take w.mu. If the decision runs
// while that mutex is held, no stream can join or leave during it, which is
// the ordering the rule needs. TryLock is the assertion: it succeeds only
// when the mutex is free, so failing to take it is the proof that it is not.
func TestAdmissionIsDecidedUnderTheLockAStreamLeavesUnder(t *testing.T) {
	t.Parallel()
	watch := NewWatch()
	_, leave := watch.Join()

	locked := false
	admitted := watch.Admit(func(connected bool) bool {
		free := watch.mu.TryLock()
		if free {
			// It should not be free. Give it straight back, so that the
			// departure below reports a failed assertion rather than hanging.
			watch.mu.Unlock()
		}
		locked = !free
		return connected
	})

	testutil.Expect(t, "the decision saw the one open stream", admitted, true)
	testutil.Expect(t, "and was made while no stream could join or leave", locked, true)
	leave()
	testutil.Expect(t, "after which the stream has gone", watch.Connected(), false)
}

// The owner reads the connection Admit hands it and nothing else. A second
// look of its own would be the two acts this fix replaced.
func TestTheOwnerTrustsOnlyWhatAdmitHandsIt(t *testing.T) {
	t.Parallel()
	watch := NewWatch()
	_, leave := watch.Join()
	defer leave()
	attempts := 0
	owner := &Owner{
		Attempt: func() error { attempts++; return nil },
		// A watch with a stream open, and an admission that says the last one
		// has just gone: the owner must believe the admission.
		Admit: func(decide func(connected bool) bool) bool { return decide(false) },
		Now:   func() time.Time { return now },
	}

	testutil.Expect(t, "a refused admission is no attempt", owner.Consider(), false)
	testutil.Expect(t, "however many streams the watch still shows", watch.Connected(), true)
	testutil.Expect(t, "and nothing reached the transport", attempts, 0)
}

// The owner over the real watch: connected admits, disconnected refuses.
func TestTheOwnerTakesItsSlotThroughTheWatch(t *testing.T) {
	t.Parallel()
	watch := NewWatch()
	attempts := 0
	at := now
	owner := &Owner{
		Attempt: func() error { attempts++; return nil },
		Admit:   watch.Admit,
		Now:     func() time.Time { return at },
	}

	testutil.Expect(t, "nobody watching, no fetch", owner.Consider(), false)
	_, leave := watch.Join()
	testutil.Expect(t, "one browser is enough", owner.Consider(), true)
	at = at.Add(time.Hour)
	leave()
	testutil.Expect(t, "and the last one leaving ends it", owner.Consider(), false)
	testutil.Expect(t, "so exactly one attempt was made", attempts, 1)
}

/* --------------------------------------------- which run a file belongs to -- */

// The metadata file outlives the server that wrote it. A reader that cannot
// show it belongs to the run it is talking to must treat it as another
// server's, or it will cite a success no live server has made.
func TestMetadataIsOnlyThisRunsWhenItNamesThisRun(t *testing.T) {
	t.Parallel()
	written := Metadata{Run: "01RUNONE", Namespace: seat.UINamespace, SucceededAt: seat.FormatTime(now)}

	testutil.Expect(t, "its own run reads it", written.OfRun("01RUNONE"), true)
	testutil.Expect(t, "a later run does not", written.OfRun("01RUNTWO"), false)
	testutil.Expect(t, "a reader that cannot name its run does not", written.OfRun(""), false)
	testutil.Expect(t, "and a file from before this field existed does not",
		Metadata{SucceededAt: seat.FormatTime(now)}.OfRun("01RUNONE"), false)
}

func TestEveryMetadataWriteNamesTheRunThatMadeIt(t *testing.T) {
	t.Parallel()
	drive := newDriver()
	drive.owner.RunID = "01RUNONE"

	drive.owner.Consider()

	drive.mu.Lock()
	written := append([]Metadata(nil), drive.written...)
	drive.mu.Unlock()
	testutil.Require(t, "one file was written", len(written), 1)
	testutil.Expect(t, "naming this run", written[0].Run, "01RUNONE")
	testutil.Expect(t, "which is the run that may cite it", written[0].OfRun("01RUNONE"), true)
}

// A metadata write that failed costs the fetch nothing — the copy is in the
// namespace either way — but it is not silence: the Partner's tool is reading
// a file that has stopped moving, and the page says so.
func TestAFailedMetadataWriteIsVisibleInTheState(t *testing.T) {
	t.Parallel()
	drive := newDriver()
	drive.owner.Record = func(Metadata) error {
		return errors.New("artifacts/agents/ui: permission denied")
	}

	testutil.Expect(t, "the attempt itself succeeded", drive.owner.Consider(), true)
	state := drive.owner.State()
	testutil.Expect(t, "the fetch has nothing wrong with it", state.Problem, "")
	testutil.Expect(t, "and the copy is this server's own", drive.owner.Succeeded(), true)
	testutil.Expect(t, "but the tool's file says what happened", state.MetadataProblem,
		"the Partner's presence metadata could not be written: artifacts/agents/ui: permission denied")

	drive.owner.Record = func(Metadata) error { return nil }
	drive.tick(time.Minute)
	drive.owner.Consider()
	testutil.Expect(t, "a write that lands clears it", drive.owner.State().MetadataProblem, "")
}

/* ------------------------------------------------ when an attempt finished -- */

// A bounded fetch may take the whole of its budget. Stamping what it found
// with the instant it STARTED would date the copy a minute earlier than it
// is, and that number is what "presence fetched 40 s ago" is measured from.
func TestWhatAnAttemptFoundIsDatedFromWhenItFinished(t *testing.T) {
	t.Parallel()
	drive := newDriver()
	drive.entered = make(chan struct{})
	drive.release = make(chan struct{})

	done := make(chan bool, 1)
	go func() { done <- drive.owner.Consider() }()
	<-drive.entered
	drive.tick(45 * time.Second)
	close(drive.release)
	testutil.Require(t, "the attempt ran", <-done, true)

	state := drive.owner.State()
	testutil.Expect(t, "the attempt is dated from when it began",
		state.AttemptedAt, seat.FormatTime(now))
	testutil.Expect(t, "and what it found from when it came back",
		state.SucceededAt, seat.FormatTime(now.Add(45*time.Second)))
}

func TestAFailureIsDatedFromWhenItCameBackToo(t *testing.T) {
	t.Parallel()
	drive := newDriver()
	drive.fail = errors.New("presence fetch: the remote refused")
	drive.entered = make(chan struct{})
	drive.release = make(chan struct{})

	done := make(chan bool, 1)
	go func() { done <- drive.owner.Consider() }()
	<-drive.entered
	drive.tick(20 * time.Second)
	close(drive.release)
	testutil.Require(t, "the attempt ran", <-done, true)

	state := drive.owner.State()
	testutil.Expect(t, "the attempt began when it began", state.AttemptedAt, seat.FormatTime(now))
	testutil.Expect(t, "and failed when it came back",
		state.FailedAt, seat.FormatTime(now.Add(20*time.Second)))
}
