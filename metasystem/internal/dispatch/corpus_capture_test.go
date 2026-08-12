package dispatch

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

// The Phase 5.1 golden corpus (plans/typed-documents-design.md, TD-2): each
// case drives the CURRENT writer and commits the exact bytes it produced.
// Run with -capture-corpus to (re)record; without the flag the test DIFFS
// the writer's output against the committed corpus, so any conversion that
// changes a byte fails here before it can ship.
//
// Inputs are fixed values only — no clocks, no randomness — so the corpus
// is stable across machines and time.

var captureCorpus = flag.Bool("capture-corpus", false, "re-record the golden corpus from the current writer")

var jobRecordCorpus = []struct {
	name   string
	record map[string]any
}{
	{"minimal-pending", map[string]any{
		"jobId": "corpus-min", "status": "pending-setup", "phase": "setup",
		"role": "implementer", "error": nil,
	}},
	{"html-survives-unescaped", map[string]any{
		"jobId": "corpus-html", "status": "running",
		"brief": "compare a<b && b>c & echo", "error": nil,
	}},
	{"number-spellings", map[string]any{
		"jobId": "corpus-num", "status": "running",
		"capMin": 30, "fraction": 0.5, "big": 1786541185,
		"exponent": 1e6, "negative": -7,
	}},
	{"null-vs-absent", map[string]any{
		"jobId": "corpus-null", "status": "running",
		"present-null": nil, "empty-string": "", "zero": 0,
	}},
	{"unknown-nested-structure", map[string]any{
		"jobId": "corpus-nested", "status": "running",
		"unknownBlock": map[string]any{
			"deep": map[string]any{"z-last": 1, "a-first": 2},
			"list": []any{"one", 2, nil, map[string]any{"k": "v"}},
		},
	}},
	{"terminal-with-metadata", map[string]any{
		"jobId": "corpus-terminal", "status": "completed",
		"endedAt": "2026-08-12T00:00:00Z",
		"mirror":  map[string]any{"sha256": "abc123", "path": "evidence/x"},
	}},
	{"key-sorting-adversarial", map[string]any{
		"jobId": "corpus-sort", "status": "running",
		"zz": 1, "aa": 2, "Zcap": 3, "acap": 4, "_underscore": 5,
	}},
}

func corpusPath(name string) string {
	return filepath.Join("testdata", "corpus", "jobrecord", name+".json")
}

func TestJobRecordCorpus(t *testing.T) {
	for _, entry := range jobRecordCorpus {
		target := filepath.Join(t.TempDir(), "record.json")
		if err := writeRecord(target, entry.record); err != nil {
			t.Fatalf("%s: write failed: %v", entry.name, err)
		}
		produced, err := os.ReadFile(target)
		if err != nil {
			t.Fatal(err)
		}
		golden := corpusPath(entry.name)
		if *captureCorpus {
			if err := os.WriteFile(golden, produced, 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		expected, err := os.ReadFile(golden)
		if err != nil {
			t.Fatalf("%s: corpus file missing — run once with -capture-corpus: %v", entry.name, err)
		}
		if string(produced) != string(expected) {
			t.Fatalf("%s: the writer's bytes diverged from the committed corpus\nproduced:\n%s\ncorpus:\n%s",
				entry.name, produced, expected)
		}
	}
}
