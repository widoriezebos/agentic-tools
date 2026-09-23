package httpd

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/project"
)

// The Project section's write surface: five POST routes, on loopback, in the
// human's own checkout.
//
// Every one of them goes through the same policy the read routes go through —
// the allowed host, the same-site check, the single allowed origin — before it
// is dispatched, and nothing about them relaxes it. What they add is a method:
// a path below is answered for POST and for nothing else, and every other path
// is answered for GET and HEAD and for nothing else, so a request that names
// the wrong verb is told which one the resource takes rather than being told
// the resource is not there.
//
// No route takes a path. A record is created in the home its kind names, under
// a file name this server derives from the title; a status names a record by
// its id; a question names a row by its id; a goal named on a record is a
// ledger id in a body. There is nothing in any body that reaches the
// filesystem as a name. The edit route below is the one that names a document,
// and it names it with the read route's own id, in the read route's own
// prefix: a name it will write to is a name the read serves.
const (
	recordsPath     = "/api/project/records"
	recordsPrefix   = "/api/project/records/"
	questionsPath   = "/api/project/questions"
	questionsPrefix = "/api/project/questions/"
	statusSuffix    = "/status"
	// recordGoalsSuffix is where the machine writes the association a human
	// would otherwise type: a ledger goal named on the record the path names.
	recordGoalsSuffix = "/goals"
	// The document's own two writes, under the prefix it is read from. The
	// preview path is exact and names no document: no id this server serves
	// ends without ".md", so nothing is shadowed by it.
	previewPath = documentPrefix + "preview"
	editSuffix  = "/edit"
)

// maxWriteBody is what a write route reads. The bodies are a title, a handful
// of ids and a sentence; a megabyte of them is not a mistake this server
// carries into memory.
const maxWriteBody = 64 << 10

// maxDocumentBody is what the two document routes read. A document is bounded
// at a mebibyte, and JSON escaping can spend several bytes on one of them, so
// the body is given room for the worst of that and no more. A body past it is
// answered 413, because what is wrong with it is its size.
const maxDocumentBody = 8 << 20

// written names one of the four routes, with the id the path carried where the
// route takes one.
type written struct {
	route string
	id    string
}

// The five routes, by name, and the document's two.
const (
	routeCreateRecord   = "create-record"
	routeRecordStatus   = "record-status"
	routeRecordGoals    = "record-goals"
	routeAskQuestion    = "ask-question"
	routeQuestionStatus = "question-status"
	routeEditDocument   = "edit-document"
	routePreviewSource  = "preview-source"
)

// writeRouteOf reports which write route a path names, if any. The three
// routes beneath a collection carry an id between a prefix and a suffix; an
// empty id names no record and no row, so it is no route and falls through to
// the 404 every unserved path under a reserved prefix gets.
func writeRouteOf(path string) (written, bool) {
	switch path {
	case recordsPath:
		return written{route: routeCreateRecord}, true
	case questionsPath:
		return written{route: routeAskQuestion}, true
	case signInPath:
		return written{route: routeSignIn}, true
	case signOutPath:
		return written{route: routeSignOut}, true
	case previewPath:
		return written{route: routePreviewSource}, true
	}
	if id, ok := idBetween(path, recordsPrefix, statusSuffix); ok {
		return written{route: routeRecordStatus, id: id}, true
	}
	if id, ok := idBetween(path, recordsPrefix, recordGoalsSuffix); ok {
		return written{route: routeRecordGoals, id: id}, true
	}
	if id, ok := idBetween(path, questionsPrefix, statusSuffix); ok {
		return written{route: routeQuestionStatus, id: id}, true
	}
	if id, ok := editID(path); ok {
		return written{route: routeEditDocument, id: id}, true
	}
	// The backlog's four acts are writes under the same policy, and are
	// routed here so that the method, the host, the site and the origin are
	// judged for them exactly as they are for the Project's four.
	return actRouteOf(path)
}

// editID reports which document a path names for editing. The id is the whole
// of the path between the prefix and the suffix, separators included, because
// a document's id is a checkout-relative path; an empty id names no document
// and is no route.
func editID(path string) (string, bool) {
	rest, beneath := strings.CutPrefix(path, documentPrefix)
	if !beneath {
		return "", false
	}
	id, ends := strings.CutSuffix(rest, editSuffix)
	if !ends || id == "" {
		return "", false
	}
	return id, true
}

// idBetween is the one segment a path carries between a collection's prefix
// and one act's suffix. An id that is empty, or that carries a separator and
// so names something deeper than one member, is no route.
func idBetween(path, prefix, suffix string) (string, bool) {
	rest, beneath := strings.CutPrefix(path, prefix)
	if !beneath {
		return "", false
	}
	id, ends := strings.CutSuffix(rest, suffix)
	if !ends || id == "" || strings.Contains(id, "/") {
		return "", false
	}
	return id, true
}

