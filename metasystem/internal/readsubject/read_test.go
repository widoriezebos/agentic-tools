package readsubject

import (
	"encoding/json"
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
