package dispatch

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/wiredoc"
)

// The cross-writer proof: for every corpus case, the wiredoc envelope
// renders byte-identically to the current writer's committed golden bytes.
// This equivalence is what makes the conversion safe — when writeRecord
// later delegates to the envelope, this test is the reason nothing on disk
// can change.
func TestWiredocMatchesTheCorpus(t *testing.T) {
	for _, entry := range jobRecordCorpus {
		golden, err := os.ReadFile(corpusPath(entry.name))
		if err != nil {
			t.Fatalf("%s: corpus missing: %v", entry.name, err)
		}
		// The same input the current writer received, through the envelope:
		// decode the input map's canonical JSON (UseNumber, like every real
		// read), then render.
		seed, err := json.Marshal(entry.record)
		if err != nil {
			t.Fatal(err)
		}
		doc, err := wiredoc.Decode(seed)
		if err != nil {
			t.Fatalf("%s: envelope refused the input: %v", entry.name, err)
		}
		rendered, err := doc.Render()
		if err != nil {
			t.Fatal(err)
		}
		if string(rendered) != string(golden) {
			t.Fatalf("%s: the envelope's bytes diverge from the current writer's\nenvelope:\n%s\ngolden:\n%s",
				entry.name, rendered, golden)
		}
	}
}
