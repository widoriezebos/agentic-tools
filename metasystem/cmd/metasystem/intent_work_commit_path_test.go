package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

var workCommitPathStage = flag.String("work-commit-path-stage", "", "fixture commit command stage")

func TestWorkCommitFindsRunningEngineWithoutCallerPathGitAdapter(t *testing.T) {
	t.Parallel()
	if *workCommitPathStage == "report" {
		fmt.Print("engine command ran\n")
		return
	}
	if *workCommitPathStage == "run" {
		c, owners := readPublicationAdapterBed(t)
		writeUnitCarryFile(t, filepath.Join(c.worktree, "metasystem.conf"), "proof.cheap=metasystem -test.run=^TestWorkCommitFindsRunningEngineWithoutCallerPathGitAdapter$ -test.timeout=30m -work-commit-path-stage=report\n")
		connectionGit(t, c.worktree, "add", "metasystem.conf")
		connectionGit(t, c.worktree, "commit", "-qm", "goal "+c.id+" units settings\n\nGoal-Unit: "+c.id+"/settings")
		writeUnitCarryFile(t, filepath.Join(c.worktree, "hand.txt"), "hand change\n")
		connectionGit(t, c.worktree, "add", "hand.txt")
		owners.connection.rebaseGate = func(string) (string, error) { return "static-green", nil }
		code, result := c.runJSON(owners, "work", "commit", c.id, "--work", "hand")
		if code != 0 || c.commits != 1 {
			t.Fatalf("commit's declared check: exit=%d result=%+v", code, result)
		}
		return
	}
	data, err := os.ReadFile(commandTestExecutable(t))
	if err != nil {
		t.Fatal(err)
	}
	engine := filepath.Join(t.TempDir(), "metasystem")
	if err := testexec.WriteFile(engine, data, 0700); err != nil {
		t.Fatal(err)
	}
	child := exec.CommandContext(t.Context(), engine, "-test.run=^TestWorkCommitFindsRunningEngineWithoutCallerPathGitAdapter$", "-test.timeout=30m", "-work-commit-path-stage=run")
	// Git's index and scratch commit are the adapter's real boundary. The
	// engine is absent from these system directories.
	child.Env = fixtureCommandEnvironment(t, "PATH=/usr/bin:/bin")
	if output, err := child.CombinedOutput(); err != nil {
		t.Fatalf("commit engine without caller PATH: %v\n%s", err, output)
	}
}
