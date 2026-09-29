package lane

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise"
)

// The keeper's thrashing bounds, supervise.Breaker's numbers: the fifth
// death in a row stops the restarts; a restart after the first waits one
// base interval, doubling to the cap.
const (
	GiveUpAt     = 5
	BaseInterval = time.Minute
	BackoffCap   = 10 * time.Minute
)

// KeeperState is what the keeper remembers between cycles, shared by every
// steward of the host under the lane flock.
type KeeperState struct {
	// Failures are the owner's consecutive dead observations.
	Failures int `json:"failures,omitempty"`
	// Restarts are the starts since the owner was last seen running.
	Restarts int `json:"restarts,omitempty"`
	// Since is the first dead observation of this run of failures.
	Since      string `json:"since,omitempty"`
	LastLaunch string `json:"lastLaunch,omitempty"`
	// RetryAt is a restart the backoff holds until then.
	RetryAt   string `json:"retryAt,omitempty"`
	GaveUp    string `json:"gaveUp,omitempty"`
	LastError string `json:"lastError,omitempty"`
}

// Pause is a person's deliberate stop of the lane's owner for maintenance.
type Pause struct {
	By string `json:"by"`
	At string `json:"at"`
}

func keeperPath(home string) string { return filepath.Join(HostDir(home), "landing-lane-keeper.json") }
func pausePath(home string) string  { return filepath.Join(HostDir(home), "landing-lane-paused.json") }

// ReadKeeper is the keeper's state; the zero state when none is kept or it
// is unreadable.
func ReadKeeper(home string) KeeperState {
	var state KeeperState
	if _, err := readJSON(keeperPath(home), &state); err != nil {
		return KeeperState{}
	}
	return state
}

// ResetKeeper forgets every death: a person's landing start.
func ResetKeeper(home string) error {
	return withLock(home, func() error { return removeIfPresent(keeperPath(home)) })
}

// ReadPause is the person's pause; false when the lane is not paused.
func ReadPause(home string) (Pause, bool) {
	var pause Pause
	ok, err := readJSON(pausePath(home), &pause)
	return pause, ok && err == nil
}

// SetPause records a person's pause; a lane already paused is unchanged.
func SetPause(home, by string, now time.Time) (changed bool, err error) {
	err = withLock(home, func() error {
		if _, paused := ReadPause(home); paused {
			return nil
		}
		changed = true
		return writeJSON(home, pausePath(home), Pause{By: by, At: now.UTC().Format(time.RFC3339)})
	})
	return changed, err
}

// ClearPause ends a pause; a lane not paused is unchanged.
func ClearPause(home string) (changed bool, err error) {
	err = withLock(home, func() error {
		if _, paused := ReadPause(home); !paused {
			return nil
		}
		changed = true
		return removeIfPresent(pausePath(home))
	})
	return changed, err
}

// Keeper keeps the host lane's owner alive: one Step per steward cycle.
// Inspect reports whether the owner of a lane root runs; Start starts it
// under the lane's own ensure lock.
type Keeper struct {
	Home    string
	Now     func() time.Time
	Inspect func(root string) (bool, error)
	Start   func(root string) error
}

// Step observes the owner once and restarts it when it is dead, within the
// breaker's bounds. It returns the line a steward prints when it changes;
// empty when no lane is registered.
func (k Keeper) Step() string {
	line := ""
	err := withLock(k.Home, func() error {
		record, ok, err := Read(k.Home)
		if err != nil || !ok {
			return err
		}
		line = k.step(record)
		return nil
	})
	if err != nil {
		return "landing lane keeper: " + err.Error()
	}
	return line
}

