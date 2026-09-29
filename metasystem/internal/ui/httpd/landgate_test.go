package httpd

// The landing gate at the boundary (g1-s70 §6, §8): the board's Review rows
// carry the gate's reading, the workspace carries the two settings with their
// sources, the inbox carries a landing row, land without a sitting is the
// signed-in human's with its reason, and a review sitting on a waiting goal
// holds it before it opens and releases it when it ends.

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/decisions"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner/fakeacp"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/project"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/review"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/session"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/snapshot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/workspace"
)

func gateSettingsFixture() (config.LandingGate, error) {
	return config.LandingGate{HumanFromTier: 2, AutoAfter: 4 * time.Hour,
		Tier:  config.LandingGateFact{Key: config.LandingHumanFromTierKey, Value: "1", Source: "conf-local"},
		After: config.LandingGateFact{Key: config.LandingAutoAfterKey, Value: "4h", Source: "default"}}, nil
}

// gateLedger is the board with one goal waiting to land, whose history the
// fake acts below write the hold and its release onto.
type gateLedger struct {
	mu      sync.Mutex
	lines   []goal.HistoryLine
	holds   []string
	decided []string
}

func (l *gateLedger) observe() snapshot.Observation {
	observed := withALanding()
	l.mu.Lock()
	defer l.mu.Unlock()
	landing := observed.Tree.Live["landing"]
	landing.History = append(landing.History, l.lines...)
	return observed
}

func (l *gateLedger) sitting(_ *session.Session, id, record string, open bool) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.holds = append(l.holds, map[bool]string{true: "hold ", false: "release "}[open]+id+" "+record)
	// The ledger names the record from the installation's root, as the act
	// resolves it; the room names it from the checkout's.
	ledgerPath := record[strings.Index(record, "plans/reviews/"):]
	l.lines = append(l.lines, goal.HistoryLine{At: "2026-09-28T09:00:00Z", Verb: "review", Actor: "human:Wido",
		Reason: goal.SittingReason(open, ledgerPath, "Wido")})
	return nil
}

func (l *gateLedger) landWithoutSitting(_ *session.Session, id, tip, reason string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.decided = append(l.decided, id+" "+tip+" "+reason)
	return nil
}

func (l *gateLedger) wire(info *Info) {
	info.Observe = l.observe
	info.LandingGate = gateSettingsFixture
	info.Sitting = l.sitting
	info.LandWithoutSitting = l.landWithoutSitting
}

func TestTheBoardsReviewRowCarriesTheGatesReading(t *testing.T) {
	t.Parallel()
	ledger := &gateLedger{}
	info := Info{}
	ledger.wire(&info)
	served := New(info, loopback(), testBundle())
	var board backlogPayload
	testutil.Require(t, "decode", json.Unmarshal(get(t, served, backlogPath, nil).Body.Bytes(), &board), nil)
	for _, row := range board.Rows {
		if row.ID != "landing" {
			testutil.Expect(t, row.ID+" carries no gate", row.Gate == nil, true)
			continue
		}
		if row.Gate == nil || !row.Gate.WaitsForHuman || row.Gate.Tier != 3 || row.Gate.HumanFromTier != 2 {
			t.Fatalf("the Review row's gate = %+v", row.Gate)
		}
	}
}

func TestTheWorkspaceCarriesTheTwoSettingsWithTheirSources(t *testing.T) {
	t.Parallel()
	served := New(Info{Describe: func() (workspace.Workspace, error) { return workspace.Workspace{}, nil }, LandingGate: gateSettingsFixture},
		loopback(), testBundle())
	var described workspace.Workspace
	testutil.Require(t, "decode", json.Unmarshal(get(t, served, workspacePath, nil).Body.Bytes(), &described), nil)
	testutil.Require(t, "the landing gate", described.LandingGate != nil, true)
	testutil.Expect(t, "the facts", described.LandingGate.Facts, []config.LandingGateFact{
		{Key: "landing.review.human-from-tier", Value: "1", Source: "conf-local"},
		{Key: "landing.review.auto-after", Value: "4h", Source: "default"},
	})
}

func TestTheInboxCarriesTheGoalThatWaitsForYourReview(t *testing.T) {
	t.Parallel()
	ledger := &gateLedger{}
	info := Info{Project: func() (project.Pane, error) { return project.Pane{}, nil }}
	ledger.wire(&info)
	served := New(info, loopback(), testBundle())
	var page decisions.Page
	testutil.Require(t, "decode", json.Unmarshal(get(t, served, decisionsPath, nil).Body.Bytes(), &page), nil)
	found := false
	for _, need := range page.NeedsYou {
		if need.Kind == decisions.KindLanding && need.ID == "landing" && need.Act == decisions.ActLandWithoutSitting {
			found = true
		}
	}
	testutil.Expect(t, "the landing row", found, true)
}

