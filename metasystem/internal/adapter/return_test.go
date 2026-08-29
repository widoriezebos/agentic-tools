package adapter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func normalizeInto(t *testing.T, dir, candidate, transcript, record, session string) (map[string]any, error) {
	t.Helper()
	candidatePath := ""
	if candidate != "" {
		candidatePath = filepath.Join(dir, "raw.out")
		writeFile(t, candidatePath, candidate)
	}
	transcriptPath := ""
	if transcript != "" {
		transcriptPath = filepath.Join(dir, "transcript.jsonl")
		writeFile(t, transcriptPath, transcript)
	}
	recordPath := filepath.Join(dir, "job.json")
	writeFile(t, recordPath, record)
	output := filepath.Join(dir, "return.json")
	markdown := filepath.Join(dir, "return.md")
	err := NormalizeReturn(candidatePath, transcriptPath, recordPath, output, markdown, session)
	if err != nil {
		return nil, err
	}
	return readJSONFile(t, output), nil
}

const v2Reply = `Some prose before the return.

` + "```json" + `
{
  "schemaVersion": 2,
  "jobId": "job-1",
  "round": 1,
  "runtime": "codex",
  "sessionId": "claimed-session",
  "model": {"requested": "m1", "effective": "claimed-model"},
  "evidence": [],
  "gaps": [],
  "mode": "design"
}
` + "```" + `
And prose after.`

func TestNormalizeReturnReconcilesObservedIdentity(t *testing.T) {
	dir := t.TempDir()
	result, err := normalizeInto(t, dir, v2Reply, "",
		`{"effectiveModel": "observed-model"}`, "observed-session")
	if err != nil {
		t.Fatal(err)
	}
	if result["sessionId"] != "observed-session" {
		t.Fatalf("sessionId = %v", result["sessionId"])
	}
	model := result["model"].(map[string]any)
	if model["effective"] != "observed-model" || model["requested"] != "m1" {
		t.Fatalf("model = %v", model)
	}
	// The delegate's differing claims are preserved as claims, never as facts.
	claimed := result["claimed"].(map[string]any)
	if claimed["sessionId"] != "claimed-session" || claimed["model"] != "claimed-model" {
		t.Fatalf("claimed = %v", claimed)
	}
	markdown, err := os.ReadFile(filepath.Join(dir, "return.md"))
	if err != nil || !strings.Contains(string(markdown), "Canonical JSON: return.json") {
		t.Fatalf("return.md = %q, %v", markdown, err)
	}
}

func TestNormalizeReturnClaimsNothingWhenIdentityMatches(t *testing.T) {
	reply := `{"schemaVersion": 2, "jobId": "j", "round": 1, "runtime": "codex",
	  "sessionId": "s1", "model": {"effective": "m1"}, "evidence": [], "gaps": [], "mode": "design"}`
	result, err := normalizeInto(t, t.TempDir(), reply, "", `{"effectiveModel": "m1"}`, "s1")
	if err != nil {
		t.Fatal(err)
	}
	// Both members present, both null: this family's way of claiming nothing.
	claimed, ok := result["claimed"].(map[string]any)
	if !ok {
		t.Fatalf("claimed missing: %v", result)
	}
	if session, present := claimed["sessionId"]; !present || session != nil {
		t.Fatalf("claimed.sessionId = %v", claimed)
	}
	if model, present := claimed["model"]; !present || model != nil {
		t.Fatalf("claimed.model = %v", claimed)
	}
}

func TestNormalizeVersionThreeReturnClaimsNothingWhenIdentityMatches(t *testing.T) {
	reply := `{"schemaVersion": 3, "jobId": "j", "round": 1, "runtime": "codex",
	  "sessionId": "s1", "model": {"effective": "m1"}, "evidence": [], "gaps": [],
	  "mode": "design", "rigor": []}`
	result, err := normalizeInto(t, t.TempDir(), reply, "", `{"effectiveModel": "m1"}`, "s1")
	if err != nil {
		t.Fatal(err)
	}
	claimed, ok := result["claimed"].(map[string]any)
	if !ok || claimed["sessionId"] != nil || claimed["model"] != nil {
		t.Fatalf("version-three claimed envelope is wrong: %v", result["claimed"])
	}
}

func TestNormalizeReturnFindsReturnInsideWrapperResultString(t *testing.T) {
	// The common wrapper shape: the reply is a JSON envelope whose result
	// member is the return object serialized as a string.
	wrapper := `{"type": "result", "result": "{\"schemaVersion\": 2, \"jobId\": \"j\", \"round\": 1, \"runtime\": \"claude\", \"sessionId\": \"x\", \"model\": {\"effective\": \"y\"}, \"evidence\": [], \"gaps\": [], \"mode\": \"design\"}"}`
	result, err := normalizeInto(t, t.TempDir(), wrapper, "", `{"effectiveModel": "real"}`, "real-session")
	if err != nil {
		t.Fatal(err)
	}
	if result["jobId"] != "j" || result["sessionId"] != "real-session" {
		t.Fatalf("result = %v", result)
	}
}

func TestNormalizeReturnPrefersTheFullerObject(t *testing.T) {
	// A partial echo of the contract must lose to the complete return, even
	// when the partial one appears first.
	partial := `{"jobId": "j", "round": 1}`
	full := `{"jobId": "j", "round": 1, "runtime": "codex", "sessionId": "s",
	  "model": "m", "evidence": [], "gaps": [], "mode": "design"}`
	result, err := normalizeInto(t, t.TempDir(), partial+"\n"+full, "", `{}`, "s")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := result["evidence"]; !ok {
		t.Fatalf("picked the partial object: %v", result)
	}
	// A string model is an older shape: identity passes through untouched.
	if result["model"] != "m" || result["sessionId"] != "s" {
		t.Fatalf("older shape was rewritten: %v", result)
	}
}

func TestNormalizeReturnSearchesTheTranscriptToo(t *testing.T) {
	transcript := `{"event": "turn"}
{"event": "message", "structured_output": {"jobId": "j", "round": 2, "runtime": "codex", "sessionId": "s", "model": "m", "evidence": [], "gaps": [], "mode": "design"}}`
	result, err := normalizeInto(t, t.TempDir(), "no json here", transcript, `{}`, "s")
	if err != nil {
		t.Fatal(err)
	}
	if result["jobId"] != "j" {
		t.Fatalf("result = %v", result)
	}
}

func TestNormalizeReturnFailsLoudlyWithoutAReturnObject(t *testing.T) {
	_, err := normalizeInto(t, t.TempDir(), "just prose, no return", "", `{}`, "s")
	if err == nil || !strings.Contains(err.Error(), "no JSON return object found in runtime output") {
		t.Fatalf("err = %v", err)
	}
}
