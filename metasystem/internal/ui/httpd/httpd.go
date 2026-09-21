package httpd

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"io/fs"
	"net"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/snapshot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/web"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/workspace"
)

type Info struct {
	Checkout         string
	StartedAt        string
	EngineBuild      string
	ExecutableDigest string
	BundleDigest     string
	// Describe answers the workspace resource. It is called once per request
	// rather than read at construction, so an installation whose adoption line
	// is filled in while the server runs is read without a restart. A nil
	// Describe is an engine that cannot answer, which the route says.
	Describe func() (workspace.Workspace, error)
	// Observe answers what this workspace's accepted ledger says. It is called
	// once per request and reads the accepted ref as it stands: it moves no
	// ref and starts no fetch. A nil Observe is an engine that cannot answer,
	// which the route says.
	Observe func() snapshot.Observation
}

// absentBundleStatement is what a page request gets from an engine built
// without a bundle. It says what is missing and how to get it back rather than
// serving an empty page or a stack trace.
const absentBundleStatement = "MetaSystem interface: this executable was built without the interface bundle. Rebuild it from a checkout that contains internal/ui/web/bundle/dist, then run: metasystem ui restart"

// reservedPrefixes never answer with the page, exactly or with anything
// beneath them: /- belongs to the server, /api to the API, /assets to the
// bundle's own files. What is neither health nor a served file under them is a
// 404, so a mistyped asset or endpoint says so instead of returning the page
// with status 200 and leaving the caller to parse HTML as JSON.
var reservedPrefixes = []string{"/-", "/api", "/assets"}

// workspacePath is the one API route this build answers, matched exactly: what
// lies beneath it belongs to no resource, so it is a 404 like any other
// unserved path under a reserved prefix.
const workspacePath = "/api/workspace"

// The policy allows nothing by default and no source outside this origin: no
// unsafe-inline, no data:, no host source, no report-uri, no Trusted Types. A
// page response adds a nonce for the one inline <style> the dialog's scroll
// lock appends; every other response carries the same string without it.
const (
	policyHead = "default-src 'none'; script-src 'self'; style-src 'self'"
	policyTail = "; img-src 'self'; font-src 'self'; connect-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'"
)

type handler struct {
	info         Info
	allowedHosts map[string]struct{}
	dist         fs.FS
	page         [][]byte
	nonce        func() string
}

func New(info Info, bound net.Addr, bundle fs.FS) http.Handler {
	return newHandler(info, bound, bundle, randomNonce)
}

// newHandler takes the nonce generator so a test can assert what reaches the
// header and the body; every other caller gets the random one.
func newHandler(info Info, bound net.Addr, bundle fs.FS, nonce func() string) *handler {
	page := readPage(bundle)
	if page == nil {
		// A bundle whose index.html cannot be read is no bundle: it serves the
		// same statement and the same 404s as a build without one, rather than
		// answering for some paths and not others.
		bundle = nil
	}
	return &handler{
		info:         info,
		allowedHosts: allowedHosts(bound),
		dist:         bundle,
		page:         page,
		nonce:        nonce,
	}
}

// readPage reads index.html once, at construction, and splits it on the nonce
// placeholder, so a page response writes the parts around a fresh nonce and
// opens nothing. nil means this build has no page to serve.
func readPage(bundle fs.FS) [][]byte {
	if bundle == nil {
		return nil
	}
	index, err := fs.ReadFile(bundle, "index.html")
	if err != nil {
		return nil
	}
	return bytes.Split(index, []byte(web.NoncePlaceholder))
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

	if r.URL.Path == "/-/health" {
		h.health(w)
		return
	}
	if r.URL.Path == workspacePath {
		h.workspace(w)
		return
	}
	if r.URL.Path == backlogPath {
		h.backlog(w)
		return
	}
	h.route(w, r)
}

