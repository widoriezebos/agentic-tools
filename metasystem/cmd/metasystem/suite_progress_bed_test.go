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
	note, err := os.ReadFile(filepath.Join(result, "copy-note.txt"))
	if err != nil || !strings.Contains(string(note), "DROPPED "+evidence) || !strings.Contains(string(note), "copied-bytes=4\n") {
		t.Fatalf("bounded evidence note did not name the dropped source or the retained bytes: %q, %v", note, err)
	}
}

// deepest-heartbeat and background watcher: the public heartbeat verb and
// the background job watcher read the same journal view and both relay the
// deepest open section; the watcher prefixes its one reportable job note.
func TestSuiteProgressBedHeartbeatAndWatcherRelayTheDeepestSection(t *testing.T) {
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

	code, heartbeat, _ := captureCommandOutput(t, true, false, func() int {
		return runProofRunHeartbeat([]string{"--root", workspace})
	})
	if code != 0 || !regexp.MustCompile(`^inner:child since [0-9]+min\n$`).MatchString(heartbeat) {
		t.Fatalf("deepest live heartbeat was not selected: code=%d output=%q", code, heartbeat)
	}

	installation := t.TempDir()
	if err := os.WriteFile(filepath.Join(installation, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	jobs := filepath.Join(t.TempDir(), "jobs")
	if err := os.MkdirAll(jobs, 0o700); err != nil {
		t.Fatal(err)
	}
	record := filepath.Join(jobs, "prefix-job.json")
	if err := os.WriteFile(record, []byte(`{"status":"completed","workspaceRoot":"`+workspace+`"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	state := filepath.Join(t.TempDir(), "watch.state")
	if err := os.WriteFile(state, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	code, watched, _ := captureCommandOutput(t, true, false, func() int {
		return runReportWatchJobs([]string{"--root", installation, "--dir", jobs, "--scope", workspace, "--state", state, "--once"})
	})
	note := regexp.MustCompile(`(?m)^inner:child since [0-9]+min DONE prefix-job status=completed age=[0-9]+m record=` + regexp.QuoteMeta(record) + `$`)
	if code != 0 || len(note.FindAllString(watched, -1)) != 1 || strings.Count(watched, "DONE prefix-job") != 1 {
		t.Fatalf("background watcher did not emit exactly one complete deepest-heartbeat job note: code=%d\n%s", code, watched)
	}
}
