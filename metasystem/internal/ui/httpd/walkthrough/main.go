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
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/fleet"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/httpd"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/overview"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner"
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
	// The calm workspace. Overview's good outcome is a page that says nothing
	// needs you, and a fixture that can only show the busy one can only show
	// half of what the section is for. Calm proves its human, admits every
	// goal, accepts every draft, closes the register, marks the finished
	// design done, and plants a journal the steward delivered — which is the
	// whole of "nothing needs you" and "one line of health".
	calm := flag.Bool("calm", false, "serve a workspace with nothing waiting, which is the page's good outcome")
	// What this fixture's freshness loop is doing. The interface judges its
	// own freshness by that loop, so the chip's three states and the health
	// pill's three states are all reachable by saying what the loop last did:
	// a look that landed a second ago, a look that has not landed for two
	// hours, and a look that failed three times running.
	freshness := flag.String("freshness", string(snapshot.FreshnessCurrent),
		"what this fixture's fetch loop last did: current, behind or failed")
	// How long ago the accepted tip was committed. It is a fixture knob
	// because it is the fact the interface used to warn about and no longer
	// does: a quiet week of commits with a loop that is looking every five
	// seconds is a current board, and this is how that is stood in front of.
	lastChange := flag.Duration("last-change", time.Minute,
		"how long ago the accepted tip was committed, which is a fact and never a warning")
	// The Project Partner, from a canned ACP server. `fake` is not an admitted
	// runtime and never will be: it is this walkthrough's own server, in this
	// process, over a pipe, so the drawer can be driven end to end without an
	// agent, a key, or a network.
	partnerRuntime := flag.String("partner", "", "serve a Project Partner from the canned ACP server: fake")
	// The by-hand check before a release. It serves nothing: it asks a real
	// runtime a dozen questions through the production path, over a temporary
	// copy of this walkthrough's fixture checkout with this kit's own
	// documents in it, and writes what came back for a human to read.
	smoke := flag.String("smoke", "", "ask a real runtime the release questions and write the answers: claude, codex or devin")
	smokeModel := flag.String("smoke-model", "", "the model the smoke run asks for; empty is the runtime's own default")
	smokeEngine := flag.String("smoke-engine", "bin/metasystem", "the metasystem executable that serves the Partner's read tools")
	smokeKit := flag.String("smoke-kit", ".", "the metasystem installation whose glossary, rulings, routes and skills the fixture copies")
	smokeOut := flag.String("smoke-out", "partner-smoke.md", "where the smoke run writes its answers")
	flag.Parse()
	if *smoke != "" {
		runSmoke(*smoke, *smokeModel, *smokeEngine, *smokeKit, *smokeOut)
		return
	}
	if *freshness != string(snapshot.FreshnessCurrent) &&
		*freshness != string(snapshot.FreshnessBehind) &&
		*freshness != string(snapshot.FreshnessFailed) {
		log.Fatalf("-freshness takes current, behind or failed, not %q", *freshness)
	}

	manifest, err := web.ReadManifest()
	if err != nil {
		log.Fatalf("this executable carries no bundle: %v", err)
	}
	state := newLedger(*calm)
	state.calm = *calm
	state.freshness = snapshot.Freshness(*freshness)
	state.lastChange = *lastChange
	// A checkout with one document in it, so the document reader and the
	// in-place editor have something real to open: the editor writes to disk,
	// reads it back, and answers what is there, and a walkthrough over a
	// canned payload would prove none of that.
	checkout := fixtureCheckout(*calm)
	fmt.Println("checkout " + checkout)
	roots := project.Roots{Checkout: checkout, Installation: checkout, StateRoot: checkout}
	state.roots = roots
	authority := httpd.AuthorityInfo{Reason: agentReason}
	if *proven || *calm {
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
	journal := fixtureJournal(checkout, *calm)
	if *notifyEvery > 0 {
		go appendFixtureNotifications(journal, *notifyEvery)
	}
	info := httpd.Info{
		Checkout: "/walkthrough", StartedAt: time.Now().UTC().Format(time.RFC3339),
		EngineBuild: "walkthrough", BundleDigest: manifest.SourceDigest,
		NotificationJournal: journal,
		Observe:             state.observe,
		// The fleet, invented: three machines, one of each standing, joined
		// to the claims the canned ledger carries. -proven is the armed seat.
		Fleet: fixtureFleet(*proven),
		// The browsers holding the stream open, which the fleet event rides
		// back to. This fixture fetches no presence, so nothing announces on
		// it; it exists so the stream registers a connection the way the
		// engine's own server does.
		Watch: fleet.NewWatch(),
		// What the board's Refresh runs before it observes. This fixture has
		// no remote to reach, so the look is recorded rather than made: what
		// it proves in a browser is that pressing Refresh runs one and that
		// the state a human then reads is the state that look left behind.
		Fetch:     state.fetch,
		Authority: authority,
		Sessions:  sessions,
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
		Open: func(_ *session.Session, opened act.Opened) error { return state.open(opened) },
		Block: func(_ *session.Session, dependent, blocker string) error {
			return state.block(dependent, blocker)
		},
		Unblock: func(_ *session.Session, dependent, blocker string) error {
			return state.unblock(dependent, blocker)
		},
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
		SetRecordGoals: func(id string, goals []string) (project.Document, error) {
			return project.SetGoals(roots, id, goals, time.Now().UTC())
		},
		PreviewDocument: project.PreviewDocument,
		// The landing page's marker, over the fixture checkout, through the
		// same package the engine wires: a walkthrough that kept the visit in
		// memory would never show the second visit's window, which is the
		// whole of what the rule is for.
		Visit: func(human string, now time.Time) (time.Time, bool, error) {
			return overview.Visit(checkout, human, now)
		},
		BudgetDefaults: func() (map[string]goalbudget.Budget, error) {
			return map[string]goalbudget.Budget{"3": {
				ElapsedLimit: "8h", AttemptLimit: 10, ReservedJobMinutesLimit: 1200, ActiveJobLimit: 1, ReviewRoundLimit: 3,
			}}, nil
		},
	}
	if *partnerRuntime != "" {
		if *partnerRuntime != "fake" {
			log.Fatalf("-partner takes fake, not %q", *partnerRuntime)
		}
		// The Partner reads what the pages read, so the "Seeing:" sheet shows
		// this fixture's own board rather than an empty block.
		service := fakePartner(checkout, partner.Facts{
			Observe: state.observe,
			Project: func() (project.Pane, error) { return state.project(), nil },
			Document: func(id string) (project.Document, error) {
				return project.Read(roots, id, time.Now().UTC())
			},
		})
		info.Partner = service
		info.PartnerConfigured = true
		defer service.Close()
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
	// calm is the workspace with nothing waiting: every draft accepted, the
	// register closed, and the finished design marked done. It changes what
	// the Project pane answers and nothing about how it is answered.
	calm bool
	// freshness is what this fixture's fetch loop last did, which is what the
	// interface judges its own freshness by. looks counts the Refreshes that
	// ran one, so the walkthrough can show that pressing Refresh looks.
	freshness snapshot.Freshness
	looks     int
	// lastChange is how old the accepted commit is. It reaches the board as a
	// fact in the chip's tooltip and nothing else judges it.
	lastChange time.Duration
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
	// The intent index, which the canned pane already names. It is planted so
	// that the walkthrough has one record of a kind that is about the project
	// as a whole by definition: its page states that scope and offers no act
	// to change it, which is the half of the About row that cannot be shown
	// from a design.
	walkthroughBook = "docs/intent/index.md"
	walkthroughText = `# Reading and editing in place

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
	walkthroughBookText = `# Intent

- Kind: intent
- Id: intent-index
- Status: accepted

The MetaSystem is the machinery a human runs a fleet of agents with. It exists
so that one person can hold the intent while the work is done by many hands.

## What this is for

A record of this kind is about the project as a whole, and the grammar refuses
a Goals line on one: there is no goal an intent is narrowed to.
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
func fixtureCheckout(calm bool) string {
	// The calm workspace's finished design is marked done on disk, because
	// the two designs below are read back from the file rather than from the
	// pane: a design whose goals have all landed and which nobody has closed
	// is exactly what Overview says needs a human, so the calm fixture closes
	// it where the busy one leaves it open.
	landed := walkthroughLandedText
	if calm {
		landed = strings.Replace(landed, "- Status: accepted", "- Status: done", 1)
	}
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
		{walkthroughBook, walkthroughBookText},
		{walkthroughDocument, walkthroughText},
		{walkthroughRecord, walkthroughHead},
		{walkthroughPartly, walkthroughPartlyText},
		{walkthroughLanded, landed},
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
func newLedger(calm bool) *ledger {
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
	claimed.Claimed = &goal.ClaimRecord{Machine: "m1e", Lineage: "coordinator", At: stampedAgo(5 * time.Hour)}
	second := add(ranked(walkthroughGoal("g1-s19", goal.StateClaimed, "The document reader anchors a heading"), 2, 7))
	second.Claimed = &goal.ClaimRecord{Machine: "m2a", Lineage: "implementer", At: stampedAgo(90 * time.Minute)}

	// A third claim, by a machine that has published no presence at all. It
	// is the one shape of the fleet page that cannot be shown from a presence
	// record, because it is the absence of one: the ledger names the machine
	// and nothing else does.
	absent := add(ranked(walkthroughGoal("g1-s27", goal.StateClaimed, "The fleet channel gateway opens"), 2, 8))
	absent.Claimed = &goal.ClaimRecord{Machine: "m0b", Lineage: "implementer", At: stampedAgo(4 * time.Hour)}

	// Built and waiting to land, which is the Review lane; and held by a
	// park, which is Waiting with a reason and a stamp. Without these two the
	// lane strip has two empty counts and the Waiting block has nothing to
	// name, and neither is a shape a walkthrough should leave untried.
	landing := add(ranked(walkthroughGoal("g1-s21", goal.StateClaimed, "The Overview reads what needs a human"), 2, 10))
	landing.Claimed = &goal.ClaimRecord{Machine: "m1e", Lineage: "coordinator", At: stampedAgo(7 * time.Hour)}
	landing.Landing = &goal.LandingRecord{At: stampedAgo(35 * time.Minute)}

	parked := add(ranked(walkthroughGoal("g1-s22", goal.StateParked, "The Fleet section reads the census"), 2, 11))
	parked.Parked = &goal.ParkRecord{
		By: "human:Wido", At: stampedAgo(30 * time.Hour),
		Because: "the census format is still being decided",
	}

	// One goal that waits for two others and is itself waited for by one, so
	// that the goal page has both directions to show, the Waiting card has
	// its line, and removing an edge from the page has an edge to remove.
	waits := add(ranked(walkthroughGoal("g1-s23", goal.StateParked, "The seat census answers the fleet page"), 2, 12))
	waits.Blocked = []string{"g1-s24", "g1-s25"}
	waits.Parked = &goal.ParkRecord{
		By: "human:Wido", At: stampedAgo(20 * time.Hour), Blocker: "g1-s24",
		Because: "blocked by g1-s24, g1-s25; returns when they are done",
	}
	add(ranked(walkthroughGoal("g1-s24", goal.StateQueued, "The census format is decided"), 2, 13))
	add(ranked(walkthroughGoal("g1-s25", goal.StateQueued, "The seat roster is read from the registry"), 2, 14))
	holder := add(ranked(walkthroughGoal("g1-s26", goal.StateQueued, "The fleet page draws the census"), 2, 15))
	holder.Blocked = []string{"g1-s23"}

	// Two goals the ledger wrote to inside the day, so that what changed
	// since the last visit has rows rather than a sentence saying nothing
	// did. The stamps are relative to this process's clock for the reason the
	// conclusions below are.
	touch(claimed, "claim", 5*time.Hour)
	touch(second, "claim", 90*time.Minute)

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

	if calm {
		admitEverything(tree)
	}
	return &ledger{tree: tree}
}

// admitEverything is the calm workspace's ledger: every queued goal carries a
// human's approval, so nothing in To Do is waiting on one. It is the one
// change calm makes to the tree — the work itself, and every other lane, is
// the same fixture.
func admitEverything(tree *goal.TreeGoals) {
	for _, file := range tree.Live {
		if file.State != goal.StateQueued {
			continue
		}
		file.State = goal.StateApproved
		file.Approved = &goal.ApprovalRecord{
			By: "human:Wido", At: stampedAgo(26 * time.Hour),
			Authority: goal.ApprovalAuthorityProven, Revision: file.Revision,
		}
		file.Budget = &goalbudget.Budget{
			ElapsedLimit: "4h", AttemptLimit: 6, ReservedJobMinutesLimit: 720,
			ActiveJobLimit: 1, ReviewRoundLimit: 2,
		}
	}
}

// stampedAgo is an instant this long before this process started, written the
// way a record writes one. The fixture stamps relative to its own clock for
// the reason the conclusions do: a date written down drifts out of every
// window the day after it is written, and the walkthrough then shows an empty
// block where the design says there is something to read.
func stampedAgo(ago time.Duration) string {
	return time.Now().UTC().Add(-ago).Format(time.RFC3339)
}

// touch appends the History line the ledger writes when something happens to a
// goal, which is what "changed since your last visit" reads.
func touch(file *goal.GoalFile, verb string, ago time.Duration) *goal.GoalFile {
	file.History = append(file.History, goal.HistoryLine{
		At: stampedAgo(ago), Opid: "op-" + verb + "-" + file.Id, Verb: verb, Actor: "m1e+coordinator",
	})
	return file
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
		Tip: "c5d517f427e35e19c2944ab9aefee4d9c9cd9e6c", CommittedAt: now.Add(-l.lastChange),
		Tree: l.tree, Horizon: horizon, SyncMode: goal.SyncLocal,
		// The claim gate cannot be asked about a tree with no repository
		// behind it, so the walkthrough answers it directly: every approved
		// goal is one a seat could claim.
		Admission: l.admission(),
		Fetch:     l.loop(now),
	}
}

// fetch is the look the board's Refresh runs before it observes. There is no
// remote under this fixture, so the look is recorded and the loop stays what
// the flag said it was: a fixture that healed itself on the first Refresh
// could not show the two states a Refresh does not cure.
func (l *ledger) fetch() {
	l.looks++
	fmt.Printf("refresh ran a fetch (%d so far)\n", l.looks)
}

// loop is what this fixture's freshness loop last did. The three answers are
// the three the interface judges: a look that landed a second ago on the tip
// this clone has accepted, a look that has not landed for two hours, and a
// look that failed three times running.
func (l *ledger) loop(now time.Time) snapshot.FetchState {
	const tip = "c5d517f427e35e19c2944ab9aefee4d9c9cd9e6c"
	landed := snapshot.FetchState{
		Outcome: snapshot.OutcomeCurrent, StartedAt: now.Add(-2 * time.Second), FinishedAt: now.Add(-time.Second),
		Tip: tip, Detail: "already at the canonical tip",
		SucceededAt: now.Add(-time.Second), SucceededTip: tip,
		Cadence: snapshot.CadenceConnected, NextAt: now.Add(5 * time.Second),
	}
	switch l.freshness {
	case snapshot.FreshnessBehind:
		behind := now.Add(-2 * time.Hour)
		landed.StartedAt, landed.FinishedAt, landed.SucceededAt = behind.Add(-time.Second), behind, behind
		landed.NextAt = now.Add(5 * time.Second)
		return landed
	case snapshot.FreshnessFailed:
		landed.Outcome = snapshot.OutcomeFailed
		landed.StartedAt, landed.FinishedAt = now.Add(-31*time.Second), now.Add(-30*time.Second)
		landed.Tip, landed.Detail = "", ""
		landed.Message = "ssh: connect to host ledger.example.org port 22: Operation timed out"
		landed.Failures = 3
		landed.SucceededAt = now.Add(-9 * time.Minute)
		landed.NextAt = now.Add(40 * time.Second)
		return landed
	}
	return landed
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
	"g1-s23": "The seat census",
	"g1-s24": "The census format",
	"g1-s25": "The seat roster",
	"g1-s26": "The fleet page's census",
}

// paneGoals are the goals the Project pane carries, in the order the ledger
// lists them: the live ones first, then the ones that concluded. The concluded
// ones are here because a design's work is read out of them — a payload that
// carried only live goals would show a shipped design as a design naming a
// goal nobody has heard of.
var paneGoals = []string{"g1-s15", "g1-s12", "g1-s13", "g1-s9", "g1-s10", "g1-s23", "g1-s24", "g1-s25", "g1-s26"}

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
	pane := project.Pane{
		SchemaVersion: project.SchemaVersion,
		ReadAt:        time.Now().UTC().Format(time.RFC3339),
		Goals:         goals,
		Records: []project.Record{
			{
				Kind: "design", ID: "design-decisions", Status: "accepted", Goals: []string{"g1-s15"},
				Title: "The Decisions section", Path: "plans/designs/decisions.md", Home: "plans/designs",
				Summary:   "How a question is answered from the browser.",
				ChangedAt: stampedAgo(3 * time.Hour),
				Slices: []string{
					"The register is read and shown with its open questions first",
					"Answering one publishes through the project writer",
					"A question a human asks arrives in the same register",
				},
			},
			{
				Kind: "design", ID: "design-board", Status: "accepted", Goals: []string{"g1-s12"},
				Title: "The backlog board", Path: "plans/designs/board.md", Home: "plans/designs",
				Summary:   "The board, and the acts on it.",
				ChangedAt: stampedAgo(9 * 24 * time.Hour),
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
				ChangedAt: stampedAgo(80 * time.Minute),
			},
			// The two readings of a design's work: one part of the way there,
			// and one whose goals have all landed, which is the one that is
			// offered the act that follows.
			l.asItReads(project.Record{
				Kind: "design", ID: "design-reader", Status: "accepted", Goals: []string{"g1-s9", "g1-s13"},
				Title: "The document reader", Path: walkthroughPartly, Home: "plans/designs",
				Summary:   "A document is read among its siblings, with its outline beside it.",
				ChangedAt: stampedAgo(6 * 24 * time.Hour),
				Slices:    []string{},
			}),
			l.asItReads(project.Record{
				Kind: "design", ID: "design-shell", Status: "accepted", Goals: []string{"g1-s9", "g1-s10"},
				Title: "The application shell", Path: walkthroughLanded, Home: "plans/designs",
				Summary:   "The rail, the header and the work area are one shell every section is read in.",
				ChangedAt: stampedAgo(20 * time.Hour),
				Slices:    []string{},
			}),
			// The designs about the project as a whole, which is what a head
			// with no Goals line means. They are what the Designs tab opens
			// on: the scope control's default is the project's own records,
			// and a tab whose kind had none of them would open empty on
			// every walkthrough.
			{
				Kind: "design", ID: "design-interface", Status: "accepted", Goals: []string{},
				Title: "The interface", Path: "plans/designs/interface.md", Home: "plans/designs",
				Summary:   "The one standing design every slice of the interface is held to.",
				ChangedAt: stampedAgo(4 * 24 * time.Hour),
				Slices:    []string{},
			},
			{
				Kind: "design", ID: "design-grammar", Status: "draft", Goals: []string{},
				Title: "The record grammar", Path: "plans/designs/grammar.md", Home: "plans/designs",
				Summary:   "What a head declares, and what each key means.",
				ChangedAt: stampedAgo(26 * time.Hour),
				Slices:    []string{},
			},
			// Three decisions: two about the project as a whole and one about
			// a goal, so the Decisions tab has rows in every scope and on a
			// goal's own page.
			{
				Kind: "decision", ID: "decision-one-binary", Status: "accepted", Goals: []string{},
				Title: "One binary", Path: "docs/decisions/0001-one-binary.md", Home: "docs/decisions",
				Summary:   "The engine ships as one executable, and the interface is served from it.",
				ChangedAt: stampedAgo(31 * 24 * time.Hour),
				Slices:    []string{},
			},
			{
				Kind: "decision", ID: "decision-loopback", Status: "accepted", Goals: []string{},
				Title: "Loopback only", Path: "docs/decisions/0003-loopback.md", Home: "docs/decisions",
				Summary:   "The interface is served to the machine it runs on and to nowhere else.",
				ChangedAt: stampedAgo(18 * 24 * time.Hour),
				Slices:    []string{},
			},
			{
				Kind: "decision", ID: "decision-answering", Status: "draft", Goals: []string{"g1-s15"},
				Title: "A question is answered by a record", Path: "docs/decisions/0002-answering.md",
				Home:      "docs/decisions",
				Summary:   "Answering names the record that answered it, and the row keeps the name.",
				ChangedAt: stampedAgo(50 * time.Minute),
				Slices:    []string{},
			},
		},
		// The two books, with an index each, so the memory block has a
		// chapter count to show and a sentence to open with. They are canned
		// like the rest of this pane: what Overview reads out of them is a
		// number and a first sentence, and both are here.
		Intent: project.Book{
			Index: &project.Record{
				Kind: "intent", ID: "intent-index", Status: "accepted",
				Title: "Intent", Path: "plans/intent/index.md", Home: "plans/intent",
				Summary: "The MetaSystem is the machinery a human runs a fleet of agents with. It exists so that one person can hold the intent while the work is done by many hands.",
				Goals:   []string{}, Slices: []string{},
			},
			Chapters: []project.Chapter{
				{ID: "intent-shift", Title: "1. The shift", Summary: "Software is no longer written by hand."},
				{ID: "intent-human", Title: "2. What the human keeps", Summary: "Intent, approval and judgement stay with the person."},
				{Path: "docs/paper/03-the-fleet.md", Title: "3. The fleet", Summary: "Many seats, one ledger, one accepted tip."},
			},
		},
		Doctrine: project.Book{
			Index: &project.Record{
				Kind: "doctrine", ID: "doctrine-index", Status: "accepted",
				Title: "Doctrine", Path: "plans/doctrine/index.md", Home: "plans/doctrine",
				Summary: "The rules every design is held to, decided once.",
				Goals:   []string{}, Slices: []string{},
			},
			Chapters: []project.Chapter{
				{ID: "doctrine-owners", Title: "1. One owner per answer", Summary: "Nothing is derived twice."},
				{ID: "doctrine-refusal", Title: "2. Refuse visibly", Summary: "A gap is named, never filled in."},
			},
		},
		Questions: []project.Question{
			{
				ID: "q-census", Opened: stampedAgo(2 * time.Hour), Status: "open",
				Question: "Does the census belong to Fleet or to Settings?", Goals: []string{"g1-s22"},
			},
			{
				ID: "q-window", Opened: stampedAgo(3 * 24 * time.Hour), Status: "open",
				Question: "How far back should the Done lane reach by default?", Goals: []string{},
			},
			{
				ID: "q-naming", Opened: stampedAgo(11 * 24 * time.Hour), Status: "open",
				Question: "Is \"seat\" the word a human outside this project would use?", Goals: []string{},
			},
			{
				ID: "q-bundle", Opened: stampedAgo(40 * 24 * time.Hour), Status: "answered",
				Question: "Is the bundle committed or built on demand?", Goals: []string{"g1-s8"},
			},
		},
		Problems: []project.Problem{},
		Documents: []project.File{
			{Path: walkthroughBook, Title: "Intent"},
			{Path: walkthroughDocument, Title: "Reading and editing in place"},
			{Path: walkthroughRecord, Title: "The reading pane"},
			{Path: walkthroughPartly, Title: "The document reader"},
			{Path: walkthroughLanded, Title: "The application shell"},
		},
	}
	if l.calm {
		return settled(pane)
	}
	return pane
}

// settled is the calm workspace's records: every draft accepted and the
// register closed. Nothing else changes — the same designs, the same books,
// the same documents — so what the calm page proves is the wording of "nothing
// needs you" rather than a second fixture.
func settled(pane project.Pane) project.Pane {
	records := make([]project.Record, 0, len(pane.Records))
	for _, record := range pane.Records {
		if record.Status == "draft" {
			record.Status = "accepted"
		}
		records = append(records, record)
	}
	pane.Records = records
	questions := make([]project.Question, 0, len(pane.Questions))
	for _, question := range pane.Questions {
		question.Status = "answered"
		questions = append(questions, question)
	}
	pane.Questions = questions
	return pane
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
	// Both directions of the blocked relation, as the engine writes them: the
	// goals that will wait for this one park behind it, and this one parks
	// behind the goals it waits for unless every one of them is done.
	for _, held := range opened.Blocks {
		if err := l.block(held, opened.ID); err != nil {
			return err
		}
	}
	for _, blocker := range opened.BlockedBy {
		if err := l.block(opened.ID, blocker); err != nil {
			return err
		}
	}
	l.plantGoal(opened.ID, opened.Intent)
	return nil
}

// block and unblock are the walkthrough's two edge acts.
//
// They are the engine's rules in the small: the edge lands on the goal that
// waits, a blocker that is already done parks nothing, and the park returns
// only when it was the dependency's own and every remaining goal it waits for
// is done. A person's ordinary park carries no marker and is left alone.
func (l *ledger) block(dependent, blocker string) error {
	file := l.tree.Live[dependent]
	if file == nil {
		return fmt.Errorf("goal %s is not live", dependent)
	}
	if dependent == blocker {
		return fmt.Errorf("goal %s cannot wait for itself", dependent)
	}
	if !l.tree.Exists(blocker) {
		return fmt.Errorf("the ledger carries no goal named %s", blocker)
	}
	for _, named := range file.Blocked {
		if named == blocker {
			return fmt.Errorf("goal %s already waits for %s", dependent, blocker)
		}
	}
	file.Blocked = append(append([]string(nil), file.Blocked...), blocker)
	sort.Strings(file.Blocked)
	if l.tree.Done[blocker] != nil || file.State == goal.StateParked {
		return nil
	}
	file.State = goal.StateParked
	file.Claimed, file.Landing = nil, nil
	file.Parked = &goal.ParkRecord{
		By: "human:Wido", At: stampedAgo(time.Minute), Blocker: blocker,
		Because: "blocked by " + strings.Join(file.Blocked, ", ") + "; returns when they are done",
	}
	return nil
}

func (l *ledger) unblock(dependent, blocker string) error {
	file := l.tree.Live[dependent]
	if file == nil {
		return fmt.Errorf("goal %s is not live", dependent)
	}
	remaining := []string{}
	found := false
	for _, named := range file.Blocked {
		if named == blocker {
			found = true
			continue
		}
		remaining = append(remaining, named)
	}
	if !found {
		return fmt.Errorf("goal %s does not wait for %s", dependent, blocker)
	}
	file.Blocked = remaining
	if file.State != goal.StateParked || file.Parked == nil || file.Parked.Blocker == "" {
		return nil
	}
	open := []string{}
	for _, named := range remaining {
		if l.tree.Done[named] == nil {
			open = append(open, named)
		}
	}
	if len(open) == 0 {
		file.State = goal.StateQueued
		if file.Approved != nil {
			file.State = goal.StateApproved
		}
		file.Parked = nil
		return nil
	}
	file.Parked.Blocker = open[0]
	file.Parked.Because = "blocked by " + strings.Join(open, ", ") + "; returns when they are done"
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
// plantedNotice is one line of the fixture journal, written relative to this
// process's clock.
type plantedNotice struct {
	ago       time.Duration
	source    string
	ref       string
	message   string
	delivered bool
	problem   string
}

// calmNotices is the journal of a steward that did its work and reached the
// operator every time: nothing addressed to a human, and nothing the channel
// refused. It is what the calm workspace's health line is composed from.
var calmNotices = []plantedNotice{
	{31 * time.Hour, "steward", "", "steward: the runner is armed and ticking", true, ""},
	{20 * time.Hour, "steward", "", "steward: reaped 2 finished workers", true, ""},
	{9 * time.Hour, "verdict", "verdict-budget-kept", "steward: g1-s14 is inside its elapsed budget", true, ""},
	{2 * time.Hour, "steward", "", "steward: the accepted ledger advanced to c5d517f", true, ""},
}

func fixtureJournal(checkout string, calm bool) string {
	path := filepath.Join(checkout, "artifacts", "agents", "steward", "notifications.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		log.Fatalf("cannot make the walkthrough journal: %v", err)
	}
	now := time.Now().UTC()
	planted := []plantedNotice{
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
	if calm {
		planted = calmNotices
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
