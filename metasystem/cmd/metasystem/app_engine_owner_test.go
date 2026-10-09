package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// The engine that supervises a run is the invocation's own owner, never a
// process-wide variable: parallel tests that each name their engine cannot
// swap it under one another, and a start launches exactly the engine its own
// invocation was given.
func TestAppStartLaunchesTheInvocationsOwnEngine(t *testing.T) {
	t.Parallel()
	address := appHeldPort(t)
	bed := newAppBed(t, appHTTPContract(appFixtureApp(t), address))
	missing := filepath.Join(t.TempDir(), "this-invocations-engine")
	bed.engine = missing
	code, out := bed.run("app", "start")
	if code == 0 {
		t.Fatalf("a start whose engine cannot run is refused:\n%s", out)
	}
	if !strings.Contains(out, "this-invocations-engine") {
		t.Fatalf("the start launched an engine other than its invocation's own:\n%s", out)
	}
}
