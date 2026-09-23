package httpd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner/fakeacp"
)

// The Partner at the boundary: what the read route answers, what a send is
// admitted or refused as, and how a running turn's beats reach the page. What
// a turn does is proved in internal/ui/partner; this layer proves the routes.

func servedPartner(t *testing.T, script fakeacp.Script) (http.Handler, *partner.Service) {
	t.Helper()
	root := t.TempDir()
	runtime := partner.Runtime{Name: "fake", Model: "fake-1", ReadOnly: "a fake server reads nothing"}
	// The fake offers the model this seat asked for, so the send goes through
	// the selection the host makes at session setup rather than round it.
	script.Models = []string{"fake-1"}
	host := partner.NewHostOn(runtime, root, fakeacp.Open(script))
	t.Cleanup(host.Close)
	service := partner.NewService(runtime, host,
		func(human string) (*partner.Conversation, error) { return partner.OpenConversation(root, human) },
		partner.Facts{}, func() time.Time { return time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC) })
	info := Info{
		Observe:           readObservation,
		Authority:         proven(),
		Partner:           service,
		PartnerConfigured: true,
	}
	return New(info, loopback(), testBundle()), service
}

func partnerSnapshot(t *testing.T, response *httptest.ResponseRecorder) partner.Snapshot {
	t.Helper()
	var snapshot partner.Snapshot
	if err := json.Unmarshal(response.Body.Bytes(), &snapshot); err != nil {
		t.Fatalf("the Partner snapshot must decode: %v\n%s", err, response.Body.Bytes())
	}
	return snapshot
}

// The read route answers everything a page needs from cold.
func TestThePartnerReadRouteAnswersTheRuntimeAndTheConversation(t *testing.T) {
	t.Parallel()
	served, service := servedPartner(t, fakeacp.Script{Chunks: []string{"two goals"}})
	events, stop := service.Subscribe()
	defer stop()

	cold := partnerSnapshot(t, get(t, served, partnerPath, nil))
	testutil.Expect(t, "which runtime", cold.Runtime, "fake")
	testutil.Expect(t, "which model", cold.Model, "fake-1")
	testutil.Expect(t, "whose conversation", cold.Human, "Wido")
	testutil.Expect(t, "nothing running", cold.Busy, false)
	testutil.Expect(t, "no messages yet", len(cold.Messages), 0)
	testutil.Expect(t, "and what makes it read-only", cold.ReadOnly, "a fake server reads nothing")

	accepted := post(t, served, partnerTurnsPath, `{"key":"k1","text":"which goals are ready?","about":{"section":"Backlog"}}`, nil)
	testutil.Require(t, "accepted", accepted.Code, http.StatusAccepted)
	drain(t, events)

	warm := partnerSnapshot(t, get(t, served, partnerPath, nil))
	testutil.Require(t, "both messages", len(warm.Messages), 2)
	testutil.Expect(t, "the question", warm.Messages[0].Text, "which goals are ready?")
	testutil.Expect(t, "the answer", warm.Messages[1].Text, "two goals")
	testutil.Expect(t, "and its outcome", warm.Messages[1].Outcome, partner.OutcomeComplete)
}

// A send answers 202 with the turn, and the same key again answers the same
// turn rather than asking twice.
func TestASendIsAcceptedOnceAndIsIdempotentOnItsKey(t *testing.T) {
	t.Parallel()
	served, service := servedPartner(t, fakeacp.Script{Chunks: []string{"once"}})
	events, stop := service.Subscribe()
	defer stop()

	first := post(t, served, partnerTurnsPath, `{"key":"k1","text":"hello","about":{}}`, nil)
	testutil.Require(t, "accepted", first.Code, http.StatusAccepted)
	drain(t, events)
	again := post(t, served, partnerTurnsPath, `{"key":"k1","text":"hello","about":{}}`, nil)
	testutil.Require(t, "accepted again", again.Code, http.StatusAccepted)
	testutil.Expect(t, "the same turn", turnOf(t, again), turnOf(t, first))
	snapshot := partnerSnapshot(t, get(t, served, partnerPath, nil))
	testutil.Expect(t, "and nothing was asked twice", len(snapshot.Messages), 2)
}

