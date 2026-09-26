package httpd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner/fakeacp"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/project"
)

// The close at the boundary, and the Sittings list the project payload carries
// (g1-s55 D2, D3).
//
// What a close IS is proved in internal/ui/partner; this layer proves the route:
// that it asks one turn and ends nothing, that a seat with no Partner says so,
// and that the read route resumes a standing sitting whose session has gone.

// Ending a sitting asks the closing turn and leaves the sitting standing, so the
// card the turn offers is admitted against the record the sitting is on.
func TestTheCloseRouteAsksTheClosingTurnAndEndsNothing(t *testing.T) {
	t.Parallel()
	served, service, _ := servedSitting(t, fakeacp.Script{Chunks: []string{"Here is what it came to."}})
	events, stop := service.Subscribe()
	defer stop()
	testutil.Require(t, "the sitting opened",
		post(t, served, partnerSittingPath, onARecord, nil).Code, http.StatusOK)
	drain(t, events)

	closed := post(t, served, partnerSittingClosePath,
		`{"about":{"section":"Project","path":"/project"}}`, nil)
	testutil.Require(t, "the close was asked", closed.Code, http.StatusOK)
	answer := partnerSnapshot(t, closed)
	testutil.Require(t, "the sitting still stands", answer.Sitting != nil, true)
	testutil.Expect(t, "on the record it was on", answer.Sitting.Subject.ID, "plans/designs/sessions.md")
	testutil.Require(t, "the closing turn is already in the transcript", len(answer.Messages), 3)
	asked := answer.Messages[2]
	testutil.Expect(t, "marked as the interface's", asked.Interface, true)
	testutil.Expect(t, "asking for the closing deposit",
		strings.Contains(asked.Text, "Draft its closing deposit"), true)
	testutil.Expect(t, "of one kind", strings.Contains(asked.Text, "one deposit of kind outcome"), true)
	drain(t, events)

	// And ending it is still the other route, which is what the page reaches
	// after the human has recorded the outcome or left without it.
	ended := post(t, served, partnerSittingEndPath, `{}`, nil)
	testutil.Require(t, "it ended", ended.Code, http.StatusOK)
	testutil.Expect(t, "with no sitting on the conversation", partnerSnapshot(t, ended).Sitting == nil, true)
}

// Closing with no sitting open is a bad request in the service's own words, and
// the transcript is untouched.
func TestTheCloseRouteRefusesWithNoSittingOpen(t *testing.T) {
	t.Parallel()
	served, _, _ := servedSitting(t, fakeacp.Script{Chunks: []string{"never asked"}})
	refused := post(t, served, partnerSittingClosePath, `{}`, nil)
	testutil.Expect(t, "it is a bad request", refused.Code, http.StatusBadRequest)
	testutil.Expect(t, "saying there is nothing to close",
		strings.Contains(refused.Body.String(), "nothing to close"), true)
	cold := partnerSnapshot(t, get(t, served, partnerPath, nil))
	testutil.Expect(t, "and nothing was asked", len(cold.Messages), 0)
}

// The close is a POST like the other two, and a seat with no Partner says what
// to configure rather than failing silently.
func TestTheCloseRouteIsAPostAndSaysWhenThereIsNoPartner(t *testing.T) {
	t.Parallel()
	served, _, _ := servedSitting(t, fakeacp.Script{Chunks: []string{"never asked"}})
	answered := get(t, served, partnerSittingClosePath, nil)
	testutil.Expect(t, "it is not read", answered.Code, http.StatusMethodNotAllowed)
	testutil.Expect(t, "and says which verb it takes", answered.Header().Get("Allow"), "POST")

	bare := New(Info{Observe: readObservation, Authority: proven()}, loopback(), testBundle())
	refused := post(t, bare, partnerSittingClosePath, `{}`, nil)
	testutil.Expect(t, "a seat with no Partner is unavailable", refused.Code, http.StatusServiceUnavailable)
	testutil.Expect(t, "and says what to configure",
		strings.Contains(refused.Body.String(), "ui.partner.runtime"), true)
}

