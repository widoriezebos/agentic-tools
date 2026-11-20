package plain

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/state"
)

// LaneQuestionHeadline names the waiting answer and the person's remedies.
func LaneQuestionHeadline(q channel.Question) string {
	return state.Describe(state.Record{Questions: []state.Question{{ID: q.ID, Since: state.Since{At: q.OpenedAt}}}}, state.RenderInputs{Questions: map[string]state.QuestionInput{q.ID: {FirstLine: firstQuestionLine(q), Policy: channel.LaneStopCommand(q) != ""}}}).Questions[0]
}

var mainQuestionCommit = regexp.MustCompile(`(?i)main is at commit ([0-9a-f]{40})(?:[^0-9a-f]|$)`)

// WithdrawStaleLaneQuestion releases only the landing agent's explicit Git
// premises. Policy requests are closed by their recorded effects instead.
func WithdrawStaleLaneQuestion(install string, q channel.Question, seams ProveSeams) (bool, error) {
	if q.Lineage != lane.AgentLineage || channel.LaneStopCommand(q) != "" {
		return false, nil
	}
	facts, err := laneQuestionFacts(install, q, seams)
	if due, _ := state.LifecycleDue("question", state.LifecycleFacts{Premise: state.QuestionPremise(facts).Word}, seams.now()); err != nil || !due {
		return false, nil
	}
	err = withLock(install, func() error {
		return transitionWrite(install, "questions", "withdraw", seams.now(), "", func() error {
			_, err := channel.Withdraw(install, q.ID, "the checkout or main commit premise is no longer current", nil, channel.DestinationConfig{})
			return err
		}, seams)
	})
	return err == nil, err
}

func LaneQuestionPremise(install string, q channel.Question, seams ProveSeams) (string, error) {
	facts, err := laneQuestionFacts(install, q, seams)
	if err != nil {
		facts.Unreadable = err.Error()
	}
	return state.QuestionPremise(facts).Word, err
}

func laneQuestionFacts(install string, q channel.Question, seams ProveSeams) (f state.PremiseFacts, err error) {
	if len(q.Facts) > 1 && strings.HasPrefix(q.Facts[1], "lane stop: ") {
		var stop Stop
		if err := json.Unmarshal([]byte(strings.TrimPrefix(q.Facts[1], "lane stop: ")), &stop); err != nil {
			return f, err
		}
		stops, err := OpenStops(install)
		f.Policy, f.Satisfied = true, !slices.ContainsFunc(stops, func(s Stop) bool { return stopKey(s) == stopKey(stop) })
		return f, err
	}
	f.Policy, f.LaneAgent = channel.LaneStopCommand(q) != "", q.Lineage == lane.AgentLineage
	if f.Policy {
		f.Satisfied, err = PolicyQuestionSatisfied(install, q)
		return f, err
	}
	if !f.LaneAgent {
		return f, nil
	}
	facts := strings.Join(q.Facts, "\n")
	f.CheckoutOffMain = strings.Contains(strings.ToLower(facts), "checkout is not on main")
	if f.CheckoutOffMain {
		f.Branch, err = seams.git(install, "symbolic-ref", "--quiet", "--short", "HEAD")
		if err != nil || f.Branch == "main" {
			return f, err
		}
	}
	if match := mainQuestionCommit.FindStringSubmatch(facts); len(match) > 0 {
		f.ExpectedMain = match[1]
		f.Main, err = checkoutGit(install, seams).main()
		if err != nil {
			return f, err
		}
		if f.Main != f.ExpectedMain {
			f.Contains, err = checkoutGit(install, seams).contains(f.Main, f.ExpectedMain)
		}
	}
	return f, err
}

func firstQuestionLine(q channel.Question) string {
	if len(q.Facts) > 0 {
		return q.Facts[0]
	}
	return ""
}

// HoldingLaneQuestions selects the questions that hold this computer's keeper.
func HoldingLaneQuestions(module string, open []channel.Question, machineFn func(string) (string, error)) ([]channel.Question, error) {

	if machineFn == nil {
		machineFn = goal.ResolveMachine
	}

	var lane []channel.Question
	for _, q := range open {
		if len(q.Facts) > 1 && strings.HasPrefix(q.Facts[1], "lane steward: ") {
			continue
		}
		if satisfied, err := PolicyQuestionSatisfied(module, q); err != nil {
			return nil, err
		} else if satisfied {
			continue
		}
		if q.Goal == "" && q.About == "lane" && (q.State == "open" || q.State != "closed" && channel.LaneStopCommand(q) != "") {
			lane = append(lane, q)
		}
	}
	if len(lane) == 0 {
		return nil, nil
	}
	machine, err := machineFn(module)
	if err != nil {
		return nil, fmt.Errorf("a question about the lane is open and this computer's name cannot be read: %w", err)
	}
	return slices.DeleteFunc(lane, func(q channel.Question) bool { return q.Machine != machine }), nil
}

func stopHasQuestion(stop Stop, questions []channel.Question) bool {
	for _, q := range questions {
		if len(q.Facts) < 2 {
			continue
		}
		data, ok := strings.CutPrefix(q.Facts[1], "lane stop: ")
		var bound Stop
		if ok && json.Unmarshal([]byte(data), &bound) == nil && stopKey(bound) == stopKey(stop) {
			return true
		}
	}
	return false
}
