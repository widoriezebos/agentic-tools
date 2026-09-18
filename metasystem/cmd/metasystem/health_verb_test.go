package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

func TestHealthIsRegisteredAtTheTopLevel(t *testing.T) {
	stderr, code := captureStderr(t, func() int {
		return dispatch([]string{"health"})
	})
	if code != 2 || !strings.Contains(stderr, "health: --repo is required") || strings.Contains(stderr, "unknown family") {
		t.Fatalf("top-level health must route to the steward health implementation: code=%d stderr=%q", code, stderr)
	}
}

func TestHealthHookPreviewJSONMatchesTextFromOneEvaluation(t *testing.T) {
	originalPreview := stewardPreviewHealthAt
	originalNow := stewardHealthNow
	t.Cleanup(func() {
		stewardPreviewHealthAt = originalPreview
		stewardHealthNow = originalNow
	})
	now := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	stewardHealthNow = func() time.Time { return now }
	calls := 0
	verdict := steward.HealthVerdict{
		Schema: 1, ObservedAt: now, Aggregate: "unknown",
		Roles: []steward.RoleVerdict{
			{Role: steward.RoleStewardRunner, Status: steward.HealthUnknown, Reason: "runner state unknown", Remedy: "restart runner"},
			{Role: steward.RoleRetroDebt, Status: steward.HealthDead, Reason: "retro due", Remedy: "run scripts/receipt.sh check", NoAutomaticRemedy: true},
			{Role: steward.RoleSpendFence, Status: steward.HealthAlive, Reason: "daily spend crossed 2x", Remedy: "review the ceiling"},
			{Role: steward.RoleRepoWatcher, Status: steward.HealthDead, Reason: "watcher is dead", Remedy: "restart supervision"},
		},
		FindingDigest: strings.Repeat("d", 64),
	}
	stewardPreviewHealthAt = func(string, string, time.Time, identity.Prober) steward.HealthVerdict {
		calls++
		return verdict
	}
	run := func(args ...string) (string, string, int) {
		var stdout string
		stderr, code := captureStderr(t, func() int {
			var inner int
			stdout, inner = captureStdout(t, func() int { return runStewardHealth(args) })
			return inner
		})
		return stdout, stderr, code
	}
	plain, stderr, code := run("--repo", t.TempDir(), "--hook-preview")
	if code != verdict.ExitCode() || stderr != "" || plain != verdict.Line()+"\n" || calls != 1 {
		t.Fatalf("unflagged preview = code %d calls %d stdout %q stderr %q", code, calls, plain, stderr)
	}
	text, stderr, code := run("--repo", t.TempDir(), "--hook-preview", "--format=text")
	if code != verdict.ExitCode() || stderr != "" || text != plain || calls != 2 {
		t.Fatalf("text preview = code %d calls %d stdout %q stderr %q", code, calls, text, stderr)
	}
	encoded, stderr, code := run("--repo", t.TempDir(), "--hook-preview", "--format=json")
	if code != verdict.ExitCode() || stderr != "" || calls != 3 {
		t.Fatalf("JSON preview = code %d calls %d stderr %q", code, calls, stderr)
	}
	var preview steward.HookHealthPreview
	if err := json.Unmarshal([]byte(encoded), &preview); err != nil {
		t.Fatalf("JSON preview did not decode: %q: %v", encoded, err)
	}
	if preview.SchemaVersion != 1 || preview.ExitCode != verdict.ExitCode() || preview.Line != verdict.Line() || len(preview.Verdict.Roles) != 4 || len(preview.Interventions) != 4 {
		t.Fatalf("JSON preview lost health detail: %+v", preview)
	}
	if preview.Interventions[0].SupervisionRepair || preview.Interventions[1].HumanRequired || preview.Interventions[1].SupervisionRepair ||
		preview.Interventions[2].HumanRequired || preview.Interventions[2].SupervisionRepair || !preview.Interventions[3].SupervisionRepair {
		t.Fatalf("health intervention classifications drifted: %+v", preview.Interventions)
	}

	before := calls
	for _, args := range [][]string{
		{"--repo", t.TempDir(), "--hook-preview", "--format=yaml"},
		{"--repo", t.TempDir(), "--format=json"},
	} {
		stdout, stderr, code := run(args...)
		if code != 2 || stdout != "" || stderr == "" || calls != before {
			t.Fatalf("invalid format evaluated health: code=%d calls=%d stdout=%q stderr=%q", code, calls, stdout, stderr)
		}
	}
}

