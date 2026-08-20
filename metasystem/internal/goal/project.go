package goal

// The canonical projection (BGS-13): every read — list, next, the
// open-work verdict — comes from the ACCEPTED ref's tree, never the
// checkout. Offline reads work from the last accepted tree with a
// staleness banner past the threshold; the sync mode is a durable
// identity (the root record's word against the clone's config,
// mismatch refused by name); single-machine mode banners itself and
// names backlog-local-promotion as the only exit.

import (
	"fmt"
	"strings"
	"time"
)

// Projection is one read of the accepted world.
type Projection struct {
	Tip     string
	Tree    *TreeGoals
	Banners []string
}

// StaleThreshold is how old an accepted tree may grow before the
// projection banners its staleness.
const StaleThreshold = 30 * time.Minute

// Project reads the accepted tree. With fetchFirst, the read-side
// validator runs before the read (the --fetch flag); otherwise the
// read is offline-capable and banners staleness.
func Project(e Endpoint, fetchFirst bool, now time.Time) (Projection, error) {
	if fetchFirst {
		if _, err := FetchAdvance(e); err != nil {
			return Projection{}, err
		}
	}
	tipOut, err := goalGit(e.Root, nil, "rev-parse", "--verify", "--quiet", AcceptedRef)
	if err != nil {
		return Projection{}, fmt.Errorf("no accepted tree; the first fetch or the migration bootstraps it")
	}
	tip := strings.TrimSpace(tipOut)
	tree, err := loadTree(e.Root, tip)
	if err != nil {
		return Projection{}, err
	}
	p := Projection{Tip: tip, Tree: tree}

	// The durable sync-mode identity: the root record's word against
	// the clone's config. A local-mode ledger with a remote config is
	// the forbidden promotion; a remote-mode ledger pointed at local
	// is a split-brain risk. Both refuse by name.
	if tree.Root != nil {
		recordMode := tree.Root.SyncMode
		configLocal := e.LocalMode()
		if recordMode == SyncLocal && !configLocal {
			return Projection{}, fmt.Errorf("sync-mode mismatch refused: the ledger is committed local, the config says remote %q — promotion is the backlog-local-promotion goal, not a config flip", e.Remote)
		}
		if recordMode == SyncRemote && configLocal {
			return Projection{}, fmt.Errorf("sync-mode mismatch refused: the ledger is committed remote, the config says local — a split brain is not a mode")
		}
		if recordMode == SyncLocal {
			p.Banners = append(p.Banners, "single-machine mode: multi-machine guarantees are void here; joining a fleet is the backlog-local-promotion goal")
		}
	}

	// Staleness: the accepted COMMIT's age is the tree's age.
	if ageOut, err := goalGit(e.Root, nil, "log", "-1", "--format=%ct", tip); err == nil {
		if seconds := strings.TrimSpace(ageOut); seconds != "" {
			var epoch int64
			if _, scanErr := fmt.Sscanf(seconds, "%d", &epoch); scanErr == nil {
				age := now.Sub(time.Unix(epoch, 0))
				if age > StaleThreshold {
					p.Banners = append(p.Banners, fmt.Sprintf("the accepted tree is %s old; goal list --fetch validates and advances it", age.Round(time.Minute)))
				}
			}
		}
	}
	return p, nil
}

// NextVerdict is the frontier read the dispatcher and the steward
// consume: claimed-first, then the queued frontier, then blocked.
type NextVerdict struct {
	Claimed []string // this machine's claimed goals, sorted
	Ready   []string // queued with every blocker done
	Blocked []string // queued behind an open blocker
}

// Next computes the frontier for one machine from a projection.
func Next(p Projection, machine string) NextVerdict {
	v := NextVerdict{}
	t := p.Tree
	for _, id := range sortedGoalIds(t.Live) {
		f := t.Live[id]
		switch f.State {
		case StateClaimed:
			if f.Claimed != nil && f.Claimed.Machine == machine {
				v.Claimed = append(v.Claimed, id)
			}
		case StateQueued:
			ready := true
			for _, dep := range f.Blocked {
				if depState(t, dep) != StateDone {
					ready = false
					break
				}
			}
			if ready {
				v.Ready = append(v.Ready, id)
			} else {
				v.Blocked = append(v.Blocked, id)
			}
		}
	}
	return v
}
