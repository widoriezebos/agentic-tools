package batchowner

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

// The owner's log is append-only and bounded: past its cap the file rolls to
// one previous generation, so the newest lines are always in landing-owner.log.
func TestOwnerLogAppendsAndRollsAtItsCap(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	log := newOwnerLog(root, 64)
	for index := 0; index < 5; index++ {
		if _, err := log.Write([]byte(strings.Repeat("x", 20) + "\n")); err != nil {
			t.Fatal(err)
		}
	}
	current, err := os.ReadFile(OwnerLogPath(root))
	if err != nil {
		t.Fatal(err)
	}
	previous, err := os.ReadFile(OwnerLogPath(root) + ".1")
	if err != nil {
		t.Fatal(err)
	}
	if len(current) > 64 || len(previous) > 64 || len(current)+len(previous) < 5*21-64 {
		t.Fatalf("log sizes current=%d previous=%d, want each at most the cap and the newest kept", len(current), len(previous))
	}
	if filepath.Dir(OwnerLogPath(root)) != filepath.Join(root, "artifacts", "agents", "supervision") {
		t.Fatalf("owner log path %s is not in the supervision directory", OwnerLogPath(root))
	}
}

// Every owner report goes to the log as a JSON line and becomes the owner's
// last tick error; a pass with no report clears it, one with a report keeps
// the newest.
func TestOwnerReportsRecordTheLastTickErrorUntilACleanPass(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	var logged strings.Builder
	ticks := NewTickErrors(root)
	report := ownerReport(&logged, ticks)
	ticks.Begin()
	report("01j5x00000000000000000rb01", errors.New("read joined goal change:5555 before rebind: absent"))
	ticks.End()
	if !strings.Contains(logged.String(), `"component":"landing-owner"`) || !strings.Contains(logged.String(), "before rebind") {
		t.Fatalf("logged %q", logged.String())
	}
	line := lane.LastTickErrorLine(root)
	if !strings.Contains(line, "batch 01j5x00000000000000000rb01") || !strings.Contains(line, "before rebind") {
		t.Fatalf("last tick error %q", line)
	}
	ticks.Begin()
	ticks.End()
	if line := lane.LastTickErrorLine(root); line != "" {
		t.Fatalf("a clean pass left the tick error %q", line)
	}
}
