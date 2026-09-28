package httpd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner/fakeacp"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/project"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/review"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/snapshot"
)

// The review room at the boundary (g1-s65 §6): Start from a goal creates the
// review record and opens its own conversation, the Partner routes name the
// conversation they speak to, the desk's three reads answer from the reviewed
// tree, the room is kept through the sitting route, and the board and the
// project payload carry the door.

const reviewed = "metasystem/plans/reviews/review-of-landing.md"

var reviewTip = strings.Repeat("e", 40)

// deskGit is the candidate as a table: goal/landing at origin, its merge base
// with main, one changed file and one binary.
type deskGit struct{}

func (deskGit) ResolveCommit(rev string) (string, error) {
	switch rev {
	case "origin/goal/landing":
		return reviewTip, nil
	case "origin/main":
		return strings.Repeat("a", 40), nil
	}
	return "", fmt.Errorf("gittree commit %q is unreadable", rev)
}

func (deskGit) MergeBases(string, string) ([]string, error) {
	return []string{strings.Repeat("b", 40)}, nil
}
func (deskGit) TreeOf(rev string) (string, error) { return "tree:" + rev, nil }

func (deskGit) FileAt(_ string, path string) ([]byte, bool, error) {
	switch path {
	case "internal/owner.go":
		return []byte("package owner\n\ntype owner struct{}\n"), true, nil
	case "docs/shot.png":
		return []byte("\x89PNG\x00"), true, nil
	}
	return nil, false, nil
}

func (deskGit) FileCounts(string, string) ([]gittree.FileCount, error) {
	return []gittree.FileCount{{Path: "internal/owner.go", Added: 1, Deleted: 1}, {Path: "docs/shot.png", Binary: true}}, nil
}

func (deskGit) PathDiff(string, string, string) ([]byte, error) {
	return []byte("@@ -1,3 +1,3 @@\n package owner\n \n-type owner int\n+type owner struct{}\n"), nil
}

func (deskGit) CommitsCarrying(string, string) ([]string, error) { return nil, errors.New("none") }

func (deskGit) LineCommits(string, string) ([]string, error) { return nil, errors.New("no blame") }

// withALanding is the board with one goal built and waiting to land.
func withALanding() snapshot.Observation {
	observed := readObservation()
	landing := routeGoal("landing", goal.StateClaimed)
	landing.Claimed = &goal.ClaimRecord{Machine: "m1e", Lineage: "coordinator", At: "2026-09-20T09:00:00Z"}
	landing.Landing = &goal.LandingRecord{At: "2026-09-20T10:00:00Z"}
	observed.Tree.Live["landing"] = landing
	return observed
}

func reviewSource() string {
	return "# Review of landing\n\n- Kind: review\n- Id: 01R\n- Status: draft\n- Goals: landing\n" +
		"- Reviewed: " + reviewTip + " (the tip of goal/landing)\n\n## Facts\n\n## Findings\n\n" +
		"- 2026-09-28 · Wido · the press dies here [d:local-1]\n  - Anchor: internal/owner.go:3\n  - Answer: unanswered\n" +
		"- 2026-09-28 · Wido · the reconcile is fine [d:local-2]\n  - Answer: left open\n"
}

type servedReview struct {
	handler http.Handler
	service *partner.Service
	root    string
	// checkout is the checkout a shaping desk reads as it stands.
	checkout string
	created  []project.NewReview
}

func serveReview(t *testing.T, script fakeacp.Script) *servedReview {
	t.Helper()
	script.Models = []string{"fake-1"}
	return serveReviewOn(t, fakeacp.Open(script))
}

