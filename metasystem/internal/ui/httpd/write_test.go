package httpd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/markdown"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/project"
)

// The write routes, at the boundary: which verb each path takes, what a body
// has to be, how a refusal is answered, and that the policy the read routes
// are held to holds here too.
//
// The writer itself is a recorder rather than the real one — what it does to a
// checkout is proved in internal/ui/project — so every case can also assert
// the one thing only this layer can: whether the writer was reached at all.

type recorder struct {
	records   []project.NewRecord
	questions []project.NewQuestion
	statuses  [][2]string
	answers   [][2]string
	// The document's two: what a save was asked to write, and what a preview
	// was asked to render.
	edits    []edited
	previews []string
	// named is every goal a record was asked to name, as the pair the route
	// carried: the record's id from the path, and the goal from the body.
	named [][2]string
	// scoped is every whole list a record was told it is about, as the route
	// carried it: the record's id from the path, and the goals from the body.
	// It is kept apart from named because the two are different statements
	// made through the same resource, and only this layer can say which of
	// them a body asked for.
	scoped  []scopedTo
	refusal error
}

// scopedTo is one statement of what a record is about, as it reached the
// writer: which record, and the whole list it was told to say.
type scopedTo struct {
	id    string
	goals []string
}

// edited is one save as it reached the writer: which document, the text, and
// the revision the caller said it was holding.
type edited struct{ id, source, revision string }

func (rec *recorder) writing() Info {
	return Info{
		CreateRecord: func(asked project.NewRecord) (project.Written, error) {
			rec.records = append(rec.records, asked)
			if rec.refusal != nil {
				return project.Written{}, rec.refusal
			}
			return writtenRecord(), nil
		},
		SetStatus: func(id, status string) (project.Written, error) {
			rec.statuses = append(rec.statuses, [2]string{id, status})
			if rec.refusal != nil {
				return project.Written{}, rec.refusal
			}
			written := writtenRecord()
			written.Record.Status = status
			return written, nil
		},
		AddRecordGoal: func(id, goal string) (project.Document, error) {
			rec.named = append(rec.named, [2]string{id, goal})
			if rec.refusal != nil {
				return project.Document{}, rec.refusal
			}
			return namedDocument(goal), nil
		},
		SetRecordGoals: func(id string, goals []string) (project.Document, error) {
			rec.scoped = append(rec.scoped, scopedTo{id: id, goals: goals})
			if rec.refusal != nil {
				return project.Document{}, rec.refusal
			}
			return scopedDocument(goals), nil
		},
		AskQuestion: func(asked project.NewQuestion) (project.Asked, error) {
			rec.questions = append(rec.questions, asked)
			if rec.refusal != nil {
				return project.Asked{}, rec.refusal
			}
			return askedQuestion("open"), nil
		},
		SetQuestionStatus: func(id, status string) (project.Asked, error) {
			rec.answers = append(rec.answers, [2]string{id, status})
			if rec.refusal != nil {
				return project.Asked{}, rec.refusal
			}
			return askedQuestion(status), nil
		},
		EditDocument: func(id, source, revision string) (project.Document, error) {
			rec.edits = append(rec.edits, edited{id: id, source: source, revision: revision})
			if rec.refusal != nil {
				return project.Document{}, rec.refusal
			}
			return savedDocument(id, source), nil
		},
		PreviewDocument: func(source string) (project.Preview, error) {
			rec.previews = append(rec.previews, source)
			if rec.refusal != nil {
				return project.Preview{}, rec.refusal
			}
			return project.Preview{Blocks: []markdown.Block{{Type: "paragraph"}}}, nil
		},
	}
}

func (rec *recorder) reached() int {
	return len(rec.records) + len(rec.questions) + len(rec.statuses) + len(rec.answers) +
		len(rec.edits) + len(rec.previews) + len(rec.named)
}