func (k Keeper) step(record Record) string {
	root, now := record.Root, k.Now().UTC()
	if gone(root) {
		return goneRefusal(record).Error()
	}
	if pause, paused := ReadPause(k.Home); paused {
		return pausedLine(root, pause)
	}
	state := ReadKeeper(k.Home)
	alive, err := k.Inspect(root)
	if err != nil {
		return fmt.Sprintf("landing lane owner at %s: whether it runs is unknown (%v); nothing was restarted", root, err)
	}
	if alive {
		if state != (KeeperState{}) {
			if err := removeIfPresent(keeperPath(k.Home)); err != nil {
				return "landing lane keeper: " + err.Error()
			}
		}
		return "landing lane owner at " + root + " is running"
	}
	if state.GaveUp != "" {
		return GiveUpLine(root, state)
	}
	if last, err := time.Parse(time.RFC3339, state.LastLaunch); err == nil && now.Sub(last) < BaseInterval {
		return "landing lane owner at " + root + " is starting (started " + localClock(last) + ")"
	}
	if retry, err := time.Parse(time.RFC3339, state.RetryAt); err == nil {
		if now.Before(retry) {
			return fmt.Sprintf("landing lane owner at %s is down; restart %d waits until %s", root, state.Restarts+1, localClock(retry))
		}
		return k.launch(root, state, now)
	}
	breaker := supervise.Breaker{Consecutive: state.Failures, GiveUpAt: GiveUpAt, BaseInterval: BaseInterval, BackoffCap: BackoffCap}
	verdict := breaker.Advance(supervise.Failing)
	state.Failures = breaker.Consecutive
	if state.Since == "" {
		state.Since = now.Format(time.RFC3339)
	}
	if verdict.GiveUp {
		state.GaveUp = now.Format(time.RFC3339)
		if err := writeJSON(k.Home, keeperPath(k.Home), state); err != nil {
			return "landing lane keeper: " + err.Error()
		}
		return GiveUpLine(root, state)
	}
	if verdict.RelaunchAfter > 0 {
		state.RetryAt = now.Add(verdict.RelaunchAfter).Format(time.RFC3339)
		if err := writeJSON(k.Home, keeperPath(k.Home), state); err != nil {
			return "landing lane keeper: " + err.Error()
		}
		return fmt.Sprintf("landing lane owner at %s is down; restart %d waits until %s", root, state.Restarts+1, localClock(now.Add(verdict.RelaunchAfter)))
	}
	return k.launch(root, state, now)
}

func (k Keeper) launch(root string, state KeeperState, now time.Time) string {
	startErr := k.Start(root)
	state.Restarts++
	state.LastLaunch, state.RetryAt, state.LastError = now.Format(time.RFC3339), "", ""
	line := fmt.Sprintf("landing lane owner at %s was down; restarted it (restart %d)", root, state.Restarts)
	if startErr != nil {
		state.LastError = startErr.Error()
		line = fmt.Sprintf("landing lane owner at %s was down; restart %d failed: %v", root, state.Restarts, startErr)
	}
	if err := writeJSON(k.Home, keeperPath(k.Home), state); err != nil {
		return "landing lane keeper: " + err.Error()
	}
	return line
}

// GiveUpLine says, for a person, that the keeper stopped restarting the
// owner: what died, how often, since when, what to read and what to run.
func GiveUpLine(root string, state KeeperState) string {
	line := fmt.Sprintf("the landing lane owner at %s died %d times since %s and was restarted %d times; restarts stopped at %s",
		root, state.Failures, LocalText(state.Since), state.Restarts, LocalText(state.GaveUp))
	if state.LastError != "" {
		line += "; the last start failed: " + state.LastError
	}
	return line + fmt.Sprintf(". Read %s and owner.log beside it, fix the cause, then run: metasystem landing start", LastErrorPath(root))
}

// LastErrorPath is the owner's last pass error in the lane's supervision.
func LastErrorPath(root string) string {
	return filepath.Join(root, "artifacts", "agents", "supervision", "landing-owner.last-error")
}

func pausedLine(root string, pause Pause) string {
	return fmt.Sprintf("landing lane owner at %s is paused by %s at %s; metasystem landing start resumes it", root, pause.By, LocalText(pause.At))
}

func localClock(at time.Time) string { return at.Local().Format("15:04 MST") }

// LocalText is a recorded RFC 3339 time as a person reads it, in local time.
func LocalText(stamp string) string {
	at, err := time.Parse(time.RFC3339, stamp)
	if err != nil {
		return "an unrecorded time"
	}
	return at.Local().Format("2006-01-02 15:04 MST")
}
