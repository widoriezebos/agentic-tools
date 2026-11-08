package supervisor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/returnschema"
)

// TestCodeCriticIsHandedTheSchemaTheStopCollects pins the version each role
// is handed, and that the code critic's materialized schema requires the
// members the unit stop reads from a material finding (finding 570: a
// version-4 material finding fails "incomplete stop evidence").
func TestCodeCriticIsHandedTheSchemaTheStopCollects(t *testing.T) {
	t.Parallel()
	for role, want := range map[string]int{"code-critic": 6, "design-critic": 4, "warden": 4, "implementer": 2} {
		if got := returnSchemaVersion(role); got != want {
			t.Fatalf("%s: schema version %d, want %d", role, got, want)
		}
	}
	path := filepath.Join(t.TempDir(), "return-schema.json")
	if err := returnschema.Materialize("code-critic", returnSchemaVersion("code-critic"), path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Properties struct {
			Findings struct {
				Items struct {
					Required []string `json:"required"`
				} `json:"items"`
			} `json:"findings"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	for _, member := range []string{"class", "where", "change"} {
		if !slices.Contains(schema.Properties.Findings.Items.Required, member) {
			t.Fatalf("the code critic's schema does not require %s: %v", member, schema.Properties.Findings.Items.Required)
		}
	}
}

// TestCodeCriticReturnShapedByItsSchemaCollects is the producer-to-collector
// proof: a finding carrying exactly the members the materialized schema
// requires, marked material, is collected by the unit stop's reader with a
// material count of one.
func TestCodeCriticReturnShapedByItsSchemaCollects(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "return-schema.json")
	if err := returnschema.Materialize("code-critic", returnSchemaVersion("code-critic"), path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Properties struct {
			SchemaVersion struct {
				Enum []int `json:"enum"`
			} `json:"schemaVersion"`
			Findings struct {
				Items struct {
					Required []string `json:"required"`
				} `json:"items"`
			} `json:"findings"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	if len(schema.Properties.SchemaVersion.Enum) != 1 {
		t.Fatalf("schemaVersion enum: %v", schema.Properties.SchemaVersion.Enum)
	}
	finding := map[string]any{}
	for _, member := range schema.Properties.Findings.Items.Required {
		switch member {
		case "material":
			finding[member] = true
		case "severity":
			finding[member] = "medium"
		case "class":
			finding[member] = "regression"
		case "where":
			finding[member] = "metasystem/cmd/metasystem/landing_agent.go"
		case "resolves":
			finding[member] = nil
		default:
			finding[member] = member + " text"
		}
	}
	ret, err := json.Marshal(map[string]any{"schemaVersion": schema.Properties.SchemaVersion.Enum[0], "verdictMaterialCount": 1, "findings": []any{finding}})
	if err != nil {
		t.Fatal(err)
	}
	read, err := readsubject.Collect("job", readsubject.ReadSubject{}, "engine", "model", "output", ret, "")
	if err != nil || read.Material != 1 {
		t.Fatalf("the code critic's return is not collected: material=%d err=%v required=%v", read.Material, err, schema.Properties.Findings.Items.Required)
	}
}
