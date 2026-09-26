package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/backlog"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/project"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/snapshot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/uitools"
)

// The by-hand check, before a release: a real runtime, a dozen questions, and
// a file a human reads.
//
// It is smoke and not certification, and it says so in the file it writes. A
// dozen real answers cannot establish that this Partner is an expert; they can
// show that it contradicts itself, claims what it cannot do, cites a source
// that says something else, or describes a page this build does not have —
// and none of those shows up in a deterministic test, because a deterministic
// test asserts what was SENT and a model's answer is what came back.
//
// It runs through the production path and nothing beside it: the admitted
// runtime, its own read-only configuration, the real host, the real
// conversation owner, the real context composer, and the engine's own tool
// server over stdio. What is a fixture is only the workspace: a temporary
// checkout with this walkthrough's canned ledger behind it and a copy of this
// kit's own documents in it, so that nothing a real agent does can touch the
// repository the human is working in.

// The dozen questions. They are the design's seven kinds spread over twelve
// asks, one per territory the Partner has to tell apart: the interface's own
// words, a lane, a section this build does not have, an act and its hand, its
// own inability to write, the kit's verbs, concepts and rulings, where the
// records live, what this seat is configured to, and the page in front of it.
var smokeQuestions = []struct{ kind, text string }{
	{"a term", "What does Ready for Work mean in this interface?"},
	{"a term", "What is a tier, and who decides it?"},
	{"a lane", "What is the Waiting lane for, and is it one of the lanes this board shows?"},
	{"an unavailable section", "Where in this interface do I look at the fleet's health?"},
	{"an unavailable section", "What would the Decisions section show me, and can I open it today?"},
	{"an act", "What do I have to be for this interface to let me approve a goal?"},
	{"can you write", "Can you edit this design with me and save it?"},
	{"a verb", "What does the command `metasystem goals --ready` do?"},
	{"a ruling", "Is there a standing ruling about getting a design critiqued before it is built?"},
	{"a concept", "What is a lease epoch?"},
	{"the records", "Where does this checkout keep its decisions, and what is a decision for?"},
	{"the page", "What am I looking at right now, and what is in the To Do lane?"},
}

// runSmoke asks the dozen questions of one real runtime and writes the answers
// where a human can read them.
func runSmoke(runtimeName, model, engine, kit, out string) {
	checkout := smokeCheckout(kit)
	fmt.Println("smoke checkout " + checkout)

	admitted, err := partner.Admit(runtimeName, nil, model, checkout)
	if err != nil {
		log.Fatalf("this build does not admit %q as a Partner: %v", runtimeName, err)
	}
	admitted.Tools = &partner.ToolServer{
		Name: uitools.ServerName, Command: engine,
		Args: []string{"ui", "tools", "--root", checkout, "--metasystem-root", checkout},
	}

	state := newLedger(false)
	state.roots = project.Roots{Checkout: checkout, Installation: checkout, StateRoot: checkout}
	facts := partner.Facts{
		Observe: state.observe,
		Project: func() (project.Pane, error) { return state.project(), nil },
		Document: func(id string) (project.Document, error) {
			return project.Read(state.roots, id, time.Now().UTC())
		},
	}

	host := partner.NewHost(admitted, checkout,
		filepath.Join(fixtureConversations(checkout), "wire.jsonl"))
	defer host.Close()
	service := partner.NewService(admitted, host,
		func(human string) (*partner.Conversation, error) {
			return partner.OpenConversation(fixtureConversations(checkout), human)
		},
		facts, func() time.Time { return time.Now().UTC() })
	defer service.Close()

	file, err := os.Create(out)
	if err != nil {
		log.Fatalf("cannot write the smoke file: %v", err)
	}
	defer func() { _ = file.Close() }()
	writeSmokeHead(file, admitted, checkout)

	page := partner.Page{
		Section: "Backlog", Path: "/backlog", View: "board",
		Label: "Backlog · board", Lanes: smokeLanes(state),
	}
	for at, asked := range smokeQuestions {
		fmt.Printf("asking %d of %d: %s\n", at+1, len(smokeQuestions), asked.text)
		events, stop := service.Subscribe()
		turn, err := service.Submit(context.Background(), "Wido",
			"smoke-"+fmt.Sprint(at), asked.text, page)
		if err != nil {
			stop()
			fmt.Fprintf(file, "\n## %d. %s\n\n**%s**\n\nThe turn was refused: %v\n",
				at+1, asked.kind, asked.text, err)
			continue
		}
		waitForTurn(events)
		stop()
		writeSmokeAnswer(file, service, at+1, asked.kind, asked.text, turn)
	}

	fmt.Println()
	fmt.Println("The smoke file is at " + out)
	fmt.Println("A HUMAN READS IT. It is smoke and not certification: a dozen answers cannot")
	fmt.Println("establish expertise. Read it for contradictions, for claims nothing supports,")
	fmt.Println("for a citation that does not say what the answer says it says, and for any")
	fmt.Println("page or act the Partner offered that this build does not have.")
}