// namedDocument is what naming a goal answers with: the document as it now
// reads from disk, which at this layer is the head carrying the goal that was
// just written into it.
// scopedDocument is the document the set act answers with: the read of what
// is now on disk, which at this layer is whatever the writer was handed.
func scopedDocument(goals []string) project.Document {
	document := namedDocument("")
	document.Record = &project.Head{Kind: "design", ID: "01K7", Status: "accepted", Goals: goals}
	return document
}

func namedDocument(goal string) project.Document {
	return project.Document{
		Kind: "document", ID: "plans/designs/the-design.md", Title: "The design",
		Path: "/work/repository/plans/designs/the-design.md", State: "readable",
		Record: &project.Head{Kind: "design", ID: "01K7", Status: "accepted", Goals: []string{goal}},
	}
}

// savedDocument is the document a save answers with: the read of what is now
// on disk, which at this layer is whatever the writer was handed.
func savedDocument(id, source string) project.Document {
	return project.Document{
		Kind: "document", ID: id, Title: "The ledger", Source: source,
		Revision: "blob:0000000000000000000000000000000000000002",
		Owner:    "project", Path: "/work/repository/" + id, State: "readable",
		Blocks: []markdown.Block{{Type: "heading", Level: 1, Text: "The ledger"}},
	}
}

func writtenRecord() project.Written {
	return project.Written{
		Record: project.Record{
			Kind: "decision", ID: "01K7", Status: "draft", Goals: []string{"g1-s22"},
			Title: "The Project Partner is a drawer", Path: "metasystem/docs/decisions/the-drawer.md",
			Home: "metasystem/docs/decisions", Summary: "",
		},
		Path:     "metasystem/docs/decisions/the-drawer.md",
		Absolute: "/work/repository/metasystem/docs/decisions/the-drawer.md",
	}
}

func askedQuestion(status string) project.Asked {
	return project.Asked{Question: project.Question{
		ID: "Q-01K8", Opened: "2026-09-22", Question: "Who accepts a decision?",
		Goals: []string{"g1-s22"}, Status: status,
	}}
}

// post is request with a body, which the read routes never need.
func post(t *testing.T, handler http.Handler, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "http://example.invalid"+path, strings.NewReader(body))
	req.Host = "127.0.0.1:7878"
	req.Header.Set("Content-Type", "application/json")
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	return response
}

func refusalBody(t *testing.T, what string, response *httptest.ResponseRecorder) struct {
	Error    string            `json:"error"`
	Problems []project.Problem `json:"problems"`
} {
	t.Helper()
	var body struct {
		Error    string            `json:"error"`
		Problems []project.Problem `json:"problems"`
	}
	testutil.Require(t, "decode "+what, json.Unmarshal(response.Body.Bytes(), &body), nil)
	return body
}

// A created record comes back as the pane shapes it, with both the paths a
// caller needs: the one every answer names it by, and the file an editor opens.
func TestCreateRecordRoute(t *testing.T) {
	t.Parallel()

	rec := &recorder{}
	served := New(rec.writing(), loopback(), testBundle())

	response := post(t, served, "/api/project/records",
		`{"kind":"decision","title":"The Project Partner is a drawer","goals":["g1-s22"],"affects":["01K5"]}`, nil)

	testutil.Require(t, "status", response.Code, http.StatusOK)
	testutil.Expect(t, "content type", response.Header().Get("Content-Type"), "application/json")
	var written project.Written
	testutil.Require(t, "decode", json.Unmarshal(response.Body.Bytes(), &written), nil)
	testutil.Expect(t, "the payload", written, writtenRecord())
	testutil.Expect(t, "what the writer was asked for", rec.records, []project.NewRecord{{
		Kind: "decision", Title: "The Project Partner is a drawer",
		Goals: []string{"g1-s22"}, Affects: []string{"01K5"},
	}})
}