func serveReviewOn(t *testing.T, opener func(context.Context) (partner.Endpoint, error)) *servedReview {
	t.Helper()
	root := t.TempDir()
	runtime := partner.Runtime{Name: "fake", Model: "fake-1", ReadOnly: "a fake server reads nothing"}
	host := partner.NewHostOn(runtime, root, opener)
	t.Cleanup(host.Close)
	document := func(id string) (project.Document, error) {
		switch id {
		case reviewed:
			return project.Document{Kind: "document", ID: id, Title: "Review of landing", Source: reviewSource(),
				Revision: "blob:1", Record: &project.Head{Kind: "review", Goals: []string{"landing"}}}, nil
		case "plans/designs/sessions.md":
			return project.Document{Kind: "document", ID: id, Title: "Sessions", Source: "# Sessions\n",
				Record: &project.Head{Kind: "design"}}, nil
		case "plans/intent/sessions.md":
			return project.Document{Kind: "document", ID: id, Title: "Sessions", Source: "# Sessions\n",
				Record: &project.Head{Kind: "intent"}}, nil
		case "plans/doctrine/sessions.md":
			return project.Document{Kind: "document", ID: id, Title: "Sessions", Source: "# Sessions\n",
				Record: &project.Head{Kind: "doctrine"}}, nil
		}
		return project.Document{}, project.ErrNotFound
	}
	service := partner.NewService(runtime, host,
		func(human string) (*partner.Conversation, error) { return partner.OpenConversation(root, human) },
		partner.Facts{Document: document}, func() time.Time { return time.Date(2026, 9, 28, 9, 0, 0, 0, time.UTC) })
	served := &servedReview{service: service, root: root, checkout: t.TempDir()}
	info := Info{
		Observe: withALanding, Authority: proven(), Partner: service, PartnerConfigured: true,
		Document: document,
		Review:   &review.Owner{Git: deskGit{}, Checkout: served.checkout},
		CreateReview: func(asked project.NewReview) (project.Written, error) {
			served.created = append(served.created, asked)
			return project.Written{Path: reviewed, Record: project.Record{Kind: "review", Title: "Review of landing",
				Goals: []string{"landing"}, Path: reviewed}}, nil
		},
		Project: func() (project.Pane, error) {
			return project.Pane{
				Records: []project.Record{{Kind: "review", ID: "01R", Title: "Review of landing", Path: reviewed,
					Goals: []string{"landing"}}},
				Sittings: []project.Sitting{{Record: project.SatOn{Kind: "review", ID: "01R", Path: reviewed,
					Title: "Review of landing"}, Counts: project.PileCounts{Findings: 2, Unanswered: 1}}},
			}, nil
		},
	}
	served.handler = New(info, loopback(), testBundle())
	return served
}

const reviewLanding = `{"purpose":"review","subject":{"kind":"goal","id":"landing"},` +
	`"about":{"section":"Backlog","path":"/backlog"}}`

func TestAReviewStartsFromAGoalAndOpensItsOwnConversation(t *testing.T) {
	t.Parallel()
	served := serveReview(t, fakeacp.Script{Chunks: []string{"Asked: ..."}})
	events, stop := served.service.Subscribe()
	defer stop()

	started := post(t, served.handler, partnerSittingPath, reviewLanding, nil)
	testutil.Require(t, "the review opened", started.Code, http.StatusOK)
	drain(t, events)
	answer := partnerSnapshot(t, started)
	testutil.Expect(t, "the answer is the review's own conversation", answer.Conversation, reviewed)
	testutil.Require(t, "with the sitting on it", answer.Sitting != nil, true)
	testutil.Expect(t, "a review", answer.Sitting.Purpose, partner.PurposeReview)
	testutil.Expect(t, "on the record the server created", answer.Sitting.Subject.ID, reviewed)
	testutil.Require(t, "the record was created once", len(served.created), 1)
	testutil.Expect(t, "for the goal, at its branch tip", served.created[0],
		project.NewReview{Goal: "landing", Reviewed: reviewTip + " (the tip of goal/landing)"})

	ordinary := partnerSnapshot(t, get(t, served.handler, partnerPath, nil))
	testutil.Expect(t, "the drawer's conversation carries no sitting", ordinary.Sitting == nil, true)
	testutil.Expect(t, "and none of the review's words", len(ordinary.Messages), 0)
	room := partnerSnapshot(t, get(t, served.handler, partnerPath+"?conversation="+reviewed, nil))
	testutil.Expect(t, "the room reads its own", room.Conversation, reviewed)
	testutil.Require(t, "with the opening turn in it", len(room.Messages), 2)

	// Review it on a goal whose review stands opens that review again.
	again := post(t, served.handler, partnerSittingPath, reviewLanding, nil)
	testutil.Require(t, "the door opens", again.Code, http.StatusOK)
	testutil.Expect(t, "onto the same conversation", partnerSnapshot(t, again).Conversation, reviewed)
	testutil.Expect(t, "and no second record", len(served.created), 1)
}

