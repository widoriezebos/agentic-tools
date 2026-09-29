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
		Candidate: func(goal, action string) (Candidate, error) {
			rec.runs = append(rec.runs, goal+" "+action)
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
		`{"record":"metasystem/plans/reviews/review-of-g.md","verdict":"send-back","brief":"# Brief\n\n1. Fix it.\n","work":"writer"}`, cookie)
	testutil.Require(t, "status", response.Code, http.StatusOK)
	testutil.Expect(t, "what the act was asked", rec.asked, []act.Reviewed{{
		Record: "metasystem/plans/reviews/review-of-g.md", Verdict: "send-back", Brief: "# Brief\n\n1. Fix it.\n", Work: "writer",
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
