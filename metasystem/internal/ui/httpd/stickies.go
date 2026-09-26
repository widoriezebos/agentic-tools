package httpd

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/stickies"
)

// The notepad: one read and three writes, all over one human's own stickies.
//
// It is the one address in this server that is both read and written. A
// notepad is one list, every act on it changes that list, and every act
// answers the whole of it — so a collection read at one address and added to
// at another would be two names for one thing and a page free to believe two
// different orders. The method says which act a request is; the policy every
// other route takes is unchanged.
//
// Nothing here asks who the caller is beyond what the server already knows.
// The human a request acts as is the signed-in session's, else the boot
// proof's, else the handle this seat was configured with — session.go's own
// answer — and a server that knows none of the three still keeps a notepad,
// under the empty handle, which is the seat's own. The page says so.
const (
	stickiesPath   = "/api/stickies"
	stickiesPrefix = "/api/stickies/"
	// removeSuffix is the one act that is not a field of a sticky, so it is
	// the one that needs a path of its own: removing is not editing.
	removeSuffix = "/remove"
)

// The four routes, by name. The first is the read; the other three are the
// acts, and the describe table carries all four.
const (
	routeStickies     = "stickies"
	routeAddSticky    = "add-sticky"
	routeEditSticky   = "edit-sticky"
	routeRemoveSticky = "remove-sticky"
)

// stickyRouteOf reports which act beneath the collection a path names. The
// collection itself is not here: it is both read and written, and which it is
// depends on the method, which ServeHTTP decides.
func stickyRouteOf(path string) (written, bool) {
	if id, ok := idBetween(path, stickiesPrefix, removeSuffix); ok {
		return written{route: routeRemoveSticky, id: id}, true
	}
	rest, beneath := strings.CutPrefix(path, stickiesPrefix)
	if beneath && rest != "" && !strings.Contains(rest, "/") {
		return written{route: routeEditSticky, id: rest}, true
	}
	return written{}, false
}

// The two bodies. A field a route does not take is not read, and a field that
// is absent is a field nobody is changing: the three pointers on an edit are
// how "leave the text alone" and "make the text empty" stay two requests.
type stickyBody struct {
	Text  string           `json:"text"`
	About []stickies.About `json:"about"`
}

type stickyEditBody struct {
	Text  *string           `json:"text"`
	About *[]stickies.About `json:"about"`
	Done  *bool             `json:"done"`
}

// stickies answers this human's notepad, whole.
func (h *handler) stickies(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if h.info.Stickies == nil {
		writeFailure(w, "this engine was built without a notepad")
		return
	}
	answerStickies(w, func() (stickies.List, error) { return h.info.Stickies.List(h.humanOf(r)) })
}

// addSticky writes one and answers the whole list.
func (h *handler) addSticky(w http.ResponseWriter, r *http.Request) {
	if h.info.Stickies == nil {
		writeFailure(w, "this engine was built without a notepad")
		return
	}
	var body stickyBody
	if !decode(w, r, &body) {
		return
	}
	human := h.humanOf(r)
	answerStickies(w, func() (stickies.List, error) {
		return h.info.Stickies.Add(human, body.Text, body.About)
	})
}

// editSticky changes one and answers the whole list.
func (h *handler) editSticky(w http.ResponseWriter, r *http.Request, id string) {
	if h.info.Stickies == nil {
		writeFailure(w, "this engine was built without a notepad")
		return
	}
	var body stickyEditBody
	if !decode(w, r, &body) {
		return
	}
	human := h.humanOf(r)
	answerStickies(w, func() (stickies.List, error) {
		return h.info.Stickies.Edit(human, id, body.Text, body.About, body.Done)
	})
}

// removeSticky drops one and answers the whole list.
func (h *handler) removeSticky(w http.ResponseWriter, r *http.Request, id string) {
	if h.info.Stickies == nil {
		writeFailure(w, "this engine was built without a notepad")
		return
	}
	// The body is read and must be the empty object this route takes, so a
	// request carrying a sticky's fields learns that removing takes none
	// rather than having them silently ignored.
	var body struct{}
	if !decode(w, r, &body) {
		return
	}
	human := h.humanOf(r)
	answerStickies(w, func() (stickies.List, error) { return h.info.Stickies.Remove(human, id) })
}

// humanOf is whose notepad this request is about: the human this server is
// acting as, which is session.go's own answer and not a second one.
func (h *handler) humanOf(r *http.Request) string { return h.state(r).Human }

// answerStickies runs one act and says what happened. A refusal carries the
// status its shape deserves and the sentence a human reads; anything else is a
// 500 carrying its reason, as every other route's failure is.
func answerStickies(w http.ResponseWriter, act func() (stickies.List, error)) {
	list, err := act()
	if err == nil {
		_ = json.NewEncoder(w).Encode(list)
		return
	}
	var refusal *stickies.Refusal
	if errors.As(err, &refusal) {
		writeRefusal(w, stickyStatusOf(refusal.Kind), refusal.Message, nil)
		return
	}
	writeFailure(w, err.Error())
}

// stickyStatusOf says what a human can do about a refusal: 404 for a sticky
// that is not theirs, 422 for a well-formed act past one of the notepad's two
// bounds, and 400 for a request that was wrong.
func stickyStatusOf(kind stickies.RefusalKind) int {
	switch kind {
	case stickies.RefusalAbsent:
		return http.StatusNotFound
	case stickies.RefusalBounds:
		return http.StatusUnprocessableEntity
	default:
		return http.StatusBadRequest
	}
}