func TestTheDesksReadsAnswerFromTheReviewedTree(t *testing.T) {
	t.Parallel()
	served := serveReview(t, fakeacp.Script{})
	base := reviewPrefix + reviewed

	source := get(t, served.handler, base+"/source?path=internal/owner.go&from=2&to=3", nil)
	testutil.Require(t, "the source read", source.Code, http.StatusOK)
	var read review.Source
	testutil.Require(t, "the source decodes", json.Unmarshal(source.Body.Bytes(), &read), nil)
	testutil.Expect(t, "at the tip", read.Commit, reviewTip)
	testutil.Expect(t, "the lines, the touched one marked", read.Lines,
		[]review.SourceLine{{Number: 2, Text: ""}, {Number: 3, Text: "type owner struct{}", Touched: true}})

	index := get(t, served.handler, base+"/changes", nil)
	testutil.Require(t, "the change index", index.Code, http.StatusOK)
	var changes review.Index
	testutil.Require(t, "the index decodes", json.Unmarshal(index.Body.Bytes(), &changes), nil)
	testutil.Expect(t, "two files", len(changes.Files), 2)
	testutil.Expect(t, "the branch now", changes.Current, reviewTip)

	diff := get(t, served.handler, base+"/changes?path=internal/owner.go", nil)
	testutil.Require(t, "one file's diff", diff.Code, http.StatusOK)
	var hunks review.Diff
	testutil.Require(t, "the diff decodes", json.Unmarshal(diff.Body.Bytes(), &hunks), nil)
	testutil.Expect(t, "its hunk", hunks.Parts[0].Hunks[0].Header, "@@ -1,3 +1,3 @@")

	for _, refused := range []struct {
		what, path string
		status     int
		says       string
	}{
		{"a path outside the tree", "/source?path=../secrets", http.StatusBadRequest, `"../secrets" is not a path inside the reviewed tree`},
		{"a range past the file", "/source?path=internal/owner.go&from=9", http.StatusBadRequest, "internal/owner.go has 3 lines; line 9 is past its end"},
		{"a binary file", "/source?path=docs/shot.png", http.StatusBadRequest, "docs/shot.png is a binary file, and the desk shows text"},
		{"a range that is not a number", "/source?path=internal/owner.go&from=two", http.StatusBadRequest, `from is a line number, and "two" is not one`},
		{"nothing moved", "/changes?since=1", http.StatusBadRequest, "goal/landing has not moved since it was reviewed"},
	} {
		answered := get(t, served.handler, base+refused.path, nil)
		testutil.Expect(t, refused.what+": status", answered.Code, refused.status)
		testutil.Expect(t, refused.what+": words", errorOf(t, answered), refused.says)
	}
	// A design is a sitting's record now, with no change to index (g1-s67 §6).
	notAReview := get(t, served.handler, reviewPrefix+"plans/designs/sessions.md/changes", nil)
	testutil.Expect(t, "a record that is not a review", notAReview.Code, http.StatusBadRequest)
	testutil.Expect(t, "says so", errorOf(t, notAReview),
		"a sitting on a design has no change to index; its desk reads the checkout as it stands")
	missing := get(t, served.handler, reviewPrefix+"plans/reviews/gone.md/changes", nil)
	testutil.Expect(t, "no such record", missing.Code, http.StatusNotFound)
	unknown := get(t, served.handler, reviewPrefix+reviewed+"/elsewhere", nil)
	testutil.Expect(t, "no such read", unknown.Code, http.StatusNotFound)
}

