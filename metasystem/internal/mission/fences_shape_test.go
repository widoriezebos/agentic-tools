package mission

import (
	"encoding/json"
	"strings"
	"testing"
)

// The fence-counters shape validator, case by case (Phase 6).
func TestValidateFencesShapes(t *testing.T) {
	base := func() map[string]any {
		return map[string]any{
			"startedAt": "2026-08-12T00:00:00Z",
			"cycles":    json.Number("1"), "jobs": json.Number("2"),
			"activeJobs": json.Number("1"),
			"usage": []any{map[string]any{
				"provider": "anthropic", "unit": "usd", "value": json.Number("0.5"),
			}},
		}
	}
	if err := validateFences(base()); err != nil {
		t.Fatalf("a lawful counters doc refused: %v", err)
	}
	refuse := func(name string, mutate func(map[string]any), want string) {
		doc := base()
		mutate(doc)
		err := validateFences(doc)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("%s: got %v, want %q", name, err, want)
		}
	}
	if err := validateFences("not-a-map"); err == nil || !strings.Contains(err.Error(), "invalid shape") {
		t.Fatalf("non-object accepted: %v", err)
	}
	refuse("extra key", func(d map[string]any) { d["extra"] = 1 }, "invalid shape")
	refuse("bad start time", func(d map[string]any) { d["startedAt"] = "yesterday" }, "start time is invalid")
	refuse("negative counter", func(d map[string]any) { d["cycles"] = json.Number("-1") }, "counter cycles is invalid")
	refuse("active exceeds jobs", func(d map[string]any) { d["activeJobs"] = json.Number("5") }, "exceeds total jobs")
	refuse("usage not array", func(d map[string]any) { d["usage"] = "x" }, "must be an array")
	refuse("usage entry shape", func(d map[string]any) { d["usage"] = []any{map[string]any{"provider": "p"}} }, "invalid shape")
	refuse("empty provider", func(d map[string]any) {
		d["usage"] = []any{map[string]any{"provider": "", "unit": "usd", "value": json.Number("1")}}
	}, "provider/unit is invalid")
	refuse("duplicate tuple", func(d map[string]any) {
		entry := map[string]any{"provider": "p", "unit": "u", "value": json.Number("1")}
		d["usage"] = []any{entry, map[string]any{"provider": "p", "unit": "u", "value": json.Number("2")}}
	}, "repeats a provider/unit tuple")
	refuse("boolean value", func(d map[string]any) {
		d["usage"] = []any{map[string]any{"provider": "p", "unit": "u", "value": true}}
	}, "usage value is invalid")
	refuse("negative value", func(d map[string]any) {
		d["usage"] = []any{map[string]any{"provider": "p", "unit": "u", "value": json.Number("-3")}}
	}, "usage value is invalid")
}
