package httpd

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
)

type Info struct {
	Checkout         string
	StartedAt        string
	EngineBuild      string
	ExecutableDigest string
}

type handler struct {
	info         Info
	allowedHosts map[string]struct{}
}

func New(info Info, bound net.Addr) http.Handler {
	return &handler{
		info:         info,
		allowedHosts: allowedHosts(bound),
	}
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	setResponseHeaders(w.Header())
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.hostAllowed(r.Host) {
		http.Error(w, "request host is not allowed", http.StatusForbidden)
		return
	}
	if !fetchSiteAllowed(r) {
		http.Error(w, "cross-site request is not allowed", http.StatusForbidden)
		return
	}
	// A request carrying more than one Origin is malformed; no browser sends
	// that, and there is no single origin to judge, so it fails closed.
	if origin, present, single := originHeader(r.Header); present && (!single || !h.originAllowed(origin)) {
		http.Error(w, "request origin is not allowed", http.StatusForbidden)
		return
	}

	switch r.URL.Path {
	case "/-/health":
		h.health(w)
	case "/":
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("MetaSystem interface: no views are installed in this build yet."))
	default:
		http.NotFound(w, r)
	}
}

func (h *handler) health(w http.ResponseWriter) {
	payload := struct {
		Status           string `json:"status"`
		Checkout         string `json:"checkout"`
		StartedAt        string `json:"startedAt"`
		EngineBuild      string `json:"engineBuild"`
		ExecutableDigest string `json:"executableDigest"`
	}{
		Status:           "ok",
		Checkout:         h.info.Checkout,
		StartedAt:        h.info.StartedAt,
		EngineBuild:      h.info.EngineBuild,
		ExecutableDigest: h.info.ExecutableDigest,
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}

func (h *handler) hostAllowed(host string) bool {
	_, ok := h.allowedHosts[strings.ToLower(host)]
	return ok
}

func (h *handler) originAllowed(origin string) bool {
	const prefix = "http://"
	if !strings.HasPrefix(origin, prefix) {
		return false
	}
	return h.hostAllowed(origin[len(prefix):])
}

func allowedHosts(bound net.Addr) map[string]struct{} {
	hosts := make(map[string]struct{})
	if bound == nil {
		return hosts
	}
	host, port, err := net.SplitHostPort(bound.String())
	if err != nil {
		return hosts
	}
	add := func(host string) {
		hosts[strings.ToLower(host)] = struct{}{}
	}
	add(net.JoinHostPort(host, port))
	add(net.JoinHostPort("localhost", port))
	if port == "80" {
		if strings.Contains(host, ":") {
			add("[" + host + "]")
		} else {
			add(host)
		}
		add("localhost")
	}
	return hosts
}

func fetchSiteAllowed(r *http.Request) bool {
	site, present := headerValue(r.Header, "Sec-Fetch-Site")
	if !present || site == "same-origin" || site == "none" {
		return true
	}
	return r.Method == http.MethodGet &&
		r.Header.Get("Sec-Fetch-Mode") == "navigate" &&
		r.Header.Get("Sec-Fetch-Dest") == "document"
}

// originHeader reports the request's origin, whether one was sent at all, and
// whether exactly one was sent.
func originHeader(header http.Header) (value string, present, single bool) {
	values, present := header[http.CanonicalHeaderKey("Origin")]
	if !present || len(values) == 0 {
		return "", present, false
	}
	return values[0], true, len(values) == 1
}

func headerValue(header http.Header, name string) (string, bool) {
	values, present := header[http.CanonicalHeaderKey(name)]
	if !present || len(values) == 0 {
		return "", present
	}
	return values[0], true
}

func setResponseHeaders(header http.Header) {
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("X-Frame-Options", "DENY")
	header.Set("Referrer-Policy", "same-origin")
	header.Set("Cross-Origin-Resource-Policy", "same-origin")
	header.Set("Cache-Control", "no-store")
}
