package httpd

import (
	"encoding/json"
	"io/fs"
	"net"
	"net/http"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/web"
)

const (
	testIndexHTML = `<!doctype html><html lang="en"><head><meta charset="utf-8">` +
		`<meta property="csp-nonce" nonce="` + web.NoncePlaceholder + `"><title>MetaSystem interface</title>` +
		`<link rel="stylesheet" href="/assets/app.css"></head><body><div id="root"></div>` +
		`<script type="module" src="/assets/app.js"></script></body></html>`
	testNonce      = "cnVubmluZ0Egbm9uY2U"
	testNoticeText = "MetaSystem interface open-source notices\n"
)

// testBundle is a bundle of the shape the build produces: the page, the two
// assets it names, the notices, and one font.
func testBundle() fstest.MapFS {
	return fstest.MapFS{
		"index.html":                {Data: []byte(testIndexHTML)},
		"assets/app.js":             {Data: []byte("export const ready = true;\n")},
		"assets/app.css":            {Data: []byte(":root{color-scheme:light dark}\n")},
		"assets/inter.woff2":        {Data: []byte{0x77, 0x4f, 0x46, 0x32}},
		"assets/notes.bin":          {Data: []byte("outside the content-type table")},
		"THIRD-PARTY-NOTICES.txt":   {Data: []byte(testNoticeText)},
		"assets/inner/deep/app.js":  {Data: []byte("export const deep = true;\n")},
		"assets/inner/deep/app.css": {Data: []byte(".deep{}\n")},
	}
}

func testHandler(t *testing.T, bundle fs.FS) *handler {
	t.Helper()
	bound := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 7878}
	return newHandler(Info{}, bound, bundle, func() string { return testNonce })
}

