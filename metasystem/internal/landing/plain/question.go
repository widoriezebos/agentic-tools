package plain

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
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
	premise, err := LaneQuestionPremise(install, q, seams)
	if err != nil || premise != "gone" {
		return false, nil
	}
	_, err = channel.Withdraw(install, q.ID, "the checkout or main commit premise is no longer current", nil, channel.DestinationConfig{})
	return err == nil, err
}

// LaneQuestionPremise judges only the explicit evidence carried by the question.
func LaneQuestionPremise(install string, q channel.Question, seams ProveSeams) (string, error) {
	if channel.LaneStopCommand(q) != "" {
		satisfied, err := PolicyQuestionSatisfied(install, q)
		if satisfied {
			return "gone", err
		}
		return "current", err
	}
	if q.Lineage != lane.AgentLineage || channel.LaneStopCommand(q) != "" {
		return "unknown", nil
	}
	facts := strings.Join(q.Facts, "\n")
	premise := "unknown"
	if strings.Contains(strings.ToLower(facts), "checkout is not on main") {
		branch, err := seams.git(install, "symbolic-ref", "--quiet", "--short", "HEAD")
		if err != nil {
			return "unknown", err
		}
		if branch == "main" {
			return "gone", nil
		}
		premise = "current"
	}
	if match := mainQuestionCommit.FindStringSubmatch(facts); len(match) > 0 {
		main, err := checkoutGit(install, seams).main()
		if err != nil {
			return "unknown", err
		}
		if main == match[1] {
			return "current", nil
		}
		if main != match[1] {
			contained, err := checkoutGit(install, seams).contains(main, match[1])
			if err != nil {
				return "unknown", err
			}
			if contained {
				return "gone", nil
			}
		}
	}
	return premise, nil
}
