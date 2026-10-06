package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestAcceptRiskPlaceholderThroughPublicVerb(t *testing.T) {
	t.Parallel()
	bed := newIntentBed(t, false, nil)
	writeCriticChain(t, bed.root())
	path := filepath.Join(bed.root(), "artifacts", "agents", "jobs", "critic.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var record map[string]any
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatal(err)
	}
	entry := record["findingRegister"].([]any)[0].(map[string]any)
	entry["rigorClass"], entry["grain"], entry["fixture"], entry["placeholderRound"] = "unproven", "invariant", "", 1
	writeTemp(t, filepath.Dir(path), "critic.json", record)
	code, result := bed.runJSON(bed.owners(), "goal", "accept-risk", bedGoal, "--finding", "S-1", "--review", "critic-r2", "--reason", "the provider ended this round")
	if code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("public accept-risk: exit %d %+v", code, result)
	}
	data, err = os.ReadFile(path)
	if err != nil || json.Unmarshal(data, &record) != nil {
		t.Fatalf("accepted register: %v", err)
	}
	entry = record["findingRegister"].([]any)[0].(map[string]any)
	if entry["status"] != "accepted-risk" || entry["resolution"] != "accepted-risk" || entry["placeholderRound"] != float64(1) || entry["decisionOpid"] == "" {
		t.Fatalf("the public verb did not stamp the placeholder: %v", entry)
	}
}
