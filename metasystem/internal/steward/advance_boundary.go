package steward

import (
	"errors"
	"fmt"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

var ErrBoundaryClosed = errors.New("the boundary's goal is no longer open")

// AdvanceBoundary prepares the newest completed event for each goal. The
// runner calls it only after this pass observed an unchanged, ready engine.
// Preparation starts no child and never ends the predecessor's session.
func AdvanceBoundary(root string, personPolicy bool, prober identity.Prober, prepare func(UnitBoundary) (BoundaryAct, error)) error {
	held, err := AcquireArbitration(root)
	if err != nil {
		return err
	}
	defer held.Release()
	events, err := ReadUnitBoundaries(root)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	var failures error
	for index := len(events) - 1; index >= 0; index-- {
		event := events[index]
		if event.Seat != canonicalPath(root) || seen[event.Goal] {
			continue
		}
		seen[event.Goal] = true
		if !personPolicy && event.Lineage == SeatLineage {
			if event.Handoff == "" {
				continue
			}
			intent, err := readExactLiveIntent(root, event.Handoff)
			if os.IsNotExist(err) {
				intent, err = ConsumedIntent(root, event.Handoff)
			}
			if err == nil && (intent.Goal != event.Goal || intent.Handoff == nil || intent.Handoff.Session != event.Session) {
				err = fmt.Errorf("unit boundary handoff does not bind this goal and session")
			}
			if err == nil {
				_, err = verifyBoundHandoffState(root, event.Handoff, event.Goal, *intent.Handoff)
			}
			if err != nil {
				events[index].Next = &BoundaryAct{Summary: "boundary preparation unavailable: " + err.Error()}
				failures = errors.Join(failures, err)
				continue
			}
			if handoffPredecessorLiveness(*intent.Handoff, prober) != identity.Dead {
				continue
			}
		} else if !personPolicy && event.Lineage == "" {
			// Old events cannot establish whether an automatic handoff is required.
			continue
		}
		act, err := prepare(event)
		if errors.Is(err, ErrBoundaryClosed) {
			events[index].Next = nil
			continue
		}
		if err != nil {
			events[index].Next = &BoundaryAct{Summary: "boundary preparation unavailable: " + err.Error()}
			failures = errors.Join(failures, err)
			continue
		}
		events[index].Next = &act
	}
	return errors.Join(failures, writeUnitBoundaries(root, events))
}

// BoundaryAdmission rechecks the newest event and, under automatic policy,
// its bound handoff and exact predecessor.
func BoundaryAdmission(root, goal, engine string, personPolicy bool, prober identity.Prober) (UnitBoundary, error) {
	events, err := ReadUnitBoundaries(root)
	if err != nil {
		return UnitBoundary{}, err
	}
	for i := len(events) - 1; i >= 0; i-- {
		event := events[i]
		if event.Seat != canonicalPath(root) || event.Goal != goal {
			continue
		}
		if event.Engine == "" || event.Engine != engine {
			return event, errors.New("the boundary waits for this engine's successful re-arm")
		}
		if !personPolicy && event.Lineage == SeatLineage {
			intent, err := readExactLiveIntent(root, event.Handoff)
			if os.IsNotExist(err) {
				intent, err = ConsumedIntent(root, event.Handoff)
			}
			if err != nil {
				return event, err
			}
			if intent.Goal != event.Goal || intent.Handoff == nil || intent.Handoff.Session != event.Session {
				return event, errors.New("the boundary waits for its bound handoff")
			}
			if _, err := verifyBoundHandoffState(root, event.Handoff, event.Goal, *intent.Handoff); err != nil {
				return event, err
			}
			if handoffPredecessorLiveness(*intent.Handoff, prober) != identity.Dead {
				return event, errors.New("the boundary waits for proven predecessor death")
			}
		} else if !personPolicy && event.Lineage == "" {
			return event, errors.New("the boundary does not establish whether its session needs a handoff")
		}
		return event, nil
	}
	return UnitBoundary{}, nil
}

// RetainBoundary records this pass's engine or the exact event's observed effect.
func RetainBoundary(root, engine string, event *UnitBoundary) error {
	held, err := AcquireArbitration(root)
	if err != nil {
		return err
	}
	defer held.Release()
	events, err := ReadUnitBoundaries(root)
	if err != nil {
		return err
	}
	for i := range events {
		if event == nil {
			events[i].Engine = engine
		} else if events[i].Goal == event.Goal && events[i].Session == event.Session && events[i].Unit == event.Unit && events[i].Outcome == event.Outcome {
			events[i].Next = event.Next
		}
	}
	return writeUnitBoundaries(root, events)
}
