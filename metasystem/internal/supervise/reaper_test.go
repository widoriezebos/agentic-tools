package supervise

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

// casApplier applies verdicts against the on-disk record with the same
// expected-status discipline the production dispatch record owner enforces:
// the swap lands only when the record still carries the expected status.
func casApplier(t *testing.T, dir string) func(job, expect, target string, patch map[string]any) (bool, error) {
	t.Helper()
	return func(job, expect, target string, patch map[string]any) (bool, error) {
		path := filepath.Join(dir, job+".json")
		record := readStatus(t, path)
		if record["status"] != expect {
			return false, nil
		}
		record["status"] = target
		for key, value := range patch {
			record[key] = value
		}
		encoded, err := json.MarshalIndent(record, "", "  ")
		if err != nil {
			return false, err
		}
		return true, os.WriteFile(path, append(encoded, '\n'), 0o644)
	}
}

// The reaper's core under the no-kill ruling: only a provably dead custodian
// is reaped. A dead custodian past its cap reads timeout/budget-cap; a dead
// custodian within budget reads failed/process-lost; and a LIVE over-budget
// job keeps running — winding it down belongs to the kill-capable path, and
// this reaper never records a death it has not proven.
func TestReaperPassCoreTransitions(t *testing.T) {
	dir := t.TempDir()
	now := time.Unix(1786000000, 0).UTC()

	// A LIVE custodian that is over budget: untouched, no death stamp.
	liveOverBudget := writeJobRecord(t, dir, "live-over-budget", map[string]any{
		"jobId": "live-over-budget", "status": "running",
		"pid": 111, "pidStartedAt": 5, "instanceTag": "job-over",
		"startedAt": now.Add(-2 * time.Hour).Format(isoSecond), "capMin": 60,
	})
	// A DEAD custodian that is over budget: the budget verdict outranks loss.
	deadOverBudget := writeJobRecord(t, dir, "dead-over-budget", map[string]any{
		"jobId": "dead-over-budget", "status": "running",
		"pid": 666, "pidStartedAt": 4, "instanceTag": "job-dead-over",
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
		111: {start: 5, tag: "job-over"},    // over-budget custodian is LIVE
		222: {start: 6, tag: "job-healthy"}, // healthy custodian is live
		// 333, 444, 666 absent -> dead
	}
	var emitted []string
	cfg := ReaperConfig{
		JobsDir:   dir,
		Now:       func() time.Time { return now },
		Custodian: custody.liveness,
		Apply:     casApplier(t, dir),
		Emit:      func(line string) { emitted = append(emitted, line) },
	}
	if err := cfg.ReaperPass(); err != nil {
		t.Fatalf("reaper pass: %v", err)
	}

	if got := readStatus(t, liveOverBudget); got["status"] != "running" || got["groupDeathProvenAt"] != nil {
		t.Fatalf("live over-budget job must stay running with no death stamp, got %v/%v",
			got["status"], got["groupDeathProvenAt"])
	}
	if got := readStatus(t, deadOverBudget); got["status"] != "timeout" || got["error"] != "budget-cap" {
		t.Fatalf("dead over-budget job: want timeout/budget-cap, got %v/%v", got["status"], got["error"])
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

	// Every reaped record carries provenance for a death that was PROVEN.
	for _, path := range []string{deadOverBudget, lost, pendingLost} {
		if got := readStatus(t, path); got["phase"] != "supervision" || got["groupDeathProvenAt"] == nil {
			t.Fatalf("reaped record %s missing provenance: %v", path, got)
		}
	}
	// Three reap verdicts plus the live-over-budget DECLINE (review F5):
	// the state the reaper may not act on is still said out loud.
	if len(emitted) != 4 {
		t.Fatalf("expected three reap lines and one decline, got %v", emitted)
	}
	declines := 0
	for _, line := range emitted {
		if strings.Contains(line, "REAP-DECLINED job=live-over-budget") {
			declines++
		}
	}
	if declines != 1 {
		t.Fatalf("live-over-budget must decline exactly once per pass: %v", emitted)
	}
}

// The completion-after-read race: a verdict computed from a stale read must
// be void when the record moved on before the swap. The reaper reads a
// dead-custodian running record, but a completion lands before Apply; the
// expected-status compare refuses and the completion survives.
func TestReaperPassVoidsVerdictWhenRecordMovesOn(t *testing.T) {
	dir := t.TempDir()
	now := time.Unix(1786000000, 0).UTC()
	job := writeJobRecord(t, dir, "raced", map[string]any{
		"jobId": "raced", "status": "running",
		"pid": 333, "pidStartedAt": 7, "instanceTag": "job-raced",
		"startedAt": now.Add(-1 * time.Minute).Format(isoSecond), "capMin": 60,
	})
	var emitted []string
	apply := casApplier(t, dir)
	cfg := ReaperConfig{
		JobsDir:   dir,
		Now:       func() time.Time { return now },
		Custodian: func(int64, int64, string) identity.Liveness { return identity.Dead },
		Apply: func(jobID, expect, target string, patch map[string]any) (bool, error) {
			// The adapter completes the job between the reaper's read and
			// its swap attempt.
			record := readStatus(t, job)
			record["status"] = "completed"
			encoded, _ := json.MarshalIndent(record, "", "  ")
			os.WriteFile(job, append(encoded, '\n'), 0o644)
			return apply(jobID, expect, target, patch)
		},
		Emit: func(line string) { emitted = append(emitted, line) },
	}
	if err := cfg.ReaperPass(); err != nil {
		t.Fatalf("reaper pass: %v", err)
	}
	got := readStatus(t, job)
	if got["status"] != "completed" || got["error"] != nil || got["groupDeathProvenAt"] != nil {
		t.Fatalf("completion was clobbered by a stale verdict: %v", got)
	}
	if len(emitted) != 0 {
		t.Fatalf("a void verdict must not be emitted, got %v", emitted)
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
		Apply: casApplier(t, dir),
	}
	if err := cfg.ReaperPass(); err != nil {
		t.Fatalf("reaper pass: %v", err)
	}
	if got := readStatus(t, job); got["status"] != "running" {
		t.Fatalf("a job with an unreadable custodian must be deferred, got %v", got["status"])
	}
}

// A capDeadline in the past expires the budget — but only a DEAD custodian
// lets this reaper act on it.
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
			return identity.Dead
		},
		Apply: casApplier(t, dir),
	}
	if err := cfg.ReaperPass(); err != nil {
		t.Fatalf("reaper pass: %v", err)
	}
	if got := readStatus(t, job); got["status"] != "timeout" || got["error"] != "budget-cap" {
		t.Fatalf("cap-deadline job: want timeout/budget-cap, got %v/%v", got["status"], got["error"])
	}
}

