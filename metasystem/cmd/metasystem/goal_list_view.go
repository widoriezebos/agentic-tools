package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/textui"
)

// goalListing is what goal list shows (output-style.md §6.7, decision D1):
// by default the active goals (claimed, approved, and queued at priority 1),
// with --all or a label filter every open goal, and with --all the done and
// abandoned ones too. --json is untouched: it lists every open goal.
type goalListing struct {
	grouped            map[string][]*goal.GoalFile
	open               int
	archived, filtered bool
	history            bool
	grants             []goal.PowerOfAttorneyEntry
	banners            []string
	trunkRed           []goal.TrunkRedEntry
	horizon            goal.ApprovalHorizon
	tip                string
	rankWidth, idWidth int
}

// goalRank is a goal's place in the queue, priority.sequence, or a dash
// when it has none.
func goalRank(env textui.Env, file *goal.GoalFile) string {
	if file.Priority != 0 {
		return fmt.Sprintf("%d.%d", file.Priority, file.Sequence)
	}
	if env.ASCII {
		return "-"
	}
	return "–"
}

// goalListActive is the default listing's rule: a queued goal shows only at
// priority 1, a parked one never.
func goalListActive(file *goal.GoalFile) bool {
	switch file.State {
	case goal.StateClaimed, goal.StateApproved:
		return true
	case goal.StateQueued:
		return file.Priority == 1
	}
	return false
}

func (listing goalListing) view(page *textui.Page) {
	listing.rankWidth, listing.idWidth = 0, 0
	env := page.Env()
	every := listing.archived || listing.filtered
	var banners []textui.Attention
	for _, notice := range listing.banners {
		banners = append(banners, textui.Attention{State: textui.Alert, Text: strings.Join(strings.Fields(notice), " ")})
	}
	for _, entry := range listing.trunkRed {
		if entry.Closed != nil {
			continue
		}
		owner := entry.Owner.Machine
		if owner == "" {
			owner = "nobody"
		}
		text := fmt.Sprintf("main is red in %s, owned by %s", entry.Group, owner)
		if opened, err := time.Parse(time.RFC3339, entry.Opened); err == nil {
			text += ", " + env.Since(opened)
		}
		banners = append(banners, textui.Attention{State: textui.Failed, Text: text,
			Hint: textui.Hint{Argv: []string{"metasystem", "incident", "list"}, Reason: "who fixes it"}})
	}
	page.Banner(banners...)

	facts := []string{}
	for _, state := range syncedListStates {
		if count := len(listing.grouped[state]); count > 0 {
			facts = append(facts, textui.Number(int64(count))+" "+state)
		}
	}
	if listing.archived {
		for _, state := range []string{goal.StateDone, goal.StateAbandoned} {
			facts = append(facts, textui.Number(int64(len(listing.grouped[state])))+" "+state)
		}
	}
	if listing.open == 0 && !listing.archived {
		page.Headline("No open goals", facts...)
	} else {
		page.Headline(textui.Count(listing.open, "open goal", "open goals"), facts...)
	}
	if page.Verbose() {
		page.Facts(textui.KV{Key: "ledger", Value: []textui.Span{textui.Plain(textui.SHA(listing.tip))}})
	}

	// Every section's rank and goal columns share one width, so the
	// sections read as one table.
	var shown []*goal.GoalFile
	for _, state := range append(append([]string(nil), syncedListStates...), goal.StateDone, goal.StateAbandoned) {
		for _, file := range listing.grouped[state] {
			if every || goalListActive(file) {
				shown = append(shown, file)
			}
		}
	}
	for _, file := range shown {
		listing.rankWidth = max(listing.rankWidth, len(goalRank(env, file)))
		listing.idWidth = max(listing.idWidth, len([]rune(file.Id)))
	}
	hidden := 0
	for _, state := range syncedListStates {
		files := listing.ordered(state)
		title := strings.ToUpper(state[:1]) + state[1:]
		shown := files
		aside := ""
		if !every {
			shown = nil
			for _, file := range files {
				if goalListActive(file) {
					shown = append(shown, file)
				}
			}
			hidden += len(files) - len(shown)
			if state == goal.StateQueued {
				title, aside = "Queued, priority 1", fmt.Sprintf("%d of %d", len(shown), len(files))
			}
		}
		listing.section(page, title, aside, state, shown)
	}
	if listing.archived {
		listing.section(page, "Done", "", goal.StateDone, listing.ordered(goal.StateDone))
		listing.section(page, "Abandoned", "", goal.StateAbandoned, listing.ordered(goal.StateAbandoned))
	}
	if listing.history {
		for _, state := range append(append([]string(nil), syncedListStates...), goal.StateDone, goal.StateAbandoned) {
			for _, file := range listing.ordered(state) {
				if !every && !goalListActive(file) {
					continue
				}
				history := page.Section(file.Id, "history")
				for _, entry := range file.History {
					line := entry.Verb + " by " + goalHistoryActor(entry, listing.grants)
					if at, err := time.Parse(time.RFC3339, entry.At); err == nil {
						line = env.Time(at) + "  " + line
					}
					if entry.Reason != "" {
						line += ": " + entry.Reason
					}
					history.Text(line)
				}
			}
		}
	}
	if hidden > 0 {
		page.Hint(textui.Hint{Argv: []string{"metasystem", "goal", "list", "--all"},
			Reason: "every open goal (" + textui.Number(int64(hidden)) + " more), and the concluded ones"})
	}
}

