package ledgerfence

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// legacyHook is a retired script-guard composer for a checkout at prefix,
// as earlier versions wrote it.
func legacyHook(prefix, body string) string {
	return "#!/usr/bin/env bash\nguard=\"$(git rev-parse --show-toplevel)/\"" +
		shellSingleQuote(prefix+"scripts/agents/pre-commit-guard.sh") + "\n" + body
}

func TestEnsureRefusesACheckoutWithoutAnExecutableEngine(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		engine func(t *testing.T, path string)
	}{
		{"absent", func(*testing.T, string) {}},
		{"not executable", func(t *testing.T, path string) {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("#!/usr/bin/env bash\nexit 0\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			engine := filepath.Join(root, "bin", "metasystem")
			tc.engine(t, engine)
			err := Ensure(root)
			if err == nil {
				t.Fatal("Ensure enrolled a fence with no executable engine")
			}
			if !strings.Contains(err.Error(), "no built metasystem at "+engine+" runs the commit hook") || !strings.Contains(err.Error(), "go run ./cmd/devgate build") {
				t.Fatalf("refusal does not name the engine and its build: %v", err)
			}
			// Refusing before any probe means nothing was written: no
			// repository, no hooks.
			if _, statErr := os.Stat(filepath.Join(root, ".git")); !os.IsNotExist(statErr) {
				t.Fatalf("refusal touched the checkout: %v", statErr)
			}
		})
	}
}

func TestOurComposerIsRecognizedByItsExactShape(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		hook    string
		current bool
	}{
		{"engine composer at the toplevel", composerFor(""), true},
		{"engine composer below the toplevel", composerFor("vendor/metasystem/"), true},
		{"engine composer, prefix carrying a quote and a space", composerFor("it's a dir/"), true},
		{"retired fail-closed at the toplevel", legacyHook("", composerBodyFailClosed), false},
		{"retired fail-closed below the toplevel", legacyHook("vendor/metasystem/", composerBodyFailClosed), false},
		{"retired fail-open", legacyHook("", composerBodyFailOpen), false},
		{"retired pinned absolute path, double quoted", "#!/usr/bin/env bash\nguard=\"/repo/scripts/agents/pre-commit-guard.sh\"\n" + composerBodyFailOpen, false},
		{"retired pinned absolute path, single quoted", "#!/usr/bin/env bash\nguard='/repo/scripts/agents/pre-commit-guard.sh'\n" + composerBodyFailClosed, false},
		{"retired toplevel expansion inside the quotes", "#!/usr/bin/env bash\nguard=\"$(git rev-parse --show-toplevel)/scripts/agents/pre-commit-guard.sh\"\n" + composerBodyFailClosed, false},
	} {
		if !isOurComposer(tc.hook) {
			t.Errorf("%s: our own composer was not recognized:\n%s", tc.name, tc.hook)
		}
		if isCurrentComposer(tc.hook) != tc.current {
			t.Errorf("%s: current=%t, want %t", tc.name, !tc.current, tc.current)
		}
	}
}

