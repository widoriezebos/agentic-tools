package report

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunningJobDetail(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	if detail, ok := runningJobDetail(write("a.json",
		`{"jobId":"job-a","role":"implementer","runtime":"codex","status":"running"}`)); !ok ||
		detail != "implementer job-a [running, codex]" {
		t.Fatalf("running detail = %q ok=%v", detail, ok)
	}
	if detail, ok := runningJobDetail(write("stem-job.json", `{"status":"pending"}`)); !ok ||
		detail != "? stem-job [pending, ?]" {
		t.Fatalf("defaulted detail = %q ok=%v", detail, ok)
	}
	if _, ok := runningJobDetail(write("done.json", `{"jobId":"x","status":"completed"}`)); ok {
		t.Fatal("a terminal record is not running work")
	}
	// The raw-grep false positive this port kills: a nested "running" in an
	// error field is not a live status.
	if _, ok := runningJobDetail(write("nested.json",
		`{"jobId":"y","status":"failed","error":"mirror said \"status\": \"running\""}`)); ok {
		t.Fatal("a nested status string must not read as running")
	}
	if _, ok := runningJobDetail(write("broken.json", `{broken`)); ok {
		t.Fatal("an unparsable record is not provably running")
	}
}

func TestMissionRootBase(t *testing.T) {
	if got := missionRootBase("bash mission-runner.sh run-loop --root /tmp/targets/bench-a --mission m1"); got != "bench-a" {
		t.Fatalf("root base = %q", got)
	}
	if got := missionRootBase("bash mission-runner.sh run-loop --mission m1"); got != "" {
		t.Fatalf("absent --root should yield nothing, got %q", got)
	}
}

func TestComposeRunningClause(t *testing.T) {
	if got := composeRunningClause(nil, nil, false); got != "" {
		t.Fatalf("idle clause = %q", got)
	}
	got := composeRunningClause(
		[]string{"implementer job-a [running, codex]", "verifier job-b [pending, claude]"},
		[]string{"bench-a", "bench-b"}, true)
	want := "2 helper agent(s): implementer job-a [running, codex]; verifier job-b [pending, claude]" +
		", and a mission still going in bench-a, bench-b, and the test gates"
	if got != want {
		t.Fatalf("clause:\n got %q\nwant %q", got, want)
	}
}