// A second send while a turn runs answers 409, so the page keeps its draft.
func TestASendWhileBusyAnswersConflict(t *testing.T) {
	t.Parallel()
	served, service := servedPartner(t, fakeacp.Script{
		Chunks: []string{"a", "b", "c"}, Pause: 200 * time.Millisecond})
	events, stop := service.Subscribe()
	defer stop()
	first := post(t, served, partnerTurnsPath, `{"key":"k1","text":"first","about":{}}`, nil)
	testutil.Require(t, "accepted", first.Code, http.StatusAccepted)
	waitForKind(t, events, partner.EventText)
	second := post(t, served, partnerTurnsPath, `{"key":"k2","text":"second","about":{}}`, nil)
	testutil.Require(t, "refused", second.Code, http.StatusConflict)
	testutil.Expect(t, "as busy", actRefusal(t, second).Code, "busy")
	drain(t, events)
}

// A runtime that cannot start answers 503 with its own words and the line that
// installs it.
func TestARuntimeThatCannotStartAnswersUnavailableWithItsWords(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	runtime := partner.Runtime{Name: "claude", Install: "npm install -g @agentclientprotocol/claude-agent-acp"}
	host := partner.NewHostOn(runtime, root, fakeacp.Open(fakeacp.Script{InitError: "Authentication required"}))
	t.Cleanup(host.Close)
	service := partner.NewService(runtime, host,
		func(human string) (*partner.Conversation, error) { return partner.OpenConversation(root, human) },
		partner.Facts{}, nil)
	served := New(Info{Authority: proven(), Partner: service, PartnerConfigured: true}, loopback(), testBundle())

	response := post(t, served, partnerTurnsPath, `{"key":"k1","text":"hello","about":{}}`, nil)
	testutil.Require(t, "unavailable", response.Code, http.StatusServiceUnavailable)
	var body struct {
		Error   string `json:"error"`
		Code    string `json:"code"`
		Install string `json:"install"`
	}
	testutil.Require(t, "decode", json.Unmarshal(response.Body.Bytes(), &body), nil)
	testutil.Expect(t, "the runtime's words", body.Error, "Authentication required")
	testutil.Expect(t, "and the install line", body.Install, "npm install -g @agentclientprotocol/claude-agent-acp")
}

// A seat with no Partner says so on every Partner route rather than pretending
// to have one.
func TestASeatWithNoPartnerSaysSoOnItsRoutes(t *testing.T) {
	t.Parallel()
	served := New(Info{Authority: proven()}, loopback(), testBundle())
	read := get(t, served, partnerPath, nil)
	testutil.Require(t, "unavailable", read.Code, http.StatusServiceUnavailable)
	testutil.Expect(t, "it names the setting",
		strings.Contains(actRefusal(t, read).Error, "ui.partner.runtime"), true)
	send := post(t, served, partnerTurnsPath, `{"key":"k1","text":"hello","about":{}}`, nil)
	testutil.Require(t, "unavailable too", send.Code, http.StatusServiceUnavailable)
}

// A configured runtime that could not be admitted answers with the admission's
// own refusal.
func TestARefusedAdmissionIsWhatThePartnerRouteAnswers(t *testing.T) {
	t.Parallel()
	served := New(Info{
		Authority:         proven(),
		PartnerConfigured: true,
		PartnerRefusal:    "gemini is not a Partner runtime this build admits",
	}, loopback(), testBundle())
	read := get(t, served, partnerPath, nil)
	testutil.Require(t, "unavailable", read.Code, http.StatusServiceUnavailable)
	testutil.Expect(t, "in the admission's words", actRefusal(t, read).Error,
		"gemini is not a Partner runtime this build admits")
}

