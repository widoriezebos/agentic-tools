package gaterun

import (
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