// A status change names a record by its id in the path and carries the new
// status in the body; nothing about the file is in either.
func TestRecordStatusRoute(t *testing.T) {
	t.Parallel()

	rec := &recorder{}
	served := New(rec.writing(), loopback(), testBundle())

	response := post(t, served, "/api/project/records/01K7/status", `{"status":"accepted"}`, nil)

	testutil.Require(t, "status", response.Code, http.StatusOK)
	var written project.Written
	testutil.Require(t, "decode", json.Unmarshal(response.Body.Bytes(), &written), nil)
	testutil.Expect(t, "the record's status", written.Record.Status, "accepted")
	testutil.Expect(t, "what the writer was asked for", rec.statuses, [][2]string{{"01K7", "accepted"}})
}

// Naming a goal on a record carries the record's id in the path and the
// goal's in the body, and answers the document as it now reads from disk, so
// the page re-renders from the file rather than from the request.
func TestRecordGoalsRoute(t *testing.T) {
	t.Parallel()

	rec := &recorder{}
	served := New(rec.writing(), loopback(), testBundle())

	response := post(t, served, "/api/project/records/01K7/goals", `{"add":"g1-s26"}`, nil)

	testutil.Require(t, "status", response.Code, http.StatusOK)
	testutil.Expect(t, "content type", response.Header().Get("Content-Type"), "application/json")
	var document project.Document
	testutil.Require(t, "decode", json.Unmarshal(response.Body.Bytes(), &document), nil)
	testutil.Expect(t, "the payload", document, namedDocument("g1-s26"))
	testutil.Expect(t, "what the writer was asked for", rec.named, [][2]string{{"01K7", "g1-s26"}})
}

// The same resource carries two different statements, and the body says which.
//
// A list is a human saying what this record is for, from the record's own
// page; a single id is the machine writing down a goal it has just minted. So
// a body with a list reaches the one writer and a body with an id reaches the
// other, and neither route guesses.
func TestRecordGoalsRouteSetsTheWholeList(t *testing.T) {
	t.Parallel()

	rec := &recorder{}
	served := New(rec.writing(), loopback(), testBundle())

	response := post(t, served, "/api/project/records/01K7/goals", `{"set":["g1-s26","g1-s27"]}`, nil)

	testutil.Require(t, "status", response.Code, http.StatusOK)
	testutil.Expect(t, "content type", response.Header().Get("Content-Type"), "application/json")
	var document project.Document
	testutil.Require(t, "decode", json.Unmarshal(response.Body.Bytes(), &document), nil)
	testutil.Expect(t, "the payload", document, scopedDocument([]string{"g1-s26", "g1-s27"}))
	testutil.Expect(t, "what the writer was asked for", rec.scoped,
		[]scopedTo{{id: "01K7", goals: []string{"g1-s26", "g1-s27"}}})
	testutil.Expect(t, "and the other act was not reached", rec.named, [][2]string(nil))
}

// An empty list is a statement and a missing one is not: a record told it is
// about nothing is a record about the project as a whole, and a body that
// carries no list at all is asking for the other act.
func TestRecordGoalsRouteTellsAnEmptyListFromAMissingOne(t *testing.T) {
	t.Parallel()

	rec := &recorder{}
	served := New(rec.writing(), loopback(), testBundle())

	empty := post(t, served, "/api/project/records/01K7/goals", `{"set":[]}`, nil)

	testutil.Require(t, "the empty list is answered", empty.Code, http.StatusOK)
	testutil.Expect(t, "the whole list, which is none of them", rec.scoped,
		[]scopedTo{{id: "01K7", goals: []string{}}})
	testutil.Expect(t, "and the other act was not reached", rec.named, [][2]string(nil))

	// The same record, through the same resource, asking for the other act.
	added := post(t, served, "/api/project/records/01K7/goals", `{"add":"g1-s26"}`, nil)

	testutil.Require(t, "the add is answered too", added.Code, http.StatusOK)
	testutil.Expect(t, "the machine's own act", rec.named, [][2]string{{"01K7", "g1-s26"}})
	testutil.Expect(t, "and no second statement of scope", len(rec.scoped), 1)
}

