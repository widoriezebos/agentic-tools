package config

import (
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"os"
	"path/filepath"
	"testing"
)

// programs makes a directory holding one executable file per name, and a
// non-executable file for each name in plain.
func programs(t *testing.T, executable []string, plain ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range executable {
		if err := testexec.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range plain {
		putFile(t, filepath.Join(dir, name), "not a program\n")
	}
	return dir
}

// auto takes the first runtime of metasystem.runtimes, in its order, whose
// program is an executable on the asking environment's PATH; with none it
// takes the first listed; a key naming a runtime wins over it.
func TestAutoRuntimeFollowsThePathInPreferenceOrder(t *testing.T) {
	t.Parallel()
	conf := filepath.Join(t.TempDir(), "metasystem.conf")
	putFile(t, conf, "# overrides only\n")
	onlyDevin := programs(t, []string{"devin"}, "claude")
	codexAndDevin := programs(t, []string{"codex", "devin"})
	for _, test := range []struct {
		name, path, runtimes, want string
		detected                   bool
	}{
		{name: "a plain file is no program", path: onlyDevin, want: "devin", detected: true},
		{name: "list order decides", path: codexAndDevin, want: "codex", detected: true},
		{name: "a reordered list decides", path: codexAndDevin, runtimes: "claude,devin,codex", want: "devin", detected: true},
		{name: "nothing on PATH takes the first listed", path: t.TempDir(), want: "claude"},
		{name: "no PATH takes the first listed", runtimes: "codex,claude", want: "codex"},
	} {
		env := map[string]string{}
		if test.path != "" {
			env["PATH"] = t.TempDir() + string(os.PathListSeparator) + test.path
		}
		if test.runtimes != "" {
			env[EnvName("metasystem.runtimes")] = test.runtimes
		}
		lookup := mapEnv(env)
		for _, key := range []string{"role.default.runtime", "role.code-critic.runtime", "mode.design.role.implementer.runtime", "launch.read.runtime"} {
			if value, _, err := Get(GetParams{Key: key, ConfPath: conf, LookupEnv: lookup}); err != nil || value != test.want {
				t.Fatalf("%s: Get(%s) = %q, %v; want %q", test.name, key, value, err, test.want)
			}
		}
		if raw, _, err := Get(GetParams{Key: "launch.read.runtime", ConfPath: conf, LookupEnv: lookup, KeepAuto: true}); err != nil || raw != AutoRuntime {
			t.Fatalf("%s: KeepAuto = %q, %v", test.name, raw, err)
		}
		choice, err := ResolveAutoRuntime(conf, lookup)
		if err != nil || choice.Runtime != test.want || choice.Detected != test.detected {
			t.Fatalf("%s: choice = %+v, %v", test.name, choice, err)
		}
	}
	lookup := mapEnv(map[string]string{"PATH": codexAndDevin})
	putFile(t, conf, "launch.read.runtime=claude\n")
	if value, _, err := Get(GetParams{Key: "launch.read.runtime", ConfPath: conf, LookupEnv: lookup}); err != nil || value != "claude" {
		t.Fatalf("an explicit runtime lost to auto: %q, %v", value, err)
	}
	if value, _, err := Get(GetParams{Key: "launch.read.runtime", ConfPath: conf, LookupEnv: mapEnv(map[string]string{"PATH": codexAndDevin, EnvName("launch.read.runtime"): "auto"})}); err != nil || value != "codex" {
		t.Fatalf("auto from the environment = %q, %v", value, err)
	}
	if value, _, err := Get(GetParams{Key: "launch.read.window.tokens", ConfPath: conf, LookupEnv: lookup, Flag: "auto", FlagSet: true}); err != nil || value != "auto" {
		t.Fatalf("a key that picks no runtime resolved auto: %q, %v", value, err)
	}
}

func TestAutoRuntimeDescribesItsChoice(t *testing.T) {
	t.Parallel()
	order := []string{"claude", "codex", "devin"}
	if got := (RuntimeChoice{Runtime: "codex", Order: order, Detected: true}).Describe(); got != "auto: first of claude,codex,devin on PATH" {
		t.Fatalf("detected: %q", got)
	}
	if got := (RuntimeChoice{Runtime: "claude", Order: order}).Describe(); got != "auto: none of claude,codex,devin on PATH, first listed" {
		t.Fatalf("fallback: %q", got)
	}
	conf := filepath.Join(t.TempDir(), "metasystem.conf")
	putFile(t, conf, "metasystem.runtimes=\n")
	if _, err := ResolveAutoRuntime(conf, noEnv); err == nil {
		t.Fatal("an empty metasystem.runtimes resolved auto")
	}
}

// Validation answers for every host: an auto key needs a model on each
// detectable listed runtime and on the first listed one, and nothing more.
func TestValidateAutoNeedsAModelOnEveryRuntimeItCanBecome(t *testing.T) {
	t.Parallel()
	base := func(runtimes string) string {
		return "metasystem.version=1\nmetasystem.template=true\nmetasystem.runtimes=" + runtimes +
			"\ntesting.contract=testing.json\nruntime.claude.maximal-models=claude-fable-5-1\nevidence.root=@EVIDENCE@\nrole.default.runtime=claude\n"
	}
	// Fake is never detected, so an auto role becomes it only when it is
	// listed first, and must then have a fake model.
	if problems := validateRepo(t, base("fake,claude")); !hasProblem(problems, "role design-critic resolves to fake but has no model.fake value") {
		t.Fatalf("an auto role that can become fake passed without a fake model: %v", problems)
	}
	if problems := validateRepo(t, base("claude,fake")); len(problems) != 0 {
		t.Fatalf("an auto role that can never become fake needed a fake model: %v", problems)
	}
	lane := base("claude,codex") + "launch.design.model.codex=\n"
	if problems := validateRepo(t, lane); !hasProblem(problems, "launch.design resolves to codex but has neither launch.design.model nor launch.design.model.codex") {
		t.Fatalf("an auto lane without a Codex model passed: %v", problems)
	}
	if problems := validateRepo(t, lane+"launch.design.model=one-for-all\n"); len(problems) != 0 {
		t.Fatalf("a runtime-independent lane model was not enough: %v", problems)
	}
}
