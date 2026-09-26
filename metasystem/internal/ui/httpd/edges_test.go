package httpd

import (
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalbudget"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/act"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/session"
)

// The bed the two tests above stand on: one real clone of one real origin,
// declaring the fake runtime so a fixture human proof is admissible, with the
// pre-commit guard every goal mutation stands on and a lawful root record.
//
// It is the act package's own bed, written again here rather than exported,
// because what these tests prove is this package's: that the route reaches
// that engine with the hand the request carried.

var edgeNow = time.Date(2026, 9, 22, 9, 30, 0, 0, time.UTC)

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	command.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid",
		"GIT_COMMITTER_NAME=fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid")
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func writeFixture(t *testing.T, path, body string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), mode); err != nil {
		t.Fatal(err)
	}
}

func edgeEndpoint(root string) goal.Endpoint {
	return goal.Endpoint{Root: root, Remote: "origin", Branch: "refs/heads/main"}
}

func edgeLedger(t *testing.T) string {
	t.Helper()
	origin := filepath.Join(t.TempDir(), "origin.git")
	runGit(t, t.TempDir(), "init", "-q", "--bare", "-b", "main", origin)
	seed := filepath.Join(t.TempDir(), "seed")
	runGit(t, t.TempDir(), "clone", "-q", origin, seed)
	writeFixture(t, filepath.Join(seed, "README.md"), "seed\n", 0o644)
	writeFixture(t, filepath.Join(seed, "metasystem.conf"),
		"metasystem.runtimes=fake\nmetasystem.budget.review-round-max=3\n", 0o644)
	runGit(t, seed, "add", "README.md", "metasystem.conf")
	runGit(t, seed, "commit", "-qm", "seed")
	runGit(t, seed, "push", "-q", "origin", "main")

	root := filepath.Join(t.TempDir(), "clone")
	runGit(t, t.TempDir(), "clone", "-q", origin, root)
	runGit(t, root, "config", "metasystem.goal.machine", "mac-ui")
	writeFixture(t, filepath.Join(root, "scripts", "agents", "pre-commit-guard.sh"), "#!/usr/bin/env bash\nexit 0\n", 0o755)

	record := &goal.RootRecord{
		Identity: "01J5X000000000000000000000", FormatVersion: "1",
		SyncMode: goal.SyncRemote, MigrationEpoch: "2026-09-01T00:00:00Z",
		ManifestDigest: strings.Repeat("ab", 32), MigrationMode: "manifest", Revision: 1,
		History: []goal.HistoryLine{{
			At: "2026-09-01T09:00:00Z", Opid: "01J5X0000000000000000000A0-mac-ui-1a2b3c4d",
			Verb: "migrate", Actor: "mac-ui+terminal-ttys004-1", Keep: -1,
		}},
	}
	result, err := goal.Publish(edgeEndpoint(root), goal.PublishRequest{
		Opid: "01J5X0000000000000000000A1-mac-ui-1a2b3c4d", Machine: "mac-ui", Lineage: "terminal-ttys004-1",
		Intent:  goal.Intent{Verb: "migrate", Targets: []string{"backlog"}},
		Message: "seed the ledger root record",
		Mutate: func(string) ([]goal.Change, error) {
			return []goal.Change{{Path: "plans/goals/backlog.md", Content: goal.RenderRoot(record)}}, nil
		},
	})
	if err != nil || result.Outcome != goal.OutcomeConfirmed {
		t.Fatalf("seeding the ledger: %+v %v", result, err)
	}
	return root
}

// openBlockedGoal plants the one shape these tests are about: a live goal,
// and a goal parked behind it that only a person may lift early. Both are
// written by the proven fixture human, through the same act path the routes
// use, so the bed is not a hand-built record either.
func openBlockedGoal(t *testing.T, root string) {
	t.Helper()
	hand, err := act.Fixture(root, "Wido", "terminal-ttys004-1", edgeNow)
	if err != nil {
		t.Fatal(err)
	}
	risk := goal.RiskRecord{Severity: 1, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "route fixture"}
	if err := hand.Open(act.Opened{
		ID: "dep-live", Intent: "The work the other one waits for.",
		NextStep: "Take it to a working end state.", Risk: risk,
	}); err != nil {
		t.Fatalf("open the blocker: %v", err)
	}
	if err := hand.Open(act.Opened{
		ID: "waits", Intent: "The work that waits.", NextStep: "Wait for it.",
		BlockedBy: []string{"dep-live"}, Risk: risk,
	}); err != nil {
		t.Fatalf("open the goal that waits: %v", err)
	}
}

