package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/textui"
)

// answeredGroup is the questions that one refusal made agents ask.
type answeredGroup struct {
	Refusal        string `json:"refusal"`
	Count          int    `json:"count"`
	Answered       int    `json:"answered"`
	Withdrawn      int    `json:"withdrawn"`
	MedianToAnswer string `json:"medianToAnswer,omitempty"`
	waits          []time.Duration
}

// questionRefusal is the refusal line of a question: the first fact after
// the question itself, or the question when it states no fact.
func questionRefusal(q channel.Question) string {
	switch {
	case len(q.Facts) > 1:
		return q.Facts[1]
	case len(q.Facts) == 1:
		return q.Facts[0]
	}
	return "(no refusal stated)"
}

// parseQuestionAge reads --since: a number of days (7d), or a Go duration
// (36h, 90m).
func parseQuestionAge(text string) (time.Duration, error) {
	if days, ok := strings.CutSuffix(text, "d"); ok {
		n, err := strconv.Atoi(days)
		if err != nil || n <= 0 {
			return 0, fmt.Errorf("%q is not a positive number of days", text)
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	age, err := time.ParseDuration(text)
	if err != nil || age <= 0 {
		return 0, fmt.Errorf("%q is not an age such as 7d, 36h or 90m", text)
	}
	return age, nil
}

// answeredQuestions is question list --answered: the questions that ended,
// answered or withdrawn, grouped by refusal line, most frequent first.
func (inv *intentInvocation) answeredQuestions() intentResult {
	var since time.Time
	window := ""
	if inv.input.has("since") {
		age, err := parseQuestionAge(inv.input.text("since"))
		if err != nil {
			return intentResult{Outcome: intentRefused, code: 2, Summary: "--since " + err.Error() + ", so nothing was listed",
				next: inv.retryWith([]string{"since"}, "--since", "7d"), nextReason: "the last seven days"}
		}
		now, err := goalCommandNow(inv.stateRoot)
		if err != nil {
			return intentResult{Outcome: intentFailed, code: 1, Summary: "this checkout's clock cannot be read, so nothing was listed: " + err.Error(),
				next: inv.publicArgv("system", "check"), nextReason: "names what is wrong here"}
		}
		since = now.Add(-age)
		window = " in the last " + inv.input.text("since")
	}
	all, unreadable := channel.WalkQuestions(inv.stateRoot)
	groups := map[string]*answeredGroup{}
	var ended []channel.Question
	for _, q := range all {
		if q.State == "open" || q.OpenedAt.Before(since) {
			continue
		}
		ended = append(ended, q)
		refusal := questionRefusal(q)
		group := groups[refusal]
		if group == nil {
			group = &answeredGroup{Refusal: refusal}
			groups[refusal] = group
		}
		group.Count++
		if q.Answer == nil {
			group.Withdrawn++
			continue
		}
		group.Answered++
		if !q.Answer.At.IsZero() {
			group.waits = append(group.waits, q.Answer.At.Sub(q.OpenedAt))
		}
	}
	ordered := make([]*answeredGroup, 0, len(groups))
	for _, group := range groups {
		if median, ok := medianDuration(group.waits); ok {
			group.MedianToAnswer = median.String()
		}
		ordered = append(ordered, group)
	}
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].Count != ordered[j].Count {
			return ordered[i].Count > ordered[j].Count
		}
		return ordered[i].Refusal < ordered[j].Refusal
	})
	var allWaits []time.Duration
	lines := []string{}
	for _, group := range ordered {
		allWaits = append(allWaits, group.waits...)
		line := fmt.Sprintf("  %d× %s: %d answered, %d withdrawn", group.Count, group.Refusal, group.Answered, group.Withdrawn)
		if group.MedianToAnswer != "" {
			line += ", median " + group.MedianToAnswer + " to answer"
		}
		lines = append(lines, line)
	}
	data := map[string]any{"groups": ordered, "unreadable": nonNilLines(unreadable)}
	verbose := inv.input.switched("verbose")
	if verbose {
		views := []map[string]any{}
		for _, q := range ended {
			view := map[string]any{"reference": "channel:" + q.ID, "state": q.State, "goal": q.Goal, "about": q.About,
				"refusal": questionRefusal(q), "openedAt": q.OpenedAt}
			if q.Answer != nil {
				view["answer"], view["answeredAt"] = q.Answer.Text, q.Answer.At
			}
			views = append(views, view)
			lines = append(lines, fmt.Sprintf("  channel:%s  %s  %s  %s", q.ID, q.State, channel.QuestionSubject(q), questionRefusal(q)))
		}
		data["questions"] = views
	}
	for _, problem := range unreadable {
		lines = append(lines, "  unreadable: "+problem)
	}
	summary := fmt.Sprintf("%d question(s) answered or withdrawn%s, from %d refusal(s)", len(ended), window, len(ordered))
	if median, ok := medianDuration(allWaits); ok {
		summary += "; median time to answer " + median.String()
	}
	result := intentResult{Outcome: intentConfirmed, text: lines, Data: data, Summary: summary,
		view: answeredQuestionsView(summary, ordered, ended, verbose)}
	if len(unreadable) > 0 {
		result.Outcome, result.code = intentPartial, 1
	}
	return result
}

func medianDuration(values []time.Duration) (time.Duration, bool) {
	if len(values) == 0 {
		return 0, false
	}
	sorted := append([]time.Duration(nil), values...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	middle := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[middle], true
	}
	return (sorted[middle-1] + sorted[middle]) / 2, true
}

// answeredQuestionsView is question list --answered: the summary, then one
// card per refusal with its counts; --verbose adds each question.
func answeredQuestionsView(summary string, groups []*answeredGroup, ended []channel.Question, verbose bool) func(*textui.Page) {
	return func(page *textui.Page) {
		if len(groups) == 0 {
			page.Headline("No question was answered or withdrawn in this window")
			return
		}
		page.Headline(strings.ToUpper(summary[:1]) + summary[1:])
		section := page.Section("", "")
		for _, group := range groups {
			card := section.Item(textui.Done, fmt.Sprintf("%d× %s", group.Count, group.Refusal))
			counts := fmt.Sprintf("%d answered, %d withdrawn", group.Answered, group.Withdrawn)
			if group.MedianToAnswer != "" {
				counts += ", median " + group.MedianToAnswer + " to answer"
			}
			card.KV("ended", textui.Plain(counts))
		}
		if !verbose {
			return
		}
		each := page.Section("Each question", "")
		for _, q := range ended {
			card := each.Item(textui.Done, "channel:"+q.ID+" about "+channel.QuestionSubject(q))
			card.KV("refusal", textui.Plain(questionRefusal(q)))
			if q.Answer != nil {
				card.KV("answer", textui.Plain(q.Answer.Text))
			} else {
				card.KV("answer", textui.Plain("withdrawn"))
			}
		}
	}
}
