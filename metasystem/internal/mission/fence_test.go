package mission

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var fixedNow = time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)

func withFixedClock(t *testing.T) {
	t.Helper()
	old := clock
	clock = func() time.Time { return fixedNow }
	t.Cleanup(func() { clock = old })
}

const fenceContract = "```mission\n" +
	"fence.wall-clock-hours=12\n" +
	"fence.cycles=1\n" +
	"fence.jobs=5\n" +
	"fence.concurrency=2\n" +
	"fence.job-cap-min=240\n" +
	"cap.min.codex.gpt-5-6-sol=180\n" +
	"```\n"

// fenceEnv writes a sealed contract and its fence counters, and returns the repo.
func fenceEnv(t *testing.T) (repo, mission string) {
	t.Helper()
	withFixedClock(t)
	repo = t.TempDir()
	mission = "demo"
	plans := filepath.Join(repo, "plans")
	if err := os.MkdirAll(plans, 0o755); err != nil {
		t.Fatal(err)
	}
	contractPath := filepath.Join(plans, "mission-demo.contract.md")
	writeText(t, contractPath, fenceContract)
	data, _ := os.ReadFile(contractPath)
	approved := sha256Hex(string(data))

	fencesDir := missionDir(repo, mission)
	if err := os.MkdirAll(fencesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	fences := map[string]any{
		"schemaVersion": 1, "missionId": mission, "startedAt": fenceNowISO(),
		"cycles": 0, "reservations": map[string]any{}, "approvedContractSha256": approved,
	}
	if err := atomicWriteJSON(filepath.Join(fencesDir, "fences.json"), fences); err != nil {
		t.Fatal(err)
	}
	return repo, mission
}

func TestContractValidationRejectsNonCanonicalCap(t *testing.T) {
	repo := t.TempDir()
	// A cap key whose model segment is not canonical.
	bad := "```mission\nfence.wall-clock-hours=12\nfence.cycles=1\nfence.jobs=5\nfence.concurrency=2\nfence.job-cap-min=240\ncap.min.codex.GPT_5=180\n```\n"
	if _, err := contractValuesFromBytes([]byte(bad), repo); err == nil ||
		!strings.Contains(err.Error(), "not canonical") {
		t.Fatalf("a non-canonical cap key must be rejected, got %v", err)
	}
	// A contract missing a universal fence.
	missing := "```mission\nfence.cycles=1\nfence.jobs=5\nfence.concurrency=2\nfence.job-cap-min=240\n```\n"
	if _, err := contractValuesFromBytes([]byte(missing), repo); err == nil ||
		!strings.Contains(err.Error(), "universal lifecycle fence") {
		t.Fatalf("a missing fence must be rejected, got %v", err)
	}
}

func TestAuthorizeCapUsesPairCap(t *testing.T) {
	repo, mission := fenceEnv(t)
	result, err := AuthorizeCap(repo, mission, "job-1", "codex", "gpt-5-6-sol", nil)
	if err != nil {
		t.Fatalf("authorize should succeed: %v", err)
	}
	if cap, _ := intValue(result["capMin"]); cap != 180 {
		t.Fatalf("cap should be the signed pair cap 180, got %v", result["capMin"])
	}
	source, _ := result["source"].(map[string]any)
	if source["rule"] != "contract-pair" {
		t.Fatalf("source rule should be contract-pair: %v", source)
	}
	// The reservation was recorded.
	fences, _ := readJSONObjectFile(filepath.Join(missionDir(repo, mission), "fences.json"))
	if _, ok := reservationsMap(fences)["job-1"]; !ok {
		t.Fatal("the job reservation should be recorded")
	}
}

func TestAuthorizeCapRefusesAboveSigned(t *testing.T) {
	repo, mission := fenceEnv(t)
	requested := 300
	if _, err := AuthorizeCap(repo, mission, "job-1", "codex", "gpt-5-6-sol", &requested); err == nil ||
		!strings.Contains(err.Error(), "above signed") {
		t.Fatalf("a request above the signed cap must be refused, got %v", err)
	}
}

func TestReserveCycleTripsFenceAndBatchesAsk(t *testing.T) {
	repo, mission := fenceEnv(t)
	// fence.cycles=1: the first cycle is allowed.
	if err := ReserveCycle(repo, mission); err != nil {
		t.Fatalf("the first cycle should be allowed: %v", err)
	}
	// The second trips the cycles fence and writes a batched ask.
	err := ReserveCycle(repo, mission)
	if err == nil || !strings.Contains(err.Error(), "cycles") {
		t.Fatalf("the cycle fence should refuse the second cycle, got %v", err)
	}
	asks, _ := filepath.Glob(filepath.Join(missionDir(repo, mission), "asks", "fence-bound*.json"))
	if len(asks) != 1 {
		t.Fatalf("a single batched ask should exist, got %d", len(asks))
	}
	ask, _ := readJSONObjectFile(asks[0])
	if q, _ := ask["question"].(string); !strings.Contains(q, "`cycles`") {
		t.Fatalf("the ask should name the cycles fence: %q", q)
	}
}

func TestAggregateUsageSumsTerminalJobs(t *testing.T) {
	repo, mission := fenceEnv(t)
	jobs := filepath.Join(repo, "artifacts", "agents", "jobs")
	if err := os.MkdirAll(jobs, 0o755); err != nil {
		t.Fatal(err)
	}
	writeText(t, filepath.Join(jobs, "job-a.json"),
		`{"jobId":"job-a","mission":"demo","status":"completed","runtime":"codex","usage":{"outputTokens":100}}`)
	writeText(t, filepath.Join(jobs, "job-b.json"),
		`{"jobId":"job-b","mission":"demo","status":"completed","runtime":"codex","usage":{"outputTokens":40}}`)
	// A running job is not aggregated.
	writeText(t, filepath.Join(jobs, "job-c.json"),
		`{"jobId":"job-c","mission":"demo","status":"running","runtime":"codex","usage":{"outputTokens":999}}`)

	if err := AggregateUsage(repo, mission); err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	usage, _ := readJSONObjectFile(filepath.Join(missionDir(repo, mission), "usage.json"))
	units, _ := usage["units"].([]any)
	var total float64
	for _, u := range units {
		item, _ := u.(map[string]any)
		if item["unit"] == "tokens.outputTokens" {
			total, _ = floatValue(item["value"])
		}
	}
	if total != 140 {
		t.Fatalf("terminal output tokens should sum to 140, got %v (units=%v)", total, units)
	}
}