// actingHand is the hand the route found, as the engine wiring builds it from
// the live browser session the request carried.
func actingHand(t *testing.T, root string, signed *session.Session) act.Authority {
	t.Helper()
	if signed == nil {
		t.Fatal("the route reached the engine with no session behind it")
	}
	proof, err := humanauthority.SignedInSessionProof(root, signed.Human, "sess-01J5XROUTE", "browser", edgeNow)
	if err != nil {
		t.Fatal(err)
	}
	hand, err := act.SignedIn(root, signed.Human, "sess-01J5XROUTE", proof)
	if err != nil {
		t.Fatal(err)
	}
	return hand
}

// seatUnblock reaches the engine with no person behind the act at all, and
// answers the way an act refusal answers, so the route maps it as it maps
// every other refusal the engine gives.
func seatUnblock(root, dependent, blocker string) error {
	ulid, err := goal.NewOperationULID()
	if err != nil {
		return &act.Refusal{Kind: act.KindFailed, Code: "no-operation-id", Message: err.Error()}
	}
	result, publishErr := goal.Unblock(goal.VerbRequest{
		Endpoint: edgeEndpoint(root),
		Actor:    goal.Actor{Machine: "mac-ui", Lineage: "terminal-ttys004-1"},
		Ulid:     ulid, Now: edgeNow, ClaimEpoch: 1,
	}, dependent, blocker, nil)
	if publishErr != nil {
		return &act.Refusal{Kind: act.KindEngine, Code: "refused", Message: publishErr.Error()}
	}
	if result.Outcome != goal.OutcomeConfirmed {
		return &act.Refusal{Kind: act.KindEngine, Code: string(result.Outcome), Message: result.Detail}
	}
	return nil
}

func acceptedTip(t *testing.T, root string) string {
	t.Helper()
	projection, err := goal.Project(edgeEndpoint(root), false, edgeNow)
	if err != nil {
		t.Fatal(err)
	}
	return projection.Tip
}

func readGoalFile(t *testing.T, root, id string) *goal.GoalFile {
	t.Helper()
	projection, err := goal.Project(edgeEndpoint(root), false, edgeNow)
	if err != nil {
		t.Fatal(err)
	}
	file := projection.Tree.Live[id]
	if file == nil {
		t.Fatalf("goal %s is not live", id)
	}
	return file
}

// The two directions of one relation, at the boundary: how the open request
// names them, and how the two edge routes name the goal that waits.

// A field that used to be one string and is now a list has to read both, or
// the first act a browser makes after the engine is replaced is refused for a
// field the human never touched.
func TestOpenRouteReadsBlocksAsAStringOrAList(t *testing.T) {
	t.Parallel()

	const head = `{"id":"ui-new","intent":"The board opens a goal.","nextStep":"Take it.",` +
		`"tier":0,"why":"","labels":[],"severity":1,"novelty":1,"exposure":1,"accumulation":1,` +
		`"basis":"one change behind an existing seam"`

	cases := []struct {
		name      string
		fields    string
		blocks    []string
		blockedBy []string
	}{
		{name: "the empty string is no goals", fields: `,"blocks":""}`},
		{name: "one string is one goal", fields: `,"blocks":"alpha"}`, blocks: []string{"alpha"}},
		{
			name: "a comma string is several goals, in the order they were named",
			// The order is the human's: the first goal named is the one an
			// open's park takes its marker from, so it must not be sorted.
			fields: `,"blocks":"beta, alpha ,,beta"}`, blocks: []string{"beta", "alpha"},
		},
		{name: "an array is several goals", fields: `,"blocks":["alpha","beta"]}`, blocks: []string{"alpha", "beta"}},
		{name: "an empty array is no goals", fields: `,"blocks":[]}`},
		{name: "null is no goals", fields: `,"blocks":null}`},
		{name: "an absent field is no goals", fields: "}"},
		{
			name:   "both directions travel together",
			fields: `,"blocks":["alpha"],"blockedBy":["gamma","delta"]}`,
			blocks: []string{"alpha"}, blockedBy: []string{"gamma", "delta"},
		},
		{
			name:      "blockedBy alone reads a comma string too",
			fields:    `,"blockedBy":"gamma,delta"}`,
			blockedBy: []string{"gamma", "delta"},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			rec := &acted{authorized: proven()}
			served := New(rec.acting(), loopback(), testBundle())

			response := post(t, served, "/api/backlog/goals", head+test.fields, nil)

			testutil.Require(t, "status", response.Code, http.StatusOK)
			testutil.Require(t, "one open reached the engine", len(rec.opened), 1)
			testutil.Expect(t, "the goals that will wait for this one", rec.opened[0].Blocks, test.blocks)
			testutil.Expect(t, "the goals this one will wait for", rec.opened[0].BlockedBy, test.blockedBy)
		})
	}
}

