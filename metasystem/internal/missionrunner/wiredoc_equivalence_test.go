package missionrunner

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/wiredoc"
)

// TD-1 for the missionrunner family: RenderEscaped must produce bytes
// identical to the current writer's committed corpus. This family's wire
// dialect is MarshalIndent — HTML escaped — and the envelope must carry
// that dialect, not dispatch's unescaped one.
func TestWiredocEscapedMatchesTheCorpus(t *testing.T) {
	for _, entry := range turnDocCorpus {
		golden, err := os.ReadFile(corpusFile(entry.name))
		if err != nil {
			t.Fatalf("%s: corpus missing: %v", entry.name, err)
		}
		seed, err := json.Marshal(entry.doc)
		if err != nil {
			t.Fatal(err)
		}
		doc, err := wiredoc.Decode(seed)
		if err != nil {
			t.Fatalf("%s: %v", entry.name, err)
		}
		rendered, err := doc.RenderEscaped()
		if err != nil {
			t.Fatal(err)
		}
		if string(rendered) != string(golden) {
			t.Fatalf("%s: escaped-dialect bytes diverge\nenvelope:\n%s\ngolden:\n%s", entry.name, rendered, golden)
		}
	}
}

// The frozen grammar for this family's reader: decodeJSONDoc's verdicts,
// including the errNotJSONObject distinction readDocLabeled words
// differently from unreadable bytes (E1).
func TestTurnStateGrammarFrozen(t *testing.T) {
	if doc, err := decodeJSONDoc([]byte(`{"a":"first","a":"second"}`)); err != nil || doc["a"] != "second" {
		t.Fatalf("duplicate keys: %v %v", doc, err)
	}
	if doc, err := decodeJSONDoc([]byte(`{"kept":1}` + "\ntrailing")); err != nil || doc["kept"] == nil {
		t.Fatalf("trailing tolerance: %v %v", doc, err)
	}
	if _, err := decodeJSONDoc([]byte(`["array"]`)); err != errNotJSONObject {
		t.Fatalf("non-object must be THE named error: %v", err)
	}
	if _, err := decodeJSONDoc([]byte(``)); err == nil || err == errNotJSONObject {
		t.Fatalf("empty input is unreadable, not not-an-object: %v", err)
	}
	if doc, err := decodeJSONDoc([]byte(`{"n":1.0}`)); err != nil {
		t.Fatal(err)
	} else if literal, ok := doc["n"].(json.Number); !ok || literal.String() != "1.0" {
		t.Fatalf("number spelling lost: %v", doc["n"])
	}
}