// The bodies the five routes read. A field a route does not take is not read,
// and a body that is not an object at all is a bad request.
type recordBody struct {
	Kind    string   `json:"kind"`
	Title   string   `json:"title"`
	Goals   []string `json:"goals"`
	Affects []string `json:"affects"`
	Cites   []string `json:"cites"`
}

type statusBody struct {
	Status string `json:"status"`
}

// recordGoalsBody names one ledger goal to add to the record the path names.
// It is one goal rather than a list because it is one act: a goal was just
// opened, and this is where it is written down.
type recordGoalsBody struct {
	Add string `json:"add"`
}

type questionBody struct {
	Question string   `json:"question"`
	Goals    []string `json:"goals"`
}

// The document's two bodies. An edit carries the whole file as it was typed
// and the revision it was opened at; a preview carries only the text, because
// it is rendered and never written.
type editBody struct {
	Source   string `json:"source"`
	Revision string `json:"revision"`
}

type previewBody struct {
	Source string `json:"source"`
}

func (h *handler) write(w http.ResponseWriter, r *http.Request, route written) {
	w.Header().Set("Content-Type", "application/json")
	switch route.route {
	case routeCreateRecord:
		h.createRecord(w, r)
	case routeRecordStatus:
		h.recordStatus(w, r, route.id)
	case routeRecordGoals:
		h.recordGoals(w, r, route.id)
	case routeAskQuestion:
		h.askQuestion(w, r)
	case routeQuestionStatus:
		h.questionStatus(w, r, route.id)
	case routeEditDocument:
		h.editDocument(w, r, route.id)
	case routePreviewSource:
		h.previewSource(w, r)
	case routeApprove:
		h.approveGoal(w, r, route.id)
	case routeWithdraw:
		h.withdrawGoal(w, r, route.id)
	case routePriority:
		h.setGoalPriority(w, r, route.id)
	case routeOpen:
		h.openGoal(w, r)
	case routeSignIn:
		h.signIn(w, r)
	case routeSignOut:
		h.signOut(w, r)
	}
}

func (h *handler) createRecord(w http.ResponseWriter, r *http.Request) {
	if h.info.CreateRecord == nil {
		writeFailure(w, "this engine was built without a project writer")
		return
	}
	var body recordBody
	if !decode(w, r, &body) {
		return
	}
	answer(w, func() (any, error) {
		return h.info.CreateRecord(project.NewRecord{
			Kind: body.Kind, Title: body.Title, Goals: body.Goals, Affects: body.Affects, Cites: body.Cites,
		})
	})
}

func (h *handler) recordStatus(w http.ResponseWriter, r *http.Request, id string) {
	if h.info.SetStatus == nil {
		writeFailure(w, "this engine was built without a project writer")
		return
	}
	var body statusBody
	if !decode(w, r, &body) {
		return
	}
	answer(w, func() (any, error) { return h.info.SetStatus(id, body.Status) })
}

// recordGoals names one more ledger goal on a record, and answers the document
// as it now reads from disk, so the page re-renders from the file rather than
// from the request. The id in the path is a record's, never a file's: what is
// rewritten is the head of the record that declares it.
func (h *handler) recordGoals(w http.ResponseWriter, r *http.Request, id string) {
	if h.info.AddRecordGoal == nil {
		writeFailure(w, "this engine was built without a project writer")
		return
	}
	var body recordGoalsBody
	if !decode(w, r, &body) {
		return
	}
	answer(w, func() (any, error) { return h.info.AddRecordGoal(id, body.Add) })
}

func (h *handler) askQuestion(w http.ResponseWriter, r *http.Request) {
	if h.info.AskQuestion == nil {
		writeFailure(w, "this engine was built without a project writer")
		return
	}
	var body questionBody
	if !decode(w, r, &body) {
		return
	}
	answer(w, func() (any, error) {
		return h.info.AskQuestion(project.NewQuestion{Question: body.Question, Goals: body.Goals})
	})
}

func (h *handler) questionStatus(w http.ResponseWriter, r *http.Request, id string) {
	if h.info.SetQuestionStatus == nil {
		writeFailure(w, "this engine was built without a project writer")
		return
	}
	var body statusBody
	if !decode(w, r, &body) {
		return
	}
	answer(w, func() (any, error) { return h.info.SetQuestionStatus(id, body.Status) })
}

// editDocument saves one document, and answers the document as it now reads
// from disk so the page re-renders from the file rather than from the request.
func (h *handler) editDocument(w http.ResponseWriter, r *http.Request, id string) {
	if h.info.EditDocument == nil {
		writeFailure(w, "this engine was built without a project writer")
		return
	}
	var body editBody
	if !decodeDocument(w, r, &body) {
		return
	}
	answerDocument(w, id, func() (any, error) { return h.info.EditDocument(id, body.Source, body.Revision) })
}

