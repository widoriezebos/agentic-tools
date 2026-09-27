package goal

// Abandon from a signed-in browser session (R-128-ui).
//
// The ruling admits one more verb at the grade a human's abandon asks for: the
// proof the seat's one-time code mints is this checkout's, so it is admitted
// beside an enrolled terminal's, and the History line the abandon appends names
// the session as the hand. Nothing else about the verb moves, which is what the
// last two tests here hold: a terminal proof is still admitted, no proof is
// still refused, and a live dependent nobody covered still refuses — now in
// words that name the public flag.

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
)

var abandonSessionNow = time.Date(2026, 9, 27, 10, 0, 0, 0, time.UTC)

// abandonSessionUlid is one operation id per step of one of these tests. The
// letters keep it out of every other fixture's range in this package.
func abandonSessionUlid(index, step int) string {
	return fmt.Sprintf("01J5X0000000000000000SB%d%02d", index, step)
}

// abandonSessionBed opens one goal, and a dependent waiting for it where the
// test asks for one, over the package's own in-memory ledger.
func abandonSessionBed(t *testing.T, index int, dependent bool) Endpoint {
	t.Helper()
	endpoint, _ := fakeGoalEndpoint(t)
	if result, err := Open(verbReqFor(endpoint, abandonSessionUlid(index, 1), "mac-ui"), "ui-abandon",
		"Retire the idea nobody will work.", OriginHuman, "Decide whether it is carried."); err != nil ||
		result.Outcome != OutcomeConfirmed {
		t.Fatalf("open the subject: %+v %v", result, err)
	}
	if !dependent {
		return endpoint
	}
	if result, err := Open(verbReqFor(endpoint, abandonSessionUlid(index, 2), "mac-ui"), "ui-waiter",
		"Depend on the retired idea.", OriginHuman, "Resume when the blocker is settled."); err != nil ||
		result.Outcome != OutcomeConfirmed {
		t.Fatalf("open the dependent: %+v %v", result, err)
	}
	blocked := []string{"ui-abandon"}
	if result, err := Edit(verbReqFor(endpoint, abandonSessionUlid(index, 3), "mac-ui"), "ui-waiter",
		EditFields{Blocked: &blocked}); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("wire the dependent: %+v %v", result, err)
	}
	return endpoint
}

// abandonSessionRequest is the request the act layer assembles: this human's
// name, and the session proof as the request's own authority.
func abandonSessionRequest(t *testing.T, endpoint Endpoint, index, step int) (VerbRequest, *humanauthority.Proof) {
	t.Helper()
	request := verbReqFor(endpoint, abandonSessionUlid(index, step), "mac-ui")
	request.Actor.Human = "Wido"
	request.Actor.Lineage = "browser-session"
	request.Now = abandonSessionNow
	proof := sessionProofForTest(t, endpoint.Root, request.Now)
	request.Authority = proof
	return request, proof
}

// The act the ruling opened: an abandon a browser made, with its reason and the
// successor carrying the work, and a History line that names the session.
func TestAbandonUnderASignedInSessionLandsAndNamesTheSession(t *testing.T) {
	t.Parallel()
	endpoint := abandonSessionBed(t, 1, false)
	if result, err := Open(verbReqFor(endpoint, abandonSessionUlid(1, 4), "mac-ui"), "ui-successor",
		"Carry the retired idea's work.", OriginHuman, "Take over the work."); err != nil ||
		result.Outcome != OutcomeConfirmed {
		t.Fatalf("open the successor: %+v %v", result, err)
	}
	request, proof := abandonSessionRequest(t, endpoint, 1, 5)

	result, err := Abandon(request, "ui-abandon",
		AbandonSpec{Because: "superseded by the seat inventory", Carried: "ui-successor"}, proof)
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("abandon under a session: %+v %v", result, err)
	}

	tree, err := loadTreeFor(endpoint, result.Tip)
	if err != nil {
		t.Fatal(err)
	}
	file := tree.Abandoned["ui-abandon"]
	if file == nil {
		t.Fatalf("the goal was not abandoned: %+v", tree.Live)
	}
	if file.State != StateAbandoned || file.Abandoned == nil {
		t.Fatalf("the abandoned goal carries no record: %+v", file)
	}
	if file.Abandoned.By != "human:Wido" {
		t.Fatalf("the abandon's actor = %q, want human:Wido", file.Abandoned.By)
	}
	if file.Abandoned.Because != "superseded by the seat inventory" || file.Abandoned.Carried != "ui-successor" {
		t.Fatalf("the abandon record does not carry the reason and the successor: %+v", file.Abandoned)
	}
	last := file.History[len(file.History)-1]
	if last.Verb != "abandon" {
		t.Fatalf("the last verb = %q, want abandon", last.Verb)
	}
	if last.Reason != "superseded by the seat inventory" || last.Carried != "ui-successor" {
		t.Fatalf("the History line does not carry the reason and the successor: %+v", last)
	}
	assertSessionLine(t, last)
	if parsed, problems := ParseFile(RenderFile(file)); parsed == nil || len(problems) != 0 {
		t.Fatalf("the written goal does not read back clean: %v", problems)
	}
}

