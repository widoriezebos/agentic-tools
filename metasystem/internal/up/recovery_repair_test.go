package up

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginebuild"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/up/testdata/startfixture"
)

func TestRecoveryAtTheRepairLimitNamesSystemRestart(t *testing.T) {
	t.Parallel()
	for _, status := range []string{steward.AutoHealEnded, "NOT_ENROLLED", "ENROLLMENT_CHANGED"} {
		t.Run(status, func(t *testing.T) {
			t.Parallel()
			options := seatUpOptions(t)
			options.RecoverOnly, options.IfDown = true, true
			if err := os.WriteFile(filepath.Join(options.Root, "metasystem.conf"), []byte("steward.runner-repair-limit=7\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			options.RepairStewardRunner = func(string) (steward.RunnerRepairOutcome, error) {
				return steward.RunnerRepairOutcome{Status: status}, nil
			}
			got := recovery(options)
			want := "run ordinary metasystem session start from a session to establish the steward generation"
			if status == steward.AutoHealEnded {
				want = "the steward runner was started 7 times without a successful tick; run metasystem system restart in this checkout, which starts the count afresh"
			}
			if got.Outcome != "recovery-partial" || got.Remedy != want || component(got, "steward-runner").Remedy != want || !strings.Contains(strings.Join(got.Lines(), "\n"), want) {
				t.Fatalf("recovery %s: %+v", status, got)
			}
		})
	}
}

func buildRecoveryRunner(t *testing.T, directory, stamp string) string {
	t.Helper()
	binary := filepath.Join(directory, stamp)
	cmd := testenv.Go("build", "-buildvcs=false", "-ldflags", enginebuild.StampLinkerFlags(stamp), "-o", binary, "../steward/testdata/fakerunner")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build recovery runner: %v\n%s", err, out)
	}
	return binary
}

func TestRecoveryLeavesACurrentRunnerAlone(t *testing.T) {
	t.Parallel()
	options := seatUpOptions(t)
	options.RecoverOnly, options.IfDown = true, true
	options.RepairStewardRunner = nil
	t.Cleanup(func() {
		if _, err := steward.Disarm(options.Root); err != nil {
			t.Errorf("disarm recovery runner: %v", err)
		}
	})
	options.Binary = buildRecoveryRunner(t, t.TempDir(), "current-source")
	installed := stageEnrollment(t, options.Root, options.Binary, 1)
	options.Executions = startfixture.Current(installed.InstallDigest)
	if got := recovery(options); got.Outcome != "recovery-started" || component(got, "steward-runner").Outcome != "restored" {
		t.Fatalf("first recovery: %+v", got)
	}
	before, alive := steward.LiveRunner(options.Root)
	if !alive {
		t.Fatalf("first recovery left no live runner: %+v", before)
	}
	got := recovery(options)
	if got.Outcome != "recovery-not-needed" || component(got, "steward-runner").Outcome != "current" {
		t.Fatalf("second recovery: %+v", got)
	}
	after, alive := steward.LiveRunner(options.Root)
	if !alive || after != before {
		t.Fatalf("second recovery changed the runner: before=%+v after=%+v alive=%t", before, after, alive)
	}
}

func TestRecoveryReplacesALiveRunnerThroughTheRealRepair(t *testing.T) {
	t.Parallel()
	options := seatUpOptions(t)
	options.RecoverOnly, options.IfDown = true, true
	options.RepairStewardRunner = nil
	t.Cleanup(func() {
		if _, err := steward.Disarm(options.Root); err != nil {
			t.Errorf("disarm recovery runner: %v", err)
		}
	})
	engines := t.TempDir()
	var previous int64
	for i, stamp := range []string{"old-source", "new-source"} {
		binary := buildRecoveryRunner(t, engines, stamp)
		installed := stageEnrollment(t, options.Root, binary, i+1)
		options.Binary = binary
		options.Executions = startfixture.Current(installed.InstallDigest)
		got := recovery(options)
		if got.Outcome != "recovery-started" || component(got, "steward-runner").Outcome != "restored" {
			t.Fatalf("recovery generation %d: %+v", i+1, got)
		}
		record, alive := steward.LiveRunner(options.Root)
		if !alive || record.Pid == previous {
			t.Fatalf("generation %d did not announce a new live runner: %+v alive=%t previous=%d", i+1, record, alive, previous)
		}
		execution := steward.ReadRunnerExecution(options.Root, nil)
		if execution.Digest != installed.InstallDigest || execution.Generation != i+1 || execution.Source != stamp {
			t.Fatalf("generation %d runs the wrong pin: %+v", i+1, execution)
		}
		if _, err := os.Stat(filepath.Join(options.Root, "artifacts", "agents", "steward", "runner-replacement.json")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("completed recovery left a transition: %v", err)
		}
		previous = record.Pid
	}
}