func errorOf(t *testing.T, response *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("a refusal must decode: %v\n%s", err, response.Body.Bytes())
	}
	return body.Error
}

func TestAWalkAndATurnSpeakToTheRoomsConversation(t *testing.T) {
	t.Parallel()
	served := serveReview(t, fakeacp.Script{Chunks: []string{"Built: ..."}})
	events, stop := served.service.Subscribe()
	defer stop()
	testutil.Require(t, "the review opened", post(t, served.handler, partnerSittingPath, reviewLanding, nil).Code, http.StatusOK)
	drain(t, events)

	walked := post(t, served.handler, partnerWalkPath,
		`{"conversation":"`+reviewed+`","part":"built","about":{"section":"Review"}}`, nil)
	testutil.Require(t, "the walk was asked", walked.Code, http.StatusOK)
	drain(t, events)
	asked := post(t, served.handler, partnerTurnsPath,
		`{"conversation":"`+reviewed+`","key":"k1","text":"what if the press dies here?","about":{"section":"Review"}}`, nil)
	testutil.Require(t, "the question was accepted", asked.Code, http.StatusAccepted)
	drain(t, events)

	room := partnerSnapshot(t, get(t, served.handler, partnerPath+"?conversation="+reviewed, nil))
	testutil.Require(t, "opening, walk and question with their answers", len(room.Messages), 6)
	testutil.Expect(t, "the walk is the interface's", room.Messages[2].Interface, true)
	testutil.Expect(t, "the question is the human's", room.Messages[4].Interface, false)
	testutil.Expect(t, "in the room", room.Messages[4].Text, "what if the press dies here?")
	ordinary := partnerSnapshot(t, get(t, served.handler, partnerPath, nil))
	testutil.Expect(t, "the drawer's conversation is untouched", len(ordinary.Messages), 0)

	refused := post(t, served.handler, partnerWalkPath, `{"conversation":"`+reviewed+`","part":"gossip"}`, nil)
	testutil.Expect(t, "a walk the room has not", refused.Code, http.StatusBadRequest)
}

func TestTheRoomIsKeptThroughTheSittingRoute(t *testing.T) {
	t.Parallel()
	served := serveReview(t, fakeacp.Script{Chunks: []string{"Asked: ..."}})
	events, stop := served.service.Subscribe()
	defer stop()
	testutil.Require(t, "the review opened", post(t, served.handler, partnerSittingPath, reviewLanding, nil).Code, http.StatusOK)
	drain(t, events)

	kept := post(t, served.handler, partnerSittingPath, `{"conversation":"`+reviewed+`",`+
		`"room":{"desk":{"items":[{"kind":"changes"}],"current":0},"face":"desk","drafts":{"local-3":{"text":"half"}}}}`, nil)
	testutil.Require(t, "kept", kept.Code, http.StatusOK)
	room := partnerSnapshot(t, get(t, served.handler, partnerPath+"?conversation="+reviewed, nil))
	testutil.Require(t, "the room came back", room.Sitting != nil && room.Sitting.Room != nil, true)
	testutil.Expect(t, "the drafts", string(room.Sitting.Room.Drafts), `{"local-3":{"text":"half"}}`)
	testutil.Expect(t, "the time it was kept", room.Sitting.Room.At, "2026-09-28T09:00:00Z")

	nowhere := post(t, served.handler, partnerSittingPath, `{"conversation":"plans/designs/sessions.md","room":{"face":"desk"}}`, nil)
	testutil.Expect(t, "no room where nothing stands", nowhere.Code, http.StatusBadRequest)
}

