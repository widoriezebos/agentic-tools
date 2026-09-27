package report

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type watchBed struct {
	options  WatchJobsOptions
	out, err strings.Builder
	jobs     string
}

func newWatchBed(t *testing.T) *watchBed {
	t.Helper()
	base := t.TempDir()
	bed := &watchBed{jobs: filepath.Join(base, "jobs")}
	if err := os.MkdirAll(bed.jobs, 0o755); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	bed.options = WatchJobsOptions{
		Dirs: []string{bed.jobs}, ScopeField: "workspaceRoot", TempDir: base,
		StaleMin: 20, CapMin: 180, StartVerifyMin: 5, Interval: time.Second, Once: true, Fingerprint: "fixture",
		Now: func() time.Time { return now }, Sleep: func(time.Duration) {},
	}
	bed.options.Out, bed.options.Err = &bed.out, &bed.err
	return bed
}

func (bed *watchBed) write(t *testing.T, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(bed.jobs, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (bed *watchBed) run(t *testing.T) (string, int) {
	t.Helper()
	bed.out.Reset()
	bed.err.Reset()
	code := WatchJobs(bed.options)
	return bed.out.String(), code
}

// Ported from the watch-background-jobs section: an armed (existing) state
// reports a terminal job once and never re-reports it; a fresh state file
// adopts history instead of flooding; an explicit baseline suppresses every
// pre-existing job; a watcher without --dir is a usage error.
func TestWatchJobsReportsOnceAndBaselinesHistory(t *testing.T) {
	t.Parallel()
	bed := newWatchBed(t)
	bed.write(t, "done.json", `{"status":"completed"}`)
	bed.write(t, "live.json", `{"status":"running"}`)
	armed := filepath.Join(t.TempDir(), "armed.state")
	if err := os.WriteFile(armed, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	bed.options.StateFile = armed
	out, code := bed.run(t)
	if code != 0 || !strings.Contains(out, "\nDONE done status=completed") || strings.Contains(out, "live") ||
		!strings.HasPrefix(out, "ARMED watcher fp=fixture ") || !strings.Contains(out, "stale=20m cap=180m start-verify=5m auto-baseline=0") {
		t.Fatalf("first pass code=%d:\n%s", code, out)
	}
	out, _ = bed.run(t)
	if strings.Count(out, "\n") != 1 {
		t.Fatalf("an already-reported job was reported again:\n%s", out)
	}

	bed.options.StateFile = filepath.Join(t.TempDir(), "fresh.state")
	out, _ = bed.run(t)
	if strings.Contains(out, "DONE") || !strings.Contains(out, "auto-baseline=1") || !strings.Contains(bed.err.String(), "auto-baselined historical jobs") {
		t.Fatalf("fresh state replayed history:\n%s\n%s", out, bed.err.String())
	}

	bed.options.StateFile = filepath.Join(t.TempDir(), "explicit.state")
	bed.options.Baseline = true
	if _, code := bed.run(t); code != 0 || !strings.Contains(bed.err.String(), "baseline recorded in") {
		t.Fatalf("baseline code=%d stderr=%q", code, bed.err.String())
	}
	bed.options.Baseline = false
	if out, _ := bed.run(t); strings.Contains(out, "DONE done") {
		t.Fatalf("baseline did not suppress pre-existing jobs:\n%s", out)
	}

	bed.options.Dirs = nil
	if _, code := bed.run(t); code != 2 {
		t.Fatalf("no --dir accepted: code=%d", code)
	}
}

// Distinct scopes derive distinct default state files, so one project's
// reports never suppress another's; each reports a job that arrives after its
// own arming.
func TestWatchJobsDefaultStateIsPerScope(t *testing.T) {
	t.Parallel()
	bed := newWatchBed(t)
	if DefaultWatchJobsState(bed.options.TempDir, bed.options.Dirs, "/r/mine") ==
		DefaultWatchJobsState(bed.options.TempDir, bed.options.Dirs, "/r/other") {
		t.Fatal("distinct scopes share a default state file")
	}
	for _, scope := range []string{"/r/mine", "/r/other"} {
		bed.options.Scope = scope
		bed.run(t)
	}
	bed.write(t, "nu-mine.json", `{"status":"completed","workspaceRoot":"/r/mine"}`)
	bed.write(t, "nu-other.json", `{"status":"completed","workspaceRoot":"/r/other"}`)
	for _, scope := range []string{"mine", "other"} {
		bed.options.Scope = "/r/" + scope
		if out, _ := bed.run(t); !strings.Contains(out, "\nDONE nu-"+scope+" ") || strings.Count(out, "\nDONE ") != 1 {
			t.Fatalf("scope %s after arming:\n%s", scope, out)
		}
	}
}

// Report lines carry the deepest live suite heartbeat as a prefix, and a
// pass with nothing to report still prints the heartbeat.
func TestWatchJobsPrefixesTheSuiteHeartbeat(t *testing.T) {
	t.Parallel()
	bed := newWatchBed(t)
	bed.options.StateFile = filepath.Join(t.TempDir(), "state")
	if err := os.WriteFile(bed.options.StateFile, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	bed.options.Scope = "/r/mine"
	bed.options.Heartbeat = func() string { return "inner:child since 3min" }
	bed.write(t, "job.json", `{"status":"completed","workspaceRoot":"/r/mine"}`)
	out, _ := bed.run(t)
	if !strings.Contains(out, "\ninner:child since 3min DONE job status=completed") {
		t.Fatalf("heartbeat prefix missing:\n%s", out)
	}
	out, _ = bed.run(t)
	if !strings.HasSuffix(out, "\ninner:child since 3min\n") {
		t.Fatalf("quiet pass lost the heartbeat:\n%s", out)
	}
}

// The census runs before every pass: its CENSUS-SLOW warnings reach stderr,
// its output accrues in the census log, which rotates once past its bound,
// and a refused census ends the watcher.
func TestWatchJobsCensusLogsRotatesAndStopsOnRefusal(t *testing.T) {
	t.Parallel()
	bed := newWatchBed(t)
	bed.options.StateFile = filepath.Join(t.TempDir(), "state")
	if err := os.WriteFile(bed.options.StateFile, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(t.TempDir(), "census.log")
	bed.options.CensusLog, bed.options.CensusLogMaxBytes = log, 60
	passes := 0
	bed.options.Census = func() (string, error) {
		passes++
		return "WARNING CENSUS-SLOW durationMs=9 intervalMs=1 budgetPercent=50 budgetMs=0 defect=none\n", nil
	}
	if _, code := bed.run(t); code != 0 || !strings.Contains(bed.err.String(), "WARNING CENSUS-SLOW") {
		t.Fatalf("census pass code=%d stderr=%q", code, bed.err.String())
	}
	bed.run(t)
	if _, err := os.Stat(log + ".1"); err != nil || passes != 2 {
		t.Fatalf("census log did not rotate past its bound (passes=%d): %v", passes, err)
	}

	bed.options.Census = func() (string, error) {
		return "live census writer already owns the lock\n", errors.New("refused")
	}
	if _, code := bed.run(t); code != 1 || !strings.Contains(bed.err.String(), "live census writer already owns") {
		t.Fatalf("refused census code=%d stderr=%q", code, bed.err.String())
	}
	data, _ := os.ReadFile(log)
	if !strings.Contains(string(data), "live census writer already owns") {
		t.Fatalf("refusal missing from the census log: %q", data)
	}
}

// Without --once the watcher polls at its interval until stopped.
func TestWatchJobsPollsAtItsIntervalUntilStopped(t *testing.T) {
	t.Parallel()
	bed := newWatchBed(t)
	bed.options.StateFile = filepath.Join(t.TempDir(), "state")
	if err := os.WriteFile(bed.options.StateFile, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	bed.options.Once = false
	bed.options.Interval = 7 * time.Second
	var slept []time.Duration
	bed.options.Sleep = func(wait time.Duration) { slept = append(slept, wait) }
	passes := 0
	bed.options.Stop = func() bool { passes++; return passes == 3 }
	if _, code := bed.run(t); code != 0 || len(slept) != 2 || slept[0] != 7*time.Second {
		t.Fatalf("code=%d slept=%v", code, slept)
	}
}