// waitForTurn reads one turn's beats up to its terminal one.
func waitForTurn(events <-chan partner.Event) {
	for event := range events {
		switch event.Kind {
		case partner.EventDone, partner.EventError, partner.EventStopped:
			return
		}
	}
}

func writeSmokeHead(file *os.File, admitted partner.Runtime, checkout string) {
	fmt.Fprintf(file, "# Project Partner smoke, %s\n\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(file, "- Runtime: %s\n- Model: %s\n- Read-only by: %s\n- Fixture checkout: %s\n",
		admitted.Name, admitted.Model, admitted.ReadOnly, checkout)
	fmt.Fprintf(file, "- Tool server: %s %s\n\n", admitted.Tools.Command, strings.Join(admitted.Tools.Args, " "))
	fmt.Fprint(file, "This is smoke and not certification. A dozen answers cannot establish that this\n"+
		"Partner is an expert on this interface or on the metasystem. What a human reads it\n"+
		"for is what a deterministic test cannot see: an answer that contradicts itself or\n"+
		"another answer, a claim nothing in the sources supports, a citation that does not\n"+
		"say what the answer says it says, a page or an act offered that this build does not\n"+
		"have, and any suggestion that the Partner could write.\n")
}

func writeSmokeAnswer(file *os.File, service *partner.Service, number int, kind, question, turn string) {
	read, err := service.Snapshot("Wido", 100)
	if err != nil {
		fmt.Fprintf(file, "\n## %d. %s\n\n**%s**\n\nThe conversation could not be read back: %v\n",
			number, kind, question, err)
		return
	}
	fmt.Fprintf(file, "\n## %d. %s\n\n**%s**\n\n", number, kind, question)
	for _, message := range read.Messages {
		if message.Turn != turn || message.Role != partner.RolePartner {
			continue
		}
		fmt.Fprintf(file, "%s\n\n", strings.TrimSpace(message.Text))
		fmt.Fprintf(file, "- Outcome: %s %s\n", message.Outcome, message.Detail)
		if len(message.Looked) == 0 {
			fmt.Fprint(file, "- Read nothing beyond the page it was given.\n")
		}
		for _, look := range message.Looked {
			fmt.Fprintf(file, "- Read: %s — from %s — %s\n", look.What, look.Source, look.Outcome)
		}
		for _, line := range message.Activity {
			fmt.Fprintf(file, "- Activity: %s\n", line)
		}
	}
}

