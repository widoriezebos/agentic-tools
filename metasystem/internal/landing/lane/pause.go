package lane

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

// Pause is a person's deliberate stop of the lane for maintenance.
type Pause struct {
	By string `json:"by"`
	At string `json:"at"`
	// Reason is why it was stopped, in the stopper's words; empty when
	// none was given.
	Reason string `json:"reason,omitempty"`
	// unreadable marks the pause ReadPause gives for a record it could not
	// read.
	unreadable bool
}

// Unreadable says whether this pause stands for a pause record that could
// not be read (ReadPause fails closed); the lane's status says it.
func (pause Pause) Unreadable() bool { return pause.unreadable }

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
		return Pause{By: unreadablePauseBy, unreadable: true}, true
	}
	return pause, ok
}

// SetPause records a person's pause; a lane already paused is unchanged.
func SetPause(home, by string, now time.Time) (changed bool, err error) {
	return SetPauseBecause(home, by, "", now)
}

// SetPauseBecause records a person's pause with why it was made; a lane
// already paused is unchanged, its first reason kept. A pause written but not
// confirmed on disk returns changed and ErrNotDurable.
func SetPauseBecause(home, by, reason string, now time.Time) (changed bool, err error) {
	err = withLock(home, func() error {
		changed, err = setPauseLockedBecause(home, by, reason, now)
		return err
	})
	return changed, err
}

// setPauseLocked is SetPause for a caller that holds the lane flock.
func setPauseLocked(home, by string, now time.Time) (bool, error) {
	return setPauseLockedBecause(home, by, "", now)
}

func setPauseLockedBecause(home, by, reason string, now time.Time) (bool, error) {
	if _, paused := ReadPause(home); paused {
		return false, nil
	}
	return true, writeJSON(home, pausePath(home), Pause{By: by, At: now.UTC().Format(time.RFC3339), Reason: PauseReason(reason)})
}

// pauseReasonLimit caps a pause's reason, which line 1 of messages shows.
const pauseReasonLimit = 200

// PauseReason is a stop's reason as one plain line: control characters and
// runs of whitespace become one space, and it is cut at pauseReasonLimit
// characters.
func PauseReason(reason string) string {
	plain := strings.Join(strings.Fields(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, reason)), " ")
	if runes := []rune(plain); len(runes) > pauseReasonLimit {
		plain = strings.TrimSpace(string(runes[:pauseReasonLimit-1])) + "…"
	}
	return plain
}

// Who names who stopped the lane and why, as line 1 words: "Wido" or
// "Wido (the same member red twice)".
func (pause Pause) Who() string {
	by := pause.By
	if by == "" {
		by = "a person"
	}
	if pause.Reason == "" {
		return by
	}
	return by + " (" + pause.Reason + ")"
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
