package steward

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A health remedy is read by people and agents: it names a public command
// or a plain act, never an internal entrypoint (audit EM-40).
func TestCapabilitySnapshotRemediesNameNoInternalEntrypoint(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC)
	for _, conf := range []string{"metasystem.runtimes=claude,codex\n", "metasystem.runtimes=\n", "metasystem.runtimes=claude\ncapability.snapshot-max-age-days=x\n"} {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte(conf), 0o644); err != nil {
			t.Fatal(err)
		}
		role := checkCapabilitySnapshots(root, root, now)
		if role.Status == HealthAlive {
			continue
		}
		if role.Remedy == "" || strings.Contains(role.Remedy, "internal") || strings.Contains(role.Remedy, "probe --root") {
			t.Errorf("%q: remedy %q", conf, role.Remedy)
		}
	}
}

// The hook-freshness remedy names the act that records a hook turn, never a
// re-run of the check that reported it.
func TestHookFreshnessRemedyIsNotTheCheckItself(t *testing.T) {
	t.Parallel()
	role := checkHookFreshnessAt(t.TempDir(), time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC), false)
	if role.Status == HealthAlive || strings.Contains(role.Remedy, "system check") || !strings.Contains(role.Remedy, "metasystem system") {
		t.Fatalf("hook freshness: %+v", role)
	}
}

// A health line for people carries each role's reason, not the owners'
// remedies; the public remedies are listed on their own.
func TestHealthLineWithoutRemediesOmitsThem(t *testing.T) {
	t.Parallel()
	verdict := HealthVerdict{Aggregate: "DEGRADED", Roles: []RoleVerdict{{Role: RoleHookFreshness, Status: HealthDead, Reason: "no hook turn", Remedy: "metasystem internal x"}}}
	if line := verdict.LineWithoutRemedies(); strings.Contains(line, "remedy") || !strings.Contains(line, "no hook turn") {
		t.Fatalf("line %q", line)
	}
}
