package steward

import (
	"fmt"
	"slices"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/outage"
)

// SeatWorkSnapshot retains only milestones of this goal's work. Branch tips,
// messages and handoff notes remain evidence for other readers, not this cap.
type SeatWorkSnapshot struct {
	Marks []string `json:"marks"`
}

func seatWorkSnapshot(root, id string, d seatDependencies) (*SeatWorkSnapshot, error) {
	if d.Units == nil {
		return nil, fmt.Errorf("unit reader unavailable")
	}
	units, err := d.Units(root, id)
	if err != nil {
		return nil, err
	}
	result := &SeatWorkSnapshot{Marks: []string{}}
	for _, unit := range units {
		for _, mark := range unit.Advances {
			result.Marks = append(result.Marks, unit.Unit+":"+mark)
		}
	}
	questions, bad := channel.WalkQuestions(root)
	if len(bad) > 0 {
		return nil, fmt.Errorf("goal question records are unreadable")
	}
	for _, q := range questions {
		if q.Goal == id && q.State == "closed" && q.Answer != nil && q.Answer.Phase == "closed" {
			result.Marks = append(result.Marks, "answered:"+q.ID)
		}
	}
	p, err := d.Project(root, d.Now())
	if err != nil {
		return nil, err
	}
	if p.Tree != nil {
		file := p.Tree.Live[id]
		if file == nil {
			file = p.Tree.Done[id]
		}
		if file != nil {
			for _, h := range file.History {
				if h.Verb == goal.LandedVerb {
					result.Marks = append(result.Marks, "landed:"+h.Opid)
				}
			}
		}
	}
	slices.Sort(result.Marks)
	result.Marks = slices.Compact(result.Marks)
	return result, nil
}

func seatGoalProgress(root string, record SeatRecord, d seatDependencies) (bool, string) {
	if record.WorkAtStart == nil {
		return false, "no work baseline was retained; no goal progress proved"
	}
	current, err := seatWorkSnapshot(root, record.Goal, d)
	if err != nil {
		return false, "goal work could not be read; no progress proved"
	}
	for _, mark := range current.Marks {
		if !slices.Contains(record.WorkAtStart.Marks, mark) {
			return true, "goal work advanced: " + mark
		}
	}
	return false, "no advance of the goal's work record"
}

// cleanSeatEnd reads durable continuation evidence each time admission is
// checked. A successful exit never manufactures progress or erases history.
// The caller has already selected a due, held claim under the current approval.
func cleanSeatEnd(root string, records []SeatRecord, selected SeatSelection, d seatDependencies) (bool, string, error) {
	if selected.Kind != "" || !selected.Held || len(records) == 0 {
		return false, "", nil
	}
	last := records[len(records)-1]
	if last.Goal != selected.Goal || last.ApprovalOpid != selected.ApprovalOpid || last.Kind != "" {
		return false, "", nil
	}
	if last.LaunchState != "completed" || last.Outcome == SeatProviderLimit || last.Outcome == SeatStartFailed {
		return false, "", nil
	}
	state, err := d.Launcher.SeatLaunch(last.LaunchID)
	if err != nil {
		return false, "", err
	}
	if !state.Found || !state.Terminal || state.State != "completed" {
		return false, "", fmt.Errorf("seat launch outcome is not a confirmed completed end")
	}
	intents, err := readLiveHandoffIntents(root)
	if err != nil {
		return false, "", err
	}
	for _, intent := range intents {
		binding := intent.Handoff
		if intent.Goal != selected.Goal || binding == nil || state.Child == nil || state.Child.Pid == 0 || binding.Predecessor != *state.Child || binding.RecordedAt.Before(seatTime(last.StartedAt)) {
			continue
		}
		if _, err := verifyBoundHandoffState(root, intent.Nonce, intent.Goal, *binding); err != nil {
			return false, "", err
		}
		return true, "clean; successor starts at the next decision", nil
	}
	if d.Units == nil {
		return false, "", fmt.Errorf("unit reader unavailable")
	}
	units, err := d.Units(root, selected.Goal)
	if err != nil {
		return false, "", err
	}
	continuable := len(units) > 0
	for _, unit := range units {
		continuable = continuable && unit.Continuable
	}
	if continuable {
		return true, "clean; successor starts at the next decision", nil
	}
	return false, "ended completed without a handoff or continuable state", nil
}

// completedSeatRevival reports the same automatic decision without clearing its
// evidence. A person's ordinary close remains available for abnormal endings.
func completedSeatRevival(root string, d seatDependencies) (error, bool) {
	state := reapSeatLaunches(root, d, d.Now())
	if state.Err != nil {
		return state.Err, true
	}
	last := state.Records[len(state.Records)-1]
	clean, _, readErr := cleanSeatEnd(root, state.Records, SeatSelection{Goal: last.Goal, Held: true, ApprovalOpid: last.ApprovalOpid}, d)
	if readErr != nil {
		return readErr, true
	}
	if !clean {
		return nil, false
	}
	p, err := d.Project(root, d.Now())
	if err != nil {
		return err, true
	}
	machine, err := d.Machine(root)
	if err != nil {
		return err, true
	}
	work, err := goal.ClaimableWorkFromProjection(p, machine, d.Prober)
	if err != nil {
		return err, true
	}
	workers, err := d.Census.Workers(root)
	if err != nil {
		return err, true
	}
	kind, _, _ := classifySharedBacklog(work)
	cfg := (TickConfig{Now: d.Now(), ProviderHome: d.ProviderHome}).withDefaults()
	provider := func(runtime string, err error) (outage.Mark, bool) {
		return standingProviderOutage(runtime, d.Now(), nil, d.ProviderHome)
	}
	decision, selection, _ := seatDecision(root, cfg, kind, work, workers, provider, d, state)
	if selection != nil {
		clean, _, err := cleanSeatEnd(root, state.Records, *selection, d)
		if err != nil {
			return err, true
		}
		if clean {
			return SeatRevivalClosed{Launch: last.LaunchID, Goal: last.Goal, Automatic: true}, true
		}
	}
	if decision.Action != ActRevive && decision.Reason != "" {
		return SeatRevivalClosed{Launch: last.LaunchID, Goal: last.Goal, Wait: decision.Reason}, true
	}
	return nil, false
}
