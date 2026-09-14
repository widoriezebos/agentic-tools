package main

import (
	"encoding/json"
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

func TestHealthAcknowledgmentIsRegisteredAtTheTopLevel(t *testing.T) {
	stderr, code := captureStderr(t, func() int {
		return dispatch([]string{"health", "acknowledge-alert"})
	})
	if code != 2 || !strings.Contains(stderr, "--episode is required") || strings.Contains(stderr, "unknown family") {
		t.Fatalf("acknowledge-alert must route through health: code=%d stderr=%q", code, stderr)
	}
}
