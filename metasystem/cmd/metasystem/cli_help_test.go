package main

import (
	"bytes"
	"strings"
	"testing"
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
	// A bare call is read-only help and succeeds.
	code, output, problem := runCLIHelp(nil, registered)
	if code != 0 || output != want || problem != "" {
		t.Errorf("bare command = code %d, stdout equal %t, stderr %q", code, output == want, problem)
	}
}

func TestFamilyHelpAliasesForEveryRegisteredFamily(t *testing.T) {
	t.Parallel()
	registered := families()
	for _, fam := range registered {
		t.Run(fam.name, func(t *testing.T) {
			t.Parallel()
			code, want, problem := runCLIHelp([]string{"internal", fam.name, "--help"}, registered)
			if code != 0 || problem != "" {
				t.Fatalf("internal FAMILY --help = code %d, stderr %q", code, problem)
			}
			if code, got, problem := runCLIHelp([]string{"internal", fam.name, "-h"}, registered); code != 0 || got != want || problem != "" {
				t.Errorf("internal -h = code %d, stdout equal %t, stderr %q", code, got == want, problem)
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
				example := "metasystem internal " + fam.name + " " + fam.verbs[0].name + " --help"
				if !strings.Contains(want, example) {
					t.Errorf("family help omits leaf-flag example %q", example)
				}
			}
			for _, args := range [][]string{{"help", fam.name}, {fam.name, "--help"}, {fam.name, "-h"}} {
				code, got, problem := runCLIHelp(args, registered)
				if isIntentObject(fam.name) {
					var page bytes.Buffer
					writeIntentObjectHelp(&page, fam.name)
					if code != 0 || got != page.String() || problem != "" {
						t.Errorf("%v must show the object's public actions: %d %q %q", args, code, got, problem)
					}
				} else if code != 2 || got != "" || !strings.Contains(problem, "metasystem lists the objects") || strings.Contains(problem, "<verb>") {
					t.Errorf("%v must refuse without private discovery: %d %q %q", args, code, got, problem)
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
	for _, args := range [][]string{{"internal", "safe", "--help"}, {"internal", "safe", "-h"}} {
		if code, _, problem := runCLIHelp(args, registered); code != 0 || problem != "" {
			t.Fatalf("%v = code %d, stderr %q", args, code, problem)
		}
	}
	for _, args := range [][]string{{"help", "safe"}, {"safe"}, {"safe", "--help"}, {"help", "internal"}} {
		if code, out, problem := runCLIHelp(args, registered); code != 2 || out != "" || !strings.Contains(problem, "metasystem lists the objects") {
			t.Errorf("%v = code %d, stdout %q, stderr %q", args, code, out, problem)
		}
	}

	if called != 0 {
		t.Fatalf("family help invoked a handler %d times", called)
	}
}

func TestLaunchFamilyHelpShowsRecordCommands(t *testing.T) {
	t.Parallel()
	code, output, problem := runCLIHelp([]string{"internal", "launch", "--help"}, families())
	if code != 0 || problem != "" {
		t.Fatalf("launch help = code %d, stderr %q", code, problem)
	}
	for _, want := range []string{
		"metasystem internal launch start --help",
		"current user", "~/.metasystem/launch", "--root is not a launch flag",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("launch help omits %q", want)
		}
	}
}

func TestUnknownFamilyHelpAliasErrors(t *testing.T) {
	t.Parallel()
	registered := families()
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"help", "no-such-family"}, `unknown object "no-such-family"`},
		{[]string{"no-such-family", "--help"}, `unknown object "no-such-family"`},
		{[]string{"launch", "no-such-verb"}, `unknown object "launch"`},
		{[]string{"help", "launch", "extra"}, `unknown object "launch"`},
		{[]string{"launch", "--help", "extra"}, `unknown object "launch"`},
		{[]string{"--help", "extra"}, "usage: metasystem help [OBJECT [ACTION]]"},
		{[]string{"goal", "--help", "extra"}, "usage: metasystem goal ACTION"},
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
	for _, args := range [][]string{{"internal", "launch"}, {"internal", "launch", "no-such-verb"}} {
		code, output, problem := runCLIHelp(args, families())
		if code != 2 || output != "" || !strings.Contains(problem, "usage: metasystem internal launch <verb> [flags]") || !strings.Contains(problem, "Flags are specific to each verb") || strings.Contains(problem, "usage: metasystem <family> <verb>") {
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
