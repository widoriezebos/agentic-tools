package act

// Abandon from a signed-in browser (R-128-ui).
//
// These drive the chain the ruling opened above the engine: the act layer's
// Abandon, the session proof it carries, abandon's own human row, and the
// History line the ledger keeps. They run over the same fixture ledger every
// other act test here runs over — an in-memory repository, a fake runtime, no
// remote anybody can reach — so nothing about them runs Git.

import (
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// readAbandoned is the goal after it has left the live tree, which is where an
// abandon puts it: readGoal reads the live one and would fail here.
func readAbandoned(t *testing.T, bed *ledgerBed, id string) *goal.GoalFile {
	t.Helper()
	projection, err := goal.Project(bed.endpoint(), false, fixtureNow)
	if err != nil {
		t.Fatalf("projecting the ledger: %v", err)
	}
	file := projection.Tree.Abandoned[id]
	if file == nil {
		t.Fatalf("goal %s is not abandoned at the accepted tip", id)
	}
	return file
}

// An abandon with a reason, from the browser, is the human's own abandon: it
// names them, it records the session that carried it, and the ledger reads back
// clean afterwards.
func TestAbandonFromASignedInSessionLandsAndNamesTheSession(t *testing.T) {
	t.Parallel()
	bed := ledger(t)
	openGoal(t, bed, "ui-abandon")
	authority := sessionFor(t, bed)

	if err := authority.Abandon("ui-abandon", "superseded by the seat inventory", ""); err != nil {
		t.Fatalf("abandon: %v", err)
	}

	file := readAbandoned(t, bed, "ui-abandon")
	testutil.Expect(t, "the goal is abandoned", file.State, goal.StateAbandoned)
	testutil.Expect(t, "the abandon's actor", file.Abandoned.By, "human:Wido")
	testutil.Expect(t, "the abandon's reason", file.Abandoned.Because, "superseded by the seat inventory")
	testutil.Expect(t, "no successor was named", file.Abandoned.Carried, "")

	last := file.History[len(file.History)-1]
	testutil.Expect(t, "the last verb", last.Verb, "abandon")
	testutil.Expect(t, "the History outcome", last.AuthorityOutcome, goal.AuthorityOutcomeSignedInSession)
	testutil.Expect(t, "the issuer", last.ChannelProvider, "browser")
	testutil.Expect(t, "the handle", last.ChannelUser, "Wido")
	testutil.Expect(t, "the session", last.ChannelRef, testSession)
	if _, problems := goal.ParseFile(goal.RenderFile(file)); len(problems) != 0 {
		t.Fatalf("the written goal does not read back clean: %v", problems)
	}
}

// With a successor the engine does not refuse a goal's live dependents: it
// repoints every one of them in the same act, which is what the card tells the
// human before they press (Astra S64-02).
func TestAbandonWithASuccessorRepointsTheLiveDependents(t *testing.T) {
	t.Parallel()
	bed := ledger(t)
	openGoal(t, bed, "ui-retired")
	openGoal(t, bed, "ui-carries")
	openGoal(t, bed, "ui-waits")
	// The edge is written by this same hand, through the act layer's own Block:
	// a seat's block is refused for a goal it does not hold (R-93-m1e), and what
	// this test needs is the edge a human would have drawn from the page.
	if err := sessionFor(t, bed).Block("ui-waits", "ui-retired"); err != nil {
		t.Fatalf("block: %v", err)
	}

	if err := sessionFor(t, bed).Abandon("ui-retired", "the successor carries it", "ui-carries"); err != nil {
		t.Fatalf("abandon with a successor: %v", err)
	}

	file := readAbandoned(t, bed, "ui-retired")
	testutil.Expect(t, "the successor is recorded", file.Abandoned.Carried, "ui-carries")
	waiter := readGoal(t, bed, "ui-waits")
	testutil.Expect(t, "the dependent waits for the successor now",
		strings.Join(waiter.Blocked, ","), "ui-carries")
}

// Without a successor, a goal with a live dependent is refused by the engine in
// its own words — which name the terminal forms, so the line can say them.
func TestAbandonWithoutASuccessorIsRefusedForALiveDependent(t *testing.T) {
	t.Parallel()
	bed := ledger(t)
	openGoal(t, bed, "ui-held-up")
	openGoal(t, bed, "ui-dependent")
	if err := sessionFor(t, bed).Block("ui-dependent", "ui-held-up"); err != nil {
		t.Fatalf("block: %v", err)
	}

	err := sessionFor(t, bed).Abandon("ui-held-up", "nobody will work it", "")

	refusal, ok := err.(*Refusal)
	if !ok {
		t.Fatalf("abandon with a live dependent = %v, want an act.Refusal", err)
	}
	testutil.Expect(t, "the engine refused it", refusal.Kind, KindEngine)
	for _, wanted := range []string{
		"goal ui-dependent is blocked by ui-held-up",
		"re-point it with --successor",
		"waive it with --waive ui-dependent=<reason>",
		"abandon it with --also ui-dependent",
	} {
		if !strings.Contains(refusal.Message, wanted) {
			t.Fatalf("the refusal %q does not say %q", refusal.Message, wanted)
		}
	}
	testutil.Expect(t, "the goal is still live", readGoal(t, bed, "ui-held-up").State, goal.StateQueued)
}

// A blank reason and a nameless goal are the request's own fault, refused
// before anything is published; an unproven server writes nothing either way.
func TestAbandonRefusesWhatItCannotPublish(t *testing.T) {
	t.Parallel()
	bed := ledger(t)
	openGoal(t, bed, "ui-guarded")
	authority := sessionFor(t, bed)

	for name, err := range map[string]error{
		"no goal":   authority.Abandon("", "nobody will work it", ""),
		"no reason": authority.Abandon("ui-guarded", "   ", ""),
	} {
		refusal, ok := err.(*Refusal)
		if !ok {
			t.Fatalf("%s = %v, want an act.Refusal", name, err)
		}
		testutil.Expect(t, name+" is the request's own fault", refusal.Kind, KindRequest)
	}
	testutil.Expect(t, "nothing was published", readGoal(t, bed, "ui-guarded").State, goal.StateQueued)

	unproven := Unproven("the interface was started by an agent process (claude-code); " + Restart)
	refusal, ok := unproven.Abandon("ui-guarded", "nobody will work it", "").(*Refusal)
	if !ok {
		t.Fatal("an unproven server's abandon did not answer with an act.Refusal")
	}
	testutil.Expect(t, "it refuses as unproven", refusal.Kind, KindUnproven)
	testutil.Expect(t, "the goal is untouched", readGoal(t, bed, "ui-guarded").State, goal.StateQueued)
}

// A successor that is not a live goal is the engine's refusal and not this
// layer's guess: nothing here reads the ledger to check it first.
func TestAbandonLeavesTheSuccessorsOwnRuleToTheEngine(t *testing.T) {
	t.Parallel()
	bed := ledger(t)
	openGoal(t, bed, "ui-successorless")

	err := sessionFor(t, bed).Abandon("ui-successorless", "nobody will work it", "ui-nothing")

	refusal, ok := err.(*Refusal)
	if !ok {
		t.Fatalf("abandon naming an absent successor = %v, want an act.Refusal", err)
	}
	testutil.Expect(t, "the refusal is the engine's", refusal.Kind, KindEngine)
	if !strings.Contains(refusal.Message, "live successor") {
		t.Fatalf("the refusal does not name the rule it broke: %q", refusal.Message)
	}
	testutil.Expect(t, "the goal is still live", readGoal(t, bed, "ui-successorless").State, goal.StateQueued)
}
