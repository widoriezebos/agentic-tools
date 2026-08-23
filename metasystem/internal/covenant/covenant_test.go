package covenant

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The kit-extracted covenant loads whole: identity, rows, battery,
// budgets, guards, and the guardrail net in the shared grammar.
func TestLoadKitExtractedCovenant(t *testing.T) {
	c, err := Load(filepath.Join("testdata", "taskrun-covenant.json"))
	if err != nil {
		t.Fatal(err)
	}
	if c.Identity.Name != "taskrun" || len(c.Requirements) != 3 {
		t.Fatalf("identity or rows wrong: %+v", c)
	}
	if c.Battery.Command != "bash gate.sh" || c.Battery.Threshold != ">=26" {
		t.Fatalf("battery wrong: %+v", c.Battery)
	}
	if len(c.Budgets) != 2 || c.Budgets[0].Metric != "dependency_count" || c.Budgets[0].Bound != 0 {
		t.Fatalf("budgets wrong: %+v", c.Budgets)
	}
	if len(c.Guards) != 1 || c.Guards[0].Cadence != 1 || c.Guards[0].Floor != 1 {
		t.Fatalf("guards wrong: %+v", c.Guards)
	}
	if !c.GuardrailSet.Covers("grader/checks.md") || !c.GuardrailSet.Covers("gate.sh") || c.GuardrailSet.Covers("src/main/App.java") {
		t.Fatalf("the net must cover its declarations and nothing else")
	}
}

// Every refusal names its section and rule; a covenant that guarantees
// nothing, hides a duplicate promise, or declares an unlawful net
// refuses whole.
func TestLoadRefusals(t *testing.T) {
	base := func() map[string]any {
		data, err := os.ReadFile(filepath.Join("testdata", "taskrun-covenant.json"))
		if err != nil {
			t.Fatal(err)
		}
		var doc map[string]any
		json.Unmarshal(data, &doc)
		return doc
	}
	write := func(doc map[string]any) string {
		path := filepath.Join(t.TempDir(), Filename)
		data, _ := json.Marshal(doc)
		os.WriteFile(path, data, 0o644)
		return path
	}
	cases := map[string]struct {
		mutate func(map[string]any)
		expect string
	}{
		"unknown key":        {func(d map[string]any) { d["extra"] = 1 }, "must contain exactly"},
		"wrong version":      {func(d map[string]any) { d["schemaVersion"] = 2 }, "schemaVersion must be 1"},
		"empty requirements": {func(d map[string]any) { d["requirements"] = []any{} }, "guarantees nothing"},
		"duplicate id": {func(d map[string]any) {
			rows := d["requirements"].([]any)
			d["requirements"] = append(rows, rows[0])
		}, "appears twice"},
		"missing proof": {func(d map[string]any) {
			row := d["requirements"].([]any)[0].(map[string]any)
			row["proof"] = ""
		}, "proof must be a non-empty string"},
		"bad direction": {func(d map[string]any) {
			d["battery"].(map[string]any)["direction"] = "up"
		}, "direction must be max or min"},
		"bad cadence": {func(d map[string]any) {
			d["guards"].([]any)[0].(map[string]any)["cadence"] = 0
		}, "positive whole number"},
		"unlawful net": {func(d map[string]any) {
			d["guardrails"] = []any{"../outside"}
		}, "guardrails"},
	}
	for name, tc := range cases {
		doc := base()
		tc.mutate(doc)
		if _, err := Load(write(doc)); err == nil || !strings.Contains(err.Error(), tc.expect) {
			t.Fatalf("%s: want refusal containing %q, got %v", name, tc.expect, err)
		}
	}
	if _, err := Load(filepath.Join(t.TempDir(), Filename)); err == nil {
		t.Fatal("a missing covenant must refuse")
	}
}