// A proof this session never minted — and the terminal's own proof, which the
// admission leaves exactly as it was. The session is admitted beside the
// terminal and not instead of it.
func TestAbandonStillTakesTheTerminalsProofAndRefusesNone(t *testing.T) {
	t.Parallel()
	endpoint := abandonSessionBed(t, 2, false)

	nothing := verbReqFor(endpoint, abandonSessionUlid(2, 5), "mac-ui")
	nothing.Actor.Human = "Wido"
	nothing.Now = abandonSessionNow
	_, err := Abandon(nothing, "ui-abandon", AbandonSpec{Because: "no proof accompanies this"}, nil)
	if err == nil || err.Error() !=
		"abandon requires freshly observed enrolled-terminal human authority or a signed-in browser session" {
		t.Fatalf("abandon with no proof at all: %v", err)
	}

	terminal := verbReqFor(endpoint, abandonSessionUlid(2, 6), "mac-ui")
	terminal.Actor.Human = "Wido"
	terminal.Now = abandonSessionNow
	proof := goalHumanProof(t, endpoint.Root, terminal.Now)
	terminal.Authority = proof
	result, err := Abandon(terminal, "ui-abandon", AbandonSpec{Because: "abandoned at the terminal"}, proof)
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("abandon under an enrolled terminal's proof: %+v %v", result, err)
	}
	tree, err := loadTreeFor(endpoint, result.Tip)
	if err != nil {
		t.Fatal(err)
	}
	file := tree.Abandoned["ui-abandon"]
	if file == nil {
		t.Fatalf("the terminal's abandon did not land: %+v", tree.Live)
	}
	// And it says nothing about a session, because no session made it.
	last := file.History[len(file.History)-1]
	if last.AuthorityOutcome == AuthorityOutcomeSignedInSession || last.ChannelRef != "" {
		t.Fatalf("a terminal's abandon named a session: %+v", last)
	}
}

// The dependents rule stands for the browser's hand, and its words name the
// public flag a human would type: --successor, not the internal --carried.
func TestAbandonUnderASessionRefusesALiveDependentInTheEnginesWords(t *testing.T) {
	t.Parallel()
	endpoint := abandonSessionBed(t, 3, true)
	request, proof := abandonSessionRequest(t, endpoint, 3, 5)

	result, err := Abandon(request, "ui-abandon", AbandonSpec{Because: "nobody will work it"}, proof)

	// The dependents rule is inside the transaction, so it comes back as a
	// rejected publication carrying the engine's own sentence — which is what
	// the act layer hands the route and the card shows.
	if err != nil {
		t.Fatalf("the refusal arrived as a failure rather than a rejection: %v", err)
	}
	if result.Outcome != OutcomeRejected {
		t.Fatalf("a live dependent nobody covered did not refuse the abandon: %+v", result)
	}
	words := result.Detail
	for _, wanted := range []string{
		"goal ui-waiter is blocked by ui-abandon",
		"re-point it with --successor",
		"waive it with --waive ui-waiter=<reason>",
		"abandon it with --also ui-waiter",
	} {
		if !strings.Contains(words, wanted) {
			t.Fatalf("the refusal %q does not say %q", words, wanted)
		}
	}
	if strings.Contains(words, "--carried") {
		t.Fatalf("the refusal names the internal flag rather than the public one: %q", words)
	}
	projection, projectErr := Project(endpoint, false, abandonSessionNow)
	if projectErr != nil {
		t.Fatal(projectErr)
	}
	if projection.Tree.Live["ui-abandon"] == nil {
		t.Fatal("the refused abandon moved the goal anyway")
	}
}

// A successor repoints the live dependents in the same act rather than
// refusing, which is what the card tells the human before they press.
func TestAbandonUnderASessionRepointsItsLiveDependentsToTheSuccessor(t *testing.T) {
	t.Parallel()
	endpoint := abandonSessionBed(t, 4, true)
	if result, err := Open(verbReqFor(endpoint, abandonSessionUlid(4, 4), "mac-ui"), "ui-successor",
		"Carry the retired idea's work.", OriginHuman, "Take over the work."); err != nil ||
		result.Outcome != OutcomeConfirmed {
		t.Fatalf("open the successor: %+v %v", result, err)
	}
	request, proof := abandonSessionRequest(t, endpoint, 4, 5)

	result, err := Abandon(request, "ui-abandon",
		AbandonSpec{Because: "nobody will work it", Carried: "ui-successor"}, proof)
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("abandon with a successor: %+v %v", result, err)
	}

	tree, err := loadTreeFor(endpoint, result.Tip)
	if err != nil {
		t.Fatal(err)
	}
	waiter := tree.Live["ui-waiter"]
	if waiter == nil {
		t.Fatalf("the dependent left the live tree: %+v", tree.Abandoned)
	}
	if len(waiter.Blocked) != 1 || waiter.Blocked[0] != "ui-successor" {
		t.Fatalf("the dependent's blockers = %v, want the successor alone", waiter.Blocked)
	}
	// The line the verb APPENDS is the abandoned goal's own, and that is the one
	// the session is recorded on (D1). The dependent's own line is a merge into
	// whatever the same operation already wrote there, and the ruling does not
	// reach it.
	abandoned := tree.Abandoned["ui-abandon"]
	if abandoned == nil {
		t.Fatalf("the subject was not abandoned: %+v", tree.Live)
	}
	assertSessionLine(t, abandoned.History[len(abandoned.History)-1])
}
