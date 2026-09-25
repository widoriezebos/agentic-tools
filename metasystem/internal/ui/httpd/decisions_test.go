package httpd

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/rulings"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/decisions"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/project"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/snapshot"
)

// The Decisions route: one read of six answers this server already has.

// decisionsInfo is a server that can answer the page: a project, a ledger,
// this seat's asks, the register, and a clock.
func decisionsInfo() Info {
	return Info{
		Project: func() (project.Pane, error) { return describedPane(), nil },
		Observe: func() snapshot.Observation { return overviewObservation() },
		Now:     func() time.Time { return overviewNow },
		Asks: func() ([]channel.Question, error) {
			return []channel.Question{{
				ID: "q-1", Goal: "g1-s22", Kind: "decision", Machine: "m1e",
				OpenedAt: overviewNow.Add(-2 * time.Hour), State: "open",
				Facts:          []string{"two readers disagree about the seat key"},
				Wants:          "the seat key spelled one way",
				Options:        []channel.Option{{Label: "machine", Consequence: "two seats on one host collide"}},
				Recommendation: "machine+lineage",
			}}, nil
		},
		Rulings: func() (rulings.Register, error) {
			return rulings.Register{
				Rows: []rulings.Row{{
					ID: "R-1", Date: "2026-09-01", Words: "The board's two drag moves are the only acts, and g1-s22 is where they land",
					Context: "given with the board design", Owner: "Wido",
					Condition: "class=temporary due=2026-09-20", Class: "temporary", Due: "2026-09-20",
				}},
				Reviews: []rulings.Review{{ID: "R-1", Owner: "Wido", Class: "temporary", Due: "2026-09-20"}},
				Defects: []rulings.Defect{{Label: "R-9", Reason: "review condition needs due= or event="}},
			}, nil
		},
	}
}

func decisionsPage(t *testing.T, served http.Handler, named string) decisions.Page {
	t.Helper()
	response := request(t, served, http.MethodGet, "/api/decisions", "127.0.0.1:7878", nil)
	testutil.Require(t, named+" status", response.Code, http.StatusOK)
	testutil.Expect(t, named+" content type", response.Header().Get("Content-Type"), "application/json")
	var page decisions.Page
	testutil.Require(t, "decode "+named, json.Unmarshal(response.Body.Bytes(), &page), nil)
	return page
}

// The route answers the composed page, with the six answers in it.
func TestDecisionsPayload(t *testing.T) {
	t.Parallel()

	served := New(decisionsInfo(), loopback(), testBundle())
	page := decisionsPage(t, served, "the read")

	testutil.Expect(t, "the schema version", page.SchemaVersion, decisions.SchemaVersion)
	testutil.Expect(t, "when it was read", page.ReadAt, overviewNow.Format(time.RFC3339))
	testutil.Expect(t, "a seat nothing proves asks for a sign-in", page.SignIn, true)

	kinds := map[string]int{}
	for _, need := range page.NeedsYou {
		kinds[need.Kind]++
	}
	// The ledger's one queued goal carries no approval; the register's one
	// review came due five days before this clock; and the seat's own
	// question is the third row.
	testutil.Expect(t, "an approval waiting", kinds[decisions.KindApproval], 1)
	testutil.Expect(t, "a ruling review that came due", kinds[decisions.KindRulingReview], 1)
	testutil.Expect(t, "this seat's own ask", kinds[decisions.KindAsk], 1)
	testutil.Expect(t, "the inbox count is the inbox", page.Counts.NeedsYou, len(page.NeedsYou))

	testutil.Require(t, "how many rulings", len(page.Decided.Rulings), 1)
	testutil.Expect(t, "the register's words came through whole",
		page.Decided.Rulings[0].Words,
		"The board's two drag moves are the only acts, and g1-s22 is where they land")
	testutil.Expect(t, "and the goal it names is a mention",
		page.Decided.Rulings[0].Mentions, []string{"g1-s22"})
	testutil.Expect(t, "the register's defect is listed",
		page.Decided.Defects, []string{"R-9: review condition needs due= or event="})
}

// A build with no channel reader and no register reader still answers: both
// are ordinary states of an adopted repository, and each costs the page one
// block and nothing else.
func TestDecisionsWithoutAChannelOrARegister(t *testing.T) {
	t.Parallel()

	info := decisionsInfo()
	info.Asks, info.Rulings = nil, nil
	page := decisionsPage(t, New(info, loopback(), testBundle()), "the read")

	for _, need := range page.NeedsYou {
		if need.Kind == decisions.KindAsk || need.Kind == decisions.KindRulingReview {
			t.Fatalf("a build with no reader invented a %s row: %+v", need.Kind, need)
		}
	}
	testutil.Expect(t, "no rulings", len(page.Decided.Rulings), 0)
	testutil.Expect(t, "and no register count", page.Counts.Rulings, 0)
	testutil.Expect(t, "the approval still waits", page.Counts.NeedsYou, 1)
}

// A reader that fails is a 500 carrying its own reason, like every other
// read: a page that swallowed it would say this human has decided nothing.
func TestDecisionsSaysWhyAReaderFailed(t *testing.T) {
	t.Parallel()

	for _, one := range []struct {
		name   string
		shape  func(*Info)
		reason string
	}{
		{
			name:   "no project reader",
			shape:  func(info *Info) { info.Project = nil },
			reason: "this engine was built without a project reader",
		},
		{
			name:   "no ledger reader",
			shape:  func(info *Info) { info.Observe = nil },
			reason: "this engine was built without a ledger reader",
		},
		{
			name: "a register this seat could not read",
			shape: func(info *Info) {
				info.Rulings = func() (rulings.Register, error) {
					return rulings.Register{}, errors.New("memory/rulings.md: permission denied")
				}
			},
			reason: "memory/rulings.md: permission denied",
		},
		{
			name: "a channel this seat could not walk",
			shape: func(info *Info) {
				info.Asks = func() ([]channel.Question, error) {
					return nil, errors.New("the question directory could not be listed")
				}
			},
			reason: "the question directory could not be listed",
		},
	} {
		t.Run(one.name, func(t *testing.T) {
			t.Parallel()

			info := decisionsInfo()
			one.shape(&info)
			served := New(info, loopback(), testBundle())

			response := request(t, served, http.MethodGet, "/api/decisions", "127.0.0.1:7878", nil)

			testutil.Require(t, "status", response.Code, http.StatusInternalServerError)
			var refusal struct {
				Error string `json:"error"`
			}
			testutil.Require(t, "decode the refusal", json.Unmarshal(response.Body.Bytes(), &refusal), nil)
			testutil.Expect(t, "the reason", refusal.Error, one.reason)
		})
	}
}

// The resource is matched exactly: what lies beneath it belongs to no
// resource and is a 404 like any other unserved path under /api.
func TestDecisionsMatchesExactly(t *testing.T) {
	t.Parallel()

	served := New(decisionsInfo(), loopback(), testBundle())
	response := request(t, served, http.MethodGet, "/api/decisions/R-1", "127.0.0.1:7878", nil)
	testutil.Expect(t, "status", response.Code, http.StatusNotFound)
}