// A refusal of the set act is answered exactly as a refusal of the add act is:
// the status the refusal deserves, and the writer's own sentence, because that
// sentence is what a human acts on.
func TestRecordGoalsRouteAnswersASetRefusal(t *testing.T) {
	t.Parallel()

	rec := &recorder{refusal: &project.Refusal{
		Kind: project.RefusalBad, Message: `the goal "g1-s99" is not in the ledger`,
	}}
	served := New(rec.writing(), loopback(), testBundle())

	response := post(t, served, "/api/project/records/01K7/goals", `{"set":["g1-s99"]}`, nil)

	testutil.Require(t, "status", response.Code, http.StatusBadRequest)
	testutil.Expect(t, "the writer's own words", refusalBody(t, "the refusal", response).Error,
		`the goal "g1-s99" is not in the ledger`)
	testutil.Expect(t, "the writer was reached", len(rec.scoped), 1)
}

// An engine built without a project writer says so rather than answering as
// though the write had happened.
func TestRecordGoalsRouteSaysWhenThereIsNoWriter(t *testing.T) {
	t.Parallel()

	served := New(Info{}, loopback(), testBundle())

	response := post(t, served, "/api/project/records/01K7/goals", `{"set":["g1-s26"]}`, nil)

	testutil.Expect(t, "status", response.Code, http.StatusInternalServerError)
}

// The goals route refuses what every other write route refuses, with the
// status a caller acts on: a goal the ledger does not carry is a bad request,
// and a goal the head already names is a conflict.
func TestRecordGoalsRefusals(t *testing.T) {
	t.Parallel()

	for _, refused := range []struct {
		what   string
		err    error
		status int
	}{
		{"a goal the ledger does not carry",
			&project.Refusal{Kind: project.RefusalBad, Message: `the goal "logistics" is not in the ledger`},
			http.StatusBadRequest},
		{"a goal the head already names",
			&project.Refusal{Kind: project.RefusalExists, Message: "plans/designs/the-design.md already names the goal g1-s26"},
			http.StatusConflict},
		{"a chapter of a book",
			&project.Refusal{Kind: project.RefusalBad, Message: "an intent or doctrine record names goals"},
			http.StatusBadRequest},
		{"an id nothing declares",
			&project.Refusal{Kind: project.RefusalAbsent, Message: "no record declares the id 01K9"},
			http.StatusNotFound},
	} {
		t.Run(refused.what, func(t *testing.T) {
			t.Parallel()
			rec := &recorder{refusal: refused.err}
			served := New(rec.writing(), loopback(), testBundle())

			response := post(t, served, "/api/project/records/01K7/goals", `{"add":"g1-s26"}`, nil)

			testutil.Expect(t, "status", response.Code, refused.status)
			testutil.Expect(t, "the reason", refusalBody(t, "the refusal", response).Error, refused.err.Error())
		})
	}
}

// A cross-site request to the goals route writes nothing, exactly as it writes
// nothing to the four routes that were here before it.
func TestTheGoalsRouteKeepsThePolicy(t *testing.T) {
	t.Parallel()

	rec := &recorder{}
	served := New(rec.writing(), loopback(), testBundle())

	response := post(t, served, "/api/project/records/01K7/goals", `{"add":"g1-s26"}`,
		map[string]string{"Sec-Fetch-Site": "cross-site"})

	testutil.Expect(t, "status", response.Code, http.StatusForbidden)
	testutil.Expect(t, "the writer was not reached", rec.reached(), 0)
}