func TestHealthCommandUsesOnlyAnAuthorizedFixtureClock(t *testing.T) {
	originalPreview, originalNow := stewardPreviewHealthAt, stewardHealthNow
	t.Cleanup(func() { stewardPreviewHealthAt, stewardHealthNow = originalPreview, originalNow })
	wall := time.Date(2026, 9, 16, 18, 0, 0, 0, time.UTC)
	fixtureNow := wall.Add(time.Hour)
	stewardHealthNow = func() time.Time { return wall }
	var observed time.Time
	stewardPreviewHealthAt = func(_ string, _ string, now time.Time, _ identity.Prober) steward.HealthVerdict {
		observed = now
		return steward.HealthVerdict{Schema: 1, ObservedAt: now, Aggregate: "healthy"}
	}

	runtimeRoot := func(runtime string) string {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes="+runtime+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		return root
	}
	root, fixtureRoot := t.TempDir(), runtimeRoot("fake")
	run := func(clockRoot string, preview bool) int {
		args := []string{"--repo", root, "--metasystem-root", clockRoot}
		if preview {
			args = append(args, "--hook-preview")
		}
		_, code := captureStdout(t, func() int { return runStewardHealth(args) })
		return code
	}
	t.Setenv("METASYSTEM_GOAL_NOW", fixtureNow.Format(time.RFC3339))
	code := run(fixtureRoot, true)
	if code != 0 || !observed.Equal(fixtureNow) {
		t.Fatalf("authorized fixture clock = %v, code %d; want %v", observed, code, fixtureNow)
	}
	code = run(fixtureRoot, false)
	if code == 0 {
		t.Fatal("empty repository unexpectedly reported healthy")
	}
	recordData, err := os.ReadFile(steward.HealthRecordPath(root))
	if stamp := fixtureNow.Format(time.RFC3339); err != nil || !strings.Contains(string(recordData), `"observedAt": "`+stamp+`"`) {
		t.Fatalf("normal health observation omitted %s: read error %v; record %s", stamp, err, recordData)
	}

	t.Setenv("METASYSTEM_GOAL_NOW", "not-a-time")
	code = run(runtimeRoot("none"), true)
	if code != 0 || !observed.Equal(wall) {
		t.Fatalf("production health clock = %v, code %d; want wall clock %v", observed, code, wall)
	}
}

func TestHealthAcknowledgmentIsRegisteredAtTheTopLevel(t *testing.T) {
	stderr, code := captureStderr(t, func() int {
		return dispatch([]string{"health", "acknowledge-alert"})
	})
	if code != 2 || !strings.Contains(stderr, "--episode is required") || strings.Contains(stderr, "unknown family") {
		t.Fatalf("acknowledge-alert must route through health: code=%d stderr=%q", code, stderr)
	}
}

func TestHealthNamesTheCertainFixtureSurvivor(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	processFile := filepath.Join(t.TempDir(), "processes.json")
	t.Setenv("METASYSTEM_CENSUS_PROCESS_FILE", processFile)
	base := int64(os.Getpid()) * 10
	deadOwner := commandSurvivorRef(base+1, 201)
	liveOwner := commandSurvivorRef(base+2, 202)
	deadKey := identity.FixtureKey{Owner: deadOwner, Test: "TestHealthDead", Nonce: "00000001"}
	liveKey := identity.FixtureKey{Owner: liveOwner, Test: "TestHealthLive", Nonce: "00000002"}
	dead := commandSurvivorProcess(base+10, 210)
	dead.Exe, dead.Argv = "/tmp/fixture-dead", "/bin/sh fixture-dead"
	dead.Environ = []string{commandSurvivorTag(t, deadKey)}
	live := commandSurvivorProcess(base+11, 211)
	live.Environ = []string{commandSurvivorTag(t, liveKey)}
	liveOwnerRow := commandSurvivorProcess(liveOwner.Pid, 202)
	unreadable := commandSurvivorProcess(base+12, 212)
	unreadable.PGID, unreadable.Unreadable = dead.Pid, true
	writeCommandProcessFile(t, processFile, []census.Process{dead, live, liveOwnerRow, unreadable})

	originalObserve := stewardObserveHealth
	stewardObserveHealth = func(string, time.Time, identity.Prober) (steward.HealthVerdict, error) {
		return steward.HealthVerdict{Schema: 1, ObservedAt: time.Now(), Aggregate: "healthy"}, nil
	}
	t.Cleanup(func() { stewardObserveHealth = originalObserve })
	run := func(args ...string) (string, string, int) {
		code, stdout, stderr := captureCommandOutput(t, true, true, func() int {
			return runStewardHealth(append([]string{"--repo", root, "--metasystem-root", root}, args...))
		})
		return stdout, stderr, code
	}
	stdout, stderr, code := run()
	if code != 1 || stderr != "" || !strings.HasPrefix(stdout, "HEALTH healthy") ||
		!strings.Contains(stdout, "fixture-survivor pid=") || !strings.Contains(stdout, "exe=/tmp/fixture-dead") ||
		!strings.Contains(stdout, `argv="/bin/sh fixture-dead"`) || !strings.Contains(stdout, "fixture-survivor? pid=") ||
		strings.Contains(stdout, "TestHealthLive") {
		t.Fatalf("health survivor output = code %d stdout %q stderr %q", code, stdout, stderr)
	}

	writeCommandProcessFile(t, processFile, []census.Process{live, liveOwnerRow})
	stdout, stderr, code = run()
	if code != 0 || stderr != "" || strings.Contains(stdout, "fixture-survivor") || !strings.HasPrefix(stdout, "HEALTH healthy") {
		t.Fatalf("healthy survivor-free output = code %d stdout %q stderr %q", code, stdout, stderr)
	}
	if err := os.WriteFile(processFile, []byte("not json\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code = run()
	if code != 0 || stderr != "" || strings.Count(stdout, "process table is unreadable") != 1 {
		t.Fatalf("unreadable health table = code %d stdout %q stderr %q", code, stdout, stderr)
	}
}
