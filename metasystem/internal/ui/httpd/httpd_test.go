package httpd

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

func TestRequestChecks(t *testing.T) {
	t.Parallel()

	type requestCase struct {
		name       string
		bound      *net.TCPAddr
		method     string
		host       string
		headers    map[string]string
		wantStatus int
		wantBody   string
	}
	standardBound := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 7878}
	cases := []requestCase{
		{name: "GET", bound: standardBound, method: http.MethodGet, host: "127.0.0.1:7878", wantStatus: http.StatusOK},
		{name: "HEAD", bound: standardBound, method: http.MethodHead, host: "127.0.0.1:7878", wantStatus: http.StatusOK},
		{name: "method before host", bound: standardBound, method: http.MethodPost, host: "attacker.invalid", headers: map[string]string{"Origin": "https://attacker.invalid"}, wantStatus: http.StatusMethodNotAllowed, wantBody: "method not allowed\n"},
		{name: "foreign host", bound: standardBound, method: http.MethodGet, host: "attacker.invalid", wantStatus: http.StatusForbidden, wantBody: "request host is not allowed\n"},
		{name: "host before fetch site", bound: standardBound, method: http.MethodGet, host: "attacker.invalid", headers: map[string]string{"Sec-Fetch-Site": "cross-site"}, wantStatus: http.StatusForbidden, wantBody: "request host is not allowed\n"},
		{name: "localhost mixed case", bound: standardBound, method: http.MethodGet, host: "LoCaLhOsT:7878", wantStatus: http.StatusOK},
		{name: "same origin fetch", bound: standardBound, method: http.MethodGet, host: "127.0.0.1:7878", headers: map[string]string{"Sec-Fetch-Site": "same-origin"}, wantStatus: http.StatusOK},
		{name: "direct fetch", bound: standardBound, method: http.MethodGet, host: "127.0.0.1:7878", headers: map[string]string{"Sec-Fetch-Site": "none"}, wantStatus: http.StatusOK},
		{name: "empty fetch site", bound: standardBound, method: http.MethodGet, host: "127.0.0.1:7878", headers: map[string]string{"Sec-Fetch-Site": ""}, wantStatus: http.StatusForbidden, wantBody: "cross-site request is not allowed\n"},
		{name: "empty fetch site navigation", bound: standardBound, method: http.MethodGet, host: "127.0.0.1:7878", headers: map[string]string{"Sec-Fetch-Site": "", "Sec-Fetch-Mode": "navigate", "Sec-Fetch-Dest": "document"}, wantStatus: http.StatusOK},
		{name: "cross-site fetch", bound: standardBound, method: http.MethodGet, host: "127.0.0.1:7878", headers: map[string]string{"Sec-Fetch-Site": "cross-site"}, wantStatus: http.StatusForbidden, wantBody: "cross-site request is not allowed\n"},
		{name: "cross-site navigation", bound: standardBound, method: http.MethodGet, host: "127.0.0.1:7878", headers: map[string]string{"Sec-Fetch-Site": "cross-site", "Sec-Fetch-Mode": "navigate", "Sec-Fetch-Dest": "document"}, wantStatus: http.StatusOK},
		{name: "HEAD has no navigation exemption", bound: standardBound, method: http.MethodHead, host: "127.0.0.1:7878", headers: map[string]string{"Sec-Fetch-Site": "cross-site", "Sec-Fetch-Mode": "navigate", "Sec-Fetch-Dest": "document"}, wantStatus: http.StatusForbidden, wantBody: "cross-site request is not allowed\n"},
		{name: "navigation mode required", bound: standardBound, method: http.MethodGet, host: "127.0.0.1:7878", headers: map[string]string{"Sec-Fetch-Site": "cross-site", "Sec-Fetch-Mode": "cors", "Sec-Fetch-Dest": "document"}, wantStatus: http.StatusForbidden, wantBody: "cross-site request is not allowed\n"},
		{name: "document destination required", bound: standardBound, method: http.MethodGet, host: "127.0.0.1:7878", headers: map[string]string{"Sec-Fetch-Site": "cross-site", "Sec-Fetch-Mode": "navigate", "Sec-Fetch-Dest": "empty"}, wantStatus: http.StatusForbidden, wantBody: "cross-site request is not allowed\n"},
		{name: "allowed IP origin", bound: standardBound, method: http.MethodGet, host: "127.0.0.1:7878", headers: map[string]string{"Origin": "http://127.0.0.1:7878"}, wantStatus: http.StatusOK},
		{name: "allowed localhost origin mixed case", bound: standardBound, method: http.MethodGet, host: "127.0.0.1:7878", headers: map[string]string{"Origin": "http://LOCALHOST:7878"}, wantStatus: http.StatusOK},
		{name: "empty origin", bound: standardBound, method: http.MethodGet, host: "127.0.0.1:7878", headers: map[string]string{"Origin": ""}, wantStatus: http.StatusForbidden, wantBody: "request origin is not allowed\n"},
		{name: "uppercase Origin scheme", bound: standardBound, method: http.MethodGet, host: "127.0.0.1:7878", headers: map[string]string{"Origin": "HTTP://127.0.0.1:7878"}, wantStatus: http.StatusForbidden, wantBody: "request origin is not allowed\n"},
		{name: "foreign origin", bound: standardBound, method: http.MethodGet, host: "127.0.0.1:7878", headers: map[string]string{"Origin": "http://attacker.invalid"}, wantStatus: http.StatusForbidden, wantBody: "request origin is not allowed\n"},
		{name: "HTTPS origin", bound: standardBound, method: http.MethodGet, host: "127.0.0.1:7878", headers: map[string]string{"Origin": "https://127.0.0.1:7878"}, wantStatus: http.StatusForbidden, wantBody: "request origin is not allowed\n"},
		{name: "fetch site before origin", bound: standardBound, method: http.MethodGet, host: "127.0.0.1:7878", headers: map[string]string{"Sec-Fetch-Site": "cross-site", "Origin": "http://attacker.invalid"}, wantStatus: http.StatusForbidden, wantBody: "cross-site request is not allowed\n"},
		{name: "port 80 IP with port", bound: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 80}, method: http.MethodGet, host: "127.0.0.1:80", wantStatus: http.StatusOK},
		{name: "port 80 bare IP", bound: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 80}, method: http.MethodGet, host: "127.0.0.1", wantStatus: http.StatusOK},
		{name: "port 80 bare localhost", bound: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 80}, method: http.MethodGet, host: "LOCALHOST", wantStatus: http.StatusOK},
		{name: "IPv6 bound address", bound: &net.TCPAddr{IP: net.ParseIP("::1"), Port: 7878}, method: http.MethodGet, host: "[::1]:7878", wantStatus: http.StatusOK},
		{name: "IPv6 localhost", bound: &net.TCPAddr{IP: net.ParseIP("::1"), Port: 7878}, method: http.MethodGet, host: "localhost:7878", wantStatus: http.StatusOK},
		{name: "IPv6 bare port 80", bound: &net.TCPAddr{IP: net.ParseIP("::1"), Port: 80}, method: http.MethodGet, host: "[::1]", wantStatus: http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			response := request(t, New(Info{}, tc.bound), tc.method, "/", tc.host, tc.headers)
			testutil.Expect(t, "status", response.Code, tc.wantStatus)
			if tc.wantBody != "" {
				testutil.Expect(t, "response body", response.Body.String(), tc.wantBody)
			}
			if tc.wantStatus == http.StatusMethodNotAllowed {
				testutil.Expect(t, "allowed methods", response.Header().Get("Allow"), "GET, HEAD")
			}
		})
	}
}

