package gaterun

import (
	"fmt"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// Holder is one live gate run that blocks a fence: a registered gate process
// that is alive and does not belong to the asking process's own chain.
type Holder struct {
	Pid  int64
	Gate string
}

// selfChain collects a pid and its ancestors, bounded against ppid cycles.
// The chain is what makes a fence self-exempt: the suite registers its own
// shell pid, and the go gate it sources or spawns must not read that marker
// as a foreign run.
func selfChain(pid int64) map[int64]bool {
	chain := map[int64]bool{}
	for pid > 0 && !chain[pid] {
		chain[pid] = true
		parent, ok := identity.ParentPid(pid)
		if !ok {
			break
		}
		pid = parent
	}
	return chain
}

// Fence returns the live gate runs in root that are foreign to selfPid's
// process chain, pruning dead or unparsable markers through the same
// liveMarkers scan Running uses. An empty result means the checkout is clear
// to start a gate or rebuild the binary a live gate is running on.
func Fence(root string, selfPid int64) []Holder {
	chain := selfChain(selfPid)
	var holders []Holder
	for _, marker := range liveMarkers(root) {
		if chain[marker.Pid] {
			continue
		}
		holders = append(holders, Holder{Pid: marker.Pid, Gate: marker.Gate})
	}
	return holders
}

type parentPIDLookup func(pid int64) (int64, bool)

const maxControllerAncestryDepth = 64

type ancestryIdentity struct {
	ref                identity.Ref
	startedAtUnixMicro int64
}

// ControllerDescendant proves that consumerPid is below one exact live
// controller identity. Environment correlation is not authority: the proof
// records each process identity, then confirms every child-to-parent edge and
// the terminal controller identity before it accepts the ancestry.
func ControllerDescendant(consumerPid int64, controller identity.Ref) error {
	return controllerDescendant(identity.KernelProber{}, identity.ParentPid, consumerPid, controller)
}

func controllerDescendant(prober identity.Prober, parentPID parentPIDLookup, consumerPid int64, controller identity.Ref) error {
	if consumerPid <= 0 || controller.Pid <= 0 || controller.StartedAtSec <= 0 {
		return fmt.Errorf("controller ancestry requires positive consumer and complete controller identity")
	}
	if controller.StartTicks < 0 || (controller.StartTicks == 0) != (controller.BootID == "") {
		return fmt.Errorf("controller ancestry has an incomplete pair identity")
	}
	if consumerPid == controller.Pid {
		return fmt.Errorf("consumer pid is the controller, not its descendant")
	}

	seen := make(map[int64]bool)
	chain := make([]ancestryIdentity, 0, 4)
	current := consumerPid
	for depth := 0; depth < maxControllerAncestryDepth; depth++ {
		if seen[current] {
			return fmt.Errorf("consumer ancestry contains a pid cycle at %d", current)
		}
		seen[current] = true
		recorded, err := probeAncestryIdentity(prober, current)
		if err != nil {
			return fmt.Errorf("consumer ancestry at pid %d: %w", current, err)
		}
		chain = append(chain, recorded)
		if current == controller.Pid {
			if !recorded.matchesRef(controller) {
				return fmt.Errorf("ancestry terminal pid %d does not match the recorded controller identity", current)
			}
			return confirmAncestryChain(prober, parentPID, chain, controller)
		}
		parent, ok := parentPID(current)
		if !ok || parent <= 0 || parent == current {
			return fmt.Errorf("consumer pid %d does not have readable ancestry to controller pid %d", current, controller.Pid)
		}
		current = parent
	}
	return fmt.Errorf("consumer ancestry exceeds the %d-process proof bound", maxControllerAncestryDepth)
}

func probeAncestryIdentity(prober identity.Prober, pid int64) (ancestryIdentity, error) {
	exact, live, err := prober.Probe(pid)
	if err != nil || live == identity.Unknown {
		return ancestryIdentity{}, fmt.Errorf("identity is unreadable")
	}
	if live != identity.Alive {
		return ancestryIdentity{}, fmt.Errorf("identity is dead")
	}
	if exact.Pid != pid {
		return ancestryIdentity{}, fmt.Errorf("probe returned pid %d", exact.Pid)
	}
	ref := exact.Ref()
	if ref.StartedAtSec <= 0 || exact.StartedAt.UnixMicro() <= 0 {
		return ancestryIdentity{}, fmt.Errorf("start identity is incomplete")
	}
	if ref.StartTicks < 0 || (ref.StartTicks == 0) != (ref.BootID == "") {
		return ancestryIdentity{}, fmt.Errorf("pair identity is incomplete")
	}
	return ancestryIdentity{ref: ref, startedAtUnixMicro: exact.StartedAt.UnixMicro()}, nil
}

func (recorded ancestryIdentity) sameProcess(current ancestryIdentity) bool {
	if recorded.ref.Pid != current.ref.Pid {
		return false
	}
	if recorded.ref.StartTicks > 0 && recorded.ref.BootID != "" {
		return recorded.ref.StartTicks == current.ref.StartTicks && recorded.ref.BootID == current.ref.BootID
	}
	return recorded.startedAtUnixMicro == current.startedAtUnixMicro
}

func (recorded ancestryIdentity) matchesRef(ref identity.Ref) bool {
	if recorded.ref.Pid != ref.Pid {
		return false
	}
	if ref.StartTicks > 0 && ref.BootID != "" {
		return recorded.ref.StartTicks == ref.StartTicks && recorded.ref.BootID == ref.BootID
	}
	return recorded.ref.StartedAtSec == ref.StartedAtSec
}

func confirmAncestryChain(prober identity.Prober, parentPID parentPIDLookup, chain []ancestryIdentity, controller identity.Ref) error {
	for index := 0; index+1 < len(chain); index++ {
		if err := confirmAncestryEdge(prober, parentPID, chain[index], chain[index+1]); err != nil {
			return err
		}
	}
	terminal := chain[len(chain)-1]
	current, err := probeAncestryIdentity(prober, terminal.ref.Pid)
	if err != nil {
		return fmt.Errorf("controller identity confirmation: %w", err)
	}
	if !terminal.sameProcess(current) || !current.matchesRef(controller) {
		return fmt.Errorf("controller identity changed during the ancestry proof")
	}
	return nil
}

func confirmAncestryEdge(prober identity.Prober, parentPID parentPIDLookup, child, parent ancestryIdentity) error {
	currentChild, err := probeAncestryIdentity(prober, child.ref.Pid)
	if err != nil {
		return fmt.Errorf("ancestry child pid %d confirmation: %w", child.ref.Pid, err)
	}
	if !child.sameProcess(currentChild) {
		return fmt.Errorf("ancestry child pid %d died or was reused during the proof", child.ref.Pid)
	}
	if err := confirmParentPID(parentPID, child.ref.Pid, parent.ref.Pid); err != nil {
		return err
	}
	currentParent, err := probeAncestryIdentity(prober, parent.ref.Pid)
	if err != nil {
		return fmt.Errorf("ancestry parent pid %d confirmation: %w", parent.ref.Pid, err)
	}
	if !parent.sameProcess(currentParent) {
		return fmt.Errorf("ancestry parent pid %d died or was reused during the proof", parent.ref.Pid)
	}
	currentChild, err = probeAncestryIdentity(prober, child.ref.Pid)
	if err != nil {
		return fmt.Errorf("ancestry child pid %d final confirmation: %w", child.ref.Pid, err)
	}
	if !child.sameProcess(currentChild) {
		return fmt.Errorf("ancestry child pid %d changed during edge confirmation", child.ref.Pid)
	}
	return confirmParentPID(parentPID, child.ref.Pid, parent.ref.Pid)
}

func confirmParentPID(parentPID parentPIDLookup, childPID, recordedParentPID int64) error {
	currentParentPID, ok := parentPID(childPID)
	if !ok || currentParentPID <= 0 || currentParentPID == childPID {
		return fmt.Errorf("ancestry child pid %d has unreadable parentage during confirmation", childPID)
	}
	if currentParentPID != recordedParentPID {
		return fmt.Errorf("ancestry child pid %d parent changed from %d to %d", childPID, recordedParentPID, currentParentPID)
	}
	return nil
}