func TestReaperPassClearsAbandonedSetupHusks(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 8, 10, 22, 0, 0, 0, time.UTC)
	stale := writeJobRecord(t, dir, "husk", map[string]any{
		"jobId": "husk", "status": "pending-setup", "phase": "setup",
		"createdAt": now.Add(-11 * time.Minute).Format(time.RFC3339),
	})
	fresh := writeJobRecord(t, dir, "settingup", map[string]any{
		"jobId": "settingup", "status": "pending-setup", "phase": "setup",
		"createdAt": now.Add(-1 * time.Minute).Format(time.RFC3339),
	})
	cfg := ReaperConfig{
		JobsDir:   dir,
		Now:       func() time.Time { return now },
		Custodian: func(int64, int64, string) identity.Liveness { return identity.Unknown },
		Apply:     casApplier(t, dir),
	}
	if err := cfg.ReaperPass(); err != nil {
		t.Fatalf("reaper pass: %v", err)
	}
	got := readStatus(t, stale)
	if got["status"] != "failed" || got["error"] != "abandoned-setup" {
		t.Fatalf("stale husk: want failed/abandoned-setup, got %v/%v", got["status"], got["error"])
	}
	// No process ever existed for a husk, so no death may be claimed.
	if got["groupDeathProvenAt"] != nil {
		t.Fatalf("abandoned-setup husk must not claim a death: %v", got)
	}
	if got := readStatus(t, fresh); got["status"] != "pending-setup" {
		t.Fatalf("a setup still inside its grace must be untouched, got %v", got["status"])
	}
}

// The decline is itself reportable (review F5): a running job past its cap
// with a custodian this reaper may not kill emits its state once per pass
// and the record is left untouched for the kill-capable dispatch path.
func TestReaperPassEmitsTheDeclineItCannotAct(t *testing.T) {
	dir := t.TempDir()
	now := time.Unix(1786000000, 0).UTC()
	job := writeJobRecord(t, dir, "declined", map[string]any{
		"jobId": "declined", "status": "running",
		"pid": 777, "pidStartedAt": 4, "instanceTag": "job-declined",
		"capDeadline": now.Add(-1 * time.Minute).Format(isoSecond),
	})
	within := writeJobRecord(t, dir, "budgeted", map[string]any{
		"jobId": "budgeted", "status": "running",
		"pid": 778, "pidStartedAt": 4, "instanceTag": "job-budgeted",
		"capDeadline": now.Add(30 * time.Minute).Format(isoSecond),
	})
	var emitted []string
	cfg := ReaperConfig{
		JobsDir:   dir,
		Now:       func() time.Time { return now },
		Custodian: func(int64, int64, string) identity.Liveness { return identity.Alive },
		Apply:     casApplier(t, dir),
		Emit:      func(line string) { emitted = append(emitted, line) },
	}
	if err := cfg.ReaperPass(); err != nil {
		t.Fatalf("reaper pass: %v", err)
	}
	if got := readStatus(t, job); got["status"] != "running" {
		t.Fatalf("declined job must stay running, got %v", got["status"])
	}
	if len(emitted) != 1 ||
		!strings.Contains(emitted[0], "REAP-DECLINED job=declined") ||
		!strings.Contains(emitted[0], "custodian alive") {
		t.Fatalf("decline not emitted once with its state: %v", emitted)
	}
	// A live custodian still inside its budget is unremarkable: no emit.
	if got := readStatus(t, within); got["status"] != "running" {
		t.Fatalf("budgeted job must stay running, got %v", got["status"])
	}
}

