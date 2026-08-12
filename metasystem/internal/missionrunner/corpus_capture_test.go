package missionrunner

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

// The Phase 5.2 golden corpus for the missionrunner family (turn records,
// fence counters, mission state), captured from the CURRENT writer. This
// family's dialect is MarshalIndent: HTML IS escaped, unlike dispatch.
// Fixed inputs only; -capture-corpus re-records, default mode diffs.

var captureCorpus = flag.Bool("capture-corpus", false, "re-record the golden corpus")

var turnDocCorpus = []struct {
	name string
	doc  map[string]any
}{
	{"turn-pending", map[string]any{
		"missionId": "corpus-m", "turnId": "corpus-m-c1", "cycle": 1,
		"status": "pending", "outcome": nil, "error": nil, "detail": nil,
		"pid": nil, "endedAt": nil, "turnCapMin": 30,
	}},
	{"html-is-escaped-in-this-family", map[string]any{
		"missionId": "corpus-m", "detail": "exit 1 (a<b && c>d)", "status": "failed",
	}},
	{"fence-counters", map[string]any{
		"startedAt": "2026-08-12T00:00:00Z", "cycles": 3,
		"reservations": []any{"job-1", "job-2"},
	}},
	{"state-with-unknown-block", map[string]any{
		"missionId": "corpus-m", "status": "running",
		"integrity":     map[string]any{"hash": "abc", "chain": []any{"h1", "h2"}},
		"unknownFuture": map[string]any{"z": 1, "a": []any{nil, 0.5, "x<y"}},
	}},
}

func corpusFile(name string) string {
	return filepath.Join("testdata", "corpus", "turnstate", name+".json")
}

func TestTurnStateCorpus(t *testing.T) {
	for _, entry := range turnDocCorpus {
		target := filepath.Join(t.TempDir(), "doc.json")
		if err := atomicWriteJSON(target, entry.doc); err != nil {
			t.Fatalf("%s: %v", entry.name, err)
		}
		produced, _ := os.ReadFile(target)
		golden := corpusFile(entry.name)
		if *captureCorpus {
			os.MkdirAll(filepath.Dir(golden), 0o755)
			if err := os.WriteFile(golden, produced, 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		expected, err := os.ReadFile(golden)
		if err != nil {
			t.Fatalf("%s: corpus missing — run once with -capture-corpus: %v", entry.name, err)
		}
		if string(produced) != string(expected) {
			t.Fatalf("%s: writer bytes diverged\nproduced:\n%s\ncorpus:\n%s", entry.name, produced, expected)
		}
	}
}
