package lifecycle

import (
	"errors"
	"os"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

type StopOutcome string

const (
	StoppedNow StopOutcome = "stopped-now"
	Timeout    StopOutcome = "timeout"
)

type StopOptions struct {
	Prober identity.Prober
	Send   identity.SignalFunc
	Wait   time.Duration
	After  func(time.Duration) <-chan time.Time
}

func Stop(stateRoot string, o StopOptions) (StopOutcome, *Record, error) {
	status, err := Read(stateRoot, o.Prober)
	if err != nil {
		return "", nil, err
	}
	if status.State == Busy {
		f, won, _, err := waitLock(stateRoot, false, o.Wait, o.After)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", nil, err
		}
		if won {
			releaseLock(f)
		}
		status, err = Read(stateRoot, o.Prober)
		if err != nil {
			return "", nil, err
		}
	}
	if status.State != Running {
		return StopOutcome(status.State), status.Record, nil
	}
	rec := status.Record
	ref, _ := identity.ParseRef(rec.Process)
	err = identity.SignalExact(o.Prober, ref, syscall.SIGTERM, o.Send)
	if errors.Is(err, identity.ErrGone) {
		status, err := readInactive(stateRoot, nil)
		return StopOutcome(status.State), status.Record, err
	}
	if errors.Is(err, identity.ErrUninspectable) {
		return StopOutcome(Uninspectable), rec, nil
	}
	if err != nil {
		return "", rec, err
	}
	f, won, _, err := waitLock(stateRoot, false, o.Wait, o.After)
	if err != nil {
		return "", rec, err
	}
	if won {
		defer releaseLock(f)
		return StoppedNow, rec, removeRecord(stateRoot)
	}
	if identity.AliveRef(o.Prober, ref) == identity.Dead {
		return StoppedNow, rec, nil
	}
	return Timeout, rec, nil
}

func Restart(stateRoot string, o StopOptions, start func() error) (StopOutcome, *Record, error) {
	outcome, rec, err := Stop(stateRoot, o)
	if err == nil && (outcome == StopOutcome(Stopped) || outcome == StopOutcome(Stale) || outcome == StoppedNow) {
		err = start()
	}
	return outcome, rec, err
}
