package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

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
			{Role: steward.RoleRetroDebt, Status: steward.HealthDead, Reason: "retro due", Remedy: "run metasystem receipt status", NoAutomaticRemedy: true},
			{Role: steward.RoleSpendFence, Status: steward.HealthAlive, Reason: "daily spend crossed 2x", Remedy: "review the ceiling"},
			{Role: steward.RoleRepoWatcher, Status: steward.HealthDead, Reason: "watcher is dead", Remedy: "restart supervision"},
		},
		FindingDigest: strings.Repeat("d", 64),
	}
	stewardPreviewHealthAt = func(string, string, time.Time, identity.Prober) steward.HealthVerdict {
		calls++
		return verdict
	}
	// The hook's preview owner (hook_entry.go), which the retired internal
	// health --hook-preview printed through.
	run := func(asJSON bool) (string, string, int) {
		var stdout, stderr bytes.Buffer
		code := writeHookHealthPreview(t.TempDir(), "", asJSON, &stdout, &stderr)
		return stdout.String(), stderr.String(), code
	}
	plain, stderr, code := run(false)
	if code != verdict.ExitCode() || stderr != "" || plain != verdict.Line("agent")+"\n" || calls != 1 {
		t.Fatalf("text preview = code %d calls %d stdout %q stderr %q", code, calls, plain, stderr)
	}
	encoded, stderr, code := run(true)
	if code != verdict.ExitCode() || stderr != "" || calls != 2 {
		t.Fatalf("JSON preview = code %d calls %d stderr %q", code, calls, stderr)
	}
	var preview steward.HookHealthPreview
	if err := json.Unmarshal([]byte(encoded), &preview); err != nil {
		t.Fatalf("JSON preview did not decode: %q: %v", encoded, err)
	}
	if preview.SchemaVersion != 1 || preview.ExitCode != verdict.ExitCode() || preview.Line != verdict.Line("agent") || len(preview.Verdict.Roles) != 4 || len(preview.Interventions) != 4 {
		t.Fatalf("JSON preview lost health detail: %+v", preview)
	}
	if preview.Interventions[0].SupervisionRepair || preview.Interventions[1].HumanRequired || preview.Interventions[1].SupervisionRepair ||
		preview.Interventions[2].HumanRequired || preview.Interventions[2].SupervisionRepair || !preview.Interventions[3].SupervisionRepair {
		t.Fatalf("health intervention classifications drifted: %+v", preview.Interventions)
	}
}
