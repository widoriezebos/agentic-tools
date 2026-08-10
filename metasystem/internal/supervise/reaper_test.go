package supervise

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// A fake custody table: a pid is alive only when listed with a matching start
// and tag, exactly the three-way discipline the kernel prober enforces.
type fakeCustody map[int64]struct {
	start int64
	tag   string
}

func (f fakeCustody) liveness(pid, start int64, tag string) identity.Liveness {
	entry, ok := f[pid]
	if !ok {
		return identity.Dead
	}
	if entry.start != start || entry.tag != tag {
		return identity.Dead
	}
	return identity.Alive
}

func writeJobRecord(t *testing.T, dir, id string, record map[string]any) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, id+".json")
	if err := os.WriteFile(path, append(encoded, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func readStatus(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var record map[string]any
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatal(err)
	}
	return record
}

// The reaper's core: a running job whose custodian is provably gone becomes
// failed/process-lost; a running job past its cap becomes timeout/budget-cap;
// and a healthy running job is left untouched.
func TestReaperPassCoreTransitions(t *testing.T) {
	dir := t.TempDir()
	now := time.Unix(1786000000, 0).UTC()

	// A live custodian that is over budget: budget-cap wins, timeout.
	overBudget := writeJobRecord(t, dir, "over-budget", map[string]any{
		"jobId": "over-budget", "status": "running",
		"pid": 111, "pidStartedAt": 5, "instanceTag": "job-over",
		"startedAt": now.Add(-2 * time.Hour).Format(isoSecond), "capMin": 60,
	})
	// A live custodian within budget: healthy, untouched.
	healthy := writeJobRecord(t, dir, "healthy", map[string]any{
		"jobId": "healthy", "status": "running",
		"pid": 222, "pidStartedAt": 6, "instanceTag": "job-healthy",
		"startedAt": now.Add(-1 * time.Minute).Format(isoSecond), "capMin": 60,
	})
	// A running job whose custodian is gone and is NOT over budget: process-lost.
	lost := writeJobRecord(t, dir, "lost", map[string]any{
		"jobId": "lost", "status": "running",
		"pid": 333, "pidStartedAt": 7, "instanceTag": "job-lost",
		"startedAt": now.Add(-1 * time.Minute).Format(isoSecond), "capMin": 60,
	})
	// A pending job whose named supervisor is gone: process-lost.
	pendingLost := writeJobRecord(t, dir, "pending-lost", map[string]any{
		"jobId": "pending-lost", "status": "pending",
		"pid": 444, "pidStartedAt": 8, "instanceTag": "job-pending",
		"startedAt": now.Add(-1 * time.Minute).Format(isoSecond),
	})
	// A pending job with no custodian yet (still in launch handshake): deferred.
	handshake := writeJobRecord(t, dir, "handshake", map[string]any{
		"jobId": "handshake", "status": "pending",
		"pid": nil, "instanceTag": "job-handshake",
		"startedAt": now.Add(-1 * time.Minute).Format(isoSecond),
	})

	custody := fakeCustody{
		111: {start: 5, tag: "job-over"},    // over-budget custodian is live
		222: {start: 6, tag: "job-healthy"}, // healthy custodian is live
		// 333, 444 absent -> dead
	}
	var emitted []string
	cfg := ReaperConfig{
		JobsDir:   dir,
		Now:       func() time.Time { return now },
		Custodian: custody.liveness,
		Emit:      func(line string) { emitted = append(emitted, line) },
	}
	if err := cfg.ReaperPass(); err != nil {
		t.Fatalf("reaper pass: %v", err)
	}

	if got := readStatus(t, overBudget); got["status"] != "timeout" || got["error"] != "budget-cap" {
		t.Fatalf("over-budget job: want timeout/budget-cap, got %v/%v", got["status"], got["error"])
	}
	if got := readStatus(t, healthy); got["status"] != "running" {
		t.Fatalf("healthy job must be untouched, got status %v", got["status"])
	}
	if got := readStatus(t, lost); got["status"] != "failed" || got["error"] != "process-lost" {
		t.Fatalf("lost job: want failed/process-lost, got %v/%v", got["status"], got["error"])
	}
	if got := readStatus(t, pendingLost); got["status"] != "failed" || got["error"] != "process-lost" {
		t.Fatalf("pending-lost job: want failed/process-lost, got %v/%v", got["status"], got["error"])
	}
	if got := readStatus(t, handshake); got["status"] != "pending" {
		t.Fatalf("handshake job must be deferred, got status %v", got["status"])
	}

	// Every reaped record carries the provenance stamp.
	for _, path := range []string{overBudget, lost, pendingLost} {
		if got := readStatus(t, path); got["phase"] != "supervision" || got["groupDeathProvenAt"] == nil {
			t.Fatalf("reaped record %s missing provenance: %v", path, got)
		}
	}
	if len(emitted) != 3 {
		t.Fatalf("expected three reap lines, got %v", emitted)
	}
}

// An unreadable custodian (Unknown) never reaps: indeterminacy is reported, not
// acted on.
func TestReaperPassDefersOnUnknownCustodian(t *testing.T) {
	dir := t.TempDir()
	now := time.Unix(1786000000, 0).UTC()
	job := writeJobRecord(t, dir, "unreadable", map[string]any{
		"jobId": "unreadable", "status": "running",
		"pid": 999, "pidStartedAt": 3, "instanceTag": "job-unreadable",
		"startedAt": now.Add(-1 * time.Minute).Format(isoSecond), "capMin": 60,
	})
	cfg := ReaperConfig{
		JobsDir: dir,
		Now:     func() time.Time { return now },
		Custodian: func(pid, start int64, tag string) identity.Liveness {
			return identity.Unknown
		},
	}
	if err := cfg.ReaperPass(); err != nil {
		t.Fatalf("reaper pass: %v", err)
	}
	if got := readStatus(t, job); got["status"] != "running" {
		t.Fatalf("a job with an unreadable custodian must be deferred, got %v", got["status"])
	}
}

// A capDeadline in the past expires the budget even without capMin.
func TestReaperPassHonorsCapDeadline(t *testing.T) {
	dir := t.TempDir()
	now := time.Unix(1786000000, 0).UTC()
	job := writeJobRecord(t, dir, "deadline", map[string]any{
		"jobId": "deadline", "status": "running",
		"pid": 555, "pidStartedAt": 9, "instanceTag": "job-deadline",
		"capDeadline": now.Add(-1 * time.Minute).Format(isoSecond),
	})
	cfg := ReaperConfig{
		JobsDir: dir,
		Now:     func() time.Time { return now },
		Custodian: func(pid, start int64, tag string) identity.Liveness {
			return identity.Alive
		},
	}
	if err := cfg.ReaperPass(); err != nil {
		t.Fatalf("reaper pass: %v", err)
	}
	if got := readStatus(t, job); got["status"] != "timeout" || got["error"] != "budget-cap" {
		t.Fatalf("cap-deadline job: want timeout/budget-cap, got %v/%v", got["status"], got["error"])
	}
}
