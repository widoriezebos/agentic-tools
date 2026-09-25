package httpd

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/backlog"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalbudget"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/rulings"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/act"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/fleet"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/manifest"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/project"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/session"
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
	// Fetch carries this clone's accepted ref forward now, through the
	// freshness loop's own machinery and under the loop's own budget, and
	// returns when that one look has finished. It is what the board's Refresh
	// runs before observing, so that "is this current" is answered about the
	// instant a human asked rather than about the loop's last cadence. A nil
	// Fetch is a build whose Refresh only re-observes, which is what every
	// other read of this resource does anyway.
	Fetch func()
	// Fleet composes the Fleet page from one observation and its board
	// projection, taken by the caller and handed in so that the holders on
	// the page, the titles beside them and the tip they were read at are all
	// of one commit. It reads the presence copy this interface fetched for
	// itself; it starts no fetch of its own. A nil Fleet is a build that
	// cannot answer for the fleet, which the route says and which costs the
	// board and the Overview their holder flags and nothing else.
	Fleet func(snapshot.Observation, backlog.Board, time.Time) (fleet.Page, error)
	// Launch starts one machine of this fleet joining on this host: it
	// judges the request the verb's own way, writes the launch record before
	// anything runs, and spawns `seat launch` detached under the server's
	// ownership gate with a scrubbed environment. It answers with the record
	// as it was written, and everything after it reaches the page through
	// the fleet resource. A launch.Refusal carries the status the route
	// answers with; anything else is a 500. A nil Launch is an engine that
	// cannot launch a machine, which the route says.
	Launch func(signed *session.Session, asked launch.Request) (launch.Record, error)
	// Watch is the bridge between the open notification streams and the
	// presence fetch owner: the streams are the owner's connection signal,
	// and the one `fleet` event rides back to them after every attempt. A nil
	// watch is a build with no presence fetcher, whose pages read on mount
	// and never again.
	Watch *fleet.Watch
	// Project answers what the project's records declare, per request for the
	// same reason.
	Project func() (project.Pane, error)
	// Document answers one document by its checkout-relative id. A
	// project.ErrNotFound is the route's 404; anything else is a 500.
	Document func(id string) (project.Document, error)
	// The Project section's five writes, each one the human editing their own
	// checkout on loopback. A project.Refusal carries the status the route
	// answers with; anything else is a 500 carrying its reason. A nil field is
	// an engine that cannot write, which the route says.
	CreateRecord      func(project.NewRecord) (project.Written, error)
	SetStatus         func(id, status string) (project.Written, error)
	AskQuestion       func(project.NewQuestion) (project.Asked, error)
	SetQuestionStatus func(id, status string) (project.Asked, error)
	// AddRecordGoal names one more ledger goal on one record's head, and
	// answers the document as it now reads from disk. It is the machine
	// writing the association a human would otherwise type: a goal opened
	// from a design's page is named on that design by its own id.
	AddRecordGoal func(id, goal string) (project.Document, error)
	// SetRecordGoals says what one record is about, as the whole list, and
	// answers the document as it now reads from disk. It is the human's own
	// statement rather than the machine's: made from the record's page,
	// through the picker that offers the goals the ledger carries, and an
	// empty list is the record saying it is about the project as a whole.
	SetRecordGoals func(id string, goals []string) (project.Document, error)
	// EditDocument saves one document by the id the read route serves it
	// under, against the revision the caller was given. A
	// project.ErrNotFound is the read route's own 404; a project.Refusal
	// carries the status it deserves; anything else is a 500.
	EditDocument func(id, source, revision string) (project.Document, error)
	// PreviewDocument renders what is being typed, through the same parser
	// the read route renders a file with. It opens nothing and writes
	// nothing, which is why it names no document.
	PreviewDocument func(source string) (project.Preview, error)
	// Authority is what this server's one boot-time human-authority
	// observation found. It is a value rather than a function because it
	// cannot change while the server runs: a proof is an observation of an
	// ancestry that existed at boot, and no later request can make one.
	Authority AuthorityInfo
	// Sessions is this server's signed-in browser sessions: the second way a
	// human's acts reach the ledger, through the seat's one-time code rather
	// than through the ancestry of the process that started the server. A nil
	// store is a build that cannot sign anyone in, which the routes say.
	Sessions *session.Store
	// The backlog's six acts and the Decisions queue's two, each one the human
	// working their own backlog
	// in their own checkout: admitting work, withdrawing that admission,
	// placing a goal in a priority band, opening a new goal at intake, and
	// saying which goal waits for which.
	// They publish through the engine in-process under the hand that reached
	// them: the live browser session where the request carries one, and the
	// boot proof otherwise. A nil field is an engine that cannot act, which
	// the route says. An act.Refusal carries the status the route answers
	// with; anything else is a 500 carrying its reason.
	Approve  func(signed *session.Session, id string, budget goalbudget.Budget) error
	Withdraw func(signed *session.Session, id, reason string) error
	// SetPriority places one goal in a band at a one-based position, or
	// appends it there when the position is nil. The engine renumbers the
	// band, so this act is never about one record.
	SetPriority func(signed *session.Session, id string, priority uint8, sequence *uint64) error
	// Open is the human's intake act: one new goal, under origin human.
	Open func(signed *session.Session, opened act.Opened) error
	// Block and Unblock write and remove one edge of the blocked relation.
	// The first argument is always the goal that WAITS, whichever end of the
	// relation the page acted from, so both directions reach one mutation.
	Block   func(signed *session.Session, dependent, blocker string) error
	Unblock func(signed *session.Session, dependent, blocker string) error
	// Park pauses one goal with its reason and Unpark lifts a park. They are
	// the Decisions queue's "Not now" and its undo, admitted from a browser
	// under R-125-m1u; the board offers neither. A nil field is an engine
	// that cannot act, which the route says.
	Park   func(signed *session.Session, id, because string) error
	Unpark func(signed *session.Session, id string) error
	// Edit rewrites the intent, the next step and the labels of a goal nobody
	// has approved yet, from the goal's own page or the board's card menu.
	// Only the fields a human changed are carried, so a terminal edit of an
	// untouched field survives the save; the state the goal must be in is the
	// mutation's own allowlist rather than this server's check.
	Edit func(signed *session.Session, id string, edited act.Edited) error
	// BudgetDefaults is the project's budget law by tier, read per request
	// for the reason the readers are: what the browser prefills from is what
	// the next read of the configuration will say.
	BudgetDefaults func() (map[string]goalbudget.Budget, error)
	// NotificationJournal is the file the steward appends one line to for
	// every notification it attempts to deliver. It is a path rather than a
	// reader because the two routes that serve it read it differently — a
	// page of it, and everything appended after a point — and both readings
	// are the notifications package's. An empty path is an engine with no
	// journal, which those routes say.
	NotificationJournal string
	// Asks answers this seat's open channel questions: what a seat has asked
	// the human and nobody has answered. It is called per request for the
	// reason the readers are, and it is this seat's own files — a fleet
	// where another seat holds a question shows only this seat's, which the
	// page says. A nil Asks is a build with no channel reader, which costs
	// the Decisions page its ask rows and nothing else.
	Asks func() ([]channel.Question, error)
	// Rulings answers the standing rulings register, whole rows and all. It
	// is the same read the steward's sweep makes, through the same package,
	// so the page and the digest cannot disagree about what the register
	// says. A nil Rulings is a build with no register reader, which costs
	// the Decisions page its Rulings tab and nothing else.
	Rulings func() (rulings.Register, error)
	// RegisterPath is where that register is relative to the CHECKOUT, which
	// is not where Rulings read it from: the kit keeps its memory under the
	// installation, and the document reader opens paths against the checkout.
	// The caller knows both roots and derives the one path this payload can
	// carry. An empty path leaves the composer its own default.
	RegisterPath string
	// Visit records that a human is looking at the landing page and answers
	// the window it compares against: the end of their previous visit, or a
	// day back on a first one. It is a function rather than a root because
	// the marker is the overview package's own file and this package never
	// opens it. A nil Visit is a build that keeps no marker, which the route
	// answers as a first visit rather than as a refusal — the marker is
	// preference state, and losing it changes a window and nothing else.
	Visit func(human string, now time.Time) (since time.Time, first bool, err error)
	// VisitDecisions is the same for the Decisions page, under an entry of
	// that page's own. It is a second function rather than a page name on the
	// one above because the two are two markers: a human reads Decisions and
	// Overview on different rhythms, and a page that advanced the other's
	// entry would hide from them what they never saw there. A nil one is a
	// build that keeps no marker, and the route answers it as a first visit.
	VisitDecisions func(human string, now time.Time) (since time.Time, first bool, err error)
	// Now is this server's clock, so that a test can say when a page was
	// composed. A nil Now is time.Now, which is what every run uses.
	Now func() time.Time
	// Partner is the Project Partner's conversation owner, or nil on a seat
	// that admitted no runtime. The three Partner routes and the Partner half
	// of the event stream read it; nothing else does.
	Partner *partner.Service
	// PartnerConfigured says that ui.partner.runtime names a runtime on this
	// seat, whether or not the runtime could be admitted. It is separate from
	// Partner because it decides something else: a seat with a Partner on it
	// no longer admits a cookie-less act, and a Partner that failed to start
	// is still a Partner that a local process could have started.
	PartnerConfigured bool
	// PartnerRefusal is why a configured runtime was not admitted, in the
	// admission's own words, which the Partner routes answer 503 with.
	PartnerRefusal string
	// Interface joins the interface's two halves: the one the bundle carries
	// and the one this engine composes for this seat. It is called per
	// request for the reason the readers are — a setting changed while the
	// server runs is read without a restart — and it is given rather than
	// built here because the Project Partner's own tool server composes the
	// same join in another process, from the same owners. A nil Interface is
	// an engine that cannot describe itself, which the route says.
	Interface func() (manifest.Manifest, error)
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

