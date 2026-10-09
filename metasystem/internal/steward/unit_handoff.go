package steward

import (
	"errors"
	"fmt"
	"path/filepath"
	"syscall"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// unitHandoff reads current session authority before selecting its boundary
// and capture. Callers hold handoff arbitration throughout publication.
func unitHandoff(root, session string) ([]UnitBoundary, int, *Intent, error) {
	holder, present, err := readStopCapabilityHolder(root)
	if err != nil {
		return nil, -1, nil, err
	}
	if !present || holder.OwnerLineage != SeatLineage {
		return nil, -1, nil, nil
	}
	policy, err := config.ResolvePolicy(config.GetParams{Key: "seat.driver", ConfPath: filepath.Join(root, "metasystem.conf")})
	if err != nil || policy.Value == "person" {
		return nil, -1, nil, err
	}
	events, err := ReadUnitBoundaries(root)
	if err != nil {
		return nil, -1, nil, err
	}
	intents, err := readLiveHandoffIntents(root)
	if err != nil {
		return nil, -1, nil, err
	}
	for index, event := range events {
		if event.Seat != canonicalPath(root) || event.Session != goal.NormalizeSession(session) {
			continue
		}
		for _, intent := range intents {
			binding := intent.Handoff
			if intent.Goal != event.Goal || binding.Session != event.Session || binding.MainId != holder.HolderMainID || (event.Handoff != "" && event.Handoff != intent.Nonce) {
				continue
			}
			if _, err := verifyBoundHandoffState(root, intent.Nonce, intent.Goal, *binding); err != nil {
				return nil, -1, nil, err
			}
			return events, index, &intent, nil
		}
	}
	return events, -1, nil, nil
}

func bindUnitHandoff(root, session string) ([]UnitBoundary, int, *Intent, error) {
	events, index, intent, err := unitHandoff(root, session)
	if err != nil || intent == nil {
		return events, index, intent, err
	}
	events[index].Handoff = intent.Nonce
	// Re-publication also proves durability after a prior uncertain write.
	if err := writeUnitBoundaries(root, events); err != nil {
		return nil, -1, nil, err
	}
	return events, index, intent, nil
}

// BindUnitHandoff lets a successful public capture confirm durable binding
// before the headless caller ends its turn. Person captures remain explicit.
func BindUnitHandoff(root, session string) error {
	held, err := AcquireArbitration(root)
	if err != nil {
		return err
	}
	defer held.Release()
	_, _, _, err = bindUnitHandoff(root, session)
	return err
}

// FinishUnitHandoff ends only a headless predecessor with a durable boundary
// binding. Successor admission still belongs to the exact-death check.
func FinishUnitHandoff(root, session string) error {
	held, err := AcquireArbitration(root)
	if err != nil {
		return err
	}
	defer held.Release()
	events, index, intent, err := bindUnitHandoff(root, session)
	if err != nil || intent == nil {
		return err
	}
	if events[index].Signalled {
		return nil
	}
	binding := *intent.Handoff
	prober := identity.KernelProber{}
	exact, state := inspectHandoffPredecessor(binding, prober)
	switch state {
	case identity.Dead:
		return nil
	case identity.Unknown:
		return fmt.Errorf("handoff %s held: predecessor pid %d identity is unknown; no signal sent", intent.Nonce, binding.Predecessor.Pid)
	case identity.Alive:
		if err := identity.SignalExact(prober, exact, syscall.SIGTERM); err != nil && !errors.Is(err, identity.ErrGone) {
			return fmt.Errorf("handoff %s held: predecessor signal failed: %w", intent.Nonce, err)
		}
	}
	events[index].Signalled = true
	return writeUnitBoundaries(root, events)
}
