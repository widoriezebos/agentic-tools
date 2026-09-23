// Command walkthrough serves the committed interface bundle over a canned
// backlog so the board can be driven in a real browser without a ledger, a
// terminal enrolment, or the checkout's own state.
//
// It exists for the walkthrough and nothing else: it is not wired into the
// engine, it publishes nothing, and its two acts are recorded rather than
// performed. `-proven` chooses which server the board is talking to — one
// that can act as the human, or one an agent started.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/backlog"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalbudget"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/act"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/httpd"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/project"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/session"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/snapshot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/web"
)

const agentReason = "the interface was started by an agent process (claude-code); start it from your own terminal with bin/metasystem ui restart to act as yourself"

func main() {
	listen := flag.String("listen", "127.0.0.1:7979", "loopback address")
	proven := flag.Bool("proven", false, "serve as a server that proved its human at boot")
	// The walkthrough's own one-time-code secret. It is synthetic — the base32
	// string every TOTP example uses — so the sign-in sheet can be walked
	// through without a configured seat and without a real secret anywhere
	// near it. bin/metasystem channel fake code --secret <this> prints the
	// code it accepts.
	secret := flag.String("secret", "JBSWY3DPEHPK3PXP", "synthetic one-time-code secret for the sign-in walkthrough")
	human := flag.String("human", "", "the handle this fixture seat signs in as; empty makes the sheet ask")
	// How often the fixture steward says something new. The notification
	// panel reads a journal, and a journal that never grows shows only its
	// history; this is what makes the live path — the stream, the toasts, the
	// bell's count — something a human can stand in front of and watch.
	notifyEvery := flag.Duration("notify-every", 0, "append a fixture notification this often; zero appends none")
	flag.Parse()

	manifest, err := web.ReadManifest()
	if err != nil {
		log.Fatalf("this executable carries no bundle: %v", err)
	}
	state := newLedger()
	// A checkout with one document in it, so the document reader and the
	// in-place editor have something real to open: the editor writes to disk,
	// reads it back, and answers what is there, and a walkthrough over a
	// canned payload would prove none of that.
	checkout := fixtureCheckout()
	fmt.Println("checkout " + checkout)
	roots := project.Roots{Checkout: checkout, Installation: checkout, StateRoot: checkout}
	state.roots = roots
	authority := httpd.AuthorityInfo{Reason: agentReason}
	if *proven {
		authority = httpd.AuthorityInfo{Proven: true, Human: "Wido"}
	}
	// The fixture's floor is in memory and says so: there is no checkout
	// under this server to keep one in, and a walkthrough that refused to
	// sign anybody in would walk nobody through anything.
	var floor int64
	var remembered = *human
	sessions := session.New(session.Options{
		Root: "/walkthrough", Human: *human, Lifetime: 12 * time.Hour,
		Secret: func() (string, error) { return *secret, nil },
		Floor:  func() (int64, string, error) { return floor, remembered, nil },
		Record: func(lastStep int64, named string) error {
			floor, remembered = lastStep, named
			return nil
		},
	})
	// The steward's journal, planted with a dozen entries across the four
	// sources — including one the notifier refused, which is the delivery gate
	// made visible — and then, with -notify-every, grown while the server runs.
	journal := fixtureJournal(checkout)
	if *notifyEvery > 0 {
		go appendFixtureNotifications(journal, *notifyEvery)
	}
	info := httpd.Info{
		Checkout: "/walkthrough", StartedAt: time.Now().UTC().Format(time.RFC3339),
		EngineBuild: "walkthrough", BundleDigest: manifest.SourceDigest,
		NotificationJournal: journal,
		Observe:             state.observe,
		Authority:           authority,
		Sessions:            sessions,
		// The walkthrough's acts ignore the hand that reached them: it has
		// no ledger to record one in, and the point of this server is the
		// board rather than the proof.
		Approve: func(_ *session.Session, id string, budget goalbudget.Budget) error {
			return state.approve(id, budget)
		},
		Withdraw: func(_ *session.Session, id, reason string) error { return state.withdraw(id, reason) },
		SetPriority: func(_ *session.Session, id string, priority uint8, sequence *uint64) error {
			return state.setPriority(id, priority, sequence)
		},
		Open:    func(_ *session.Session, opened act.Opened) error { return state.open(opened) },
		Project: func() (project.Pane, error) { return state.project(), nil },
		// The document reader and the in-place editor, over the fixture
		// checkout, through the same package the engine wires.
		Document: func(id string) (project.Document, error) {
			return project.Read(roots, id, time.Now().UTC())
		},
		EditDocument: func(id, source, revision string) (project.Document, error) {
			return project.EditDocument(roots, id, source, revision, time.Now().UTC())
		},
		// The two writes a design's own page makes, over the fixture checkout
		// and through the same package the engine wires: marking a design done
		// rewrites its Status line, and naming a goal on it rewrites its Goals
		// line. The goals the fixture ledger carries are the ones planted in
		// it; a goal this server's own intake act just opened lives in the
		// canned tree rather than in that ledger, so naming it is refused —
		// which is the path the page says the goal is not lost on.
		SetStatus: func(id, status string) (project.Written, error) {
			return project.SetStatus(roots, id, status)
		},
		AddRecordGoal: func(id, goal string) (project.Document, error) {
			return project.AddGoal(roots, id, goal, time.Now().UTC())
		},
		PreviewDocument: project.PreviewDocument,
		BudgetDefaults: func() (map[string]goalbudget.Budget, error) {
			return map[string]goalbudget.Budget{"3": {
				ElapsedLimit: "8h", AttemptLimit: 10, ReservedJobMinutesLimit: 1200, ActiveJobLimit: 1, ReviewRoundLimit: 3,
			}}, nil
		},
	}
	listener, err := net.Listen("tcp", *listen)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("ready http://" + listener.Addr().String())
	server := &http.Server{Handler: httpd.New(info, listener.Addr(), web.Dist()), ReadHeaderTimeout: 5 * time.Second}
	log.Fatal(server.Serve(listener))
}