// Reading the conversation resumes a sitting whose session has ended: the page
// is handed a transcript that already holds the turn, and no browser effect
// submitted one.
func TestTheReadRouteResumesASittingWhoseSessionHasEnded(t *testing.T) {
	t.Parallel()
	served, service, _ := servedSitting(t, fakeacp.Script{Chunks: []string{"Here is what is on the table."}})
	events, stop := service.Subscribe()
	defer stop()
	testutil.Require(t, "the sitting opened",
		post(t, served, partnerSittingPath, onARecord, nil).Code, http.StatusOK)
	drain(t, events)

	// A read while the session is alive asks nothing: the Partner remembers.
	warm := partnerSnapshot(t, get(t, served, partnerPath, nil))
	testutil.Expect(t, "the transcript is where it was", len(warm.Messages), 2)

	// The session ends, as it does when this server restarts or the process is
	// torn down for being idle.
	service.Close()

	read := partnerSnapshot(t, get(t, served, partnerPath, nil))
	testutil.Require(t, "the sitting resumes with its chip", read.Sitting != nil, true)
	testutil.Expect(t, "on the record it was on", read.Sitting.Subject.ID, "plans/designs/sessions.md")
	testutil.Require(t, "and the resuming turn is already in the transcript", len(read.Messages) >= 3, true)
	again := read.Messages[2]
	testutil.Expect(t, "marked as the interface's", again.Interface, true)
	testutil.Expect(t, "said as a resuming", strings.Contains(again.Text, "Resuming this sitting"), true)
	testutil.Expect(t, "asking for what the records hold",
		strings.Contains(again.Text, "Bring what the records already hold about it"), true)
	drain(t, events)
}

// The project payload says which records were sat on, and marks the one a
// sitting stands on now — which the project reader cannot know, because the mark
// is on this human's conversation.
func TestTheProjectPayloadMarksTheSittingThatStandsNow(t *testing.T) {
	t.Parallel()
	served, service := servedSittingProject(t)
	events, stop := service.Subscribe()
	defer stop()

	before := projectPayload(t, get(t, served, projectPath, nil))
	testutil.Expect(t, "nothing is sat on yet", len(before.Sittings), 0)

	testutil.Require(t, "the sitting opened",
		post(t, served, partnerSittingPath, onARecord, nil).Code, http.StatusOK)
	drain(t, events)

	after := projectPayload(t, get(t, served, projectPath, nil))
	testutil.Require(t, "the standing sitting is listed", len(after.Sittings), 1)
	testutil.Expect(t, "by the record it is on", after.Sittings[0].Record.Path, "plans/designs/sessions.md")
	testutil.Expect(t, "with nothing recorded into it", after.Sittings[0].Counts, project.PileCounts{})
	testutil.Expect(t, "and it says a sitting stands", after.Sittings[0].Standing, true)

	testutil.Require(t, "it ended", post(t, served, partnerSittingEndPath, `{}`, nil).Code, http.StatusOK)
	ended := projectPayload(t, get(t, served, projectPath, nil))
	testutil.Expect(t, "and the list is empty again, because nothing was recorded", len(ended.Sittings), 0)
}

// servedSittingProject is the sitting's routes with a project reader beside
// them, so the payload the Sittings tab reads can be asked for.
func servedSittingProject(t *testing.T) (http.Handler, *partner.Service) {
	t.Helper()
	root := t.TempDir()
	runtime := partner.Runtime{Name: "fake", Model: "fake-1", ReadOnly: "a fake server reads nothing"}
	host := partner.NewHostOn(runtime, root, fakeacp.Open(fakeacp.Script{
		Models: []string{"fake-1"}, Chunks: []string{"Two rulings touch this."},
	}))
	t.Cleanup(host.Close)
	service := partner.NewService(runtime, host,
		func(human string) (*partner.Conversation, error) { return partner.OpenConversation(root, human) },
		partner.Facts{}, func() time.Time { return time.Date(2026, 9, 26, 9, 0, 0, 0, time.UTC) })
	info := Info{
		Observe: readObservation, Authority: proven(),
		Partner: service, PartnerConfigured: true,
		Project: func() (project.Pane, error) {
			// A project whose records include the one the sitting is on, and
			// whose own reading of them finds no sitting material: nothing has
			// been recorded into it yet, which is the state a standing sitting
			// starts in.
			return project.Pane{
				SchemaVersion: project.SchemaVersion,
				ReadAt:        "2026-09-26T09:00:00Z",
				Records: []project.Record{{
					Kind: "design", ID: "design-sessions", Status: "draft",
					Title: "Session limits", Path: "plans/designs/sessions.md", Home: "plans/designs",
				}},
			}, nil
		},
	}
	return New(info, loopback(), testBundle()), service
}

// projectPayload is the project resource as the Sittings tab reads it.
//
// It fails with t.Fatalf rather than through testutil, whose labels must be
// unique within one test and which a helper called three times would repeat.
func projectPayload(t *testing.T, response *httptest.ResponseRecorder) project.Pane {
	t.Helper()
	var payload project.Pane
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("read the project payload: %v: %s", err, response.Body.String())
	}
	return payload
}
