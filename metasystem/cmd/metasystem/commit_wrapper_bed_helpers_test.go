package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// Helpers for the commit-wrapper beds ported from static-reproof-fixtures.sh
// and path-class-fixtures.sh. Those beds drive the unchanged committed
// scripts/agents/commit.sh and land.sh; physical Git is their claim (the
// index/worktree divergence, the soft rollback to the exact proved index, the
// ignored and untracked inputs, the staged symlinks, the pushed refs), and a
// Git stub under a thousand-line shell script would be a Git emulator. The
// engine the scripts call is a per-bed shell stub whose unhandled verbs go to
// this test binary acting as the engine (GO_WANT_BATCH_E2E_COMMAND).

// wrapperBedSource is the metasystem source root this package is built from.
func wrapperBedSource(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

// wrapperBedDir is a fresh canonical temporary directory.
func wrapperBedDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

// wrapperBedEnvironment is the whole environment of a bed's children: the
// caller's PATH and a private HOME and TMPDIR, so no ambient proof locator,
// engine selector or Git configuration reaches the fixture.
func wrapperBedEnvironment(t *testing.T) []string {
	t.Helper()
	return []string{"PATH=" + os.Getenv("PATH"), "HOME=" + wrapperBedDir(t), "TMPDIR=" + wrapperBedDir(t)}
}

func wrapperBedWrite(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

func wrapperBedAppend(t *testing.T, path, content string) {
	t.Helper()
	handle, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := handle.WriteString(content); err != nil {
		t.Fatal(err)
	}
	if err := handle.Close(); err != nil {
		t.Fatal(err)
	}
}

// wrapperBedCopy copies a source file (relative to the metasystem root) into
// the bed with the given mode.
func wrapperBedCopy(t *testing.T, relative, to string, mode os.FileMode) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(wrapperBedSource(t), filepath.FromSlash(relative)))
	if err != nil {
		t.Fatal(err)
	}
	wrapperBedWrite(t, to, string(data), mode)
}

// wrapperBedGitResult runs Git in dir and returns its trimmed combined output.
func wrapperBedGitResult(env []string, dir string, args ...string) (string, error) {
	command := exec.Command("git", append([]string{"-C", dir}, args...)...)
	command.Env = env
	output, err := command.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

func wrapperBedGit(t *testing.T, env []string, dir string, args ...string) string {
	t.Helper()
	output, err := wrapperBedGitResult(env, dir, args...)
	if err != nil {
		t.Fatalf("git -C %s %s: %v\n%s", dir, strings.Join(args, " "), err, output)
	}
	return output
}

// wrapperBedRun runs an executable from dir with the bed environment plus
// extra entries and returns its combined output.
func wrapperBedRun(env []string, dir string, extra []string, name string, args ...string) (string, error) {
	command := exec.Command(name, args...)
	command.Dir = dir
	command.Env = append(append([]string{}, env...), extra...)
	output, err := command.CombinedOutput()
	return string(output), err
}

// wrapperBedEngineExec is the shell line that hands a verb to this test
// binary acting as the real engine.
func wrapperBedEngineExec(t *testing.T) string {
	t.Helper()
	return "export GO_WANT_BATCH_E2E_COMMAND=1; exec " + wrapperBedShellQuote(commandTestExecutable(t)) + ` "$@"`
}

func wrapperBedShellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

// wrapperBedEngine runs one verb on the real engine.
func wrapperBedEngine(t *testing.T, env []string, args ...string) string {
	t.Helper()
	output, err := wrapperBedRun(env, wrapperBedDir(t), []string{"GO_WANT_BATCH_E2E_COMMAND=1"}, commandTestExecutable(t), args...)
	if err != nil {
		t.Fatalf("engine %s: %v\n%s", strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(output)
}

// wrapperBedGoGateCopying is a stub fast gate that succeeds by copying the
// named proof engine to the gate's --proof-out.
func wrapperBedGoGateCopying(proofEngine string) string {
	return `#!/usr/bin/env bash
set -euo pipefail
proof_out=
while (($#)); do
  case "$1" in
    --proof-out) proof_out=$2; shift 2 ;;
    *) shift ;;
  esac
done
[[ -n "$proof_out" ]]
cp ` + proofEngine + ` "$proof_out"
chmod +x "$proof_out"
`
}

// wrapperBedRealProofEngine is the real engine except for the static audit,
// whose legs belong to the real suite.
func wrapperBedRealProofEngine(t *testing.T) string {
	t.Helper()
	return "#!/usr/bin/env bash\n" +
		`[[ "${1:-} ${2:-} ${3:-}" != "internal audit metasystem" ]] || exit 0` + "\n" +
		wrapperBedEngineExec(t) + "\n"
}

func wrapperBedExitCode(err error) int {
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode()
	}
	if err != nil {
		return -1
	}
	return 0
}

func wrapperBedContainsAll(t *testing.T, leg, output string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(output, want) {
			t.Fatalf("%s: output lacks %q:\n%s", leg, want, output)
		}
	}
}

// wrapperBedFixtureGoal is the claimed goal fx with its Integrity line.
func wrapperBedFixtureGoal(intent, machine, lineage string) string {
	body := "# fx\n\n" +
		"- State: claimed\n" +
		"- Intent: " + intent + "\n" +
		"- Origin: main\n" +
		"- Next step: Land the owned record.\n" +
		"- OpenedAt: 2026-09-03T08:00:00Z\n" +
		"- Revision: 1\n" +
		"- Claimed: machine=" + machine + " lineage=" + lineage + " at=2026-09-03T08:01:00Z revision=1\n\n" +
		"History:\n" +
		"- 2026-09-03T08:01:00Z 01ARZ3NDEKTSV4RRFFQ69G5FAW-fx-00000001 claim actor=" + machine + "+" + lineage + " targets=fx\n"
	return body + "Integrity: sha256=" + goal.IntegrityDigest([]byte(body)) + "\n"
}
