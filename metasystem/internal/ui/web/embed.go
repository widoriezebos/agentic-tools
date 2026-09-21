// Package web carries the built interface bundle, the manifest that records
// the source it was built from, and the rules the server needs to serve it.
package web

import (
	"embed"
	"io/fs"
)

// bundle/README.txt is committed, so the pattern always matches and a checkout
// without a built bundle still builds; what is then absent is dist/, which the
// server reports rather than failing to start. all: takes the dot and
// underscore names a plain pattern would drop.
//
//go:embed all:bundle
var bundle embed.FS

// NoncePlaceholder is the literal the bundle script leaves in index.html. The
// server replaces it with a fresh nonce on every page response.
const NoncePlaceholder = "__METASYSTEM_CSP_NONCE__"

// Dist is the served tree: every path in it is relative to bundle/dist, so
// README.txt and bundle.json sit outside it and no request can reach them.
func Dist() fs.FS {
	dist, err := fs.Sub(bundle, "bundle/dist")
	if err != nil {
		return nil
	}
	return dist
}