func TestReaperHonorsTheCancellingMarker(t *testing.T) {
	dir := t.TempDir()
	now := time.Unix(1786000000, 0).UTC()

	// A cancel marks the record BEFORE it kills: a dead group with
	// the marker is that cancel's outcome, never a process loss —
	// the kill-before-mark window this closes turned cancelled into
	// failed under an unlucky reaper pass (three sightings,
	// 2026-08-20).
	marked := writeJobRecord(t, dir, "marked", map[string]any{
		"jobId": "marked", "status": "running", "phase": "cancelling",
		"pid": 555, "pidStartedAt": 9, "instanceTag": "job-marked",
		"startedAt": now.Add(-1 * time.Minute).Format(isoSecond), "capMin": 60,
	})
	// The marker outranks even the budget: the operator's explicit
	// stop is the truer cause of this death.
	markedOver := writeJobRecord(t, dir, "marked-over", map[string]any{
		"jobId": "marked-over", "status": "running", "phase": "cancelling",
		"pid": 556, "pidStartedAt": 9, "instanceTag": "job-marked-over",
		"startedAt": now.Add(-2 * time.Hour).Format(isoSecond), "capMin": 60,
	})
	// A LIVE custodian with the marker: the reaper still has no kill
	// authority — the cancel path owns the kill.
	markedLive := writeJobRecord(t, dir, "marked-live", map[string]any{
		"jobId": "marked-live", "status": "running", "phase": "cancelling",
		"pid": 557, "pidStartedAt": 9, "instanceTag": "job-marked-live",
		"startedAt": now.Add(-1 * time.Minute).Format(isoSecond), "capMin": 60,
	})

	custody := fakeCustody{
		557: {start: 9, tag: "job-marked-live"}, // live; 555/556 dead
	}
	cfg := ReaperConfig{
		JobsDir:   dir,
		Now:       func() time.Time { return now },
		Custodian: custody.liveness,
		Apply:     casApplier(t, dir),
	}
	if err := cfg.ReaperPass(); err != nil {
		t.Fatalf("reaper pass: %v", err)
	}

	if got := readStatus(t, marked); got["status"] != "cancelled" {
		t.Fatalf("a dead group with the cancelling marker concludes cancelled, got %v", got["status"])
	}
	if got := readStatus(t, markedOver); got["status"] != "cancelled" {
		t.Fatalf("the marker outranks the budget verdict, got %v", got["status"])
	}
	if got := readStatus(t, markedLive); got["status"] != "running" {
		t.Fatalf("a live marked custodian stays with the cancel path, got %v", got["status"])
	}
}

func TestReaperConcludesAMarkedRecordWithNoProcess(t *testing.T) {
	dir := t.TempDir()
	now := time.Unix(1786000000, 0).UTC()
	// A cancel marked a pending record whose launch never recorded a
	// group (the ownership write voids on the marker): no custodian
	// exists to prove dead, and nothing but this pass will ever
	// conclude it. No death is claimed.
	marked := writeJobRecord(t, dir, "marked-pidless", map[string]any{
		"jobId": "marked-pidless", "status": "pending", "phase": "cancelling",
		"pid": nil, "instanceTag": "job-mp",
		"startedAt": now.Add(-1 * time.Minute).Format(isoSecond),
	})
	// An UNMARKED pidless pending record still defers — it may be
	// inside its launch handshake.
	handshaking := writeJobRecord(t, dir, "handshaking", map[string]any{
		"jobId": "handshaking", "status": "pending",
		"pid": nil, "instanceTag": "job-hs",
		"startedAt": now.Add(-1 * time.Minute).Format(isoSecond),
	})
	cfg := ReaperConfig{
		JobsDir:   dir,
		Now:       func() time.Time { return now },
		Custodian: fakeCustody{}.liveness,
		Apply:     casApplier(t, dir),
	}
	if err := cfg.ReaperPass(); err != nil {
		t.Fatalf("reaper pass: %v", err)
	}
	if got := readStatus(t, marked); got["status"] != "cancelled" || got["groupDeathProvenAt"] != nil {
		t.Fatalf("a marked pidless record concludes cancelled with no death claim: %v/%v", got["status"], got["groupDeathProvenAt"])
	}
	if got := readStatus(t, handshaking); got["status"] != "pending" {
		t.Fatalf("an unmarked handshaking record is deferred: %v", got["status"])
	}
}
