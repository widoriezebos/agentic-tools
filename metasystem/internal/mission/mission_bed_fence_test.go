package mission

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// Ported from scripts/agents/mission-fixtures.sh (contract-and-state): the
// reaper's locked fence refusal writes its ask, and two simultaneous job
// reservations cannot both cross a concurrency fence of one.

// missionBedFences writes a pinned contract with the given concurrency fence
// and its counters, stamped at a fixed instant the caller's clock follows.
func missionBedFences(t *testing.T, mission string, concurrency string, at time.Time) string {
	t.Helper()
	repo := t.TempDir()
	contract := "```mission\n" +
		"fence.wall-clock-hours=2\n" +
		"fence.cycles=3\n" +
		"fence.jobs=4\n" +
		"fence.concurrency=" + concurrency + "\n" +
		"fence.job-cap-min=30\n" +
		"```\n"
	contractPath := filepath.Join(repo, "plans", "mission-"+mission+".contract.md")
	if err := os.MkdirAll(filepath.Dir(contractPath), 0o755); err != nil {
		t.Fatal(err)
	}
	writeText(t, contractPath, contract)
	if err := os.MkdirAll(missionDir(repo, mission), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := atomicWriteJSON(filepath.Join(missionDir(repo, mission), "fences.json"), map[string]any{
		"schemaVersion": 1, "missionId": mission, "startedAt": fenceISOAt(at),
		"cycles": 0, "reservations": map[string]any{}, "approvedContractSha256": sha256Hex(contract),
	}); err != nil {
		t.Fatal(err)
	}
	return repo
}

func TestMissionBedFenceRefuseWritesTheJobCapAsk(t *testing.T) {
	t.Parallel()
	repo := missionBedFences(t, "timeout-ask", "2", time.Date(2026, 9, 27, 10, 0, 0, 0, time.UTC))
	askPath, err := Refuse(repo, "timeout-ask", "job-cap-min")
	if err != nil {
		t.Fatalf("mission timeout refusal: %v", err)
	}
	want := filepath.Join(missionDir(repo, "timeout-ask"), "asks", "fence-bound.json")
	if askPath != want {
		t.Fatalf("mission timeout ask path = %q, want %q", askPath, want)
	}
	ask, err := readJSONObjectFile(want)
	if err != nil {
		t.Fatalf("mission timeout refusal did not write its ask: %v", err)
	}
	if question, _ := ask["question"].(string); !strings.Contains(question, "`job-cap-min`") {
		t.Fatalf("mission timeout ask omitted its reached fence: %q", question)
	}
}

func TestMissionBedConcurrentReservationsAdmitExactlyOne(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 9, 27, 10, 0, 0, 0, time.UTC)
	repo := missionBedFences(t, "race", "1", at)
	clock := func() time.Time { return at.Add(time.Minute) }
	start := make(chan struct{})
	results := make([]error, 2)
	var group sync.WaitGroup
	for index, job := range []string{"race-a", "race-b"} {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			results[index] = CheckOrReserveWithClock(repo, "race", job, 30, true, clock)
		}()
	}
	close(start)
	group.Wait()

	admitted, refused := 0, 0
	for _, err := range results {
		switch {
		case err == nil:
			admitted++
		case strings.Contains(err.Error(), "concurrency"):
			refused++
		default:
			t.Fatalf("unexpected reservation error: %v", err)
		}
	}
	if admitted != 1 || refused != 1 {
		t.Fatalf("mission concurrency lock admitted %d and refused %d reservations: %v", admitted, refused, results)
	}
	fences, err := readJSONObjectFile(filepath.Join(missionDir(repo, "race"), "fences.json"))
	if err != nil || len(reservationsMap(fences)) != 1 {
		t.Fatalf("fences after the race = %v, %v; want exactly one reservation", fences, err)
	}
	ask, err := readJSONObjectFile(filepath.Join(missionDir(repo, "race"), "asks", "fence-bound.json"))
	if err != nil {
		t.Fatalf("concurrent mission refusal did not write its batched ask: %v", err)
	}
	if question, _ := ask["question"].(string); !strings.Contains(question, "`concurrency`") {
		t.Fatalf("concurrent mission ask omitted the concurrency fence: %q", question)
	}
}
