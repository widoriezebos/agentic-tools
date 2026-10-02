package steward

// The seat ladder (g1-s77, D-ladder and D-retry): when ready work has no seat
// the steward starts a headless seat main of its own lineage, and a dead seat
// of that lineage holding a claim is succeeded by the next one.

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

// SeatLineage is the one owner lineage of every seat main the steward starts
// on this installation, the machinery's own convention like the landing
// lane's owner lineage: a successor inherits a dead predecessor's lease and
// claim by the lease's own succession rule. It is the launch lane's
// constant, the lineage the seat launch sets in the seat's environment, so
// the two cannot drift.
const SeatLineage = launch.SeatOwnerLineage

// SeatHeld is one claim this machine holds, as the seat ladder reads it.
type SeatHeld struct {
	Goal    string
	Lineage string
	// Landing is a claim waiting to land.
	Landing bool
	// StepDue says the holder has a step to take on it: always for a working
	// claim; for a landing claim only when HolderStepsDue names one and no
	// review hold stands on the goal.
	StepDue bool
	// Wait is why a landing claim is not due.
	Wait         string
	ApprovalOpid string
}

// SeatReady is one ready goal, in the reader's order.
type SeatReady struct {
	Goal         string
	HumanWord    bool
	ApprovalOpid string
}

// SeatWorld is what the seat ladder selects from.
type SeatWorld struct {
	Held  []SeatHeld
	Ready []SeatReady
}

// SeatSelection is the goal a seat start names and what its record keeps.
type SeatSelection struct {
	Goal         string   `json:"goal"`
	Held         bool     `json:"held"`
	ApprovalOpid string   `json:"approvalOpid"`
	Ready        []string `json:"ready"`
	SeatHeld     []string `json:"seatHeld"`
}

// SeatWorldFrom reads one claimable-work snapshot and its accepted goal files
// into the seat ladder's world. A working claim whose next step waits on a
// human word, or that an open question names, is not due, as the Stop path
// reads a held goal (SOL-A-01); a ready goal either way waits for a person. A
// landing claim is held work for a successor only when HolderStepsDue names a
// step for it and no review hold stands on the goal (the gate's own reading
// of holds): LandingDue does not see a hold placed after a clear-to-land,
// while the gate refuses to land under it; and a due landing is held work
// only when the gate admits it at the goal branch's tip now (tips, by goal),
// since a word binds to the tip it was given at (SOL-A-02).
func SeatWorldFrom(work goal.ClaimableBudgetedWork, live map[string]*goal.GoalFile, settings goal.GateSettings, tips map[string]string, now time.Time) SeatWorld {
	var world SeatWorld
	for _, id := range work.Claimed {
		file, _ := work.OwnedClaim(id)
		held := SeatHeld{Goal: id, Lineage: claimLineage(file), StepDue: true, ApprovalOpid: approvalOpid(file)}
		if facts := work.GoalFacts[id]; facts.AskedOpen {
			held.StepDue, held.Wait = false, "an open question on it waits on a person's answer"
		} else if goal.NextStepNamesAPendingHumanWord(facts.NextStep) {
			held.StepDue, held.Wait = false, "its next step waits on a human word"
		}
		world.Held = append(world.Held, held)
	}
	for _, id := range work.Landing {
		file, _ := work.OwnedClaim(id)
		held := SeatHeld{Goal: id, Lineage: claimLineage(file), Landing: true, ApprovalOpid: approvalOpid(file)}
		holds := goal.HoldsOf(file)
		steps := goal.HolderStepsDue([]*goal.GoalFile{file}, settings, now)
		switch {
		case file == nil:
			held.Wait = "its record is not in this reading"
		case len(holds) > 0:
			held.Wait = "it waits under a review sitting by " + holds[0].By
		case len(steps) == 0:
			held.Wait = gateWait(file, settings, now)
		case !steps[0].Revise:
			if _, err := goal.Gate(file, tips[id], settings); err != nil {
				var refusal *goal.GateRefusal
				if errors.As(err, &refusal) {
					held.Wait = refusal.Reason
				} else {
					held.Wait = err.Error()
				}
				break
			}
			held.StepDue = true
		default:
			held.StepDue = true
		}
		world.Held = append(world.Held, held)
	}
	for _, id := range work.Claimable {
		ready := SeatReady{Goal: id, HumanWord: work.GoalFacts[id].AskedOpen || goal.NextStepNamesAPendingHumanWord(work.GoalFacts[id].NextStep)}
		ready.ApprovalOpid = approvalOpid(live[id])
		world.Ready = append(world.Ready, ready)
	}
	return world
}

func claimLineage(file *goal.GoalFile) string {
	if file == nil || file.Claimed == nil {
		return ""
	}
	return file.Claimed.Lineage
}

func approvalOpid(file *goal.GoalFile) string {
	if file == nil || file.Approved == nil {
		return ""
	}
	return file.Approved.Opid
}

// gateWait says why a landing claim with no due step waits, in the gate's
// own terms.
func gateWait(file *goal.GoalFile, settings goal.GateSettings, now time.Time) string {
	read := goal.ReadGate(file, settings, now)
	switch {
	case read.Landed:
		return "its landing is recorded"
	case read.WaitsForHuman && read.Word == "":
		return fmt.Sprintf("it waits for a person's review word (tier %d, landing.review.human-from-tier=%d)", read.Tier, settings.HumanFromTier)
	case !read.WaitsForHuman && !read.AutoLandsAt.IsZero() && !read.Eligible:
		return fmt.Sprintf("it waits for landing.review.auto-after=%s, until %s", settings.AutoAfterText, read.AutoLandsAt.UTC().Format(time.RFC3339))
	default:
		return "it waits at the landing gate"
	}
}

