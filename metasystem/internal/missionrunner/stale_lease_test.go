package missionrunner

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// cleanupStaleLease's verdict ladder with real files (Phase 6): nothing to
// clean, a dead holder swept, a live holder refused.
func TestCleanupStaleLease(t *testing.T) {
	engine := &Engine{Root: t.TempDir(), Mission: "mr-stale"}
	dir := engine.missionDir()

	// No lease pieces at all: a clean no-op.
	if err := engine.cleanupStaleLease(); err != nil {
		t.Fatalf("empty mission dir: %v", err)
	}

	// A lease whose recorded pid cannot exist is stale: swept clean.
	os.MkdirAll(filepath.Join(dir, "lease.d"), 0o755)
	lease := filepath.Join(dir, "lease.json")
	os.WriteFile(lease, []byte(`{"missionId":"mr-stale","pid":1073741824,"pgid":1,"instanceTag":"gone-tag","startedAt":"x","renewedAt":"x"}`), 0o644)
	if err := engine.cleanupStaleLease(); err != nil {
		t.Fatalf("sweeping a dead holder: %v", err)
	}
	if pathExists(filepath.Join(dir, "lease.d")) || pathExists(lease) {
		t.Fatal("the stale lease pieces survived the sweep")
	}

	// A live holder — our own pid, with a tag our argv carries — refuses.
	self := os.Getpid()
	selfCommand := processCommand(self, false)
	if selfCommand == "" {
		t.Skip("own argv unreadable on this host")
	}
	tag := selfCommand[strings.LastIndex(selfCommand, "/")+1:]
	if idx := strings.Index(tag, " "); idx > 0 {
		tag = tag[:idx]
	}
	if tag == "" {
		t.Skip("no usable self tag")
	}
	os.WriteFile(lease, []byte(`{"missionId":"mr-stale","pid":`+strconv.Itoa(self)+`,"pgid":1,"instanceTag":"`+tag+`","startedAt":"x","renewedAt":"x"}`), 0o644)
	err := engine.cleanupStaleLease()
	if err == nil || !strings.Contains(err.Error(), "already live") {
		t.Fatalf("a live holder was not refused: %v", err)
	}
	// A malformed lease file is an error, not a silent sweep.
	os.WriteFile(lease, []byte("{broken"), 0o644)
	if err := engine.cleanupStaleLease(); err == nil {
		t.Fatal("a malformed lease swept silently")
	}
}
