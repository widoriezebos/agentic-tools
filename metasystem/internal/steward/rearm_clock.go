package steward

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"
)

// RearmClock is the clock used by external re-arm judgment steps. Tests use
// it to advance silence deadlines without waiting for wall time.
type RearmClock interface {
	Now() time.Time
	After(time.Duration) <-chan time.Time
}

type systemRearmClock struct{}

func (systemRearmClock) Now() time.Time                                { return time.Now() }
func (systemRearmClock) After(duration time.Duration) <-chan time.Time { return time.After(duration) }

// SystemRearmClock returns the production clock for a re-arm judgment.
func SystemRearmClock() RearmClock { return systemRearmClock{} }

type judgmentStallError struct {
	step    string
	seconds int
}

func (err *judgmentStallError) Error() string {
	return fmt.Sprintf("%s exceeded the configured %d-second bound (%s)", err.step, err.seconds, rearmResolveSecondsConfig)
}

func (err *judgmentStallError) Unwrap() error { return ErrJudgmentStalled }

// JudgmentStall reports the external step and silence bound carried by err.
func JudgmentStall(err error) (step string, seconds int, ok bool) {
	var stalled *judgmentStallError
	if !errors.As(err, &stalled) {
		return "", 0, false
	}
	return stalled.step, stalled.seconds, true
}

// RunRearmStep bounds silence, not total work. Each progress callback resets
// the deadline and is acknowledged before the producer writes more bytes.
func RunRearmStep(parent context.Context, clock RearmClock, silence time.Duration, step string, run func(context.Context, func()) error) error {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	progress := make(chan chan struct{})
	result := make(chan error, 1)
	lastProgress := clock.Now()
	deadline := clock.After(silence)
	go func() {
		result <- run(ctx, func() {
			ack := make(chan struct{})
			select {
			case progress <- ack:
			case <-ctx.Done():
				return
			}
			select {
			case <-ack:
			case <-ctx.Done():
			}
		})
	}()
	for {
		select {
		case err := <-result:
			return err
		case ack := <-progress:
			lastProgress = clock.Now()
			close(ack)
		case <-deadline:
			// Prefer a byte that is already waiting at the boundary. A select
			// may otherwise choose the timer and call a live step silent.
			select {
			case ack := <-progress:
				lastProgress = clock.Now()
				close(ack)
				deadline = clock.After(silence)
				continue
			default:
			}
			remaining := silence - clock.Now().Sub(lastProgress)
			if remaining > 0 {
				deadline = clock.After(remaining)
				continue
			}
			return &judgmentStallError{step: step, seconds: int(silence / time.Second)}
		case <-parent.Done():
			return parent.Err()
		}
	}
}

type rearmProgressWriter struct {
	destination io.Writer
	progress    func()
}

func (writer rearmProgressWriter) Write(data []byte) (int, error) {
	if len(data) > 0 && writer.progress != nil {
		writer.progress()
	}
	return writer.destination.Write(data)
}

// RearmProgressWriter reports every non-empty write before forwarding it.
func RearmProgressWriter(destination io.Writer, progress func()) io.Writer {
	return rearmProgressWriter{destination: destination, progress: progress}
}
