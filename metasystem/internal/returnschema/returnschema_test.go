package returnschema

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func v1Schema() map[string]any {
	return map[string]any{
		"title":    "Impl return",
		"required": []any{"notes"},
		"properties": map[string]any{
			"notes": map[string]any{"type": "string"},
			"model": map[string]any{
				"type":       "object",
				"properties": map[string]any{"requested": map[string]any{"type": "string"}},
			},
		},
	}
}

func TestVersionTwoAddsEnvelope(t *testing.T) {
	out, err := VersionTwo(v1Schema())
	if err != nil {
		t.Fatal(err)
	}
	if out["$comment"] != "metasystem.version=2" {
		t.Fatalf("missing version marker: %v", out["$comment"])
	}
	if out["title"] != "Impl return version 2" {
		t.Fatalf("title not versioned: %v", out["title"])
	}
	required := out["required"].([]any)
	if len(required) != 3 || required[0] != "schemaVersion" || required[1] != "claimed" || required[2] != "notes" {
		t.Fatalf("required must prepend schemaVersion and claimed: %v", required)
	}
	props := out["properties"].(map[string]any)
	sv := props["schemaVersion"].(map[string]any)
	if sv["type"] != "integer" {
		t.Fatalf("schemaVersion must be typed integer: %v", sv)
	}
	if _, ok := props["claimed"].(map[string]any); !ok {
		t.Fatal("claimed member must be added")
	}
	if props["sessionId"].(map[string]any)["type"] != "string" {
		t.Fatalf("sessionId must be added: %v", props["sessionId"])
	}
	modelProps := props["model"].(map[string]any)["properties"].(map[string]any)
	if _, ok := modelProps["effective"].(map[string]any); !ok {
		t.Fatal("model.effective must be added")
	}
}

func TestMaterializeV1AndV2(t *testing.T) {
	root := t.TempDir()
	schemaDir := filepath.Join(root, "scripts/agents/schemas")
	if err := os.MkdirAll(schemaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(v1Schema())
	if err := os.WriteFile(filepath.Join(schemaDir, "implementer.schema.json"), body, 0o644); err != nil {
		t.Fatal(err)
	}

	// v1 is materialized unchanged (no version marker).
	v1Out := filepath.Join(root, "v1.json")
	if err := Materialize(root, "implementer", 1, v1Out); err != nil {
		t.Fatal(err)
	}
	var v1 map[string]any
	data, _ := os.ReadFile(v1Out)
	_ = json.Unmarshal(data, &v1)
	if _, ok := v1["$comment"]; ok {
		t.Fatal("v1 must not carry the v2 marker")
	}

	// v2 is transformed.
	v2Out := filepath.Join(root, "v2.json")
	if err := Materialize(root, "implementer", 2, v2Out); err != nil {
		t.Fatal(err)
	}
	var v2 map[string]any
	data, _ = os.ReadFile(v2Out)
	_ = json.Unmarshal(data, &v2)
	if v2["$comment"] != "metasystem.version=2" {
		t.Fatalf("v2 output missing the marker: %v", v2["$comment"])
	}
}
