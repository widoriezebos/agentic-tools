package main

import (
	"bytes"
	"io"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

func TestStewardRunIsArmedWithCardRetirement(t *testing.T) {
	t.Parallel()
	bed := newProcessBed(t)
	var problem bytes.Buffer
	called := false
	code := runStewardRunWithDependencies([]string{"--repo", bed.root()}, io.Discard, &problem,
		func(_ string, _ steward.WorkerCensus, _ func() error, _ time.Duration, cfg steward.TickConfig) error {
			called = true
			if cfg.RetireCards == nil {
				t.Error("steward has no card retirement pass")
			}
			return nil
		}, nil, nil, func(string) int { return 1 }, func() (bool, error) { return false, nil })
	if code != 0 || !called {
		t.Fatalf("steward run: %d %s, loop called %t", code, problem.String(), called)
	}
}
