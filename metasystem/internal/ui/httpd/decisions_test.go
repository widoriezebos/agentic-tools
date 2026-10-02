package httpd

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/rulings"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/decisions"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/notifications"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner/fakeacp"
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
	testutil.Expect(t, "and the goal it names is a mention that opens the goal",
		page.Decided.Rulings[0].Mentions,
		[]decisions.Mention{{ID: "g1-s22", Where: decisions.Where{Kind: decisions.WhereGoal, ID: "g1-s22"}}})
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

// An alert inside the seven-day window reaches the inbox however busy the
// week was: the route reads pages of the journal until one of them reaches the
// window's start, rather than one page of two hundred that a noisy fortnight
// can fill entirely with lines addressed to nobody.
func TestTheAlertsInTheWindowAreReadPastTheFirstPageOfTheJournal(t *testing.T) {
	t.Parallel()

	lines := []string{}
	// Oldest first, which is the journal's own order: the one alert, then more
	// than a page of ticks recorded after it.
	lines = append(lines, journalAt(t, "n-alert", "alert",
		"the steward could not reach the operator", overviewNow.Add(-6*24*time.Hour)))
	for index := 0; index < notifications.DefaultLimit+20; index++ {
		lines = append(lines, journalAt(t, "n-tick-"+strconv.Itoa(index), "tick",
			"a tick ran", overviewNow.Add(-time.Duration(index+1)*time.Minute)))
	}

	info := decisionsInfo()
	info.NotificationJournal = journalWith(t, lines...)
	page := decisionsPage(t, New(info, loopback(), testBundle()), "the read")

	alerts := []decisions.Need{}
	for _, need := range page.NeedsYou {
		if need.Kind == decisions.KindAlert {
			alerts = append(alerts, need)
		}
	}
	testutil.Require(t, "how many alerts the inbox carries", len(alerts), 1)
	testutil.Expect(t, "and it is the one the second page holds", alerts[0].ID, "n-alert")
}

