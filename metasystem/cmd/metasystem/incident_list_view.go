package main

import (
	"fmt"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/textui"
)

// incidentListView is incident list's page: how many tests fail on main
// and whether someone fixes each, then the tracked flakes and hangs apart,
// since they never hold a landing.
func incidentListView(listed, tracked []goal.TrunkRedEntry, all bool) func(*textui.Page) {
	return func(page *textui.Page) {
		env := page.Env()
		unowned := 0
		for _, entry := range listed {
			if entry.Closed == nil && entry.FixGoal == "" {
				unowned++
			}
		}
		facts := []string{}
		if unowned > 0 {
			facts = append(facts, textui.Number(int64(unowned))+" nobody fixes")
		}
		if len(tracked) > 0 {
			facts = append(facts, textui.Count(len(tracked), "tracked flake or hang", "tracked flakes or hangs"))
		}
		switch {
		case len(listed) == 0 && all:
			page.Headline("No incidents on main recorded", facts...)
		case len(listed) == 0:
			page.Headline("No open incidents on main", facts...)
		default:
			page.Headline(textui.Count(len(listed), "incident on main", "incidents on main"), facts...)
		}
		since := func(entry goal.TrunkRedEntry) string {
			if opened, err := time.Parse(time.RFC3339, entry.Opened); err == nil {
				return env.Since(opened)
			}
			return ""
		}
		if len(listed) > 0 {
			table := page.Section("On main", "").Table(textui.Column{}, textui.Column{}, textui.Column{Flex: true})
			for _, entry := range listed {
				state, words := textui.Alert, "nobody fixes it"
				switch {
				case entry.Closed != nil:
					state, words = textui.Done, "closed"
				case entry.FixGoal != "":
					state, words = textui.Running, "goal "+entry.FixGoal+" fixes it"
				}
				if at := since(entry); at != "" && entry.Closed == nil {
					words += " · failing " + at
				}
				table.Row(textui.Marked(state, entry.ID), textui.Plain(entry.Group), textui.Plain(words))
			}
		}
		if len(tracked) > 0 {
			table := page.Section("Tracked", "they never hold a landing").Table(textui.Column{}, textui.Column{}, textui.Column{Flex: true})
			for _, entry := range tracked {
				state, words := textui.Idle, trackedDefectLabel(entry.EntryClass())
				if entry.Closed != nil {
					state, words = textui.Done, fmt.Sprintf("%s · closed", words)
				}
				table.Row(textui.Marked(state, entry.ID), textui.Plain(entry.Group), textui.Plain(words))
			}
		}
	}
}
