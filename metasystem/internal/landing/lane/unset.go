package lane

import (
	"errors"
	"path/filepath"
	"time"
)

// The steps of landing unset (design r10 §1), journaled in host state as
// each completes.
const (
	StepFenced  = "fenced"
	StepSettled = "settled"
)

// UnsetJournal is a person's unset under way: the lane it takes away, who
// asked and the steps done. While it exists the lane is fenced: no agent
// starts.
type UnsetJournal struct {
	By           string      `json:"by"`
	At           string      `json:"at"`
	Root         string      `json:"root"`
	Install      string      `json:"install"`
	CustodyEpoch uint64      `json:"custodyEpoch"`
	Steps        []UnsetStep `json:"steps"`
}

// UnsetStep is one completed step and when.
type UnsetStep struct {
	Step string `json:"step"`
	At   string `json:"at"`
}

// Done reports whether the journal records step as completed.
func (journal UnsetJournal) Done(step string) bool {
	for _, done := range journal.Steps {
		if done.Step == step {
			return true
		}
	}
	return false
}

func unsetPath(home string) string { return filepath.Join(HostDir(home), "landing-lane-unset.json") }

// ReadUnset reads the unset journal. fenced is true whenever a journal is
// there, readable or not: the fence fails closed, and an unreadable
// journal is returned with its error.
func ReadUnset(home string) (journal UnsetJournal, fenced bool, err error) {
	ok, err := readJSON(unsetPath(home), &journal)
	if err != nil {
		return UnsetJournal{}, true, err
	}
	return journal, ok, nil
}

// Settlement is what settling the lane found still running: a Live agent
// the unset waits for, and Unknown state a person may override with --force.
type Settlement struct {
	Live    []string `json:"live,omitempty"`
	Unknown []string `json:"unknown,omitempty"`
}

// Settled reports whether nothing live remains and nothing unknown, or a
// person forced past the unknown.
func (s Settlement) Settled(force bool) bool {
	return len(s.Live) == 0 && (len(s.Unknown) == 0 || force)
}

// UnsetSeams are the unset's steps over the lane's layout.
type UnsetSeams struct {
	// Settle stops the lane's agent and reads whether it ended (step 2).
	Settle func(Layout) (Settlement, error)
}

// UnsetReport is where an unset stands after one call.
type UnsetReport struct {
	// NoLane: no lane was registered and no unset was under way.
	NoLane bool `json:"noLane,omitempty"`
	// Resumed: this call continued an unset a call before it began.
	Resumed bool   `json:"resumed,omitempty"`
	Record  Record `json:"record"`
	// Unregistered: the lane is gone; each seat lands its own work.
	Unregistered bool `json:"unregistered,omitempty"`
	// CheckoutGone: the lane's checkout no longer existed; the lane was
	// unregistered as it stood.
	CheckoutGone bool `json:"checkoutGone,omitempty"`
	// Stopped is the step the unset waits at when it is not done.
	Stopped    string     `json:"stopped,omitempty"`
	Settlement Settlement `json:"settlement"`
}

// Unset takes the host's lane away at a person's word (design r10 §1). It
// is never refused and it is resumable: each step is journaled in host
// state, and the same call continues from the journal.
//
//  1. Fence: the journal is written and the lane paused, under the host
//     flock; no agent starts from then on.
//  2. Settle: stop the landing agent and wait for it to end (a person's
//     force overrides only unknown state).
//  3. Unregister, under the host flock.
func Unset(home, by string, now time.Time, force bool, seams UnsetSeams) (UnsetReport, error) {
	journal, fresh, goneRoot, err := fence(home, by, now)
	if goneRoot != "" && err == nil {
		return UnsetReport{Record: Record{Root: goneRoot}, Unregistered: true, CheckoutGone: true}, nil
	}
	if err != nil || journal == nil {
		return UnsetReport{NoLane: journal == nil && err == nil}, err
	}
	report := UnsetReport{Resumed: !fresh, Record: Record{Root: journal.Root, Install: journal.Install, CustodyEpoch: journal.CustodyEpoch}}
	layout, err := report.Record.Layout()
	if err != nil {
		return report, err
	}
	if !journal.Done(StepSettled) {
		settlement, err := seams.Settle(layout)
		if err != nil {
			return report, err
		}
		report.Settlement = settlement
		if !settlement.Settled(force) {
			report.Stopped = StepSettled
			return report, nil
		}
		if err := journalStep(home, StepSettled, now); err != nil {
			return report, err
		}
	}
	if err := unregister(home); err != nil {
		return report, err
	}
	report.Unregistered = true
	return report, nil
}

