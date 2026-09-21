package web

import (
	"path"
	"strings"
)

// contentTypes is a fixed table rather than mime.TypeByExtension, which reads
// the host's mime.types files and would make what the server answers depend on
// the machine it runs on. A bundle holds these kinds and no others.
var contentTypes = map[string]string{
	".html":  "text/html; charset=utf-8",
	".js":    "text/javascript; charset=utf-8",
	".css":   "text/css; charset=utf-8",
	".json":  "application/json",
	".txt":   "text/plain; charset=utf-8",
	".svg":   "image/svg+xml",
	".woff2": "font/woff2",
	".png":   "image/png",
}

// ContentType reports the type a bundle file is served with, and false for a
// name outside the table, which is then not served at all.
func ContentType(name string) (string, bool) {
	contentType, known := contentTypes[strings.ToLower(path.Ext(name))]
	return contentType, known
}