// The API routes this build reads from. The first two are matched exactly:
// what lies beneath them belongs to no resource, so it is a 404 like any other
// unserved path under a reserved prefix. The third is a prefix, because the
// document's id is the rest of the path; the prefix alone names no document
// and is a 404 too. The four routes this build writes through are in write.go,
// with the policy they share with these.
const (
	workspacePath  = "/api/workspace"
	projectPath    = "/api/project"
	documentPrefix = "/api/documents/"
)

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
	// streamTick and streamHeartbeat are the notification stream's clock,
	// per handler so that a test's faster clock is its own server's alone.
	streamTick      time.Duration
	streamHeartbeat time.Duration
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
	handler := &handler{
		info:         info,
		allowedHosts: allowedHosts(bound),
		dist:         bundle,
		page:         page,
		nonce:        nonce,

		streamTick:      notificationTick,
		streamHeartbeat: notificationHeartbeat,
	}
	// The Partner is told what the landing page shows from this server's own
	// composition of it, which needs the journal and the seat's standing as
	// well as the two readers the Partner was built with.
	if info.Partner != nil {
		info.Partner.SeesOverview(handler.partnerOverview)
	}
	return handler
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
	// A write route takes POST and a read route takes GET and HEAD. The method
	// is judged before anything else, exactly as it was before the writes
	// existed, so a request with the wrong verb learns which verb the resource
	// takes and nothing else about this server.
	route, isWrite := writeRouteOf(r.URL.Path)
	if isWrite && r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !isWrite && r.Method != http.MethodGet && r.Method != http.MethodHead {
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

	if isWrite {
		h.write(w, r, route)
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
	if r.URL.Path == sessionPath {
		h.session(w, r)
		return
	}
	if r.URL.Path == backlogPath {
		h.backlog(w, r)
		return
	}
	if r.URL.Path == notificationsPath {
		h.notifications(w, r)
		return
	}
	// The stream is a read like any other and takes the same checks above;
	// what is different is that it does not end when the response is written.
	if r.URL.Path == notificationsStreamPath {
		h.notificationStream(w, r)
		return
	}
	if r.URL.Path == projectPath {
		h.project(w)
		return
	}
	if r.URL.Path == interfacePath {
		h.describe(w)
		return
	}
	if r.URL.Path == overviewPath {
		h.overview(w, r)
		return
	}
	if r.URL.Path == decisionsPath {
		h.decisions(w, r)
		return
	}
	if r.URL.Path == fleetPath {
		h.fleet(w)
		return
	}
	if r.URL.Path == partnerPath {
		h.partner(w, r)
		return
	}
	if id, beneath := strings.CutPrefix(r.URL.Path, documentPrefix); beneath && id != "" {
		h.document(w, id)
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
		writeFailure(w, "this engine was built without a workspace description")
		return
	}
	described, err := h.info.Describe()
	if err != nil {
		writeFailure(w, err.Error())
		return
	}
	_ = json.NewEncoder(w).Encode(described)
}

// project answers the project's declared memory. A failure is a 500 carrying
// the reason, for the workspace route's reason: the reason is what a human
// acts on.
func (h *handler) project(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	if h.info.Project == nil {
		writeFailure(w, "this engine was built without a project reader")
		return
	}
	pane, err := h.info.Project()
	if err != nil {
		writeFailure(w, err.Error())
		return
	}
	_ = json.NewEncoder(w).Encode(pane)
}

// document answers one document. Every refusal the reader makes is the same
// 404 naming the id, so a caller learns that this checkout serves no document
// at that id and nothing else about the filesystem.
func (h *handler) document(w http.ResponseWriter, id string) {
	w.Header().Set("Content-Type", "application/json")
	if h.info.Document == nil {
		writeFailure(w, "this engine was built without a document reader")
		return
	}
	document, err := h.info.Document(id)
	if err != nil {
		if errors.Is(err, project.ErrNotFound) {
			w.WriteHeader(http.StatusNotFound)
			writeError(w, "no document at "+id)
			return
		}
		writeFailure(w, err.Error())
		return
	}
	_ = json.NewEncoder(w).Encode(document)
}

// writeFailure is the 500 every route shares. writeError, which writes the
// body, lives in backlog.go over an io.Writer, so both spellings of a failing
// route reach one encoder.
func writeFailure(w http.ResponseWriter, reason string) {
	w.WriteHeader(http.StatusInternalServerError)
	writeError(w, reason)
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