// ledger is the canned tree the board reads, and the two acts change it the
// way the engine would: an approval moves a goal to approved, a withdrawal
// returns it to queued. Everything else refuses.
//
// roots is the fixture checkout beneath it. The Project pane is canned, but
// two of the designs in it are real files this server's own writes change, so
// their head is read from disk rather than remembered: marking one done on its
// page has to show up in the listing the next read of it makes.
type ledger struct {
	tree  *goal.TreeGoals
	roots project.Roots
	// opened are the goals this server's own intake act made, in the order it
	// made them, so the Project pane carries them beside the canned ones and a
	// design that names one can show where it stands.
	opened []string
}

// The fixture checkout's four documents.
//
// One is plain: it declares no head, so the editor can be walked through over
// a file the resolver has nothing to say about, and it carries one of each
// block the reader renders so the preview has something to show. The second is
// a record in a real home, over a real ledger goal, so that saving a head the
// project refuses can be walked through as well — which is the one refusal
// with something to show on the page.
//
// The last two are the two readings of a design's work. One names a goal that
// landed and a goal that has not, so its page counts what is in and offers
// nothing; the other names only goals that landed, so its page says so and
// offers the one act that follows.
const (
	walkthroughDocument = "docs/reading.md"
	walkthroughRecord   = "plans/designs/reading.md"
	walkthroughPartly   = "plans/designs/reader.md"
	walkthroughLanded   = "plans/designs/shell.md"
	walkthroughText     = `# Reading and editing in place

A document is read here as a chapter of a book rather than as a file, and from
this slice it is edited here too: the article becomes a text area holding the
source, and the save refuses to write over a file that changed underneath.

## What the editor is

- A plain text area over the Markdown, in the monospace face.
- A preview, rendered by the engine through the same parser the reader uses.
- A save that carries the revision the file was opened at.

## What it is not

There is no toolbar of formatting buttons and no second, richer editing
surface. The text is the document:

    - Kind: design
    - Status: draft

Editing the words is editing the file.
`
	walkthroughHead = `# The reading pane

- Kind: design
- Id: design-reading
- Status: draft
- Goals: reading-pane

## Outcome

A document is read as a chapter of a book rather than as a file.

## Verification

Change the status above to something the grammar does not carry, and the save
is refused with the problem the check verb would print.
`
	walkthroughPartlyText = `# The document reader

- Kind: design
- Id: design-reader
- Status: accepted
- Goals: g1-s9 g1-s13

## Outcome

A document is read among its siblings, with its outline beside it.
`
	walkthroughLandedText = `# The application shell

- Kind: design
- Id: design-shell
- Status: accepted
- Goals: g1-s9 g1-s10

## Outcome

The rail, the header and the work area are one shell every section is read in.
`
)

