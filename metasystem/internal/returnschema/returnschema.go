// Package returnschema materializes the versioned role-return schemas.
// Version 1 is the frozen source on disk. Later versions are generated without
// changing those checked-in files.
package returnschema

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Roles are the role schemas this materializer knows.
var Roles = map[string]bool{
	"behavior-judge": true, "code-critic": true, "design-critic": true,
	"implementer": true, "investigator": true, "steward-continuation": true,
	"verifier": true, "warden": true,
}

// VersionThreeRoles are the critic roles whose return contract carries rigor
// classifications.
var VersionThreeRoles = map[string]bool{
	"code-critic": true, "design-critic": true, "warden": true,
}

// VersionTwo returns the v2 form of a v1 schema: a version marker, the
// schemaVersion and claimed members added to properties and required, and the
// model's effective field. Every property is listed in required and "nothing
// claimed" is a null member, because a provider's structured output rejects an
// object schema without required and treats an absent key as a violation.
func VersionTwo(schema map[string]any) (map[string]any, error) {
	value := schema
	value["$comment"] = "metasystem.version=2"
	title, _ := value["title"].(string)
	if title == "" {
		title = "Agent return"
	}
	value["title"] = title + " version 2"

	required, ok := value["required"].([]any)
	if !ok {
		return nil, fmt.Errorf("schema has no required array")
	}
	value["required"] = append([]any{"schemaVersion", "claimed"}, required...)

	properties, ok := value["properties"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("schema has no properties object")
	}
	properties["schemaVersion"] = map[string]any{"type": "integer", "enum": []any{2}}
	properties["claimed"] = map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []any{"sessionId", "model"},
		"properties": map[string]any{
			"sessionId": map[string]any{"type": []any{"string", "null"}},
			"model":     map[string]any{"type": []any{"string", "null"}},
		},
	}
	properties["sessionId"] = map[string]any{"type": "string"}

	model, ok := properties["model"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("schema has no model property")
	}
	modelProps, ok := model["properties"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("model property has no properties object")
	}
	modelProps["effective"] = map[string]any{"type": "string"}
	return value, nil
}

// VersionThree returns the critic-only v3 form. It retains the v2 identity
// envelope and adds one rigor row for every material finding.
func VersionThree(schema map[string]any) (map[string]any, error) {
	value, err := VersionTwo(schema)
	if err != nil {
		return nil, err
	}
	value["$comment"] = "metasystem.version=3"
	title, _ := value["title"].(string)
	value["title"] = strings.TrimSuffix(title, " version 2") + " version 3"

	required := value["required"].([]any)
	value["required"] = append(required, "rigor")
	properties := value["properties"].(map[string]any)
	properties["schemaVersion"] = map[string]any{"type": "integer", "enum": []any{3}}
	properties["rigor"] = rigorSchema()
	return value, nil
}

func rigorSchema() map[string]any {
	boolean := func() map[string]any { return map[string]any{"type": "boolean"} }
	factProperties := map[string]any{
		"local":                             boolean(),
		"recoverable":                       boolean(),
		"proofBoundaryCrossed":              boolean(),
		"authorityBoundaryCrossed":          boolean(),
		"secretsBoundaryCrossed":            boolean(),
		"irreversibleDataBoundaryCrossed":   boolean(),
		"externalSideEffectBoundaryCrossed": boolean(),
	}
	return map[string]any{
		"type": "array",
		"items": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []any{"findingId", "rigorClass", "facts", "reopeningTrigger"},
			"properties": map[string]any{
				"findingId":  map[string]any{"type": "string"},
				"rigorClass": map[string]any{"type": "string", "enum": []any{"severe", "bounded", "unproven"}},
				"facts": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required": []any{
						"local", "recoverable", "proofBoundaryCrossed", "authorityBoundaryCrossed",
						"secretsBoundaryCrossed", "irreversibleDataBoundaryCrossed", "externalSideEffectBoundaryCrossed",
					},
					"properties": factProperties,
				},
				"reopeningTrigger": map[string]any{"type": "string"},
			},
		},
	}
}

// Materialize reads a role's v1 schema, applies the requested version, and
// writes it (indented, key-sorted) to outputPath.
func Materialize(root, role string, version int, outputPath string) error {
	source := filepath.Join(root, "scripts/agents/schemas", role+".schema.json")
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	var schema map[string]any
	if err := json.Unmarshal(data, &schema); err != nil {
		return fmt.Errorf("%s is not valid JSON: %w", source, err)
	}
	if version == 2 {
		if schema, err = VersionTwo(schema); err != nil {
			return err
		}
	} else if version == 3 {
		if !VersionThreeRoles[role] {
			return fmt.Errorf("schema version 3 is only available for critic roles")
		}
		if schema, err = VersionThree(schema); err != nil {
			return err
		}
	}
	encoded, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(outputPath, append(encoded, '\n'), 0o644)
}