func TestAHumansHookIsNeverTakenForOurComposer(t *testing.T) {
	t.Parallel()
	legacy := legacyHook("", composerBodyFailClosed)
	guardLine := strings.SplitN(legacy, "\n", 3)[1]
	current := composerFor("vendor/")
	prefixLine := strings.SplitN(current, "\n", 3)[1]
	for _, tc := range []struct {
		name string
		hook string
	}{
		{"empty", ""},
		{"shebang only", "#!/usr/bin/env bash\n"},
		{"two lines", "#!/usr/bin/env bash\n" + guardLine},
		{"another shebang", strings.Replace(legacy, "#!/usr/bin/env bash", "#!/bin/sh", 1)},
		{"no guard assignment", "#!/usr/bin/env bash\necho hi\n" + composerBodyFailClosed},
		{"trailing command", "#!/usr/bin/env bash\n" + guardLine + "; run-check\n" + composerBodyFailClosed},
		{"background", "#!/usr/bin/env bash\n" + guardLine + " & x\n" + composerBodyFailClosed},
		{"pipe", "#!/usr/bin/env bash\nguard=\"$(x | y)/scripts/agents/pre-commit-guard.sh\"\n" + composerBodyFailClosed},
		{"backticks", "#!/usr/bin/env bash\nguard=\"`x`/scripts/agents/pre-commit-guard.sh\"\n" + composerBodyFailClosed},
		{"input process substitution", "#!/usr/bin/env bash\nguard=\"<(x)/scripts/agents/pre-commit-guard.sh\"\n" + composerBodyFailClosed},
		{"output process substitution", "#!/usr/bin/env bash\nguard=\">(x)/scripts/agents/pre-commit-guard.sh\"\n" + composerBodyFailClosed},
		{"another guard file", "#!/usr/bin/env bash\nguard=\"/repo/scripts/agents/other-guard.sh\"\n" + composerBodyFailClosed},
		{"unquoted guard", "#!/usr/bin/env bash\nguard=/repo/scripts/agents/pre-commit-guard.sh\n" + composerBodyFailClosed},
		{"foreign substitution", "#!/usr/bin/env bash\nguard=\"$(pwd)/scripts/agents/pre-commit-guard.sh\"\n" + composerBodyFailClosed},
		{"second substitution", "#!/usr/bin/env bash\nguard=\"$(git rev-parse --show-toplevel)/$(pwd)/scripts/agents/pre-commit-guard.sh\"\n" + composerBodyFailClosed},
		{"extended retired body", legacy + "run-my-check\n"},
		{"edited retired body", strings.Replace(legacy, "exit 1", "exit 0", 1)},
		{"retired body missing its last newline", strings.TrimSuffix(legacy, "\n")},
		{"engine body behind a guard line", "#!/usr/bin/env bash\n" + guardLine + "\n" + composerBodyEngine},
		{"prefix line with a trailing command", "#!/usr/bin/env bash\n" + prefixLine + "; run-check\n" + composerBodyEngine},
		{"unquoted prefix", "#!/usr/bin/env bash\nprefix=vendor/\n" + composerBodyEngine},
		{"double-quoted prefix", "#!/usr/bin/env bash\nprefix=\"$(pwd)\"\n" + composerBodyEngine},
		{"extended engine body", current + "run-my-check\n"},
		{"edited engine body", strings.Replace(current, "exit 1", "exit 0", 1)},
	} {
		if isOurComposer(tc.hook) {
			t.Errorf("%s: a foreign hook was taken for ours:\n%s", tc.name, tc.hook)
		}
	}
}

func TestTheComposerBodiesRunTheGuardThenTheLocalHook(t *testing.T) {
	t.Parallel()
	for _, want := range []string{
		`"$engine" internal pre-commit --root "$installation"`,
		`[[ $status -eq 0 ]] || exit $status`,
		`exec "$here/pre-commit.local" "$@"`,
		"--git-common-dir",
		"go run ./cmd/devgate build",
	} {
		if !strings.Contains(composerBodyEngine, want) {
			t.Fatalf("the engine composer lacks %q:\n%s", want, composerBodyEngine)
		}
	}
	// The retired bodies stay distinct from the current one, so a retired
	// composer always upgrades and the current one never loops.
	for _, body := range []string{composerBodyFailClosed, composerBodyFailOpen} {
		if body == composerBodyEngine || !strings.Contains(body, `"$guard" || exit $?`) {
			t.Fatalf("a retired composer body is not the retired script guard:\n%s", body)
		}
	}
}

