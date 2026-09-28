package main

import (
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

type processRefProber struct {
	exact identity.Exact
	state identity.Liveness
	err   error
}

func (p processRefProber) Probe(int64) (identity.Exact, identity.Liveness, error) {
	return p.exact, p.state, p.err
}

func TestProcSetsidRefusesWithoutACommandAndAnAbsentOne(t *testing.T) {
	// The exec path replaces the test process; only the refusals are checked
	// here.
	if got := runProcSetsid([]string{"--"}); got != 2 {
		t.Fatalf("proc setsid without a command exit = %d, want 2", got)
	}
	if got := runProcSetsid([]string{"--", "/nonexistent/metasystem-no-such-command"}); got != 127 {
		t.Fatalf("proc setsid with an absent command exit = %d, want 127", got)
	}
}
