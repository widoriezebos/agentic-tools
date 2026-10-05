package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHealthCapabilitySnapshotsHaveTheStewardsAutomaticRemedy(t *testing.T) {
	t.Parallel()
	bed := newProcessBed(t)
	if err := os.WriteFile(filepath.Join(bed.root(), "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := bed.run(bed.owners(), "system", "check")
	visible := strings.Join(strings.Fields(stdout), " ")
	if code != 1 || !strings.Contains(visible, "capability-snapshots") || !strings.Contains(visible, "the steward tick probes") {
		t.Fatalf("metasystem system check lost the automatic probe remedy: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}