// Stop ends the running turn and answers the settled snapshot.
func TestTheStopRouteEndsTheTurnAndAnswersTheSnapshot(t *testing.T) {
	t.Parallel()
	served, service := servedPartner(t, fakeacp.Script{
		Chunks: []string{"one ", "two ", "three"}, Pause: 200 * time.Millisecond})
	events, stop := service.Subscribe()
	defer stop()
	accepted := post(t, served, partnerTurnsPath, `{"key":"k1","text":"a long one","about":{}}`, nil)
	testutil.Require(t, "accepted", accepted.Code, http.StatusAccepted)
	waitForKind(t, events, partner.EventText)
	stopped := post(t, served, partnerTurnsPre+turnOf(t, accepted)+stopSuffix, `{}`, nil)
	testutil.Require(t, "stopped", stopped.Code, http.StatusOK)
	drain(t, events)
	snapshot := partnerSnapshot(t, get(t, served, partnerPath, nil))
	testutil.Expect(t, "the transcript says stopped", snapshot.Messages[1].Outcome, partner.OutcomeStopped)
}

// The Partner's beats ride the page's one stream, under their own event type
// and with no id of their own, so Last-Event-ID stays the notification
// journal's cursor.
func TestPartnerEventsRideTheOneStreamWithNoIdOfTheirOwn(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	runtime := partner.Runtime{Name: "fake"}
	host := partner.NewHostOn(runtime, root, fakeacp.Open(fakeacp.Script{
		Activity: "Read plans/goals/backlog.md", Chunks: []string{"an answer"}}))
	t.Cleanup(host.Close)
	service := partner.NewService(runtime, host,
		func(human string) (*partner.Conversation, error) { return partner.OpenConversation(root, human) },
		partner.Facts{}, nil)
	info := Info{
		Authority: proven(), Partner: service, PartnerConfigured: true,
		NotificationJournal: journalWith(t),
	}
	stream := streamedFrom(t, info, "")
	// The send goes to a second handler over the same service, which is what a
	// browser does: the stream is one request and the send is another.
	served := New(info, loopback(), testBundle())
	accepted := post(t, served, partnerTurnsPath, `{"key":"k1","text":"hello","about":{}}`, nil)
	testutil.Require(t, "accepted", accepted.Code, http.StatusAccepted)

	var kinds []string
	for at := 0; at < 3; at++ {
		id, name, data := stream.event(t)
		testutil.Expect(t, "the event type is the Partner's "+strconv.Itoa(at), name, "partner")
		testutil.Expect(t, "and it carries no id of its own "+strconv.Itoa(at), id, "")
		var event partner.Event
		testutil.Require(t, "decode "+strconv.Itoa(at), json.Unmarshal([]byte(data), &event), nil)
		kinds = append(kinds, event.Kind)
		if event.Kind == partner.EventDone {
			break
		}
	}
	testutil.Expect(t, "the beats in order", kinds,
		[]string{partner.EventActivity, partner.EventText, partner.EventDone})
}

func turnOf(t *testing.T, response *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Turn string `json:"turn"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("an accepted turn must decode: %v\n%s", err, response.Body.Bytes())
	}
	return body.Turn
}

// drain reads the event stream until the turn ends.
func drain(t *testing.T, events <-chan partner.Event) {
	t.Helper()
	deadline := time.NewTimer(20 * time.Second)
	defer deadline.Stop()
	for {
		select {
		case event := <-events:
			if event.Kind == partner.EventDone || event.Kind == partner.EventStopped || event.Kind == partner.EventError {
				return
			}
		case <-deadline.C:
			t.Fatal("the turn never ended")
			return
		}
	}
}

func waitForKind(t *testing.T, events <-chan partner.Event, kind string) {
	t.Helper()
	deadline := time.NewTimer(20 * time.Second)
	defer deadline.Stop()
	for {
		select {
		case event := <-events:
			if event.Kind == kind {
				return
			}
		case <-deadline.C:
			t.Fatalf("no %s beat arrived", kind)
			return
		}
	}
}
