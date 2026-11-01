package humanauthority

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestADecisionLineIsDurableAndReadBack(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	if got, bad, err := ReadDecisions(home); err != nil || len(got) != 0 || bad != 0 {
		t.Fatalf("missing log: %v, %d, %v", got, bad, err)
	}
	first := Decision{ID: "first", At: time.Date(2026, 9, 30, 8, 0, 0, 0, time.UTC), Actor: "main-1", Lineage: "seat", Machine: "m1e", Grant: "grant-1", By: "Wido", Class: "ordering", Text: "G first", Rejected: "leave G last", Context: "queue", Undo: "goal prioritize G 2", Argv: []string{"goal", "prioritize", "G", "3"}, Outcome: "confirmed", Words: "G prioritized", Effect: json.RawMessage(`{"owner":{"tip":"abc123","detail":"published"}}`)}
	second := first
	second.ID, second.At, second.Outcome, second.Effect = "second", first.At.Add(time.Minute), "unchanged", json.RawMessage(`{"targets":[]}`)
	want := []Decision{first, second}
	for _, d := range want {
		if err := AppendDecision(home, d); err != nil {
			t.Fatal(err)
		}
	}
	if got, bad, err := ReadDecisions(home); err != nil || bad != 0 || !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip: %+v, %d, %v; want %+v", got, bad, err, want)
	}
	path := DecisionLogPath(home)
	if path != filepath.Join(home, "artifacts", "agents", "authority", "decisions.jsonl") {
		t.Fatalf("log path: %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil || bytes.Count(data, []byte("\n")) != 2 {
		t.Fatalf("two appended lines: %q, %v", data, err)
	}
	torn := bytes.Replace(data, []byte("\n"), []byte("\n{torn\n"), 1)
	if err := os.WriteFile(path, torn, 0o600); err != nil {
		t.Fatal(err)
	}
	if got, bad, err := ReadDecisions(home); err != nil || bad != 1 || !reflect.DeepEqual(got, want) {
		t.Fatalf("torn line: %+v, %d, %v", got, bad, err)
	}
	if err := AppendAttorneyLog(home, "answered grant=grant-1"); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(AttorneyLogPath(home)); err != nil || string(got) != "answered grant=grant-1\n" {
		t.Fatalf("attorney log: %q, %v", got, err)
	}
}
