package lane

import (
	"fmt"
	"path/filepath"
	"time"
)

// Pause is a person's deliberate stop of the lane for maintenance.
type Pause struct {
	By string `json:"by"`
	At string `json:"at"`
}

func pausePath(home string) string { return filepath.Join(HostDir(home), "landing-lane-paused.json") }

// unreadablePauseBy names who paused a lane whose pause record cannot be read.
const unreadablePauseBy = "an unreadable pause record"

// ReadPause is the person's pause; false only when no pause record exists.
// The pause fails closed (design r10 K2): a record that is there but cannot
// be read, for any reason, reads as paused.
func ReadPause(home string) (Pause, bool) {
	var pause Pause
	ok, err := readJSON(pausePath(home), &pause)
	if err != nil {
		return Pause{By: unreadablePauseBy}, true
	}
	return pause, ok
}

// PauseState is ReadPause for a reader that must tell an unreadable pause
// from none: the error is set when the pause record exists but cannot be
// read or decoded.
func PauseState(home string) (Pause, bool, error) {
	var pause Pause
	ok, err := readJSON(pausePath(home), &pause)
	if err != nil {
		return Pause{}, false, err
	}
	return pause, ok, nil
}

// SetPause records a person's pause; a lane already paused is unchanged.
func SetPause(home, by string, now time.Time) (changed bool, err error) {
	err = withLock(home, func() error {
		changed, err = setPauseLocked(home, by, now)
		return err
	})
	return changed, err
}

// setPauseLocked is SetPause for a caller that holds the lane flock.
func setPauseLocked(home, by string, now time.Time) (bool, error) {
	if _, paused := ReadPause(home); paused {
		return false, nil
	}
	return true, writeJSON(home, pausePath(home), Pause{By: by, At: now.UTC().Format(time.RFC3339)})
}

// ClearPause ends a pause, a readable one or not; a lane not paused is
// unchanged.
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

// UnarmedRefusal is a lane whose checkout's supervision is not armed:
// nothing there wakes its landing agent, whatever landing start asks for.
func UnarmedRefusal(root string) *Refusal {
	return &Refusal{Code: CodeUnarmed,
		Message: fmt.Sprintf("the landing lane can't run: its checkout %s isn't started (its supervision is not armed)", root),
		Fix:     "a person runs, at a terminal no agent started: metasystem system start --repo " + root,
		Argv:    []string{"metasystem", "system", "start", "--repo", root}}
}

// NoMachineRefusal is a landing checkout with no machine nickname: the
// lane's claim identity names that machine, so the lane can't run there.
func NoMachineRefusal(root string) *Refusal {
	return &Refusal{Code: CodeNoMachine,
		Message: fmt.Sprintf("the landing checkout %s has no machine nickname yet, so the lane can't run there", root),
		Fix:     "name it once: git -C " + root + " config metasystem.goal.machine landing (any one word), then run metasystem landing start",
		Argv:    []string{"git", "-C", root, "config", "metasystem.goal.machine", "landing"}}
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
