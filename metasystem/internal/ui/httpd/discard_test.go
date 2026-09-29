package httpd

// POST /api/fleet/launches/{id}/discard, at the boundary: the path names the
// launch, the body is the empty object, the answer is the record as it now
// reads, and a refusal comes back under the launch's own code.
//
// What a discard does to the record is proved in internal/seat/launch; the
// discarder here is a recorder.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

const discardedLaunch = "01M3BQAVYXE2AT6F0JG9YB64PG"

type discarding struct {
	asked   []string
	refusal error
}

func (rec *discarding) serving() Info {
	return Info{
		Observe: readObservation,
		DiscardLaunch: func(id string) (launch.Record, error) {
			rec.asked = append(rec.asked, id)
			if rec.refusal != nil {
				return launch.Record{}, rec.refusal
			}
			at := "2026-09-29T10:00:00Z"
			return launch.Record{Launch: id, Machine: "m1f", Outcome: launch.OutcomeFailed, DiscardedAt: &at}, nil
		},
	}
}

func discardPathOf(id string) string { return launchesPrefix + id + discardSuffix }

func TestADiscardMarksTheLaunchThePathNamesAndAnswersTheRecord(t *testing.T) {
	t.Parallel()
	rec := &discarding{}
	served := New(rec.serving(), loopback(), testBundle())

	response := post(t, served, discardPathOf(discardedLaunch), `{}`, nil)

	testutil.Require(t, "status", response.Code, http.StatusOK)
	testutil.Expect(t, "the launch the path named", rec.asked, []string{discardedLaunch})
	var answered launch.Record
	testutil.Require(t, "decode the record", json.Unmarshal(response.Body.Bytes(), &answered), nil)
	testutil.Expect(t, "the answer carries the mark", answered.DiscardedAt != nil && *answered.DiscardedAt == "2026-09-29T10:00:00Z", true)
}

func TestADiscardRefusalCarriesItsCodeAndTheStatusThatFitsIt(t *testing.T) {
	t.Parallel()
	for code, status := range map[string]int{
		launch.CodeUnknown:        http.StatusNotFound,
		launch.CodeDiscardRunning: http.StatusConflict,
		launch.CodeIDInvalid:      http.StatusUnprocessableEntity,
	} {
		rec := &discarding{refusal: &launch.Refusal{Code: code, Message: "the owner's own sentence"}}
		served := New(rec.serving(), loopback(), testBundle())

		response := post(t, served, discardPathOf(discardedLaunch), `{}`, nil)

		testutil.Require(t, "status for "+code, response.Code, status)
		refusal := actRefusal(t, response)
		testutil.Expect(t, "code for "+code, refusal.Code, code)
		testutil.Expect(t, "the owner's words for "+code, refusal.Error, "the owner's own sentence")
	}
}

// The write policy every write takes: another site's request, a body carrying
// fields the route does not take, and a GET all reach nothing.
func TestADiscardTakesTheWritePolicyAndNothingLess(t *testing.T) {
	t.Parallel()
	rec := &discarding{}
	served := New(rec.serving(), loopback(), testBundle())

	crossSite := post(t, served, discardPathOf(discardedLaunch), `{}`, map[string]string{"Origin": "http://attacker.invalid"})
	testutil.Expect(t, "another site is refused", crossSite.Code, http.StatusForbidden)
	fields := post(t, served, discardPathOf(discardedLaunch), `{"launch":"x"}`, nil)
	testutil.Expect(t, "a body with fields is refused", fields.Code, http.StatusBadRequest)

	get := httptest.NewRequest(http.MethodGet, "http://example.invalid"+discardPathOf(discardedLaunch), nil)
	get.Host = "127.0.0.1:7878"
	answered := httptest.NewRecorder()
	served.ServeHTTP(answered, get)
	testutil.Expect(t, "a GET is told the verb", answered.Code, http.StatusMethodNotAllowed)
	testutil.Expect(t, "nothing was discarded", len(rec.asked), 0)
}

func TestAnEngineThatCannotDiscardSaysSo(t *testing.T) {
	t.Parallel()
	served := New(Info{Observe: readObservation}, loopback(), testBundle())
	response := post(t, served, discardPathOf(discardedLaunch), `{}`, nil)
	testutil.Require(t, "status", response.Code, http.StatusInternalServerError)
}