// The Review lane's door line reads the record (Astra S65-01): the findings and
// the unanswered ones are counts of the record, and the time is the room's.
func TestTheBoardCarriesEachReviewsDoor(t *testing.T) {
	t.Parallel()
	served := serveReview(t, fakeacp.Script{Chunks: []string{"Asked: ..."}})
	events, stop := served.service.Subscribe()
	defer stop()
	testutil.Require(t, "the review opened", post(t, served.handler, partnerSittingPath, reviewLanding, nil).Code, http.StatusOK)
	drain(t, events)
	testutil.Require(t, "kept", post(t, served.handler, partnerSittingPath,
		`{"conversation":"`+reviewed+`","room":{"face":"desk"}}`, nil).Code, http.StatusOK)

	var board struct {
		Reviews []reviewDoor `json:"reviews"`
	}
	testutil.Require(t, "the board decodes", json.Unmarshal(get(t, served.handler, backlogPath, nil).Body.Bytes(), &board), nil)
	testutil.Expect(t, "one door, from the record and the room", board.Reviews, []reviewDoor{{
		Goal: "landing", Record: reviewed, Title: "Review of landing", Findings: 2, Unanswered: 1,
		Standing: true, SteppedOutAt: "2026-09-28T09:00:00Z",
	}})

	var pane project.Pane
	testutil.Require(t, "the project decodes", json.Unmarshal(get(t, served.handler, projectPath, nil).Body.Bytes(), &pane), nil)
	testutil.Require(t, "one sitting row", len(pane.Sittings), 1)
	testutil.Expect(t, "standing, with its door time", []any{pane.Sittings[0].Standing, pane.Sittings[0].SteppedOutAt},
		[]any{true, "2026-09-28T09:00:00Z"})
}

func TestAnUnsettledSwitchIsAConflictInWords(t *testing.T) {
	t.Parallel()
	answered := httptest.NewRecorder()
	(&handler{}).refuseTurn(answered, partner.ErrUnsettled, "")
	testutil.Expect(t, "a conflict", answered.Code, http.StatusConflict)
	testutil.Expect(t, "in the service's words", errorOf(t, answered), partner.ErrUnsettled.Error())
	testutil.Expect(t, "coded", strings.Contains(answered.Body.String(), `"code":"unsettled"`), true)
}

