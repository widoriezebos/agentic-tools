package httpd

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/project"
)

// The Project section's write surface: four POST routes, on loopback, in the
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
// its id; a question names a row by its id. There is nothing in any body that
// reaches the filesystem as a name.
const (
	recordsPath     = "/api/project/records"
	recordsPrefix   = "/api/project/records/"
	questionsPath   = "/api/project/questions"
	questionsPrefix = "/api/project/questions/"
	statusSuffix    = "/status"
)

// maxWriteBody is what a write route reads. The bodies are a title, a handful
// of ids and a sentence; a megabyte of them is not a mistake this server
// carries into memory.
const maxWriteBody = 64 << 10

// written names one of the four routes, with the id the path carried where the
// route takes one.
type written struct {
	route string
	id    string
}

// The four routes, by name.
const (
	routeCreateRecord   = "create-record"
	routeRecordStatus   = "record-status"
	routeAskQuestion    = "ask-question"
	routeQuestionStatus = "question-status"
)

// writeRouteOf reports which write route a path names, if any. The two status
// routes carry an id between a prefix and a suffix; an empty id names no
// record and no row, so it is no route and falls through to the 404 every
// unserved path under a reserved prefix gets.
func writeRouteOf(path string) (written, bool) {
	switch path {
	case recordsPath:
		return written{route: routeCreateRecord}, true
	case questionsPath:
		return written{route: routeAskQuestion}, true
	}
	if id, ok := statusID(path, recordsPrefix); ok {
		return written{route: routeRecordStatus, id: id}, true
	}
	if id, ok := statusID(path, questionsPrefix); ok {
		return written{route: routeQuestionStatus, id: id}, true
	}
	return written{}, false
}

func statusID(path, prefix string) (string, bool) {
	rest, beneath := strings.CutPrefix(path, prefix)
	if !beneath {
		return "", false
	}
	id, ends := strings.CutSuffix(rest, statusSuffix)
	if !ends || id == "" || strings.Contains(id, "/") {
		return "", false
	}
	return id, true
}

// The bodies the four routes read. A field a route does not take is not read,
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

type questionBody struct {
	Question string   `json:"question"`
	Goals    []string `json:"goals"`
}

func (h *handler) write(w http.ResponseWriter, r *http.Request, route written) {
	w.Header().Set("Content-Type", "application/json")
	switch route.route {
	case routeCreateRecord:
		h.createRecord(w, r)
	case routeRecordStatus:
		h.recordStatus(w, r, route.id)
	case routeAskQuestion:
		h.askQuestion(w, r)
	case routeQuestionStatus:
		h.questionStatus(w, r, route.id)
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

func statusOf(kind project.RefusalKind) int {
	switch kind {
	case project.RefusalAbsent:
		return http.StatusNotFound
	case project.RefusalExists:
		return http.StatusConflict
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
