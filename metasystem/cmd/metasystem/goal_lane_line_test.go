package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

func TestGoalNextPrintsTheLaneLine(t *testing.T) {
	t.Parallel()
	home, root := t.TempDir(), t.TempDir()
	record := lane.Record{Root: root, Install: root, CustodyEpoch: 1}
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(lane.HostDir(home), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lane.RecordPath(home), data, 0600); err != nil {
		t.Fatal(err)
	}
	deps := syncRequestDependencies{laneHome: func() (string, error) { return home, nil }}
	now := time.Date(2026, 10, 25, 14, 31, 0, 0, time.UTC)
	line := plain.StewardLine{At: now.Add(-5 * time.Minute), Line: "lane held; overdue-goal", Rows: []plain.Fired{{Row: "overdue-goal"}}, Identity: "identity"}
	for _, mode := range []string{"row", "report", "quiet", "missing", "unreadable"} {
		if mode == "report" {
			line.Rows = nil
			line.Report.At, line.Report.Summary = now, "hour report"
		}
		if mode == "quiet" {
			line.Report.At = now.Add(-2 * time.Hour)
		}
		data, err := json.Marshal(line)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(plain.Dir(root), 0700); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(plain.Dir(root), "steward-line.json")
		if mode == "unreadable" {
			data = []byte("{")
		}
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		if mode == "missing" {
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
		}
		var out, stderr strings.Builder
		if code := runGoalNextWithInputs([]string{"--root", root}, deps, func(string) (time.Time, error) { return now, nil }, &out, &stderr); code != 0 {
			t.Fatalf("code=%d err=%s", code, stderr.String())
		}
		text := out.String()
		if (mode == "row" || mode == "report") && !strings.Contains(text, "lane: lane held; overdue-goal (5 min ago)") || mode == "quiet" && strings.Contains(text, "lane:") || (mode == "missing" || mode == "unreadable") && !strings.Contains(text, "lane line unreadable:") {
			t.Fatalf("%s: %s", mode, text)
		}
	}
}
