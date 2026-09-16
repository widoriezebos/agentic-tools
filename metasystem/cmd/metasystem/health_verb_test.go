package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
