package httpd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/project"
)

// The document's two write routes, at the boundary: which verb each path
// takes, what a body has to be, which status each refusal earns, and that the
// policy every other write is held to holds here too.
//
// The writer is the recorder rather than the real one — what it does to a
// checkout is proved in internal/ui/project — so every case can also assert
// the one thing only this layer can: whether the writer was reached at all.

const editPath = "/api/documents/metasystem/plans/designs/ledger.md/edit"

func documentBody(t *testing.T, what string, response *httptest.ResponseRecorder) project.Document {
	t.Helper()
	var document project.Document
	testutil.Require(t, "decode "+what, json.Unmarshal(response.Body.Bytes(), &document), nil)
	return document
}

// A save names the document by the read route's own id, carries the text and
// the revision it was opened at, and is answered with the document as it now
// reads from disk.
func TestEditDocumentRoute(t *testing.T) {
	t.Parallel()

	rec := &recorder{}
	served := New(rec.writing(), loopback(), testBundle())

	response := post(t, served, editPath,
		`{"source":"# The ledger\n\nEdited.\n","revision":"blob:0000000000000000000000000000000000000001"}`, nil)

	testutil.Expect(t, "status", response.Code, http.StatusOK)
	testutil.Require(t, "one save was made", len(rec.edits), 1)
	testutil.Expect(t, "the save", rec.edits[0], edited{
		id:       "metasystem/plans/designs/ledger.md",
		source:   "# The ledger\n\nEdited.\n",
		revision: "blob:0000000000000000000000000000000000000001",
	})
	answered := documentBody(t, "the document", response)
	testutil.Expect(t, "the answer is the document", answered.ID, "metasystem/plans/designs/ledger.md")
	testutil.Expect(t, "it carries the source", answered.Source, "# The ledger\n\nEdited.\n")
	testutil.Expect(t, "it carries the revision to save against next",
		answered.Revision, "blob:0000000000000000000000000000000000000002")
	testutil.Expect(t, "it carries the blocks", len(answered.Blocks), 1)
}

// A preview names no document and writes nothing: it takes the text and is
// answered with the blocks it renders to.
func TestPreviewDocumentRoute(t *testing.T) {
	t.Parallel()

	rec := &recorder{}
	served := New(rec.writing(), loopback(), testBundle())

	response := post(t, served, "/api/documents/preview", `{"source":"# A title\n"}`, nil)

	testutil.Expect(t, "status", response.Code, http.StatusOK)
	testutil.Expect(t, "the source it rendered", rec.previews, []string{"# A title\n"})
	testutil.Expect(t, "no save was made", len(rec.edits), 0)
	var preview project.Preview
	testutil.Require(t, "decode the preview", json.Unmarshal(response.Body.Bytes(), &preview), nil)
	testutil.Expect(t, "the blocks", len(preview.Blocks), 1)
}

// Each refusal the save can make carries the status a caller acts on: the file
// changed, the text is too large, the project refuses the record, the text is
// not the JSON this route takes.
func TestEditRefusalsCarryTheirStatus(t *testing.T) {
	t.Parallel()

	for _, refused := range []struct {
		what   string
		err    error
		status int
	}{
		{
			"a file that changed underneath",
			&project.Refusal{Kind: project.RefusalStale, Message: "the file changed since you opened it"},
			http.StatusConflict,
		},
		{
			"more text than the reader reads back",
			&project.Refusal{Kind: project.RefusalTooLarge, Message: "the text is larger than 1 MiB, which is more than this interface reads back"},
			http.StatusRequestEntityTooLarge,
		},
		{
			"text that is not text",
			&project.Refusal{Kind: project.RefusalBad, Message: "the text is not valid UTF-8"},
			http.StatusBadRequest,
		},
		{
			"a record the project refuses",
			&project.Refusal{
				Kind:    project.RefusalProject,
				Message: "the record this would write is one the project refuses",
				Problems: []project.Problem{
					{Path: "metasystem/plans/designs/ledger.md", Line: 3, Message: "the status invented is not one of draft, accepted, superseded, done"},
				},
			},
			http.StatusUnprocessableEntity,
		},
	} {
		t.Run(refused.what, func(t *testing.T) {
			t.Parallel()
			rec := &recorder{refusal: refused.err}
			served := New(rec.writing(), loopback(), testBundle())

			response := post(t, served, editPath, `{"source":"# The ledger\n","revision":"blob:0"}`, nil)

			testutil.Expect(t, "status", response.Code, refused.status)
			testutil.Expect(t, "the reason", refusalBody(t, "the refusal", response).Error, refused.err.Error())
		})
	}
}