func TestResponsesCarryBrowserSecurityHeaders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		host string
	}{
		{name: "success", host: "127.0.0.1:7878"},
		{name: "refusal", host: "untrusted.example"},
	}
	wantHeaders := map[string]string{
		"X-Content-Type-Options":       "nosniff",
		"X-Frame-Options":              "DENY",
		"Referrer-Policy":              "same-origin",
		"Cross-Origin-Resource-Policy": "same-origin",
		"Cache-Control":                "no-store",
	}
	bound := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 7878}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			response := request(t, New(Info{}, bound), http.MethodGet, "/", tc.host, nil)
			for name, want := range wantHeaders {
				testutil.Expect(t, "header "+name, response.Header().Get(name), want)
			}
			var accessControlHeaders []string
			for name := range response.Header() {
				if strings.HasPrefix(strings.ToLower(name), "access-control-") {
					accessControlHeaders = append(accessControlHeaders, name)
				}
			}
			testutil.Expect(t, "Access-Control headers", accessControlHeaders, []string(nil))
		})
	}
}

func TestHealthPayload(t *testing.T) {
	t.Parallel()

	info := Info{
		Checkout:         "/work/repository",
		StartedAt:        "2026-09-21T10:11:12Z",
		EngineBuild:      "build-stamp",
		ExecutableDigest: "sha256:0123456789abcdef",
	}
	bound := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 7878}
	response := request(t, New(info, bound), http.MethodGet, "/-/health", "127.0.0.1:7878", nil)
	testutil.Require(t, "status", response.Code, http.StatusOK)
	testutil.Expect(t, "content type", response.Header().Get("Content-Type"), "application/json")
	var payload struct {
		Status           string `json:"status"`
		Checkout         string `json:"checkout"`
		StartedAt        string `json:"startedAt"`
		EngineBuild      string `json:"engineBuild"`
		ExecutableDigest string `json:"executableDigest"`
	}
	err := json.Unmarshal(response.Body.Bytes(), &payload)
	testutil.Require(t, "decode health response", err, nil)
	testutil.Expect(t, "health status", payload.Status, "ok")
	testutil.Expect(t, "checkout", payload.Checkout, info.Checkout)
	testutil.Expect(t, "started at", payload.StartedAt, info.StartedAt)
	testutil.Expect(t, "engine build", payload.EngineBuild, info.EngineBuild)
	testutil.Expect(t, "executable digest", payload.ExecutableDigest, info.ExecutableDigest)
}

