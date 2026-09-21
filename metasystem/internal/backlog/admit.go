package backlog

import (
	"sort"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// Admission is the claim gate's answer for every approved live goal at one
// observation. Whether a goal is ready is not a shape this projection can
// derive from the record: the gate reads the tier-box configuration, norm
// coverage, and the approval record itself, and can refuse work that looks
// eligible. So the frontier is asked, never imitated.
type Admission struct {
	// Answered is false when the frontier could not be computed at all. The
	// engine treats uncertainty about the admission law as uncertainty about
	// every candidate, and so does this: no goal is called ready, blocked, or
	// refused on a guess.
	Answered bool
	// Message carries the engine's words when Answered is false.
	Message string

	Ready    map[string]bool
	Blocked  map[string]bool
	Awaiting map[string]bool
	// Refused maps a goal id to the gate's own cause.
	Refused map[string]string
}

// Admit composes the frontier for every approved live goal. The frontier is
// computed per machine, because a goal pinned to one machine is skipped by
// every other machine's read; asking once without a machine and once for each
// machine the tree pins to places all of them, and the gate's verdict does not
// depend on which machine asked.
func Admit(p goal.Projection) Admission {
	if p.Tree == nil {
		return Admission{}
	}
	admission := Admission{
		Answered: true,
		Ready:    map[string]bool{},
		Blocked:  map[string]bool{},
		Awaiting: map[string]bool{},
		Refused:  map[string]string{},
	}
	for _, machine := range readers(p.Tree) {
		verdict, err := goal.Next(p, machine)
		if err != nil {
			return Admission{Message: err.Error()}
		}
		for _, id := range verdict.Ready {
			if placedHere(p.Tree, id, machine) {
				admission.Ready[id] = true
			}
		}
		for _, id := range verdict.Blocked {
			if placedHere(p.Tree, id, machine) {
				admission.Blocked[id] = true
			}
		}
		for _, id := range verdict.Awaiting {
			if placedHere(p.Tree, id, machine) {
				admission.Awaiting[id] = true
			}
		}
		for _, refusal := range verdict.Refused {
			if placedHere(p.Tree, refusal.GoalID, machine) {
				admission.Refused[refusal.GoalID] = refusal.Cause
			}
		}
	}
	return admission
}

// readers lists the machines whose frontier must be read to place every
// approved goal once: the unpinned reader, then each machine some approved
// goal is pinned to, in a stable order.
func readers(tree *goal.TreeGoals) []string {
	pinned := map[string]bool{}
	for _, file := range tree.Live {
		if file.State == goal.StateApproved && file.Pinned != "" {
			pinned[file.Pinned] = true
		}
	}
	machines := make([]string, 0, len(pinned)+1)
	for machine := range pinned {
		machines = append(machines, machine)
	}
	sort.Strings(machines)
	return append([]string{""}, machines...)
}

// placedHere reports whether this reader's frontier is the one that speaks
// for the goal: an approved live goal is placed by the reader whose machine
// is its pin, so a goal pinned elsewhere is not taken from the unpinned read.
func placedHere(tree *goal.TreeGoals, id, machine string) bool {
	file := tree.Live[id]
	return file != nil && file.State == goal.StateApproved && file.Pinned == machine
}
