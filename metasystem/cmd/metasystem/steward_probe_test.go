package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

var fixtureRunnerChild = flag.Bool("fixture-runner-child", false, "run the steward with a private runtime PATH")

func TestHealthCapabilitySnapshotsHaveTheStewardsAutomaticRemedy(t *testing.T) {
	t.Parallel()
	bed := newProcessBed(t)
	if err := os.WriteFile(filepath.Join(bed.root(), "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := bed.run(bed.owners(), "system", "check")
	visible := strings.Join(strings.Fields(stdout), " ")
	if code != 1 || !strings.Contains(visible, "capability-snapshots") || !strings.Contains(visible, "the steward tick probes") {
		t.Fatalf("metasystem system check lost the automatic probe remedy: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestHealthSystemCheckEndedProbeNamesThePersonsAct(t *testing.T) {
	t.Parallel()
	testEndedHealthRemedy(t, "TestTickFailedCapabilityProbeKeepsRunnerAlive", steward.RoleCapabilitySnapshots, "claude auth login")
}

func TestStewardFixtureRunnerNeverProbes(t *testing.T) {
	t.Parallel()
	if !*fixtureRunnerChild {
		path := t.TempDir()
		goBin, err := exec.LookPath("go")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(goBin, filepath.Join(path, "go")); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "claude"), []byte("fixture executable; never run\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		command := exec.Command(commandTestExecutable(t), "-test.run=^"+t.Name()+"$", "-test.timeout=30m", "-fixture-runner-child")
		command.Env = fixtureCommandEnvironment(t, "PATH="+path, "METASYSTEM_GOAL_NOW=")
		if out, err := command.CombinedOutput(); err != nil {
			t.Fatalf("fixture-mode steward run: %v\n%s", err, out)
		}
		return
	}
	if _, err := exec.LookPath("claude"); err != nil {
		t.Fatalf("fixture claude is absent from PATH: %v", err)
	}
	bed := newProcessBed(t)
	root := bed.root()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "artifacts", "agents", "capabilities")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	stale := []byte(`{"runtime":"claude","capturedAt":"2020-01-01T00:00:00Z"}`)
	path = filepath.Join(path, "claude-stale.json")
	if err := os.WriteFile(path, stale, 0o644); err != nil {
		t.Fatal(err)
	}
	probed := false
	var stdout, stderr bytes.Buffer
	code := runStewardRunWith([]string{"--repo", root}, &stdout, &stderr,
		func(got string, _ steward.WorkerCensus, _ func() error, _ time.Duration, cfg steward.TickConfig) error {
			if got != root {
				t.Fatalf("runner root %q, want %q", got, root)
			}
			if err := cfg.ProbeRuntime(root, "claude"); err == nil || err.Error() != "fixture: no probe" {
				t.Errorf("fixture runner probe = %v, want fixture: no probe", err)
			}
			return nil
		}, func(string, string) error { probed = true; return nil })
	data, err := os.ReadFile(path)
	if code != 0 || probed || err != nil || !bytes.Equal(data, stale) {
		t.Fatalf("fixture runner: code=%d adapter=%t snapshot=%s error=%v stderr=%s", code, probed, data, err, stderr.String())
	}
}

func TestHealthSystemCheckEndedLedgerExaminationNamesThePersonsAct(t *testing.T) {
	t.Parallel()
	testEndedHealthRemedy(t, "TestTickLedgerExaminationFailureStaysOnItsRole", steward.RoleLedgerAttention, "metasystem goal sync")
}

func testEndedHealthRemedy(t *testing.T, producer string, want steward.HealthRole, act string) {
	t.Helper()
	if *remedyPreviewRoot == "" {
		root, path := t.TempDir(), t.TempDir()
		goBin, err := exec.LookPath("go")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(goBin, filepath.Join(path, "go")); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "claude"), []byte("fixture executable; never run\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		binary := filepath.Join(t.TempDir(), "steward.test")
		build := exec.Command(goBin, "test", "-c", "-timeout", "30m", "-o", binary, "./internal/steward/")
		build.Dir = "../.."
		if out, err := build.CombinedOutput(); err != nil {
			t.Fatalf("build steward fixture: %v\n%s", err, out)
		}
		env := fixtureCommandEnvironment(t, "PATH="+path)
		produce := exec.Command(binary, "-test.run=^"+producer+"$", "-test.timeout=30m", "-remedy-health-root="+root, "-remedy-health-observations=5")
		produce.Env = env
		if out, err := produce.CombinedOutput(); err != nil {
			t.Fatalf("failed remedy fixture: %v\n%s", err, out)
		}
		check := exec.Command(commandTestExecutable(t), "-test.run=^"+t.Name()+"$", "-test.timeout=30m", "-remedy-preview-root="+root)
		check.Env = env
		if out, err := check.CombinedOutput(); err != nil {
			t.Fatalf("public system check after the breaker: %v\n%s", err, out)
		}
		return
	}
	b := newProcessBed(t)
	owners := b.owners()
	owners.processes.health = func(_, _ string, now time.Time) steward.HealthVerdict {
		return steward.PreviewInstalledHealth(*remedyPreviewRoot, *remedyPreviewRoot, now, remedyPreviewProbe{})
	}
	owners.processes.healthNow = func(string) (time.Time, error) {
		return time.Date(2026, 9, 20, 10, 0, 0, 500_000_000, time.UTC), nil
	}
	code, result := b.runJSON(owners, "system", "check")
	data, err := json.Marshal(result.Data)
	if err != nil {
		t.Fatal(err)
	}
	var preview steward.HookHealthPreview
	if err := json.Unmarshal(data, &preview); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, role := range preview.Verdict.Roles {
		if role.Role == want {
			found = true
			remedyMatches := strings.Contains(role.Remedy, act)
			if want == steward.RoleLedgerAttention {
				remedyMatches = role.Remedy == act
			}
			if role.Status != steward.HealthDead || role.FailureEscalation != steward.AutoHealEnded ||
				role.ConsecutiveFailures != 5 || !remedyMatches || strings.Contains(role.Remedy, "the steward") {
				t.Errorf("ended automatic remedy: %+v", role)
			}
		}
	}
	codeText, stdout, stderr := b.run(owners, "system", "check")
	visible := strings.Join(strings.Fields(stdout), " ")
	if code != 1 || codeText != 1 || !found || !strings.Contains(visible, act) || strings.Contains(visible, "the steward tick probes") {
		t.Fatalf("system check after the breaker: exit=%d/%d found=%t stdout=%q stderr=%q", code, codeText, found, stdout, stderr)
	}
}