// A refused record carries the check verb's own problems, so a human reads in
// the editor the sentence they would read in the terminal.
func TestARefusedSaveCarriesItsProblems(t *testing.T) {
	t.Parallel()

	problems := []project.Problem{
		{Path: "metasystem/plans/designs/ledger.md", Line: 3, Message: "the goal logistics is not in the ledger"},
	}
	rec := &recorder{refusal: &project.Refusal{
		Kind: project.RefusalProject, Message: "the record this would write is one the project refuses", Problems: problems,
	}}
	served := New(rec.writing(), loopback(), testBundle())

	response := post(t, served, editPath, `{"source":"# The ledger\n","revision":"blob:0"}`, nil)

	testutil.Expect(t, "status", response.Code, http.StatusUnprocessableEntity)
	testutil.Expect(t, "the problems", refusalBody(t, "the refusal", response).Problems, problems)
}

// An id the reader refuses is refused by the save in the reader's own words:
// this checkout serves no document at that id, and nothing else about the
// filesystem is said.
func TestASaveOfAnIDTheReaderRefuses(t *testing.T) {
	t.Parallel()

	rec := &recorder{refusal: project.ErrNotFound}
	served := New(rec.writing(), loopback(), testBundle())

	response := post(t, served, "/api/documents/nothing/here.md/edit", `{"source":"# A\n","revision":"blob:0"}`, nil)

	testutil.Expect(t, "status", response.Code, http.StatusNotFound)
	testutil.Expect(t, "the reason", refusalBody(t, "the refusal", response).Error, "no document at nothing/here.md")
}

// A body that is not the object these routes take is a bad request, and the
// writer is never reached.
func TestABadDocumentBodyIsRefusedBeforeTheWriter(t *testing.T) {
	t.Parallel()

	for _, bad := range []struct{ what, path, body string }{
		{"not JSON", editPath, "source=hello"},
		{"not an object", editPath, `["# A"]`},
		{"a field the route does not take", editPath, `{"source":"# A","revision":"blob:0","path":"../../etc/passwd"}`},
		{"a second value after the first", editPath, `{"source":"# A"} {"source":"# B"}`},
		{"nothing at all", editPath, ""},
		{"a preview that is not an object", "/api/documents/preview", `"# A"`},
	} {
		t.Run(bad.what, func(t *testing.T) {
			t.Parallel()
			rec := &recorder{}
			served := New(rec.writing(), loopback(), testBundle())

			response := post(t, served, bad.path, bad.body, nil)

			testutil.Expect(t, "status", response.Code, http.StatusBadRequest)
			testutil.Expect(t, "the writer was not reached", rec.reached(), 0)
			testutil.Expect(t, "the body carries a reason", refusalBody(t, "the refusal", response).Error != "", true)
		})
	}
}

// A body past what these routes carry is refused for its size, before it is
// parsed and before the writer is reached.
func TestADocumentBodyPastTheBoundIsRefusedForItsSize(t *testing.T) {
	t.Parallel()

	rec := &recorder{}
	served := New(rec.writing(), loopback(), testBundle())

	response := post(t, served, editPath,
		`{"source":"`+strings.Repeat("x", maxDocumentBody)+`","revision":"blob:0"}`, nil)

	testutil.Expect(t, "status", response.Code, http.StatusRequestEntityTooLarge)
	testutil.Expect(t, "the writer was not reached", rec.reached(), 0)
}