func TestRoutes(t *testing.T) {
	t.Parallel()

	bound := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 7878}
	handler := New(Info{}, bound)
	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantBody   string
	}{
		{name: "root", path: "/", wantStatus: http.StatusOK, wantBody: "MetaSystem interface: no views are installed in this build yet."},
		{name: "unknown", path: "/missing", wantStatus: http.StatusNotFound, wantBody: "404 page not found\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			response := request(t, handler, http.MethodGet, tc.path, "127.0.0.1:7878", nil)
			testutil.Expect(t, "status", response.Code, tc.wantStatus)
			testutil.Expect(t, "body", response.Body.String(), tc.wantBody)
		})
	}
}

func TestRefusalsDoNotEchoInput(t *testing.T) {
	t.Parallel()

	const offendingValue = "do-not-echo-this-value.example"
	bound := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 7878}
	tests := []struct {
		name    string
		method  string
		host    string
		headers map[string]string
	}{
		{name: "method", method: offendingValue, host: "127.0.0.1:7878"},
		{name: "host", method: http.MethodGet, host: offendingValue},
		{name: "fetch site", method: http.MethodGet, host: "127.0.0.1:7878", headers: map[string]string{"Sec-Fetch-Site": offendingValue}},
		{name: "origin", method: http.MethodGet, host: "127.0.0.1:7878", headers: map[string]string{"Origin": "http://" + offendingValue}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			response := request(t, New(Info{}, bound), tc.method, "/", tc.host, tc.headers)
			testutil.Expect(t, "refusal status", response.Code == http.StatusForbidden || response.Code == http.StatusMethodNotAllowed, true)
			testutil.Expect(t, "offending value echoed", strings.Contains(response.Body.String(), offendingValue), false)
		})
	}
}

func request(t *testing.T, handler http.Handler, method, path, host string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, "http://example.invalid"+path, nil)
	req.Host = host
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	return response
}
