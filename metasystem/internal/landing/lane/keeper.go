package lane

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
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
// under the lane's own ensure lock. Ready says whether an owner could run at
// all (a *Refusal naming the fix when it cannot); a lane that is not ready
// is never restarted and its deaths are not counted, because a restart
// cannot succeed until a person acts.
type Keeper struct {
	Home    string
	Now     func() time.Time
	Inspect func(root string) (bool, error)
	Start   func(root string) error
	Ready   func(root string) error
	// Hold names why no owner may start now, empty when one may: a landing
	// agent launch that has not ended is the lane's one composition owner,
	// and no batch owner starts beside it. An error holds too. nil holds
	// nothing.
	Hold func(root string) (string, error)
}

// UnarmedRefusal is a lane whose checkout's supervision is not armed:
// nothing there can start or keep its owner, whatever the keeper or landing
// start asks for.
func UnarmedRefusal(root string) *Refusal {
	return &Refusal{Code: CodeUnarmed,
		Message: fmt.Sprintf("the landing lane can't run: its checkout %s isn't started (its supervision is not armed)", root),
		Fix:     "a person runs, at a terminal no agent started: metasystem system start --repo " + root,
		Argv:    []string{"metasystem", "system", "start", "--repo", root}}
}

// NoMachineRefusal is a landing checkout with no machine nickname: its owner
// signs what it lands with that name, so it stops at every start there.
func NoMachineRefusal(root string) *Refusal {
	return &Refusal{Code: CodeNoMachine,
		Message: fmt.Sprintf("the landing checkout %s has no machine nickname yet, so the lane can't run there", root),
		Fix:     "name it once: git -C " + root + " config metasystem.goal.machine landing (any one word), then run metasystem landing start",
		Argv:    []string{"git", "-C", root, "config", "metasystem.goal.machine", "landing"}}
}

// notReadyLine is the keeper's line for a lane whose owner cannot run.
func notReadyLine(root string, err error) string {
	var refusal *Refusal
	if errors.As(err, &refusal) {
		return refusal.Message + "; nothing was started; " + refusal.Fix
	}
	return fmt.Sprintf("landing lane owner at %s: whether it can run is unknown (%v); nothing was started", root, err)
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
	if k.Ready != nil {
		if err := k.Ready(root); err != nil {
			return notReadyLine(root, err)
		}
	}
	if k.Hold != nil {
		reason, err := k.Hold(root)
		if err != nil {
			return fmt.Sprintf("landing lane owner at %s is down and was not restarted: whether a landing agent runs can't be read (%v)", root, err)
		}
		if reason != "" {
			return fmt.Sprintf("landing lane owner at %s is down and was not restarted: %s, and no batch owner runs beside it", root, reason)
		}
	}
	if state.GaveUp != "" {
		return GiveUpLine(root, state)
	}
	if last, err := time.Parse(time.RFC3339, state.LastLaunch); err == nil && now.Sub(last) < BaseInterval {
		return "landing lane owner at " + root + " is starting (started " + localClock(last) + ")"
	}
	if retry, err := time.Parse(time.RFC3339, state.RetryAt); err == nil {
		if now.Before(retry) {
			return fmt.Sprintf("landing lane owner at %s is down%s; restart %d waits until %s", root, lastErrorClause(root), state.Restarts+1, localClock(retry))
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
		return fmt.Sprintf("landing lane owner at %s is down%s; restart %d waits until %s", root, lastErrorClause(root), state.Restarts+1, localClock(now.Add(verdict.RelaunchAfter)))
	}
	return k.launch(root, state, now)
}

func (k Keeper) launch(root string, state KeeperState, now time.Time) string {
	startErr := k.Start(root)
	state.Restarts++
	state.LastLaunch, state.RetryAt, state.LastError = now.Format(time.RFC3339), "", ""
	// The start asks the checkout's supervision for an owner; whether one
	// runs is the next cycle's observation, never this line's claim.
	line := fmt.Sprintf("landing lane owner at %s was down%s; asked its supervision to start it (restart %d)", root, lastErrorClause(root), state.Restarts)
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
	} else if last := LastErrorLine(root); last != "" {
		line += "; its last error: " + last
	}
	return line + fmt.Sprintf(". Read %s and owner.log beside it, fix the cause, then run: metasystem landing start", LastErrorPath(root))
}

// LastErrorPath is the owner's last pass error in the lane's supervision:
// the installation's, which a checkout that nests the module keeps under
// <lane>/metasystem.
func LastErrorPath(root string) string {
	return filepath.Join(batch.ModuleRoot(root), "artifacts", "agents", "supervision", "landing-owner.last-error")
}

// LastErrorLine is the owner's last pass error as one line; empty when it
// recorded none.
func LastErrorLine(root string) string { return oneLine(LastErrorPath(root)) }

// TickErrorPath is the owner's last batch tick error, beside its last pass
// error: written when a pass's tick reports one, removed by a clean pass.
func TickErrorPath(root string) string {
	return filepath.Join(batch.ModuleRoot(root), "artifacts", "agents", "supervision", "landing-owner.last-tick-error")
}

// LastTickErrorLine is the owner's last tick error as one line; empty when
// its last pass ticked clean.
func LastTickErrorLine(root string) string { return oneLine(TickErrorPath(root)) }

func oneLine(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var lines []string
	for _, line := range strings.Split(string(data), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "; ")
}

func lastErrorClause(root string) string {
	if last := LastErrorLine(root); last != "" {
		return " (its last error: " + last + ")"
	}
	return ""
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
