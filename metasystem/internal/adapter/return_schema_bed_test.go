package adapter

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/returnschema"
)

// The adapter legs of scripts/agents/return-schema-fixtures.sh, ported with
// the adapters (verbs-object-action U6a): the implementer-v1-v2 scenario's
// normalize_return legs through the real normalization owner, and the
// critic-v3 scenario's fake critic return, each validated against the
// shipped role schema.

func sourceRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func readReturnObject(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]any
	if err := json.Unmarshal(data, &object); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return object
}

func writeJSONFile(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestReturnSchemaBedNormalizeAdoptsObservedIdentity(t *testing.T) {
	t.Parallel()
	root := sourceRoot(t)
	dir := t.TempDir()
	record := filepath.Join(dir, "record.json")
	writeJSONFile(t, record, map[string]any{"effectiveModel": "observed-model"})
	base := func() map[string]any {
		return map[string]any{
			"schemaVersion": 2, "jobId": "fixture-job", "round": 1, "runtime": "fake",
			"sessionId": "claimed-session",
			"model":     map[string]any{"requested": "requested-model", "effective": "claimed-model"},
			"claimed":   map[string]any{"model": "earlier-claim"},
			"evidence":  []any{}, "gaps": []any{}, "mode": "implement", "riskiestPart": "fixture",
			"diffBoundary": []any{}, "whatWasDone": "fixture",
		}
	}
	candidate := filepath.Join(dir, "candidate.json")
	writeJSONFile(t, candidate, base())
	output, markdown := filepath.Join(dir, "return.json"), filepath.Join(dir, "return.md")
	if err := NormalizeReturn(candidate, "", record, output, markdown, "observed-session"); err != nil {
		t.Fatal(err)
	}
	normalized := readReturnObject(t, output)
	if normalized["schemaVersion"] != float64(2) {
		t.Fatalf("normalized return lost its schema version: %v", normalized)
	}
	if normalized["sessionId"] != "observed-session" {
		t.Fatalf("normalized return did not adopt the observed session: %v", normalized)
	}
	if model, _ := normalized["model"].(map[string]any); model["effective"] != "observed-model" {
		t.Fatalf("normalized return did not adopt the record's observed model: %v", normalized)
	}
	if want := map[string]any{"sessionId": "claimed-session", "model": "claimed-model"}; !reflect.DeepEqual(normalized["claimed"], want) {
		t.Fatalf("normalized return did not preserve both claims: %v", normalized["claimed"])
	}
	if violations := returnschema.ReturnCompleteRole(root, "implementer", output); len(violations) != 0 {
		t.Fatalf("normalized return failed the implementer schema: %v", violations)
	}

	// A claim on ONE member and agreement on the other still carries both
	// keys: structured output rejects an object schema that leaves any
	// property out of `required`.
	oneClaim := base()
	oneClaim["sessionId"] = "observed-session"
	delete(oneClaim, "claimed")
	writeJSONFile(t, candidate, oneClaim)
	if err := NormalizeReturn(candidate, "", record, output, markdown, "observed-session"); err != nil {
		t.Fatal(err)
	}
	if want := map[string]any{"sessionId": nil, "model": "claimed-model"}; !reflect.DeepEqual(readReturnObject(t, output)["claimed"], want) {
		t.Fatalf("a claim on one member did not keep both claimed keys: %v", readReturnObject(t, output)["claimed"])
	}
	if violations := returnschema.ReturnCompleteRole(root, "implementer", output); len(violations) != 0 {
		t.Fatalf("one-claim return failed the implementer schema: %v", violations)
	}
}

func TestReturnSchemaBedFakeCriticSpeaksVersionThree(t *testing.T) {
	t.Parallel()
	root := sourceRoot(t)
	dir := t.TempDir()
	record := filepath.Join(dir, "record.json")
	writeJSONFile(t, record, map[string]any{
		"jobId": "fake-critic-v3", "round": 1, "role": "code-critic", "sessionId": "fake-session",
		"requestedModel": "fake-model", "effectiveModel": "fake-model",
	})
	prompt := filepath.Join(dir, "prompt.md")
	if err := os.WriteFile(prompt, []byte("Working Mode: critique\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(dir, "return.json")
	if err := WriteFakeReturn(record, prompt, output); err != nil {
		t.Fatal(err)
	}
	if violations := returnschema.ReturnCompleteRole(root, "code-critic", output); len(violations) != 0 {
		t.Fatalf("fake critic return failed the code-critic schema: %v", violations)
	}
	object := readReturnObject(t, output)
	if object["schemaVersion"] != float64(3) {
		t.Fatalf("fake critic did not speak return schema version 3: %v", object["schemaVersion"])
	}
	if rigor, ok := object["rigor"].([]any); !ok || len(rigor) != 0 {
		t.Fatalf("zero-finding fake critic did not emit empty rigor: %v", object["rigor"])
	}
}
