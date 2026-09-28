package hooks

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/jsonedit"
)

// stopDeadlineParentEnv names the deadline parent to its Stop worker, and
// stopDeadlineStartedEnv carries the parent's start (epoch seconds).
const (
	stopDeadlineParentEnv  = "METASYSTEM_STOP_DEADLINE_PARENT"
	stopDeadlineStartedEnv = "METASYSTEM_STOP_DEADLINE_STARTED"
	stopDeadlineBudgetEnv  = "METASYSTEM_STOP_DEADLINE_BUDGET_SEC"
)

// internalSkipResult is the one worker output the deadline parent treats as
// an intentional skip (a delegate, or another runtime's session).
const internalSkipResult = "METASYSTEM_INTERNAL_HOOK_SKIP_V1"

var runtimeNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)

// hookExit ends the hook with a status. The entry recovers it, so every
// branch that exited the script returns through one place.
type hookExit struct{ status int }

func exitHook(status int) { panic(hookExit{status}) }

// RunRuntimeHook runs one runtime lifecycle event: start, stop, end, receipt,
// or Claude's tool gate. It returns the process exit status.
func RunRuntimeHook(inv Invocation, ops Ops) (status int) {
	defer func() {
		if recovered := recover(); recovered != nil {
			exit, ok := recovered.(hookExit)
			if !ok {
				panic(recovered)
			}
			status = exit.status
		}
	}()
	inv = inv.withDefaults()
	if inv.Runtime == "claude" && inv.Event == "tool" {
		return runToolGate(inv)
	}
	if inv.Event == "start" {
		return runStart(inv, ops)
	}
	if !runtimeNamePattern.MatchString(inv.Runtime) {
		return 2
	}
	switch inv.Event {
	case "receipt", "stop", "end":
	default:
		return 2
	}
	return runLifecycle(inv, ops)
}

func (inv Invocation) withDefaults() Invocation {
	if inv.Lookup == nil {
		inv.Lookup = func(string) (string, bool) { return "", false }
	}
	if inv.Stdout == nil {
		inv.Stdout = io.Discard
	}
	if inv.Stderr == nil {
		inv.Stderr = io.Discard
	}
	if inv.Stdin == nil {
		inv.Stdin = strings.NewReader("")
	}
	if inv.IsExecutable == nil {
		inv.IsExecutable = isExecutableFile
	}
	if inv.Environ == nil {
		inv.Environ = func() []string { return nil }
	}
	if inv.TempDir == "" {
		inv.TempDir = os.TempDir()
	}
	return inv
}

func (inv Invocation) env(name string) string {
	value, _ := inv.Lookup(name)
	return value
}

func isExecutableFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Mode()&0o111 != 0
}

// runToolGate is Claude's PreToolUse path. A delegate job is fenced by its
// adapter. A host tool call execs the engine the last start that resolved an
// executable engine recorded; missing or stale cache state leaves the call
// untouched.
func runToolGate(inv Invocation) int {
	if inv.env("METASYSTEM_HOOK_DELEGATE_JOB") != "" {
		return 0
	}
	cache := inv.Installation + "/artifacts/agents/context/engine-path"
	data, err := os.ReadFile(cache)
	if err != nil {
		return 0
	}
	lines := strings.SplitAfter(string(data), "\n")
	// Both lines must be complete: the shell's two reads refused a final
	// line without its newline.
	if len(lines) < 2 || !strings.HasSuffix(lines[0], "\n") || !strings.HasSuffix(lines[1], "\n") {
		return 0
	}
	gate := strings.TrimSuffix(lines[0], "\n")
	installation := strings.TrimSuffix(lines[1], "\n")
	if gate == "" || installation == "" || !inv.IsExecutable(gate) || inv.Exec == nil {
		return 0
	}
	_ = inv.Exec(gate, []string{gate, "adapter", "claude-tool-gate", "--root", installation}, inv.Environ())
	return 0
}

// trimNewlines is command substitution's view of an output: trailing
// newlines removed.
func trimNewlines(value string) string { return strings.TrimRight(value, "\n") }

