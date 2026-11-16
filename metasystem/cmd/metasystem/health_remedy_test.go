package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

var remedyPreviewRoot = flag.String("remedy-preview-root", "", "read the steward failed-probe fixture")

type remedyPreviewProbe struct{}

func (remedyPreviewProbe) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	return identity.Exact{Pid: pid, StartedAt: time.Unix(1_700_000_000+pid, 0), StartTicks: pid * 10, BootID: "health-bed-boot"}, identity.Alive, nil
}

func TestHealthSystemCheckFailedProbeKeepsStewardGreen(t *testing.T) {
	t.Parallel()
	if *remedyPreviewRoot == "" {
		root, path := t.TempDir(), t.TempDir()
		if err := testexec.WriteFile(filepath.Join(path, "claude"), []byte("fixture executable; never run\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		goBin, err := exec.LookPath("go")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(goBin, filepath.Join(path, "go")); err != nil {
			t.Fatal(err)
		}
		binary := stewardTestBinary(t)
		env := os.Environ()
		for index, value := range env {
			if strings.HasPrefix(value, "PATH=") {
				env[index] = "PATH=" + path
			}
		}
		produce := exec.Command(binary, "-test.run=^TestTickFailedCapabilityProbeKeepsRunnerAlive$", "-test.timeout=30m", "-remedy-health-root="+root)
		produce.Env = env
		// A failing producer still writes its terminal evidence. Reading that
		// evidence through the public command is this test's assertion.
		out, producerErr := produce.CombinedOutput()
		if _, err := os.Stat(steward.ComponentEvidencePath(root, "steward-tick")); err != nil {
			t.Fatalf("fixture produced no tick: %v %v\n%s", err, producerErr, out)
		}
		self, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		check := exec.Command(self, "-test.run=^TestHealthSystemCheckFailedProbeKeepsStewardGreen$", "-test.timeout=30m", "-remedy-preview-root="+root)
		check.Env = env
		if out, err := check.CombinedOutput(); err != nil {
			t.Fatalf("public system check: %v\n%s", err, out)
		}
		if producerErr != nil {
			t.Fatalf("steward fixture: %v\n%s", producerErr, out)
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
	data, _ := json.Marshal(result.Data)
	var preview steward.HookHealthPreview
	if err := json.Unmarshal(data, &preview); err != nil {
		t.Fatal(err)
	}
	found := 0
	for _, role := range preview.Verdict.Roles {
		switch role.Role {
		case steward.RoleStewardRunner:
			found++
			if role.Status != steward.HealthAlive {
				t.Errorf("system check steward row: %+v", role)
			}
		case steward.RoleCapabilitySnapshots:
			found++
			if role.Status != steward.HealthDead || !strings.Contains(role.Reason, "fixture login unavailable") {
				t.Errorf("system check snapshots row: %+v", role)
			}
		}
	}
	if code != 1 || found != 2 {
		t.Fatalf("system check: code=%d roles=%+v", code, preview.Verdict.Roles)
	}
}

var stewardTestBuild struct {
	once sync.Once
	path string
	err  error
}

// stewardTestBinary is the health producer shared by this command test run.
func stewardTestBinary(t *testing.T) string {
	t.Helper()
	stewardTestBuild.once.Do(func() {
		directory, err := testenv.MkdirTemp("steward-test-")
		if err != nil {
			stewardTestBuild.err = err
			return
		}
		stewardTestBuild.path = filepath.Join(directory, "steward.test")
		build := testenv.Go("test", "-c", "-timeout", "30m", "-o", stewardTestBuild.path, "./internal/steward")
		build.Dir = "../.."
		if output, err := build.CombinedOutput(); err != nil {
			stewardTestBuild.err = fmt.Errorf("build steward fixture: %w\n%s", err, output)
		}
	})
	if stewardTestBuild.err != nil {
		t.Fatal(stewardTestBuild.err)
	}
	return stewardTestBuild.path
}
