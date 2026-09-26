package httpd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner/fakeacp"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/uitools"
)

// The one route that writes what a press did to a proposed action.
//
// It carries no authority and makes no act: the act itself went to the ledger's
// own route under the human's session, and this writes down what that answered
// where the proposal is. What it must do is refuse a write that would let two
// tabs move one line — because an approve applied twice is two approval records,
// and the card is the only thing that remembers a line was applied.
//
// The version is the whole mechanism. A state comparison cannot tell a fresh
// attempt from an abandoned one once a line returns to `applying`; the version
// can, because it moves under every admitted write.

// proposedPark is the canned propose call these tests run: one park on a goal the
// fixture ledger carries.
func proposedPark(goalID string) fakeacp.Read {
	return fakeacp.Read{
		Name:  "mcp__" + uitools.ServerName + "__" + uitools.OpPropose,
		Title: "propose(park-goal " + goalID + ")",
		Result: uitools.PreparedProposalLine + "\n" +
			uitools.ProposalHeader + uitools.ProposePark + "\n" +
			uitools.ProposalGoal + goalID + "\n" +
			uitools.ProposalBecause + "superseded by the seat inventory\n" +
			uitools.ProposalSeparator + "\nput it away\n",
	}
}

// servedProposing is a served Partner whose conversation owner can read the
// ledger, which is what an action is admitted against. The read route's own
// harness gives the service no reader — the turns it drives propose nothing —
// so this one is built here rather than that one widened.
func servedProposing(t *testing.T, script fakeacp.Script) (http.Handler, *partner.Service) {
	t.Helper()
	root := t.TempDir()
	runtime := partner.Runtime{Name: "fake", Model: "fake-1", ReadOnly: "a fake server reads nothing"}
	script.Models = []string{"fake-1"}
	host := partner.NewHostOn(runtime, root, fakeacp.Open(script))
	t.Cleanup(host.Close)
	service := partner.NewService(runtime, host,
		func(human string) (*partner.Conversation, error) { return partner.OpenConversation(root, human) },
		partner.Facts{Observe: readObservation},
		func() time.Time { return time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC) })
	info := Info{
		Observe:           readObservation,
		Authority:         proven(),
		Partner:           service,
		PartnerConfigured: true,
	}
	return New(info, loopback(), testBundle()), service
}

// proposing is a served Partner that has answered one turn with one proposed
// action, and the turn that answer belongs to.
func proposing(t *testing.T) (http.Handler, string) {
	t.Helper()
	served, service := servedProposing(t, fakeacp.Script{
		Reads:  []fakeacp.Read{proposedPark("waiting")},
		Chunks: []string{"I have proposed it."},
	})
	events, stop := service.Subscribe()
	defer stop()
	accepted := post(t, served, partnerTurnsPath,
		`{"key":"k1","text":"put it away","about":{"section":"Decisions"}}`, nil)
	testutil.Require(t, "the turn is admitted", accepted.Code, http.StatusAccepted)
	drain(t, events)

	snapshot := partnerSnapshot(t, get(t, served, partnerPath, nil))
	answered := snapshot.Messages[len(snapshot.Messages)-1]
	testutil.Require(t, "the answer carries one action", len(answered.Proposals), 1)
	testutil.Require(t, "which is offered", answered.Proposals[0].Offered, true)
	testutil.Require(t, "at version one", answered.Proposals[0].Version, 1)
	return served, answered.Turn
}

func proposalPath(turn string, index int) string {
	return partnerTurnsPre + turn + proposalsInfix + strconv.Itoa(index)
}

// wrote is one press on one line. It answers the response and the entry as the
// route handed it back.
func wrote(t *testing.T, served http.Handler, turn string, index, version int, state, words string) (*httptest.ResponseRecorder, partner.Proposal) {
	t.Helper()
	body := fmt.Sprintf(`{"version":%d,"state":%q,"words":%q}`, version, state, words)
	response := post(t, served, proposalPath(turn, index), body, nil)
	var answered struct {
		Proposal partner.Proposal `json:"proposal"`
	}
	_ = json.Unmarshal(response.Body.Bytes(), &answered)
	return response, answered.Proposal
}

