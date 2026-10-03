package httpd

// GET /api/fleet/proof-logs/<attempt>: one landing proof's log, whole, as
// text (fleet-panel-ux step 2, 2a.3). The page links it from a red proof in
// Needs you and from the lane's Details, and it opens in a tab of its own.
//
// The client never names a path. The attempt is looked up in this computer's
// lane records (results.jsonl, then running.json) and the log that record
// names is served only when it lies directly inside the lane's proofs folder
// (plain.ProofLog, behind BoardSource.ProofLog). Anything else is a 404 in
// words. It is a read on the terms /api/board is read on: GET and HEAD, the
// allowed host, the same site and one origin, which ServeHTTP judges before
// it gets here; the board already names the log's path, the commit and the
// reason, so no sign-in is asked.

import (
	"errors"
	"io"
	"io/fs"
	"net/http"
	"os"
	"strconv"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

// proofLogPrefix is the proof logs' address; the attempt is the rest.
const proofLogPrefix = "/api/fleet/proof-logs/"

func (h *handler) proofLog(w http.ResponseWriter, r *http.Request, attempt string) {
	source := h.info.Board
	if source == nil || source.ProofLog == nil {
		http.Error(w, "this engine was built without a landing-log reader", http.StatusInternalServerError)
		return
	}
	if !board.SafeName(attempt) {
		http.Error(w, "no landing log is served for "+strconv.Quote(attempt)+": an attempt is one word of letters, digits, '.', '-' and '_'", http.StatusNotFound)
		return
	}
	path, err := source.ProofLog(attempt)
	if errors.Is(err, plain.ErrNoProofLog) {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "the log of attempt "+attempt+" can't be read: "+err.Error(), http.StatusInternalServerError)
		return
	}
	file, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		http.Error(w, "the log of attempt "+attempt+" is gone: "+path, http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "the log of attempt "+attempt+" can't be read: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		http.Error(w, "the log of attempt "+attempt+" can't be read: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if !info.Mode().IsRegular() {
		http.Error(w, "the log of attempt "+attempt+" is not a file: "+path, http.StatusNotFound)
		return
	}
	// A running proof's log grows while it is read: it is served as far as
	// it was written when it was opened.
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
	if r.Method == http.MethodHead {
		return
	}
	_, _ = io.CopyN(w, file, info.Size())
}
