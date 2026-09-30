package main

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

// The command line never confuses its caller (Wido, 2026-09-30: "An error
// message that confuses you IS REALLY BAD."). These witnesses pin the four
// rules for the internal surface that survives, and that every public miss
// names the nearest real command.

func runCLI(args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	code := dispatchWithFamilies(args, &stdout, &stderr, families())
	return code, stdout.String(), stderr.String()
}

// A bare family word or a family word with a verb it lacks is answered with
// that family's own help, marked internal; never "unknown object".
func TestFamilyMissesAnswerWithTheFamilysInternalHelp(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		args   []string
		family string
	}{
		{[]string{"config"}, "config"}, {[]string{"config", "gett"}, "config"}, {[]string{"validate"}, "validate"},
		{[]string{"proof-run", "nosuch"}, "proof-run"}, {[]string{"internal", "config"}, "config"}, {[]string{"internal", "config", "gett"}, "config"},
	} {
		code, _, stderr := runCLI(row.args...)
		if code != 2 || strings.Contains(stderr, "unknown object") || !strings.Contains(stderr, "metasystem internal "+row.family+":") ||
			!strings.Contains(stderr, "process entrypoints") {
			t.Errorf("%v: code %d stderr %q", row.args, code, stderr)
		}
	}
}

// An internal word no entrypoint answers points at metasystem internal.
func TestInternalMissPointsAtTheInternalList(t *testing.T) {
	t.Parallel()
	code, _, stderr := runCLI("internal", "nosuch")
	if code != 2 || strings.Contains(stderr, "unknown object") || !strings.Contains(stderr, "run: metasystem help") || !strings.Contains(stderr, "metasystem internal lists the entrypoints") || !strings.Contains(stderr, `"nosuch"`) {
		t.Fatalf("internal miss: code %d stderr %q", code, stderr)
	}
}

// Every internal entrypoint answers an unknown flag in the public style,
// before it does anything.
func TestEveryInternalEntrypointNamesAnUnknownFlag(t *testing.T) {
	const bogus = "--u9b-no-such-option"
	check := func(t *testing.T, name string, run func(stdout, stderr io.Writer) int) {
		code, stdout, stderr := runOnOwnStreams(run)
		if code != 2 || strings.Contains(stderr, "flag provided but not defined") || strings.Contains(stderr, "Usage of") ||
			!strings.Contains(stderr, "does not take "+bogus) {
			t.Errorf("%s %s: code %d stdout %q stderr %q", name, bogus, code, stdout, stderr)
		}
	}
	for _, fam := range families() {
		for _, v := range fam.verbs {
			v := v
			check(t, fam.name+" "+v.name, func(stdout, stderr io.Writer) int { return v.run([]string{bogus}, stdout, stderr) })
		}
	}
	for _, entry := range topLevelEntries() {
		entry := entry
		check(t, entry.name, func(stdout, stderr io.Writer) int {
			return entry.run([]string{bogus}, stdout, stderr, func(string) (string, error) { return "", nil })
		})
	}
}

// Every public miss names the nearest real command, however far it is.
func TestEveryPublicMissNamesTheNearestCommand(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{{"zzzzzzzzzz"}, {"goal", "zzzzzzzzzz"}, {"work", "qqqqqqqqqqqq"}} {
		code, _, stderr := runCLI(args...)
		if code != 2 || !strings.Contains(stderr, "did you mean: metasystem ") {
			t.Errorf("%v: code %d stderr %q", args, code, stderr)
		}
	}
}

// Every entrypoint that cannot run without an option names it as required,
// before it does anything.
func TestEveryInternalEntrypointNamesItsRequiredOptions(t *testing.T) {
	check := func(t *testing.T, name, first string, run func(stdout, stderr io.Writer) int) {
		code, stdout, stderr := runOnOwnStreams(run)
		if code != 2 || !strings.Contains(stderr, "--"+first+" is required") {
			t.Errorf("%s without options: code %d stdout %q stderr %q", name, code, stdout, stderr)
		}
	}
	for _, fam := range families() {
		for _, v := range fam.verbs {
			if len(v.required) == 0 {
				continue
			}
			v := v
			check(t, fam.name+" "+v.name, v.required[0], func(stdout, stderr io.Writer) int { return v.run(nil, stdout, stderr) })
		}
	}
	for _, entry := range topLevelEntries() {
		if len(entry.required) == 0 {
			continue
		}
		entry := entry
		check(t, entry.name, entry.required[0], func(stdout, stderr io.Writer) int {
			return entry.run(nil, stdout, stderr, func(string) (string, error) { return "", nil })
		})
	}
}

// Every internal entrypoint answers --help with its options and succeeds,
// before it does anything; a help request is never refused as an unknown
// option.
func TestEveryInternalEntrypointAnswersHelp(t *testing.T) {
	check := func(t *testing.T, name string, run func(stdout, stderr io.Writer) int) {
		code, stdout, stderr := runOnOwnStreams(run)
		if code != 0 || strings.Contains(stderr, "does not take") || !strings.Contains(stdout, "usage: metasystem internal "+name) {
			t.Errorf("%s --help: code %d stdout %q stderr %q", name, code, stdout, stderr)
		}
	}
	for _, fam := range families() {
		for _, v := range fam.verbs {
			v := v
			check(t, fam.name+" "+v.name, func(stdout, stderr io.Writer) int { return v.run([]string{"--help"}, stdout, stderr) })
		}
	}
	for _, entry := range topLevelEntries() {
		entry := entry
		check(t, entry.name, func(stdout, stderr io.Writer) int {
			return entry.run([]string{"--help"}, stdout, stderr, func(string) (string, error) { return "", nil })
		})
	}
}

// A help lookup of a process entrypoint's word says what the word is.
func TestHelpOfAnEntrypointWordSaysWhatItIs(t *testing.T) {
	t.Parallel()
	for _, row := range []struct{ word, want string }{
		{"launch", "family of process entrypoints; metasystem internal launch lists them"},
		{"up", "process entrypoint that internal/testrun/rearm.go starts"},
	} {
		code, _, stderr := runCLI("help", row.word)
		if code != 2 || strings.Contains(stderr, "unknown object") || !strings.Contains(stderr, row.want) {
			t.Errorf("help %s: code %d stderr %q", row.word, code, stderr)
		}
	}
}
