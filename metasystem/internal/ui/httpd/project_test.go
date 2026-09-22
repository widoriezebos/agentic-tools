package httpd

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/markdown"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/project"
)

// errFailed is a reader that cannot answer, which is a 500 carrying its reason.
var errFailed = errors.New("the checkout could not be opened")

func loopback() *net.TCPAddr { return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 7878} }

func describedPane() project.Pane {
	return project.Pane{
		SchemaVersion: project.SchemaVersion,
		ReadAt:        "2026-09-21T10:11:12Z",
		Areas:         []project.Area{{Slug: "interface", Name: "The browser workspace"}},
		Records: []project.Record{{
			Kind: "design", ID: "01K5", Status: "accepted", Areas: []string{"interface"},
			Title: "The pane", Path: "plans/designs/pane.md", Home: "plans/designs",
		}},
		Intent:    project.Book{Chapters: []project.Chapter{}},
		Doctrine:  project.Book{Chapters: []project.Chapter{}},
		Questions: []project.Question{},
		Problems:  []project.Problem{},
		Documents: []project.File{{Path: "docs/a.md", Title: "A"}},
	}
}

func describedDocument() project.Document {
	return project.Document{
		Kind: "document", ID: "docs/a.md", Title: "A", Revision: "blob:0123456789abcdef",
		Owner: "app-owned", Path: "/work/repository/docs/a.md", Bytes: 4,
		ModifiedAt: "2026-09-21T09:00:00Z", ReadAt: "2026-09-21T10:11:12Z", State: "readable",
		Headings: []markdown.Heading{{Level: 1, ID: "a", Text: "A"}},
		Blocks:   []markdown.Block{{Type: "heading", Level: 1, ID: "a"}},
	}
}

// The pane is answered per request, so a record written while the server runs
// is read without a restart.
func TestProjectPayload(t *testing.T) {
	t.Parallel()

	calls := 0
	info := Info{Project: func() (project.Pane, error) {
		calls++
		return describedPane(), nil
	}}
	served := New(info, loopback(), testBundle())

	response := request(t, served, http.MethodGet, "/api/project", "127.0.0.1:7878", nil)

	testutil.Require(t, "status", response.Code, http.StatusOK)
	testutil.Expect(t, "content type", response.Header().Get("Content-Type"), "application/json")
	var payload project.Pane
	testutil.Require(t, "decode the response", json.Unmarshal(response.Body.Bytes(), &payload), nil)
	testutil.Expect(t, "payload", payload, describedPane())

	second := request(t, served, http.MethodGet, "/api/project", "127.0.0.1:7878", nil)

	testutil.Expect(t, "second status", second.Code, http.StatusOK)
	testutil.Expect(t, "reads", calls, 2)
}

func TestDocumentPayload(t *testing.T) {
	t.Parallel()

	asked := ""
	info := Info{Document: func(id string) (project.Document, error) {
		asked = id
		return describedDocument(), nil
	}}
	served := New(info, loopback(), testBundle())

	response := request(t, served, http.MethodGet, "/api/documents/docs/a.md", "127.0.0.1:7878", nil)

	testutil.Require(t, "status", response.Code, http.StatusOK)
	testutil.Expect(t, "content type", response.Header().Get("Content-Type"), "application/json")
	testutil.Expect(t, "the id asked for", asked, "docs/a.md")
	var payload project.Document
	testutil.Require(t, "decode the response", json.Unmarshal(response.Body.Bytes(), &payload), nil)
	testutil.Expect(t, "payload", payload, describedDocument())
}

// The route hands the reader the id the caller wrote, cleaning nothing: the
// reader is the one boundary, and a path this layer tidied first would be a
// second opinion about what was asked for.
func TestDocumentIDReachesTheReaderUntouched(t *testing.T) {
	t.Parallel()

	for _, id := range []string{"../../etc/passwd", "docs/../../secret.md", "metasystem/metasystem.conf.local", "docs//a.md"} {
		t.Run(id, func(t *testing.T) {
			t.Parallel()

			asked := ""
			info := Info{Document: func(requested string) (project.Document, error) {
				asked = requested
				return project.Document{}, project.ErrNotFound
			}}

			response := request(t, New(info, loopback(), testBundle()), http.MethodGet, "/api/documents/"+id, "127.0.0.1:7878", nil)

			testutil.Expect(t, "the id asked for", asked, id)
			testutil.Expect(t, "status", response.Code, http.StatusNotFound)
		})
	}
}

