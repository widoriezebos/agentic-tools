package overview_test

import (
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/backlog"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/rulings"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/decisions"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/overview"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/project"
)

// "Needs you" is what the Decisions inbox says needs the person, read from the
// inbox itself: of three unapproved goals only the one ranked first counts, an
// open row of the questions register counts as nothing and a seat's open
// channel question as one, and a ruling past its review is counted with the
// kinds the block does not list. The register's open row is still counted
// where the project's memory is shown.
func TestNeedsYouIsWhatTheDecisionsInboxSaysNeedsThePerson(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 25, 11, 0, 0, 0, time.UTC)
	todo := func(id string, priority uint8) backlog.Row {
		return backlog.Row{
			Ref: backlog.Ref{Kind: "goal", ID: id, Revision: 1}, Where: backlog.WhereLive,
			Lane: backlog.LaneToDo, Intent: id, Priority: priority,
		}
	}
	in := decisions.Inputs{
		Rows: []backlog.Row{todo("g-first", 1), todo("g-lower", 2), todo("g-unranked", 0)},
		Project: project.Pane{
			Records: []project.Record{{Kind: "decision", ID: "d-draft", Status: "draft", Title: "A draft", Path: "docs/d-draft.md"}},
			Questions: []project.Question{
				{ID: "Q-1", Opened: "2026-09-20", Question: "A register question", Status: "open", Goals: []string{}},
			},
		},
		Asks: []channel.Question{{ID: "q-1", Goal: "g-first", State: "open", OpenedAt: now.Add(-time.Hour)}},
		Register: rulings.Register{
			Rows:    []rulings.Row{{ID: "R-2", Words: "A temporary ruling."}},
			Reviews: []rulings.Review{{ID: "R-2", Owner: "Wido", Class: "temporary", Due: "2026-09-20"}},
		},
	}
	page := overview.Compose(overview.Inputs{
		Project: in.Project, Rows: in.Rows,
		Inbox: decisions.ForOverview(decisions.Inbox(in, now)),
	}, now)
	needs := page.NeedsYou

	testutil.Expect(t, "only the first-ranked goal awaits approval", needs.Approvals.Count, 1)
	testutil.Expect(t, "the goal ranked first", needs.Approvals.Items[0].ID, "g-first")
	testutil.Expect(t, "the channel question and not the register's", needs.Questions.Count, 1)
	testutil.Expect(t, "the seat's question", needs.Questions.Items[0].ID, "q-1")
	testutil.Expect(t, "the draft", needs.Drafts.Count, 1)
	testutil.Expect(t, "the ruling past its review", needs.Other.Count, 1)
	testutil.Expect(t, "the total is the Decisions count", needs.Total, decisions.Compose(in, now).Counts.NeedsYou)
	testutil.Expect(t, "the groups add up to it", needs.Approvals.Count+needs.Questions.Count+
		needs.Drafts.Count+needs.Designs.Count+needs.Alerts.Count+needs.Other.Count, needs.Total)
	testutil.Expect(t, "the register's open row stays in the memory block", page.Memory.Questions, 1)
}
