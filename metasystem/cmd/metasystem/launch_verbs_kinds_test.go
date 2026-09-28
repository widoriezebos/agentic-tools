package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

func launchRawString(value string) json.RawMessage {
	data, _ := json.Marshal(value)
	return data
}

func TestPageSizeIsRecordedAndReported(t *testing.T) {
	work, state, projects := t.TempDir(), t.TempDir(), t.TempDir()
	page := filepath.Join(work, "page.md")
	wantPage := []byte("one two\nthree\nlast")
	if err := os.WriteFile(page, wantPage, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(state, "result.json"), []byte(`{"session_id":"sid","is_error":false,"num_turns":3,"result":"tail one\ntail two"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	transcriptDir := filepath.Join(projects, "project")
	os.MkdirAll(transcriptDir, 0o700)
	os.WriteFile(filepath.Join(transcriptDir, "sid.jsonl"), nil, 0o600)
	record := launch.Record{ID: "page", Kind: "design", WorkingDirectory: work, AdapterData: map[string]json.RawMessage{"page": launchRawString("page.md")}}
	measurement, outputs, _, err := (launch.ClaudeHeadless{ProjectsRoot: projects}).Measure(record, state)
	if err != nil || measurement.PageLines != 2 || measurement.PageWords != 4 || measurement.Turns != 3 || len(outputs) != 1 {
		t.Fatalf("measurement=%+v outputs=%+v err=%v", measurement, outputs, err)
	}
	copied, err := os.ReadFile(outputs[0].Path)
	if err != nil || string(copied) != string(wantPage) || !strings.HasPrefix(outputs[0].Path, state+string(os.PathSeparator)) {
		t.Fatalf("copied=%q output=%+v err=%v", copied, outputs[0], err)
	}
	record.Measurement = measurement
	report := launchReport(record)
	if !strings.Contains(report, "page-lines=2 page-words=4") || !strings.Contains(report, "turns=3") || !strings.Contains(report, "compactions=0") {
		t.Fatalf("report=%q", report)
	}
	record.AdapterData["page"] = launchRawString("missing.md")
	measurement, _, _, err = (launch.ClaudeHeadless{ProjectsRoot: projects}).Measure(record, state)
	record.Measurement = measurement
	if err != nil || !measurement.PageMissing || !strings.Contains(launchReport(record), "page=missing") {
		t.Fatalf("missing measurement=%+v report=%q err=%v", measurement, launchReport(record), err)
	}
}