func TestLandWithoutASittingIsTheSignedInHumansWithItsReason(t *testing.T) {
	t.Parallel()
	ledger := &gateLedger{}
	signing := newSigning(t, "Wido", workingSecret)
	info := Info{Authority: proven(), Sessions: signing.store, Review: &review.Owner{Git: deskGit{}}}
	ledger.wire(&info)
	served := New(info, loopback(), testBundle())
	path := goalsPrefix + "landing" + landWithoutSittingSuffix

	unsigned := post(t, served, path, `{"reason":"read the diff"}`, nil)
	testutil.Require(t, "unsigned", unsigned.Code, http.StatusForbidden)
	testutil.Expect(t, "says the remedy", actRefusal(t, unsigned).SignIn, true)

	signedIn := post(t, served, signInPath, `{"code":"`+routeCode(t, routeNow)+`","human":""}`, nil)
	testutil.Require(t, "signed in", signedIn.Code, http.StatusOK)
	cookie := carrying(mintedCookie(t, signedIn).Value)
	empty := post(t, served, path, `{"reason":"  "}`, cookie)
	testutil.Require(t, "no reason", empty.Code, http.StatusBadRequest)
	testutil.Expect(t, "the code", actRefusal(t, empty).Code, "reason")
	testutil.Expect(t, "nothing decided", len(ledger.decided), 0)

	decided := post(t, served, path, `{"reason":"one-line doc fix, read the diff on the card"}`, cookie)
	testutil.Require(t, "decided", decided.Code, http.StatusOK)
	testutil.Expect(t, "at the branch's tip, with the reason", ledger.decided, []string{"landing " + reviewTip + " one-line doc fix, read the diff on the card"})
	var board backlogPayload
	testutil.Require(t, "the board rides along", json.Unmarshal(decided.Body.Bytes(), &board), nil)
}

func TestAReviewSittingOnAWaitingGoalHoldsItAndItsEndReleasesIt(t *testing.T) {
	t.Parallel()
	ledger := &gateLedger{}
	signing := newSigning(t, "Wido", workingSecret)
	served := serveReviewOn(t, fakeacp.Open(fakeacp.Script{Chunks: []string{"Asked: ..."}, Models: []string{"fake-1"}}), func(info *Info) {
		ledger.wire(info)
		info.Sessions = signing.store
	})
	unsigned := post(t, served.handler, partnerSittingPath, reviewLanding, nil)
	testutil.Require(t, "a sitting that would hold asks for the sign-in", unsigned.Code, http.StatusForbidden)
	testutil.Expect(t, "nothing created", len(served.created), 0)
	testutil.Expect(t, "nothing held", len(ledger.holds), 0)

	signedIn := post(t, served.handler, signInPath, `{"code":"`+routeCode(t, routeNow)+`","human":""}`, nil)
	testutil.Require(t, "signed in", signedIn.Code, http.StatusOK)
	cookie := carrying(mintedCookie(t, signedIn).Value)
	started := post(t, served.handler, partnerSittingPath, reviewLanding, cookie)
	testutil.Require(t, "the review opened", started.Code, http.StatusOK)
	testutil.Expect(t, "held before it opened", ledger.holds, []string{"hold landing " + reviewed})

	ended := post(t, served.handler, partnerSittingEndPath, `{"conversation":"`+reviewed+`"}`, cookie)
	testutil.Require(t, "the sitting ended", ended.Code, http.StatusOK)
	testutil.Expect(t, "and released its hold", ledger.holds, []string{"hold landing " + reviewed, "release landing " + reviewed})

	// A sitting whose hold a verdict already released ends with nothing more.
	again := post(t, served.handler, partnerSittingEndPath, `{"conversation":"`+reviewed+`"}`, cookie)
	testutil.Expect(t, "a second end", again.Code == http.StatusOK || again.Code == http.StatusInternalServerError, true)
	testutil.Expect(t, "releases nothing again", len(ledger.holds), 2)
}

// The server reads the newest word against the goal branch's tip at origin
// (SOL-S70-04): a word given before the branch moved says so on the board's
// row, and the inbox asks for the word again, for a verdict and a decision to
// land without a sitting alike.
func TestAMovedTipIsReadOnTheBoardAndAsksForTheWordAgain(t *testing.T) {
	t.Parallel()
	const given = "9c1f0a2b3c4d5e6f708192a3b4c5d6e7f8091a2b"
	for _, reason := range []string{
		"reviewed verdict=clear-to-land tip=" + given + " record=plans/reviews/review-of-landing.md by=Wido",
		"landed-without-sitting tip=" + given + " by=Wido because=one-line doc fix",
	} {
		verb := "review"
		if strings.HasPrefix(reason, "landed-without-sitting") {
			verb = goal.LandWithoutSittingVerb
		}
		ledger := &gateLedger{lines: []goal.HistoryLine{{At: "2026-09-28T08:00:00Z", Opid: "01J5X0000000000000000000W1-mac-ui-1a2b3c4d", Verb: verb, Actor: "human:Wido", Reason: reason}}}
		info := Info{Project: func() (project.Pane, error) { return project.Pane{}, nil }, Review: &review.Owner{Git: deskGit{}}}
		ledger.wire(&info)
		served := New(info, loopback(), testBundle())

		var board backlogPayload
		testutil.Require(t, verb+": the board", json.Unmarshal(get(t, served, backlogPath, nil).Body.Bytes(), &board), nil)
		var word *struct{ Moved bool }
		for _, row := range board.Rows {
			if row.ID == "landing" && row.Gate != nil && row.Gate.Reviewed != nil {
				testutil.Expect(t, verb+": where the branch is now", row.Gate.Reviewed.BranchTip, reviewTip)
				word = &struct{ Moved bool }{row.Gate.Reviewed.Moved}
			}
		}
		if word == nil || !word.Moved {
			t.Fatalf("%s: the board does not say the branch moved past the word: %+v", verb, board.Rows)
		}

		var page decisions.Page
		testutil.Require(t, verb+": the inbox", json.Unmarshal(get(t, served, decisionsPath, nil).Body.Bytes(), &page), nil)
		found := false
		for _, need := range page.NeedsYou {
			if need.Kind == decisions.KindLanding && need.ID == "landing" && need.Act == decisions.ActLandWithoutSitting {
				found = true
			}
		}
		testutil.Expect(t, verb+": the inbox asks for the word again", found, true)
	}
}
