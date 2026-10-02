package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
)

// processScratchDispatchHelper runs one invocation through
// dispatchProcess in a child of this test binary, as main does, with a
// verb that takes a scratch directory and is done with it, as every site
// is: the root and its record go only by the release on the way to the
// exit code, which is dispatch's (R2).
const processScratchDispatchHelper = "test-helper-process-scratch-dispatch"

func init() {
	testHelperCommands[processScratchDispatchHelper] = func([]string) int {
		scratch := family{name: "scratchfixture", verbs: []verb{{name: "use", run: func(_ []string, stdout, stderr io.Writer) int {
			dir, done, err := diskstore.ScratchDir("verb-")
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 3
			}
			defer done()
			if err := os.WriteFile(filepath.Join(dir, "bytes"), []byte("x"), 0o600); err != nil {
				fmt.Fprintln(stderr, err)
				return 3
			}
			fmt.Fprintf(stdout, "dir=%s\n", dir)
			return 0
		}}}}
		return dispatchProcess([]string{"internal", "scratchfixture", "use"}, os.Stdout, os.Stderr, []family{scratch})
	}
}

// A process that takes scratch through a verb and ends through dispatch
// leaves nothing: no root, no record, no record lock.
func TestDispatchReleasesTheProcessScratch(t *testing.T) {
	t.Parallel()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	temp, home := filepath.Join(dir, "tmp"), filepath.Join(dir, "home")
	for _, path := range []string{temp, home} {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	command := exec.Command(commandTestExecutable(t), processScratchDispatchHelper)
	command.Env = fixtureCommandEnvironment(t, "TMPDIR="+temp, "METASYSTEM_SUPERVISION_REGISTRY_HOME="+home)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("helper: %v\n%s", err, output)
	}
	used := ""
	for _, line := range strings.Split(string(output), "\n") {
		if value, ok := strings.CutPrefix(line, "dir="); ok {
			used = value
		}
	}
	root := filepath.Dir(used)
	if used == "" || filepath.Dir(root) != filepath.Join(temp, "metasystem") {
		t.Fatalf("the verb's scratch %q is not under the process's TMPDIR/metasystem:\n%s", used, output)
	}
	if _, err := os.Lstat(root); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("dispatch left the process scratch root %s: %v", root, err)
	}
	stores := diskstore.MachineRegistry(filepath.Join(home, ".metasystem")).Dir
	entries, err := os.ReadDir(stores)
	if err != nil {
		t.Fatalf("the process registered no store: %v", err)
	}
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), ".") {
			t.Fatalf("dispatch left %s in the machine registry", entry.Name())
		}
	}
}

// The brain boot reader is an engine child writing into this process's
// scratch: it starts with TMPDIR at the scratch root, so its own scratch
// nests there, and it holds the writer lock (fd 3) while it runs.
func TestBrainBootReaderStartsInTheProcessScratch(t *testing.T) {
	t.Parallel()
	b, _ := declaredBootBed(t)
	report := filepath.Join(t.TempDir(), "child")
	deps := brainBootDependencies{
		ledgerIdentity: func(string) string { return brainBedLedger },
		inputsCommand: func(_ string, args ...string) *exec.Cmd {
			dir := args[len(args)-1]
			return exec.Command("sh", "-c", `{ printf '%s\n' "$TMPDIR" "$1"; if [ -e /dev/fd/3 ]; then echo lock; fi; } > "$2"`, "sh", dir, report)
		},
		now: func() time.Time { return time.Unix(1, 0) },
		timer: func(time.Duration) brainBootTimer {
			return brainBootTimer{C: make(chan time.Time), Stop: func() bool { return true }}
		},
	}
	if _, err := composeBrainBootWith(b.root, b.root, 1<<20, 1000, false, deps); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(report)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 || lines[2] != "lock" {
		t.Fatalf("the reader did not inherit the writer lock: %q", lines)
	}
	if tmpdir, dir := lines[0], lines[1]; filepath.Dir(dir) != tmpdir || filepath.Base(filepath.Dir(tmpdir)) != "metasystem" {
		t.Fatalf("the reader's TMPDIR %s is not the process scratch root holding its --dir %s", tmpdir, dir)
	}
}

// An in-process command run leaves the process scratch alone: the test
// binary runs many commands in one process at once, and a root released by
// one of them was replaced under the others (flaky "projection index path
// is not task-private"). Only the process's end releases it.
func TestAnInProcessDispatchKeepsTheProcessScratch(t *testing.T) {
	t.Parallel()
	before, err := diskstore.ProcessScratch()
	if err != nil {
		t.Fatal(err)
	}
	code, _, stderr := runOnOwnStreams(func(stdout, stderr io.Writer) int {
		return dispatchOn([]string{"help"}, stdout, stderr)
	})
	if code != 0 {
		t.Fatalf("help: code=%d stderr=%q", code, stderr)
	}
	if _, err := os.Lstat(before); err != nil {
		t.Fatalf("an in-process command released the process scratch %s: %v", before, err)
	}
	after, err := diskstore.ProcessScratch()
	if err != nil || after != before {
		t.Fatalf("the process scratch changed under an in-process command: %s -> %s (%v)", before, after, err)
	}
}
