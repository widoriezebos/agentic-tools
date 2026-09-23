package httpd

import (
	"encoding/json"
	"net/http"
)

// GET /api/interface: what this interface is made of, as the server joins it.
//
// The two halves are joined by the composer this server is given rather than
// here, because the same join answers the Project Partner's own tool from a
// different process, and one join means the page and the Partner cannot be
// told two different things about the same build.
const interfacePath = "/api/interface"

func (h *handler) describe(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	if h.info.Interface == nil {
		writeFailure(w, "this engine cannot describe its own interface")
		return
	}
	described, err := h.info.Interface()
	if err != nil {
		writeFailure(w, err.Error())
		return
	}
	_ = json.NewEncoder(w).Encode(described)
}
