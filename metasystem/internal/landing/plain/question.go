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
	sentence := ""
	if len(q.Facts) > 0 {
		sentence = strings.Join(strings.Fields(q.Facts[0]), " ")
		if i := strings.IndexAny(sentence, ".?!"); i >= 0 {
			sentence = sentence[:i+1]
		}
	}
	return fmt.Sprintf("Waiting for a person's answer to question %s since %s: %s; run: metasystem question show channel:%s; when stale: metasystem question withdraw channel:%s --reason TEXT", q.ID, q.OpenedAt.In(time.Local).Format("2006-01-02 15:04 MST"), sentence, q.ID, q.ID)
}

var mainQuestionCommit = regexp.MustCompile(`(?i)main is at commit ([0-9a-f]{40})(?:[^0-9a-f]|$)`)

// WithdrawStaleLaneQuestion releases only the landing agent's explicit Git
// premises. Policy requests are closed by their recorded effects instead.
func WithdrawStaleLaneQuestion(install string, q channel.Question, seams ProveSeams) (bool, error) {
	if q.Lineage != lane.AgentLineage || channel.LaneStopCommand(q) != "" {
		return false, nil
	}
	facts := strings.Join(q.Facts, "\n")
	stale := false
	if strings.Contains(strings.ToLower(facts), "checkout is not on main") {
		branch, err := seams.git(install, "symbolic-ref", "--quiet", "--short", "HEAD")
		stale = err == nil && branch == "main"
	}
	if match := mainQuestionCommit.FindStringSubmatch(facts); len(match) > 0 {
		main, err := checkoutGit(install, seams).main()
		if err == nil && main != match[1] {
			contained, err := checkoutGit(install, seams).contains(main, match[1])
			stale = stale || err == nil && contained
		}
	}
	if !stale {
		return false, nil
	}
	_, err := channel.Withdraw(install, q.ID, "the checkout or main commit premise is no longer current", nil, channel.DestinationConfig{})
	return err == nil, err
}
