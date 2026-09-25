package main

import (
	"crypto/sha1"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// Each CLI leg has its own transcript. The accepted blobs come from the
// in-memory admission repository, not from a physical Git object database.
type hostLoadGitFixture struct {
	root, top, tip, dir, packageDir string
	now                             time.Time
}

func hostLoadGitNext(kind string) []string {
	switch kind {
	case "":
		return []string{"top"}
	case "top":
		return []string{"remote", "machine"}
	case "pinned-top", "machine":
		return []string{"remote"}
	case "remote":
		return []string{"branch"}
	case "branch":
		return []string{"tip"}
	case "tip":
		return []string{"tree"}
	case "tree":
		return []string{"blob"}
	case "blob":
		return []string{"log", "remote", "top", "pinned-top"}
	case "log":
		return []string{"machine", "remote", "top", "pinned-top"}
	}
	return nil
}

func newHostLoadGitFixture(t *testing.T, root, tip string, now time.Time, accepted map[string][]byte) hostLoadGitFixture {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	shim := "#!/bin/sh\nexec \"$HOSTLOAD_GIT_HELPER\" -test.run=^TestHostLoadGitHelper$ -- \"$@\"\n"
	if err := testexec.WriteFile(filepath.Join(bin, "git"), []byte(shim), 0o755); err != nil {
		t.Fatal(err)
	}
	deny := filepath.Join(dir, "deny")
	if err := os.Mkdir(deny, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(filepath.Join(deny, "git"), []byte("#!/bin/sh\nprintf 'proof fixture Git shim is missing\\n' >&2\nexit 97\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"backlog.md", "standing-validation.md"} {
		blob := accepted["metasystem/plans/goals/"+name]
		if len(blob) == 0 {
			t.Fatalf("accepted proof fixture has no %s", name)
		}
		if err := os.WriteFile(filepath.Join(dir, name), blob, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	packageDir, err := filepath.EvalSymlinks(wd)
	if err != nil {
		t.Fatal(err)
	}
	return hostLoadGitFixture{root: root, top: filepath.Dir(root), tip: tip, dir: dir, packageDir: packageDir, now: now}
}

func (f hostLoadGitFixture) environment(leg string) []string {
	return []string{
		"PATH=" + filepath.Join(f.dir, "bin") + string(os.PathListSeparator) + filepath.Join(f.dir, "deny") + string(os.PathListSeparator) + os.Getenv("PATH"),
		"HOSTLOAD_GIT_HELPER=" + os.Args[0],
		"HOSTLOAD_GIT_ROOT=" + f.root,
		"HOSTLOAD_GIT_TOP=" + f.top,
		"HOSTLOAD_GIT_TIP=" + f.tip,
		"HOSTLOAD_GIT_DIR=" + f.dir,
		"HOSTLOAD_GIT_PACKAGE=" + f.packageDir,
		"HOSTLOAD_GIT_NOW=" + strconv.FormatInt(f.now.Unix(), 10),
		"HOSTLOAD_GIT_CALLS=" + filepath.Join(f.dir, leg+".calls"),
		"METASYSTEM_WAIT_BINARY=" + os.Getenv("METASYSTEM_WAIT_BINARY"),
		"METASYSTEM_WAIT_BINARY_SOURCE=" + os.Getenv("METASYSTEM_WAIT_BINARY"),
	}
}

func (f hostLoadGitFixture) assertCalls(t *testing.T, leg string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(f.dir, leg+".calls"))
	if err != nil {
		t.Fatalf("%s Git helper evidence: %v", leg, err)
	}
	got := strings.Fields(string(data))
	seen := map[string]bool{}
	for _, kind := range got {
		if hostLoadGitNext(kind) == nil {
			t.Fatalf("%s Git helper recorded an undeclared call: %s", leg, data)
		}
		seen[kind] = true
	}
	for _, kind := range []string{"top", "remote", "branch", "tip", "tree", "blob", "log", "machine"} {
		if !seen[kind] {
			t.Fatalf("%s Git helper did not observe %s: %s", leg, kind, data)
		}
	}
	t.Logf("%s: %d declared Git reads consumed by the built CLI", leg, len(got))
}

func TestHostLoadGitHelper(t *testing.T) {
	t.Parallel()
	if os.Getenv("HOSTLOAD_GIT_CALLS") == "" {
		return
	}
	f := hostLoadGitFixture{root: os.Getenv("HOSTLOAD_GIT_ROOT"), top: os.Getenv("HOSTLOAD_GIT_TOP"), tip: os.Getenv("HOSTLOAD_GIT_TIP"),
		dir: os.Getenv("HOSTLOAD_GIT_DIR"), packageDir: os.Getenv("HOSTLOAD_GIT_PACKAGE")}
	var argv []string
	for i, arg := range os.Args {
		if arg == "--" {
			argv = os.Args[i+1:]
			break
		}
	}
	input, readErr := io.ReadAll(os.Stdin)
	calls := os.Getenv("HOSTLOAD_GIT_CALLS")
	prior, historyErr := os.ReadFile(calls)
	kind, output, status, err := f.answer(argv, input)
	if readErr != nil || historyErr != nil && !os.IsNotExist(historyErr) {
		err = fmt.Errorf("read stdin/history: %v / %v", readErr, historyErr)
	}
	previous := strings.Fields(string(prior))
	index := len(previous)
	last := ""
	if index > 0 {
		last = previous[index-1]
	}
	if err == nil && !slices.Contains(hostLoadGitNext(last), kind) {
		err = fmt.Errorf("request order %d: %s after %s", index, kind, last)
	}
	record := kind + "\n"
	if err != nil {
		record = fmt.Sprintf("FAIL cwd=%q argv=%q stdin=%q index=%d: %v\n", currentHostLoadGitCwd(), argv, input, index, err)
		status = 97
	}
	file, openErr := os.OpenFile(calls, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if openErr != nil {
		fmt.Fprintln(os.Stderr, "record Git helper call:", openErr)
		os.Exit(97)
	}
	_, writeErr := io.WriteString(file, record)
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		fmt.Fprintln(os.Stderr, "record Git helper call:", writeErr, closeErr)
		os.Exit(97)
	}
	if err != nil {
		fmt.Fprint(os.Stderr, record)
	} else {
		_, _ = os.Stdout.Write(output)
	}
	os.Exit(status)
}

func currentHostLoadGitCwd() string {
	wd, err := os.Getwd()
	if err != nil {
		return "<unknown>"
	}
	physical, err := filepath.EvalSymlinks(wd)
	if err != nil {
		return "<unresolved>"
	}
	return physical
}

func (f hostLoadGitFixture) answer(argv []string, input []byte) (string, []byte, int, error) {
	root, tip := f.root, f.tip
	cwd := currentHostLoadGitCwd()
	if cwd == f.packageDir {
		plain := []string{"-C", root, "rev-parse", "--show-toplevel"}
		pinned := []string{"-C", root, "-c", "core.fileMode=true", "-c", "diff.noprefix=false", "-c", "diff.mnemonicPrefix=false", "-c", "apply.ignoreWhitespace=no", "-c", "core.logAllRefUpdates=false", "-c", "core.useReplaceRefs=false", "-c", "gc.auto=0", "-c", "maintenance.auto=false", "rev-parse", "--show-toplevel"}
		log := []string{"-C", root, "-c", "core.logAllRefUpdates=false", "log", "-1", "--format=%ct", tip}
		switch {
		case slices.Equal(argv, plain) && len(input) == 0:
			return "top", []byte(f.top + "\n"), 0, nil
		case slices.Equal(argv, pinned) && len(input) == 0:
			return "pinned-top", []byte(f.top + "\n"), 0, nil
		case slices.Equal(argv, log) && len(input) == 0:
			return "log", []byte(os.Getenv("HOSTLOAD_GIT_NOW") + "\n"), 0, nil
		}
	}
	if cwd == root && len(input) == 0 {
		for _, config := range []struct {
			key, kind, value string
			status           int
		}{
			{"goal.sync-remote", "remote", "local\n", 0},
			{"goal.sync-branch", "branch", "", 1},
			{"metasystem.goal.machine", "machine", "mac-cli\n", 0},
		} {
			if slices.Equal(argv, []string{"config", "--get", config.key}) {
				return config.kind, []byte(config.value), config.status, nil
			}
		}
		if slices.Equal(argv, []string{"rev-parse", "--verify", "--quiet", "refs/metasystem/goals/accepted^{commit}"}) {
			return "tip", []byte(tip + "\n"), 0, nil
		}
		if slices.Equal(argv, []string{"ls-tree", "-r", "--name-only", tip, "--", "plans/goals/", "records/goals/"}) {
			return "tree", []byte("plans/goals/backlog.md\nplans/goals/standing-validation.md\n"), 0, nil
		}
	}
	if cwd == root && slices.Equal(argv, []string{"cat-file", "--batch"}) {
		want := tip + ":./plans/goals/backlog.md\n" + tip + ":./plans/goals/standing-validation.md\n"
		if string(input) == want {
			var output []byte
			for _, name := range []string{"backlog.md", "standing-validation.md"} {
				blob, err := os.ReadFile(filepath.Join(f.dir, name))
				if err != nil {
					return "", nil, 97, err
				}
				hash := sha1.Sum(append([]byte(fmt.Sprintf("blob %d\x00", len(blob))), blob...))
				output = fmt.Appendf(output, "%x blob %d\n", hash, len(blob))
				output = append(output, blob...)
				output = append(output, '\n')
			}
			return "blob", output, 0, nil
		}
	}
	return "", nil, 97, fmt.Errorf("undeclared Git read or input")
}
