package testenv

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

type awaitRecorder struct {
	failures []string
}

func (*awaitRecorder) Helper() {}

func (r *awaitRecorder) Fatalf(format string, args ...any) {
	r.failures = append(r.failures, fmt.Sprintf(format, args...))
}

func TestAwaitReturnsOnTheObservedEventWhateverItTook(t *testing.T) {
	t.Parallel()
	recorder := &awaitRecorder{}
	observations, pauses := 0, 0
	remaining := func() (time.Duration, bool) { return awaitReserve + time.Nanosecond, true }
	await(recorder, "the third observation", func() bool {
		observations++
		return observations == 3
	}, func() { pauses++ }, remaining, nil)
	if len(recorder.failures) != 0 || observations != 3 || pauses != 2 {
		t.Fatalf("failures=%q observations=%d pauses=%d, want none, 3 and 2", recorder.failures, observations, pauses)
	}
}

func TestAwaitFailsOnlyAtTheTestBinaryDeadline(t *testing.T) {
	t.Parallel()
	recorder := &awaitRecorder{}
	observations := 0
	left := []time.Duration{time.Hour, awaitReserve + time.Nanosecond, awaitReserve}
	remaining := func() (time.Duration, bool) {
		next := left[0]
		left = left[1:]
		return next, true
	}
	await(recorder, "the release file", func() bool {
		observations++
		return false
	}, func() {}, remaining, nil)
	if observations != 3 || len(recorder.failures) != 1 || !strings.Contains(recorder.failures[0], "the release file") || !strings.Contains(recorder.failures[0], "deadline") {
		t.Fatalf("observations=%d failures=%q, want 3 and one failure naming the awaited event and the deadline", observations, recorder.failures)
	}
}

func TestAwaitWithoutADeadlineNeverGivesUp(t *testing.T) {
	t.Parallel()
	recorder := &awaitRecorder{}
	observations := 0
	await(recorder, "the thousandth observation", func() bool {
		observations++
		return observations == 1000
	}, func() {}, func() (time.Duration, bool) { return 0, false }, nil)
	if len(recorder.failures) != 0 || observations != 1000 {
		t.Fatalf("failures=%q observations=%d, want none and 1000", recorder.failures, observations)
	}
}

func TestAwaitOrReportsWhatItsGiveUpGathers(t *testing.T) {
	t.Parallel()
	recorder := &awaitRecorder{}
	gathered := 0
	await(recorder, "the launcher's exit", func() bool { return false }, func() {}, func() (time.Duration, bool) { return awaitReserve, true },
		func() string { gathered++; return "goroutine dump" })
	if gathered != 1 || len(recorder.failures) != 1 || !strings.Contains(recorder.failures[0], "goroutine dump") {
		t.Fatalf("gathered=%d failures=%q, want the give-up's report in the one failure", gathered, recorder.failures)
	}
}

type awaitDeadline struct {
	deadline time.Time
	bounded  bool
}

func (d awaitDeadline) Deadline() (time.Time, bool) { return d.deadline, d.bounded }

func TestDeadlineRemainingPreservesExpiredDeadline(t *testing.T) {
	t.Parallel()
	remaining, bounded := DeadlineRemaining(awaitDeadline{bounded: true})
	if !bounded || remaining >= 0 {
		t.Fatalf("expired deadline remaining=%s bounded=%t, want a negative duration and a bound", remaining, bounded)
	}
}

func TestDeadlineRemainingWithoutDeadlineIsUnbounded(t *testing.T) {
	t.Parallel()
	remaining, bounded := DeadlineRemaining(awaitDeadline{})
	if bounded || remaining != 0 {
		t.Fatalf("absent deadline remaining=%s bounded=%t, want zero and no bound", remaining, bounded)
	}
}
