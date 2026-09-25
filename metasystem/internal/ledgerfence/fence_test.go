package ledgerfence

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// composedHook is the hook Ensure writes for a checkout at prefix, built the
// same way Ensure builds it.
func composedHook(prefix, body string) string {
	return "#!/usr/bin/env bash\nguard=\"$(git rev-parse --show-toplevel)/\"" +
		shellSingleQuote(prefix+"scripts/agents/pre-commit-guard.sh") + "\n" + body
}

func TestEnsureRefusesACheckoutWithoutAnExecutableGuard(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		guard func(t *testing.T, path string)
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
			guard := filepath.Join(root, "scripts", "agents", "pre-commit-guard.sh")
			tc.guard(t, guard)
			err := Ensure(root)
			if err == nil {
				t.Fatal("Ensure enrolled a fence with no executable guard")
			}
			if !strings.Contains(err.Error(), "ships no executable pre-commit guard at "+guard) {
				t.Fatalf("refusal does not name the guard: %v", err)
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
		name string
		hook string
	}{
		{"fail-closed at the toplevel", composedHook("", composerBodyFailClosed)},
		{"fail-closed below the toplevel", composedHook("vendor/metasystem/", composerBodyFailClosed)},
		{"fail-open from an earlier version", composedHook("", composerBodyFailOpen)},
		{"quoted prefix carrying a quote and a space", composedHook("it's a dir/", composerBodyFailClosed)},
		{"pinned absolute path, double quoted", "#!/usr/bin/env bash\nguard=\"/repo/scripts/agents/pre-commit-guard.sh\"\n" + composerBodyFailOpen},
		{"pinned absolute path, single quoted", "#!/usr/bin/env bash\nguard='/repo/scripts/agents/pre-commit-guard.sh'\n" + composerBodyFailClosed},
		{"toplevel expansion inside the quotes", "#!/usr/bin/env bash\nguard=\"$(git rev-parse --show-toplevel)/scripts/agents/pre-commit-guard.sh\"\n" + composerBodyFailClosed},
	} {
		if !isOurComposer(tc.hook) {
			t.Errorf("%s: our own composer was not recognized:\n%s", tc.name, tc.hook)
		}
	}
}

func TestAHumansHookIsNeverTakenForOurComposer(t *testing.T) {
	t.Parallel()
	ours := composedHook("", composerBodyFailClosed)
	guardLine := strings.SplitN(ours, "\n", 3)[1]
	for _, tc := range []struct {
		name string
		hook string
	}{
		{"empty", ""},
		{"shebang only", "#!/usr/bin/env bash\n"},
		{"two lines", "#!/usr/bin/env bash\n" + guardLine},
		{"another shebang", strings.Replace(ours, "#!/usr/bin/env bash", "#!/bin/sh", 1)},
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
		{"extended body", ours + "run-my-check\n"},
		{"edited body", strings.Replace(ours, "exit 1", "exit 0", 1)},
		{"body missing its last newline", strings.TrimSuffix(ours, "\n")},
	} {
		if isOurComposer(tc.hook) {
			t.Errorf("%s: a foreign hook was taken for ours:\n%s", tc.name, tc.hook)
		}
	}
}

func TestTheComposerBodiesDifferOnlyInTheFence(t *testing.T) {
	t.Parallel()
	// Ensure tells the two bodies apart by the fail-closed test alone; the
	// upgrade would loop if the fail-open body carried it too.
	failClosedTest := "if [[ ! -x \"$guard\" ]]"
	if !strings.Contains(composerBodyFailClosed, failClosedTest) || strings.Contains(composerBodyFailOpen, failClosedTest) {
		t.Fatal("the fail-closed test does not separate the two composer bodies")
	}
	for _, body := range []string{composerBodyFailClosed, composerBodyFailOpen} {
		if !strings.Contains(body, `"$guard" || exit $?`) || !strings.Contains(body, `exec "$here/pre-commit.local" "$@"`) {
			t.Fatalf("a composer body does not run the guard and then the local hook:\n%s", body)
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