// fixtureCheckout makes the walkthrough's own checkout in a temporary
// directory and plants the two documents above in it, in a layout the
// resolver reads: a configuration file, an agents directory, a one-goal
// ledger, and a design home. It is thrown away with the temporary directory,
// so a walkthrough that saves over a file changes nothing a human keeps.
func fixtureCheckout() string {
	directory, err := os.MkdirTemp("", "metasystem-walkthrough-")
	if err != nil {
		log.Fatalf("cannot make the walkthrough checkout: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(directory, "scripts", "agents"), 0o755); err != nil {
		log.Fatalf("cannot make the walkthrough checkout: %v", err)
	}
	for _, planted := range []struct{ relative, text string }{
		{"metasystem.conf", ""},
		{"plans/goals/backlog.md", "# backlog\n\n- SyncMode: local\n"},
		{"plans/goals/reading-pane.md", "# reading-pane\n\n- State: approved\n- Intent: The pane reads a document as a chapter\n"},
		// The two goals that landed live where the ledger keeps concluded
		// work, so the resolver validates the two designs below against the
		// same two homes the pane reads them out of.
		{"plans/goals/g1-s13.md", "# g1-s13\n\n- State: queued\n- Intent: The goal page reads the whole record\n"},
		{"records/goals/g1-s9.md", "# g1-s9\n\n- State: done\n- Intent: The application shell, the rail and the header\n"},
		{"records/goals/g1-s10.md", "# g1-s10\n\n- State: done\n- Intent: The backlog's data path and the list\n"},
		{walkthroughDocument, walkthroughText},
		{walkthroughRecord, walkthroughHead},
		{walkthroughPartly, walkthroughPartlyText},
		{walkthroughLanded, walkthroughLandedText},
	} {
		full := filepath.Join(directory, filepath.FromSlash(planted.relative))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			log.Fatalf("cannot make the walkthrough checkout: %v", err)
		}
		if err := os.WriteFile(full, []byte(planted.text), 0o644); err != nil {
			log.Fatalf("cannot plant %s: %v", planted.relative, err)
		}
	}
	return directory
}

