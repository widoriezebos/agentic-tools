package steward

import (
	"errors"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Every tick that is not at the helm runs the pattern pass once, after its
// health pass, at the tick's own time; a pass that fails is the tick's
// error, never a silent one.
func TestTickRunsPatternsAfterHealth(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeLedger(t, root, "# Goals\n\n## Current goal: fix-it — Repair the thing\n- Origin: main\n- Next step: Repair it.\n")
	now := time.Date(2026, 9, 30, 11, 10, 0, 0, time.UTC)
	var calls atomic.Int32
	var healthBefore bool
	cfg := TickConfig{Now: now, Patterns: func(repo string, at time.Time) error {
		calls.Add(1)
		_, err := os.Stat(ComponentEvidencePath(root, "narrator"))
		healthBefore = err == nil
		if repo != root || !at.Equal(now) {
			t.Errorf("the pass ran for %s at %s", repo, at)
		}
		return errors.New("the alerts store is full")
	}}
	_, err := RunTick(root, cfg, fakeCensus{})
	if calls.Load() != 1 || !healthBefore {
		t.Fatalf("the pattern pass ran %d times, after health %t", calls.Load(), healthBefore)
	}
	if err == nil || !strings.Contains(err.Error(), "behaviour patterns: the alerts store is full") {
		t.Fatalf("the pass's failure is not the tick's: %v", err)
	}
}