// ordered is one state's goals in the ledger's order.
func (listing goalListing) ordered(state string) []*goal.GoalFile {
	files := map[string]*goal.GoalFile{}
	for _, file := range listing.grouped[state] {
		files[file.Id] = file
	}
	ordered := make([]*goal.GoalFile, 0, len(files))
	for _, id := range goal.OrderedOpenGoalIDs(files) {
		ordered = append(ordered, files[id])
	}
	return ordered
}

// section is one state's table: rank, goal, tier, the claiming machine for
// claimed goals, and the next step (the column cut at the width).
func (listing goalListing) section(page *textui.Page, title, aside, state string, files []*goal.GoalFile) {
	if len(files) == 0 {
		return
	}
	env := page.Env()
	claimed := state == goal.StateClaimed
	cols := []textui.Column{{}, {}, {}}
	if claimed {
		cols = append(cols, textui.Column{})
	}
	cols = append(cols, textui.Column{Flex: true})
	table := page.Section(title, aside).Table(cols...)
	for _, file := range files {
		rank := goalRank(env, file)
		tier := "tier ?"
		if file.Tier != 0 {
			tier = fmt.Sprintf("tier %d", file.Tier)
		}
		cells := []textui.Span{textui.Plain(rank + strings.Repeat(" ", max(listing.rankWidth-len([]rune(rank)), 0))),
			textui.Plain(file.Id + strings.Repeat(" ", max(listing.idWidth-len([]rune(file.Id)), 0))), textui.Dim(tier)}
		if claimed {
			machine := ""
			if file.Claimed != nil {
				machine = file.Claimed.Machine
			}
			cells = append(cells, textui.Plain(machine))
		}
		cells = append(cells, textui.Plain(listing.nextText(env, file)))
		table.Row(cells...)
	}
}

// nextText is a row's free text: its markers (pin, landing, a relayed
// approval) before the next step, or why a parked goal waits, or a
// concluded goal's conclusion.
func (listing goalListing) nextText(env textui.Env, file *goal.GoalFile) string {
	var markers []string
	if file.Pinned != "" && file.State != goal.StateClaimed {
		markers = append(markers, "pin "+file.Pinned)
	}
	if file.Landing != nil {
		marker := "landing"
		if at, err := time.Parse(time.RFC3339, file.Landing.At); err == nil {
			marker += " " + env.Since(at)
		}
		markers = append(markers, marker)
	}
	if file.Approved != nil && file.Approved.Authority == goal.ApprovalAuthorityRelayed {
		if expired, _ := file.ApprovalExpired(listing.horizon); expired {
			markers = append(markers, "relayed approval expired")
		} else {
			markers = append(markers, "relayed approval, review by "+file.Approved.ReviewBy)
		}
	}
	text := goalNextSentence(file.NextStep)
	switch {
	case file.State == goal.StateParked && file.Parked != nil && file.Parked.Because != "":
		text = "waits: " + strings.Join(strings.Fields(file.Parked.Because), " ")
	case (file.State == goal.StateDone || file.State == goal.StateAbandoned) && file.Conclude != "":
		text = goalNextSentence(file.Conclude)
	}
	return strings.Join(append(markers, text), " · ")
}

// goalTiersView is goal list --tiers: how the open goals with a risk record
// spread over the tiers, as recorded and as their answers derive, and the
// goals a person may lower.
func goalTiersView(probe goal.TierProbe, recordedShare, derivedShare int) func(*textui.Page) {
	return func(page *textui.Page) {
		page.Headline(textui.Count(probe.Open, "open goal with a risk record", "open goals with a risk record"))
		spread := func(counts map[uint8]int, share int) textui.Span {
			return textui.Plain(fmt.Sprintf("tier 1: %d · tier 2: %d · tier 3: %d (tier 3 is %d%%)", counts[1], counts[2], counts[3], share))
		}
		page.Facts(textui.KV{Key: "recorded", Value: []textui.Span{spread(probe.Recorded, recordedShare)}},
			textui.KV{Key: "derived", Value: []textui.Span{spread(probe.Derived, derivedShare)}})
		if len(probe.Lowerable) == 0 {
			return
		}
		table := page.Section("A person may lower", "").Table(textui.Column{}, textui.Column{}, textui.Column{Flex: true})
		for _, lower := range probe.Lowerable {
			why := fmt.Sprintf("recorded tier %d, its answers derive tier %d", lower.Recorded, lower.Derived)
			if lower.Override {
				why += " (raised on purpose)"
			}
			table.Row(textui.Plain(lower.ID), textui.Dim(lower.State), textui.Plain(why))
		}
	}
}