func TestQuestionRoutes(t *testing.T) {
	t.Parallel()

	rec := &recorder{}
	served := New(rec.writing(), loopback(), testBundle())

	asked := post(t, served, "/api/project/questions",
		`{"question":"Who accepts a decision?","goals":["g1-s22"]}`, nil)

	testutil.Require(t, "ask status", asked.Code, http.StatusOK)
	var body project.Asked
	testutil.Require(t, "decode the question", json.Unmarshal(asked.Body.Bytes(), &body), nil)
	testutil.Expect(t, "the row", body, askedQuestion("open"))
	testutil.Expect(t, "what the writer was asked for", rec.questions,
		[]project.NewQuestion{{Question: "Who accepts a decision?", Goals: []string{"g1-s22"}}})

	answered := post(t, served, "/api/project/questions/Q-01K8/status",
		`{"status":"answered: 01K7"}`, nil)

	testutil.Require(t, "answer status", answered.Code, http.StatusOK)
	testutil.Require(t, "decode the answer", json.Unmarshal(answered.Body.Bytes(), &body), nil)
	testutil.Expect(t, "the row's status", body.Question.Status, "answered: 01K7")
	testutil.Expect(t, "what the writer was asked to answer", rec.answers, [][2]string{{"Q-01K8", "answered: 01K7"}})
}

// The three shapes a refusal takes, each with the status a caller acts on, and
// the check verb's own problems where the project refused what it was given.
func TestRefusalsCarryTheirStatus(t *testing.T) {
	t.Parallel()

	for _, refused := range []struct {
		what   string
		err    error
		status int
	}{
		{"a bad request", &project.Refusal{Kind: project.RefusalBad, Message: `the goal "logistics" is not in the ledger`}, http.StatusBadRequest},
		{"an id nothing declares", &project.Refusal{Kind: project.RefusalAbsent, Message: "no record declares the id 01K9"}, http.StatusNotFound},
		{"a file that is already there", &project.Refusal{Kind: project.RefusalExists, Message: "a file already lives at metasystem/docs/decisions/one-binary.md"}, http.StatusConflict},
	} {
		t.Run(refused.what, func(t *testing.T) {
			t.Parallel()
			rec := &recorder{refusal: refused.err}
			served := New(rec.writing(), loopback(), testBundle())

			response := post(t, served, "/api/project/records",
				`{"kind":"decision","title":"One binary","goals":["logistics"]}`, nil)

			testutil.Expect(t, "status", response.Code, refused.status)
			testutil.Expect(t, "the reason", refusalBody(t, "the refusal", response).Error, refused.err.Error())
		})
	}
}

// A refusal the project made over what would have been written carries the
// problems the check verb would have printed, so a human reads the same
// sentence in the sheet that they would read in the terminal.
func TestARefusedRecordCarriesItsProblems(t *testing.T) {
	t.Parallel()

	problems := []project.Problem{
		{Path: "metasystem/docs/decisions/one.md", Line: 3, Message: "the goal logistics is not in the ledger"},
	}
	rec := &recorder{refusal: &project.Refusal{
		Kind: project.RefusalBad, Message: "the record this would write is one the project refuses", Problems: problems,
	}}
	served := New(rec.writing(), loopback(), testBundle())

	response := post(t, served, "/api/project/records",
		`{"kind":"decision","title":"One","goals":["logistics"]}`, nil)

	testutil.Expect(t, "status", response.Code, http.StatusBadRequest)
	testutil.Expect(t, "the problems", refusalBody(t, "the refusal", response).Problems, problems)
}

// A body that is not the object the route takes is a bad request, and the
// writer is never reached.
func TestABadBodyIsRefusedBeforeTheWriter(t *testing.T) {
	t.Parallel()

	for _, bad := range []struct{ what, body string }{
		{"not JSON", "kind=decision"},
		{"not an object", `["decision"]`},
		{"a field the route does not take", `{"kind":"decision","path":"../../etc/passwd"}`},
		{"a second value after the first", `{"kind":"decision"} {"kind":"design"}`},
		{"nothing at all", ""},
	} {
		t.Run(bad.what, func(t *testing.T) {
			t.Parallel()
			rec := &recorder{}
			served := New(rec.writing(), loopback(), testBundle())

			response := post(t, served, "/api/project/records", bad.body, nil)

			testutil.Expect(t, "status", response.Code, http.StatusBadRequest)
			testutil.Expect(t, "the writer was not reached", rec.reached(), 0)
			testutil.Expect(t, "the body carries a reason", refusalBody(t, "the refusal", response).Error != "", true)
		})
	}
}

