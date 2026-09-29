package supervise

import (
	"os"
	"strings"
	"testing"
	"time"
)

// ackRepo extends watchdogRepo's fixture table with THIS test's parent
// process, so tests have two guaranteed-live pids (self at start 100,
// parent at start 200) — kernel death vetoes fixture entries, so only
// genuinely live pids are usable in exact-token tests.
func ackRepo(t *testing.T) (repo string, self, parent int64) {
	t.Helper()
	repo = watchdogRepo(t)
	self = int64(os.Getpid())
	parent = int64(os.Getppid())
	table := `{"` + itoaTest(self) + `":{"started":100,"command":"owner"},` +
		`"` + itoaTest(parent) + `":{"started":200,"command":"parent"}}`
	if err := os.WriteFile(repo+"/proc-table.json", []byte(table), 0o644); err != nil {
		t.Fatal(err)
	}
	return repo, self, parent
}

func ackCensus(self, parent int64, completed int64) string {
	return `{"verdict":"SUCCESS","completedAtEpoch":` + itoaTest(completed) +
		`,"intervalSec":60,"fingerprint":"fp-1","inventory":[` +
		`{"class":"UNTRACKED","pid":` + itoaTest(self) + `,"pidStartedAt":100,"pidStartedAtExactMicro":100000000,"runtime":"claude"},` +
		`{"class":"UNTRACKED","pid":` + itoaTest(parent) + `,"pidStartedAt":200,"pidStartedAtExactMicro":200000000,"runtime":"claude"}]}`
}

func validAck(pid, startSec, exactMicro int64) string {
	return `{"pid":` + itoaTest(pid) + `,"pidStartedAt":` + itoaTest(startSec) +
		`,"pidStartedAtExactMicro":` + itoaTest(exactMicro) +
		`,"reason":"idle editor","acknowledgedAt":"2026-08-16T00:00:00Z"}`
}

func TestLoadAcknowledgedAbsentIsEmpty(t *testing.T) {
	acks, err := LoadAcknowledged(t.TempDir())
	if err != nil || len(acks) != 0 {
		t.Fatalf("absent record should be empty, got %d entries, err %v", len(acks), err)
	}
}

func TestLoadAcknowledgedCorruptIsError(t *testing.T) {
	repo := watchdogRepo(t)
	writeSupervisionFile(t, repo, "acknowledged-processes.json", `{not valid json`)
	if _, err := LoadAcknowledged(repo); err == nil {
		t.Fatal("a corrupt record must be an error, not a silent empty set")
	}
}

func TestLoadAcknowledgedUnreadableIsError(t *testing.T) {
	repo := watchdogRepo(t)
	if err := os.MkdirAll(acknowledgedPath(repo), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadAcknowledged(repo); err == nil {
		t.Fatal("an unreadable record path must be an error")
	}
}

// A partial or null field must be an ERROR (which the watchdog treats
// as an empty set — shout), never a zero value that silences: a
// missing field decoded as pid 0 or start 0 could match and silence
// the wrong process.
func TestLoadAcknowledgedStrictFields(t *testing.T) {
	cases := []struct{ name, body string }{
		{"missing pid", `[{"pidStartedAt":100,"pidStartedAtExactMicro":1,"reason":"r","acknowledgedAt":"2026-08-16T00:00:00Z"}]`},
		{"null start", `[{"pid":7,"pidStartedAt":null,"pidStartedAtExactMicro":1,"reason":"r","acknowledgedAt":"2026-08-16T00:00:00Z"}]`},
		{"missing exact token", `[{"pid":7,"pidStartedAt":100,"reason":"r","acknowledgedAt":"2026-08-16T00:00:00Z"}]`},
		{"missing reason", `[{"pid":7,"pidStartedAt":100,"pidStartedAtExactMicro":1,"acknowledgedAt":"2026-08-16T00:00:00Z"}]`},
		{"missing acknowledgedAt", `[{"pid":7,"pidStartedAt":100,"pidStartedAtExactMicro":1,"reason":"r"}]`},
		{"invalid acknowledgedAt", `[{"pid":7,"pidStartedAt":100,"pidStartedAtExactMicro":1,"reason":"r","acknowledgedAt":"yesterday"}]`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := watchdogRepo(t)
			writeSupervisionFile(t, repo, "acknowledged-processes.json", tc.body)
			if _, err := LoadAcknowledged(repo); err == nil {
				t.Fatal("an invalid entry must be an error")
			}
		})
	}
}

// The silence proof is the full identity: (pid, second) match AND the
// exact birth token verified by a fresh probe. A different exact token
// — a pid recycled within the same second — still shouts, and an
// unacknowledged process always shouts.
func TestWatchdogSilencesOnlyTheProvenProcess(t *testing.T) {
	repo, self, parent := ackRepo(t)
	writeSupervisionFile(t, repo, "last-census.json", ackCensus(self, parent, watchdogNow))
	writeSupervisionFile(t, repo, "state.json", healthyState())

	// Baseline: both shout.
	before := strings.Join(WatchdogReport(repo, time.Unix(watchdogNow, 0)), " ")
	if !strings.Contains(before, itoaTest(self)) || !strings.Contains(before, itoaTest(parent)) {
		t.Fatalf("expected both pids in the baseline nag, got %q", before)
	}

	// Acknowledge self with the CORRECT exact token (fixture start 100).
	writeSupervisionFile(t, repo, "acknowledged-processes.json",
		"["+validAck(self, 100, 100_000_000)+"]")
	after := strings.Join(WatchdogReport(repo, time.Unix(watchdogNow, 0)), " ")
	if strings.Contains(after, itoaTest(self)) {
		t.Fatalf("acknowledged self should be silent, got %q", after)
	}
	if !strings.Contains(after, itoaTest(parent)) {
		t.Fatalf("unacknowledged parent should still shout, got %q", after)
	}

	// The SAME (pid, second) with a DIFFERENT exact token: a recycled
	// pid within the second. Must shout.
	writeSupervisionFile(t, repo, "acknowledged-processes.json",
		"["+validAck(self, 100, 100_000_001)+"]")
	recycled := strings.Join(WatchdogReport(repo, time.Unix(watchdogNow, 0)), " ")
	if !strings.Contains(recycled, itoaTest(self)) {
		t.Fatalf("a mismatched birth token must not be silenced, got %q", recycled)
	}
}

// An acknowledged entry whose process is provably dead never silences:
// the census may still list it (a stale-but-fresh-window snapshot), but
// the exact-token probe fails on kernel death and the item shouts.
func TestWatchdogDeadAcknowledgedStillShouts(t *testing.T) {
	repo, _, _ := ackRepo(t)
	census := `{"verdict":"SUCCESS","completedAtEpoch":` + itoaTest(watchdogNow) +
		`,"intervalSec":60,"fingerprint":"fp-1","inventory":[` +
		`{"class":"UNTRACKED","pid":999999,"pidStartedAt":5,"runtime":"claude"}]}`
	writeSupervisionFile(t, repo, "last-census.json", census)
	writeSupervisionFile(t, repo, "state.json", healthyState())
	writeSupervisionFile(t, repo, "acknowledged-processes.json",
		"["+validAck(999999, 5, 5_000_000)+"]")
	out := strings.Join(WatchdogReport(repo, time.Unix(watchdogNow, 0)), " ")
	if !strings.Contains(out, "999999") {
		t.Fatalf("a dead acknowledged pid must not be silenced, got %q", out)
	}
}
