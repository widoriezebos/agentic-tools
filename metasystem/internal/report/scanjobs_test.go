package report

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// scanFixture builds a jobs dir; files get mtimes now minus the given ages.
type scanFixture struct {
	t     *testing.T
	dir   string
	now   time.Time
	state string
	run   string
}

func newScanFixture(t *testing.T) *scanFixture {
	t.Helper()
	base := t.TempDir()
	return &scanFixture{
		t: t, dir: filepath.Join(base, "jobs"), now: time.Unix(1786000000, 0),
		state: filepath.Join(base, "seen.state"), run: filepath.Join(base, "running"),
	}
}

func (f *scanFixture) write(name, body string, ageMin int64) string {
	f.t.Helper()
	if err := os.MkdirAll(f.dir, 0o755); err != nil {
		f.t.Fatal(err)
	}
	path := filepath.Join(f.dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		f.t.Fatal(err)
	}
	stamp := f.now.Add(-time.Duration(ageMin) * time.Minute)
	if err := os.Chtimes(path, stamp, stamp); err != nil {
		f.t.Fatal(err)
	}
	return path
}

func (f *scanFixture) scan(baseline bool) string {
	f.t.Helper()
	var out strings.Builder
	err := ScanJobs(ScanJobsParams{
		Dirs: []string{f.dir}, StateFile: f.state, RunningFile: f.run,
		ScopeField: "workspaceRoot",
		StaleMin:   30, CapMin: 120, StartVerifyMin: 10,
		Baseline: baseline, Now: f.now,
	}, &out)
	if err != nil {
		f.t.Fatal(err)
	}
	return out.String()
}

func TestScanJobsVerdicts(t *testing.T) {
	f := newScanFixture(t)
	doneAt := f.write("done-job.json", `{"status":"completed"}`, 5)
	cappedAt := f.write("capped-job.json", `{"status":"running"}`, 200)
	queuedAt := f.write("queued-job.json", `{"status":"queued"}`, 15)
	staleAt := f.write("stale-job.json", `{"status":"running"}`, 45)
	f.write("live-job.json", `{"status":"running"}`, 1)

	got := f.scan(false)
	want := "DONE done-job status=completed age=5m record=" + doneAt + "\n" +
		"CAPPED capped-job status=running age=200m record=" + cappedAt + "\n" +
		"NEVER-STARTED queued-job status=queued age=15m record=" + queuedAt + "\n" +
		"STALE stale-job status=running age=45m record=" + staleAt + "\n"
	// Glob order is lexical; rebuild the expectation in that order.
	lines := map[string]string{}
	for _, line := range strings.Split(strings.TrimSuffix(want, "\n"), "\n") {
		lines[strings.Fields(line)[1]] = line
	}
	expected := lines["capped-job"] + "\n" + lines["done-job"] + "\n" +
		lines["queued-job"] + "\n" + lines["stale-job"] + "\n"
	if got != expected {
		t.Fatalf("report lines:\n got %q\nwant %q", got, expected)
	}

	// Marked ids never report twice; the live job carries into the running
	// set and VANISHES when its record disappears.
	if second := f.scan(false); second != "" {
		t.Fatalf("second pass must be quiet, got %q", second)
	}
	if err := os.Remove(filepath.Join(f.dir, "live-job.json")); err != nil {
		t.Fatal(err)
	}
	third := f.scan(false)
	if !strings.HasPrefix(third, "VANISHED live-job status=running age=0m record=") {
		t.Fatalf("vanished line = %q", third)
	}

	state, _ := os.ReadFile(f.state)
	for _, id := range []string{"done-job", "capped-job", "queued-job", "stale-job", "live-job"} {
		if !strings.Contains(string(state), id+"\n") {
			t.Fatalf("state file lost %s: %q", id, state)
		}
	}
}

