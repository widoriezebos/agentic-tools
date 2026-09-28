package mission

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// A fence ask a person answers names the public act that reseals the
// contract after an edit, never an internal verb.
func TestFenceAskNamesThePublicReseal(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	path, err := Refuse(repo, "demo", "job-cap-min")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var ask map[string]any
	if err := json.Unmarshal(data, &ask); err != nil {
		t.Fatal(err)
	}
	question, _ := ask["question"].(string)
	if !strings.Contains(question, "metasystem mission seal demo") || strings.Contains(question, "internal") || !strings.Contains(question, "plans/mission-demo.contract.md") {
		t.Fatalf("question %q", question)
	}
}
