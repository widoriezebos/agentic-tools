package returnschema

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

func TestVersionThreeAddsCriticRigorRows(t *testing.T) {
	out, err := VersionThree(v1Schema())
	if err != nil {
		t.Fatal(err)
	}
	if out["$comment"] != "metasystem.version=3" || out["title"] != "Impl return version 3" {
		t.Fatalf("version-three identity is wrong: comment=%v title=%v", out["$comment"], out["title"])
	}
	props := out["properties"].(map[string]any)
	version := props["schemaVersion"].(map[string]any)["enum"].([]any)
	if len(version) != 1 || version[0] != 3 {
		t.Fatalf("schemaVersion does not require 3: %v", version)
	}
	rigor := props["rigor"].(map[string]any)
	row := rigor["items"].(map[string]any)
	rowProps := row["properties"].(map[string]any)
	class := rowProps["rigorClass"].(map[string]any)["enum"].([]any)
	if fmt.Sprint(class) != "[severe bounded unproven]" {
		t.Fatalf("rigor classes are wrong: %v", class)
	}
	facts := rowProps["facts"].(map[string]any)
	if len(facts["required"].([]any)) != 7 || len(facts["properties"].(map[string]any)) != 7 {
		t.Fatalf("the seven evidence facts are not all required: %v", facts)
	}
}

func TestMaterializeV1V2AndCriticV3(t *testing.T) {
	root := t.TempDir()
	schemaDir := filepath.Join(root, "scripts/agents/schemas")
	if err := os.MkdirAll(schemaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(v1Schema())
	if err := os.WriteFile(filepath.Join(schemaDir, "implementer.schema.json"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(schemaDir, "code-critic.schema.json"), body, 0o644); err != nil {
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

	v3Out := filepath.Join(root, "v3.json")
	if err := Materialize(root, "code-critic", 3, v3Out); err != nil {
		t.Fatal(err)
	}
	var v3 map[string]any
	data, _ = os.ReadFile(v3Out)
	_ = json.Unmarshal(data, &v3)
	if v3["$comment"] != "metasystem.version=3" {
		t.Fatalf("v3 output missing the marker: %v", v3["$comment"])
	}
	if err := Materialize(root, "implementer", 3, filepath.Join(root, "forbidden.json")); err == nil {
		t.Fatal("version 3 materialized for a non-critic role")
	}
}

// The structured-output invariants, in the generator's own package:
// every object typed and closed, every required
// list complete, every property declaring a type. The defect classes
// this guards against — an object without required
// and a bare const without a type — each fail every codex
// dispatch before the model produces a token.
func TestMaterializedSchemasObeyStructuredOutputRules(t *testing.T) {
	// go test runs with the package directory as cwd; the shipped role
	// schemas this linter guards live two levels up.
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	roles := []string{"behavior-judge", "code-critic", "design-critic", "implementer", "investigator", "verifier", "warden"}
	var problems []string
	declaresAType := func(node map[string]any) bool {
		for _, key := range []string{"type", "enum", "anyOf", "oneOf", "allOf", "$ref"} {
			if _, ok := node[key]; ok {
				return true
			}
		}
		return false
	}
	var walk func(node any, where string)
	walk = func(node any, where string) {
		object, ok := node.(map[string]any)
		if !ok {
			return
		}
		if properties, ok := object["properties"].(map[string]any); ok {
			if object["type"] != "object" {
				problems = append(problems, where+": has properties but is not typed object")
			}
			required, ok := object["required"].([]any)
			if !ok {
				problems = append(problems, where+": object without a required list")
			} else {
				have := map[string]bool{}
				for _, name := range required {
					if s, ok := name.(string); ok {
						have[s] = true
					}
				}
				var absent []string
				for name := range properties {
					if !have[name] {
						absent = append(absent, name)
					}
				}
				sort.Strings(absent)
				if len(absent) > 0 {
					problems = append(problems, fmt.Sprintf("%s: properties absent from required: %v", where, absent))
				}
			}
			if object["additionalProperties"] != false {
				problems = append(problems, where+": object without additionalProperties false")
			}
			for name, child := range properties {
				if childObject, ok := child.(map[string]any); ok && !declaresAType(childObject) {
					problems = append(problems, where+"/"+name+": declares neither type nor enum")
				}
				walk(child, where+"/"+name)
			}
		}
		if items, ok := object["items"].(map[string]any); ok {
			walk(items, where+"[]")
		}
		for _, key := range []string{"anyOf", "oneOf", "allOf"} {
			if children, ok := object[key].([]any); ok {
				for index, child := range children {
					walk(child, fmt.Sprintf("%s/%s[%d]", where, key, index))
				}
			}
		}
	}
	for _, role := range roles {
		output := filepath.Join(t.TempDir(), role+".json")
		version := 2
		if VersionThreeRoles[role] {
			version = 3
		}
		if err := Materialize(root, role, version, output); err != nil {
			t.Fatalf("materialize %s: %v", role, err)
		}
		data, err := os.ReadFile(output)
		if err != nil {
			t.Fatal(err)
		}
		var schema any
		if err := json.Unmarshal(data, &schema); err != nil {
			t.Fatalf("materialized %s is not JSON: %v", role, err)
		}
		walk(schema, role)
	}
	if len(problems) > 0 {
		t.Fatalf("materialized schemas violate the structured-output rules:\n%s", strings.Join(problems, "\n"))
	}
}
