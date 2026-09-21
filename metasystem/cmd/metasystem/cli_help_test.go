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
