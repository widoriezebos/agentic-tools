package main

import (
	"fmt"
	"strings"
	"time"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/textui"
)

// goalShown is goal show's page (output-style.md §6.8): the goal's state
// in the headline, its intent and next step cut to a few lines (whole with
// --verbose), its budget as spent of limit, its designs and what it is
// allowed.
type goalShown struct {
	file          *goal.GoalFile
	where, tip    string
	budget        intentBudgetView
	designs       []intentDesignRef
	designProblem string
	allowed       []string
	history       bool
	grants        []goal.PowerOfAttorneyEntry
}

// goalShowProse is how much of the intent and the next step the default
// page shows: about four lines.
const goalShowProse = 380

func (shown goalShown) view(page *textui.Page) {
	env, file := page.Env(), shown.file
	if file.StopFence != nil {
		text := "Stopped: " + goalStopWords(file.StopFence.Reason)
		if at, err := time.Parse(time.RFC3339, file.StopFence.ClosedAt); err == nil {
			text = "Stopped " + env.Time(at) + ": " + goalStopWords(file.StopFence.Reason)
		}
		page.Banner(textui.Attention{State: textui.Alert, Text: text})
	}
	facts, headline := []string{}, file.Id+" is "+file.State
	if file.Claimed != nil && file.State == goal.StateClaimed {
		headline += " by " + file.Claimed.Machine
	}
	if file.Tier != 0 {
		facts = append(facts, fmt.Sprintf("tier %d", file.Tier))
	} else {
		facts = append(facts, "no tier yet")
	}
	if file.Priority != 0 {
		facts = append(facts, fmt.Sprintf("priority %d.%d", file.Priority, file.Sequence))
	}
	if file.Pinned != "" {
		facts = append(facts, "pinned to "+file.Pinned)
	}
	if file.StopFence != nil {
		facts = append(facts, "stopped by its budget")
	}
	page.Headline(headline, facts...)
	if page.Verbose() {
		page.Facts(textui.KV{Key: "ledger", Value: []textui.Span{textui.Plain(textui.SHA(shown.tip))}},
			textui.KV{Key: "kept", Value: []textui.Span{textui.Plain(shown.where)}})
	}

	prose := func(text string) string {
		text = strings.Join(strings.Fields(text), " ")
		if page.Verbose() || len([]rune(text)) <= goalShowProse {
			return text
		}
		cut := string([]rune(text)[:goalShowProse])
		if space := strings.LastIndex(cut, " "); space > goalShowProse/2 {
			cut = cut[:space]
		}
		return cut + " …"
	}
	page.Section("Intent", "").Text(prose(file.Intent))
	page.Section("Next step", "").Text(prose(file.NextStep))
	if file.Parked != nil && file.Parked.Because != "" {
		page.Section("Waits", "").Text(prose(file.Parked.Because))
	}
	if (file.State == goal.StateDone || file.State == goal.StateAbandoned) && file.Conclude != "" {
		page.Section("Conclusion", "").Text(prose(file.Conclude))
	}
	shown.budgetSection(page)

	if len(shown.designs) > 0 || shown.designProblem != "" {
		designs := page.Section("Designs", "")
		if shown.designProblem != "" {
			designs.Text("! the design records can't be read: " + shown.designProblem)
		}
		table := designs.Table(textui.Column{}, textui.Column{}, textui.Column{Flex: true})
		for _, design := range shown.designs {
			state := textui.Running
			switch design.Status {
			case "done":
				state = textui.Done
			case "superseded", "rejected", "withdrawn":
				state = textui.Stopped
			}
			table.Row(textui.Marked(state, design.Status), textui.Plain(env.Path(design.Path)), textui.Plain(design.Title))
		}
	}
	if len(shown.allowed) > 0 {
		page.Section("Allowed", "").Text(strings.Join(shown.allowed, ", "))
	}
	if shown.history {
		history := page.Section("History", "")
		for _, entry := range file.History {
			line := entry.Verb + " by " + goalHistoryActor(entry, shown.grants)
			if at, err := time.Parse(time.RFC3339, entry.At); err == nil {
				line = env.Time(at) + "  " + line
			}
			if entry.Reason != "" {
				line += ": " + entry.Reason
			}
			history.Text(line)
		}
		if file.Conclude != "" {
			history.Text("conclusion: " + file.Conclude)
		}
	}
}

// budgetSection is the budget as spent of each limit, the elapsed row
// marked when it is past its limit.
func (shown goalShown) budgetSection(page *textui.Page) {
	view := shown.budget
	if view.Projection == nil {
		page.Section("Budget", "").Text("none yet; approval gives the goal its tier's box")
		return
	}
	aside := "this claim"
	if view.Lens != "claim" {
		aside = "since the last approval"
	}
	if page.Verbose() {
		aside += " · box " + view.Box
	}
	section := page.Section("Budget", aside)
	projection := view.Projection
	if projection.Status != dispatchcore.BudgetKnown {
		reason := "its spending records can't be read"
		if projection.Unknown != nil && projection.Unknown.Reason != "" {
			reason = projection.Unknown.Reason
		}
		section.Text("? unknown: " + reason)
		return
	}
	limits := projection.Limits
	elapsed := []textui.Span{textui.Plain(textui.Duration(projection.Elapsed.Round(time.Minute)) + " of " + limits.ElapsedLimit)}
	switch projection.ElapsedState {
	case dispatchcore.ElapsedBreach:
		elapsed = append(elapsed, textui.Plain("  "), textui.Marked(textui.Alert, "past its limit"))
	case dispatchcore.AdmissionClosedElapsed:
		elapsed = append(elapsed, textui.Plain("  "), textui.Marked(textui.Alert, "no new work starts"))
	}
	section.KV("elapsed", elapsed...)
	section.KV("attempts", textui.Plain(fmt.Sprintf("%s of %s", textui.Number(int64(projection.Attempts)), textui.Number(int64(limits.AttemptLimit)))))
	section.KV("job minutes", textui.Plain(fmt.Sprintf("%s of %s", textui.Number(int64(projection.ReservedJobMinutes)), textui.Number(int64(limits.ReservedJobMinutesLimit)))))
	section.KV("active jobs", textui.Plain(fmt.Sprintf("%s of %s", textui.Number(int64(projection.ActiveJobs)), textui.Number(int64(limits.ActiveJobLimit)))))
}

// goalStopWords says why a goal's budget stopped it, in a person's words.
func goalStopWords(reason string) string {
	switch reason {
	case goal.StopReasonElapsedLimit:
		return "its elapsed-time budget ran out"
	case goal.StopReasonCorruptOverLimit:
		return "its spending records could not be read, so it was stopped as over its budget"
	}
	return "its budget stopped it (" + strings.ToLower(strings.ReplaceAll(reason, "_", " ")) + ")"
}