// newLedger builds the canned tree.
//
// It carries one of everything the board has a reading for, because a
// walkthrough that shows only the happy lane proves only the happy lane: a
// priority band deep enough to reorder within, two seats rather than one, a
// planning arc, a split with its members and its retired parent, conclusions
// at three different ages so the Done lane's window has something to do, an
// abandoned goal, and one record whose state this build cannot place.
func newLedger() *ledger {
	tree := &goal.TreeGoals{
		Root:      &goal.RootRecord{Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "2", SyncMode: goal.SyncLocal, Revision: 1},
		Live:      map[string]*goal.GoalFile{},
		Done:      map[string]*goal.GoalFile{},
		Abandoned: map[string]*goal.GoalFile{},
	}
	add := func(file *goal.GoalFile) *goal.GoalFile {
		switch file.State {
		case goal.StateDone:
			tree.Done[file.Id] = file
		case goal.StateAbandoned:
			tree.Abandoned[file.Id] = file
		default:
			tree.Live[file.Id] = file
		}
		return file
	}

	// To Do, four deep in priority 2, which is the band a reorder moves
	// inside, plus one at priority 1 to drag across a band.
	add(ranked(walkthroughGoal("g1-s12", goal.StateQueued, "The board, and approve and withdraw by drag"), 2, 1))
	add(ranked(walkthroughGoal("g1-s13", goal.StateQueued, "The goal page reads the whole record"), 2, 2))
	add(ranked(walkthroughGoal("g1-s16", goal.StateQueued, "The Overview reads the fleet's standing"), 2, 3))
	add(ranked(walkthroughGoal("g1-s17", goal.StateQueued, "Decisions are recorded from the browser"), 2, 4))
	add(ranked(walkthroughGoal("g1-s18", goal.StateQueued, "The application section reads the build"), 1, 1))

	ready := add(ranked(walkthroughGoal("g1-s14", goal.StateApproved, "The Fleet section reads the seats"), 2, 5))
	ready.Approved = &goal.ApprovalRecord{By: "human:Wido", At: "2026-09-20T09:00:00Z", Authority: goal.ApprovalAuthorityProven, Revision: 3}
	ready.Budget = &goalbudget.Budget{ElapsedLimit: "4h", AttemptLimit: 6, ReservedJobMinutesLimit: 720, ActiveJobLimit: 1, ReviewRoundLimit: 2}

	claimed := add(ranked(walkthroughGoal("g1-s15", goal.StateClaimed, "The Decisions section answers a question"), 2, 6))
	claimed.Claimed = &goal.ClaimRecord{Machine: "m1e", Lineage: "coordinator", At: "2026-09-21T08:00:00Z"}
	second := add(ranked(walkthroughGoal("g1-s19", goal.StateClaimed, "The document reader anchors a heading"), 2, 7))
	second.Claimed = &goal.ClaimRecord{Machine: "m2a", Lineage: "implementer", At: "2026-09-21T10:00:00Z"}

	// A planning arc, which is an arc and not a split.
	add(arced(ranked(walkthroughGoal("harvest-1", goal.StateQueued, "Harvest the covenant survey's findings"), 3, 1), "covenant-harvest"))
	add(arced(ranked(walkthroughGoal("harvest-2", goal.StateQueued, "Rule on the survey's open questions"), 3, 2), "covenant-harvest"))

	// A split: the parent is concluded and retired by the root record, and
	// each member is born carrying the parent's id as its arc.
	parent := add(concluded(walkthroughGoal("g1-s20", goal.StateDone, "The Project section reads the checkout"), time.Now().UTC().Add(-10*24*time.Hour)))
	parent.Conclude = "decomposed into arc g1-s20: goal:g1-s20a, goal:g1-s20b"
	tree.Root.Decomposed = []goal.DecomposedEntry{{Id: parent.Id, Opid: "op-split", At: "2026-09-12T00:00:00Z"}}
	add(arced(ranked(walkthroughGoal("g1-s20a", goal.StateQueued, "The briefing opens each kind in its own words"), 2, 8), parent.Id))
	add(arced(ranked(walkthroughGoal("g1-s20b", goal.StateQueued, "The document reader walks its siblings"), 2, 9), parent.Id))

	// Conclusions at three ages, so the window select has something to do.
	// They are stamped back from this process's own clock rather than
	// written down, because a fixed date drifts out of every window the day
	// after it is written and the walkthrough would then show an empty lane.
	now := time.Now().UTC()
	add(concluded(walkthroughGoal("g1-s9", goal.StateDone, "The application shell, the rail and the header"), now.Add(-6*time.Hour)))
	add(concluded(walkthroughGoal("g1-s10", goal.StateDone, "The backlog's data path and the list"), now.Add(-4*24*time.Hour)))
	add(concluded(walkthroughGoal("g1-s8", goal.StateDone, "The frontend toolchain and the committed bundle"), now.Add(-40*24*time.Hour)))

	dropped := add(walkthroughGoal("g1-s7", goal.StateAbandoned, "A second bundler beside the first"))
	dropped.Abandoned = &goal.AbandonRecord{By: "human:Wido", At: "2026-09-05T00:00:00Z", Because: "overtaken by g1-s8"}

	// One record this build cannot place, so the line above the lanes has
	// something to say and can be opened.
	add(ranked(walkthroughGoal("g1-s99", "surveying", "A state this build has no lane for"), 3, 3))

	return &ledger{tree: tree}
}

func walkthroughGoal(id, state, intent string) *goal.GoalFile {
	return &goal.GoalFile{
		Id: id, State: state, Intent: intent, Origin: "main", Tier: 3, Priority: 2, Sequence: 1,
		NextStep: "Start " + id + ".", OpenedAt: "2026-09-01T00:00:00Z", Revision: 3,
		History: []goal.HistoryLine{{At: "2026-09-01T00:00:00Z", Opid: "op-open", Verb: "open", Actor: "m1e+coordinator"}},
	}
}

func ranked(file *goal.GoalFile, priority uint8, sequence uint64) *goal.GoalFile {
	file.Priority, file.Sequence = priority, sequence
	return file
}

func arced(file *goal.GoalFile, arc string) *goal.GoalFile {
	file.Arc = arc
	return file
}

// concluded writes the `done` History line the Done lane's window reads. The
// state alone does not carry a date, which is the whole reason the row has
// one.
func concluded(file *goal.GoalFile, at time.Time) *goal.GoalFile {
	file.Priority, file.Sequence = 0, 0
	file.Conclude = "landed"
	file.History = append(file.History, goal.HistoryLine{
		At: at.Format(time.RFC3339), Opid: "op-done-" + file.Id, Verb: "done", Actor: "m1e+coordinator",
	})
	return file
}

