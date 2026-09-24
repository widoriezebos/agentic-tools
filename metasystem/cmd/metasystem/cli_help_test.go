package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

func runCLIHelp(args []string, registered []family) (int, string, string) {
	var stdout, stderr bytes.Buffer
	code := dispatchWithFamilies(args, &stdout, &stderr, registered)
	return code, stdout.String(), stderr.String()
}

func TestRootHelpAliases(t *testing.T) {
	t.Parallel()
	registered := families()
	code, want, problem := runCLIHelp([]string{"help"}, registered)
	if code != 0 || problem != "" {
		t.Fatalf("root help = code %d, stderr %q", code, problem)
	}
	for _, alias := range []string{"--help", "-h"} {
		code, got, problem := runCLIHelp([]string{alias}, registered)
		if code != 0 || got != want || problem != "" {
			t.Errorf("root %s = code %d, stdout equal %t, stderr %q", alias, code, got == want, problem)
		}
	}
	code, output, problem := runCLIHelp(nil, registered)
	if code != 2 || output != "" || !strings.Contains(problem, "usage: metasystem") {
		t.Errorf("bare command = code %d, stdout %q, stderr %q", code, output, problem)
	}
}

func TestFamilyHelpAliasesForEveryRegisteredFamily(t *testing.T) {
	t.Parallel()
	registered := families()
	for _, fam := range registered {
		fam := fam
		t.Run(fam.name, func(t *testing.T) {
			t.Parallel()
			code, want, problem := runCLIHelp([]string{"help", fam.name}, registered)
			if code != 0 || problem != "" {
				t.Fatalf("help FAMILY = code %d, stderr %q", code, problem)
			}
			for _, args := range [][]string{{fam.name, "--help"}, {fam.name, "-h"}} {
				code, got, problem := runCLIHelp(args, registered)
				if code != 0 || got != want || problem != "" {
					t.Errorf("%v = code %d, stdout equal %t, stderr %q", args, code, got == want, problem)
				}
			}
			if !strings.Contains(want, fam.summary) {
				t.Errorf("family help omits summary %q", fam.summary)
			}
			for _, command := range fam.verbs {
				if !strings.Contains(want, command.name) || !strings.Contains(want, command.summary) {
					t.Errorf("family help omits registered verb %q", command.name)
				}
			}
			if len(fam.verbs) > 0 {
				example := "metasystem " + fam.name + " " + fam.verbs[0].name + " --help"
				if !strings.Contains(want, example) {
					t.Errorf("family help omits leaf-flag example %q", example)
				}
			}
		})
	}
}

func TestFamilyHelpDoesNotInvokeHandler(t *testing.T) {
	t.Parallel()
	called := 0
	registered := []family{{name: "safe", summary: "safe help", verbs: []verb{{
		name: "mutate", summary: "must remain idle", run: func([]string) int { called++; return 0 },
	}}}}
	for _, args := range [][]string{{"help", "safe"}, {"safe", "--help"}, {"safe", "-h"}} {
		if code, _, problem := runCLIHelp(args, registered); code != 0 || problem != "" {
			t.Fatalf("%v = code %d, stderr %q", args, code, problem)
		}
	}
	if called != 0 {
		t.Fatalf("family help invoked a handler %d times", called)
	}
}

func TestLaunchFamilyHelpShowsRecordCommands(t *testing.T) {
	t.Parallel()
	code, output, problem := runCLIHelp([]string{"launch", "--help"}, families())
	if code != 0 || problem != "" {
		t.Fatalf("launch help = code %d, stderr %q", code, problem)
	}
	for _, want := range []string{
		"metasystem launch start --help",
		"metasystem launch status --id <id>",
		"metasystem launch wait --id <id> [--timeout <duration>]",
		"metasystem launch cancel --id <id>",
		"current user", "~/.metasystem/launch", "--root is not a launch flag",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("launch help omits %q", want)
		}
	}
}