// What the review routes refuse, and in whose words: a build with no review
// reader, a record the project would not write, a record that names nothing
// yet, a walk with no Partner, and a done goal reviewed through its landed
// commits.
func TestTheReviewRoutesRefuseInTheirOwnWords(t *testing.T) {
	t.Parallel()
	bare := New(Info{Observe: withALanding, Authority: proven()}, loopback(), testBundle())
	unread := get(t, bare, reviewPrefix+reviewed+"/changes", nil)
	testutil.Expect(t, "no review reader", unread.Code, http.StatusInternalServerError)
	testutil.Expect(t, "said", errorOf(t, unread), "this engine was built without a review reader")
	walked := post(t, bare, partnerWalkPath, `{"conversation":"`+reviewed+`","part":"built"}`, nil)
	testutil.Expect(t, "no Partner to walk with", walked.Code, http.StatusServiceUnavailable)

	served := serveReview(t, fakeacp.Script{Chunks: []string{"Asked: ..."}})
	withoutWriter := New(Info{Observe: withALanding, Authority: proven(), Partner: served.service,
		PartnerConfigured: true}, loopback(), testBundle())
	unwritable := post(t, withoutWriter, partnerSittingPath, reviewLanding, nil)
	testutil.Expect(t, "no review writer", unwritable.Code, http.StatusInternalServerError)

	refusing := New(Info{Observe: withALanding, Authority: proven(), Partner: served.service, PartnerConfigured: true,
		Review: &review.Owner{Git: deskGit{}},
		CreateReview: func(project.NewReview) (project.Written, error) {
			return project.Written{}, &project.Refusal{Kind: project.RefusalBad, Message: `the goal "landing" is not in the ledger`}
		}}, loopback(), testBundle())
	refused := post(t, refusing, partnerSittingPath, reviewLanding, nil)
	testutil.Expect(t, "the project's own refusal", refused.Code, http.StatusBadRequest)
	testutil.Expect(t, "in its words", strings.Contains(refused.Body.String(), `the goal \"landing\" is not in the ledger`), true)

	named := []project.NewReview{}
	done := New(Info{Observe: withALanding, Authority: proven(), Partner: served.service, PartnerConfigured: true,
		Review: &review.Owner{Git: deskGit{}},
		CreateReview: func(asked project.NewReview) (project.Written, error) {
			named = append(named, asked)
			return project.Written{}, errors.New("the disk is full")
		}}, loopback(), testBundle())
	failed := post(t, done, partnerSittingPath, `{"purpose":"review","subject":{"kind":"goal","id":"finished"}}`, nil)
	testutil.Expect(t, "a writer that fails", failed.Code, http.StatusInternalServerError)
	testutil.Require(t, "was asked once", len(named), 1)
	testutil.Expect(t, "a done goal with no landed commits names none", named[0].Reviewed, review.NoneFound)

	blank := New(Info{Observe: withALanding, Authority: proven(), Review: &review.Owner{Git: deskGit{}},
		Document: func(id string) (project.Document, error) {
			if id == "broken.md" {
				return project.Document{}, errors.New("the disk is unreadable")
			}
			return project.Document{ID: id, Source: "- Reviewed: " + review.NoneFound + "\n",
				Record: &project.Head{Kind: "review"}}, nil
		}}, loopback(), testBundle())
	nothing := get(t, blank, reviewPrefix+"blank.md/changes", nil)
	testutil.Expect(t, "a review naming no commits", nothing.Code, http.StatusBadRequest)
	testutil.Expect(t, "says so", errorOf(t, nothing), "this review names no commits yet; write them on its Reviewed line")
	broken := get(t, blank, reviewPrefix+"broken.md/source?path=a.go", nil)
	testutil.Expect(t, "a document the reader could not read", broken.Code, http.StatusInternalServerError)
}

// The stop route answers the conversation it was asked from.
func TestTheStopRouteAnswersTheRoomsConversation(t *testing.T) {
	t.Parallel()
	served := serveReview(t, fakeacp.Script{Chunks: []string{"Asked: ..."}})
	events, stop := served.service.Subscribe()
	defer stop()
	testutil.Require(t, "the review opened", post(t, served.handler, partnerSittingPath, reviewLanding, nil).Code, http.StatusOK)
	drain(t, events)
	stopped := post(t, served.handler, partnerTurnsPre+"no-such-turn"+stopSuffix, `{"conversation":"`+reviewed+`"}`, nil)
	testutil.Require(t, "a stop for a turn that ended is no error", stopped.Code, http.StatusOK)
	testutil.Expect(t, "it answers the room", partnerSnapshot(t, stopped).Conversation, reviewed)
}

