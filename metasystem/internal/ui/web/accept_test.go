package web

import (
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

func TestAcceptsHTML(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		accept string
		want   bool
	}{
		{name: "no header", accept: "", want: false},
		{name: "exactly html", accept: "text/html", want: true},
		{name: "a browser's header", accept: "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8", want: true},
		{name: "upper case with a parameter", accept: "TEXT/HTML;level=1, */*;q=0.1", want: true},
		{name: "spaces around the element", accept: "  text/html ; q=0.5 ", want: true},
		{name: "any media range", accept: "*/*", want: false},
		{name: "a type range", accept: "text/*", want: false},
		{name: "html refused by weight", accept: "text/html;q=0", want: false},
		{name: "html refused by a longer zero", accept: "text/html;q=0.000", want: false},
		{name: "html at the smallest positive weight", accept: "text/html;q=0.001", want: true},
		{name: "an unparsable weight", accept: "text/html;q=high", want: false},
		{name: "json only", accept: "application/json", want: false},
		{name: "a malformed element is skipped", accept: "text/;, text/html", want: true},
		{name: "a malformed element alone", accept: "text/;", want: false},
		{name: "html after a refusal of html", accept: "text/html;q=0, text/html", want: true},
		{name: "a subtype that only starts with html", accept: "text/htmlish", want: false},
		{name: "a type that ends with html", accept: "application/xhtml+xml", want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			testutil.Expect(t, "accepts HTML", AcceptsHTML(tc.accept), tc.want)
		})
	}
}

func TestContentType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		file     string
		want     string
		wantKnwn bool
	}{
		{name: "page", file: "index.html", want: "text/html; charset=utf-8", wantKnwn: true},
		{name: "script", file: "assets/app.js", want: "text/javascript; charset=utf-8", wantKnwn: true},
		{name: "stylesheet", file: "assets/app.css", want: "text/css; charset=utf-8", wantKnwn: true},
		{name: "manifest", file: "bundle.json", want: "application/json", wantKnwn: true},
		{name: "notices", file: "THIRD-PARTY-NOTICES.txt", want: "text/plain; charset=utf-8", wantKnwn: true},
		{name: "vector", file: "assets/icon.svg", want: "image/svg+xml", wantKnwn: true},
		{name: "font", file: "assets/inter.woff2", want: "font/woff2", wantKnwn: true},
		{name: "image", file: "assets/shot.png", want: "image/png", wantKnwn: true},
		{name: "upper case extension", file: "assets/APP.JS", want: "text/javascript; charset=utf-8", wantKnwn: true},
		{name: "outside the table", file: "assets/app.wasm", want: "", wantKnwn: false},
		{name: "no extension", file: "LICENSE", want: "", wantKnwn: false},
		{name: "a dot in a directory name only", file: "assets.js/name", want: "", wantKnwn: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			contentType, known := ContentType(tc.file)
			testutil.Expect(t, "content type", contentType, tc.want)
			testutil.Expect(t, "known", known, tc.wantKnwn)
		})
	}
}
