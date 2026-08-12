package host

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/wiredoc"
)

// The Phase 5.3 golden corpus for host results: this family's dialect is
// the unescaped canon (canonicalJSON), like dispatch and unlike
// missionrunner.

var captureCorpus = flag.Bool("capture-corpus", false, "re-record the golden corpus")

var hostResultCorpus = []struct {
	name string
	doc  map[string]any
}{
	{"result-minimal", map[string]any{
		"turnId": "corpus-t1", "outcome": "completed", "error": nil,
	}},
	{"html-unescaped-in-this-family", map[string]any{
		"turnId": "corpus-t2", "detail": "run a<b && c>d & exit",
	}},
	{"unknown-nested", map[string]any{
		"turnId":   "corpus-t3",
		"envelope": map[string]any{"z": 1, "a": []any{nil, "x", 0.5}},
	}},
}

func corpusFile(name string) string {
	return filepath.Join("testdata", "corpus", "hostresult", name+".json")
}

func TestHostResultCorpus(t *testing.T) {
	for _, entry := range hostResultCorpus {
		target := filepath.Join(t.TempDir(), "result.json")
		if err := atomicWriteJSON(target, entry.doc); err != nil {
			t.Fatalf("%s: %v", entry.name, err)
		}
		produced, _ := os.ReadFile(target)
		golden := corpusFile(entry.name)
		if *captureCorpus {
			os.MkdirAll(filepath.Dir(golden), 0o755)
			os.WriteFile(golden, produced, 0o644)
			continue
		}
		expected, err := os.ReadFile(golden)
		if err != nil {
			t.Fatalf("%s: corpus missing: %v", entry.name, err)
		}
		if string(produced) != string(expected) {
			t.Fatalf("%s: bytes diverged\nproduced:\n%s\ngolden:\n%s", entry.name, produced, expected)
		}
	}
}

// TD-1 for this family: the envelope's unescaped Render matches the corpus.
func TestWiredocMatchesHostCorpus(t *testing.T) {
	for _, entry := range hostResultCorpus {
		golden, err := os.ReadFile(corpusFile(entry.name))
		if err != nil {
			t.Fatalf("%s: corpus missing: %v", entry.name, err)
		}
		seed, _ := json.Marshal(entry.doc)
		doc, err := wiredoc.Decode(seed)
		if err != nil {
			t.Fatal(err)
		}
		rendered, err := doc.Render()
		if err != nil {
			t.Fatal(err)
		}
		if string(rendered) != string(golden) {
			t.Fatalf("%s: envelope bytes diverge\nenvelope:\n%s\ngolden:\n%s", entry.name, rendered, golden)
		}
	}
}
