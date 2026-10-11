package plain

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

var declaredPathStage = flag.String("declared-path-stage", "", "fixture command stage")

func TestDeclaredProofAndFixFindRunningEngine(t *testing.T) {
	t.Parallel()
	if *declaredPathStage == "report" {
		fmt.Print("LANDING-CHECKED\t0\n")
		return
	}
	if *declaredPathStage == "run" {
		dir := t.TempDir()
		command := "metasystem -test.run=^TestDeclaredProofAndFixFindRunningEngine$ -test.timeout=30m -declared-path-stage=report"
		log, err := os.CreateTemp(dir, "check-*.log")
		if err != nil {
			t.Fatal(err)
		}
		defer log.Close()
		report, err := runCheck(ProveSeams{}, dir, command, Running{Log: log.Name()}, "fixture/unit", scopeDecision{}, log, &proofOutput{output: io.Discard})
		if err != nil || report.kind != "complete" {
			t.Fatalf("detached proof: %+v, %v", report, err)
		}
		home := t.TempDir()
		if err := os.MkdirAll(filepath.Dir(lane.LockPath(home)), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(Dir(dir), 0700); err != nil {
			t.Fatal(err)
		}
		err = runRegeneration(home, dir, RunningRegeneration{Goal: "fixture", Log: log.Name()}, []string{"/bin/sh", "-c", command}, dir, log, nil)
		if err != nil {
			t.Fatalf("fix check: %v", err)
		}
		return
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	engine := filepath.Join(t.TempDir(), "metasystem")
	if err := testexec.WriteFile(engine, data, 0700); err != nil {
		t.Fatal(err)
	}
	child := exec.CommandContext(t.Context(), engine, "-test.run=^TestDeclaredProofAndFixFindRunningEngine$", "-test.timeout=30m", "-declared-path-stage=run")
	child.Env = append(child.Environ(), "PATH="+t.TempDir())
	if output, err := child.CombinedOutput(); err != nil {
		t.Fatalf("engine without caller PATH: %v\n%s", err, output)
	}
}