func (l *ledger) observe() snapshot.Observation {
	now := time.Now().UTC()
	horizon := goal.NewApprovalHorizon(l.tree, now)
	return snapshot.Observation{
		ObservedAt: now, StateRoot: "/walkthrough", State: snapshot.StateRead,
		Tip: "c5d517f427e35e19c2944ab9aefee4d9c9cd9e6c", CommittedAt: now.Add(-time.Minute),
		Tree: l.tree, Horizon: horizon, SyncMode: goal.SyncLocal,
		// The claim gate cannot be asked about a tree with no repository
		// behind it, so the walkthrough answers it directly: every approved
		// goal is one a seat could claim.
		Admission: l.admission(),
		Fetch: snapshot.FetchState{
			Outcome: snapshot.OutcomeCurrent, StartedAt: now.Add(-2 * time.Second), FinishedAt: now.Add(-time.Second),
			Tip: "c5d517f427e35e19c2944ab9aefee4d9c9cd9e6c", Detail: "already at the canonical tip",
			Cadence: snapshot.CadenceConnected, NextAt: now.Add(5 * time.Second),
		},
	}
}

func (l *ledger) admission() backlog.Admission {
	admission := backlog.Admission{
		Answered: true,
		Ready:    map[string]bool{}, Blocked: map[string]bool{}, Awaiting: map[string]bool{},
		Refused: map[string]string{},
	}
	for id, file := range l.tree.Live {
		if file.State == goal.StateApproved {
			admission.Ready[id] = true
		}
	}
	return admission
}

func (l *ledger) approve(id string, budget goalbudget.Budget) error {
	file := l.tree.Live[id]
	if file == nil {
		return fmt.Errorf("goal %s is not live", id)
	}
	if file.State != goal.StateQueued {
		return fmt.Errorf("goal %s is %s; approve admits queued work", id, file.State)
	}
	file.State = goal.StateApproved
	file.Budget = &budget
	file.Approved = &goal.ApprovalRecord{
		By: "human:Wido", At: time.Now().UTC().Format(time.RFC3339),
		Authority: goal.ApprovalAuthorityProven, Revision: file.Revision,
	}
	return nil
}

// walkthroughTitles are the headings the fixture's goal files carry. A ledger
// whose goal file is titled by its id reads by that id; these are titled, so
// the sheet's About line has both halves to show.
var walkthroughTitles = map[string]string{
	"g1-s15": "The Decisions section",
	"g1-s12": "The backlog board",
	"g1-s13": "The goal page",
	"g1-s9":  "The application shell",
	"g1-s10": "The backlog's data path",
}

// paneGoals are the goals the Project pane carries, in the order the ledger
// lists them: the live ones first, then the ones that concluded. The concluded
// ones are here because a design's work is read out of them — a payload that
// carried only live goals would show a shipped design as a design naming a
// goal nobody has heard of.
var paneGoals = []string{"g1-s15", "g1-s12", "g1-s13", "g1-s9", "g1-s10"}

// goalFile is one goal of the fixture tree, live or concluded.
func (l *ledger) goalFile(id string) *goal.GoalFile {
	if file := l.tree.Live[id]; file != nil {
		return file
	}
	if file := l.tree.Done[id]; file != nil {
		return file
	}
	return l.tree.Abandoned[id]
}