func TestScanJobsBaselineAdoptsHistory(t *testing.T) {
	f := newScanFixture(t)
	f.write("old-done.json", `{"status":"completed"}`, 5)
	f.write("old-stale.json", `{"status":"running"}`, 45)
	if got := f.scan(true); got != "" {
		t.Fatalf("baseline must report nothing, got %q", got)
	}
	state, _ := os.ReadFile(f.state)
	if !strings.Contains(string(state), "old-done\n") {
		t.Fatal("baseline must mark terminal history")
	}
	// The shipped baseline leaves STALE unmarked: it reports on the next
	// live pass instead of being silently adopted.
	if strings.Contains(string(state), "old-stale") {
		t.Fatal("baseline must not adopt a stale record")
	}
	if got := f.scan(false); !strings.HasPrefix(got, "STALE old-stale ") {
		t.Fatalf("post-baseline pass = %q", got)
	}
}

func TestScanJobsSidecarAndScope(t *testing.T) {
	f := newScanFixture(t)
	// The .log sidecar carries no fields; the .json primary does. Only the
	// fields-carrying record reports, and its fresher sidecar mtime keeps
	// the job live past the record's own age.
	f.write("job-a.json", `{"status":"running","workspaceRoot":"/scope/repo"}`, 45)
	f.write("job-a.log", "streaming progress", 1)
	// A foreign workspace stays unreported under scope.
	f.write("job-b.json", `{"status":"running","workspaceRoot":"/elsewhere/repo"}`, 45)

	var out strings.Builder
	err := ScanJobs(ScanJobsParams{
		Dirs: []string{f.dir}, StateFile: f.state, RunningFile: f.run,
		Scope: "/scope/repo", ScopeField: "workspaceRoot",
		StaleMin: 30, CapMin: 120, StartVerifyMin: 10,
		Now: f.now,
	}, &out)
	if err != nil {
		t.Fatal(err)
	}
	if out.String() != "" {
		t.Fatalf("sibling mtime must keep job-a live and scope must drop job-b, got %q", out.String())
	}
}

func TestScanJobsNonJSONRecordGetsMtimeVerdictsOnly(t *testing.T) {
	f := newScanFixture(t)
	// The header's promise: a non-JSON record ages into
	// STALE/CAPPED on mtime alone and never takes a status-based verdict —
	// NEVER-STARTED's empty-status leg requires a record that parses.
	notJSON := f.write("raw-log-job.txt", "not json at all", 45)
	got := f.scan(false)
	if got != "STALE raw-log-job status=running age=45m record="+notJSON+"\n" {
		t.Fatalf("non-JSON verdict = %q", got)
	}
}

func TestScanJobsBaselineSuppressesVanished(t *testing.T) {
	f := newScanFixture(t)
	f.write("fleeting.json", `{"status":"running"}`, 1)
	f.scan(false)
	if err := os.Remove(filepath.Join(f.dir, "fleeting.json")); err != nil {
		t.Fatal(err)
	}
	if got := f.scan(true); got != "" {
		t.Fatalf("baseline must not report VANISHED, got %q", got)
	}
	state, _ := os.ReadFile(f.state)
	if strings.Contains(string(state), "fleeting") {
		t.Fatal("baseline must not mark a vanished id")
	}
}

func TestScanJobsRefusesUnwritableState(t *testing.T) {
	f := newScanFixture(t)
	f.write("job.json", `{"status":"running"}`, 1)
	err := ScanJobs(ScanJobsParams{
		Dirs: []string{f.dir}, StateFile: filepath.Join(f.dir, "no", "such", "dir", "s"),
		RunningFile: f.run, StaleMin: 30, CapMin: 120, Now: f.now,
	}, &strings.Builder{})
	if err == nil || !strings.Contains(err.Error(), "cannot append to the state file") {
		t.Fatalf("unwritable state must refuse: %v", err)
	}
}

func TestScanJobsRefusesEmptyThresholds(t *testing.T) {
	f := newScanFixture(t)
	err := ScanJobs(ScanJobsParams{
		Dirs: []string{f.dir}, StateFile: f.state, RunningFile: f.run,
		StaleMin: 0, CapMin: 120, Now: f.now,
	}, &strings.Builder{})
	if err == nil || !strings.Contains(err.Error(), "thresholds must be positive") {
		t.Fatalf("an unset stale-min silently disabled STALE in the shell; the engine must refuse: %v", err)
	}
}