// A body that is neither a list of ids nor a line of them is the request's
// own fault, and is said so rather than being quietly read as nothing.
func TestOpenRouteRefusesABlocksFieldThatIsNeitherShape(t *testing.T) {
	t.Parallel()
	rec := &acted{authorized: proven()}
	served := New(rec.acting(), loopback(), testBundle())

	response := post(t, served, "/api/backlog/goals",
		`{"id":"ui-new","intent":"x","nextStep":"y","tier":0,"why":"","blocks":7,"labels":[],`+
			`"severity":1,"novelty":1,"exposure":1,"accumulation":1,"basis":"z"}`, nil)

	testutil.Require(t, "status", response.Code, http.StatusBadRequest)
	testutil.Expect(t, "the engine was not reached", rec.reached(), 0)
}

// The id in the path is the goal that WAITS, on both routes, whichever end of
// the relation the page acted from. The blocker travels in the body.
func TestTheTwoEdgeRoutesNameTheGoalThatWaitsInThePath(t *testing.T) {
	t.Parallel()
	rec := &acted{authorized: proven()}
	served := New(rec.acting(), loopback(), testBundle())

	blocked := post(t, served, "/api/backlog/goals/waiting/block", `{"blocker":"the-blocker"}`, nil)
	testutil.Require(t, "status of block", blocked.Code, http.StatusOK)
	unblocked := post(t, served, "/api/backlog/goals/waiting/unblock", `{"blocker":" the-blocker "}`, nil)
	testutil.Require(t, "status of unblock", unblocked.Code, http.StatusOK)

	testutil.Expect(t, "the edge that was written", rec.blocked, [][2]string{{"waiting", "the-blocker"}})
	testutil.Expect(t, "the edge that was removed", rec.unblocked, [][2]string{{"waiting", "the-blocker"}})
	payload := decodeBacklog(t, blocked.Result())
	testutil.Expect(t, "the answer is the backlog", payload["schemaVersion"] != nil, true)
}

// Authority is the act owner's and never the body's: a server nothing proves,
// that nobody has signed into, writes no edge and says how to fix that.
func TestTheTwoEdgeRoutesTakeTheSameAuthorityEveryActTakes(t *testing.T) {
	t.Parallel()
	for _, path := range []string{"/api/backlog/goals/waiting/block", "/api/backlog/goals/waiting/unblock"} {
		rec := &acted{authorized: agentStarted()}
		served := New(rec.acting(), loopback(), testBundle())

		response := post(t, served, path, `{"blocker":"the-blocker"}`, nil)

		testutil.Require(t, "status for "+path, response.Code, http.StatusForbidden)
		refusal := actRefusal(t, response)
		testutil.Expect(t, "the remedy is in the page for "+path, refusal.SignIn, true)
		testutil.Expect(t, "the engine was not reached for "+path, rec.reached(), 0)
	}
}