// Sol SOL-A-05: a Start whose sitting failed to open left its record, and the
// next press is about that record rather than a second one.
func TestAStartThatFailedToOpenIsRetriedOnItsOwnRecord(t *testing.T) {
	t.Parallel()
	// The drawer's conversation holds the first session; the runtime fails the
	// one open of the review's own session, then answers again.
	opens := 0
	opener := fakeacp.Open(fakeacp.Script{Chunks: []string{"Asked: ..."}, Models: []string{"fake-1"}})
	served := serveReviewOn(t, func(ctx context.Context) (partner.Endpoint, error) {
		opens++
		if opens == 2 {
			return partner.Endpoint{}, errors.New("the runtime could not start")
		}
		return opener(ctx)
	})
	events, stop := served.service.Subscribe()
	defer stop()
	asked := post(t, served.handler, partnerTurnsPath, `{"key":"k1","text":"a question","about":{}}`, nil)
	testutil.Require(t, "the drawer's question", asked.Code, http.StatusAccepted)
	drain(t, events)

	refused := post(t, served.handler, partnerSittingPath, reviewLanding, nil)
	testutil.Require(t, "the first open is refused", refused.Code != http.StatusOK, true)
	var body struct {
		Draft string `json:"draft"`
	}
	testutil.Require(t, "the refusal", json.Unmarshal(refused.Body.Bytes(), &body), nil)
	testutil.Expect(t, "it names the record it made", body.Draft, reviewed)

	again := post(t, served.handler, partnerSittingPath, reviewLanding, nil)
	testutil.Require(t, "the second press opens the sitting", again.Code, http.StatusOK)
	drain(t, events)
	testutil.Expect(t, "one record", len(served.created), 1)
	testutil.Expect(t, "on that record", partnerSnapshot(t, again).Conversation, reviewed)
}

// The verdict is a fact of the Outcome card, not of the page (Sol SOL-A-02,
// SOL-A-07): the closing turn's deposit carries the verdict the human chose on
// the stream, in the answer the read route gives, and in the transcript on disk
// — so a card rebuilt after a reload records under the same shape.
func TestTheClosingDepositCarriesItsVerdict(t *testing.T) {
	t.Parallel()
	served := serveReview(t, fakeacp.Script{
		Reads: []fakeacp.Read{{Name: "mcp__metasystem__deposit", Title: "deposit(outcome)",
			Result: "prepared\nDeposit: outcome\n--- the deposit follows, whole and to the end ---\n" +
				"Verdict: clear to land\nExamined: the owner and its test\n"}},
		Chunks: []string{"Here is what it came to."},
	})
	events, stop := served.service.Subscribe()
	defer stop()
	testutil.Require(t, "the review opened", post(t, served.handler, partnerSittingPath, reviewLanding, nil).Code, http.StatusOK)
	drain(t, events)

	closed := post(t, served.handler, partnerSittingClosePath,
		`{"conversation":"`+reviewed+`","verdict":"clear to land","about":{"section":"Review"}}`, nil)
	testutil.Require(t, "the close was asked", closed.Code, http.StatusOK)
	streamed := ""
	for beat := range events {
		if beat.Deposit != nil && beat.Deposit.Kind == "outcome" {
			streamed = beat.Deposit.Verdict
		}
		if beat.Kind == partner.EventDone || beat.Kind == partner.EventStopped || beat.Kind == partner.EventError {
			break
		}
	}
	testutil.Expect(t, "the stream carries the verdict", streamed, partner.VerdictClear)

	outcomeOf := func(messages []partner.Message) *partner.Deposit {
		for at := len(messages) - 1; at >= 0; at-- {
			for index := range messages[at].Deposits {
				if messages[at].Deposits[index].Kind == "outcome" {
					return &messages[at].Deposits[index]
				}
			}
		}
		return nil
	}
	room := partnerSnapshot(t, get(t, served.handler, partnerPath+"?conversation="+reviewed, nil))
	read := outcomeOf(room.Messages)
	testutil.Require(t, "the read route carries the outcome", read != nil, true)
	testutil.Expect(t, "with its verdict", read.Verdict, partner.VerdictClear)

	kept, err := partner.OpenConversation(served.root, "Wido", reviewed)
	testutil.Require(t, "the transcript reopened from disk", err, nil)
	persisted := outcomeOf(kept.Messages(100))
	testutil.Require(t, "the transcript keeps the outcome", persisted != nil, true)
	testutil.Expect(t, "and its verdict", persisted.Verdict, partner.VerdictClear)
}