// previewSource renders what is being typed. It names no document and writes
// nothing, so there is no id here and no refusal but the request's own.
func (h *handler) previewSource(w http.ResponseWriter, r *http.Request) {
	if h.info.PreviewDocument == nil {
		writeFailure(w, "this engine was built without a document reader")
		return
	}
	var body previewBody
	if !decodeDocument(w, r, &body) {
		return
	}
	answer(w, func() (any, error) { return h.info.PreviewDocument(body.Source) })
}

// decode reads one JSON object from a bounded body, and reports whether the
// route may go on. A body that is not one object, or that carries a second
// value after it, is a bad request and is answered here.
func decode(w http.ResponseWriter, r *http.Request, into any) bool {
	reader := json.NewDecoder(io.LimitReader(r.Body, maxWriteBody+1))
	reader.DisallowUnknownFields()
	if err := reader.Decode(into); err != nil {
		writeRefusal(w, http.StatusBadRequest, "the request body is not the JSON object this route takes: "+err.Error(), nil)
		return false
	}
	if reader.More() {
		writeRefusal(w, http.StatusBadRequest, "the request body carries more than one JSON value", nil)
		return false
	}
	return true
}

// decodeDocument is decode over a body that may carry a whole document.
//
// The bytes are taken first and counted, because a body past the bound is
// refused for its size and not for its shape: a caller that sent a megabyte
// too much is told so, rather than being told its JSON did not parse.
func decodeDocument(w http.ResponseWriter, r *http.Request, into any) bool {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxDocumentBody+1))
	if err != nil {
		writeRefusal(w, http.StatusBadRequest, "the request body could not be read: "+err.Error(), nil)
		return false
	}
	if len(body) > maxDocumentBody {
		writeRefusal(w, http.StatusRequestEntityTooLarge, "the request body is larger than this route carries", nil)
		return false
	}
	reader := json.NewDecoder(bytes.NewReader(body))
	reader.DisallowUnknownFields()
	if err := reader.Decode(into); err != nil {
		writeRefusal(w, http.StatusBadRequest, "the request body is not the JSON object this route takes: "+err.Error(), nil)
		return false
	}
	if reader.More() {
		writeRefusal(w, http.StatusBadRequest, "the request body carries more than one JSON value", nil)
		return false
	}
	return true
}

// answer runs one write and says what happened. A refusal carries the status
// it deserves and, where the project itself refused what would have been
// written, the problems the check verb would have printed; anything else is a
// 500 carrying its reason, as every other route's failure is.
func answer(w http.ResponseWriter, write func() (any, error)) {
	payload, err := write()
	if err == nil {
		_ = json.NewEncoder(w).Encode(payload)
		return
	}
	var refusal *project.Refusal
	if errors.As(err, &refusal) {
		writeRefusal(w, statusOf(refusal.Kind), refusal.Message, refusal.Problems)
		return
	}
	writeFailure(w, err.Error())
}

// answerDocument is answer with the document route's own 404 in front of it,
// so an id this checkout does not serve is refused by the write exactly as the
// read refuses it: the same sentence, naming the id and nothing else.
func answerDocument(w http.ResponseWriter, id string, write func() (any, error)) {
	payload, err := write()
	if err == nil {
		_ = json.NewEncoder(w).Encode(payload)
		return
	}
	if errors.Is(err, project.ErrNotFound) {
		w.WriteHeader(http.StatusNotFound)
		writeError(w, "no document at "+id)
		return
	}
	var refusal *project.Refusal
	if errors.As(err, &refusal) {
		writeRefusal(w, statusOf(refusal.Kind), refusal.Message, refusal.Problems)
		return
	}
	writeFailure(w, err.Error())
}

func statusOf(kind project.RefusalKind) int {
	switch kind {
	case project.RefusalAbsent:
		return http.StatusNotFound
	case project.RefusalExists, project.RefusalStale:
		return http.StatusConflict
	case project.RefusalTooLarge:
		return http.StatusRequestEntityTooLarge
	case project.RefusalProject:
		// The request was well formed and the project refuses what it says:
		// unprocessable, with the check verb's problems in the body.
		return http.StatusUnprocessableEntity
	default:
		return http.StatusBadRequest
	}
}

// writeRefusal is the body every refusal shares: the reason, and the problems
// where there are any. A caller that reads only `error` reads every refusal.
func writeRefusal(w http.ResponseWriter, status int, reason string, problems []project.Problem) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(struct {
		Error    string            `json:"error"`
		Problems []project.Problem `json:"problems,omitempty"`
	}{Error: reason, Problems: problems})
}