func TestBundleRoutes(t *testing.T) {
	t.Parallel()

	served := testHandler(t, testBundle())
	tests := []struct {
		name        string
		path        string
		accept      string
		wantStatus  int
		wantType    string
		wantBody    string
		wantHasBody string
	}{
		{name: "root is the page", path: "/", wantStatus: http.StatusOK, wantType: "text/html; charset=utf-8", wantHasBody: "nonce=\"" + testNonce + "\""},
		{name: "asset", path: "/assets/app.js", wantStatus: http.StatusOK, wantType: "text/javascript; charset=utf-8", wantBody: "export const ready = true;\n"},
		{name: "stylesheet", path: "/assets/app.css", wantStatus: http.StatusOK, wantType: "text/css; charset=utf-8"},
		{name: "font", path: "/assets/inter.woff2", wantStatus: http.StatusOK, wantType: "font/woff2"},
		{name: "notices", path: "/THIRD-PARTY-NOTICES.txt", wantStatus: http.StatusOK, wantType: "text/plain; charset=utf-8", wantBody: testNoticeText},
		{name: "nested asset", path: "/assets/inner/deep/app.js", wantStatus: http.StatusOK, wantType: "text/javascript; charset=utf-8"},
		{name: "index.html is the page, never the file", path: "/index.html", accept: "text/html", wantStatus: http.StatusOK, wantType: "text/html; charset=utf-8", wantHasBody: "nonce=\"" + testNonce + "\""},
		{name: "deep path with a media range", path: "/some/deep/path", accept: "TEXT/HTML;level=1, */*;q=0.1", wantStatus: http.StatusOK, wantType: "text/html; charset=utf-8", wantHasBody: "<div id=\"root\"></div>"},
		{name: "missing asset", path: "/assets/missing.js", accept: "text/html", wantStatus: http.StatusNotFound, wantBody: "404 page not found\n"},
		{name: "asset directory with a slash", path: "/assets/", accept: "text/html", wantStatus: http.StatusNotFound, wantBody: "404 page not found\n"},
		{name: "asset directory", path: "/assets", accept: "text/html", wantStatus: http.StatusNotFound, wantBody: "404 page not found\n"},
		{name: "asset outside the content-type table", path: "/assets/notes.bin", accept: "text/html", wantStatus: http.StatusNotFound, wantBody: "404 page not found\n"},
		{name: "api path", path: "/api/goals", accept: "text/html", wantStatus: http.StatusNotFound, wantBody: "404 page not found\n"},
		{name: "beneath the workspace route", path: "/api/workspace/goals", accept: "text/html", wantStatus: http.StatusNotFound, wantBody: "404 page not found\n"},
		{name: "api root", path: "/api", accept: "text/html", wantStatus: http.StatusNotFound, wantBody: "404 page not found\n"},
		{name: "server prefix", path: "/-", accept: "text/html", wantStatus: http.StatusNotFound, wantBody: "404 page not found\n"},
		{name: "server prefix beneath", path: "/-/elsewhere", accept: "text/html", wantStatus: http.StatusNotFound, wantBody: "404 page not found\n"},
		{name: "dot segments are resolved before routing", path: "/assets/../nothing", accept: "text/html", wantStatus: http.StatusOK, wantType: "text/html; charset=utf-8"},
		{name: "any media range", path: "/nothing", accept: "*/*", wantStatus: http.StatusNotFound, wantBody: "404 page not found\n"},
		{name: "html refused by weight", path: "/nothing", accept: "text/html;q=0", wantStatus: http.StatusNotFound, wantBody: "404 page not found\n"},
		{name: "type range", path: "/nothing", accept: "text/*", wantStatus: http.StatusNotFound, wantBody: "404 page not found\n"},
		{name: "no accept header", path: "/nothing", wantStatus: http.StatusNotFound, wantBody: "404 page not found\n"},
		{name: "a name that is not a valid path", path: "//nothing", accept: "text/html", wantStatus: http.StatusOK, wantType: "text/html; charset=utf-8"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			headers := map[string]string{}
			if tc.accept != "" {
				headers["Accept"] = tc.accept
			}
			response := request(t, served, http.MethodGet, tc.path, "127.0.0.1:7878", headers)
			testutil.Expect(t, "status", response.Code, tc.wantStatus)
			if tc.wantType != "" {
				testutil.Expect(t, "content type", response.Header().Get("Content-Type"), tc.wantType)
			}
			if tc.wantBody != "" {
				testutil.Expect(t, "body", response.Body.String(), tc.wantBody)
			}
			if tc.wantHasBody != "" {
				testutil.Expect(t, "body holds "+tc.wantHasBody, strings.Contains(response.Body.String(), tc.wantHasBody), true)
			}
		})
	}
}

func TestPageCarriesTheNonceOfItsPolicy(t *testing.T) {
	t.Parallel()

	bound := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 7878}
	served := New(Info{}, bound, testBundle())

	first := request(t, served, http.MethodGet, "/", "127.0.0.1:7878", nil)
	testutil.Require(t, "first status", first.Code, http.StatusOK)
	firstNonce := policyNonce(t, first.Header().Get("Content-Security-Policy"))
	testutil.Expect(t, "nonce length", len(firstNonce), 22)
	testutil.Expect(t, "body carries the header nonce", strings.Contains(first.Body.String(), `nonce="`+firstNonce+`"`), true)
	testutil.Expect(t, "placeholder is gone", strings.Contains(first.Body.String(), web.NoncePlaceholder), false)

	second := request(t, served, http.MethodGet, "/", "127.0.0.1:7878", nil)
	testutil.Require(t, "second status", second.Code, http.StatusOK)
	secondNonce := policyNonce(t, second.Header().Get("Content-Security-Policy"))
	testutil.Expect(t, "a second response reuses the nonce", secondNonce == firstNonce, false)
	testutil.Expect(t, "second body carries its own nonce", strings.Contains(second.Body.String(), `nonce="`+secondNonce+`"`), true)
}