// The policy the read routes are held to holds here: a cross-site request, a
// foreign origin, and a host this server was not bound to are each refused
// before the route is reached, and none of them writes.
func TestTheWriteRoutesKeepTheReadRoutesPolicy(t *testing.T) {
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
			rec := &recorder{}
			served := New(rec.writing(), loopback(), testBundle())
			req := httptest.NewRequest(http.MethodPost, "http://example.invalid/api/project/records",
				strings.NewReader(`{"kind":"decision","title":"One binary","goals":["g1-s22"]}`))
			req.Host = refused.host
			for name, value := range refused.headers {
				req.Header.Set(name, value)
			}
			response := httptest.NewRecorder()

			served.ServeHTTP(response, req)

			testutil.Expect(t, "status", response.Code, http.StatusForbidden)
			testutil.Expect(t, "the writer was not reached", rec.reached(), 0)
		})
	}
}

// A write route takes POST and a read route takes GET, and each says which
// verb it takes rather than pretending the resource is not there.
func TestEachRouteNamesTheVerbItTakes(t *testing.T) {
	t.Parallel()

	rec := &recorder{}
	info := rec.writing()
	info.Project = func() (project.Pane, error) { return describedPane(), nil }
	served := New(info, loopback(), testBundle())

	for _, path := range []string{
		"/api/project/records", "/api/project/records/01K7/status", "/api/project/records/01K7/goals",
		"/api/project/questions", "/api/project/questions/Q-1/status",
	} {
		response := request(t, served, http.MethodGet, path, "127.0.0.1:7878", nil)
		testutil.Expect(t, "GET "+path+" status", response.Code, http.StatusMethodNotAllowed)
		testutil.Expect(t, "GET "+path+" allows", response.Header().Get("Allow"), "POST")
	}

	read := post(t, served, "/api/project", `{}`, nil)
	testutil.Expect(t, "POST /api/project status", read.Code, http.StatusMethodNotAllowed)
	testutil.Expect(t, "POST /api/project allows", read.Header().Get("Allow"), "GET, HEAD")
	testutil.Expect(t, "the writer was not reached", rec.reached(), 0)
}

// A path that is not one of the four is no route at all: it falls through to
// the 404 every unserved path under a reserved prefix gets.
func TestAPathThatIsNoWriteRoute(t *testing.T) {
	t.Parallel()

	rec := &recorder{}
	served := New(rec.writing(), loopback(), testBundle())

	for _, path := range []string{
		"/api/project/records/",
		"/api/project/records/01K7",
		"/api/project/records/01K7/status/extra",
		"/api/project/records/a/b/status",
		"/api/project/records//goals",
		"/api/project/records/a/b/goals",
		"/api/project/records/01K7/goals/extra",
		"/api/project/questions//status",
	} {
		response := post(t, served, path, `{"status":"accepted"}`, nil)
		testutil.Expect(t, "POST "+path+" is not a write route", response.Code != http.StatusOK, true)
	}
	testutil.Expect(t, "the writer was not reached", rec.reached(), 0)
}

// An engine built without a writer says so, as every other route says what it
// was built without.
func TestAnEngineWithoutAWriter(t *testing.T) {
	t.Parallel()

	served := New(Info{}, loopback(), testBundle())

	for _, path := range []string{
		"/api/project/records", "/api/project/records/01K7/status", "/api/project/records/01K7/goals",
		"/api/project/questions", "/api/project/questions/Q-1/status",
	} {
		response := post(t, served, path, `{"status":"accepted","kind":"decision","question":"a","add":"g1"}`, nil)
		testutil.Expect(t, path+" status", response.Code, http.StatusInternalServerError)
		testutil.Expect(t, path+" reason", refusalBody(t, path, response).Error,
			"this engine was built without a project writer")
	}
}
