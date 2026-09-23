package partner_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner/fakeacp"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/project"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/uitools"
)

// What the Partner was actually told, read off the wire.
//
// A scripted answer proves nothing about composition: the fake server streams
// the same words whether or not it was given an index, a skill or a tool
// server. So these assert against the prompt the client really sent and the
// tool servers it really handed over, and every assertion names something that
// can only be there because its input was supplied — a record id out of the
// project reader, a heading out of the kit's own skill file. A build that
// stopped supplying one of them fails here rather than passing on an empty
// string.

// The first prompt of a session carries all three: the standing rule, how to
// answer here, and a map of the project's memory.
func TestTheFirstPromptCarriesTheStandingRuleTheSkillAndTheIndex(t *testing.T) {
	t.Parallel()
	service, handed := composing(t, indexedProject())
	ask(t, service, "key-1", "what has this project decided?")

	prompt, sent := handed.First()
	testutil.Require(t, "a prompt was sent", sent, true)

	testutil.Expect(t, "the standing rule is in front of it",
		strings.HasPrefix(prompt, "You are the Project Partner for this MetaSystem workspace."), true)
	testutil.Expect(t, "it says what the Partner may not do",
		strings.Contains(prompt, "You do not write, you do not run commands, and you do not act"), true)

	testutil.Expect(t, "the kit's own skill is carried",
		strings.Contains(prompt, "How to answer here, from this kit's own "+partner.SkillPath), true)
	testutil.Expect(t, "with its three territories",
		strings.Contains(prompt, "**The metasystem itself.**"), true)

	testutil.Expect(t, "the index is carried",
		strings.Contains(prompt, "The project's memory, indexed at"), true)
	testutil.Expect(t, "and it is a map rather than evidence",
		strings.Contains(prompt, "it is not evidence that something exists, is absent, or has the status shown"), true)
	// Read out of the project reader and nowhere else: no reader, no row.
	testutil.Expect(t, "with the rows the project reader answered with",
		strings.Contains(prompt, "01DECISION · One ACP client"), true)
	testutil.Expect(t, "and the question register's own row ids",
		strings.Contains(prompt, "- Q-4 · Which runtime answers?"), true)

	testutil.Expect(t, "the page comes after all of it",
		strings.Index(prompt, "The project's memory, indexed at") <
			strings.Index(prompt, "What the human sees now"), true)
	testutil.Expect(t, "and the question is last",
		strings.HasSuffix(strings.TrimSpace(prompt), "The human asks:\nwhat has this project decided?"), true)
}

// Each of the three is a separate input, and removing one shows in the prompt.
// This is what makes the assertions above evidence rather than decoration: the
// same test with an input taken away must not still pass.
func TestRemovingAnInputChangesWhatTheFirstPromptCarries(t *testing.T) {
	t.Parallel()
	service, handed := composing(t, nil)
	ask(t, service, "key-1", "what has this project decided?")

	prompt, sent := handed.First()
	testutil.Require(t, "a prompt was sent", sent, true)
	testutil.Expect(t, "the standing rule is still there",
		strings.Contains(prompt, "You are the Project Partner for this MetaSystem workspace."), true)
	testutil.Expect(t, "and the skill is still there",
		strings.Contains(prompt, "How to answer here, from this kit's own "+partner.SkillPath), true)
	testutil.Expect(t, "but the rows are gone with the reader",
		strings.Contains(prompt, "01DECISION · One ACP client"), false)
	testutil.Expect(t, "and the index says so rather than looking empty",
		strings.Contains(prompt, "This build has no reader for the project's records"), true)
}

// A later prompt of the same session carries the moment the map was read, and
// not the map: a map re-sent every turn would be a promise that it is current,
// which is exactly what it is not.
func TestALaterPromptCarriesTheIndexsMomentAndNotTheIndex(t *testing.T) {
	t.Parallel()
	service, handed := composing(t, indexedProject())
	ask(t, service, "key-1", "first question")
	ask(t, service, "key-2", "second question")

	prompts := handed.Prompts()
	testutil.Require(t, "two prompts were sent", len(prompts), 2)
	later := prompts[1]
	testutil.Expect(t, "the map is not sent again",
		strings.Contains(later, "The project's memory, indexed at"), false)
	testutil.Expect(t, "nor is the skill",
		strings.Contains(later, "How to answer here, from this kit's own"), false)
	testutil.Expect(t, "the moment it was read is named",
		strings.Contains(later, "was read at 2026-09-23T12:00:00Z"), true)
	testutil.Expect(t, "and the standing rule is still every turn's",
		strings.HasPrefix(later, "You are the Project Partner for this MetaSystem workspace."), true)
}

