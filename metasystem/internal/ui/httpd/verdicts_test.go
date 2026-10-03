package httpd

// The verdict route and the candidate's three routes, at the boundary (g1-s69
// §6, §8): the launch route's session rule for every write, the act reached
// with what the room sent, a refusal in the engine's words, a goal without a
// launch contract refused in words, and status carrying the running commit.

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/act"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/review"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/session"
)

type verdicting struct {
	asked     []act.Reviewed
	refusal   error
	runs      []string
	run       Candidate
	runRefuse error
	sessions  *session.Store
}

func (rec *verdicting) serving() Info {
	return Info{
		Observe:  readObservation,
		Sessions: rec.sessions,
		Verdict: func(_ *session.Session, id string, asked act.Reviewed) (act.Recorded, error) {
			rec.asked = append(rec.asked, asked)
			if rec.refusal != nil {
				return act.Recorded{}, rec.refusal
			}
			return act.Recorded{Verdict: asked.Verdict, Tip: "9c1f0a2b3c4d5e6f708192a3b4c5d6e7f8091a2b", Record: asked.Record, By: "Wido",
				Line: "reviewed verdict=" + asked.Verdict}, nil
		},
		Candidate: func(goal, at, action string) (Candidate, error) {
			rec.runs = append(rec.runs, strings.Join(strings.Fields(goal+" "+at+" "+action), " "))
			if rec.runRefuse != nil {
				return Candidate{}, rec.runRefuse
			}
			return rec.run, nil
		},
	}
}

const clearBody = `{"record":"metasystem/plans/reviews/review-of-g.md","verdict":"clear-to-land","brief":"","work":""}`

func signedInto(t *testing.T, rec *verdicting) (http.Handler, map[string]string) {
	t.Helper()
	signing := newSigning(t, "Wido", workingSecret)
	rec.sessions = signing.store
	served := New(rec.serving(), loopback(), testBundle())
	signedIn := post(t, served, signInPath, `{"code":"`+routeCode(t, routeNow)+`","human":""}`, nil)
	testutil.Require(t, "signed in", signedIn.Code, http.StatusOK)
	return served, carrying(mintedCookie(t, signedIn).Value)
}

// The boot proof never answers for a verdict or a candidate's run: only a
// signed-in session does, as for a launch.
func TestTheVerdictAndTheCandidateRefuseEveryHandButASignedInOne(t *testing.T) {
	t.Parallel()
	rec := &verdicting{}
	info := rec.serving()
	info.Authority = proven()
	served := New(info, loopback(), testBundle())
	for _, path := range []string{goalsPrefix + "g" + reviewSuffix, appPrefix + "g" + appStartPart, appPrefix + "g" + appStopPart} {
		response := post(t, served, path, clearBody, nil)
		testutil.Require(t, path+" status", response.Code, http.StatusForbidden)
		testutil.Expect(t, path+" says the remedy is here", actRefusal(t, response).SignIn, true)
	}
	testutil.Expect(t, "no verdict was recorded", len(rec.asked), 0)
	testutil.Expect(t, "nothing was run", len(rec.runs), 0)
}

func TestASignedInHumanRecordsAVerdictAndIsAnsweredTheLine(t *testing.T) {
	t.Parallel()
	rec := &verdicting{}
	served, cookie := signedInto(t, rec)
	response := post(t, served, goalsPrefix+"g"+reviewSuffix,
		`{"record":"metasystem/plans/reviews/review-of-g.md","verdict":"send-back","brief":"# Brief\n\n1. Fix it.\n","work":"writer",`+
			`"tip":"9c1f0a2b3c4d5e6f708192a3b4c5d6e7f8091a2b","revision":"blob:7f3e"}`, cookie)
	testutil.Require(t, "status", response.Code, http.StatusOK)
	// RF-02: the version and the saved record the person decided on travel
	// with the verdict, for the act to compare with what it publishes.
	testutil.Expect(t, "what the act was asked", rec.asked, []act.Reviewed{{
		Record: "metasystem/plans/reviews/review-of-g.md", Verdict: "send-back", Brief: "# Brief\n\n1. Fix it.\n", Work: "writer",
		Tip: "9c1f0a2b3c4d5e6f708192a3b4c5d6e7f8091a2b", Revision: "blob:7f3e",
	}})
	var answered struct {
		Recorded act.Recorded   `json:"recorded"`
		Backlog  backlogPayload `json:"backlog"`
	}
	testutil.Require(t, "decode", json.Unmarshal(response.Body.Bytes(), &answered), nil)
	testutil.Expect(t, "the line", answered.Recorded.Line, "reviewed verdict=send-back")
	testutil.Expect(t, "the backlog rides along", answered.Backlog.Rows != nil, true)
}

