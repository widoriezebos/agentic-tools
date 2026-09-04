package goal

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadCommitGoalsUsesOneBatchBlobRead(t *testing.T) {
	_, root := oneClone(t)
	seedLedger(t, root)
	opened, err := Open(verbReq(root, "01J5X00000000000000000B001", "mac-a"), "batch-read", "Exercise one batch read.", "main", "Read it.")
	if err != nil || opened.Outcome != OutcomeConfirmed {
		t.Fatalf("open batch-read goal: %+v %v", opened, err)
	}
	tip := opened.Tip
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	binDir := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "git-calls.log")
	wrapper := filepath.Join(binDir, "git")
	script := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' \"$*\" >> %q\nexec %q \"$@\"\n", logPath, realGit)
	if err := os.WriteFile(wrapper, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	files, err := ReadCommitGoals(root, tip)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) < 2 || len(files[goalsPrefix+"backlog.md"]) == 0 {
		t.Fatalf("the batched reader returned an incomplete ledger: paths=%v", sortedKeys(files))
	}
	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	calls := string(logBytes)
	if strings.Count(calls, "cat-file --batch") != 1 || strings.Contains(calls, "cat-file -p") {
		t.Fatalf("ledger blobs must use one batch process, got calls:\n%s", calls)
	}
}