// jsonGet is `json get` on content: the rendered value (a trailing newline
// unless shell-safe) and its status (3 absent, 1 unreadable, 2 no input).
func jsonGet(content, field string, shellSafe bool, def *string) (string, int) {
	if content == "" {
		return "", 2
	}
	data := []byte(content)
	var out string
	var ok bool
	if shellSafe {
		out, ok = jsonedit.GetShellString(data, field, def)
	} else {
		out, ok = jsonedit.Get(data, field, def)
	}
	if !ok {
		if shellSafe {
			if _, present := jsonedit.Get(data, field, nil); present {
				return "", 1
			}
			if !jsonedit.FieldAbsent(data, field) {
				return "", 1
			}
		}
		marker := "json-get-field-absent"
		if _, absent := jsonedit.Get(data, field, &marker); absent {
			return "", 3
		}
		return "", 1
	}
	if shellSafe {
		return out, 0
	}
	return out + "\n", 0
}

// jsonValue is `$(json get ... 2>/dev/null || true)`: the value, or empty.
func jsonValue(content, field string) string {
	out, status := jsonGet(content, field, false, nil)
	if status != 0 {
		return ""
	}
	return trimNewlines(out)
}

// jsonValueDefault is jsonValue with --default.
func jsonValueDefault(content, field, def string) string {
	out, status := jsonGet(content, field, false, &def)
	if status != 0 {
		return ""
	}
	return trimNewlines(out)
}

// jsonValueStatus is `value=$(json get ...) || status=$?`.
func jsonValueStatus(content, field string) (string, int) {
	out, status := jsonGet(content, field, false, nil)
	return trimNewlines(out), status
}

// jsonStrip is `json strip` on content: the indented remainder object.
func jsonStrip(content string, keys ...string) (string, int) {
	object, err := jsonedit.StripKeys([]byte(content), keys)
	if err != nil {
		return "", 1
	}
	encoded, err := json.MarshalIndent(object, "", "  ")
	if err != nil {
		return "", 1
	}
	return string(encoded), 0
}

// jsonObject is `json object key=value`: one compact, HTML-unescaped object.
func jsonObject(pairs ...string) string {
	line, err := jsonedit.Object(pairs)
	if err != nil {
		return ""
	}
	return line
}

// physicalDirectory is `cd -- DIR && pwd -P`.
func physicalDirectory(path string) (string, bool) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", false
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", false
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.IsDir() {
		return "", false
	}
	return resolved, true
}

func oneLine(value string) bool {
	return value != "" && !strings.ContainsAny(value, "\n\r")
}

// worldInstallation prints the physical installation that governs this hook.
// Every linked worktree maps once to the same relative installation beneath
// its primary checkout because the engine arms no linked worktree.
// Repository identification must succeed; a failed query never becomes proof
// that the candidate is an ordinary checkout.
func worldInstallation(ops Ops, harnessRoot string) (string, bool) {
	ids, err := ops.Git("-C", harnessRoot, "rev-parse", "--path-format=absolute", "--git-dir", "--git-common-dir")
	if err != nil {
		return "", false
	}
	ids = trimNewlines(ids)
	gitDir, gitCommon, found := strings.Cut(ids, "\n")
	if !found || gitDir == "" || gitCommon == "" {
		return "", false
	}
	world := harnessRoot
	if gitDir != gitCommon {
		if filepath.Base(gitCommon) != ".git" {
			return "", false
		}
		primaryTop, ok := physicalDirectory(filepath.Dir(gitCommon))
		if !ok {
			return "", false
		}
		top, err := ops.Git("-C", harnessRoot, "rev-parse", "--show-toplevel")
		if err != nil {
			return "", false
		}
		worktreeTop, ok := physicalDirectory(trimNewlines(top))
		if !ok {
			return "", false
		}
		relative := ""
		switch {
		case harnessRoot == worktreeTop:
		case strings.HasPrefix(harnessRoot, worktreeTop+"/"):
			relative = strings.TrimPrefix(harnessRoot, worktreeTop+"/")
		default:
			return "", false
		}
		world = primaryTop
		if relative != "" {
			world += "/" + relative
		}
		if info, err := os.Stat(filepath.Join(world, "metasystem.conf")); err != nil || !info.Mode().IsRegular() {
			return "", false
		}
	}
	return world, true
}

// installationRoot is the physical installation the hook serves: the
// directory the direct settings command entered.
func (inv Invocation) installationRoot() (string, bool) {
	if inv.Installation == "" {
		return "", false
	}
	return physicalDirectory(inv.Installation)
}

// writeLine prints one line; its error is the stream's.
func writeLine(w io.Writer, line string) error {
	_, err := io.WriteString(w, line+"\n")
	return err
}
