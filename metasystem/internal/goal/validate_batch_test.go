package goal

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

func TestReadCommitGoalsUsesOneBatchBlobRead(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	physicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	tip := strings.Repeat("a", 40)
	rootPath := goalsPrefix + "backlog.md"
	goalPath := goalsPrefix + "batch-read.md"
	wantFiles := map[string][]byte{
		rootPath: RenderRoot(vRoot()),
		goalPath: RenderFile(vGoal("batch-read", StateQueued)),
	}
	if _, problems := ParseRoot(wantFiles[rootPath]); len(problems) != 0 {
		t.Fatalf("root fixture is invalid: %v", problems)
	}
	if _, problems := ParseFile(wantFiles[goalPath]); len(problems) != 0 {
		t.Fatalf("goal fixture is invalid: %v", problems)
	}
	payloadDir := t.TempDir()
	rootPayload := filepath.Join(payloadDir, "root.blob")
	goalPayload := filepath.Join(payloadDir, "goal.blob")
	for path, content := range map[string][]byte{rootPayload: wantFiles[rootPath], goalPayload: wantFiles[goalPath]} {
		if err := testexec.WriteFile(path, content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	binDir := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "git-calls.log")
	script := fmt.Sprintf(`#!/bin/sh
printf '%%s\n' "$*" >> %q
[ "$(pwd -P)" = %q ] || exit 97
if [ "$#" -eq 7 ] && [ "$1" = ls-tree ] && [ "$2" = -r ] &&
   [ "$3" = --name-only ] && [ "$4" = %q ] && [ "$5" = -- ] &&
   [ "$6" = %q ] && [ "$7" = %q ]; then
    printf '%%s\n' %q %q
    exit 0
fi
if [ "$#" -eq 2 ] && [ "$1" = cat-file ] && [ "$2" = --batch ]; then
    IFS= read -r first || exit 97
    IFS= read -r second || exit 97
    [ "$first" = %q ] && [ "$second" = %q ] || exit 97
    extra=''
    if IFS= read -r extra || [ -n "$extra" ]; then exit 97; fi
    printf '%%s blob %%s\n' %q %d
    /bin/cat %q || exit 97
    printf '\n'
    printf '%%s blob %%s\n' %q %d
    /bin/cat %q || exit 97
    printf '\n'
    exit 0
fi
exit 97
`, logPath, physicalRoot, tip, goalsPrefix, recordsGoalsPrefix,
		rootPath, goalPath, tip+":./"+rootPath, tip+":./"+goalPath,
		strings.Repeat("b", 40), len(wantFiles[rootPath]), rootPayload,
		strings.Repeat("c", 40), len(wantFiles[goalPath]), goalPayload)
	if err := testexec.WriteFile(filepath.Join(binDir, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	environment := testEnvironment(os.Environ(), "PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	files, err := ReadCommitGoals(root, tip, environment)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) < 2 || len(files[goalsPrefix+"backlog.md"]) == 0 {
		t.Fatalf("the batched reader returned an incomplete ledger: paths=%v", sortedKeys(files))
	}
	if len(files) != len(wantFiles) {
		t.Fatalf("the batched reader returned unexpected paths: %v", sortedKeys(files))
	}
	for path, want := range wantFiles {
		if !bytes.Equal(files[path], want) {
			t.Fatalf("the batched reader changed %s: got %q, want %q", path, files[path], want)
		}
	}
	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	calls := string(logBytes)
	if strings.Count(calls, "cat-file --batch") != 1 || strings.Contains(calls, "cat-file -p") {
		t.Fatalf("ledger blobs must use one batch process, got calls:\n%s", calls)
	}
	wantCalls := fmt.Sprintf("ls-tree -r --name-only %s -- %s %s\ncat-file --batch\n", tip, goalsPrefix, recordsGoalsPrefix)
	if calls != wantCalls {
		t.Fatalf("unexpected git calls:\n%s", calls)
	}
}
