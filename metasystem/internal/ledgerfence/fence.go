// Package ledgerfence owns the pre-commit guard enrollment that every goal
// mutation stands on.
//
// It was the command edge's own function until the interface server needed
// the same precondition: a server that publishes to the ledger in-process
// makes a mutation like any other, and the fence is a fact about the
// checkout, not about who typed. The code below is that function, moved
// whole, so the command edge and the interface can never drift into
// enforcing different fences.
package ledgerfence

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// Ensure installs or composes the pre-commit guard before any goal mutation
// publishes: git does not clone hooks, so a fresh clone would otherwise mutate
// the ledger with no accidental-edit fence. The guard is the engine's
// `internal pre-commit` entry (plans/designs/verbs-object-action.md 3.3,
// VOA-20). An existing hook is preserved as pre-commit.local behind the
// guard; a hook already running the guard is left alone; BOTH files existing
// without the guard refuses toward manual composition — enrollment never
// clobbers (the never-clobber rule, held here too). A composer this program
// wrote earlier (including the retired script-guard shapes) is upgraded in
// place.
func Ensure(root string) error {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	engine := filepath.Join(absRoot, "bin", "metasystem")
	if !fenceEngineRuns(absRoot, engine) {
		// Fail closed: a checkout without an EXECUTABLE engine cannot run
		// the fence, and a mutation without the fence is exactly what
		// enrollment forbids.
		return fmt.Errorf("this checkout has no executable engine at %s to run the pre-commit guard; the ledger fence cannot be enrolled, so the mutation refuses; build it with: go run ./cmd/devgate build", engine)
	}
	// The probes run with git's steering env scrubbed (GIT_DIR and
	// friends): an inherited broken GIT_DIR must neither read as
	// "nothing to enroll" nor answer for some OTHER repository. With
	// the scrub, "not a repository" is a true fact about the root —
	// and a target with no git has nothing that commits, so there is
	// no fence to enroll (adoption installs it when git init lands).
	probe := exec.Command("git", "-C", absRoot, "rev-parse", "--is-inside-work-tree")
	probe.Env = EnvironWithoutGitSteering()
	probeOut, probeErr := probe.CombinedOutput()
	if probeErr != nil {
		// Exit 128 alone is not proof: a malformed configuration in a
		// VALID repository exits 128 too, and reading that as "no
		// repository" would skip the fence while goal writes proceed.
		if strings.Contains(string(probeOut), "not a git repository") {
			return nil
		}
		return fmt.Errorf("the target's repository shape cannot be proven: %v (%s)", probeErr, strings.TrimSpace(string(probeOut)))
	}
	// --git-path hooks honors core.hooksPath: writing under
	// .git/hooks while git reads a configured hooks path elsewhere
	// would enroll a hook git never invokes. A probe that cannot
	// answer refuses.
	hooksCmd := exec.Command("git", "-C", absRoot, "rev-parse", "--path-format=absolute", "--git-path", "hooks")
	hooksCmd.Env = EnvironWithoutGitSteering()
	out, err := hooksCmd.Output()
	if err != nil {
		return fmt.Errorf("the hooks directory cannot be resolved for %s: %v", absRoot, err)
	}
	// Newline-only trim: a core.hooksPath ending in a space is a
	// lawful (if hostile) directory name, and deleting the byte would
	// install the hook where git never looks.
	hookDir := strings.TrimRight(string(out), "\n")
	// A hooks directory OUTSIDE both the repository's common dir and
	// its toplevel is SHARED (a global core.hooksPath): composing our
	// fail-closed guard there would break every unrelated repository
	// that commits through it.
	commonCmd := exec.Command("git", "-C", absRoot, "rev-parse", "--path-format=absolute", "--git-common-dir")
	commonCmd.Env = EnvironWithoutGitSteering()
	commonOut, commonErr := commonCmd.Output()
	topCmd0 := exec.Command("git", "-C", absRoot, "rev-parse", "--show-toplevel")
	topCmd0.Env = EnvironWithoutGitSteering()
	topOut0, topErr0 := topCmd0.Output()
	if commonErr != nil || topErr0 != nil {
		return fmt.Errorf("the repository's own directories cannot be resolved for enrollment")
	}
	within := func(dir, parent string) bool {
		// Filesystem identity, not lexical shape: a lexically inner
		// path whose component is a symlink OUT of the repository
		// must not pass containment.
		if rd, rdErr := filepath.EvalSymlinks(dir); rdErr == nil {
			dir = rd
		}
		if rp, rpErr := filepath.EvalSymlinks(parent); rpErr == nil {
			parent = rp
		}
		rel, relErr := filepath.Rel(parent, dir)
		return relErr == nil && (rel == "." || filepath.IsLocal(rel))
	}
	commonDir := strings.TrimRight(string(commonOut), "\n")
	topDir := strings.TrimRight(string(topOut0), "\n")
	if !within(hookDir, commonDir) && !within(hookDir, topDir) {
		return fmt.Errorf("this repository's hooks directory %s is shared (core.hooksPath outside the repository); composing the ledger fence there would break unrelated repositories — enroll by hand, then re-run", hookDir)
	}
	hookPath := filepath.Join(hookDir, "pre-commit")
	existing, readErr := os.ReadFile(hookPath)
	if readErr != nil && !os.IsNotExist(readErr) {
		// An unreadable hook is not an absent hook: falling through
		// would overwrite a file whose content was never seen.
		return fmt.Errorf("the existing pre-commit hook cannot be read: %v", readErr)
	}
	// Enrollment means the guard RUNS: static reading of a shell hook
	// is an arms race (reassignments, echoes, substitutions all fool
	// it), so the proof is BEHAVIORAL — the hook chain is executed
	// with a probe nonce and must return the guard's acknowledgment.
	// The guard answers the probe first thing and exits distinctly,
	// so no downstream hook does real work.
	enrolled := false
	if readErr == nil {
		if hookInfo, hookStatErr := os.Stat(hookPath); hookStatErr == nil && hookInfo.Mode()&0o111 != 0 {
			nonce, nonceErr := goal.NewOperationULID()
			if nonceErr != nil {
				return nonceErr
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			probeRun := exec.CommandContext(ctx, hookPath)
			probeRun.Dir = absRoot
			probeRun.Env = append(EnvironWithoutGitSteering(), "METASYSTEM_GUARD_PROBE="+nonce)
			probeRun.WaitDelay = 5 * time.Second
			probeOutBytes, probeRunErr := probeRun.CombinedOutput()
			cancel()
			// The ack alone is not enrollment: the guard exits 42
			// under probe, and the hook chain must PROPAGATE that
			// status — a wrapper that swallows it ("$guard" || true)
			// would equally swallow the guard's real rejections. A
			// timeout or any other exit refuses. (A hook that FORGES
			// the ack and the status is its owner sabotaging their
			// own fence — the fence guards accidents, not authors.)
			var exitErr *exec.ExitError
			ackSeen := strings.Contains(string(probeOutBytes), "guard-probe-ack "+nonce)
			statusPropagated := errors.As(probeRunErr, &exitErr) && exitErr.ExitCode() == 42 && ctx.Err() == nil
			enrolled = ackSeen && statusPropagated
		}
	}
	// An enrolled hook of OUR OWN shape that is not the current composer
	// (a retired script-guard composer) upgrades in place.
	upgradeOurs := readErr == nil && isOurComposer(string(existing)) && !isCurrentComposer(string(existing))
	if enrolled && !upgradeOurs {
		return nil
	}
	if err := os.MkdirAll(hookDir, 0o755); err != nil {
		return err
	}
	localPath := filepath.Join(hookDir, "pre-commit.local")
	if readErr == nil && !isOurComposer(string(existing)) {
		if _, localErr := os.Stat(localPath); localErr == nil {
			return fmt.Errorf("pre-commit and pre-commit.local both exist and neither enrolls the guard; compose them by hand before mutating the ledger")
		}
		if err := os.Rename(hookPath, localPath); err != nil {
			return err
		}
	}
	// A hook WE wrote earlier (recognized by its own shape) upgrades or
	// recomposes in place — the local dispatch stays intact. The engine is
	// resolved per invocation: hooks live in the COMMON git directory shared
	// by every linked worktree, and a linked worktree without its own built
	// engine runs its primary checkout's. The composer FAILS CLOSED when no
	// engine runs the guard — a fence that silently steps aside is no fence.
	prefixCmd := exec.Command("git", "-C", absRoot, "rev-parse", "--show-prefix")
	prefixCmd.Env = EnvironWithoutGitSteering()
	prefixOut, prefixErr := prefixCmd.Output()
	if prefixErr != nil {
		return fmt.Errorf("the checkout's toplevel prefix cannot be resolved: %v", prefixErr)
	}
	// Newline-only trim (a prefix beginning with whitespace is a lawful
	// path), and the prefix rides SINGLE-QUOTED: a directory component
	// carrying $(), backticks, or quotes must reach bash as bytes.
	composer := composerFor(strings.TrimRight(string(prefixOut), "\n"))
	if err := os.WriteFile(hookPath, []byte(composer), 0o755); err != nil {
		return err
	}
	// WriteFile's mode is umask-filtered; the fence exists only if
	// git can actually execute the hook, so set and re-verify.
	if err := os.Chmod(hookPath, 0o755); err != nil {
		return err
	}
	hookInfo, hookStatErr := os.Stat(hookPath)
	if hookStatErr != nil || hookInfo.Mode()&0o111 == 0 {
		return fmt.Errorf("the enrolled pre-commit hook did not come out executable; fix the filesystem before mutating the ledger")
	}
	return nil
}

// fenceEngineRuns reports whether the composed hook finds an executable
// engine for the checkout at absRoot: its own bin/metasystem, or, as the
// composer resolves it for a linked worktree without a built engine, the
// primary checkout's at the same installation prefix.
func fenceEngineRuns(absRoot, engine string) bool {
	executable := func(path string) bool {
		info, err := os.Stat(path)
		return err == nil && info.Mode().IsRegular() && info.Mode()&0o111 != 0
	}
	if executable(engine) {
		return true
	}
	common := exec.Command("git", "-C", absRoot, "rev-parse", "--path-format=absolute", "--git-common-dir")
	common.Env = EnvironWithoutGitSteering()
	commonOut, commonErr := common.Output()
	prefix := exec.Command("git", "-C", absRoot, "rev-parse", "--show-prefix")
	prefix.Env = EnvironWithoutGitSteering()
	prefixOut, prefixErr := prefix.Output()
	if commonErr != nil || prefixErr != nil {
		return false
	}
	primary := filepath.Join(filepath.Dir(strings.TrimRight(string(commonOut), "\n")), strings.TrimRight(string(prefixOut), "\n"), "bin", "metasystem")
	return executable(primary)
}

// composerFor is the hook this program writes for an installation at prefix
// below the Git toplevel: the prefix line, then the engine body.
func composerFor(prefix string) string {
	return "#!/usr/bin/env bash\nprefix=" + shellSingleQuote(prefix) + "\n" + composerBodyEngine
}

// composerBodyEngine is the current composer body, byte-exact below the
// prefix line: it runs the engine's guard entry, then the local hook.
const composerBodyEngine = `installation="$(git rev-parse --show-toplevel)/$prefix"
engine="${installation}bin/metasystem"
if [[ ! -x "$engine" ]]; then
  engine="$(dirname "$(git rev-parse --path-format=absolute --git-common-dir)")/${prefix}bin/metasystem"
fi
if [[ ! -x "$engine" ]]; then
  echo "pre-commit: no metasystem engine at $engine runs the ledger fence, so the commit is refused; build one with: go run ./cmd/devgate build" >&2
  exit 1
fi
"$engine" internal pre-commit --root "$installation"
status=$?
if [[ $status -eq 2 ]]; then
  echo "pre-commit: the engine at $engine does not run the pre-commit guard; rebuild it with: go run ./cmd/devgate build" >&2
fi
[[ $status -eq 0 ]] || exit $status
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
if [[ -x "$here/pre-commit.local" ]]; then
  exec "$here/pre-commit.local" "$@"
fi
exit 0
`

// The retired composer bodies, byte-exact below their guard= line: they ran
// the deleted scripts/agents/pre-commit-guard.sh. They are recognized so an
// enrolled checkout upgrades in place instead of stacking.
const composerBodyFailClosed = `if [[ ! -x "$guard" ]]; then
  echo "pre-commit: the metasystem ledger guard is missing at $guard; refusing to commit without the fence" >&2
  exit 1
fi
"$guard" || exit $?
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
if [[ -x "$here/pre-commit.local" ]]; then
  exec "$here/pre-commit.local" "$@"
fi
exit 0
`

const composerBodyFailOpen = `if [[ -x "$guard" ]]; then
  "$guard" || exit $?
fi
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
if [[ -x "$here/pre-commit.local" ]]; then
  exec "$here/pre-commit.local" "$@"
fi
exit 0
`

// isCurrentComposer recognizes the current composer by its exact shape: the
// shebang, one prefix= line whose value is a single-quoted literal, and the
// engine body byte-for-byte.
func isCurrentComposer(hook string) bool {
	lines := strings.SplitN(hook, "\n", 3)
	if len(lines) < 3 || lines[0] != "#!/usr/bin/env bash" || lines[2] != composerBodyEngine {
		return false
	}
	quoted, found := strings.CutPrefix(lines[1], "prefix=")
	if !found {
		return false
	}
	value, ok := parseSingleQuoted(quoted)
	return ok && shellSingleQuote(value) == quoted
}

// parseSingleQuoted reads a word made only of '...' runs and \' escapes.
func parseSingleQuoted(word string) (string, bool) {
	var out strings.Builder
	for word != "" {
		switch {
		case strings.HasPrefix(word, `\'`):
			out.WriteByte('\'')
			word = word[2:]
		case strings.HasPrefix(word, "'"):
			end := strings.IndexByte(word[1:], '\'')
			if end < 0 {
				return "", false
			}
			out.WriteString(word[1 : 1+end])
			word = word[2+end:]
		default:
			return "", false
		}
	}
	return out.String(), true
}

// isOurComposer recognizes a composer this program (or an earlier adopter) wrote by
// its EXACT shape: the current engine composer, or a retired script-guard
// composer (the shebang, one guard= assignment, and one of the two retired
// bodies byte-for-byte). A foreign or locally extended hook that merely
// contains similar fragments is a human's file — rewriting it would delete
// their checks.
func isOurComposer(hook string) bool {
	if isCurrentComposer(hook) {
		return true
	}
	lines := strings.SplitN(hook, "\n", 3)
	if len(lines) < 3 || lines[0] != "#!/usr/bin/env bash" {
		return false
	}
	// The guard line is ONE assignment ending in the quoted guard
	// filename — a trailing command ("guard=...; run-custom-check")
	// is a human's extension, and rewriting it would delete their
	// check.
	guardLine := lines[1]
	if !strings.HasPrefix(guardLine, "guard=") || strings.ContainsAny(guardLine, ";&|`") ||
		strings.Contains(guardLine, "<(") || strings.Contains(guardLine, ">(") {
		return false
	}
	if !strings.HasSuffix(guardLine, `pre-commit-guard.sh"`) && !strings.HasSuffix(guardLine, "pre-commit-guard.sh'") {
		return false
	}
	// The ONLY lawful command substitution is the literal toplevel
	// resolution our composers emitted; any other $( is a human's logic.
	rest := strings.ReplaceAll(guardLine, `guard="$(git rev-parse --show-toplevel)/"`, "")
	rest = strings.ReplaceAll(rest, `"$(git rev-parse --show-toplevel)/`, "")
	if strings.Contains(rest, "$(") {
		return false
	}
	return lines[2] == composerBodyFailClosed || lines[2] == composerBodyFailOpen
}

// shellSingleQuote renders s as one single-quoted shell word: inside
// single quotes the shell expands nothing, so path components
// carrying $, backticks, quotes, or spaces reach bash as bytes.
func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// EnvironWithoutGitSteering mirrors the goal package's scrub: the
// enrollment probes must answer about the ROOT, never about whatever
// repository an inherited GIT_DIR happens to point at.
func EnvironWithoutGitSteering() []string {
	steering := map[string]bool{
		"GIT_DIR": true, "GIT_WORK_TREE": true, "GIT_COMMON_DIR": true,
		"GIT_INDEX_FILE": true, "GIT_CEILING_DIRECTORIES": true,
		"GIT_OBJECT_DIRECTORY": true, "GIT_ALTERNATE_OBJECT_DIRECTORIES": true,
		"GIT_CONFIG": true, "GIT_CONFIG_PARAMETERS": true,
		"GIT_CONFIG_COUNT": true, "GIT_CONFIG_GLOBAL": true,
		"GIT_CONFIG_SYSTEM": true, "GIT_CONFIG_NOSYSTEM": true,
		"GIT_GRAFT_FILE": true, "GIT_SHALLOW_FILE": true,
		"GIT_REPLACE_REF_BASE": true,
	}
	var out []string
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if steering[name] || strings.HasPrefix(name, "GIT_CONFIG_KEY_") || strings.HasPrefix(name, "GIT_CONFIG_VALUE_") {
			continue
		}
		out = append(out, entry)
	}
	return out
}
