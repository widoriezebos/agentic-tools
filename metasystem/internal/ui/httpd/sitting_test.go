package httpd

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner/fakeacp"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/project"
)

// The sitting at the boundary: what the two routes admit, what they answer with,
// and what a sitting started on a draft asks the project's writer for. What a
// sitting IS, and what it refuses, is proved in internal/ui/partner; this layer
// proves the routes.

// servedSitting is the Partner routes over a fake runtime, with the project's
// writer recorded so a sitting started on a new draft can be seen reaching it.
func servedSitting(t *testing.T, script fakeacp.Script) (http.Handler, *partner.Service, *recorder) {
	t.Helper()
	root := t.TempDir()
	runtime := partner.Runtime{Name: "fake", Model: "fake-1", ReadOnly: "a fake server reads nothing"}
	script.Models = []string{"fake-1"}
	host := partner.NewHostOn(runtime, root, fakeacp.Open(script))
	t.Cleanup(host.Close)
	service := partner.NewService(runtime, host,
		func(human string) (*partner.Conversation, error) { return partner.OpenConversation(root, human) },
		partner.Facts{}, func() time.Time { return time.Date(2026, 9, 26, 9, 0, 0, 0, time.UTC) })
	rec := &recorder{}
	info := rec.writing()
	info.Observe = readObservation
	info.Authority = proven()
	info.Partner = service
	info.PartnerConfigured = true
	return New(info, loopback(), testBundle()), service, rec
}

const onARecord = `{"purpose":"shape a design",` +
	`"subject":{"kind":"record","id":"plans/designs/sessions.md","title":"Session limits"},` +
	`"about":{"section":"Project","path":"/project","kind":"record","subject":"plans/designs/sessions.md"}}`

// Starting a sitting on a record this checkout already has answers the
// conversation, with the sitting on it and the opening turn already in the
// transcript. It answers the snapshot rather than the sitting alone because the
// opening turn is a turn the page did not send.
func TestStartingASittingAnswersTheConversationWithTheOpeningTurnInIt(t *testing.T) {
	t.Parallel()
	served, service, rec := servedSitting(t, fakeacp.Script{Chunks: []string{"Two rulings touch this."}})
	events, stop := service.Subscribe()
	defer stop()

	started := post(t, served, partnerSittingPath, onARecord, nil)
	testutil.Require(t, "the sitting opened", started.Code, http.StatusOK)
	answer := partnerSnapshot(t, started)
	testutil.Require(t, "the answer carries the sitting", answer.Sitting != nil, true)
	testutil.Expect(t, "on the record the page named", answer.Sitting.Subject.ID, "plans/designs/sessions.md")
	testutil.Expect(t, "for what the human chose", answer.Sitting.Purpose, partner.PurposeShapeDesign)
	testutil.Require(t, "the opening turn is already in it", len(answer.Messages), 1)
	testutil.Expect(t, "marked as the interface's", answer.Messages[0].Interface, true)
	testutil.Expect(t, "asking for what the records hold",
		strings.Contains(answer.Messages[0].Text, "Bring what the records already hold about it"), true)
	testutil.Expect(t, "and no record was created for it", len(rec.records), 0)
	drain(t, events)

	// The read route says the same thing afterwards, from the conversation
	// rather than from this answer.
	warm := partnerSnapshot(t, get(t, served, partnerPath, nil))
	testutil.Require(t, "the sitting is still there", warm.Sitting != nil, true)
	testutil.Expect(t, "whole", warm.Sitting.Subject.Title, "Session limits")
}

// A sitting started with a title and no subject creates the draft first, through
// the project's own writer, in the home the purpose's own kind names — and the
// sitting is then about the record that writer answered with.
func TestASittingStartedOnADraftCreatesItFirstInTheKindThePurposeNames(t *testing.T) {
	t.Parallel()
	for _, probe := range []struct {
		purpose string
		kind    string
	}{
		{partner.PurposeShapeIntent, "intent"},
		{partner.PurposeShapeDesign, "design"},
	} {
		served, service, rec := servedSitting(t, fakeacp.Script{Chunks: []string{"Nothing is recorded yet."}})
		events, stop := service.Subscribe()
		started := post(t, served, partnerSittingPath,
			`{"purpose":"`+probe.purpose+`","title":"Session limits","about":{"section":"Project","path":"/project"}}`, nil)
		testutil.Require(t, probe.purpose+" opened", started.Code, http.StatusOK)
		drain(t, events)
		stop()

		testutil.Require(t, probe.purpose+" reached the writer once", len(rec.records), 1)
		testutil.Expect(t, probe.purpose+" asked for the kind it implies", rec.records[0].Kind, probe.kind)
		testutil.Expect(t, probe.purpose+" carried the title the human typed", rec.records[0].Title, "Session limits")
		testutil.Expect(t, probe.purpose+" named no goal on it", len(rec.records[0].Goals), 0)

		answer := partnerSnapshot(t, started)
		testutil.Require(t, probe.purpose+" carries a sitting", answer.Sitting != nil, true)
		testutil.Expect(t, probe.purpose+" is about the record the writer answered with",
			answer.Sitting.Subject.ID, writtenRecord().Path)
		testutil.Expect(t, probe.purpose+" addresses it as a record",
			answer.Sitting.Subject.Kind, partner.SubjectRecord)
		testutil.Expect(t, probe.purpose+" and titles it as the writer did",
			answer.Sitting.Subject.Title, writtenRecord().Record.Title)
	}
}