func (h *handler) route(w http.ResponseWriter, r *http.Request) {
	requested := path.Clean(r.URL.Path)
	if !strings.HasPrefix(requested, "/") {
		http.NotFound(w, r)
		return
	}
	if h.serveFile(w, requested) {
		return
	}
	if reserved(requested) {
		http.NotFound(w, r)
		return
	}
	// Any other path reloads to the page, so the application owns its routes
	// and a deep link survives a refresh; a client that did not ask for HTML
	// gets the honest 404 instead.
	if requested == "/" || web.AcceptsHTML(r.Header.Get("Accept")) {
		h.servePage(w)
		return
	}
	http.NotFound(w, r)
}

// serveFile answers with one file from the bundle and reports whether it did.
func (h *handler) serveFile(w http.ResponseWriter, requested string) bool {
	if h.dist == nil {
		return false
	}
	name := strings.TrimPrefix(requested, "/")
	// index.html is reached only as the page, so there is one way to get it and
	// it always carries a nonce.
	if !fs.ValidPath(name) || name == "." || name == "index.html" {
		return false
	}
	contentType, known := web.ContentType(name)
	if !known {
		return false
	}
	file, err := h.dist.Open(name)
	if err != nil {
		return false
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
	_, _ = io.Copy(w, file)
	return true
}

func (h *handler) servePage(w http.ResponseWriter) {
	if h.page == nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = io.WriteString(w, absentBundleStatement)
		return
	}
	nonce := h.nonce()
	w.Header().Set("Content-Security-Policy", policy(nonce))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	for index, part := range h.page {
		if index > 0 {
			_, _ = io.WriteString(w, nonce)
		}
		_, _ = w.Write(part)
	}
}

// workspace answers what this workspace is. A failure is a 500 carrying the
// reason, not an empty body: the page renders its sections either way and says
// that the workspace is unknown, and the reason is what a human acts on.
func (h *handler) workspace(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	if h.info.Describe == nil {
		writeWorkspaceFailure(w, "this engine was built without a workspace description")
		return
	}
	described, err := h.info.Describe()
	if err != nil {
		writeWorkspaceFailure(w, err.Error())
		return
	}
	_ = json.NewEncoder(w).Encode(described)
}

func writeWorkspaceFailure(w http.ResponseWriter, reason string) {
	w.WriteHeader(http.StatusInternalServerError)
	_ = json.NewEncoder(w).Encode(struct {
		Error string `json:"error"`
	}{Error: reason})
}

func (h *handler) health(w http.ResponseWriter) {
	payload := struct {
		Status           string `json:"status"`
		Checkout         string `json:"checkout"`
		StartedAt        string `json:"startedAt"`
		EngineBuild      string `json:"engineBuild"`
		ExecutableDigest string `json:"executableDigest"`
		BundleDigest     string `json:"bundleDigest"`
	}{
		Status:           "ok",
		Checkout:         h.info.Checkout,
		StartedAt:        h.info.StartedAt,
		EngineBuild:      h.info.EngineBuild,
		ExecutableDigest: h.info.ExecutableDigest,
		BundleDigest:     h.info.BundleDigest,
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}

func reserved(requested string) bool {
	for _, prefix := range reservedPrefixes {
		if requested == prefix || strings.HasPrefix(requested, prefix+"/") {
			return true
		}
	}
	return false
}

func policy(nonce string) string {
	if nonce == "" {
		return policyHead + policyTail
	}
	return policyHead + " 'nonce-" + nonce + "'" + policyTail
}

// randomNonce is 16 bytes from crypto/rand in base64 without padding: 22
// characters, a valid CSP base64-value, fresh for every page response, so a
// nonce read from one response allows nothing in the next. crypto/rand.Read
// fills the slice entirely and never reports an error.
func randomNonce() string {
	var value [16]byte
	_, _ = rand.Read(value[:])
	return base64.RawStdEncoding.EncodeToString(value[:])
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
	header.Set("Content-Security-Policy", policy(""))
}
