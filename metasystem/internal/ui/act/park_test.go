package act

// Not now, and back, from a signed-in browser (R-125-m1u).
//
// These drive the whole chain the ruling opened: the act layer's Park and
// Unpark, the session proof they carry, the engine's three flagged rows, and
// the History line the ledger keeps. They run over the same fixture ledger
// the approval tests run over — a bare repository in t.TempDir, a fake
// runtime, no remote anybody else can reach — and the branch check is the
// package's own, over a checkout with no goal branch, which is the case that
// answers without reading a remote at all.

import (
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// The act the design is named for. A human-origin goal's park asks a
// terminal-grade proof of every other hand; the session carries none, and the
// ruling admits it here.
func TestParkFromASignedInSessionLandsAndNamesTheSession(t *testing.T) {
	t.Parallel()
	bed := ledger(t)
	openGoal(t, bed, "ui-park")
	authority := sessionFor(t, bed)

	if err := authority.Park("ui-park", "not now; the census format is still being decided"); err != nil {
		t.Fatalf("park: %v", err)
	}

	file := readGoal(t, bed, "ui-park")
	testutil.Expect(t, "the goal is parked", file.State, goal.StateParked)
	testutil.Expect(t, "the park's actor", file.Parked.By, "human:Wido")
	testutil.Expect(t, "the park's reason", file.Parked.Because,
		"not now; the census format is still being decided")

	last := file.History[len(file.History)-1]
	testutil.Expect(t, "the last verb", last.Verb, "park")
	testutil.Expect(t, "the History outcome", last.AuthorityOutcome, goal.AuthorityOutcomeSignedInSession)
	testutil.Expect(t, "the issuer", last.ChannelProvider, "browser")
	testutil.Expect(t, "the handle", last.ChannelUser, "Wido")
	testutil.Expect(t, "the session", last.ChannelRef, testSession)
	if _, problems := goal.ParseFile(goal.RenderFile(file)); len(problems) != 0 {
		t.Fatalf("the written goal does not read back clean: %v", problems)
	}
}

// And back. A human's park is the standing reservation an agent cannot
// silently lift; the session that made it may lift it, and the goal returns
// to the state it rests in.
func TestUnparkFromASignedInSessionReturnsTheGoalAndNamesTheSession(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name     string
		approved bool
		want     string
	}{
		{name: "a goal nobody has approved returns to the queue", want: goal.StateQueued},
		{name: "a goal whose approval stands returns to approved", approved: true, want: goal.StateApproved},
	} {
		t.Run(test.name, func(t *testing.T) {
			bed := ledger(t)
			id := "ui-unpark-queued"
			if test.approved {
				id = "ui-unpark-approved"
			}
			openGoal(t, bed, id)
			authority := sessionFor(t, bed)
			if test.approved {
				if err := authority.Approve(id, box()); err != nil {
					t.Fatalf("approve: %v", err)
				}
			}
			if err := authority.Park(id, "not now"); err != nil {
				t.Fatalf("park: %v", err)
			}

			if err := authority.Unpark(id); err != nil {
				t.Fatalf("unpark: %v", err)
			}

			file := readGoal(t, bed, id)
			testutil.Expect(t, "the resting state", file.State, test.want)
			testutil.Expect(t, "no park stands", file.Parked == nil, true)
			last := file.History[len(file.History)-1]
			testutil.Expect(t, "the last verb", last.Verb, "unpark")
			testutil.Expect(t, "the History outcome", last.AuthorityOutcome, goal.AuthorityOutcomeSignedInSession)
			testutil.Expect(t, "the session", last.ChannelRef, testSession)
		})
	}
}

// The row the ruling does not name. A goal another pair claimed between the
// page's read and the act is refused rather than displaced: the page read a
// queue goal, a seat claimed it in the meantime, and the park that follows
// asks a grade the session does not carry.
func TestParkingAnotherPairsClaimStillRefusesASession(t *testing.T) {
	t.Parallel()
	bed := ledger(t)
	openGoal(t, bed, "ui-race")
	authority := sessionFor(t, bed)
	if err := authority.Approve("ui-race", box()); err != nil {
		t.Fatalf("approve: %v", err)
	}
	// The page has read the queue. Now a seat claims the goal.
	claim(t, bed, "ui-race")

	err := authority.Park("ui-race", "not now")

	refusal, ok := err.(*Refusal)
	if !ok {
		t.Fatalf("park of a claimed goal = %v, want an act.Refusal", err)
	}
	testutil.Expect(t, "the engine refused it", refusal.Kind, KindEngine)
	// The engine names the row it refused at, which is the row the ruling
	// deliberately leaves alone.
	if !strings.Contains(refusal.Message, "park of another pair's claim") {
		t.Fatalf("the refusal does not name the row it refused at: %q", refusal.Message)
	}
	file := readGoal(t, bed, "ui-race")
	testutil.Expect(t, "the claim still holds the goal", file.State, goal.StateClaimed)
	testutil.Expect(t, "nothing was displaced", file.Parked == nil, true)
}

