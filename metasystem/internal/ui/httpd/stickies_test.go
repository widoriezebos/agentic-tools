package httpd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/stickies"
)

// The notepad's four routes, at the boundary: which verb each path takes, whose
// notepad a request reaches, what a body has to be, and how a refusal is
// answered.
//
// The store itself is the real one over a home of this test's own — what it
// does to a file is proved in internal/ui/stickies — because what this layer
// has to answer for is the human it acts as, and a recorder would not have one.

// notepad is a server with a real notepad under a temporary home, and the
// human it is acting as.
func notepad(t *testing.T, info Info) http.Handler {
	t.Helper()
	info.Stickies = stickies.New(filepath.Join(t.TempDir(), ".metasystem"), "/tmp/workspaces/example",
		func() time.Time { return time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC) })
	return New(info, loopback(), testBundle())
}

func listBody(t *testing.T, what string, response *httptest.ResponseRecorder) stickies.List {
	t.Helper()
	var list stickies.List
	testutil.Require(t, "decode "+what, json.Unmarshal(response.Body.Bytes(), &list), nil)
	return list
}

// One address, read and written, and a third verb learns all three.
func TestTheNotepadIsReadAndWrittenAtOneAddress(t *testing.T) {
	t.Parallel()

	served := notepad(t, Info{Authority: AuthorityInfo{Proven: true, Human: "Wido"}})

	read := request(t, served, http.MethodGet, stickiesPath, "127.0.0.1:7878", nil)
	testutil.Expect(t, "reading the notepad", read.Code, http.StatusOK)
	testutil.Expect(t, "what it answers with", listBody(t, "the read", read).Human, "Wido")

	written := post(t, served, stickiesPath, `{"text":"ask Sol about the retry"}`, nil)
	testutil.Expect(t, "writing a sticky", written.Code, http.StatusOK)

	refused := request(t, served, http.MethodDelete, stickiesPath, "127.0.0.1:7878", nil)
	testutil.Expect(t, "a verb it does not take", refused.Code, http.StatusMethodNotAllowed)
	testutil.Expect(t, "what it says it takes", refused.Header().Get("Allow"), "GET, HEAD, POST")
}

// The acts beneath the collection take POST and nothing else, as every other
// write route does.
func TestTheNotepadsActsTakePostAndSayIt(t *testing.T) {
	t.Parallel()

	served := notepad(t, Info{Authority: AuthorityInfo{Proven: true, Human: "Wido"}})

	for _, path := range []string{stickiesPrefix + "STICKY1", stickiesPrefix + "STICKY1" + removeSuffix} {
		refused := request(t, served, http.MethodGet, path, "127.0.0.1:7878", nil)
		testutil.Expect(t, path+" refuses a read", refused.Code, http.StatusMethodNotAllowed)
		testutil.Expect(t, path+" says what it takes", refused.Header().Get("Allow"), "POST")
	}
}

// The four acts, end to end: write one, change it, mark it done, remove it.
// Every one of them answers the whole notepad, because every one of them
// changes what the panel shows.
func TestEveryActAnswersTheWholeNotepad(t *testing.T) {
	t.Parallel()

	served := notepad(t, Info{Authority: AuthorityInfo{Proven: true, Human: "Wido"}})

	written := post(t, served, stickiesPath,
		`{"text":"ask Sol about the retry","about":[{"kind":"goal","id":"g1-s45"}]}`, nil)
	testutil.Expect(t, "writing a sticky", written.Code, http.StatusOK)
	list := listBody(t, "the write", written)
	testutil.Expect(t, "how many", len(list.Stickies), 1)
	testutil.Expect(t, "the counts after writing", list.Counts, stickies.Counts{Open: 1})
	id := list.Stickies[0].ID

	changed := post(t, served, stickiesPrefix+id, `{"text":"ask Sol about the retry cap"}`, nil)
	testutil.Expect(t, "changing it", changed.Code, http.StatusOK)
	testutil.Expect(t, "the text it now carries",
		listBody(t, "the change", changed).Stickies[0].Text, "ask Sol about the retry cap")

	done := post(t, served, stickiesPrefix+id, `{"done":true}`, nil)
	testutil.Expect(t, "striking it off", done.Code, http.StatusOK)
	testutil.Expect(t, "the counts once it is done",
		listBody(t, "the strike", done).Counts, stickies.Counts{Done: 1})

	removed := post(t, served, stickiesPrefix+id+removeSuffix, `{}`, nil)
	testutil.Expect(t, "removing it", removed.Code, http.StatusOK)
	testutil.Expect(t, "what is left", len(listBody(t, "the removal", removed).Stickies), 0)
}

