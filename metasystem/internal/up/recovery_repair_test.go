package up

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginebuild"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/up/testdata/startfixture"
)

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
		binary := filepath.Join(engines, stamp)
		cmd := testenv.Go("build", "-buildvcs=false", "-ldflags", enginebuild.StampLinkerFlags(stamp), "-o", binary, "../steward/testdata/fakerunner")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("build recovery runner: %v\n%s", err, out)
		}
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
