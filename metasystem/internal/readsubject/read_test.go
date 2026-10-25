package readsubject

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestReadRequiresStructuredStopInputs(t *testing.T) {
	t.Parallel()
	valid := map[string]any{"verdictMaterialCount": 1, "findings": []any{map[string]any{"id": "F-1", "class": "regression", "severity": "high", "material": true, "claim": "missing reader", "evidence": "read call", "where": "internal/a.go", "change": "consume the field"}}}
	data, _ := json.Marshal(valid)
	r, err := Collect("job-one", ReadSubject{Kind: SubjectCommit, Tree: "tree"}, "engine-one", "model-one", "return.json", data, "VERDICT: fix first (1 material findings)")
	if err != nil || r.Material != 1 || r.Findings[0].ID != "job-one:1" {
		t.Fatalf("collect %+v %v", r, err)
	}
	var normalized map[string]any
	json.Unmarshal(data, &normalized)
	normalized["findings"].([]any)[0].(map[string]any)["where"] = "./internal//a.go"
	pathData, _ := json.Marshal(normalized)
	normalizedRead, pathErr := Collect("job-one", ReadSubject{}, "engine", "model", "output", pathData, "")
	if pathErr != nil || normalizedRead.Findings[0].Where != "internal/a.go" {
		t.Fatalf("relative path was not normalized: %+v %v", normalizedRead, pathErr)
	}
	_, digest := r.Canonical()
	if digest == "" {
		t.Fatal("canonical evidence has no digest")
	}
	for _, row := range []struct {
		name    string
		mutate  func(map[string]any)
		verdict string
	}{
		{"missing findings", func(v map[string]any) { delete(v, "findings") }, ""},
		{"missing class", func(v map[string]any) { delete(v["findings"].([]any)[0].(map[string]any), "class") }, ""},
		{"null material", func(v map[string]any) {
			v["findings"].([]any)[0].(map[string]any)["material"] = nil
			v["verdictMaterialCount"] = 0
		}, ""},
		{"missing material", func(v map[string]any) { delete(v["findings"].([]any)[0].(map[string]any), "material") }, ""},
		{"disagreeing count", func(v map[string]any) { v["verdictMaterialCount"] = 0 }, ""},
		{"disagreeing prose", func(map[string]any) {}, "VERDICT: LAND"},
		{"absolute path", func(v map[string]any) { v["findings"].([]any)[0].(map[string]any)["where"] = "/tmp/a.go" }, ""},
		{"traversal", func(v map[string]any) { v["findings"].([]any)[0].(map[string]any)["where"] = "../a.go" }, ""},
		{"line in path", func(v map[string]any) { v["findings"].([]any)[0].(map[string]any)["where"] = "internal/a.go:12" }, ""},
		{"other without rule", func(v map[string]any) { v["findings"].([]any)[0].(map[string]any)["class"] = "other" }, ""},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			var value map[string]any
			json.Unmarshal(data, &value)
			row.mutate(value)
			bad, _ := json.Marshal(value)
			if _, err := Collect("job-one", ReadSubject{}, "engine", "model", "output", bad, row.verdict); err == nil {
				t.Fatal("unknown stop input became a valid read")
			}
		})
	}
}

// TestCollectKeepsAnOldCriticsNote: a v4 or v5 critic's non-material row
// without class, where and change is a note; every other row keeps the full
// stop-evidence rule.
func TestCollectKeepsAnOldCriticsNote(t *testing.T) {
	t.Parallel()
	note := func() map[string]any {
		return map[string]any{"id": "N-1", "severity": "low", "material": false, "claim": "a naming nit", "evidence": "read.go reads it", "relation": "new"}
	}
	for _, version := range []int{4, 5} {
		data, _ := json.Marshal(map[string]any{"schemaVersion": version, "verdictMaterialCount": 0, "findings": []any{note()}})
		r, err := Collect("job", ReadSubject{}, "engine", "model", "output", data, "VERDICT: LAND")
		if err != nil || r.Material != 0 || len(r.Findings) != 1 || r.Findings[0].ID != "job:1" || r.Findings[0].Relation != "new" {
			t.Fatalf("v%d note: %+v %v", version, r, err)
		}
	}
	for _, row := range []struct {
		name   string
		mutate func(return_, finding map[string]any)
	}{
		{"v4 material without class", func(v, f map[string]any) { f["material"], v["verdictMaterialCount"] = true, 1 }},
		{"v4 note without evidence", func(_, f map[string]any) { delete(f, "evidence") }},
		{"v4 note with a bad severity", func(_, f map[string]any) { f["severity"] = "nit" }},
		{"note without a version", func(v, _ map[string]any) { delete(v, "schemaVersion") }},
		{"v6 note without where", func(v, _ map[string]any) { v["schemaVersion"] = 6 }},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			finding := note()
			value := map[string]any{"schemaVersion": 4, "verdictMaterialCount": 0, "findings": []any{finding}}
			row.mutate(value, finding)
			data, _ := json.Marshal(value)
			if _, err := Collect("job", ReadSubject{}, "engine", "model", "output", data, ""); err == nil || !strings.Contains(err.Error(), "incomplete stop evidence") {
				t.Fatalf("an incomplete row became a valid read: %v", err)
			}
		})
	}
}
