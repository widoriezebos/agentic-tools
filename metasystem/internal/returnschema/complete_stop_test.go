package returnschema

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCodeCriticStopEvidenceSchemaPreservesOldAttestations(t *testing.T) {
	t.Parallel()
	for _, mutation := range []string{"valid", "class", "path", "change", "other-rule", "old"} {
		t.Run(mutation, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			f := map[string]any{"id": "F1", "severity": "high", "material": true, "claim": "The read misses retained evidence.", "evidence": "Observed the missing reader.", "class": "missing-reader", "where": "metasystem/test.go", "change": "Read the retained evidence.", "resolves": nil, "relation": nil}
			r := map[string]any{"schemaVersion": 6, "claimed": map[string]any{"sessionId": nil, "model": nil}, "jobId": "critic", "round": 1, "runtime": "fixture", "sessionId": "session", "model": map[string]any{"requested": "resolved", "effective": "resolved"}, "evidence": []any{}, "gaps": []any{}, "mode": "review", "reviewedTree": strings.Repeat("a", 40), "findings": []any{f}, "verdictMaterialCount": 1, "rigor": []any{map[string]any{"findingId": "F1", "rigorClass": "bounded", "facts": map[string]any{"local": true, "recoverable": true, "proofBoundaryCrossed": false, "authorityBoundaryCrossed": false, "secretsBoundaryCrossed": false, "irreversibleDataBoundaryCrossed": false, "externalSideEffectBoundaryCrossed": false}, "reopeningTrigger": "Missing evidence recurs", "artifact": "metasystem/test.go"}}}
			switch mutation {
			case "class":
				delete(f, "class")
			case "path":
				f["where"] = "../outside.go"
			case "change":
				f["change"] = " "
			case "other-rule":
				f["class"] = "other"
			case "old":
				r["schemaVersion"] = 3
				for _, field := range []string{"class", "where", "change", "resolves", "relation"} {
					delete(f, field)
				}
				delete(r["rigor"].([]any)[0].(map[string]any), "artifact")
			}
			data, err := json.Marshal(r)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, "return.json")
			if err := os.WriteFile(path, data, 0o644); err != nil {
				t.Fatal(err)
			}
			violations := ReturnCompleteRole(root, "code-critic", path)
			wantValid := mutation == "valid" || mutation == "old"
			if (len(violations) == 0) != wantValid {
				t.Fatalf("%s validation: %s", mutation, fmt.Sprint(violations))
			}
		})
	}
}