func TestPolicyOnEveryResponseClass(t *testing.T) {
	t.Parallel()

	const withoutNonce = "default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self'; font-src 'self'; connect-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'"
	const withNonce = "default-src 'none'; script-src 'self'; style-src 'self' 'nonce-" + testNonce + "'; img-src 'self'; font-src 'self'; connect-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'"
	headers := map[string]string{
		"X-Content-Type-Options":       "nosniff",
		"X-Frame-Options":              "DENY",
		"Referrer-Policy":              "same-origin",
		"Cross-Origin-Resource-Policy": "same-origin",
		"Cache-Control":                "no-store",
	}
	served := testHandler(t, testBundle())
	absent := testHandler(t, nil)
	tests := []struct {
		name       string
		server     *handler
		path       string
		host       string
		wantPolicy string
	}{
		{name: "page", server: served, path: "/", host: "127.0.0.1:7878", wantPolicy: withNonce},
		{name: "asset", server: served, path: "/assets/app.js", host: "127.0.0.1:7878", wantPolicy: withoutNonce},
		{name: "health", server: served, path: "/-/health", host: "127.0.0.1:7878", wantPolicy: withoutNonce},
		{name: "not found", server: served, path: "/assets/missing.js", host: "127.0.0.1:7878", wantPolicy: withoutNonce},
		{name: "refusal", server: served, path: "/", host: "untrusted.example", wantPolicy: withoutNonce},
		{name: "absent bundle", server: absent, path: "/", host: "127.0.0.1:7878", wantPolicy: withoutNonce},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			response := request(t, tc.server, http.MethodGet, tc.path, tc.host, nil)
			testutil.Expect(t, "content security policy", response.Header().Get("Content-Security-Policy"), tc.wantPolicy)
			for name, want := range headers {
				testutil.Expect(t, "header "+name, response.Header().Get(name), want)
			}
		})
	}
}

func TestAbsentBundleStatesWhatIsMissing(t *testing.T) {
	t.Parallel()

	withoutIndex := fstest.MapFS{"assets/app.js": {Data: []byte("export const ready = true;\n")}}
	tests := []struct {
		name   string
		bundle fs.FS
	}{
		{name: "no bundle at all", bundle: nil},
		{name: "a bundle without index.html", bundle: withoutIndex},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			served := testHandler(t, tc.bundle)

			page := request(t, served, http.MethodGet, "/", "127.0.0.1:7878", nil)
			testutil.Expect(t, "page status", page.Code, http.StatusServiceUnavailable)
			testutil.Expect(t, "page content type", page.Header().Get("Content-Type"), "text/plain; charset=utf-8")
			testutil.Expect(t, "page body", page.Body.String(), "MetaSystem interface: this executable was built without the interface bundle. Rebuild it from a checkout that contains internal/ui/web/bundle/dist, then run: metasystem restart ui")

			deep := request(t, served, http.MethodGet, "/some/deep/path", "127.0.0.1:7878", map[string]string{"Accept": "text/html"})
			testutil.Expect(t, "deep page status", deep.Code, http.StatusServiceUnavailable)

			asset := request(t, served, http.MethodGet, "/assets/app.js", "127.0.0.1:7878", nil)
			testutil.Expect(t, "asset status", asset.Code, http.StatusNotFound)

			health := request(t, served, http.MethodGet, "/-/health", "127.0.0.1:7878", nil)
			testutil.Require(t, "health status", health.Code, http.StatusOK)
			var payload struct {
				Status       string `json:"status"`
				BundleDigest string `json:"bundleDigest"`
			}
			err := json.Unmarshal(health.Body.Bytes(), &payload)
			testutil.Require(t, "decode health response", err, nil)
			testutil.Expect(t, "health status field", payload.Status, "ok")
			testutil.Expect(t, "bundle digest", payload.BundleDigest, "")
		})
	}
}

// policyNonce reads the nonce back out of the policy the response carries, so
// the body is checked against what the browser was actually told to allow.
func policyNonce(t *testing.T, policy string) string {
	t.Helper()
	const opening = "style-src 'self' 'nonce-"
	start := strings.Index(policy, opening)
	if start < 0 {
		t.Fatalf("the policy carries no nonce: %q", policy)
	}
	rest := policy[start+len(opening):]
	end := strings.Index(rest, "'")
	if end < 0 {
		t.Fatalf("the policy's nonce is unterminated: %q", policy)
	}
	return rest[:end]
}
