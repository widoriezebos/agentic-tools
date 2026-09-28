package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/ledgerfence"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// system setup is U9's per-checkout activation (plans/designs/
// verbs-object-action.md 3.3): it validates the engine's hook entry,
// switches the runtime settings from the plumbing stub to the direct engine
// command, and re-enrolls a pre-commit composer left from before the engine
// guard. The beds are real Git checkouts; the engine is a fixture program.

const systemSetupFakeEngine = `#!/bin/sh
case "$1 $2 $3" in
  "internal hook --accepts") if [ -e "$0.old" ]; then echo 'metasystem: unknown command "hook"' >&2; exit 2; fi; exit 0 ;;
  "internal pre-commit --root")
    if [ -n "${METASYSTEM_GUARD_PROBE:-}" ]; then printf 'guard-probe-ack %s\n' "$METASYSTEM_GUARD_PROBE"; exit 42; fi
    exit 0 ;;
esac
echo "fixture engine: unexpected $*" >&2
exit 2
`

// retiredComposer is the composer adoption wrote before U5: it runs the
// deleted pre-commit-guard.sh, so it refuses every commit.
const retiredComposer = `#!/usr/bin/env bash
guard="$(git rev-parse --show-toplevel)/metasystem/scripts/agents/pre-commit-guard.sh"
if [[ ! -x "$guard" ]]; then
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

// stubEraClaudeSettings are Claude settings whose lifecycle handlers run the
// plumbing stub scripts/agents/supervision-hook.sh, as an earlier setup wrote
// them.
const stubEraClaudeSettings = `{"hooks":{
"SessionStart":[{"matcher":"startup|resume|clear|compact","hooks":[{"type":"command","command":"cd \"$CLAUDE_PROJECT_DIR/metasystem\" && bash scripts/agents/supervision-hook.sh claude start","timeout":15}]}],
"Stop":[{"hooks":[{"type":"command","command":"cd \"$CLAUDE_PROJECT_DIR/metasystem\" && bash scripts/agents/supervision-hook.sh claude stop","timeout":60}]}],
"SessionEnd":[{"hooks":[{"type":"command","command":"cd \"$CLAUDE_PROJECT_DIR/metasystem\" && bash scripts/agents/supervision-hook.sh claude end","timeout":3}]}]}}
`

type systemSetupBed struct {
	repo, installation, hook string
}

func systemSetupGit(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", dir}, args...)...)
	command.Env = append(ledgerfence.EnvironWithoutGitSteering(), "GIT_CONFIG_NOSYSTEM=1")
	output, err := command.CombinedOutput()
	return string(output), err
}

func newSystemSetupBed(t *testing.T) systemSetupBed {
	t.Helper()
	repo, _ := setupCLIFixture(t)
	repo, _ = filepath.EvalSymlinks(repo)
	installation := filepath.Join(repo, "metasystem")
	setupCLIWrite(t, filepath.Join(installation, "bin", "metasystem"), systemSetupFakeEngine, 0o755)
	setupCLIWrite(t, filepath.Join(repo, ".claude", "settings.json"), stubEraClaudeSettings, 0o644)
	if output, err := systemSetupGit(t, repo, "init", "-q"); err != nil {
		t.Fatalf("git init: %v %s", err, output)
	}
	hook := filepath.Join(repo, ".git", "hooks", "pre-commit")
	setupCLIWrite(t, hook, retiredComposer, 0o755)
	return systemSetupBed{repo: repo, installation: installation, hook: hook}
}

func (b systemSetupBed) run(t *testing.T, options ...string) (int, intentResult) {
	t.Helper()
	command, rest, ok := resolveIntentArgv(append([]string{"system", "setup", "--json"}, options...))
	if !ok {
		t.Fatal("system setup is not a public command")
	}
	var stdout, stderr bytes.Buffer
	code := runIntentIn(command, rest, &stdout, &stderr, b.repo, intentOwners{resolver: stateroot.NewResolver(stateroot.RepositoryTop, noExecutable)})
	var result intentResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("system setup printed no JSON: %v stdout %q stderr %q", err, stdout.String(), stderr.String())
	}
	return code, result
}

// The switch-over: stub-era settings become the direct engine command, the
// retired composer that refused every commit without a fix is re-enrolled,
// and a commit then runs the engine guard.
func TestSystemSetupSwitchesAStubEraCheckoutToTheEngine(t *testing.T) {
	t.Parallel()
	bed := newSystemSetupBed(t)
	setupCLIWrite(t, filepath.Join(bed.repo, "file.txt"), "content\n", 0o644)
	if output, err := systemSetupGit(t, bed.repo, "add", "file.txt"); err != nil {
		t.Fatalf("git add: %v %s", err, output)
	}
	commit := []string{"-c", "user.name=fixture", "-c", "user.email=fixture@invalid", "commit", "-qm", "fixture"}
	if output, err := systemSetupGit(t, bed.repo, commit...); err == nil || !strings.Contains(output, ledgerfence.RetiredComposerRefusal) {
		t.Fatalf("the retired composer did not refuse: %v %s", err, output)
	}

	code, result := bed.run(t)
	if code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("switch = %d %+v", code, result)
	}
	data := result.Data.(map[string]any)
	if data["fence"] != "re-enrolled" || data["engine"] != filepath.Join(bed.installation, "bin", "metasystem") {
		t.Fatalf("switch data = %+v", data)
	}
	settings, err := os.ReadFile(filepath.Join(bed.repo, ".claude", "settings.json"))
	if err != nil || strings.Contains(string(settings), "supervision-hook.sh") || strings.Count(string(settings), "internal hook claude ") != 4 {
		t.Fatalf("settings after the switch = %s, %v", settings, err)
	}
	hook, err := os.ReadFile(bed.hook)
	if err != nil || ledgerfence.RetiredComposer(string(hook)) || !strings.Contains(string(hook), "internal pre-commit --root") {
		t.Fatalf("hook after the switch = %s, %v", hook, err)
	}
	if output, err := systemSetupGit(t, bed.repo, commit...); err != nil {
		t.Fatalf("a commit after the switch was refused: %v %s", err, output)
	}
	// Only the runtimes the installation enables (metasystem.runtimes=claude)
	// are registered: Claude's skills, no Codex or Devin settings.
	for _, path := range []string{".codex/hooks.json", ".devin/config.json"} {
		if _, err := os.Lstat(filepath.Join(bed.repo, path)); !os.IsNotExist(err) {
			t.Fatalf("setup registered %s: %v", path, err)
		}
	}
	if info, err := os.Lstat(filepath.Join(bed.repo, ".claude", "skills", "demo")); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("setup did not register Claude's skills: %v", err)
	}
}

// system setup is the one activation (U9b, B1): runtime registrations, the
// hooks and commit fence, and the testing contract's merge driver, in one
// idempotent act; --runtimes and --copy-skills choose what it registers.
func TestSystemSetupRegistersRuntimesAndTheTestingMergeDriver(t *testing.T) {
	t.Parallel()
	bed := newSystemSetupBed(t)
	code, result := bed.run(t, "--runtimes", "claude,codex", "--copy-skills")
	if code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("setup = %d %+v", code, result)
	}
	if info, err := os.Lstat(filepath.Join(bed.repo, ".agents", "skills", "demo")); err != nil || !info.IsDir() {
		t.Fatalf("--runtimes claude,codex --copy-skills did not copy Codex's skills: %v", err)
	}
	attributes, err := os.ReadFile(filepath.Join(bed.repo, ".gitattributes"))
	if err != nil || !strings.Contains(string(attributes), "metasystem/testing.json merge=metasystem-testing\n") {
		t.Fatalf(".gitattributes = %q, %v", attributes, err)
	}
	driver, err := systemSetupGit(t, bed.repo, "config", "--local", "--get", "merge.metasystem-testing.driver")
	if err != nil || !strings.Contains(driver, filepath.Join(bed.installation, "bin", "metasystem")) || !strings.Contains(driver, "testing merge-driver %O %A %B") {
		t.Fatalf("merge driver = %q, %v", driver, err)
	}
	if data := result.Data.(map[string]any); data["mergeDriver"] != "registered" {
		t.Fatalf("setup data = %+v", data)
	}
	code, result = bed.run(t, "--runtimes", "claude,codex", "--copy-skills")
	if code != 0 || result.Outcome != intentUnchanged || result.Data.(map[string]any)["mergeDriver"] != "unchanged" {
		t.Fatalf("repeat = %d %+v", code, result)
	}
	if strings.Count(string(attributes), "merge=metasystem-testing") != 1 {
		t.Fatalf(".gitattributes repeats the driver: %q", attributes)
	}
}

// system register is folded into system setup, and --check is gone: system
// check reports setup drift.
func TestSystemRegisterIsFoldedIntoSetup(t *testing.T) {
	t.Parallel()
	if _, ok := findIntentAction("system", "register"); ok {
		t.Fatal("system register is still a public action")
	}
	if _, problem := parseIntentArgs(mustIntentCommand(t, "system setup"), []string{"--check"}); problem == nil {
		t.Fatal("system setup still takes --check")
	}
	bed := newSystemSetupBed(t)
	layout, err := stateroot.ResolveLayout(bed.repo)
	if err != nil {
		t.Fatal(err)
	}
	drift := setupDrift(layout)
	if len(drift) == 0 || !strings.Contains(strings.Join(drift, "\n"), "metasystem system setup") {
		t.Fatalf("drift before setup = %q", drift)
	}
	if code, result := bed.run(t); code != 0 {
		t.Fatalf("setup = %d %+v", code, result)
	}
	if drift := setupDrift(layout); len(drift) != 0 {
		t.Fatalf("drift after setup = %q", drift)
	}
}

// An engine that does not serve the hook entry, or none at all, is refused
// before any write, and the refusal names the build that fixes it (H1).
func TestSystemSetupRefusesWithoutAnEngineThatServesTheHook(t *testing.T) {
	t.Parallel()
	bed := newSystemSetupBed(t)
	// An engine older than the hook entry: the fixture refuses --accepts.
	setupCLIWrite(t, filepath.Join(bed.installation, "bin", "metasystem.old"), "", 0o644)
	before := idemTreeDigest(t, bed.repo)
	hookBefore, _ := os.ReadFile(bed.hook)
	code, result := bed.run(t)
	if code == 0 || result.Outcome != intentRefused || !strings.Contains(result.Decision, "go run ./cmd/devgate build") ||
		!strings.Contains(result.Decision, "metasystem system setup") {
		t.Fatalf("old engine = %d %+v", code, result)
	}
	idemSameTree(t, "a switch refused for an old engine", before, idemTreeDigest(t, bed.repo))
	if err := os.Remove(filepath.Join(bed.installation, "bin", "metasystem")); err != nil {
		t.Fatal(err)
	}
	before = idemTreeDigest(t, bed.repo)
	code, result = bed.run(t)
	if code == 0 || result.Outcome != intentRefused || !strings.Contains(result.Decision, "go run ./cmd/devgate build") {
		t.Fatalf("missing engine = %d %+v", code, result)
	}
	idemSameTree(t, "a refused switch", before, idemTreeDigest(t, bed.repo))
	if hookAfter, _ := os.ReadFile(bed.hook); !bytes.Equal(hookBefore, hookAfter) {
		t.Fatal("a refused switch rewrote the pre-commit hook")
	}
}

// witnessSystemSetupRepeat: a checkout already switched is success,
// unchanged, and nothing is written (R-129).
func witnessSystemSetupRepeat(t *testing.T) {
	bed := newSystemSetupBed(t)
	if code, result := bed.run(t); code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("first switch = %d %+v", code, result)
	}
	before := idemTreeDigest(t, bed.repo)
	hookBefore, err := os.Stat(bed.hook)
	if err != nil {
		t.Fatal(err)
	}
	code, result := bed.run(t)
	if code != 0 || result.Outcome != intentUnchanged || result.Data.(map[string]any)["fence"] != "unchanged" {
		t.Fatalf("repeated switch = %d %+v", code, result)
	}
	idemSameTree(t, "a repeated switch", before, idemTreeDigest(t, bed.repo))
	if hookAfter, err := os.Stat(bed.hook); err != nil || !hookAfter.ModTime().Equal(hookBefore.ModTime()) {
		t.Fatalf("a repeated switch rewrote the pre-commit hook: %v", err)
	}
}