func TestAVerdictRefusedByTheEngineSaysItsWords(t *testing.T) {
	t.Parallel()
	rec := &verdicting{refusal: &act.Refusal{Kind: act.KindRequest, Code: "record",
		Message: "the Outcome of plans/reviews/review-of-g.md was drafted for 4d2e7b1c3d4e, and the record now reviews 9c1f0a2b3c4d: the branch was retipped since; press End again"}}
	served, cookie := signedInto(t, rec)
	response := post(t, served, goalsPrefix+"g"+reviewSuffix, clearBody, cookie)
	testutil.Require(t, "status", response.Code, http.StatusBadRequest)
	if refusal := actRefusal(t, response); !strings.Contains(refusal.Error, "press End again") {
		t.Fatalf("the refusal lost the engine's words: %+v", refusal)
	}
}

// Status is a read and carries the running commit; start and stop are the
// signed-in human's; a goal without a launch contract is refused in words.
func TestTheCandidateRoutesWrapTheThreeAppForms(t *testing.T) {
	t.Parallel()
	rec := &verdicting{run: Candidate{Goal: "g", State: "running", Readiness: "answering", Address: "127.0.0.1:7981",
		Commit: "4d2e7b1c3d4e5f60718293a4b5c6d7e8f9a0b1c2", Since: "2026-09-29T09:00:00Z", Said: "the application is running and answering"}}
	served, cookie := signedInto(t, rec)

	status := get(t, served, appPrefix+"g"+appStatusPart, nil)
	testutil.Require(t, "status", status.Code, http.StatusOK)
	var run Candidate
	testutil.Require(t, "decode", json.Unmarshal(status.Body.Bytes(), &run), nil)
	testutil.Expect(t, "the running commit", run.Commit, "4d2e7b1c3d4e5f60718293a4b5c6d7e8f9a0b1c2")

	testutil.Require(t, "start", post(t, served, appPrefix+"g"+appStartPart, `{}`, cookie).Code, http.StatusOK)
	testutil.Require(t, "stop", post(t, served, appPrefix+"g"+appStopPart, `{}`, cookie).Code, http.StatusOK)
	testutil.Expect(t, "the forms run", rec.runs, []string{"g status", "g start", "g stop"})

	rec.runRefuse = &CandidateRefusal{Code: CodeNoContract, Message: "this goal's candidate cannot run from here: this project has no launch contract"}
	refused := get(t, served, appPrefix+"g"+appStatusPart, nil)
	testutil.Require(t, "no contract", refused.Code, http.StatusConflict)
	refusal := actRefusal(t, refused)
	testutil.Expect(t, "code", refusal.Code, CodeNoContract)
	if !strings.Contains(refusal.Error, "no launch contract") {
		t.Fatalf("the refusal is not in words: %+v", refusal)
	}
	testutil.Require(t, "a status is not a write", post(t, served, appPrefix+"g"+appStatusPart, `{}`, cookie).Code, http.StatusMethodNotAllowed)
}

