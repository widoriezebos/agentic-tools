package plain

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/state"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
)

// Main sightings belong to the checkout. A status read never advances the observer clock.
type mainSighting struct {
	Commit      string    `json:"commit,omitempty"`
	Tree        string    `json:"tree,omitempty"`
	Seen        time.Time `json:"seen,omitzero"`
	Checked     bool      `json:"checked,omitempty"`
	Slot        string    `json:"slot,omitempty"`
	Pass        time.Time `json:"timer,omitzero"`
	Unreachable time.Time `json:"unreachable,omitzero"`
	Reason      string    `json:"reason,omitempty"`
}

const mainSightingFile = "main-observer.json"

func mainWords(key string, args ...any) string { return fmt.Sprintf(state.Phrase(key), args...) }
func clockText(at time.Time) string            { return at.Local().Format("15:04") }
func proofClock(stamp string) string {
	at, err := time.Parse(time.RFC3339Nano, stamp)
	if err != nil {
		return state.Phrase("main-unknown-time")
	}
	return clockText(at)
}
func batchStepLine(id, step string) string { return mainWords("main-batch", id, step) }
func mainProofLine(r Result) string {
	switch {
	case r.Result == Red && r.Attempt == "":
		return mainWords("main-red-at", proofClock(r.At))
	case r.Result == Red:
		return mainWords("main-red-result", r.Attempt, proofClock(r.At))
	case r.Scope == "inherited" || r.Base != "" && !r.Executed():
		return mainWords("main-inherited", Short(r.FullTree), proofClock(r.FullAt))
	case r.Attempt == "":
		return mainWords("main-green-at", proofClock(r.At))
	case !r.Trunk:
		return mainWords("main-batch-proof", r.Attempt, proofClock(r.At))
	default:
		return mainWords("main-green-result", r.Attempt, proofClock(r.At))
	}
}
func (s mainSighting) line(now time.Time) string {
	prefix := state.Phrase("main-unseen")
	if s.Commit != "" {
		prefix = mainWords("main-first-seen", Short(s.Commit), clockText(s.Seen))
	}
	reason := s.Reason
	switch {
	case !s.Unreachable.IsZero():
		reason = mainWords("main-origin-unreachable", clockText(s.Unreachable))
	case !s.Checked && !s.Pass.IsZero() && (now.Before(s.Pass) || now.Sub(s.Pass) > 2*lane.AgentTick):
		reason = mainWords("main-watcher-overdue", clockText(s.Pass))
	case reason != "":
	case s.Pass.IsZero():
		reason = state.Phrase("main-unwatched")
	default:
		reason = state.Phrase("main-slot-free")
	}
	return prefix + ": " + reason
}

// RecoverMovedMain shares the observer and proof slot with the resident runner.
// An explicit keeper pass cannot stand in for the resident timer's heartbeat.
func RecoverMovedMain(install, checkout string, seams ProveSeams) error {
	return observeMovedMain(install, checkout, seams, false)
}
func ObserveMovedMain(install, checkout string, seams ProveSeams) error {
	return observeMovedMain(install, checkout, seams, true)
}

func observeMovedMain(install, checkout string, seams ProveSeams, timer bool) (err error) {
	if err = os.MkdirAll(Dir(install), 0755); err != nil {
		return err
	}
	// Serialize fetch and first sighting across keepers without holding the proof
	// lock over a remote operation. Start takes the install lock inside this one.
	held, err := lock.File(filepath.Join(Dir(install), "main-observer.lock"), 0600, lock.Exclusive)
	if err != nil {
		return err
	}
	defer held.Release()
	var sight mainSighting
	if err = readAgentRecord(install, mainSightingFile, &sight); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if timer {
		sight.Pass = seams.now().UTC()
	}
	if err = seams.fetchMain(checkout); err != nil {
		if sight.Unreachable.IsZero() {
			sight.Unreachable = seams.now().UTC()
		}
		if timer {
			err = errors.New(state.Phrase("main-fetch-failed"))
		}
		return errors.Join(err, writeAgentRecord(install, mainSightingFile, sight))
	}
	seams.Trunk, seams.Gate, seams.Impact, seams.Recovery = true, false, false, true
	main, tree, err := seams.subject(checkout)
	if err != nil {
		return err
	}
	changed := main != sight.Commit || tree != sight.Tree
	sight.Commit, sight.Tree, sight.Unreachable = main, tree, time.Time{}
	sight.Checked, sight.Slot = false, ""
	if changed {
		sight.Seen = seams.now().UTC()
		sight.Reason = state.Phrase("main-checking")
		if err = writeAgentRecord(install, mainSightingFile, sight); err != nil {
			return err
		}
	}
	seams.recoveryCommit = main
	seams.recoveryAdmitted = func() error {
		sight.Reason = ""
		return writeAgentRecord(install, mainSightingFile, sight)
	}
	reason, err := sight.recover(install, checkout, seams)
	if err != nil && reason == "" {
		reason = state.Phrase("main-observation-failed")
	}
	sight.Reason = reason

	return errors.Join(err, writeAgentRecord(install, mainSightingFile, sight))
}