// smokeLanes is the board as a page would say it is showing it, so the last
// question has a page to be about. It is composed from the fixture's own
// projection, which is what the board renders from.
func smokeLanes(state *ledger) []partner.Lane {
	observed := state.observe()
	if observed.State != snapshot.StateRead || observed.Tree == nil {
		return nil
	}
	board := backlog.Project(observed.Tree, observed.Horizon, observed.Admission)
	held := map[backlog.Lane][]string{}
	for _, row := range board.Rows {
		held[row.Lane] = append(held[row.Lane], row.ID)
	}
	lanes := make([]partner.Lane, 0, len(backlog.LaneOrder))
	for _, lane := range backlog.LaneOrder {
		rows := held[lane]
		if len(rows) == 0 {
			continue
		}
		lanes = append(lanes, partner.Lane{
			ID: string(lane), Title: string(lane), Total: len(rows), Goals: rows,
		})
	}
	return lanes
}

// smokeCheckout is the walkthrough's own fixture checkout with this kit's own
// documents copied into it and a Git repository around it, so the engine's
// tool server resolves the same three roots a real seat does.
//
// It is a copy and a temporary directory: a real agent runs against it, and
// nothing it could do reaches the repository a human is working in.
func smokeCheckout(kit string) string {
	checkout := fixtureCheckout(false, registerKit)
	for _, relative := range []string{
		"AGENTS.md", "wow.md", "docs/glossary.md", "docs/backlog-mechanism.md",
		"memory/rulings.md", "docs/collaboration.md", "docs/project-rules.md",
	} {
		copyInto(kit, checkout, relative)
	}
	copyTree(filepath.Join(kit, "skills"), filepath.Join(checkout, "skills"))
	// The homes the resolver reads the project's memory out of. The fixture
	// plants designs already; these give the index its books and its register.
	for relative, body := range map[string]string{
		"docs/intent/index.md": "# Intent\n\n- Kind: intent\n- Id: 01SMOKEINTENT\n- Status: accepted\n\n" +
			"This fixture workspace exists so a real Partner can be asked a dozen questions " +
			"without touching a real checkout.\n",
		"docs/doctrine/index.md": "# Doctrine\n\n- Kind: doctrine\n- Id: 01SMOKEDOCTRINE\n- Status: accepted\n\n" +
			"Knowledge is kept where it is true, and nothing is copied into a manual for the agent.\n",
		"docs/decisions/one-client.md": "# One ACP client\n\n- Kind: decision\n- Id: 01SMOKEDECISION\n- Status: accepted\n\n" +
			"The seam between this interface and any agent is the Agent Client Protocol, and there is one client.\n",
		"memory/questions.md": "# Open questions\n\n" +
			"| id | opened | question | goals | status |\n|---|---|---|---|---|\n" +
			"| Q-1 | 2026-09-20 | Which runtime should answer as the Partner by default? |  | open |\n",
	} {
		plant(checkout, relative, body)
	}
	initRepository(checkout)
	return checkout
}

func copyInto(from, to, relative string) {
	body, err := os.ReadFile(filepath.Join(from, filepath.FromSlash(relative)))
	if err != nil {
		log.Printf("the smoke checkout has no %s: %v", relative, err)
		return
	}
	plant(to, relative, string(body))
}

func copyTree(from, to string) {
	entries, err := os.ReadDir(from)
	if err != nil {
		log.Printf("the smoke checkout has no %s: %v", from, err)
		return
	}
	for _, entry := range entries {
		source, target := filepath.Join(from, entry.Name()), filepath.Join(to, entry.Name())
		if entry.IsDir() {
			copyTree(source, target)
			continue
		}
		body, err := os.ReadFile(source)
		if err != nil {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			continue
		}
		_ = os.WriteFile(target, body, 0o644)
	}
}

func plant(root, relative, body string) {
	full := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		log.Fatalf("cannot make the smoke checkout: %v", err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		log.Fatalf("cannot plant %s: %v", relative, err)
	}
}

// initRepository is what the engine's root resolution needs: the checkout has
// to be a Git repository for the tool server to derive the three roots.
func initRepository(checkout string) {
	command := exec.Command("git", "init", "--quiet")
	command.Dir = checkout
	if err := command.Run(); err != nil {
		log.Fatalf("cannot make the smoke checkout a repository: %v", err)
	}
}
