package goal

// The canonical projection: every read — list, next, the
// open-work verdict — comes from the ACCEPTED ref's tree, never the
// checkout. Offline reads work from the last accepted tree with a
// staleness banner past the threshold; the sync mode is a durable
// identity (the root record's word against the clone's config,
// mismatch refused by name); single-machine mode banners itself and
// names backlog-local-promotion as the only exit.

import (
	"fmt"
	"strconv"
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

	// Appetite breaches are COMPUTED at read time from ledger data
	// alone (the claim's At stamp and the Appetite: token opening the
	// next step), so every machine's read sees the same escalation
	// with no extra writes — the steward's covenant tick and every
	// coordinator's goal next surface identical banners.
	for _, id := range sortedGoalIds(tree.Live) {
		f := tree.Live[id]
		if f.State != StateClaimed || f.Claimed == nil {
			continue
		}
		appetite, declared := ParseAppetite(f.NextStep)
		if !declared {
			continue
		}
		claimedAt, tErr := time.Parse(time.RFC3339, f.Claimed.At)
		if tErr != nil {
			continue
		}
		if age := now.Sub(claimedAt); age > appetite {
			p.Banners = append(p.Banners, fmt.Sprintf(
				"APPETITE BREACH: %s claimed %s ago against an appetite of %s (%s+%s) — the covenant says pause it and raise it with Wido",
				id, age.Round(time.Minute), appetite, f.Claimed.Machine, f.Claimed.Lineage))
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

// ParseAppetite reads the appetite convention: the next step OPENS
// with "Appetite: <duration>" where the duration's first token is
// machine-comparable — 4h, 1d (a day is eight working hours), 30m.
// Prose after the token is welcome; prose INSTEAD of a token means
// no enforceable appetite is declared.
func ParseAppetite(nextStep string) (time.Duration, bool) {
	const prefix = "Appetite:"
	trimmed := strings.TrimSpace(nextStep)
	if !strings.HasPrefix(trimmed, prefix) {
		return 0, false
	}
	rest := strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
	token := rest
	if sp := strings.IndexAny(rest, " \t.,;"); sp >= 0 {
		token = rest[:sp]
	}
	if len(token) < 2 {
		return 0, false
	}
	unit := token[len(token)-1]
	value := token[:len(token)-1]
	n, err := strconv.Atoi(value)
	if err != nil || n <= 0 {
		return 0, false
	}
	switch unit {
	case 'm':
		return time.Duration(n) * time.Minute, true
	case 'h':
		return time.Duration(n) * time.Hour, true
	case 'd':
		return time.Duration(n) * 8 * time.Hour, true
	}
	return 0, false
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
	// An arc claims as one unit, so one member pinned elsewhere makes
	// EVERY member unclaimable on this machine — the whole arc leaves
	// this machine's frontier, not just the pinned member.
	foreignPinnedArc := map[string]bool{}
	for _, f := range t.Live {
		if f.Arc != "" && f.Pinned != "" && f.Pinned != machine {
			foreignPinnedArc[f.Arc] = true
		}
	}
	for _, id := range sortedGoalIds(t.Live) {
		f := t.Live[id]
		switch f.State {
		case StateClaimed:
			if f.Claimed != nil && f.Claimed.Machine == machine {
				v.Claimed = append(v.Claimed, id)
			}
		case StateQueued:
			// A goal pinned to another machine — or any member of an
			// arc with such a member — is invisible to this machine's
			// frontier: it can never claim it, so reporting it ready
			// would hide genuinely claimable work behind it.
			if f.Pinned != "" && f.Pinned != machine {
				continue
			}
			if f.Arc != "" && foreignPinnedArc[f.Arc] {
				continue
			}
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
