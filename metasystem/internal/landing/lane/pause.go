package lane

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/state"
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
	return setPauseBecause(home, by, "", now, "pause")
}

// SetPauseBecause records a person's pause with why it was made; a lane
// already paused is unchanged, its first reason kept. A pause written but not
// confirmed on disk returns changed and ErrNotDurable.
func SetPauseBecause(home, by, reason string, now time.Time) (changed bool, err error) {
	return setPauseBecause(home, by, reason, now, "stop")
}

func setPauseBecause(home, by, reason string, now time.Time, verb string) (changed bool, err error) {
	err = withLock(home, func() error {
		changed, err = setPauseLockedBecause(home, by, reason, now, verb)
		return err
	})
	return changed, err
}

// setPauseLocked is SetPause for a caller that holds the lane flock.
func setPauseLocked(home, by string, now time.Time) (bool, error) {
	return setPauseLockedBecause(home, by, "", now)
}

func setPauseLockedBecause(home, by, reason string, now time.Time, verbs ...string) (bool, error) {
	if _, paused := ReadPause(home); paused {
		return false, nil
	}
	pause := Pause{By: by, At: now.UTC().Format(time.RFC3339), Reason: PauseReason(reason)}
	if err := writeJSON(home, pausePath(home), pause); err != nil {
		return true, err
	}
	verb := "pause"
	if len(verbs) > 0 {
		verb = verbs[0]
	}
	return true, pauseTransition(home, "running", "paused", verb, pause)
}

func pauseTransition(home, from, to, verb string, pause Pause) error {
	registration, present, err := Read(home)
	if err != nil || !present {
		return nil
	}
	dir := filepath.Join(registration.Install, "artifacts", "agents", "landing")
	shadow, _, err := state.Read(dir)
	if err != nil {
		at, _ := time.Parse(time.RFC3339Nano, pause.At)
		state.ReportFailure(dir, at, verb, err)
		return nil
	}
	if shadow.Stop != nil {
		from, to = "stopped", "stopped"
	}
	at, _ := time.Parse(time.RFC3339Nano, pause.At)
	_, err = state.Transition(dir, at, "lane", from, to, verb, pause.Who(), func(r *state.Record) {
		r.Lane = state.Lane{Fact: state.Fact{Word: to, Since: state.Since{At: at}}, By: pause.By, Reason: pause.Reason}
		if shadow.Stop != nil && verb == "resume" {
			r.Lane = state.Lane{Fact: state.Fact{Word: to, Since: shadow.Lane.Since}}
		}
	})
	if err != nil {
		state.ReportFailure(dir, at, verb, err)
	}
	return nil
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
	return ClearPauseAt(home, "a person", time.Now())
}

func ReadResume(home string) (resume Pause, present bool, err error) {
	present, err = readJSON(filepath.Join(HostDir(home), "landing-lane-resumed.json"), &resume)
	return
}

// ResumeRecordError is an advisory failure after the pause was cleared.
type ResumeRecordError struct{ Err error }

func (e *ResumeRecordError) Error() string {
	return "the resume time could not be recorded: " + e.Err.Error()
}
func (e *ResumeRecordError) Unwrap() error { return e.Err }

// ClearPauseAt clears the pause before recording the shadow's resume time.
// A ResumeRecordError never means the lane is still paused.
func ClearPauseAt(home, by string, now time.Time) (changed bool, err error) {
	err = withLock(home, func() error {
		pause, paused := ReadPause(home)
		if !paused {
			return nil
		}
		if err := removeIfPresent(pausePath(home)); err != nil {
			return err
		}
		changed = true
		resume := Pause{At: now.UTC().Format(time.RFC3339Nano), By: by}
		if err := writeJSON(home, filepath.Join(HostDir(home), "landing-lane-resumed.json"), resume); err != nil {
			return &ResumeRecordError{Err: err}
		}
		from := "paused"
		if pause.Unreadable() {
			from = "stopped"
		}
		return pauseTransition(home, from, "running", "resume", resume)
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