// Every allowed pair is admitted with a matching version, and every admitted
// write moves the version on.
func TestEveryAllowedPairIsAdmittedAndMovesTheVersion(t *testing.T) {
	t.Parallel()
	// The pairs, in the order one line can actually walk them: waiting to
	// applying to unresolved, back to applying, to refused, to applying again, to
	// applied. The walk is the table read as a path, so no pair is asserted in
	// isolation from the line it belongs to.
	for _, walk := range [][]string{
		{partner.ProposalApplying, partner.ProposalApplied},
		{partner.ProposalApplying, partner.ProposalRefused, partner.ProposalApplying, partner.ProposalApplied},
		{partner.ProposalApplying, partner.ProposalUnresolved, partner.ProposalApplying,
			partner.ProposalApplying, partner.ProposalApplied},
		{partner.ProposalDismissed},
		{partner.ProposalApplying, partner.ProposalDismissed},
		{partner.ProposalApplying, partner.ProposalRefused, partner.ProposalDismissed},
		{partner.ProposalApplying, partner.ProposalUnresolved, partner.ProposalDismissed},
	} {
		t.Run(strings.Join(walk, "-"), func(t *testing.T) {
			t.Parallel()
			served, turn := proposing(t)
			version := 1
			for at, state := range walk {
				step := strconv.Itoa(at) + " " + state
				response, held := wrote(t, served, turn, 0, version, state, "what the act said")
				testutil.Require(t, "writing "+step+" is admitted", response.Code, http.StatusOK)
				testutil.Expect(t, "the line says "+step, held.State, state)
				testutil.Expect(t, "and the version moved at "+step, held.Version, version+1)
				version = held.Version
			}
		})
	}
}

// A pair the line may not pass through is refused with the entry as it stands,
// and nothing changes.
func TestAPairTheLineMayNotPassThroughIsRefusedWithTheEntry(t *testing.T) {
	t.Parallel()
	for what, walk := range map[string][]string{
		"applied straight from waiting":    {partner.ProposalApplied},
		"refused straight from waiting":    {partner.ProposalRefused},
		"unresolved straight from waiting": {partner.ProposalUnresolved},
		"waiting again":                    {partner.ProposalWaiting},
		"applied twice":                    {partner.ProposalApplying, partner.ProposalApplied, partner.ProposalApplied},
		"applying after it was applied":    {partner.ProposalApplying, partner.ProposalApplied, partner.ProposalApplying},
		"anything after it was dismissed":  {partner.ProposalDismissed, partner.ProposalApplying},
		"unresolved from a refused line":   {partner.ProposalApplying, partner.ProposalRefused, partner.ProposalUnresolved},
	} {
		t.Run(what, func(t *testing.T) {
			t.Parallel()
			served, turn := proposing(t)
			version := 1
			standing := partner.ProposalWaiting
			for at, state := range walk[:len(walk)-1] {
				response, held := wrote(t, served, turn, 0, version, state, "")
				testutil.Require(t, "the walk up to it is admitted at "+strconv.Itoa(at),
					response.Code, http.StatusOK)
				version, standing = held.Version, held.State
			}
			response, held := wrote(t, served, turn, 0, version, walk[len(walk)-1], "")
			testutil.Expect(t, "it is refused", response.Code, http.StatusConflict)
			testutil.Expect(t, "under the state code", codeOf(t, response), "state")
			// The entry comes back as it stands and nothing moved, so the page can
			// show the line as it really is rather than as it rendered it.
			testutil.Expect(t, "it hands back the entry's own state", held.State, standing)
			testutil.Expect(t, "at the version it really is", held.Version, version)
			after := partnerSnapshot(t, get(t, served, partnerPath, nil))
			kept := after.Messages[len(after.Messages)-1].Proposals[0]
			testutil.Expect(t, "and the transcript is untouched", kept.State, standing)
			testutil.Expect(t, "at that version", kept.Version, version)
		})
	}
}

