package httpd

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner/fakeacp"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/review"
)

// The room for every sitting at the boundary (g1-s67 §6): the desk's source
// read of a sitting that shapes a record reads the checkout as it stands, the
// change index is refused for one in words, a review's reads are unchanged, and
// the record's own page learns whether a sitting stands on it, for its door.

const shapedDesign = "plans/designs/sessions.md"

const shapeSessions = `{"purpose":"shape a design","subject":{"kind":"record","id":"` + shapedDesign +
	`","title":"Sessions"},"about":{"section":"Project","path":"/project/doc/` + shapedDesign + `"}}`

func TestAShapingDeskReadsTheCheckoutAndHasNoChange(t *testing.T) {
	t.Parallel()
	served := serveReview(t, fakeacp.Script{Chunks: []string{"The records hold ..."}})
	events, stop := served.service.Subscribe()
	defer stop()
	testutil.Require(t, "the sitting opened", post(t, served.handler, partnerSittingPath, shapeSessions, nil).Code, http.StatusOK)
	drain(t, events)
	file := filepath.Join(served.checkout, "internal", "owner.go")
	testutil.Require(t, "the directory", os.MkdirAll(filepath.Dir(file), 0o755), nil)
	testutil.Require(t, "the file as it stands", os.WriteFile(file, []byte("package owner\n\nfunc lock() {}\n"), 0o644), nil)

	source := get(t, served.handler, reviewPrefix+shapedDesign+"/source?path=internal/owner.go&from=3&to=3", nil)
	testutil.Require(t, "the source read", source.Code, http.StatusOK)
	var read review.Source
	testutil.Require(t, "the source decodes", json.Unmarshal(source.Body.Bytes(), &read), nil)
	testutil.Expect(t, "the checkout's, as it stands", read.Checkout, true)
	testutil.Expect(t, "at no commit", read.Commit, "")
	testutil.Expect(t, "its line", read.Lines, []review.SourceLine{{Number: 3, Text: "func lock() {}"}})
	intent := get(t, served.handler, reviewPrefix+"plans/intent/sessions.md/source?path=internal/owner.go", nil)
	testutil.Expect(t, "no sitting stands on the intent", intent.Code, http.StatusBadRequest)
	testutil.Expect(t, "so its desk is not read", errorOf(t, intent),
		"no sitting of yours stands on plans/intent/sessions.md; its desk is read in its room")

	escaped := get(t, served.handler, reviewPrefix+shapedDesign+"/source?path=../secrets", nil)
	testutil.Expect(t, "a path out of the checkout", escaped.Code, http.StatusBadRequest)
	testutil.Expect(t, "said so", errorOf(t, escaped), `"../secrets" is not a path inside the checkout`)

	changes := get(t, served.handler, reviewPrefix+shapedDesign+"/changes", nil)
	testutil.Expect(t, "no change index for a design", changes.Code, http.StatusBadRequest)
	testutil.Expect(t, "in words", errorOf(t, changes),
		"a sitting on a design has no change to index; its desk reads the checkout as it stands")
	diff := get(t, served.handler, reviewPrefix+"plans/intent/sessions.md/changes?path=internal/owner.go", nil)
	testutil.Expect(t, "no diff for an intent", diff.Code, http.StatusBadRequest)
	testutil.Expect(t, "in its words", errorOf(t, diff),
		"a sitting on an intent has no change to index; its desk reads the checkout as it stands")

	other := get(t, served.handler, reviewPrefix+"plans/doctrine/sessions.md/source?path=internal/owner.go", nil)
	testutil.Expect(t, "a record no sitting is about", other.Code, http.StatusBadRequest)
	testutil.Expect(t, "says so", errorOf(t, other), "plans/doctrine/sessions.md is not a record a sitting is about")

	reviewedSource := get(t, served.handler, reviewPrefix+reviewed+"/source?path=internal/owner.go", nil)
	testutil.Require(t, "a review's read", reviewedSource.Code, http.StatusOK)
	var tip review.Source
	testutil.Require(t, "the review read decodes", json.Unmarshal(reviewedSource.Body.Bytes(), &tip), nil)
	testutil.Expect(t, "a review still reads its tip, not the checkout", []any{tip.Commit, tip.Checkout, tip.Total},
		[]any{reviewTip, false, 3})
}

// The shaping desk reads only what the interface serves (Sol SOL-S67-01): the
// document reader's refused segments and the local configuration are refused,
// by name and through a link inside the checkout, and bytes that are not UTF-8
// are refused rather than shown as other characters.
func TestAShapingDeskRefusesWhatTheInterfaceNeverServes(t *testing.T) {
	t.Parallel()
	served := serveReview(t, fakeacp.Script{Chunks: []string{"The records hold ..."}})
	events, stop := served.service.Subscribe()
	defer stop()
	testutil.Require(t, "the sitting opened", post(t, served.handler, partnerSittingPath, shapeSessions, nil).Code, http.StatusOK)
	drain(t, events)
	for name, body := range map[string]string{
		"internal/owner.go":                "package owner\n",
		".git/config":                      "[core]\n",
		"artifacts/run.log":                "ran\n",
		"metasystem/metasystem.conf.local": "secret = 1\n",
		"latin.txt":                        "caf\xe9\n",
	} {
		file := filepath.Join(served.checkout, filepath.FromSlash(name))
		testutil.Require(t, "the directory of "+name, os.MkdirAll(filepath.Dir(file), 0o755), nil)
		testutil.Require(t, "the file "+name, os.WriteFile(file, []byte(body), 0o644), nil)
	}
	testutil.Require(t, "the alias",
		os.Symlink(filepath.Join("metasystem", "metasystem.conf.local"), filepath.Join(served.checkout, "alias.txt")), nil)

	plain := get(t, served.handler, reviewPrefix+shapedDesign+"/source?path=internal/owner.go", nil)
	testutil.Expect(t, "a plain source file is still read", plain.Code, http.StatusOK)
	for _, refused := range []struct{ what, path, reason string }{
		{"Git's directory", ".git/config", ".git/config is a file this interface never serves"},
		{"a run's artifacts", "artifacts/run.log", "artifacts/run.log is a file this interface never serves"},
		{"the local configuration", "metasystem/metasystem.conf.local",
			"metasystem/metasystem.conf.local is a file this interface never serves"},
		{"a link to the local configuration", "alias.txt", "alias.txt is a file this interface never serves"},
		{"a file that is not UTF-8", "latin.txt", "latin.txt is not UTF-8 text, and the desk shows text as it is"},
	} {
		read := get(t, served.handler, reviewPrefix+shapedDesign+"/source?path="+refused.path, nil)
		if read.Code != http.StatusBadRequest || errorOf(t, read) != refused.reason {
			t.Errorf("%s: %d %s, want 400 %q", refused.what, read.Code, read.Body.String(), refused.reason)
		}
	}
}