// project is the canned Project pane the goal page reads. It carries what a
// goal page is about — the ledger's goals, one of them sliced, two designs that
// name a goal, one of which records a slice plan and one of which does not, and
// two decisions, one about the project as a whole and one about a goal — so the
// Slices tab can be driven with something in it and with nothing in it, and the
// Decisions tab with rows on both pages. Nothing is planted in the two books or
// in the register, so the tabs that have nothing say so and offer their act.
func (l *ledger) project() project.Pane {
	goals := []project.Goal{}
	for _, id := range append(append([]string{}, paneGoals...), l.opened...) {
		file := l.goalFile(id)
		if file == nil {
			continue
		}
		one := project.Goal{ID: id, Title: walkthroughTitles[id], State: file.State, Intent: file.Intent}
		if id == "g1-s15" {
			one.Sliced = &project.Sliced{
				At: "2026-09-21T08:30:00Z", Machine: "m1e", Lineage: "coordinator",
			}
		}
		goals = append(goals, one)
	}
	return project.Pane{
		SchemaVersion: project.SchemaVersion,
		ReadAt:        time.Now().UTC().Format(time.RFC3339),
		Goals:         goals,
		Records: []project.Record{
			{
				Kind: "design", ID: "design-decisions", Status: "accepted", Goals: []string{"g1-s15"},
				Title: "The Decisions section", Path: "plans/designs/decisions.md", Home: "plans/designs",
				Summary: "How a question is answered from the browser.",
				Slices: []string{
					"The register is read and shown with its open questions first",
					"Answering one publishes through the project writer",
					"A question a human asks arrives in the same register",
				},
			},
			{
				Kind: "design", ID: "design-board", Status: "accepted", Goals: []string{"g1-s12"},
				Title: "The backlog board", Path: "plans/designs/board.md", Home: "plans/designs",
				Summary: "The board, and the acts on it.",
				Slices: []string{
					"The lanes read the projection",
					"A drop between lanes publishes the verb it names",
				},
			},
			// A design that governs a goal and records no plan, so the tab has
			// a goal to say "no slice plan is recorded" about.
			{
				Kind: "design", ID: "design-goal-page", Status: "draft", Goals: []string{"g1-s13"},
				Title: "The goal page", Path: "plans/designs/goal-page.md", Home: "plans/designs",
				Summary: "What one goal's page shows.", Slices: []string{},
			},
			// The two readings of a design's work: one part of the way there,
			// and one whose goals have all landed, which is the one that is
			// offered the act that follows.
			l.asItReads(project.Record{
				Kind: "design", ID: "design-reader", Status: "accepted", Goals: []string{"g1-s9", "g1-s13"},
				Title: "The document reader", Path: walkthroughPartly, Home: "plans/designs",
				Summary: "A document is read among its siblings, with its outline beside it.",
				Slices:  []string{},
			}),
			l.asItReads(project.Record{
				Kind: "design", ID: "design-shell", Status: "accepted", Goals: []string{"g1-s9", "g1-s10"},
				Title: "The application shell", Path: walkthroughLanded, Home: "plans/designs",
				Summary: "The rail, the header and the work area are one shell every section is read in.",
				Slices:  []string{},
			}),
			// Two decisions: one about the project as a whole, which is what a
			// head with no Goals line means, and one about a goal, so the
			// Decisions tab has rows on the Project page and on a goal's.
			{
				Kind: "decision", ID: "decision-one-binary", Status: "accepted", Goals: []string{},
				Title: "One binary", Path: "docs/decisions/0001-one-binary.md", Home: "docs/decisions",
				Summary: "The engine ships as one executable, and the interface is served from it.",
				Slices:  []string{},
			},
			{
				Kind: "decision", ID: "decision-answering", Status: "draft", Goals: []string{"g1-s15"},
				Title: "A question is answered by a record", Path: "docs/decisions/0002-answering.md",
				Home:    "docs/decisions",
				Summary: "Answering names the record that answered it, and the row keeps the name.",
				Slices:  []string{},
			},
		},
		Intent:    project.Book{Chapters: []project.Chapter{}},
		Doctrine:  project.Book{Chapters: []project.Chapter{}},
		Questions: []project.Question{},
		Problems:  []project.Problem{},
		Documents: []project.File{
			{Path: walkthroughDocument, Title: "Reading and editing in place"},
			{Path: walkthroughRecord, Title: "The reading pane"},
			{Path: walkthroughPartly, Title: "The document reader"},
			{Path: walkthroughLanded, Title: "The application shell"},
		},
	}
}

// asItReads is one canned row with the head its own file carries right now.
//
// The rest of this pane is invented, but these rows stand for files on disk
// that this server's own writes rewrite, and a listing that kept saying what
// the file said at boot would make a write that worked look like one that did
// not. A file that cannot be read leaves the row as it was written here.
func (l *ledger) asItReads(record project.Record) project.Record {
	document, err := project.Read(l.roots, record.Path, time.Now().UTC())
	if err != nil || document.Record == nil {
		return record
	}
	record.Status = document.Record.Status
	record.Goals = document.Record.Goals
	return record
}