// The early unblock, end to end: the route, the act, the engine and a real
// ledger, with no refusal injected anywhere.
//
// Everything above this test records what the route passed on and answers
// what it was told to answer, which proves the boundary and nothing about
// authority. An early lift is the one act on this pair of routes that the
// engine admits only for a person, so it is the one worth driving all the way
// down: a signed-in human lifts it and the ledger says who did, and a server
// nobody has signed into writes nothing at all.
func TestAnEarlyUnblockReachesTheEngineUnderASignedInHuman(t *testing.T) {
	t.Parallel()
	root := edgeLedger(t)
	openBlockedGoal(t, root)
	before := acceptedTip(t, root)

	signing := newSigning(t, "Wido", workingSecret)
	info := Info{
		Observe: readObservation,
		// A server an agent started: it can act as nobody by itself, so the
		// only hand in this test is the one that signs in.
		Authority:   agentStarted(),
		Sessions:    signing.store,
		Approve:     func(*session.Session, string, goalbudget.Budget) error { return nil },
		Withdraw:    func(*session.Session, string, string) error { return nil },
		SetPriority: func(*session.Session, string, uint8, *uint64) error { return nil },
		Open:        func(*session.Session, act.Opened) error { return nil },
		Block: func(signed *session.Session, dependent, blocker string) error {
			return actingHand(t, root, signed).Block(dependent, blocker)
		},
		Unblock: func(signed *session.Session, dependent, blocker string) error {
			return actingHand(t, root, signed).Unblock(dependent, blocker)
		},
		Park: func(signed *session.Session, id, because string) error {
			return actingHand(t, root, signed).Park(id, because)
		},
		Unpark: func(signed *session.Session, id string) error { return actingHand(t, root, signed).Unpark(id) },
	}
	served := New(info, loopback(), testBundle())

	// Nobody is signed in: the act is refused before the engine is asked,
	// and the accepted tip is where it was.
	refused := post(t, served, "/api/backlog/goals/waits/unblock", `{"blocker":"dep-live"}`, nil)
	testutil.Require(t, "status without a session", refused.Code, http.StatusForbidden)
	testutil.Expect(t, "the remedy is in the page", actRefusal(t, refused).SignIn, true)
	testutil.Expect(t, "the ledger is untouched", acceptedTip(t, root), before)

	// A human signs in and lifts it. The engine admits the session proof the
	// approval gate admits, and the line it writes says which hand acted.
	signedIn := post(t, served, signInPath, `{"code":"`+routeCode(t, routeNow)+`","human":""}`, nil)
	testutil.Require(t, "signed in", signedIn.Code, http.StatusOK)
	response := post(t, served, "/api/backlog/goals/waits/unblock", `{"blocker":"dep-live"}`,
		carrying(mintedCookie(t, signedIn).Value))
	testutil.Require(t, "status under a signed-in human", response.Code, http.StatusOK)
	testutil.Expect(t, "the ledger moved", acceptedTip(t, root) != before, true)

	lifted := readGoalFile(t, root, "waits")
	testutil.Expect(t, "the goal returned", lifted.State, goal.StateQueued)
	testutil.Expect(t, "the edge is gone", len(lifted.Blocked), 0)
	last := lifted.History[len(lifted.History)-1]
	testutil.Expect(t, "the verb", last.Verb, "unblock")
	testutil.Expect(t, "the hand", last.AuthorityOutcome, goal.AuthorityOutcomeSignedInSession)
	testutil.Expect(t, "the human", last.Actor, "human:Wido")
}

// A seat cannot lift an unfinished blocker whatever the route says, so the
// boot proof's own hand is refused by the engine in the engine's own words.
func TestAnEarlyUnblockIsRefusedWhenNoHumanIsBehindTheAct(t *testing.T) {
	t.Parallel()
	root := edgeLedger(t)
	openBlockedGoal(t, root)
	before := acceptedTip(t, root)

	info := Info{
		Observe:     readObservation,
		Authority:   proven(),
		Approve:     func(*session.Session, string, goalbudget.Budget) error { return nil },
		Withdraw:    func(*session.Session, string, string) error { return nil },
		SetPriority: func(*session.Session, string, uint8, *uint64) error { return nil },
		Open:        func(*session.Session, act.Opened) error { return nil },
		Block:       func(*session.Session, string, string) error { return nil },
		Unblock: func(_ *session.Session, dependent, blocker string) error {
			// The hand a seat's own process reaches the engine with: a
			// machine, a lineage, and no person behind either. The refusal
			// that comes back is the engine's own words, carried the way the
			// act package carries one.
			return seatUnblock(root, dependent, blocker)
		},
		Park:   func(*session.Session, string, string) error { return nil },
		Unpark: func(*session.Session, string) error { return nil },
	}
	served := New(info, loopback(), testBundle())

	response := post(t, served, "/api/backlog/goals/waits/unblock", `{"blocker":"dep-live"}`, nil)

	testutil.Require(t, "status", response.Code, http.StatusConflict)
	testutil.Expect(t, "the engine's own sentence",
		strings.Contains(actRefusal(t, response).Error, "early lift and a human act"), true)
	testutil.Expect(t, "the ledger is untouched", acceptedTip(t, root), before)
}