// A session that was lost is a first prompt again, and is given both again,
// read afresh rather than replayed.
func TestARecoveredSessionIsGivenTheSkillAndIndexAgain(t *testing.T) {
	t.Parallel()
	service, handed := composing(t, indexedProject())
	ask(t, service, "key-1", "first question")
	service.Close()
	ask(t, service, "key-2", "second question")

	prompts := handed.Prompts()
	testutil.Require(t, "two prompts were sent", len(prompts), 2)
	testutil.Expect(t, "the recovered session is given the skill again",
		strings.Contains(prompts[1], "How to answer here, from this kit's own "+partner.SkillPath), true)
	testutil.Expect(t, "and the map again",
		strings.Contains(prompts[1], "The project's memory, indexed at"), true)
	testutil.Expect(t, "with the conversation it lost",
		strings.Contains(prompts[1], "first question"), true)
}

// The two tools that make the interface and the kit answerable are offered to
// the session, through the one tool server the hand-off names.
func TestTheSessionIsOfferedTheInterfaceAndKitTools(t *testing.T) {
	t.Parallel()
	service, handed := composing(t, indexedProject())
	ask(t, service, "key-1", "what is a lane?")

	testutil.Expect(t, "the tool server is handed over", handed.Named(), []string{uitools.ServerName})
	// The catalogue that server publishes is the uitools package's own, and
	// naming it here is what ties the hand-off to the operations: a server
	// handed over that answered neither would be a Partner with no way to
	// read the interface or the kit.
	offered := map[string]bool{}
	for _, tool := range uitools.Catalogue() {
		offered[tool.Name] = true
	}
	testutil.Expect(t, "and it answers interface()", offered[uitools.OpInterface], true)
	testutil.Expect(t, "and kit()", offered[uitools.OpKit], true)
}

/* ---------------------------------------------------------------- driving -- */

// composing is a Partner over a fake ACP server that records what it was sent,
// with the project reader the test wants and the one tool server the hand-off
// names.
func composing(t *testing.T, pane func() (project.Pane, error)) (*partner.Service, *fakeacp.Servers) {
	t.Helper()
	root := t.TempDir()
	opener, handed := fakeacp.OpenWatched(fakeacp.Script{
		Models: []string{"fake-1"}, Chunks: []string{"here is an answer."}})
	runtime := partner.Runtime{
		Name: "fake", Model: "fake-1", ReadOnly: "a fake server reads nothing",
		Tools: &partner.ToolServer{
			Name: uitools.ServerName, Command: "/bin/metasystem",
			Args: []string{"ui", "tools", "--root", root},
		},
	}
	host := partner.NewHostOn(runtime, root, opener)
	t.Cleanup(host.Close)
	service := partner.NewService(runtime, host,
		func(human string) (*partner.Conversation, error) { return partner.OpenConversation(root, human) },
		partner.Facts{Project: pane},
		func() time.Time { return time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC) })
	return service, handed
}

// ask submits one turn and waits for it to settle.
func ask(t *testing.T, service *partner.Service, key, text string) {
	t.Helper()
	events, stop := service.Subscribe()
	defer stop()
	_, err := service.Submit(context.Background(), "Wido", key, text,
		partner.Page{Section: "Backlog", Path: "/backlog"})
	testutil.Require(t, "admitted "+key, err, nil)
	drain(t, events)
}

// indexedProject is a project with one of each kind, so the index has rows
// that can only have come from this reader.
func indexedProject() func() (project.Pane, error) {
	return func() (project.Pane, error) {
		return project.Pane{
			ReadAt: "2026-09-23T12:00:00Z",
			Records: []project.Record{
				{Kind: "decision", ID: "01DECISION", Status: "accepted", Title: "One ACP client",
					Path: "docs/decisions/acp.md", ChangedAt: "2026-09-20T00:00:00Z",
					Summary: "The seam is the Agent Client Protocol."},
			},
			Questions: []project.Question{
				{ID: "Q-4", Opened: "2026-09-22", Question: "Which runtime answers?", Status: "open"},
			},
		}, nil
	}
}
