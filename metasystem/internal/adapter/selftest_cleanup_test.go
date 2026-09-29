package adapter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
)

// selftestCleanupBed is a self-test directory and the seams its cleanup
// reads: job records by id, a custody-death answer per job, a cancel that
// records its calls, and a clock the cancel poll advances.
type selftestCleanupBed struct {
	dir       string
	records   map[string]map[string]any
	death     map[string]dispatch.CustodyDeathResult
	cancelled []string
	log       strings.Builder
	params    SelftestParams
}

func newSelftestCleanupBed(t *testing.T) *selftestCleanupBed {
	t.Helper()
	bed := &selftestCleanupBed{dir: t.TempDir(), records: map[string]map[string]any{}, death: map[string]dispatch.CustodyDeathResult{}}
	if err := os.WriteFile(filepath.Join(bed.dir, "brief.md"), []byte("brief\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bed.params = SelftestParams{
		Runtime: "stub", TurnCeilingSec: 2,
		Status: func(job string) string { status, _ := bed.records[job]["status"].(string); return status },
		readRecord: func(job string) (map[string]any, error) {
			record, found := bed.records[job]
			if !found {
				return nil, os.ErrNotExist
			}
			return record, nil
		},
		custodyDeath: func(record map[string]any) dispatch.CustodyDeathResult {
			return bed.death[record["jobId"].(string)]
		},
		cancel:     func(job string) error { bed.cancelled = append(bed.cancelled, job); return nil },
		cleanupLog: &bed.log,
	}
	bed.withFakeClock()
	return bed
}

func (bed *selftestCleanupBed) job(id, status string, fields map[string]any) {
	record := map[string]any{"jobId": id, "status": status, "pid": float64(4242)}
	for key, value := range fields {
		record[key] = value
	}
	bed.records[id] = record
}

// withFakeClock gives the bed an artificial clock the cancel poll advances.
func (bed *selftestCleanupBed) withFakeClock() {
	current := time.Date(2026, 9, 29, 1, 0, 0, 0, time.UTC)
	bed.params.clockNow = func() time.Time { return current }
	bed.params.clockSleep = func(interval time.Duration) { current = current.Add(interval) }
}

// Every job terminal with the reaper's recorded group-death proof, or proven
// dead now, removes the directory (disk-lifetimes A10, DL3A-06).
func TestSelftestCleanupRemovesTheDirectoryOnlyWhenEveryJobIsProvenDead(t *testing.T) {
	t.Parallel()
	bed := newSelftestCleanupBed(t)
	bed.job("main", "completed", map[string]any{"groupDeathProvenAt": "2026-09-29T00:59:00Z"})
	bed.job("cancel", "cancelled", nil)
	bed.death["cancel"] = dispatch.CustodyDeathResult{Outcome: dispatch.CustodyDeathProven}
	if kept := bed.params.settleSelftest(bed.dir, []string{"main", "cancel", "never-dispatched"}); kept {
		t.Fatalf("a proven-dead self-test kept its directory: %s", bed.log.String())
	}
	if _, err := os.Stat(bed.dir); !os.IsNotExist(err) {
		t.Fatalf("the directory survived: %v", err)
	}
}

// ALIVE and DEFERRED keep the directory and print the path, each job id
// and its outcome with the reason; a terminal status alone is no proof.
func TestSelftestCleanupKeepsTheDirectoryWhileAJobMayRun(t *testing.T) {
	t.Parallel()
	for _, outcome := range []dispatch.CustodyDeathResult{
		{Outcome: dispatch.CustodyDeathAlive, Reason: "primary-custodian-alive"},
		{Outcome: dispatch.CustodyDeathDeferred, Reason: "tag-position-proof-unavailable"},
	} {
		bed := newSelftestCleanupBed(t)
		bed.job("main", "completed", nil)
		bed.death["main"] = outcome
		if kept := bed.params.settleSelftest(bed.dir, []string{"main"}); !kept {
			t.Fatalf("%s: the directory was not kept", outcome.Outcome)
		}
		if _, err := os.Stat(filepath.Join(bed.dir, "brief.md")); err != nil {
			t.Fatalf("%s: the kept directory lost its content: %v", outcome.Outcome, err)
		}
		line := bed.log.String()
		for _, want := range []string{bed.dir, "main", string(outcome.Outcome), outcome.Reason} {
			if !strings.Contains(line, want) {
				t.Fatalf("%s: the kept line %q lacks %q", outcome.Outcome, line, want)
			}
		}
	}
}

// A job still live when the self-test ends (a probe failure after dispatch)
// is cancelled and polled; when the turn ceiling passes it is still live,
// the directory is kept. A second settle on the kept directory neither
// deletes nor recreates it (R-129).
func TestSelftestCleanupCancelsALiveJobAndKeepsPastTheCeiling(t *testing.T) {
	t.Parallel()
	bed := newSelftestCleanupBed(t)
	bed.job("main", "running", nil)
	if kept := bed.params.settleSelftest(bed.dir, []string{"main"}); !kept {
		t.Fatal("a live job's directory was removed")
	}
	if len(bed.cancelled) != 1 || bed.cancelled[0] != "main" {
		t.Fatalf("cancel calls = %v", bed.cancelled)
	}
	if !strings.Contains(bed.log.String(), "running") {
		t.Fatalf("the kept line does not say the job still runs: %q", bed.log.String())
	}
	before, err := os.Stat(bed.dir)
	if err != nil {
		t.Fatal(err)
	}
	if kept := bed.params.settleSelftest(bed.dir, []string{"main"}); !kept {
		t.Fatal("a second settle removed the kept directory")
	}
	after, err := os.Stat(bed.dir)
	if err != nil || !os.SameFile(before, after) || !after.ModTime().Equal(before.ModTime()) {
		t.Fatalf("a second settle recreated or touched the kept directory: %v", err)
	}

	// Cancelled into a terminal state and proven dead: removed.
	bed.params.cancel = func(job string) error { bed.records[job]["status"] = "cancelled"; return nil }
	bed.death["main"] = dispatch.CustodyDeathResult{Outcome: dispatch.CustodyDeathProven}
	if kept := bed.params.settleSelftest(bed.dir, []string{"main"}); kept {
		t.Fatalf("a cancelled, proven-dead job kept the directory: %s", bed.log.String())
	}
}
