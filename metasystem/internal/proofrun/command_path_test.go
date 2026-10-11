package proofrun

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

var testingPathStage = flag.String("testing-path-stage", "", "fixture testing command stage")

func TestDeclaredTestingCommandFindsRunningEngine(t *testing.T) {
	t.Parallel()
	if *testingPathStage == "report" {
		fmt.Print("declared testing engine ran\n")
		return
	}
	if *testingPathStage == "run" {
		args := []string{"metasystem", "-test.run=^TestDeclaredTestingCommandFindsRunningEngine$", "-test.timeout=30m", "-testing-path-stage=report"}
		for _, argv := range [][]string{args, {"/bin/sh", "-c", strings.Join(args, " ")}} {
			command, err := explicitEnvironmentCommand(t.Context(), t.TempDir(), []string{"PATH=" + t.TempDir()}, argv)
			if err != nil {
				t.Fatal(err)
			}
			output, err := command.CombinedOutput()
			if err != nil || !strings.Contains(string(output), "declared testing engine ran") {
				t.Fatalf("declared %v: %v\n%s", argv, err, output)
			}
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
	child := exec.CommandContext(t.Context(), engine, "-test.run=^TestDeclaredTestingCommandFindsRunningEngine$", "-test.timeout=30m", "-testing-path-stage=run")
	if output, err := child.CombinedOutput(); err != nil {
		t.Fatalf("testing engine without caller PATH: %v\n%s", err, output)
	}
}