func TestShellSingleQuoteKeepsEveryByteLiteral(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"":                  "''",
		"scripts/agents/x":  "'scripts/agents/x'",
		"a b":               "'a b'",
		"$(rm -rf /)`x`\"y": "'$(rm -rf /)`x`\"y'",
		"it's":              `'it'\''s'`,
		"''":                `''\'''\'''`,
	} {
		if got := shellSingleQuote(in); got != want {
			t.Errorf("shellSingleQuote(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestEnvironmentLosesOnlyGitSteering(t *testing.T) {
	t.Parallel()
	steering := func(name string) bool {
		switch name {
		case "GIT_DIR", "GIT_WORK_TREE", "GIT_COMMON_DIR", "GIT_INDEX_FILE",
			"GIT_CEILING_DIRECTORIES", "GIT_OBJECT_DIRECTORY",
			"GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_CONFIG", "GIT_CONFIG_PARAMETERS",
			"GIT_CONFIG_COUNT", "GIT_CONFIG_GLOBAL", "GIT_CONFIG_SYSTEM",
			"GIT_CONFIG_NOSYSTEM", "GIT_GRAFT_FILE", "GIT_SHALLOW_FILE",
			"GIT_REPLACE_REF_BASE":
			return true
		}
		return strings.HasPrefix(name, "GIT_CONFIG_KEY_") || strings.HasPrefix(name, "GIT_CONFIG_VALUE_")
	}
	// The process environment is read, never written: the expectation is
	// derived from the same snapshot the scrub sees.
	var want []string
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if !steering(name) {
			want = append(want, entry)
		}
	}
	got := EnvironWithoutGitSteering()
	if !slices.Equal(got, want) {
		t.Fatalf("scrubbed environment differs:\n got %q\nwant %q", got, want)
	}
}

// fenceGitAdapterRepo is a repository whose engine is a stand-in answering
// only the pre-commit entry: the probe with its acknowledgment, otherwise the
// verdict in verdict (exit status 0 admits). Git is the claim of these tests:
// they prove where git runs the composed hook and what it composes with.
func fenceGitAdapterRepo(t *testing.T, verdict int) (string, string) {
	t.Helper()
	root := t.TempDir()
	fenceGit(t, root, "init", "-q", "-b", "main")
	fenceGit(t, root, "config", "user.name", "fixture")
	fenceGit(t, root, "config", "user.email", "fixture@example.invalid")
	marker := filepath.Join(t.TempDir(), "guard-ran")
	engine := "#!/usr/bin/env bash\n" +
		"[[ \"$1 $2\" == \"internal pre-commit\" ]] || exit 2\n" +
		"if [[ -n \"${METASYSTEM_GUARD_PROBE:-}\" ]]; then echo \"guard-probe-ack $METASYSTEM_GUARD_PROBE\"; exit 42; fi\n" +
		"printf '%s\\n' \"$4\" >>" + shellSingleQuote(marker) + "\n" +
		"exit " + strconv.Itoa(verdict) + "\n"
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(filepath.Join(root, "bin", "metasystem"), []byte(engine), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("bin/\n.githooks/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fenceGit(t, root, "add", ".gitignore")
	fenceGit(t, root, "commit", "-qm", "seed", "--no-verify")
	return root, marker
}

func fenceGit(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", dir}, args...)...)
	command.Env = EnvironWithoutGitSteering()
	output, err := command.CombinedOutput()
	if err != nil && (len(args) == 0 || args[0] != "commit") {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return string(output), err
}

// TestEnsureComposesARejectingLocalHookGitAdapter (VOA-20): an existing hook
// is kept as pre-commit.local behind the engine guard and still rejects.
func TestEnsureComposesARejectingLocalHookGitAdapter(t *testing.T) {
	t.Parallel()
	root, marker := fenceGitAdapterRepo(t, 0)
	local := filepath.Join(root, ".git", "hooks", "pre-commit")
	if err := testexec.WriteFile(local, []byte("#!/usr/bin/env bash\necho 'project check refuses' >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Ensure(root); err != nil {
		t.Fatal(err)
	}
	hook, err := os.ReadFile(local)
	if err != nil || !isCurrentComposer(string(hook)) {
		t.Fatalf("the enrolled hook is not the engine composer: %v\n%s", err, hook)
	}
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fenceGit(t, root, "add", "a.txt")
	output, err := fenceGit(t, root, "commit", "-qm", "a")
	if err == nil || !strings.Contains(output, "project check refuses") {
		t.Fatalf("the local hook no longer rejects: %v\n%s", err, output)
	}
	if ran, _ := os.ReadFile(marker); !strings.Contains(string(ran), root) {
		t.Fatalf("the guard did not run before the local hook for %s: %q", root, ran)
	}
	if err := Ensure(root); err != nil {
		t.Fatalf("an enrolled fence refused re-enrollment: %v", err)
	}
}

// TestEnsureFencesARepositoryLocalHooksPathGitAdapter (VOA-20): a configured
// hooks directory inside the repository receives the fence, and git runs it.
func TestEnsureFencesARepositoryLocalHooksPathGitAdapter(t *testing.T) {
	t.Parallel()
	root, marker := fenceGitAdapterRepo(t, 1)
	fenceGit(t, root, "config", "core.hooksPath", ".githooks")
	if err := Ensure(root); err != nil {
		t.Fatal(err)
	}
	if hook, err := os.ReadFile(filepath.Join(root, ".githooks", "pre-commit")); err != nil || !isCurrentComposer(string(hook)) {
		t.Fatalf("the configured hooks directory has no fence: %v\n%s", err, hook)
	}
	if _, err := os.Stat(filepath.Join(root, ".git", "hooks", "pre-commit")); !os.IsNotExist(err) {
		t.Fatalf("the fence was written where git does not look: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fenceGit(t, root, "add", "a.txt")
	if output, err := fenceGit(t, root, "commit", "-qm", "a"); err == nil {
		t.Fatalf("a refusing guard admitted the commit:\n%s", output)
	}
	if ran, _ := os.ReadFile(marker); !strings.Contains(string(ran), root) {
		t.Fatalf("git never ran the fence: %q", ran)
	}
}

// TestEnsureUpgradesARetiredScriptComposerGitAdapter: a checkout enrolled
// under the deleted script guard is recomposed in place, keeping its local
// hook, so its commits run the engine guard.
func TestEnsureUpgradesARetiredScriptComposerGitAdapter(t *testing.T) {
	t.Parallel()
	root, marker := fenceGitAdapterRepo(t, 0)
	hooks := filepath.Join(root, ".git", "hooks")
	if err := testexec.WriteFile(filepath.Join(hooks, "pre-commit"), []byte(legacyHook("", composerBodyFailClosed)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(filepath.Join(hooks, "pre-commit.local"), []byte("#!/usr/bin/env bash\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Ensure(root); err != nil {
		t.Fatal(err)
	}
	hook, err := os.ReadFile(filepath.Join(hooks, "pre-commit"))
	if err != nil || !isCurrentComposer(string(hook)) {
		t.Fatalf("the retired composer was not upgraded: %v\n%s", err, hook)
	}
	if local, err := os.ReadFile(filepath.Join(hooks, "pre-commit.local")); err != nil || string(local) != "#!/usr/bin/env bash\nexit 0\n" {
		t.Fatalf("the local hook was not kept: %v %q", err, local)
	}
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fenceGit(t, root, "add", "a.txt")
	if output, err := fenceGit(t, root, "commit", "-qm", "a"); err != nil {
		t.Fatalf("the upgraded fence refused an admitted commit: %v\n%s", err, output)
	}
	if ran, _ := os.ReadFile(marker); !strings.Contains(string(ran), root) {
		t.Fatalf("the upgraded hook did not run the engine guard: %q", ran)
	}
}

// TestLinkedWorktreeRunsThePrimaryEngineGitAdapter: a linked worktree has no
// built engine of its own; enrollment accepts, and the composer runs, its
// primary checkout's.
func TestLinkedWorktreeRunsThePrimaryEngineGitAdapter(t *testing.T) {
	t.Parallel()
	root, marker := fenceGitAdapterRepo(t, 0)
	if err := Ensure(root); err != nil {
		t.Fatal(err)
	}
	linked := filepath.Join(t.TempDir(), "linked")
	fenceGit(t, root, "worktree", "add", "-q", "-b", "side", linked)
	if _, err := os.Stat(filepath.Join(linked, "bin", "metasystem")); !os.IsNotExist(err) {
		t.Fatalf("the linked worktree has its own engine: %v", err)
	}
	if err := Ensure(linked); err != nil {
		t.Fatalf("enrollment from a linked worktree refused the primary engine: %v", err)
	}
	if err := os.WriteFile(filepath.Join(linked, "b.txt"), []byte("b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fenceGit(t, linked, "add", "b.txt")
	if output, err := fenceGit(t, linked, "commit", "-qm", "b"); err != nil {
		t.Fatalf("the linked worktree could not commit: %v\n%s", err, output)
	}
	resolved, err := filepath.EvalSymlinks(linked)
	if err != nil {
		t.Fatal(err)
	}
	if ran, _ := os.ReadFile(marker); !strings.Contains(string(ran), resolved) && !strings.Contains(string(ran), linked) {
		t.Fatalf("the primary engine did not guard the linked worktree's commit: %q", ran)
	}
}

// TestEnsureLeavesAnEnrolledFenceAloneGitAdapter: a hook that already runs
// the guard is enrolled, so a repeat Ensure writes nothing (R-129), and a
// person's own hook that composes the guard is never moved aside. The probe's
// status was once read after its context was cancelled, so no hook ever
// counted as enrolled and every Ensure rewrote it.
func TestEnsureLeavesAnEnrolledFenceAloneGitAdapter(t *testing.T) {
	t.Parallel()
	root, _ := fenceGitAdapterRepo(t, 0)
	hook := filepath.Join(root, ".git", "hooks", "pre-commit")
	if err := Ensure(root); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(hook)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(hook, before.ModTime().Add(-time.Hour), before.ModTime().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	before, _ = os.Stat(hook)
	if err := Ensure(root); err != nil {
		t.Fatal(err)
	}
	if after, err := os.Stat(hook); err != nil || !after.ModTime().Equal(before.ModTime()) {
		t.Fatalf("Ensure rewrote an enrolled composer: %v", err)
	}
	// A person's own hook that runs the guard itself is enrolled as it is.
	own := "#!/usr/bin/env bash\n\"$(git rev-parse --show-toplevel)/bin/metasystem\" internal pre-commit --root \"$(git rev-parse --show-toplevel)\" || exit $?\necho mine >/dev/null\n"
	if err := testexec.WriteFile(hook, []byte(own), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Ensure(root); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(hook); err != nil || string(data) != own {
		t.Fatalf("a person's enrolled hook was replaced: %v\n%s", err, data)
	}
	if _, err := os.Stat(filepath.Join(root, ".git", "hooks", "pre-commit.local")); !os.IsNotExist(err) {
		t.Fatalf("a person's enrolled hook was moved aside: %v", err)
	}
}

// A target without a repository has no fence to enroll, and that is told by
// the filesystem, never by git's words: a checkout whose .git git cannot
// follow is a repository whose shape cannot be proven, and Ensure refuses.
func TestEnsureTellsNoRepositoryFromAnUnreadableOne(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		gitFile bool
	}{{"no repository", false}, {"unreadable repository", true}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			engine := filepath.Join(root, "bin", "metasystem")
			if err := os.MkdirAll(filepath.Dir(engine), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := testexec.WriteFile(engine, []byte("#!/usr/bin/env bash\nexit 0\n"), 0o755); err != nil {
				t.Fatal(err)
			}
			if tc.gitFile {
				if err := os.WriteFile(filepath.Join(root, ".git"), []byte("gitdir: "+filepath.Join(root, "missing")+"\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			err := Ensure(root)
			if tc.gitFile != (err != nil) || tc.gitFile && !strings.Contains(err.Error(), "cannot be proven") {
				t.Fatalf("Ensure = %v", err)
			}
		})
	}
}
