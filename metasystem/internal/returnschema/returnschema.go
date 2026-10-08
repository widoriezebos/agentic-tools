// Package returnschema materializes the versioned role-return schemas.
// Version 1 is the frozen source on disk. Later versions are generated without
// changing those checked-in files.
package returnschema

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/protocol"
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

var VersionFourRoles = VersionThreeRoles
var VersionFiveRoles = VersionThreeRoles
var VersionSixRoles = map[string]bool{"code-critic": true}

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

// VersionFour adds the artifact membership claim to critic rigor rows.
func VersionFour(schema map[string]any) (map[string]any, error) {
	value, err := VersionThree(schema)
	if err != nil {
		return nil, err
	}
	value["$comment"] = "metasystem.version=4"
	title, _ := value["title"].(string)
	value["title"] = strings.TrimSuffix(title, " version 3") + " version 4"
	properties := value["properties"].(map[string]any)
	properties["schemaVersion"] = map[string]any{"type": "integer", "enum": []any{4}}
	items := properties["rigor"].(map[string]any)["items"].(map[string]any)
	items["required"] = append(items["required"].([]any), "artifact")
	segment := `(?:[^./\\\r\n][^/\\\r\n]*|\.[^./\\\r\n][^/\\\r\n]*|\.\.[^/\\\r\n]+)`
	path := `metasystem/(?:` + segment + `/)*` + segment
	items["properties"].(map[string]any)["artifact"] = map[string]any{
		"type": "string", "pattern": `^(?:` + path + `|NEW ` + path + `|` + path + `=>` + path + `)$`,
	}
	return value, nil
}

// VersionFive adds the finding grain and the proof named by mechanical rows.
func VersionFive(schema map[string]any) (map[string]any, error) {
	value, err := VersionFour(schema)
	if err != nil {
		return nil, err
	}
	value["$comment"] = "metasystem.version=5"
	title, _ := value["title"].(string)
	value["title"] = strings.TrimSuffix(title, " version 4") + " version 5"
	properties := value["properties"].(map[string]any)
	properties["schemaVersion"] = map[string]any{"type": "integer", "enum": []any{5}}
	items := properties["rigor"].(map[string]any)["items"].(map[string]any)
	items["required"] = append(items["required"].([]any), "grain", "behaviour", "fixture")
	row := items["properties"].(map[string]any)
	row["grain"] = map[string]any{"type": "string", "enum": []any{"mechanical", "invariant"}}
	row["behaviour"] = map[string]any{"type": []any{"string", "null"}}
	row["fixture"] = map[string]any{"type": []any{"string", "null"}}
	return value, nil
}

// VersionSix records the evidence that automatic unit stops inspect. Older
// versions keep their original attestation contracts.
func VersionSix(schema map[string]any) (map[string]any, error) {
	value, err := VersionFour(schema)
	if err != nil {
		return nil, err
	}
	value["$comment"] = "metasystem.version=6"
	title, _ := value["title"].(string)
	value["title"] = strings.TrimSuffix(title, " version 4") + " version 6"
	properties := value["properties"].(map[string]any)
	properties["schemaVersion"] = map[string]any{"type": "integer", "enum": []any{6}}
	items := properties["findings"].(map[string]any)["items"].(map[string]any)
	row := items["properties"].(map[string]any)
	items["required"] = append(items["required"].([]any), "class", "where", "change", "resolves", "relation")
	row["class"] = map[string]any{"type": "string", "enum": []any{"regression", "weakened-test", "incomplete-item", "false-premise", "faked-seam", "missing-reader", "scope", "other"}}
	row["where"] = map[string]any{"type": "string"}
	row["change"] = map[string]any{"type": "string"}
	row["resolves"] = map[string]any{"type": []any{"string", "null"}}
	row["relation"] = map[string]any{"type": []any{"string", "null"}}
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

// Materialize reads a role's compiled-in v1 schema, applies the requested
// version, and writes it (indented, key-sorted) to outputPath.
func Materialize(role string, version int, outputPath string) error {
	data, err := protocol.RoleSchema(role)
	if err != nil {
		return err
	}
	return materialize(role, "protocol:schemas/"+role+".schema.json", data, version, outputPath)
}

func materialize(role, source string, data []byte, version int, outputPath string) error {
	var schema map[string]any
	err := json.Unmarshal(data, &schema)
	if err != nil {
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
	} else if version == 4 {
		if !VersionFourRoles[role] {
			return fmt.Errorf("schema version 4 is only available for critic roles")
		}
		if schema, err = VersionFour(schema); err != nil {
			return err
		}
	} else if version == 5 {
		if !VersionFiveRoles[role] {
			return fmt.Errorf("schema version 5 is only available for critic roles")
		}
		if schema, err = VersionFive(schema); err != nil {
			return err
		}
	}
	if version == 6 {
		if !VersionSixRoles[role] {
			return fmt.Errorf("schema version 6 is only available for code critics")
		}
		schema, err = VersionSix(schema)
		if err != nil {
			return err
		}
	}
	applyRoleMembers(role, int64(version), schema)
	encoded, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(outputPath, append(encoded, '\n'), 0o644)
}

// applyRoleMembers adds the members a role's schema gains at a version after
// the version transforms, so the schema the runtime is handed and the schema
// the return is validated against are built the same way.
func applyRoleMembers(role string, version int64, schema map[string]any) {
	if schema == nil {
		return
	}
	if role == "code-critic" && version >= 4 && version < 6 {
		addFindingRelation(schema)
	}
}

// addFindingRelation gives a code critic's finding its relation to the
// previous read: new, a fold that does not hold, or the same rule as an
// earlier finding (unit-rounds D4). Versions 1 and 2 are frozen byte
// contracts and structured output requires every property, so the member is
// added, required, from version 4 on, never in the base schema.
func addFindingRelation(schema map[string]any) {
	properties, _ := schema["properties"].(map[string]any)
	findings, _ := properties["findings"].(map[string]any)
	items, _ := findings["items"].(map[string]any)
	row, ok := items["properties"].(map[string]any)
	if !ok {
		return
	}
	row["relation"] = map[string]any{"type": "string", "pattern": "^(new|fold-not-holding|same-rule-as [1-9][0-9]*)$"}
	required, _ := items["required"].([]any)
	for _, name := range required {
		if name == "relation" {
			return
		}
	}
	items["required"] = append(required, "relation")
}
