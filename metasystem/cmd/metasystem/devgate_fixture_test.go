package main

import (
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// writeFixtureDevgate gives a fixture installation the devgate entries the
// engine and the commit boundary run (testutil.FixtureDevgateSource),
// delegating to the fixture's own scripts. It writes a minimal go.mod only
// when the fixture has none.
func writeFixtureDevgate(t *testing.T, installationRoot string) {
	t.Helper()
	if err := testutil.WriteFixtureDevgate(installationRoot); err != nil {
		t.Fatal(err)
	}
}