type mainAdmissionHold struct{ reason string }

func (h *mainAdmissionHold) Error() string { return h.reason }

func (s *mainSighting) recover(install, checkout string, seams ProveSeams) (string, error) {
	if seams.BatchMayContinue != nil && seams.BatchMayContinue() {
		if line, err := batchPendingStep(install, checkout, seams); line != "" || err != nil {
			return line, err
		}
	}
	if result, ok := applicableMainProof(install, checkout, s.Commit, s.Tree, seams, true); ok {
		s.Checked = true
		return mainProofLine(result), nil
	}
	if seams.MainHold != nil {
		if line := seams.MainHold(); line != "" {
			return line, nil
		}
		fence := seams.FenceCheck
		seams.FenceCheck = func() error {
			if line := seams.MainHold(); line != "" {
				return &mainAdmissionHold{line}
			}
			if fence != nil {
				return fence()
			}
			return nil
		}
	} else if seams.TimerHeld != nil && seams.TimerHeld() {
		return state.Phrase("main-held"), nil
	}
	running, already, err := Start(install, checkout, seams)
	var examined *Examined
	if errors.As(err, &examined) {
		return state.Phrase("main-examined"), nil
	}
	if seams.RecoveryStarted != nil {
		seams.RecoveryStarted(running, already)
	}
	var busy *Busy
	var repair *RepairPending
	var closed *AdmissionClosed
	var noRepeat *NoRepeat
	var refusal *Refusal
	var hold *mainAdmissionHold
	switch {
	case errors.As(err, &busy):
		s.Slot = busy.Running.Attempt
		return mainWords("main-busy", Short(busy.Running.Commit), proofClock(busy.Running.Since)), nil
	case errors.As(err, &repair):
		return mainWords("main-repair", repair.Fix), nil
	case errors.As(err, &closed) && closed.Unreadable == nil:
		return state.Phrase("main-drained"), nil
	case errors.As(err, &noRepeat):
		return state.Phrase("main-no-repeat"), nil
	case errors.As(err, &hold):
		return hold.reason, nil
	case errors.As(err, &refusal) && refusal.Code == "LANE_PROOF_SUBJECT":
		return state.Phrase("main-moved-again"), nil
	case err != nil:
		return "", err
	case running.Reused != nil:
		s.Checked = true
		return mainProofLine(*running.Reused), nil
	default:
		s.Slot = running.Attempt
		return mainWords("main-proof-running", running.Attempt, proofClock(running.Since)), nil
	}
}

func mainObserverStatus(install, checkout string, raw RawFacts, seams ProveSeams) string {
	var s mainSighting
	if err := readAgentRecord(install, mainSightingFile, &s); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return state.Phrase("main-read-failed")
		}
		if raw.Main != "" {
			reason := state.Phrase("main-unwatched")
			for i := len(raw.Results) - 1; raw.resultsComplete && i >= 0; i-- {
				if r := raw.Results[i]; r.Commit == raw.Main && r.Tree != "" {
					if proof, ok := recordedMainProof(install, checkout, raw.Main, r.Tree, seams, true, raw.Results); ok {
						reason = mainProofLine(proof)
					}
					break
				}
			}
			return mainWords("main-without-observer", Short(raw.Main), reason)
		}
		return s.line(raw.Now)
	}
	// A detached check can finish between timer passes. Its terminal result
	// is visible to status as soon as the proof owner publishes it.
	if s.Unreachable.IsZero() && (s.Checked || s.Slot != "" && s.Slot != raw.Running.Attempt) {
		if proof, ok := recordedMainProof(install, checkout, s.Commit, s.Tree, seams, true, raw.Results); raw.resultsComplete && ok {
			s.Checked = true
			s.Reason = mainProofLine(proof)
		} else {
			s.Checked = false
			s.Reason = state.Phrase("main-awaiting-pass")
			if s.Pass.IsZero() {
				s.Reason = state.Phrase("main-unwatched")
			}
		}
	}
	return s.line(raw.Now)
}
