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

func StatusResult(stateRoot string, prober identity.Prober, digest func() (string, error)) Result {
	result, _ := StatusReport(stateRoot, prober, digest)
	return result
}

// StatusReport is StatusResult with the state it read; an unreadable record
// is reported as Unreadable.
func StatusReport(stateRoot string, prober identity.Prober, digest func() (string, error)) (Result, State) {
	status, err := Read(stateRoot, prober)
	if err != nil {
		return failure(err), Unreadable
	}
	if status.State != Running {
		return Result{[]string{stateLine(stateRoot, status.State, status.Record)}, 1}, status.State
	}
	return runningResult(status.Record, digest), Running
}

func runningResult(rec *Record, digest func() (string, error)) Result {
	result := Result{Lines: []string{fmt.Sprintf("interface running at http://%s (pid %d, started %s, build %s)", rec.Address, recordPid(rec), rec.StartedAt, rec.EngineBuild)}}
	// Whether this server can act as the human is the second thing to know
	// about it, so it is the second line. A record written before this build
	// carries none, and says nothing rather than guessing.
	if rec.Authority != "" {
		result.Lines = append(result.Lines, rec.Authority)
	}
	// Who else is acting through this server: one line per browser a human
	// signed into with the seat's one-time code, with when it stops.
	result.Lines = append(result.Lines, rec.Sessions...)
	// The Project Partner, where one is configured: which runtime answers on
	// this seat, and whether it is answering right now.
	if rec.Partner != "" {
		result.Lines = append(result.Lines, rec.Partner)
	}
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

func StopResult(stateRoot string, o StopOptions) Result {
	outcome, rec, err := Stop(stateRoot, o)
	return stopResult(stateRoot, outcome, rec, o.Wait, err)
}

func RestartResult(stateRoot string, o StopOptions, start func() Result) Result {
	return RestartReportFor(stateRoot, o, start).Result
}

// RestartReport is a restart's printed result and the typed facts behind
// it: how the stop ended, and whether and how the start ran.
type RestartReport struct {
	Result    Result
	Stop      StopOutcome
	StopError string
	Started   bool
	Start     Result
}

func RestartReportFor(stateRoot string, o StopOptions, start func() Result) RestartReport {
	var started *Result
	outcome, rec, err := Restart(stateRoot, o, func() error {
		result := start()
		started = &result
		return nil
	})
	result := stopResult(stateRoot, outcome, rec, o.Wait, err)
	report := RestartReport{Stop: outcome}
	if err != nil {
		report.StopError = err.Error()
	}
	if started == nil {
		report.Result = result
		return report
	}
	if outcome == StopOutcome(Stopped) {
		result.Lines = nil
	}
	result.Lines = append(result.Lines, started.Lines...)
	result.Code = started.Code
	report.Result, report.Started, report.Start = result, true, *started
	return report
}

func stopResult(stateRoot string, outcome StopOutcome, rec *Record, wait time.Duration, err error) Result {
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
		return Result{[]string{stateLine(stateRoot, State(outcome), rec)}, code}
	}
}

func stateLine(stateRoot string, state State, rec *Record) string {
	switch state {
	case Stopped:
		return "interface not running"
	case Stale:
		return fmt.Sprintf("interface not running (stale record from pid %d removed)", recordPid(rec))
	case Uninspectable:
		return fmt.Sprintf("cannot prove pid %d is the interface server; nothing was changed", recordPid(rec))
	case Unreadable:
		return fmt.Sprintf("a process holds the interface lock but %s cannot be read; nothing was changed", recordPath(stateRoot))
	default:
		return "the interface is starting or stopping; try again"
	}
}

func recordPid(rec *Record) int64 {
	ref, _ := identity.ParseRef(rec.Process)
	return ref.Pid
}

func ServeFailure(stateRoot string, err error) string {
	var running *AlreadyRunningError
	if errors.As(err, &running) && running.Address == "" {
		return fmt.Sprintf("an interface server already runs for this checkout (address unknown: %s is missing or unreadable)", recordPath(stateRoot))
	}
	return err.Error()
}
