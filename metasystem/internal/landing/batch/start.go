package batch

import (
	"fmt"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

// batchStartRule is the timer: a batch starts at the max wait past its
// oldest join, or on a quiet host with two units. After D14 it decides only
// when the host board cannot be read.
func batchStartRule(lockFree bool, joined int, oldestJoinedAt time.Time, settings config.BatchLanding, sample proofrun.LoadSample, admission proofrun.AdmissionCap) (start bool, quietWindow string, engineCapAllows bool) {
	engineCapAllows = sample.OverlapKnown && !admission.Refuses(sample, false, true)
	if !lockFree || joined == 0 || !engineCapAllows {
		return
	}
	if settings.MaxWaitElapsed(oldestJoinedAt) {
		return true, "expired", engineCapAllows
	} else if joined >= 2 && !sample.Loaded() && sample.OverlappingHost == 0 {
		return true, "quiet", engineCapAllows
	}
	return
}

// The windows a knowledge-driven start records beside the fallback's
// expired and quiet.
const (
	WindowRedOnMain   = "red-on-main"
	WindowNothingNear = "nothing-near"
	WindowWaited      = "waited"
)

// Waited is one unit a batch waits for.
type Waited struct {
	Goal       string       `json:"goal"`
	Seat       string       `json:"seat"`
	Stage      board.Stage  `json:"stage"`
	Round      *board.Round `json:"round,omitempty"`
	Proof      *board.Proof `json:"proof,omitempty"`
	ExpectedAt time.Time    `json:"expectedAt"`
}

// WaitState is why an open batch has not started: the units it waits for,
// or the fallback's max wait when the board cannot be read. Reason is the
// identity of the wait (who, where, at what stage); a decision with the
// same reason writes nothing.
type WaitState struct {
	Reason    string        `json:"reason"`
	For       []Waited      `json:"for,omitempty"`
	ProofCost time.Duration `json:"proofCost"`
	Since     time.Time     `json:"since"`
	Basis     string        `json:"basis"`
	// Fallback names the unreadable board's cause when the timer decides.
	Fallback string    `json:"fallback,omitempty"`
	Until    time.Time `json:"until,omitempty"`
}

// Decision is one start decision: start now with a reason, or wait.
type Decision struct {
	Start  bool
	Window string
	Reason string
	Wait   *WaitState
}

// startInputs are every value a start is decided from.
type startInputs struct {
	record    Record
	joined    int
	oldest    time.Time
	facts     PipelineFacts
	open      []OpenEntry
	settings  config.BatchLanding
	sample    proofrun.LoadSample
	admission proofrun.AdmissionCap
	now       time.Time
	location  *time.Location
}

// pipelineStartRule decides a batch's start from what is underway on every
// seat of this host (D14, R22), inside the guards of the timer rule: a
// joined unit that fixes an open trunk red starts at once; with a readable
// board, a batch starts at once when nothing is within reach and otherwise
// waits for exactly the reachable units; only an unreadable board falls
// back to the max wait. A pure function of its inputs.
func pipelineStartRule(in startInputs) Decision {
	engineCapAllows := in.sample.OverlapKnown && !in.admission.Refuses(in.sample, false, true)
	if in.joined == 0 || !engineCapAllows {
		return Decision{}
	}
	joined := joinedGoals(in.record)
	for _, entry := range in.open {
		if entry.Class == ClassTrunkRed && entry.FixGoal != "" && joined[entry.FixGoal] {
			return Decision{Start: true, Window: WindowRedOnMain, Reason: fmt.Sprintf("red on main (%s fixes incident %s)", entry.FixGoal, entry.ID)}
		}
	}
	if !in.facts.Readable {
		start, window, _ := batchStartRule(true, in.joined, in.oldest, in.settings, in.sample, in.admission)
		fallback := "board unreadable (" + in.facts.Reason + ")"
		if start && window == "quiet" {
			return Decision{Start: true, Window: window, Reason: fmt.Sprintf("%s; the host is quiet with %d units joined", fallback, in.joined)}
		}
		if start {
			return Decision{Start: true, Window: window, Reason: fmt.Sprintf("%s; waited the max wait (%s)", fallback, minutes(in.settings.MaxWait))}
		}
		return Decision{Wait: &WaitState{Reason: fallback, Fallback: in.facts.Reason, Since: in.oldest, Until: in.oldest.Add(in.settings.MaxWait)}}
	}
	var reachable []Underway
	for _, unit := range in.facts.Reachable(in.oldest) {
		if !joined[unit.Goal] {
			reachable = append(reachable, unit)
		}
	}
	if len(reachable) > 0 {
		wait := &WaitState{ProofCost: in.facts.ProofCost, Since: in.oldest, Basis: in.facts.Basis}
		var identity []string
		for _, unit := range reachable {
			wait.For = append(wait.For, Waited{Goal: unit.Goal, Seat: unit.Seat, Stage: unit.Stage, Round: unit.Round, Proof: unit.Proof, ExpectedAt: unit.ExpectedReady})
			identity = append(identity, unit.Goal+" on "+unit.Seat+" at "+string(unit.Stage))
		}
		wait.Reason = "waits for " + strings.Join(identity, ", ")
		return Decision{Wait: wait}
	}
	if previous := in.record.Wait; previous != nil && len(previous.For) > 0 {
		return Decision{Start: true, Window: WindowWaited, Reason: waitEnded(in, *previous, joined)}
	}
	return Decision{Start: true, Window: WindowNothingNear, Reason: nothingNear(in, joined, "nothing within reach")}
}

// waitEnded names why a wait that named units ended: the last of them
// joined, or what happened to those that did not.
func waitEnded(in startInputs, previous WaitState, joined map[string]bool) string {
	var gone []string
	last, lastAt := "", time.Time{}
	joinedAt := joinTimes(in.record)
	for _, waited := range previous.For {
		if joined[waited.Goal] {
			if at := joinedAt[waited.Goal]; !at.Before(lastAt) {
				last, lastAt = waited.Goal+" on "+waited.Seat, at
			}
			continue
		}
		gone = append(gone, whyNotNear(in, waited.Goal, waited.Seat))
	}
	if len(gone) == 0 {
		return last + " joined, the last unit waited for"
	}
	return strings.Join(gone, "; ") + "; nothing else within reach"
}

// nothingNear names the nearest unit and why it is not waited for.
func nothingNear(in startInputs, joined map[string]bool, lead string) string {
	var named []string
	for _, unit := range in.facts.Underway {
		if !joined[unit.Goal] {
			named = append(named, fmt.Sprintf("%s on %s expected in ~%s, later than a separate proof (~%s)", unit.Goal, unit.Seat, minutes(unit.ExpectedReady.Sub(in.now)), minutes(in.facts.ProofCost)))
			break
		}
	}
	for _, unknown := range in.facts.Unknown {
		if !joined[unknown.Goal] {
			named = append(named, notNearLine(in, unknown))
		}
	}
	if len(named) == 0 {
		return lead
	}
	return lead + "; " + strings.Join(named, "; ")
}

func whyNotNear(in startInputs, goal, seat string) string {
	for _, unknown := range in.facts.Unknown {
		if unknown.Goal == goal {
			return notNearLine(in, unknown)
		}
	}
	for _, unit := range in.facts.Underway {
		if unit.Goal == goal {
			return fmt.Sprintf("%s on %s now expected in ~%s, later than a separate proof (~%s)", goal, unit.Seat, minutes(unit.ExpectedReady.Sub(in.now)), minutes(in.facts.ProofCost))
		}
	}
	return goal + " on " + seat + " left the pipeline without joining"
}

func notNearLine(in startInputs, unknown NotNear) string {
	if unknown.Reason == board.ReasonStalled {
		return fmt.Sprintf("%s on %s stalled since %s", unknown.Goal, unknown.Seat, localClock(unknown.At, in.location))
	}
	return fmt.Sprintf("%s on %s unknown (%s)", unknown.Goal, unknown.Seat, unknown.Reason)
}

func joinedGoals(record Record) map[string]bool {
	joined := map[string]bool{}
	for _, unit := range record.Units {
		if unit.State == UnitJoined {
			joined[unit.GoalID] = true
		}
	}
	return joined
}

func joinTimes(record Record) map[string]time.Time {
	times := map[string]time.Time{}
	for _, entry := range record.History {
		fields := strings.Fields(entry.Detail)
		if entry.Verb != "join" || len(fields) == 0 {
			continue
		}
		if at, err := time.Parse(time.RFC3339Nano, entry.At); err == nil {
			times[fields[0]] = at
		}
	}
	return times
}

func minutes(d time.Duration) string {
	return fmt.Sprintf("%d min", max(int(d.Round(time.Minute)/time.Minute), 0))
}

func localClock(at time.Time, location *time.Location) string {
	if location == nil {
		location = time.Local
	}
	return at.In(location).Format("15:04")
}
