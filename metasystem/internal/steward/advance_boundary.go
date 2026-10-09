package steward

import (
	"errors"
	"fmt"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

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
		if err != nil {
			events[index].Next = &BoundaryAct{Summary: "boundary preparation unavailable: " + err.Error()}
			return errors.Join(err, writeUnitBoundaries(root, events))
		}
		events[index].Next = &act
	}
	return writeUnitBoundaries(root, events)
}