func TestLaunchRecordHelpAndInvalidInputsSkipManager(t *testing.T) {
	old := launchManager
	calls := 0
	launchManager = func() *launch.Manager { calls++; return nil }
	t.Cleanup(func() { launchManager = old })
	for _, verb := range []struct {
		name string
		run  func([]string) int
	}{{"status", runLaunchStatus}, {"cancel", runLaunchCancel}} {
		for _, alias := range []string{"--help", "-h"} {
			code, output, problem := captureCommandOutput(t, true, true, func() int { return verb.run([]string{alias}) })
			if code != 0 || problem != "" || !strings.Contains(output, "usage: metasystem launch "+verb.name+" --id <id>") || !strings.Contains(output, "~/.metasystem/launch") {
				t.Errorf("%s %s = code %d, stdout %q, stderr %q", verb.name, alias, code, output, problem)
			}
		}
		for _, args := range [][]string{nil, {"--root", "metasystem", "--id", "example"}, {"--unknown"}} {
			code, output, problem := captureCommandOutput(t, true, true, func() int { return verb.run(args) })
			if code != 2 || output != "" || !strings.Contains(problem, "usage: metasystem launch "+verb.name+" --id <id>") || !strings.Contains(problem, "--root is not a launch flag") || !strings.Contains(problem, "~/.metasystem/launch") {
				t.Errorf("%s %v = code %d, stdout %q, stderr %q", verb.name, args, code, output, problem)
			}
		}
	}
	if calls != 0 {
		t.Fatalf("launch help or invalid input called manager %d times", calls)
	}
}

func TestUnknownFamilyHelpAliasErrors(t *testing.T) {
	t.Parallel()
	registered := families()
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"help", "no-such-family"}, `unknown family "no-such-family"`},
		{[]string{"no-such-family", "--help"}, `unknown family "no-such-family"`},
		{[]string{"launch", "no-such-verb"}, `unknown verb "no-such-verb"`},
		{[]string{"help", "launch", "extra"}, "usage: metasystem help [FAMILY]"},
		{[]string{"launch", "--help", "extra"}, `unknown verb "--help"`},
		{[]string{"--help", "extra"}, "usage: metasystem help [FAMILY]"},
	}
	for _, test := range cases {
		code, output, problem := runCLIHelp(test.args, registered)
		if code != 2 || output != "" || !strings.Contains(problem, test.want) {
			t.Errorf("%v = code %d, stdout %q, stderr %q; want diagnostic %q", test.args, code, output, problem, test.want)
		}
	}
}

func TestKnownFamilyVerbErrorsShowOnlyThatFamily(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{{"launch"}, {"launch", "no-such-verb"}} {
		code, output, problem := runCLIHelp(args, families())
		if code != 2 || output != "" || !strings.Contains(problem, "usage: metasystem launch <verb> [flags]") || !strings.Contains(problem, "Flags are specific to each verb") || strings.Contains(problem, "usage: metasystem <family> <verb>") {
			t.Errorf("%v = code %d, stdout %q, stderr %q", args, code, output, problem)
		}
	}
}

func TestGoalOpenHelpShowsSharedFlagsBeforeAnyMutationInputs(t *testing.T) {
	calls := 0
	inputs := legacyMutationInputs{
		repositoryTop: func(string) (string, error) { calls++; return "", nil },
		ensureGuard:   func(string) error { calls++; return nil },
	}
	for _, alias := range []string{"--help", "-h"} {
		code, output, problem := captureCommandOutput(t, true, true, func() int {
			return goalMutationWithInputs("open", []string{alias}, nil, nil,
				func(string, []string) (int, bool) { calls++; return 0, false }, inputs)
		})
		if code != 0 || problem != "" || !strings.Contains(output, "Shared synced-goal options") || !strings.Contains(output, "each verb may accept fewer flags") {
			t.Errorf("goal open %s = code %d, stdout %q, stderr %q", alias, code, output, problem)
		}
		for _, want := range []string{"-id", "-intent", "-root", "-risk"} {
			if !strings.Contains(output, want) {
				t.Errorf("goal open %s help omits %q", alias, want)
			}
		}
	}
	if calls != 0 {
		t.Fatalf("goal open help consulted mutation inputs %d times", calls)
	}
}
