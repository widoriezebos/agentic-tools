package usage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// The wire branch: per-turn numbers ride natively, absence rides
// unavailable — never a fabricated figure or transcript fallback.
func TestACPUsage(t *testing.T) {
	dir := t.TempDir()
	outcome := filepath.Join(dir, "outcome.json")
	usagePath := filepath.Join(dir, "usage.json")

	os.WriteFile(outcome, []byte(`{"row":"delivered","usage":{"inputTokens":11605,"outputTokens":90,"cachedReadTokens":11072}}`), 0o644)
	if err := ACPUsage(usagePath, outcome); err != nil {
		t.Fatal(err)
	}
	var record map[string]any
	body, _ := os.ReadFile(usagePath)
	json.Unmarshal(body, &record)
	if record["availability"] != "native" || record["inputTokens"].(float64) != 11605 ||
		record["outputTokens"].(float64) != 90 || record["cachedInputTokens"].(float64) != 11072 {
		t.Fatalf("native record drifted: %v", record)
	}

	os.WriteFile(outcome, []byte(`{"row":"turn-failed"}`), 0o644)
	if err := ACPUsage(usagePath, outcome); err != nil {
		t.Fatal(err)
	}
	body, _ = os.ReadFile(usagePath)
	json.Unmarshal(body, &record)
	if record["availability"] != "unavailable" || record["inputTokens"] != nil {
		t.Fatalf("absent usage must ride unavailable: %v", record)
	}

	if err := ACPUsage(usagePath, filepath.Join(dir, "missing.json")); err == nil {
		t.Fatal("an unreadable outcome is mechanical, not unavailable")
	}
}

// Partial usage members ride unavailable (never a half-record), and
// a missing cached counter stays null inside a native record.
func TestACPUsagePartialMembers(t *testing.T) {
	dir := t.TempDir()
	outcome := filepath.Join(dir, "outcome.json")
	usagePath := filepath.Join(dir, "usage.json")

	os.WriteFile(outcome, []byte(`{"usage":{"inputTokens":5}}`), 0o644)
	if err := ACPUsage(usagePath, outcome); err != nil {
		t.Fatal(err)
	}
	var record map[string]any
	body, _ := os.ReadFile(usagePath)
	json.Unmarshal(body, &record)
	if record["availability"] != "unavailable" {
		t.Fatalf("half a usage member must ride unavailable: %v", record)
	}

	os.WriteFile(outcome, []byte(`{"usage":{"inputTokens":5,"outputTokens":2}}`), 0o644)
	if err := ACPUsage(usagePath, outcome); err != nil {
		t.Fatal(err)
	}
	body, _ = os.ReadFile(usagePath)
	json.Unmarshal(body, &record)
	if record["availability"] != "native" || record["cachedInputTokens"] != nil {
		t.Fatalf("absent cache counter stays null in a native record: %v", record)
	}

	os.WriteFile(outcome, []byte(`not json`), 0o644)
	if err := ACPUsage(usagePath, outcome); err == nil {
		t.Fatal("garbage outcome is mechanical")
	}
}

// A thinned journal disqualifies native usage the same way (P3
// critique F8): the number cannot be provably complete when the
// raw evidence is not.
func TestACPUsageJournalThinning(t *testing.T) {
	dir := t.TempDir()
	outcome := filepath.Join(dir, "outcome.json")
	usagePath := filepath.Join(dir, "usage.json")
	os.WriteFile(outcome, []byte(`{"journalError":"disk full","usage":{"inputTokens":5,"outputTokens":2}}`), 0o644)
	if err := ACPUsage(usagePath, outcome); err != nil {
		t.Fatal(err)
	}
	var record map[string]any
	body, _ := os.ReadFile(usagePath)
	json.Unmarshal(body, &record)
	if record["availability"] != "unavailable" {
		t.Fatalf("thinned journal must ride unavailable: %v", record)
	}
}