// A journal the interface cannot read is a 500 for this page too, and the
// reading of older pages does not swallow it.
func TestDecisionsSaysWhyTheJournalCouldNotBeRead(t *testing.T) {
	t.Parallel()

	info := decisionsInfo()
	info.NotificationJournal = filepath.Join(t.TempDir(), "notifications.jsonl")
	if err := os.Mkdir(info.NotificationJournal, 0o755); err != nil {
		t.Fatal(err)
	}
	response := request(t, New(info, loopback(), testBundle()), http.MethodGet, "/api/decisions", "127.0.0.1:7878", nil)
	testutil.Expect(t, "the status", response.Code, http.StatusInternalServerError)
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

// The actions the Partner proposed reach this page under the human its own
// routes resolve, so the card in the transcript and the row in the inbox are two
// readings of one transcript (g1-s60 D2).

// decisionsProposing is a server that can answer the page AND runs a Partner
// whose answer proposed one act on the one goal this fixture's ledger carries.
// It answers the handler and the turn that answer belongs to.
func decisionsProposing(t *testing.T) (http.Handler, string) {
	t.Helper()
	root := t.TempDir()
	runtime := partner.Runtime{Name: "fake", Model: "fake-1", ReadOnly: "a fake server reads nothing"}
	host := partner.NewHostOn(runtime, root, fakeacp.Open(fakeacp.Script{
		Models: []string{"fake-1"},
		Reads:  []fakeacp.Read{proposedPark("g1-s22")},
		Chunks: []string{"I have proposed it."},
	}))
	t.Cleanup(host.Close)
	service := partner.NewService(runtime, host,
		func(human string) (*partner.Conversation, error) { return partner.OpenConversation(root, human) },
		partner.Facts{Observe: func() snapshot.Observation { return overviewObservation() }},
		func() time.Time { return overviewNow })
	info := decisionsInfo()
	info.Partner, info.PartnerConfigured = service, true
	served := New(info, loopback(), testBundle())

	events, stop := service.Subscribe()
	defer stop()
	accepted := post(t, served, partnerTurnsPath,
		`{"key":"k1","text":"put it away","about":{"section":"Decisions"}}`, nil)
	testutil.Require(t, "the turn is admitted", accepted.Code, http.StatusAccepted)
	drain(t, events)
	read := partnerSnapshot(t, get(t, served, partnerPath, nil))
	answered := read.Messages[len(read.Messages)-1]
	testutil.Require(t, "the answer carries one action", len(answered.Proposals), 1)
	testutil.Require(t, "which is offered", answered.Proposals[0].Offered, true)
	return served, answered.Turn
}

// The proposal the Partner made and nobody has answered is a row of the inbox,
// with the action on it.
func TestDecisionsCarriesTheProposalsWaitingOnThisHuman(t *testing.T) {
	t.Parallel()

	served, turn := decisionsProposing(t)
	page := decisionsPage(t, served, "the read")

	rows := []decisions.Need{}
	for _, need := range page.NeedsYou {
		if need.Kind == decisions.KindProposal {
			rows = append(rows, need)
		}
	}
	// Nothing proves a human on this seat, so this is the one case where the
	// standing names nobody and the Partner's own resolution names the seat: the
	// row is here because both ends resolved the SAME transcript.
	testutil.Require(t, "nothing proves a human here", page.SignIn, true)
	testutil.Require(t, "how many proposals wait", len(rows), 1)
	testutil.Expect(t, "the id is the answer and the place in it", rows[0].ID, turn+"/0")
	testutil.Expect(t, "the act the press makes", rows[0].Act, decisions.ActApply)
	testutil.Require(t, "the action travels", rows[0].Proposal != nil, true)
	testutil.Expect(t, "the verb", rows[0].Proposal.Verb, "park-goal")
	testutil.Expect(t, "the state it stands in", rows[0].Proposal.State, partner.ProposalWaiting)
	testutil.Expect(t, "the version a press must present", rows[0].Proposal.Version, 1)
	testutil.Expect(t, "the goal's own row is joined", rows[0].Row != nil, true)
	testutil.Expect(t, "and the inbox counts it", page.Counts.NeedsYou, len(page.NeedsYou))
}

// A line the human has already applied leaves the inbox and settles on the card:
// the row is a reading of the one record the outcome route writes.
func TestDecisionsDropsAProposalOnceItIsSettled(t *testing.T) {
	t.Parallel()

	served, turn := decisionsProposing(t)
	// Applying is written before the act and the outcome after it, exactly as a
	// run writes them; the version moves under each.
	started := post(t, served, proposalPath(turn, 0), `{"version":1,"state":"applying","words":""}`, nil)
	testutil.Require(t, "the applying write is admitted", started.Code, http.StatusOK)
	applied := post(t, served, proposalPath(turn, 0), `{"version":2,"state":"applied","words":""}`, nil)
	testutil.Require(t, "the outcome write is admitted", applied.Code, http.StatusOK)

	page := decisionsPage(t, served, "the read after the run")

	for _, need := range page.NeedsYou {
		if need.Kind == decisions.KindProposal {
			t.Fatalf("an applied proposal is still in the inbox: %+v", need)
		}
	}
}

// A seat with no Partner supplies none: the group costs the page nothing and
// every other kind is composed as it was.
func TestDecisionsWithoutAPartnerCarriesNoProposals(t *testing.T) {
	t.Parallel()

	page := decisionsPage(t, New(decisionsInfo(), loopback(), testBundle()), "the read")

	for _, need := range page.NeedsYou {
		if need.Kind == decisions.KindProposal {
			t.Fatalf("a seat with no Partner invented a proposal row: %+v", need)
		}
		if need.Proposal != nil {
			t.Fatalf("a row of another kind carries an action: %+v", need)
		}
	}
	testutil.Expect(t, "the approval still waits", page.Counts.NeedsYou, 3)
}

// TestDecisionsKeepsTheQuestionsItCouldRead: a question walk that could not
// read every record still answers the ones it could, so the inbox shows this
// seat's ask rather than refusing the page; the Fleet page is where the
// records it could not read are named.
func TestDecisionsKeepsTheQuestionsItCouldRead(t *testing.T) {
	t.Parallel()
	info := decisionsInfo()
	read := info.Asks
	info.Asks = func() ([]channel.Question, error) {
		open, _ := read()
		return open, &UnreadQuestions{Records: []string{"/c/artifacts/agents/channel/questions/q-2.json: unexpected end of JSON input"}}
	}
	served := New(info, loopback(), testBundle())

	page := decisionsPage(t, served, "the read with a torn record")

	asks := 0
	for _, need := range page.NeedsYou {
		if need.Kind == decisions.KindAsk {
			asks++
		}
	}
	testutil.Expect(t, "the readable ask is still in the inbox", asks, 1)
}