// A write whose version has moved is refused even when the pair is one the line
// could pass through: a stale tab has read a line that has since changed.
func TestAWriteWithAStaleVersionIsRefusedWithTheEntry(t *testing.T) {
	t.Parallel()
	served, turn := proposing(t)
	// One tab writes applying and the line moves to version 2.
	response, held := wrote(t, served, turn, 0, 1, partner.ProposalApplying, "")
	testutil.Require(t, "the first write is admitted", response.Code, http.StatusOK)
	testutil.Require(t, "the version moved", held.Version, 2)

	// A second tab still showing the line as waiting at version 1 presses Apply.
	// The pair waiting-to-applying is allowed; the version is not.
	stale, standing := wrote(t, served, turn, 0, 1, partner.ProposalApplying, "")
	testutil.Expect(t, "the stale write is refused", stale.Code, http.StatusConflict)
	testutil.Expect(t, "under the state code", codeOf(t, stale), "state")
	testutil.Expect(t, "and is handed the line as it stands", standing.State, partner.ProposalApplying)
	testutil.Expect(t, "at the version it really is", standing.Version, 2)
}

// Two writers racing on one line with the same version: exactly one is admitted.
func TestTwoWritersRacingOnOneLineAdmitExactlyOne(t *testing.T) {
	t.Parallel()
	for _, state := range []string{partner.ProposalApplying, partner.ProposalDismissed} {
		t.Run(state, func(t *testing.T) {
			t.Parallel()
			served, turn := proposing(t)
			var running sync.WaitGroup
			codes := make([]int, 8)
			for at := range codes {
				running.Add(1)
				go func(at int) {
					defer running.Done()
					body := fmt.Sprintf(`{"version":1,"state":%q,"words":""}`, state)
					// The recorder is this goroutine's own, so nothing here shares
					// one: what is compared afterwards is the status each got.
					codes[at] = post(t, served, proposalPath(turn, 0), body, nil).Code
				}(at)
			}
			running.Wait()
			admitted, conflicted := 0, 0
			for _, code := range codes {
				switch code {
				case http.StatusOK:
					admitted++
				case http.StatusConflict:
					conflicted++
				}
			}
			testutil.Expect(t, "exactly one writer moved the line", admitted, 1)
			testutil.Expect(t, "and every other lost", conflicted, len(codes)-1)
		})
	}
}

// Two tabs pressing Try again on one line the page shows in flight: exactly one
// gets to send an act.
func TestTwoTabsPressingTryAgainOnOneInFlightLineAdmitOne(t *testing.T) {
	t.Parallel()
	served, turn := proposing(t)
	response, held := wrote(t, served, turn, 0, 1, partner.ProposalApplying, "")
	testutil.Require(t, "the line is in flight", response.Code, http.StatusOK)
	inFlight := held.Version

	// Both tabs render the line at the same version and both press Try again,
	// which is one write each, applying to applying.
	first, _ := wrote(t, served, turn, 0, inFlight, partner.ProposalApplying, "")
	second, standing := wrote(t, served, turn, 0, inFlight, partner.ProposalApplying, "")
	testutil.Expect(t, "the first press is admitted", first.Code, http.StatusOK)
	testutil.Expect(t, "the second is refused", second.Code, http.StatusConflict)
	testutil.Expect(t, "and is handed the line at the version the first left", standing.Version, inFlight+1)
}

// Try again racing Dismiss: at most one act, and the loser is handed the entry.
func TestTryAgainRacingDismissAdmitsOne(t *testing.T) {
	t.Parallel()
	served, turn := proposing(t)
	response, held := wrote(t, served, turn, 0, 1, partner.ProposalApplying, "")
	testutil.Require(t, "the line is in flight", response.Code, http.StatusOK)
	at := held.Version

	again, _ := wrote(t, served, turn, 0, at, partner.ProposalApplying, "")
	dismiss, standing := wrote(t, served, turn, 0, at, partner.ProposalDismissed, "")
	testutil.Expect(t, "Try again is admitted", again.Code, http.StatusOK)
	testutil.Expect(t, "Dismiss on the version it read is refused", dismiss.Code, http.StatusConflict)
	testutil.Expect(t, "and is handed the line in flight", standing.State, partner.ProposalApplying)
}