// fence writes the unset journal and the pause under the host flock, or
// returns the journal of the unset already under way (fresh false). No
// registered lane and no unset returns a nil journal. A journal that cannot
// be read is written again from the record, still fenced: its steps are
// each safe to run again; with no record left it is the end of an unset
// that ended, and it is removed.
func fence(home, by string, now time.Time) (journal *UnsetJournal, fresh bool, goneRoot string, err error) {
	err = withLock(home, func() error {
		existing, fenced, readErr := ReadUnset(home)
		if fenced && readErr == nil {
			if isGone, err := checkoutGone(existing.Root); err != nil {
				return unreachableRefusal(existing.Root, err)
			} else if isGone {
				goneRoot = existing.Root
				return removeLane(home)
			}
			journal = &existing
			_, err := setPauseLocked(home, by, now)
			return written(err)
		}
		record, ok, err := Read(home)
		if ok && record.Root != "" {
			// The checkout that held the lane's batches is gone, so no
			// member is left in the lane's custody to return: the lane
			// goes, and the person hears which goals the ledger still
			// shows it holding.
			if isGone, err := checkoutGone(record.Root); err != nil {
				return unreachableRefusal(record.Root, err)
			} else if isGone {
				goneRoot = record.Root
				return removeLane(home)
			}
		}
		var refusal *Refusal
		switch {
		case err != nil && !(errors.As(err, &refusal) && refusal.Code == CodeRecordIncomplete):
			return &Refusal{Code: CodeRecordIncomplete,
				Message: "this computer's landing lane record cannot be read (" + err.Error() + "), so the lane's members are not known and nothing was returned",
				Fix:     "a person registers the landing checkout again, which replaces the record, then unsets it: metasystem landing set PATH",
				Argv:    []string{"metasystem", "landing", "set", "PATH"}}
		case !ok && fenced:
			// The record goes before the journal when an unset ends: a
			// journal left without a record is an unset that ended.
			return removeIfPresent(unsetPath(home))
		case !ok:
			return nil
		}
		install := record.Install
		if refusal != nil {
			// An older engine's record: its layout is resolved now, once, as
			// landing set would.
			layout, err := NewLayout(record.Root)
			if err != nil {
				// landing set would refuse this checkout for the same reason,
				// so the way on is to fix that, then unset again.
				why := err.Error()
				var invalid *Refusal
				if errors.As(err, &invalid) {
					why = invalid.Message + "; " + invalid.Fix
				}
				return &Refusal{Code: CodeRecordIncomplete,
					Message: "this computer's landing lane " + record.Root + " was registered by an older engine, and its layout can't be read now: " + why,
					Fix:     "fix the checkout as said, then take the lane away: metasystem landing unset",
					Argv:    []string{"metasystem", "landing", "unset"}}
			}
			install = string(layout.Install)
		}
		at := now.UTC().Format(time.RFC3339)
		journal = &UnsetJournal{By: by, At: at, Root: record.Root, Install: install, CustodyEpoch: record.CustodyEpoch, Steps: []UnsetStep{{Step: StepFenced, At: at}}}
		fresh = !fenced
		if err := written(writeJSON(home, unsetPath(home), *journal)); err != nil {
			return err
		}
		_, err = setPauseLocked(home, by, now)
		return written(err)
	})
	if err != nil {
		return nil, false, "", err
	}
	return journal, fresh, goneRoot, nil
}

func unreachableRefusal(root string, err error) *Refusal {
	return &Refusal{Code: CodeUnreachable,
		Message: "the landing checkout " + root + " can't be reached (" + err.Error() + "), so nothing was changed",
		Fix:     "make it reachable (mount its volume), then run again: metasystem landing unset",
		Argv:    []string{"metasystem", "landing", "unset"}}
}

// removeLane removes the lane's host state, the journal last; the caller
// holds the lane flock.
func removeLane(home string) error {
	for _, path := range []string{RecordPath(home), pausePath(home), unsetPath(home)} {
		if err := removeIfPresent(path); err != nil {
			return err
		}
	}
	return nil
}

// journalStep records step as done, once.
func journalStep(home, step string, now time.Time) error {
	return withLock(home, func() error {
		journal, fenced, err := ReadUnset(home)
		if err != nil || !fenced {
			return errors.Join(err, errors.New("the record of this landing unset is gone, so it did not go on"))
		}
		if !journal.Done(step) {
			journal.Steps = append(journal.Steps, UnsetStep{Step: step, At: now.UTC().Format(time.RFC3339)})
		}
		return written(writeJSON(home, unsetPath(home), journal))
	})
}

// unregister removes the lane record, the pause and the journal under the
// host flock.
func unregister(home string) error {
	return withLock(home, func() error {
		journal, fenced, err := ReadUnset(home)
		if err != nil || !fenced {
			return errors.Join(err, errors.New("the record of this landing unset is gone, so the lane was not unregistered"))
		}
		// A pre-push hook an earlier engine installed goes with it, so the
		// checkout pushes as any other. The fence's pause goes with the lane: a later landing
		// set starts a lane that is not stopped. The journal goes last, so
		// a crash in between leaves an unset that ends when run again.
		if err := removeHook(journal.Root); err != nil {
			return err
		}
		return removeLane(home)
	})
}

func unsettingRefusal(journal UnsetJournal) *Refusal {
	by := journal.By
	if by == "" {
		by = "a person"
	}
	return &Refusal{Code: CodeUnsetting,
		Message: "this computer's landing lane is being unset by " + by + ", so it takes no new work; nothing was done",
		Fix:     "the person finishes it with metasystem landing unset; after that each seat lands its own work",
		Argv:    []string{"metasystem", "landing", "unset"}}
}
