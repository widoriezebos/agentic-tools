package lifecycle

import (
	"errors"
	"fmt"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// Result keeps command outcomes and their exit codes with the lifecycle decisions.
type Result struct {
	Lines []string
	Code  int
}

func failure(err error) Result { return Result{[]string{err.Error()}, 1} }

func StartResult(spec LaunchSpec, spawn Spawn, readyWait time.Duration) Result {
	address, pid, err := Launch(spec, spawn, readyWait)
	if err != nil {
		return failure(err)
	}
	return Result{[]string{fmt.Sprintf("interface running at http://%s (pid %d)", address, pid)}, 0}
}

func StatusResult(checkout string, prober identity.Prober, digest func() (string, error)) Result {
	status, err := Read(checkout, prober)
	if err != nil {
		return failure(err)
	}
	if status.State != Running {
		return Result{[]string{stateLine(checkout, status.State, status.Record)}, 1}
	}
	rec := status.Record
	result := Result{Lines: []string{fmt.Sprintf("interface running at http://%s (pid %d, started %s, build %s)", rec.Address, recordPid(rec), rec.StartedAt, rec.EngineBuild)}}
	if digest == nil {
		digest = ExecutableDigest
	}
	current, err := digest()
	if err != nil {
		result.Lines = append(result.Lines, "cannot read the executable on disk to compare builds: "+err.Error())
	} else if ExecutableChanged(*rec, current) {
		result.Lines = append(result.Lines, "the executable on disk differs from the one the interface is running; to pick it up: metasystem ui restart")
	}
	return result
}

func StopResult(checkout string, o StopOptions) Result {
	outcome, rec, err := Stop(checkout, o)
	return stopResult(checkout, outcome, rec, o.Wait, err)
}

func RestartResult(checkout string, o StopOptions, start func() Result) Result {
	var started *Result
	outcome, rec, err := Restart(checkout, o, func() error {
		result := start()
		started = &result
		return nil
	})
	result := stopResult(checkout, outcome, rec, o.Wait, err)
	if started == nil {
		return result
	}
	if outcome == StopOutcome(Stopped) {
		result.Lines = nil
	}
	result.Lines = append(result.Lines, started.Lines...)
	result.Code = started.Code
	return result
}

func stopResult(checkout string, outcome StopOutcome, rec *Record, wait time.Duration, err error) Result {
	if err != nil {
		return failure(err)
	}
	switch outcome {
	case StoppedNow:
		return Result{[]string{"interface stopped"}, 0}
	case Timeout:
		return Result{[]string{fmt.Sprintf("interface (pid %d) did not stop within %gs; it was sent SIGTERM and left running", recordPid(rec), wait.Seconds())}, 1}
	default:
		code := 1
		if outcome == StopOutcome(Stopped) || outcome == StopOutcome(Stale) {
			code = 0
		}
		return Result{[]string{stateLine(checkout, State(outcome), rec)}, code}
	}
}

func stateLine(checkout string, state State, rec *Record) string {
	switch state {
	case Stopped:
		return "interface not running"
	case Stale:
		return fmt.Sprintf("interface not running (stale record from pid %d removed)", recordPid(rec))
	case Uninspectable:
		return fmt.Sprintf("cannot prove pid %d is the interface server; nothing was changed", recordPid(rec))
	case Unreadable:
		return fmt.Sprintf("a process holds the interface lock but %s cannot be read; nothing was changed", recordPath(checkout))
	default:
		return "the interface is starting or stopping; try again"
	}
}

func recordPid(rec *Record) int64 {
	ref, _ := identity.ParseRef(rec.Process)
	return ref.Pid
}

func ServeFailure(checkout string, err error) string {
	var running *AlreadyRunningError
	if errors.As(err, &running) && running.Address == "" {
		return fmt.Sprintf("an interface server already runs for this checkout (address unknown: %s is missing or unreadable)", recordPath(checkout))
	}
	return err.Error()
}