// The other row the ruling does not name: a blocker park a seat recorded,
// lifted before its blocker is done. The session is refused there too, so the
// admission is three rows wide and not two verbs wide.
func TestLiftingASeatsBlockerParkEarlyStillRefusesASession(t *testing.T) {
	t.Parallel()
	bed := ledger(t)
	openGoal(t, bed, "ui-blocker")
	openGoal(t, bed, "ui-held")
	if err := sessionFor(t, bed).Approve("ui-held", box()); err != nil {
		t.Fatalf("approve: %v", err)
	}
	claim(t, bed, "ui-held")
	// goal block records the edge and parks the waiting goal behind it, with
	// the seat as the hand: the park's By is the seat's pair, not a human's.
	result, err := goal.Block(request(t, bed), "ui-held", "ui-blocker", nil)
	if err != nil || result.Outcome != goal.OutcomeConfirmed {
		t.Fatalf("block: %+v %v", result, err)
	}
	held := readGoal(t, bed, "ui-held")
	testutil.Expect(t, "the goal is parked behind its blocker", held.State, goal.StateParked)
	testutil.Expect(t, "the park names the blocker", held.Parked.Blocker, "ui-blocker")
	if strings.HasPrefix(held.Parked.By, "human:") {
		t.Fatalf("the fixture recorded a human's park, so nothing here is proven: %q", held.Parked.By)
	}

	err = sessionFor(t, bed).Unpark("ui-held")

	refusal, ok := err.(*Refusal)
	if !ok {
		t.Fatalf("early unpark = %v, want an act.Refusal", err)
	}
	testutil.Expect(t, "the engine refused it", refusal.Kind, KindEngine)
	// Again the engine names the row: the session met neither grade there.
	if !strings.Contains(refusal.Message, "early unpark of a blocker park") {
		t.Fatalf("the refusal does not name the row it refused at: %q", refusal.Message)
	}
	testutil.Expect(t, "the park still holds", readGoal(t, bed, "ui-held").State, goal.StateParked)
}

// A park without a reason is refused before anything is published, because a
// pause without a why is a stall in disguise — and an unproven server writes
// nothing either way.
func TestParkAndUnparkRefuseWhatTheyCannotPublish(t *testing.T) {
	t.Parallel()
	bed := ledger(t)
	openGoal(t, bed, "ui-guard")
	authority := sessionFor(t, bed)

	for name, err := range map[string]error{
		"no goal":   authority.Park("", "not now"),
		"no reason": authority.Park("ui-guard", "   "),
		"no unpark": authority.Unpark(""),
	} {
		refusal, ok := err.(*Refusal)
		if !ok {
			t.Fatalf("%s = %v, want an act.Refusal", name, err)
		}
		testutil.Expect(t, name+" is the request's own fault", refusal.Kind, KindRequest)
	}
	testutil.Expect(t, "nothing was published", readGoal(t, bed, "ui-guard").State, goal.StateQueued)

	unproven := Unproven("the interface was started by an agent process (claude-code); " + Restart)
	for name, err := range map[string]error{
		"park":   unproven.Park("ui-guard", "not now"),
		"unpark": unproven.Unpark("ui-guard"),
	} {
		refusal, ok := err.(*Refusal)
		if !ok {
			t.Fatalf("%s on an unproven server = %v, want an act.Refusal", name, err)
		}
		testutil.Expect(t, name+" refuses as unproven", refusal.Kind, KindUnproven)
	}
}

// The branch check is carried on the request rather than left nil, which the
// engine fails closed on. A checkout with no goal branch answers without
// reading a remote, which is what makes a park of an ordinary queue goal one
// local read.
func TestEveryRequestCarriesTheParkBranchCheck(t *testing.T) {
	t.Parallel()
	bed := ledger(t)
	authority := sessionFor(t, bed)
	request, err := authority.request()
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if request.ParkBranchCheck == nil {
		t.Fatal("the request carries no branch check, so every park would fail closed")
	}
	summary, checkErr := request.ParkBranchCheck("ui-nothing", "Start ui-nothing.")
	if checkErr != nil || summary != "" {
		t.Fatalf("a branchless goal read %q, %v; want no summary and no remote read", summary, checkErr)
	}
	testutil.Expect(t, "the proof this hand carries is a session's",
		request.Authority.Outcome, humanauthority.OutcomeSession)
}
