package goal

import (
	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
)

// PeerOwnership is the one read of the goals' ownership that peer messages
// are delivered by (batch-lane design D14-r3, R26; the read's F-3): the
// accepted tip is resolved with one rev-parse, and the projection is made
// once per tip and shared by every seat of this host through a cache on the
// host board; a missing or unreadable cache is recomputed.
func PeerOwnership(root string) (board.Ownership, error) {
	home, err := board.Home()
	if err != nil {
		home = ""
	}
	return peerOwnership(root, home, acceptedTipForGates, projectPeerOwnership)
}

func peerOwnership(root, home string, tipOf func(string) (string, bool, error), project func(root, tip string) (board.Ownership, error)) (board.Ownership, error) {
	tip, resolved, err := tipOf(root)
	if err != nil {
		return board.Ownership{}, err
	}
	if !resolved {
		return board.Ownership{Live: map[string]string{}, Concluded: map[string]string{}}, nil
	}
	if ownership, ok := board.ReadOwnershipCache(home, tip); ok {
		return ownership, nil
	}
	ownership, err := project(root, tip)
	if err != nil {
		return board.Ownership{}, err
	}
	_ = board.WriteOwnershipCache(home, tip, ownership)
	return ownership, nil
}

// projectPeerOwnership reads the ownership at one accepted tip: each live
// goal and its holder, and each done or abandoned goal with its fact.
func projectPeerOwnership(root, tip string) (board.Ownership, error) {
	ownership := board.Ownership{Live: map[string]string{}, Concluded: map[string]string{}}
	hasLedger, err := tipCarriesLedger(Endpoint{Root: root}, tip)
	if err != nil || !hasLedger {
		return ownership, err
	}
	projection, err := ProjectAt(root, tip)
	if err != nil || projection.Tree == nil {
		return ownership, err
	}
	for id, file := range projection.Tree.Live {
		holder := ""
		if file != nil && file.Claimed != nil {
			holder = file.Claimed.Machine
		}
		ownership.Live[id] = holder
	}
	for state, files := range map[string]map[string]*GoalFile{"done": projection.Tree.Done, "abandoned": projection.Tree.Abandoned} {
		for id, file := range files {
			ownership.Concluded[id] = state + lastHistoryDay(file)
		}
	}
	return ownership, nil
}

// lastHistoryDay is " on YYYY-MM-DD" of the goal's last history line.
func lastHistoryDay(file *GoalFile) string {
	if file == nil || len(file.History) == 0 {
		return ""
	}
	at := file.History[len(file.History)-1].At
	if len(at) < 10 {
		return ""
	}
	return " on " + at[:10]
}