// A draft the project refuses refuses the whole act, with the project's own
// words and its own status: a sitting whose subject does not exist has nothing to
// record into, and nothing about the conversation is touched.
func TestADraftTheProjectRefusesRefusesTheSitting(t *testing.T) {
	t.Parallel()
	served, _, rec := servedSitting(t, fakeacp.Script{Chunks: []string{"never asked"}})
	rec.refusal = &project.Refusal{Kind: project.RefusalBad, Message: "a record needs a title"}

	refused := post(t, served, partnerSittingPath,
		`{"purpose":"shape a design","title":"  ","about":{"section":"Project","path":"/project"}}`, nil)
	testutil.Expect(t, "the status is the project's own", refused.Code, http.StatusBadRequest)
	testutil.Expect(t, "in the project's own words",
		strings.Contains(refused.Body.String(), "a record needs a title"), true)

	cold := partnerSnapshot(t, get(t, served, partnerPath, nil))
	testutil.Expect(t, "no sitting was opened", cold.Sitting == nil, true)
	testutil.Expect(t, "and nothing was appended", len(cold.Messages), 0)
}

// A sitting this build does not offer is refused as a bad request, in the
// service's own words, and the conversation is untouched.
func TestASittingThisBuildDoesNotOfferIsRefusedAsABadRequest(t *testing.T) {
	t.Parallel()
	served, _, rec := servedSitting(t, fakeacp.Script{Chunks: []string{"never asked"}})

	refused := post(t, served, partnerSittingPath,
		`{"purpose":"review","subject":{"kind":"record","id":"plans/designs/sessions.md"},"about":{}}`, nil)
	testutil.Expect(t, "it is a bad request", refused.Code, http.StatusBadRequest)
	testutil.Expect(t, "saying which purposes this build has",
		strings.Contains(refused.Body.String(), "review and learning sittings are not in this build"), true)
	testutil.Expect(t, "and no record was created on the way", len(rec.records), 0)

	cold := partnerSnapshot(t, get(t, served, partnerPath, nil))
	testutil.Expect(t, "no sitting", cold.Sitting == nil, true)
	testutil.Expect(t, "and no messages", len(cold.Messages), 0)
}

// Ending the sitting answers the conversation without it, and leaves the
// transcript exactly where it was: what was recorded is in the record.
func TestEndingTheSittingAnswersTheConversationWithoutIt(t *testing.T) {
	t.Parallel()
	served, service, _ := servedSitting(t, fakeacp.Script{Chunks: []string{"Two rulings touch this."}})
	events, stop := service.Subscribe()
	defer stop()
	testutil.Require(t, "the sitting opened",
		post(t, served, partnerSittingPath, onARecord, nil).Code, http.StatusOK)
	drain(t, events)

	ended := post(t, served, partnerSittingEndPath, `{}`, nil)
	testutil.Require(t, "it ended", ended.Code, http.StatusOK)
	answer := partnerSnapshot(t, ended)
	testutil.Expect(t, "with no sitting on the conversation", answer.Sitting == nil, true)
	testutil.Expect(t, "and the transcript where it was", len(answer.Messages), 2)
}

// Both routes are POST and both are refused on every other verb, exactly as the
// Partner's other two writes are.
func TestTheSittingRoutesTakePostAndNothingElse(t *testing.T) {
	t.Parallel()
	served, _, _ := servedSitting(t, fakeacp.Script{Chunks: []string{"never asked"}})
	for _, path := range []string{partnerSittingPath, partnerSittingEndPath} {
		answered := get(t, served, path, nil)
		testutil.Expect(t, path+" is not read", answered.Code, http.StatusMethodNotAllowed)
		testutil.Expect(t, path+" says which verb it takes", answered.Header().Get("Allow"), "POST")
	}
}

// A seat with no Partner answers 503 with the refusal's own words, so a human
// who cannot start a sitting reads why rather than meeting a silent failure.
func TestASeatWithNoPartnerRefusesToStartASitting(t *testing.T) {
	t.Parallel()
	served := New(Info{Observe: readObservation, Authority: proven()}, loopback(), testBundle())
	for _, path := range []string{partnerSittingPath, partnerSittingEndPath} {
		refused := post(t, served, path, `{}`, nil)
		testutil.Expect(t, path+" is unavailable", refused.Code, http.StatusServiceUnavailable)
		testutil.Expect(t, path+" says what to configure",
			strings.Contains(refused.Body.String(), "ui.partner.runtime"), true)
	}
}