// setPriority re-ranks the way internal/goal/order.go does, because the point
// of the walkthrough is to see what the engine's own renumbering looks like on
// a board: the destination band is the live goals at that priority without
// this one, a requested position outside 1..len+1 is refused outright rather
// than clamped, and inserting renumbers the whole band — and the band the goal
// left, when it changed priority.
func (l *ledger) setPriority(id string, priority uint8, sequence *uint64) error {
	file := l.tree.Live[id]
	if file == nil {
		return fmt.Errorf("goal %s is not live", id)
	}
	from := file.Priority
	destination := l.band(priority, id)
	position := uint64(len(destination) + 1)
	if sequence != nil {
		position = *sequence
	} else if from == priority {
		position = file.Sequence
	}
	if maximum := uint64(len(destination) + 1); position < 1 || position > maximum {
		return fmt.Errorf("goal %s sequence %d is outside the current destination range 1..%d for priority %d",
			id, position, maximum, priority)
	}
	if from == priority && file.Sequence == position {
		return fmt.Errorf("the requested priority and sequence already hold")
	}
	if from != 0 && from != priority {
		number(l.tree.Live, from, l.band(from, id))
	}
	at := int(position - 1)
	destination = append(destination, "")
	copy(destination[at+1:], destination[at:])
	destination[at] = id
	number(l.tree.Live, priority, destination)
	return nil
}

// band is the ids at one priority, in sequence order, without the one named.
func (l *ledger) band(priority uint8, without string) []string {
	ids := []string{}
	for id, file := range l.tree.Live {
		if file.Priority == priority && id != without {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return l.tree.Live[ids[i]].Sequence < l.tree.Live[ids[j]].Sequence })
	return ids
}

func number(live map[string]*goal.GoalFile, priority uint8, ids []string) {
	for at, id := range ids {
		live[id].Priority, live[id].Sequence = priority, uint64(at+1)
	}
}

// open creates the queued goal the intake act creates, refusing the two
// things the engine refuses first: an id that is taken, and risk answers that
// are not four answers with a basis. The new goal is appended to priority 3,
// because a goal nobody has ranked is not urgent and the engine appends.
func (l *ledger) open(opened act.Opened) error {
	if l.tree.Exists(opened.ID) {
		return fmt.Errorf("goal %s already exists", opened.ID)
	}
	if err := opened.Risk.Validate(); err != nil {
		return err
	}
	tier := opened.Tier
	if tier == 0 {
		tier = opened.Risk.DerivedTier()
	}
	risk := opened.Risk
	file := walkthroughGoal(opened.ID, goal.StateQueued, opened.Intent)
	file.Origin = goal.OriginHuman
	file.NextStep = opened.NextStep
	file.Tier = tier
	file.Labels = opened.Labels
	file.Risk = &risk
	file.Priority, file.Sequence = 0, 0
	l.tree.Live[opened.ID] = file
	if err := l.setPriority(opened.ID, 3, nil); err != nil {
		return err
	}
	l.plantGoal(opened.ID, opened.Intent)
	return nil
}

