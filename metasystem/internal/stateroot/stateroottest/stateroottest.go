// Package stateroottest builds installation and state roots for test fixtures.
//
// A fixture directory rarely has the shape of a real installation, so these
// constructors make the path absolute and clean and check nothing else. Each
// takes the test's handle: production code cannot call them, and reaches a
// root only through the resolver and the parsers of internal/stateroot.
package stateroottest

import (
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/roots"
)

// Installation names dir as the installation root of a test fixture.
func Installation(tb testing.TB, dir string) roots.Installation {
	tb.Helper()
	return roots.Installation(absolute(tb, dir))
}

// State names dir as the state root of a test fixture.
func State(tb testing.TB, dir string) roots.State {
	tb.Helper()
	return roots.State(absolute(tb, dir))
}

func absolute(tb testing.TB, dir string) string {
	tb.Helper()
	path, err := filepath.Abs(dir)
	if err != nil {
		tb.Fatalf("locate fixture root %q: %v", dir, err)
	}
	return path
}