// RF-04: Try it runs the version under review, not the branch as it stands:
// a room names its review record, the server reads the commit that record
// reviews, and status, start and stop all refer to the run at that commit. A
// record of another goal is not a way to run anything.
func TestTheCandidateRunsTheVersionThePageShows(t *testing.T) {
	t.Parallel()
	shown, now, stranger := strings.Repeat("a", 40), strings.Repeat("c", 40), strings.Repeat("f", 40)
	rec := &verdicting{run: Candidate{Goal: "g", State: "running", Readiness: "answering", Address: "127.0.0.1:7981", Commit: shown}}
	signing := newSigning(t, "Wido", workingSecret)
	rec.sessions = signing.store
	info := rec.serving()
	// Fix round 3, F-1: the page read version A; another room has since moved
	// the review and the branch on to C. Start runs A, the version the page
	// shows, because A is a commit of the goal's branch.
	info.Review = &review.Owner{Git: &movingBranch{tip: now, ancestors: map[string]bool{shown: true}}}
	served := New(info, loopback(), testBundle())
	signedIn := post(t, served, signInPath, `{"code":"`+routeCode(t, routeNow)+`","human":""}`, nil)
	testutil.Require(t, "signed in", signedIn.Code, http.StatusOK)
	cookie := carrying(mintedCookie(t, signedIn).Value)

	testutil.Require(t, "status", get(t, served, appPrefix+"g"+appStatusPart+"?commit="+shown, nil).Code, http.StatusOK)
	testutil.Require(t, "start", post(t, served, appPrefix+"g"+appStartPart, `{"commit":"`+shown+`"}`, cookie).Code, http.StatusOK)
	testutil.Require(t, "stop", post(t, served, appPrefix+"g"+appStopPart, `{"commit":"`+shown+`"}`, cookie).Code, http.StatusOK)
	testutil.Expect(t, "each at the version the page shows", rec.runs, []string{"g " + shown + " status", "g " + shown + " start", "g " + shown + " stop"})

	for _, refused := range []struct{ commit, says string }{
		{stranger, "is not a version of goal g's branch"},
		{"main", "is not a whole commit id"},
	} {
		other := post(t, served, appPrefix+"g"+appStartPart, `{"commit":"`+refused.commit+`"}`, cookie)
		testutil.Require(t, refused.commit+" is refused", other.Code, http.StatusBadRequest)
		if refusal := actRefusal(t, other); !strings.Contains(refusal.Error, refused.says) {
			t.Fatalf("the refusal is not in words: %+v", refusal)
		}
	}
	testutil.Require(t, "with no version named, the goal's branch", post(t, served, appPrefix+"g"+appStartPart, `{}`, cookie).Code, http.StatusOK)
	testutil.Expect(t, "nothing else ran", rec.runs[3:], []string{"g start"})
}

// Fix round 1, F-3: the version that would land is read at the press, not
// taken from the room's earlier read: the builder pushes after the room read
// the branch, and the verdict the act is asked carries the branch as it is
// when the person presses.
func TestAVerdictCarriesTheBranchAsItIsAtThePress(t *testing.T) {
	t.Parallel()
	rec := &verdicting{}
	signing := newSigning(t, "Wido", workingSecret)
	rec.sessions = signing.store
	branch := &movingBranch{tip: strings.Repeat("a", 40)}
	info := rec.serving()
	info.Review = &review.Owner{Git: branch}
	served := New(info, loopback(), testBundle())
	signedIn := post(t, served, signInPath, `{"code":"`+routeCode(t, routeNow)+`","human":""}`, nil)
	testutil.Require(t, "signed in", signedIn.Code, http.StatusOK)
	cookie := carrying(mintedCookie(t, signedIn).Value)

	read, err := info.Review.BranchTip("g")
	testutil.Require(t, "the room's read", err, nil)
	branch.tip = strings.Repeat("c", 40)
	testutil.Require(t, "status", post(t, served, goalsPrefix+"g"+reviewSuffix, clearBody, cookie).Code, http.StatusOK)
	testutil.Expect(t, "the room read the older version", read, strings.Repeat("a", 40))
	testutil.Expect(t, "the act is asked with the branch at the press", rec.asked[0].Branch, strings.Repeat("c", 40))
	testutil.Expect(t, "fetched first, as the gate reads it", branch.fetched, 1)
}

// movingBranch is goal/g's branch as a table one test moves, with the commits
// it carries beneath its tip.
type movingBranch struct {
	deskGit
	tip       string
	fetched   int
	ancestors map[string]bool
}

func (m *movingBranch) MergeBases(left, right string) ([]string, error) {
	if right == m.tip && (left == m.tip || m.ancestors[left]) {
		return []string{left}, nil
	}
	return []string{strings.Repeat("b", 40)}, nil
}

func (m *movingBranch) ResolveCommit(rev string) (string, error) {
	if rev == "origin/goal/g" {
		return m.tip, nil
	}
	return m.deskGit.ResolveCommit(rev)
}

func (m *movingBranch) FetchBranch(string) error {
	m.fetched++
	return nil
}
