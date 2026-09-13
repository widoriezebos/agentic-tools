package supervise

import (
	"fmt"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginebuild"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/registry"
)

func slotClassName(class registry.SlotClass) string {
	switch class {
	case registry.Free:
		return "Free"
	case registry.LiveVerified:
		return "LiveVerified"
	case registry.OpenReservation:
		return "OpenReservation"
	case registry.UnknownLiveness:
		return "UnknownLiveness"
	case registry.DeadOwner:
		return "DeadOwner"
	case registry.SweepableClosed:
		return "SweepableClosed"
	default:
		return fmt.Sprintf("SlotClass(%d)", class)
	}
}

// EngineFloorProblems reports every consumed checkout whose published engine
// cannot safely read a ledger written at the recorded fleet floor. Free slots
// carry no running or possibly-surviving seat and are deliberately ignored.
func EngineFloorProblems(floor string, isAncestor func(a, b string) (bool, error), probe identity.Prober, now time.Time) ([]string, error) {
	path, err := registry.DefaultPath()
	if err != nil {
		return nil, err
	}
	frames, err := registry.ReadFrames(path)
	if err != nil {
		return nil, fmt.Errorf("read supervision registry: %w", err)
	}
	reduction, err := registry.Reduce(frames)
	if err != nil {
		return nil, fmt.Errorf("reduce supervision registry: %w", err)
	}

	var problems []string
	for _, slot := range registry.Slots(reduction, probe, now, 0, nil) {
		if slot.Class == registry.Free {
			continue
		}
		claim := reduction.Claims[slot.OwnerTag]
		if claim == nil || claim.CheckoutPath == "" {
			problems = append(problems, fmt.Sprintf("checkout <missing for slot %s> (%s) runs engine unreadable, below the fleet floor %s; rebuild and re-arm it (metasystem up), close or sweep the stale claim, or record a lower floor only if every other seat runs that", slot.OwnerTag, slotClassName(slot.Class), floor))
			continue
		}
		engine, readErr := (&DiskCheckout{Root: claim.CheckoutPath}).EngineBuild()
		if readErr != nil {
			problems = append(problems, fmt.Sprintf("checkout %s (%s) runs engine unreadable (%v), below the fleet floor %s; rebuild and re-arm it (metasystem up), close or sweep the stale claim, or record a lower floor only if every other seat runs that", claim.CheckoutPath, slotClassName(slot.Class), readErr, floor))
			continue
		}
		commit, _, ok := enginebuild.StampCommit(engine)
		if !ok {
			problems = append(problems, fmt.Sprintf("checkout %s (%s) runs engine %s, below the fleet floor %s; rebuild and re-arm it (metasystem up), close or sweep the stale claim, or record a lower floor only if every other seat runs that", claim.CheckoutPath, slotClassName(slot.Class), engine, floor))
			continue
		}
		atOrAbove, err := isAncestor(floor, commit)
		if err != nil {
			return nil, fmt.Errorf("compare checkout %s engine %s with fleet floor %s: %w", claim.CheckoutPath, engine, floor, err)
		}
		if !atOrAbove {
			problems = append(problems, fmt.Sprintf("checkout %s (%s) runs engine %s, below the fleet floor %s; rebuild and re-arm it (metasystem up), close or sweep the stale claim, or record a lower floor only if every other seat runs that", claim.CheckoutPath, slotClassName(slot.Class), engine, floor))
		}
	}
	return problems, nil
}
