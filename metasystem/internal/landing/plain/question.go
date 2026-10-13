package plain

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/state"
)

// LaneQuestionHeadline names the waiting answer and the person's remedies.
func LaneQuestionHeadline(q channel.Question) string {
	waiting := fmt.Sprintf("Waiting for a person's answer to question %s since %s", q.ID, q.OpenedAt.In(time.Local).Format("2006-01-02 15:04 MST"))
	if channel.LaneStopCommand(q) != "" {
		return waiting + "; " + q.Facts[0]
	}
	fact := ""
	if len(q.Facts) > 0 {
		fact = q.Facts[0]
		if at := strings.IndexAny(fact, ".?!"); at >= 0 {
			fact = fact[:at+1]
		}
		fact += "; "
	}
	return waiting + "; " + fact + "metasystem question show channel:" + q.ID + "; withdraw: metasystem question withdraw channel:" + q.ID + " --reason TEXT"
}

var mainQuestionCommit = regexp.MustCompile(`(?i)main is at commit ([0-9a-f]{40})(?:[^0-9a-f]|$)`)

// WithdrawStaleLaneQuestion releases only the landing agent's explicit Git
// premises. Policy requests are closed by their recorded effects instead.
func WithdrawStaleLaneQuestion(install string, q channel.Question, seams ProveSeams) (bool, error) {
	if q.Lineage != lane.AgentLineage || channel.LaneStopCommand(q) != "" {
		return false, nil
	}
	facts, err := laneQuestionFacts(install, q, seams)
	if err != nil || state.QuestionPremise(facts).Word != state.Gone {
		return false, nil
	}
	_, err = channel.Withdraw(install, q.ID, "the checkout or main commit premise is no longer current", nil, channel.DestinationConfig{})
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