// A turn that is still running has no message to record an outcome on, which is
// also why the card's buttons are asleep until the answer's terminal beat.
func TestTheRouteRefusesARunningTurn(t *testing.T) {
	t.Parallel()
	served, service := servedProposing(t, fakeacp.Script{
		Reads:  []fakeacp.Read{proposedPark("waiting")},
		Chunks: []string{"one ", "two ", "three"},
		Pause:  30 * time.Millisecond,
	})
	events, stop := service.Subscribe()
	defer stop()
	accepted := post(t, served, partnerTurnsPath,
		`{"key":"k1","text":"put it away","about":{"section":"Decisions"}}`, nil)
	testutil.Require(t, "the turn is admitted", accepted.Code, http.StatusAccepted)
	var turn struct {
		Turn string `json:"turn"`
	}
	_ = json.Unmarshal(accepted.Body.Bytes(), &turn)
	waitForKind(t, events, partner.EventProposal)

	refused := post(t, served, proposalPath(turn.Turn, 0), `{"version":1,"state":"applying"}`, nil)
	testutil.Expect(t, "a running turn is refused", refused.Code, http.StatusBadRequest)
	testutil.Expect(t, "in words", messageOf(t, refused),
		"the Partner is still answering this turn, so its actions cannot be applied yet; "+
			"they wake when the answer ends")
	drain(t, events)
}

// An index the answer does not carry, a state that is not one of the six, and a
// turn this conversation does not have are each refused in words.
func TestTheRouteRefusesWhatItCannotName(t *testing.T) {
	t.Parallel()
	served, turn := proposing(t)
	for what, probe := range map[string]struct {
		path string
		body string
		says string
	}{
		"an index the answer does not carry": {proposalPath(turn, 4), `{"version":1,"state":"applying"}`,
			"that answer proposed 1 actions, so it has no action 4"},
		"a state that is not one of the six": {proposalPath(turn, 0), `{"version":1,"state":"landed"}`,
			"an action's state is waiting, applying, applied, refused, unresolved, dismissed, not landed"},
		"a turn this conversation has not": {proposalPath("not-a-turn", 0), `{"version":1,"state":"applying"}`,
			"this conversation has no answer for turn not-a-turn"},
		"an index that is not a number": {partnerTurnsPre + turn + proposalsInfix + "first",
			`{"version":1,"state":"applying"}`,
			"an action is named by its place in the answer, counting from zero"},
	} {
		t.Run(what, func(t *testing.T) {
			t.Parallel()
			response := post(t, served, probe.path, probe.body, nil)
			testutil.Expect(t, "it is refused", response.Code, http.StatusBadRequest)
			testutil.Expect(t, "and says why", messageOf(t, response), probe.says)
		})
	}
}

// The write rewrites that one message and leaves the rest of the transcript
// exactly as it was.
func TestTheWriteLeavesTheRestOfTheTranscriptAlone(t *testing.T) {
	t.Parallel()
	served, turn := proposing(t)
	before := partnerSnapshot(t, get(t, served, partnerPath, nil))
	testutil.Require(t, "a question and an answer", len(before.Messages), 2)

	response, _ := wrote(t, served, turn, 0, 1, partner.ProposalApplying, "")
	testutil.Require(t, "admitted", response.Code, http.StatusOK)

	after := partnerSnapshot(t, get(t, served, partnerPath, nil))
	testutil.Require(t, "still two messages", len(after.Messages), 2)
	testutil.Expect(t, "the question is untouched", after.Messages[0], before.Messages[0])
	answered := after.Messages[1]
	testutil.Expect(t, "the answer's own words are untouched", answered.Text, before.Messages[1].Text)
	testutil.Expect(t, "what it looked at is untouched", len(answered.Looked), len(before.Messages[1].Looked))
	testutil.Require(t, "and its one action moved", len(answered.Proposals), 1)
	testutil.Expect(t, "to applying", answered.Proposals[0].State, partner.ProposalApplying)
	testutil.Expect(t, "at version two", answered.Proposals[0].Version, 2)
	// And the route answered with the conversation as well, so one press is one
	// request.
	testutil.Expect(t, "the answer carries the conversation", len(before.Messages) > 0, true)
}

// codeOf and messageOf read a refusal's own body.
func codeOf(t *testing.T, response *httptest.ResponseRecorder) string {
	t.Helper()
	var refusal struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(response.Body.Bytes(), &refusal)
	return refusal.Code
}

func messageOf(t *testing.T, response *httptest.ResponseRecorder) string {
	t.Helper()
	var refusal struct {
		Error string `json:"error"`
	}
	_ = json.Unmarshal(response.Body.Bytes(), &refusal)
	return refusal.Error
}
