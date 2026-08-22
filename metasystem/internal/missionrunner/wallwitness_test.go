package missionrunner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The taint ledger's mint hands back the id it booked, so the park path
// can witness the exact taint a human must later resolve.
func TestAppendTaintEntryReturnsItsId(t *testing.T) {
	state := map[string]any{
		"workspaceTaint": map[string]any{
			"next": float64(3), "segment": float64(0), "entries": []any{"a", "b"},
		},
	}
	id, err := appendTaintEntry(state, "t9", "solo build")
	if err != nil {
		t.Fatal(err)
	}
	if id != 3 {
		t.Fatalf("minted taint id %d, want 3", id)
	}
	taint := state["workspaceTaint"].(map[string]any)
	if got, _ := jsonInt(taint["next"]); got != 4 {
		t.Fatalf("next advanced to %d, want 4", got)
	}
}

// A certification naming no authorization digest is refused AND the
// refusal is witnessed in the recorder stream — silence here would hide
// exactly the evidence the wall exists to surface.
func TestAdjudicateCertifiedWitnessesAuthorizationRefusal(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "artifacts", "agents", "jobs"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeJSONDocFile(t, filepath.Join(root, "artifacts", "agents", "jobs", "job-1.json"),
		map[string]any{"jobId": "job-1", "role": "implementer", "mission": "m1"})
	state := map[string]any{"turnLog": []any{}}
	entries := []map[string]any{{"jobId": "job-1", "verdict": "accepted", "evidence": "e"}}

	certified, rejected, err := adjudicateCertified(root, "m1", state, entries)
	if err != nil {
		t.Fatal(err)
	}
	if len(certified) != 0 || len(rejected) != 1 {
		t.Fatalf("certified=%d rejected=%d, want 0/1", len(certified), len(rejected))
	}
	data, readErr := os.ReadFile(filepath.Join(root, "artifacts", "agents", "events.jsonl"))
	if readErr != nil {
		t.Fatalf("no recorder stream written: %v", readErr)
	}
	if !strings.Contains(string(data), `"event":"authorization-refused"`) {
		t.Fatalf("the refusal left no witness:\n%s", data)
	}
}

func writeJSONDocFile(t *testing.T, path string, doc map[string]any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, encoded, 0o644); err != nil {
		t.Fatal(err)
	}
}
