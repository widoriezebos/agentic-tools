package main

// The silent-holder lines `goal next` prints.
//
// It is a flag and never an act. `NextVerdict`, what goes to standard output
// and the exit code do not change: these lines go to standard error, one per
// claim of another machine that has gone quiet, so an agent reading the
// orientation line reads the same orientation line it always did and a human
// watching the terminal learns why a goal nobody is moving is still held.
//
// Nothing here fetches. The tree is the one the verb already projected, and
// the presence copy is whatever the steward tick last brought into this
// clone: a verb that reached the network to say this would be a verb that
// hung when the presence remote did.

import (
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/fleet"
)

// silentHolderLines reads this clone's presence copy and composes the lines.
func silentHolderLines(root string, tree *goal.TreeGoals, machine string, now time.Time) []string {
	if tree == nil || len(claimsOfOthers(tree, machine)) == 0 {
		return nil
	}
	transport, err := seat.NewGit(root)
	if err != nil {
		return nil
	}
	copied, err := transport.Read(seat.TickNamespace)
	if err != nil {
		return nil
	}
	previous, _, err := seat.LoadStandings(root)
	if err != nil {
		previous = seat.StandingsState{Machines: map[string]seat.Observation{}}
	}
	return silentHolders(tree, machine, copied, previous.Machines, now, seatPresenceWindow(root))
}

// claimsOfOthers is every live goal claimed by a machine that is not this one.
func claimsOfOthers(tree *goal.TreeGoals, machine string) map[string][]string {
	claims := map[string][]string{}
	for id, file := range tree.Live {
		if file == nil || file.Claimed == nil || file.Claimed.Machine == "" {
			continue
		}
		if file.Claimed.Machine == machine {
			continue
		}
		claims[file.Claimed.Machine] = append(claims[file.Claimed.Machine], id)
	}
	return claims
}

// silentHolders is the lines themselves, from facts a caller read.
//
// The remedy is named conditionally, because the two are not interchangeable:
// `goal steal` reassigns a claim and refuses a fenced one, and `goal resume`
// lifts a breach fence and keeps the owner where it is.
//
// The flag words come from the interface's own composer rather than from
// seat.SilentHolder, so that this line, the board's card line and the Fleet
// page say one thing about one machine; that is also why this file reaches
// into the interface's fleet package for two functions and nothing else.
func silentHolders(
	tree *goal.TreeGoals, machine string,
	copied seat.Copy, previous map[string]seat.Observation,
	now time.Time, window time.Duration,
) []string {
	standings := seat.Fleet(seat.FleetInput{
		This: machine, Copy: copied, Claims: claimsOfOthers(tree, machine),
		Previous: previous, Now: now, Window: window,
	})
	lines := []string{}
	for _, standing := range standings {
		if standing.Standing == seat.Reachable || len(standing.Holds) == 0 {
			continue
		}
		flag := fleet.Flag(standing, fleet.Since(previous, standing), now)
		if flag == "" {
			continue
		}
		for _, id := range standing.Holds {
			lines = append(lines, "goal "+id+" is "+flag+"; "+remedyFor(tree.Live[id]))
		}
	}
	return lines
}

// remedyFor names the act that exists for this goal, and only that one.
func remedyFor(file *goal.GoalFile) string {
	if file.IsFencedClaim() {
		return "a human lifts the fence with goal resume"
	}
	return "a human reassigns it with goal steal"
}