// The policy the read routes are held to holds for these two as well: a
// cross-site request, a foreign origin and a host this server was not bound to
// are each refused before the route is reached, and none of them writes.
func TestTheDocumentWritesKeepTheReadRoutesPolicy(t *testing.T) {
	t.Parallel()

	for _, refused := range []struct {
		what    string
		host    string
		headers map[string]string
	}{
		{"a cross-site request", "127.0.0.1:7878", map[string]string{"Sec-Fetch-Site": "cross-site"}},
		{"a same-site request from another port", "127.0.0.1:7878", map[string]string{"Sec-Fetch-Site": "same-site"}},
		{"a foreign origin", "127.0.0.1:7878", map[string]string{"Origin": "http://attacker.invalid"}},
		{"a host this server is not bound to", "metasystem.invalid", nil},
	} {
		t.Run(refused.what, func(t *testing.T) {
			t.Parallel()
			for _, path := range []string{editPath, "/api/documents/preview"} {
				rec := &recorder{}
				served := New(rec.writing(), loopback(), testBundle())
				req := httptest.NewRequest(http.MethodPost, "http://example.invalid"+path,
					strings.NewReader(`{"source":"# The ledger\n","revision":"blob:0"}`))
				req.Host = refused.host
				for name, value := range refused.headers {
					req.Header.Set(name, value)
				}
				response := httptest.NewRecorder()

				served.ServeHTTP(response, req)

				testutil.Expect(t, path+" status", response.Code, http.StatusForbidden)
				testutil.Expect(t, path+" did not reach the writer", rec.reached(), 0)
			}
		})
	}
}

// Both paths take POST and say so, and the document they are named after is
// still read with GET.
func TestTheDocumentWritesNameTheVerbTheyTake(t *testing.T) {
	t.Parallel()

	rec := &recorder{}
	info := rec.writing()
	info.Document = func(string) (project.Document, error) { return savedDocument("a.md", "# A\n"), nil }
	served := New(info, loopback(), testBundle())

	for _, path := range []string{editPath, "/api/documents/preview"} {
		response := request(t, served, http.MethodGet, path, "127.0.0.1:7878", nil)
		testutil.Expect(t, "GET "+path+" status", response.Code, http.StatusMethodNotAllowed)
		testutil.Expect(t, "GET "+path+" allows", response.Header().Get("Allow"), "POST")
	}
	read := post(t, served, "/api/documents/a.md", `{}`, nil)
	testutil.Expect(t, "POST a document status", read.Code, http.StatusMethodNotAllowed)
	testutil.Expect(t, "POST a document allows", read.Header().Get("Allow"), "GET, HEAD")
	testutil.Expect(t, "the writer was not reached", rec.reached(), 0)
}

// A path beneath the document prefix that names no document to save is no
// write route, and nothing is written through it.
//
// What lies between the prefix and the suffix is not judged here: an id is a
// checkout-relative path, and whether this checkout serves one is the reader's
// question and not the router's, which is why every refusal of an id is the
// reader's own 404 above.
func TestAPathThatIsNoDocumentWriteRoute(t *testing.T) {
	t.Parallel()

	rec := &recorder{}
	served := New(rec.writing(), loopback(), testBundle())

	for _, path := range []string{
		"/api/documents/edit",
		"/api/documents//edit",
		"/api/documents/a.md",
		"/api/documents/a.md/preview",
	} {
		response := post(t, served, path, `{"source":"# A\n","revision":"blob:0"}`, nil)
		testutil.Expect(t, "POST "+path+" is not a save", response.Code != http.StatusOK, true)
	}
	testutil.Expect(t, "the writer was not reached", rec.reached(), 0)
}

// An engine built without the document writer says so, as every other route
// says what it was built without.
func TestAnEngineWithoutADocumentWriter(t *testing.T) {
	t.Parallel()

	served := New(Info{}, loopback(), testBundle())

	for _, absent := range []struct{ path, reason string }{
		{editPath, "this engine was built without a project writer"},
		{"/api/documents/preview", "this engine was built without a document reader"},
	} {
		response := post(t, served, absent.path, `{"source":"# A\n","revision":"blob:0"}`, nil)
		testutil.Expect(t, absent.path+" status", response.Code, http.StatusInternalServerError)
		testutil.Expect(t, absent.path+" reason", refusalBody(t, absent.path, response).Error, absent.reason)
	}
}
