package mission

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// Ported from dispatch-fixtures.sh (other-provider, lines 4803-4826): a
// provider-native unit with the same spelling from two providers stays two
// typed tuples; the aggregate never sums across providers into a
// heterogeneous total, and a job of another mission is not counted.
func TestAggregateUsageKeepsSameNamedProviderUnitsApart(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	jobs := filepath.Join(repo, "artifacts", "agents", "jobs")
	if err := os.MkdirAll(jobs, 0o755); err != nil {
		t.Fatal(err)
	}
	record := func(job, mission, runtime, status string, units int) {
		writeText(t, filepath.Join(jobs, job+".json"),
			`{"jobId":"`+job+`","mission":"`+mission+`","runtime":"`+runtime+`","status":"`+status+`","usage":{"availability":"native","inputTokens":3,"cachedInputTokens":null,"outputTokens":null,"reasoningTokens":null,"cost":null,"providerUnits":{"name":"fake-unit","value":`+strconv.Itoa(units)+`}}}`)
	}
	record("explicit", "alpha", "fake", "completed", 1)
	record("inherited", "alpha", "fake", "completed", 1)
	record("envelope-model", "alpha", "fake", "completed", 1)
	record("envelope-runtime", "alpha", "fake", "completed", 1)
	record("other-provider", "alpha", "other", "completed", 5)
	record("elsewhere", "beta", "fake", "completed", 9)
	record("still-running", "alpha", "fake", "running", 7)

	if err := AggregateUsage(repo, "alpha"); err != nil {
		t.Fatal(err)
	}
	usage, err := readJSONObjectFile(filepath.Join(missionDir(repo, "alpha"), "usage.json"))
	if err != nil {
		t.Fatal(err)
	}
	units, _ := usage["units"].([]any)
	byProvider := map[string]float64{}
	for _, raw := range units {
		item, _ := raw.(map[string]any)
		if item["unit"] == "provider.total" {
			t.Fatalf("the mission usage aggregated a heterogeneous provider total: %v", units)
		}
		if item["unit"] != "provider.fake-unit" {
			continue
		}
		provider, _ := item["provider"].(string)
		if _, seen := byProvider[provider]; seen {
			t.Fatalf("provider %s carries two fake-unit tuples: %v", provider, units)
		}
		byProvider[provider], _ = floatValue(item["value"])
	}
	if len(byProvider) != 2 || byProvider["fake"] != 4 || byProvider["other"] != 5 {
		t.Fatalf("fake-unit tuples %v, want fake=4 and other=5 kept apart", byProvider)
	}
}