// The door on the record's page (g1-s67 D6): the document route says a sitting
// stands on the record, with its purpose and when its room was last kept.
func TestADocumentSaysWhetherASittingStandsOnIt(t *testing.T) {
	t.Parallel()
	served := serveReview(t, fakeacp.Script{Chunks: []string{"The records hold ..."}})
	events, stop := served.service.Subscribe()
	defer stop()

	before := get(t, served.handler, documentPrefix+shapedDesign, nil)
	testutil.Require(t, "the record read", before.Code, http.StatusOK)
	var none struct {
		ID      string          `json:"id"`
		Sitting json.RawMessage `json:"sitting"`
	}
	testutil.Require(t, "decodes", json.Unmarshal(before.Body.Bytes(), &none), nil)
	testutil.Expect(t, "the record", none.ID, shapedDesign)
	testutil.Expect(t, "no sitting stands yet", string(none.Sitting), "")

	opened := post(t, served.handler, partnerSittingPath, shapeSessions, nil)
	testutil.Require(t, "the sitting opened", opened.Code, http.StatusOK)
	testutil.Expect(t, "answered as the room's conversation", partnerSnapshot(t, opened).Conversation, shapedDesign)
	drain(t, events)
	kept := post(t, served.handler, partnerSittingPath, `{"conversation":"`+shapedDesign+`","room":{"face":"board"}}`, nil)
	testutil.Require(t, "the room kept", kept.Code, http.StatusOK)

	after := get(t, served.handler, documentPrefix+shapedDesign, nil)
	var standing struct {
		Title   string `json:"title"`
		Sitting *struct {
			Purpose      string `json:"purpose"`
			SteppedOutAt string `json:"steppedOutAt"`
		} `json:"sitting"`
	}
	testutil.Require(t, "the standing read decodes", json.Unmarshal(after.Body.Bytes(), &standing), nil)
	testutil.Expect(t, "the document is still the document", standing.Title, "Sessions")
	testutil.Require(t, "a sitting stands", standing.Sitting != nil, true)
	testutil.Expect(t, "its purpose", standing.Sitting.Purpose, partner.PurposeShapeDesign)
	testutil.Expect(t, "when its room was kept", standing.Sitting.SteppedOutAt, "2026-09-28T09:00:00Z")
}

// A shaping room's End: the Outcome is drafted with no verdict, as landed
// (g1-s67 D7), while a review's is refused without one.
func TestAShapingRoomsOutcomeCarriesNoVerdict(t *testing.T) {
	t.Parallel()
	served := serveReview(t, fakeacp.Script{
		Reads: []fakeacp.Read{{Name: "mcp__metasystem__deposit", Title: "deposit(outcome)",
			Result: "prepared\nDeposit: outcome\n--- the deposit follows, whole and to the end ---\nDecided: sessions end on sign-out\n"}},
		Chunks: []string{"Here is what it came to."},
	})
	events, stop := served.service.Subscribe()
	defer stop()
	testutil.Require(t, "the sitting opened", post(t, served.handler, partnerSittingPath, shapeSessions, nil).Code, http.StatusOK)
	drain(t, events)

	closed := post(t, served.handler, partnerSittingClosePath,
		`{"conversation":"`+shapedDesign+`","about":{"section":"Project"}}`, nil)
	testutil.Require(t, "the close was asked without a verdict", closed.Code, http.StatusOK)
	var outcome *partner.Deposit
	for beat := range events {
		if beat.Deposit != nil && beat.Deposit.Kind == "outcome" {
			outcome = beat.Deposit
		}
		if beat.Kind == partner.EventDone || beat.Kind == partner.EventStopped || beat.Kind == partner.EventError {
			break
		}
	}
	testutil.Require(t, "the Outcome arrived", outcome != nil, true)
	testutil.Expect(t, "offered against the design", []any{outcome.Offered, outcome.Subject.ID}, []any{true, shapedDesign})
	testutil.Expect(t, "with no verdict", outcome.Verdict, "")

	testutil.Require(t, "a review opened", post(t, served.handler, partnerSittingPath, reviewLanding, nil).Code, http.StatusOK)
	drain(t, events)
	refused := post(t, served.handler, partnerSittingClosePath, `{"conversation":"`+reviewed+`"}`, nil)
	testutil.Expect(t, "a review's close needs its verdict", refused.Code, http.StatusBadRequest)
}