// A reader's refusal is a 404 naming the id, and nothing else about the
// filesystem; any other failure is the 500 carrying its reason.
func TestDocumentRefusalsAndFailures(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name       string
		info       Info
		path       string
		wantStatus int
		wantError  string
	}{
		{
			name:       "not found",
			info:       Info{Document: func(string) (project.Document, error) { return project.Document{}, project.ErrNotFound }},
			path:       "/api/documents/docs/missing.md",
			wantStatus: http.StatusNotFound,
			wantError:  "no document at docs/missing.md",
		},
		{
			name:       "a reader that fails",
			info:       Info{Document: func(string) (project.Document, error) { return project.Document{}, errFailed }},
			path:       "/api/documents/docs/a.md",
			wantStatus: http.StatusInternalServerError,
			wantError:  "the checkout could not be opened",
		},
		{
			name:       "no reader at all",
			info:       Info{},
			path:       "/api/documents/docs/a.md",
			wantStatus: http.StatusInternalServerError,
			wantError:  "this engine was built without a document reader",
		},
		{
			name:       "no pane at all",
			info:       Info{},
			path:       "/api/project",
			wantStatus: http.StatusInternalServerError,
			wantError:  "this engine was built without a project reader",
		},
		{
			name:       "a pane that fails",
			info:       Info{Project: func() (project.Pane, error) { return project.Pane{}, errFailed }},
			path:       "/api/project",
			wantStatus: http.StatusInternalServerError,
			wantError:  "the checkout could not be opened",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			response := request(t, New(tc.info, loopback(), testBundle()), http.MethodGet, tc.path, "127.0.0.1:7878", nil)

			testutil.Require(t, "status", response.Code, tc.wantStatus)
			var payload struct {
				Error string `json:"error"`
			}
			testutil.Require(t, "decode the response", json.Unmarshal(response.Body.Bytes(), &payload), nil)
			testutil.Expect(t, "the reason", payload.Error, tc.wantError)
		})
	}
}

// What lies beneath the pane, and the document prefix with no id, belong to
// no resource: they are the same 404 as any other unserved reserved path, and
// neither reaches a reader.
func TestUnservedPathsBeneathTheNewRoutes(t *testing.T) {
	t.Parallel()

	for _, path := range []string{"/api/project/", "/api/project/intent", "/api/documents", "/api/documents/"} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			reached := 0
			info := Info{
				Project:  func() (project.Pane, error) { reached++; return describedPane(), nil },
				Document: func(string) (project.Document, error) { reached++; return describedDocument(), nil },
			}

			response := request(t, New(info, loopback(), testBundle()), http.MethodGet, path, "127.0.0.1:7878", nil)

			testutil.Expect(t, "status", response.Code, http.StatusNotFound)
			testutil.Expect(t, "no reader ran", reached, 0)
		})
	}
}

// HEAD is admitted beside GET with no route branch, so on these routes it does
// the GET's work and the server discards the body. It is exercised through a
// real server, because a recorder keeps what a handler writes and could not
// tell suppression from a short-circuit that never ran the reader.
func TestHeadOnTheNewRoutesIsTheGetWithoutABody(t *testing.T) {
	t.Parallel()

	for _, path := range []string{"/api/project", "/api/documents/docs/a.md"} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			reads := 0
			var handler http.Handler
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handler.ServeHTTP(w, r)
			}))
			defer server.Close()
			handler = New(Info{
				Project:  func() (project.Pane, error) { reads++; return describedPane(), nil },
				Document: func(string) (project.Document, error) { reads++; return describedDocument(), nil },
			}, server.Listener.Addr(), testBundle())

			get, err := http.Get(server.URL + path)
			testutil.Require(t, "the GET", err, nil)
			getBody, err := io.ReadAll(get.Body)
			testutil.Require(t, "read the GET's body", err, nil)
			testutil.Require(t, "close the GET", get.Body.Close(), nil)

			head, err := http.Head(server.URL + path)
			testutil.Require(t, "the HEAD", err, nil)
			headBody, err := io.ReadAll(head.Body)
			testutil.Require(t, "read the HEAD's body", err, nil)
			testutil.Require(t, "close the HEAD", head.Body.Close(), nil)

			testutil.Expect(t, "the status", head.StatusCode, get.StatusCode)
			testutil.Expect(t, "the content type", head.Header.Get("Content-Type"), get.Header.Get("Content-Type"))
			testutil.Expect(t, "the policy", head.Header.Get("Content-Security-Policy"), get.Header.Get("Content-Security-Policy"))
			testutil.Expect(t, "the cache directive", head.Header.Get("Cache-Control"), get.Header.Get("Cache-Control"))
			testutil.Expect(t, "the GET carries a body", len(getBody) > 0, true)
			testutil.Expect(t, "the HEAD carries none", len(headBody), 0)
			testutil.Expect(t, "both did the reader's work", reads, 2)
		})
	}
}

// The headers and the policy every other response carries are on these two.
func TestTheNewRoutesCarryTheSameHeaders(t *testing.T) {
	t.Parallel()

	info := Info{
		Project:  func() (project.Pane, error) { return describedPane(), nil },
		Document: func(string) (project.Document, error) { return describedDocument(), nil },
	}
	served := New(info, loopback(), testBundle())

	thread := request(t, served, http.MethodGet, "/api/project", "127.0.0.1:7878", nil)
	document := request(t, served, http.MethodGet, "/api/documents/docs/a.md", "127.0.0.1:7878", nil)

	for name, value := range map[string]string{
		"X-Content-Type-Options":       "nosniff",
		"X-Frame-Options":              "DENY",
		"Referrer-Policy":              "same-origin",
		"Cross-Origin-Resource-Policy": "same-origin",
		"Cache-Control":                "no-store",
		"Content-Security-Policy":      policy(""),
	} {
		testutil.Expect(t, "the thread's "+name, thread.Header().Get(name), value)
		testutil.Expect(t, "the document's "+name, document.Header().Get(name), value)
	}
}
