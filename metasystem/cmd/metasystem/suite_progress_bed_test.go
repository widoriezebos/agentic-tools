package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

// These tests port the command-level scenarios of
// scripts/agents/suite-progress-fixtures.sh. Each drives the owning Go verb
// in-process; the watchdog and launcher scenarios live in internal/proofrun.

// bounded-preserve: the bounded copier names the exact truncated source in
// its loud result and in its durable copy note, keeping the bytes that fit.
func TestSuiteProgressBedBoundedPreserveNamesTheTruncatedSource(t *testing.T) {
	t.Parallel()
	bed := t.TempDir()
	source := filepath.Join(bed, "source")
	if err := os.MkdirAll(source, 0o700); err != nil {
		t.Fatal(err)
	}
	evidence := filepath.Join(source, "evidence")
	if err := os.WriteFile(evidence, []byte("eight-bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	result := filepath.Join(bed, "result")
	code, stdout, _ := captureCommandOutput(t, true, false, func() int {
		return runProofRunPreserve([]string{"--destination", result, "--max-bytes", "4", "--source", source})
	})
	if code != 0 || !strings.Contains(stdout, "DROPPED "+evidence) {
		t.Fatalf("bounded evidence result code=%d did not name the dropped source %s:\n%s", code, evidence, stdout)
	}
	if !strings.Contains(stdout, proofrun.TruncationMarker(4)) {
		t.Fatalf("bounded evidence result did not carry the truncation marker:\n%s", stdout)
	}
	note, err := os.ReadFile(filepath.Join(result, "copy-note.txt"))
	if err != nil || !strings.Contains(string(note), "DROPPED "+evidence) || !strings.Contains(string(note), "copied-bytes=4\n") ||
		!strings.Contains(string(note), proofrun.TruncationMarker(4)) {
		t.Fatalf("bounded evidence note did not name the dropped source or the retained bytes: %q, %v", note, err)
	}
}

// deepest-heartbeat: the suite heartbeat reads the journal view and relays
// the deepest open section.
func TestSuiteProgressBedHeartbeatRelaysTheDeepestSection(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	journal := filepath.Join(workspace, "artifacts", "agents", "supervision", "suite-progress.jsonl")
	if err := proofrun.AppendProgressHeader(journal, proofrun.ProgressHeader{LogPaths: []string{"suite.log"}}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, event := range []proofrun.SectionEvent{
		{Suite: "outer", Section: "parent", Event: "start", At: now, Depth: 0},
		{Suite: "inner", Section: "child", Event: "start", At: now, Depth: 1},
	} {
		if err := proofrun.AppendSectionEvent(journal, event); err != nil {
			t.Fatal(err)
		}
	}

	// The deepest live section is the heartbeat's owner (the internal
	// proof-run heartbeat printed exactly this line).
	heartbeat, found := deepestSuiteHeartbeat(workspace, time.Now())
	if !found || !regexp.MustCompile(`^inner:child since [0-9]+min$`).MatchString(heartbeat) {
		t.Fatalf("deepest live heartbeat was not selected: found=%v heartbeat=%q", found, heartbeat)
	}
}
