package httpd

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"slices"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/workspace"
)

func describedWorkspace() workspace.Workspace {
	return workspace.Workspace{
		SchemaVersion:    workspace.SchemaVersion,
		Subject:          "MetaSystem",
		Mode:             workspace.ModeSelfHosted,
		Conflict:         false,
		Checkout:         "/work/repository",
		Installation:     "/work/repository/metasystem",
		StateRoot:        "/work/repository/metasystem",
		EngineBuild:      "dev-0123456789ab",
		StartedAt:        "2026-09-21T10:11:12Z",
		ExecutableDigest: "sha256:0123456789abcdef",
		SourceHead:       "",
		AdoptedFrom:      "",
		AdoptionRecord:   string(workspace.Placeholder),
		// The private store, which Settings reads from this same resource: its
		// path, what it holds per workspace, and the bounds in words (g1-s54 D3).
		Store: &workspace.Store{
			Path: "/home/one/.metasystem/ui",
			Workspaces: []workspace.StoreWorkspace{
				{Name: "repository-0a1b2c", Size: "49 KB", Current: true},
			},
			Bounds: []string{"The Partner's wire journal is rotated at 8 MB, keeping one previous."},
		},
	}
}

// The resource carries every field the header, the tab title, and Settings'
// About card read, and it is described per request rather than at construction.
func TestWorkspacePayload(t *testing.T) {
	t.Parallel()

	calls := 0
	info := Info{Describe: func() (workspace.Workspace, error) {
		calls++
		return describedWorkspace(), nil
	}}
	bound := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 7878}
	served := New(info, bound, testBundle())

	response := request(t, served, http.MethodGet, "/api/workspace", "127.0.0.1:7878", nil)

	testutil.Require(t, "status", response.Code, http.StatusOK)
	testutil.Expect(t, "content type", response.Header().Get("Content-Type"), "application/json")
	var payload workspace.Workspace
	testutil.Require(t, "decode the response", json.Unmarshal(response.Body.Bytes(), &payload), nil)
	testutil.Expect(t, "payload", payload, describedWorkspace())

	second := request(t, served, http.MethodGet, "/api/workspace", "127.0.0.1:7878", nil)

	testutil.Expect(t, "second status", second.Code, http.StatusOK)
	testutil.Expect(t, "describe calls", calls, 2)
}

// Every field the interface reads is spelled as the resource's contract names
// it; a renamed field is a chip that silently reads undefined.
func TestWorkspacePayloadFieldNames(t *testing.T) {
	t.Parallel()

	info := Info{Describe: func() (workspace.Workspace, error) { return describedWorkspace(), nil }}
	bound := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 7878}

	response := request(t, New(info, bound, testBundle()), http.MethodGet, "/api/workspace", "127.0.0.1:7878", nil)

	testutil.Require(t, "status", response.Code, http.StatusOK)
	var fields map[string]any
	testutil.Require(t, "decode the response", json.Unmarshal(response.Body.Bytes(), &fields), nil)
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	want := []string{
		"adoptedFrom", "adoptionRecord", "checkout", "conflict", "engineBuild", "executableDigest",
		"installation", "mode", "schemaVersion", "sourceHead", "startedAt", "stateRoot", "store", "subject",
	}
	slices.Sort(names)
	testutil.Expect(t, "field names", names, want)
}

// A description that fails is a 500 carrying the reason, so the page can show
// what went wrong instead of an empty refusal.
func TestWorkspaceFailure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		describe func() (workspace.Workspace, error)
		wantBody string
	}{
		{
			name: "a description that fails",
			describe: func() (workspace.Workspace, error) {
				return workspace.Workspace{}, errors.New("cannot resolve the layout")
			},
			wantBody: "{\"error\":\"cannot resolve the layout\"}\n",
		},
		{
			name:     "an engine that cannot describe at all",
			describe: nil,
			wantBody: "{\"error\":\"this engine was built without a workspace description\"}\n",
		},
	}
	bound := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 7878}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			served := New(Info{Describe: tc.describe}, bound, testBundle())

			response := request(t, served, http.MethodGet, "/api/workspace", "127.0.0.1:7878", nil)

			testutil.Expect(t, "status", response.Code, http.StatusInternalServerError)
			testutil.Expect(t, "content type", response.Header().Get("Content-Type"), "application/json")
			testutil.Expect(t, "body", response.Body.String(), tc.wantBody)
		})
	}
}

// The route is the API's only path in this build: what lies beneath it, or
// beside it, is a 404 rather than the page, so a mistyped endpoint says so.
func TestWorkspaceRouteIsExact(t *testing.T) {
	t.Parallel()

	tests := []struct{ name, path string }{
		{name: "beneath the route", path: "/api/workspace/goals"},
		{name: "a trailing slash", path: "/api/workspace/"},
		{name: "a longer name", path: "/api/workspaces"},
		{name: "another resource", path: "/api/goals"},
	}
	info := Info{Describe: func() (workspace.Workspace, error) { return describedWorkspace(), nil }}
	bound := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 7878}
	served := New(info, bound, testBundle())
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			response := request(t, served, http.MethodGet, tc.path, "127.0.0.1:7878", map[string]string{"Accept": "text/html"})

			testutil.Expect(t, "status", response.Code, http.StatusNotFound)
			testutil.Expect(t, "body", response.Body.String(), "404 page not found\n")
		})
	}
}

// The route is behind the same checks and carries the same headers as every
// other response: no check is loosened for the API.
func TestWorkspaceRouteIsBehindTheChecks(t *testing.T) {
	t.Parallel()

	const withoutNonce = "default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self'; font-src 'self'; connect-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'"
	info := Info{Describe: func() (workspace.Workspace, error) { return describedWorkspace(), nil }}
	bound := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 7878}
	served := New(info, bound, testBundle())

	allowed := request(t, served, http.MethodGet, "/api/workspace", "127.0.0.1:7878", map[string]string{"Sec-Fetch-Site": "same-origin"})
	testutil.Require(t, "same-origin status", allowed.Code, http.StatusOK)
	testutil.Expect(t, "content security policy", allowed.Header().Get("Content-Security-Policy"), withoutNonce)
	testutil.Expect(t, "no store", allowed.Header().Get("Cache-Control"), "no-store")
	testutil.Expect(t, "no sniff", allowed.Header().Get("X-Content-Type-Options"), "nosniff")

	crossSite := request(t, served, http.MethodGet, "/api/workspace", "127.0.0.1:7878", map[string]string{"Sec-Fetch-Site": "cross-site"})
	testutil.Expect(t, "cross-site status", crossSite.Code, http.StatusForbidden)

	foreignHost := request(t, served, http.MethodGet, "/api/workspace", "attacker.invalid", nil)
	testutil.Expect(t, "foreign host status", foreignHost.Code, http.StatusForbidden)

	post := request(t, served, http.MethodPost, "/api/workspace", "127.0.0.1:7878", nil)
	testutil.Expect(t, "POST status", post.Code, http.StatusMethodNotAllowed)
}