// PlanSeat is D-ladder's selection over one world, after every guard: held
// work of the seat lineage first, then a claim under any other lineage stops
// the start, then the first ready goal free of a human word and not capped.
// owned is the proven-death branch of owned work, where the tick calls it only
// when the seat lineage holds a claim; with no due step it names the wait.
func PlanSeat(world SeatWorld, records []SeatRecord, maxRevivals int, owned bool) (Decision, *SeatSelection) {
	verdict := VerdictIdleBacklogDead
	if owned {
		verdict = VerdictStalledDead
	}
	selection := SeatSelection{}
	var foreign, waits, working []string
	var candidate *SeatHeld
	for i := range world.Held {
		held := world.Held[i]
		if held.Lineage != SeatLineage {
			foreign = append(foreign, held.Goal)
			continue
		}
		selection.SeatHeld = append(selection.SeatHeld, held.Goal)
		if !held.StepDue {
			waits = append(waits, held.Goal+": "+held.Wait)
			if !held.Landing {
				working = append(working, held.Goal)
			}
			continue
		}
		if candidate == nil {
			candidate = &world.Held[i]
		}
	}
	for _, ready := range world.Ready {
		selection.Ready = append(selection.Ready, ready.Goal)
	}
	if candidate != nil {
		if count := SeatNoProgressCount(records, candidate.Goal, candidate.ApprovalOpid); count >= maxRevivals {
			return seatCapped(verdict, candidate.Goal, count), nil
		}
		selection.Goal, selection.Held, selection.ApprovalOpid = candidate.Goal, true, candidate.ApprovalOpid
		return Decision{verdict, ActRevive, fmt.Sprintf("this seat's main is dead holding %s; starting its successor", candidate.Goal)}, &selection
	}
	// A working claim that waits keeps the seat's one working claim: a
	// successor could take no other goal beside it (SOL-A-01).
	if len(working) > 0 {
		return Decision{verdict, ActNotify, "this seat holds " + strings.Join(working, ", ") + " and waits: " + strings.Join(waits, "; ") +
			"; no successor starts until a person answers"}, nil
	}
	waiting := ""
	if len(waits) > 0 {
		waiting = "; " + strings.Join(waits, "; ") + ", and no successor starts until a step is due"
	}
	if owned {
		return Decision{verdict, ActNotify, "the claims of this seat wait: " + strings.Join(waits, "; ") + "; no successor starts until a step is due"}, nil
	}
	if len(foreign) > 0 {
		return Decision{verdict, ActNotify, fmt.Sprintf(
			"this machine holds %s under another lineage with no live process; no seat starts beside it (goal claim --take-over is a person's act)%s",
			strings.Join(foreign, ", "), waiting)}, nil
	}
	var free, worded []string
	for _, ready := range world.Ready {
		if ready.HumanWord {
			worded = append(worded, ready.Goal)
		} else {
			free = append(free, ready.Goal)
		}
	}
	if len(free) == 0 {
		return Decision{verdict, ActNotify, fmt.Sprintf(
			"every ready goal waits on a human word: %s; no seat starts%s", strings.Join(worded, ", "), waiting)}, nil
	}
	capped, cappedCount := "", 0
	for _, ready := range world.Ready {
		if ready.HumanWord {
			continue
		}
		if count := SeatNoProgressCount(records, ready.Goal, ready.ApprovalOpid); count >= maxRevivals {
			if capped == "" {
				capped, cappedCount = ready.Goal, count
			}
			continue
		}
		selection.Goal, selection.ApprovalOpid = ready.Goal, ready.ApprovalOpid
		return Decision{verdict, ActRevive, fmt.Sprintf("ready work has no seat; starting a seat for %s", ready.Goal)}, &selection
	}
	return seatCapped(verdict, capped, cappedCount), nil
}

func seatCapped(verdict Verdict, id string, count int) Decision {
	return Decision{verdict, ActNotify, fmt.Sprintf(
		"%d seats ended without progress on %s; `metasystem goal unapprove %s --reason TEXT` then `metasystem goal approve %s` starts again, `metasystem goal pause %s --reason TEXT` parks it",
		count, id, id, id, id)}
}

// SeatNoProgressCount is D-retry's count for one goal under one approval: the
// consecutive seats named for it under that approval that ended without
// progress, newest first, up to the newest seat that made progress. A
// provider-limit ending neither counts nor resets; a start that failed counts,
// for it made no progress either.
func SeatNoProgressCount(records []SeatRecord, goalID, approvalOpid string) int {
	ordered := append([]SeatRecord(nil), records...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].StartedAt != ordered[j].StartedAt {
			return ordered[i].StartedAt > ordered[j].StartedAt
		}
		return ordered[i].LaunchID > ordered[j].LaunchID
	})
	count := 0
	for _, record := range ordered {
		switch record.Outcome {
		case SeatProgress:
			return count
		case SeatNoProgress, SeatStartFailed:
			if record.Goal == goalID && record.ApprovalOpid == approvalOpid {
				count++
			}
		}
	}
	return count
}