// plantGoal writes the goal into the fixture checkout's own ledger as well.
//
// This server carries two of them: the canned tree the board reads, and the
// checkout the project writer writes into. They are one ledger everywhere
// else, so an intake that wrote to only the first would make the design page's
// own append fail against a rule — a Goals line names a goal the ledger has —
// that the real engine would have satisfied. Failing to plant it is not an
// error here: the goal is open on the board either way, and the page says what
// it could not name.
func (l *ledger) plantGoal(id, intent string) {
	if l.roots.StateRoot == "" || strings.ContainsAny(id, `/\`) {
		return
	}
	full := filepath.Join(l.roots.StateRoot, "plans", "goals", id+".md")
	_ = os.WriteFile(full, []byte("# "+id+"\n\n- State: queued\n- Intent: "+intent+"\n"), 0o644)
	l.opened = append(l.opened, id)
}

func (l *ledger) withdraw(id, reason string) error {
	file := l.tree.Live[id]
	if file == nil || file.Approved == nil {
		return fmt.Errorf("goal %s carries no approval to withdraw", id)
	}
	file.State = goal.StateQueued
	file.Approved, file.Budget = nil, nil
	_ = reason
	return nil
}

// The walkthrough's notification journal.
//
// A dozen entries across the four sources and across three days, so the
// panel's day headings have something to head and its rows have something to
// differ by, and two of them undelivered, so the line a human most needs to
// see — the steward tried and the channel refused — is on the page rather
// than described in a design.
//
// It is written into the walkthrough's own throwaway checkout, under the path
// the steward would have written it to, and read through the same package the
// engine reads the real one with.
func fixtureJournal(checkout string) string {
	path := filepath.Join(checkout, "artifacts", "agents", "steward", "notifications.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		log.Fatalf("cannot make the walkthrough journal: %v", err)
	}
	now := time.Now().UTC()
	planted := []struct {
		ago       time.Duration
		source    string
		ref       string
		message   string
		delivered bool
		problem   string
	}{
		{50 * time.Hour, "steward", "", "steward: the runner is armed and ticking", true, ""},
		{49 * time.Hour, "verdict", "verdict-stalled-alive", "steward: seat m1e+coordinator is stalled but alive \u2014 reviving", true, ""},
		{48 * time.Hour, "steward", "", "steward: reaped 3 finished workers", true, ""},
		{27 * time.Hour, "alert", "alert-9f2c1a7b4e6d8035-1", "HEALTH unhealthy \u2014 the repository watcher is dead", true, ""},
		{26 * time.Hour, "steward", "", "steward: the repository watcher was restarted", true, ""},
		{25*time.Hour + 30*time.Minute, "handoff", "handoff-500000000000000a", "steward: seat m2a+implementer is handing g1-s19 back \u2014 your turn", true, ""},
		{5 * time.Hour, "steward", "", "steward: the accepted ledger advanced to c5d517f", true, ""},
		{4 * time.Hour, "alert", "alert-3b71c0de95a24f18-1", "HEALTH unhealthy \u2014 the steward runner is stale", false,
			"notification not delivered: exit status 1 (osascript is not permitted to send notifications)"},
		{3*time.Hour + 20*time.Minute, "alert", "alert-3b71c0de95a24f18-1", "HEALTH unhealthy \u2014 the steward runner is stale", true, ""},
		{2 * time.Hour, "verdict", "verdict-budget-spent", "steward: g1-s14 has spent its elapsed budget \u2014 the seat is paused", true, ""},
		{40 * time.Minute, "steward", "", "steward: a question is waiting for you on g1-s15", true, ""},
		{6 * time.Minute, "handoff", "handoff-500000000000000b", "steward: seat m1e+coordinator is handing g1-s12 back \u2014 your turn", false,
			"notification not delivered: exit status 127 (terminal-notifier: command not found)"},
	}
	lines := []string{}
	for index, entry := range planted {
		at := now.Add(-entry.ago)
		record := map[string]any{
			"id":        fixtureNotificationID(at, index),
			"at":        at.Format(time.RFC3339),
			"message":   entry.message,
			"source":    entry.source,
			"delivered": entry.delivered,
		}
		if entry.ref != "" {
			record["ref"] = entry.ref
		}
		if !entry.delivered {
			record["error"] = entry.problem
		}
		encoded, err := json.Marshal(record)
		if err != nil {
			log.Fatalf("cannot write the walkthrough journal: %v", err)
		}
		lines = append(lines, string(encoded))
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		log.Fatalf("cannot write the walkthrough journal: %v", err)
	}
	fmt.Println("journal " + path)
	return path
}

// appendFixtureNotifications grows the journal while the server runs, so the
// stream, the toasts and the bell's count can be watched rather than imagined.
// The sources rotate, and every fourth one is undelivered.
func appendFixtureNotifications(path string, every time.Duration) {
	sources := []string{"steward", "alert", "handoff", "verdict"}
	messages := []string{
		"steward: the accepted ledger advanced",
		"HEALTH unhealthy \u2014 the hook has not run in 40 minutes",
		"steward: seat m2a+implementer is handing g1-s20a back \u2014 your turn",
		"steward: g1-s13 has spent its attempt budget",
	}
	for count := 0; ; count++ {
		time.Sleep(every)
		at := time.Now().UTC()
		source := sources[count%len(sources)]
		record := map[string]any{
			"id":        fixtureNotificationID(at, count),
			"at":        at.Format(time.RFC3339),
			"message":   messages[count%len(messages)],
			"source":    source,
			"delivered": count%4 != 3,
		}
		if count%4 == 3 {
			record["error"] = "notification not delivered: exit status 1"
		}
		encoded, err := json.Marshal(record)
		if err != nil {
			continue
		}
		file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			continue
		}
		_, _ = file.Write(append(encoded, '\n'))
		_ = file.Close()
	}
}

// fixtureNotificationID writes an identity the interface can sort: the same
// Crockford base32 encoding of a millisecond timestamp the steward mints, with
// the sequence where the randomness would be, so a walkthrough's ids are
// stable and ordered rather than random.
func fixtureNotificationID(at time.Time, sequence int) string {
	const alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	value := uint64(at.UnixMilli())
	id := make([]byte, 26)
	for index := 25; index >= 0; index-- {
		if index >= 10 {
			id[index] = alphabet[uint64(sequence)&0x1f]
			sequence >>= 5
			continue
		}
		id[index] = alphabet[value&0x1f]
		value >>= 5
	}
	return string(id)
}