// Whose notepad a request reaches is the human this server is acting as, and
// nothing in the request says who that is. A seat that knows nobody keeps the
// seat's own notepad, under the empty handle, and says so by naming nobody.
func TestTheNotepadIsTheHumanTheServerIsActingAs(t *testing.T) {
	t.Parallel()

	proven := notepad(t, Info{Authority: AuthorityInfo{Proven: true, Human: "Wido"}})
	testutil.Expect(t, "who a proven seat acts as",
		listBody(t, "a proven seat's read",
			request(t, proven, http.MethodGet, stickiesPath, "127.0.0.1:7878", nil)).Human, "Wido")

	unproven := notepad(t, Info{Authority: AuthorityInfo{Reason: "started by an agent"}})
	answered := listBody(t, "an unproven seat's read",
		request(t, unproven, http.MethodGet, stickiesPath, "127.0.0.1:7878", nil))
	testutil.Expect(t, "who a seat that knows nobody acts as", answered.Human, "")
	testutil.Expect(t, "that it still keeps a notepad", answered.SchemaVersion, stickies.SchemaVersion)
}

// A refusal carries the status its shape deserves and the sentence a human
// reads. The words are the store's own: this layer adds a number and nothing
// else.
func TestARefusedActCarriesItsStatusAndItsWords(t *testing.T) {
	t.Parallel()

	served := notepad(t, Info{Authority: AuthorityInfo{Proven: true, Human: "Wido"}})
	written := post(t, served, stickiesPath, `{"text":"one"}`, nil)
	testutil.Expect(t, "writing a sticky", written.Code, http.StatusOK)
	id := listBody(t, "the write", written).Stickies[0].ID

	for _, refused := range []struct {
		what   string
		path   string
		body   string
		status int
		says   string
	}{
		{"an empty sticky", stickiesPath, `{"text":"   "}`, http.StatusBadRequest, "is empty"},
		{"an unknown about", stickiesPath, `{"text":"one","about":[{"kind":"machine","id":"m1a"}]}`,
			http.StatusBadRequest, "a sticky is about a goal or a record"},
		{"a sticky nobody wrote", stickiesPrefix + "STICKY9", `{"done":true}`,
			http.StatusNotFound, "no sticky of yours"},
		{"removing one nobody wrote", stickiesPrefix + "STICKY9" + removeSuffix, `{}`,
			http.StatusNotFound, "no sticky of yours"},
		{"a field this route does not take", stickiesPrefix + id, `{"colour":"yellow"}`,
			http.StatusBadRequest, "not the JSON object this route takes"},
		{"a body on a removal", stickiesPrefix + id + removeSuffix, `{"done":true}`,
			http.StatusBadRequest, "not the JSON object this route takes"},
	} {
		response := post(t, served, refused.path, refused.body, nil)
		testutil.Expect(t, refused.what+" is refused", response.Code, refused.status)
		testutil.ExpectMatch(t, refused.what+" says why", response.Body.String(), matching(refused.says))
	}
}

