package usage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readJSONFile(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatalf("unparseable %s: %v", path, err)
	}
	return value
}

func TestDevinUsageDeltaAndUnavailable(t *testing.T) {
	dir := t.TempDir()
	transcript := filepath.Join(dir, "t.json")
	usage := filepath.Join(dir, "usage.json")
	cumulative := filepath.Join(dir, "cum.json")
	writeFile(t, transcript, `{"final_metrics":{"total_prompt_tokens":25799,"total_completion_tokens":1200,"total_cached_tokens":900,"total_steps":40}}`)

	// First turn (no predecessor) records the totals as the delta.
	if err := DevinUsage(usage, transcript, "", cumulative, "", false); err != nil {
		t.Fatal(err)
	}
	got := readJSONFile(t, usage)
	if got["availability"] != "native" || got["inputTokens"] != float64(25799) {
		t.Fatalf("first turn should publish totals: %v", got)
	}
	if pu := got["providerUnits"].(map[string]any); pu["name"] != "devin-steps" || pu["value"] != float64(40) {
		t.Fatalf("unexpected provider units: %v", got["providerUnits"])
	}
	cum := readJSONFile(t, cumulative)
	if cum["total_prompt_tokens"] != float64(25799) {
		t.Fatalf("cumulative totals not recorded: %v", cum)
	}

	// A resumed turn subtracts its predecessor's cumulative totals.
	previous := filepath.Join(dir, "prev.json")
	writeFile(t, previous, `{"total_prompt_tokens":12833,"total_completion_tokens":700,"total_cached_tokens":400,"total_steps":22}`)
	if err := DevinUsage(usage, transcript, "", cumulative, previous, true); err != nil {
		t.Fatal(err)
	}
	got = readJSONFile(t, usage)
	if got["inputTokens"] != float64(25799-12833) || got["outputTokens"] != float64(1200-700) {
		t.Fatalf("resumed turn should publish the delta: %v", got)
	}

	// A resumed turn whose predecessor is missing is unavailable.
	if err := DevinUsage(usage, transcript, "", cumulative, "", true); err != nil {
		t.Fatal(err)
	}
	got = readJSONFile(t, usage)
	if got["availability"] != "unavailable" || got["inputTokens"] != nil {
		t.Fatalf("a missing predecessor should be unavailable: %v", got)
	}
}

func TestDevinUsageACU(t *testing.T) {
	dir := t.TempDir()
	transcript := filepath.Join(dir, "t.json")
	usage := filepath.Join(dir, "usage.json")
	cumulative := filepath.Join(dir, "cum.json")
	// An enterprise account reports ACU and no token totals.
	writeFile(t, transcript, `{"final_metrics":{"total_acu_used": 12.5}}`)
	if err := DevinUsage(usage, transcript, "", cumulative, "", false); err != nil {
		t.Fatal(err)
	}
	got := readJSONFile(t, usage)
	if got["availability"] != "unavailable" {
		t.Fatalf("a tokenless account is unavailable for tokens: %v", got)
	}
	pu := got["providerUnits"].(map[string]any)
	if pu["name"] != "acu" || pu["value"] != float64(12.5) {
		t.Fatalf("ACU should ride in provider units: %v", got["providerUnits"])
	}
	cum := readJSONFile(t, cumulative)
	if cum["total_acu_used"] != float64(12.5) {
		t.Fatalf("the raw ACU key should be stored for the successor: %v", cum)
	}
}

func TestRootJobIDChain(t *testing.T) {
	jobs := t.TempDir()
	writeFile(t, filepath.Join(jobs, "a.json"), `{"parentJob": null}`)
	writeFile(t, filepath.Join(jobs, "b.json"), `{"parentJob": "a"}`)
	writeFile(t, filepath.Join(jobs, "c.json"), `{"parentJob": "b"}`)
	// A record with no parentJob key at all is its own root.
	writeFile(t, filepath.Join(jobs, "lone.json"), `{"round": 1}`)

	for job, want := range map[string]string{"c": "a", "b": "a", "a": "a", "lone": "lone"} {
		got, err := RootJobID(jobs, job)
		if err != nil {
			t.Fatalf("RootJobID(%q): %v", job, err)
		}
		if got != want {
			t.Fatalf("RootJobID(%q) = %q, want %q", job, got, want)
		}
	}
}

func TestRootJobIDCycle(t *testing.T) {
	jobs := t.TempDir()
	writeFile(t, filepath.Join(jobs, "x.json"), `{"parentJob": "y"}`)
	writeFile(t, filepath.Join(jobs, "y.json"), `{"parentJob": "x"}`)

	if _, err := RootJobID(jobs, "x"); err == nil || !strings.Contains(err.Error(), "cyclic") {
		t.Fatalf("expected a cyclic-chain error, got %v", err)
	}
}

// The snapshot path: usage reads the attempt snapshot when given
// one, the snapshot survives export mutation, and oversize propagates.
func TestDevinUsageThroughSnapshot(t *testing.T) {
	dir := t.TempDir()
	transcript := filepath.Join(dir, "t.json")
	if err := os.WriteFile(transcript,
		[]byte(`{"final_metrics":{"total_prompt_tokens":10,"total_cached_tokens":4,"total_completion_tokens":3,"total_steps":2}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	usage := filepath.Join(dir, "usage.json")
	cumulative := filepath.Join(dir, "cumulative.json")
	snapshot := filepath.Join(dir, "snap.json")
	if err := DevinUsage(usage, transcript, snapshot, cumulative, "", false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(snapshot); err != nil {
		t.Fatal("the snapshot must be materialized")
	}
	// Mutating the export does not change a re-read through the snapshot.
	if err := os.WriteFile(transcript, []byte(`{"final_metrics":{"total_prompt_tokens":999}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	usage2 := filepath.Join(dir, "usage2.json")
	cumulative2 := filepath.Join(dir, "cumulative2.json")
	if err := DevinUsage(usage2, transcript, snapshot, cumulative2, "", false); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(cumulative)
	second, _ := os.ReadFile(cumulative2)
	if string(first) != string(second) {
		t.Fatalf("snapshot-fed usage must be immutable across export mutation:\n%s\n%s", first, second)
	}
}
