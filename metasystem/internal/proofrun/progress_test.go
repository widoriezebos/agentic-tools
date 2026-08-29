package proofrun

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSectionProgressAcceptsSelectorAndDeclaredRepeat(t *testing.T) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	run := ProgressRun{
		Header: ProgressHeader{TmpPaths: []string{"/tmp/run"}, LogPaths: []string{"/tmp/run.log"}},
		Events: []SectionEvent{
			{Suite: "validate", Section: "first", Event: "start", At: now},
			{Suite: "validate", Section: "first", Event: "end", At: now},
			{Suite: "validate", Section: "repeat", Event: "start", At: now},
			{Suite: "validate", Section: "repeat", Event: "end", At: now},
			{Suite: "validate", Section: "repeat", Event: "start", At: now},
			{Suite: "validate", Section: "repeat", Event: "end", At: now},
		},
	}
	if err := AssertSectionProgress(run, "validate", []string{"first", "repeat"}, map[string]bool{"repeat": true}); err != nil {
		t.Fatal(err)
	}
}

func TestSectionProgressRejectsSilentMisorderedAndUndeclaredRepeat(t *testing.T) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	cases := []struct {
		name   string
		events []SectionEvent
		want   string
	}{
		{"silent", nil, "silent has 0 starts and 0 ends"},
		{"end first", []SectionEvent{{Suite: "validate", Section: "silent", Event: "end", At: now}}, "ended without a preceding start"},
		{"repeat", []SectionEvent{
			{Suite: "validate", Section: "silent", Event: "start", At: now},
			{Suite: "validate", Section: "silent", Event: "end", At: now},
			{Suite: "validate", Section: "silent", Event: "start", At: now},
			{Suite: "validate", Section: "silent", Event: "end", At: now},
		}, "expected 1 of each"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := AssertSectionProgress(ProgressRun{Events: tc.events}, "validate", []string{"silent"}, nil)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want text %q", err, tc.want)
			}
		})
	}
}

func TestLatestProgressRunStartsAtLastHeader(t *testing.T) {
	path := filepath.Join(t.TempDir(), "progress.jsonl")
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err := AppendProgressHeader(path, ProgressHeader{LogPaths: []string{"old.log"}}); err != nil {
		t.Fatal(err)
	}
	if err := AppendSectionEvent(path, SectionEvent{Suite: "old", Section: "old", Event: "start", At: now}); err != nil {
		t.Fatal(err)
	}
	if err := AppendProgressHeader(path, ProgressHeader{TmpPaths: []string{"new.tmp"}, LogPaths: []string{"new.log"}}); err != nil {
		t.Fatal(err)
	}
	if err := AppendSectionEvent(path, SectionEvent{Suite: "new", Section: "new", Event: "start", At: now}); err != nil {
		t.Fatal(err)
	}
	run, err := ReadLatestProgressRun(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(run.Events) != 1 || run.Events[0].Suite != "new" || run.Header.LogPaths[0] != "new.log" {
		t.Fatalf("latest run = %+v", run)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("journal permissions = %v, %v", info.Mode().Perm(), err)
	}
}

func TestDeepestLiveHeartbeatTracksNestedOpenSection(t *testing.T) {
	now := time.Date(2026, 8, 29, 10, 5, 0, 0, time.UTC)
	run := ProgressRun{Events: []SectionEvent{
		{Suite: "validate", Section: "adoption", Event: "start", At: now.Add(-5 * time.Minute).Format(time.RFC3339Nano), Depth: 0},
		{Suite: "adopt", Section: "nested", Event: "start", At: now.Add(-2 * time.Minute).Format(time.RFC3339Nano), Depth: 1},
	}}
	if got, ok := DeepestLiveHeartbeat(run, now); !ok || got != "adopt:nested since 2min" {
		t.Fatalf("heartbeat = %q, %v", got, ok)
	}

	run.Events = append(run.Events,
		SectionEvent{Suite: "adopt", Section: "nested", Event: "end", At: now.Format(time.RFC3339Nano), Depth: 1})
	if got, ok := DeepestLiveHeartbeat(run, now); !ok || got != "validate:adoption since 5min" {
		t.Fatalf("parent heartbeat = %q, %v", got, ok)
	}

	run.Events = append(run.Events,
		SectionEvent{Suite: "validate", Section: "adoption", Event: "end", At: now.Format(time.RFC3339Nano), Depth: 0})
	if got, ok := DeepestLiveHeartbeat(run, now); ok || got != "" {
		t.Fatalf("closed heartbeat = %q, %v", got, ok)
	}
}