// The text bound is answered as unprocessable, with the store's own sentence:
// the request was well formed and the notepad refuses what it says.
func TestTheTextBoundIsAnsweredAsUnprocessable(t *testing.T) {
	t.Parallel()

	served := notepad(t, Info{Authority: AuthorityInfo{Proven: true, Human: "Wido"}})
	long, err := json.Marshal(map[string]string{"text": strings.Repeat("a", stickies.MaxText+1)})
	testutil.Require(t, "composing the body", err, nil)

	response := post(t, served, stickiesPath, string(long), nil)

	testutil.Expect(t, "too much text", response.Code, http.StatusUnprocessableEntity)
	testutil.ExpectMatch(t, "what it says", response.Body.String(), matching("at most 2000 characters"))
}

// An engine with no notepad says so rather than answering an empty one, which
// is what every other route of this server does with a reader it was not given.
func TestAnEngineWithNoNotepadSaysSo(t *testing.T) {
	t.Parallel()

	served := New(Info{Authority: AuthorityInfo{Proven: true, Human: "Wido"}}, loopback(), testBundle())

	for _, asked := range []struct {
		what     string
		response *httptest.ResponseRecorder
	}{
		{"reading", request(t, served, http.MethodGet, stickiesPath, "127.0.0.1:7878", nil)},
		{"writing", post(t, served, stickiesPath, `{"text":"one"}`, nil)},
		{"changing", post(t, served, stickiesPrefix+"STICKY1", `{"done":true}`, nil)},
		{"removing", post(t, served, stickiesPrefix+"STICKY1"+removeSuffix, `{}`, nil)},
	} {
		testutil.Expect(t, asked.what+" with no notepad", asked.response.Code, http.StatusInternalServerError)
		testutil.ExpectMatch(t, asked.what+" says what is missing",
			asked.response.Body.String(), matching("built without a notepad"))
	}
}

// The policy every other route takes holds here too: a cross-site request and
// a host this server does not answer for are both refused before anything is
// read or written.
func TestTheNotepadTakesTheSamePolicyAsEveryOtherRoute(t *testing.T) {
	t.Parallel()

	served := notepad(t, Info{Authority: AuthorityInfo{Proven: true, Human: "Wido"}})

	crossSite := post(t, served, stickiesPath, `{"text":"one"}`, map[string]string{"Sec-Fetch-Site": "cross-site"})
	testutil.Expect(t, "a cross-site write", crossSite.Code, http.StatusForbidden)

	elsewhere := post(t, served, stickiesPath, `{"text":"one"}`,
		map[string]string{"Origin": "http://example.invalid"})
	testutil.Expect(t, "a write from another origin", elsewhere.Code, http.StatusForbidden)

	wrongHost := request(t, served, http.MethodGet, stickiesPath, "example.invalid", nil)
	testutil.Expect(t, "a read for a host this server does not answer for", wrongHost.Code, http.StatusForbidden)

	// And nothing reached the notepad.
	testutil.Expect(t, "what was written",
		len(listBody(t, "the read after the refusals",
			request(t, served, http.MethodGet, stickiesPath, "127.0.0.1:7878", nil)).Stickies), 0)
}

// A path beneath the collection that names nothing, and one that names
// something deeper than a sticky, are no route at all.
func TestAPathBeneathTheNotepadThatNamesNoStickyIsNoRoute(t *testing.T) {
	t.Parallel()

	for _, path := range []string{stickiesPrefix, stickiesPrefix + removeSuffix, stickiesPrefix + "a/b"} {
		_, routed := stickyRouteOf(path)
		testutil.Expect(t, path+" is no route", routed, false)
	}
	route, routed := stickyRouteOf(stickiesPrefix + "STICKY1")
	testutil.Expect(t, "one sticky is the edit route", routed && route.route == routeEditSticky, true)
	testutil.Expect(t, "and carries its id", route.id, "STICKY1")
}

// matching is one sentence, looked for as itself: a refusal's words are prose,
// and reading them as a pattern would make a full stop a wildcard.
func matching(said string) *regexp.Regexp { return regexp.MustCompile(regexp.QuoteMeta(said)) }
