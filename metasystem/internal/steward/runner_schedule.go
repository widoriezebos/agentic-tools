package steward

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/tickrequest"
)

// cycleSchedule keeps the ordinary clock independent of completion traffic.
type cycleSchedule struct {
	NextTick time.Time     `json:"nextTick"`
	LastEnd  time.Time     `json:"lastEnd"`
	Requests int           `json:"requestCycles"`
	Gap      time.Duration `json:"gap"`
	Limit    int           `json:"limit"`
}

func (s cycleSchedule) due(now time.Time, pending bool) time.Time {
	if !pending || s.Requests >= s.Limit {
		return s.NextTick
	}
	due := s.LastEnd.Add(s.Gap)
	if due.Before(now) {
		due = now
	}
	// A nearby scheduled cycle serves the request without an extra pass.
	if !due.Add(s.Gap).Before(s.NextTick) {
		return s.NextTick
	}
	return due
}

func readCycleSchedule(root string, now time.Time) (cycleSchedule, error) {
	gap, err := boundedConfig(root, "steward.request-gap-sec", config.MustIntDefault("steward.request-gap-sec"), 1)
	if errors.Is(err, os.ErrNotExist) {
		gap, err = config.MustIntDefault("steward.request-gap-sec"), nil
	}
	if err != nil {
		return cycleSchedule{}, err
	}
	limit, err := boundedConfig(root, "steward.request-cycle-max", config.MustIntDefault("steward.request-cycle-max"), 1)
	if errors.Is(err, os.ErrNotExist) {
		limit, err = config.MustIntDefault("steward.request-cycle-max"), nil
	}
	return cycleSchedule{NextTick: now, Gap: time.Duration(gap) * time.Second, Limit: limit}, err
}

func cycleSchedulePath(root string) string { return filepath.Join(runnerDir(root), "schedule.json") }

// CycleStatus is a read-only view; a dormant request has no promised due time.
type CycleStatus struct {
	RunnerAlive bool       `json:"runnerAlive"`
	NextDue     *time.Time `json:"nextDue,omitempty"`
	Pending     string     `json:"pending,omitempty"`
	Line        string     `json:"line"`
}

func RunnerCycleStatus(root string, now time.Time) CycleStatus {
	_, alive := LiveRunner(root)
	return runnerCycleStatus(root, now, alive)
}

func runnerCycleStatus(root string, now time.Time, alive bool) CycleStatus {
	result := CycleStatus{RunnerAlive: alive}
	request, err := tickrequest.Read(root)
	pending := err == nil && !request.At.After(now)
	if pending {
		result.Pending = request.Reason
	}
	if !alive {
		result.Line = "no steward runner is alive"
		if pending {
			result.Line += "; pending request: " + request.Reason
		}
		return result
	}
	var schedule cycleSchedule
	if err := readJSON(cycleSchedulePath(root), &schedule); err != nil {
		result.Line = "the steward's next cycle time is not available"
	} else {
		due := schedule.due(now, pending)
		result.NextDue = &due
		result.Line = "next cycle due " + due.Local().Format("2006-01-02 15:04:05 MST")
	}
	if pending {
		result.Line += "; pending request: " + request.Reason
	}
	return result
}

// pendingRequest discards invalid input once. A failed removal suppresses
// repeated reads until the next cycle; the scheduled clock always continues.
func pendingRequest(root string, now func() time.Time, report func(string), unreadable *bool) tickrequest.Request {
	if *unreadable {
		return tickrequest.Request{}
	}
	request, err := tickrequest.Read(root)
	if os.IsNotExist(err) {
		return tickrequest.Request{}
	}
	if err == nil && request.At.After(now()) {
		err = fmt.Errorf("tick request has a future time")
	}
	if err != nil {
		report("ignored tick request: " + err.Error())
		if err := tickrequest.DiscardInvalid(root, now()); err != nil {
			*unreadable = true
		}
		return tickrequest.Request{}
	}
	return request
}
