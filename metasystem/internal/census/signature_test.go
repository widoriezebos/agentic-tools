package census

import (
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
)

// supervisorExcl keeps the delegate-supervisor process out of every
// runtime's signature (it launches the CLI; it is not the CLI).
const supervisorExcl = `(^|[[:space:]/])metasystem[[:space:]]+(internal[[:space:]]+)?delegate-supervisor([[:space:]]|$)`

// The real registry patterns (verified against the runtime registry by the
// shipped-signature tests below), so the tests exercise the exact ERE shapes
// production uses.
var (
	claudeMatch = []string{`^([^[:space:]]*/)?claude([[:space:]]|$)`}
	claudeExcl  = []string{`claude-session-signal\.py`, `supervision-hook\.sh`, supervisorExcl}
	fakeMatch   = []string{`(^|[[:space:]/-])metasystem-fake-agent([[:space:]]|$)`}
	fakeExcl    = []string{`supervision-hook\.sh`, supervisorExcl}
)

func testSignatures(t *testing.T) []Signature {
	t.Helper()
	claude, err := CompileSignature("claude", claudeMatch, claudeExcl)
	if err != nil {
		t.Fatal(err)
	}
	fake, err := CompileSignature("fake", fakeMatch, fakeExcl)
	if err != nil {
		t.Fatal(err)
	}
	return []Signature{claude, fake}
}

func TestRuntimeClassification(t *testing.T) {
	sigs := testSignatures(t)
	cases := []struct {
		name string
		argv string
		want string
	}{
		{"bare claude command", "claude --flag", "claude"},
		{"claude by full path", "/usr/local/bin/claude serve", "claude"},
		// A tool shell quoting a claude path in an excluded file is
		// NOT claude.
		{"excluded session-signal", "python3 claude-session-signal.py", ""},
		{"excluded supervision hook", "bash supervision-hook.sh claude stop", ""},
		// The stub execs the engine's hook entry; that process names the
		// runtime only as an argument and is never the runtime itself.
		{"engine hook entry", "/repo/metasystem/bin/metasystem internal hook claude stop", ""},
		{"engine hook entry for fake", "/repo/metasystem/bin/metasystem internal hook fake stop", ""},
		{"excluded supervisor", "/repo/bin/metasystem delegate-supervisor claude dispatch --job j", ""},
		{"excluded supervisor naming claude first", "claude /repo/bin/metasystem delegate-supervisor claude dispatch", ""},
		// A shell whose argv merely CONTAINS 'claude' mid-word is not
		// matched (the word-boundary anchor).
		{"claude substring not a word", "echo declaudetest", ""},
		{"fake agent", "metasystem-fake-agent first", "fake"},
		{"fake agent with leading path", "/tool/metasystem-fake-agent second", "fake"},
		{"fake excluded supervisor", "metasystem-fake-agent /repo/bin/metasystem delegate-supervisor fake dispatch", ""},
		{"unrelated process", "vim notes.md", ""},
	}
	for _, row := range cases {
		t.Run(row.name, func(t *testing.T) {
			if got := Runtime(row.argv, sigs); got != row.want {
				t.Fatalf("Runtime(%q) = %q, want %q", row.argv, got, row.want)
			}
		})
	}
}

// Order is load-bearing: the first runtime in the list that claims an argv
// wins (first match in declaration order).
func TestClassifyIsOrderedFirstMatchWins(t *testing.T) {
	// Two runtimes that both match the same argv; the first wins.
	a, _ := CompileSignature("first", []string{`shared`}, nil)
	b, _ := CompileSignature("second", []string{`shared`}, nil)
	if got := Runtime("shared token", []Signature{a, b}); got != "first" {
		t.Fatalf("first-in-order must win, got %q", got)
	}
	if got := Runtime("shared token", []Signature{b, a}); got != "second" {
		t.Fatalf("order reversal must flip the winner, got %q", got)
	}
}

func TestClassifyBatchReturnsOnlyMatches(t *testing.T) {
	sigs := testSignatures(t)
	argvs := []string{
		"claude serve",              // 0: claude
		"vim x",                     // 1: none
		"metasystem-fake-agent job", // 2: fake
		"bash supervision-hook.sh",  // 3: excluded
	}
	got := Classify(argvs, sigs)
	if len(got) != 2 {
		t.Fatalf("want 2 assignments, got %d: %+v", len(got), got)
	}
	if got[0].Index != 0 || got[0].Runtime != "claude" || got[1].Index != 2 || got[1].Runtime != "fake" {
		t.Fatalf("wrong assignments: %+v", got)
	}
}

func TestCompileRejectsInvalidPattern(t *testing.T) {
	if _, err := CompileSignature("bad", []string{`[unterminated`}, nil); err == nil {
		t.Fatal("an invalid ERE must fail compilation")
	}
}

// The Devin signature's issue-#12 shapes: the HOST CLI's internal raw
// `devin acp` helper is excluded (it sits between the announced main and
// every orchestrator tool shell), while the delegate-side server this
// repository launches under argv0 devin-delegate-acp still matches — so
// hosts classify MAIN through the exclusion and delegates stay DELEGATE.
var (
	devinMatch = []string{
		`^([^[:space:]]*/)?devin([[:space:]]|$)`,
		`^([^[:space:]]*/)?devin-delegate-acp([[:space:]]|$)`,
	}
	devinExcl = []string{
		`^([^[:space:]]*/)?devin[[:space:]]+acp([[:space:]]|$)`,
		`supervision-hook\.sh`,
		supervisorExcl,
	}
)

func TestDevinSignatureIssue12Shapes(t *testing.T) {
	devin, err := CompileSignature("devin", devinMatch, devinExcl)
	if err != nil {
		t.Fatal(err)
	}
	sigs := []Signature{devin}
	cases := []struct {
		name string
		argv string
		want string
	}{
		{"host CLI", "devin -p -- do the mission turn", "devin"},
		{"host CLI by path", "/Users/w/.local/bin/devin -p", "devin"},
		{"bare devin", "devin", "devin"},
		// The host's internal ACP helper: EXCLUDED, so the ancestry walk
		// continues upward to the announced main.
		{"host acp helper", "devin acp", ""},
		{"host acp helper by path", "/Users/w/.local/bin/devin acp", ""},
		// The delegate-side server this adapter launches: argv0-marked,
		// still a delegate signature.
		{"delegate acp server", "devin-delegate-acp acp", "devin"},
		{"delegate acp server by path", "/Users/w/.local/bin/devin-delegate-acp acp", "devin"},
		{"declared lookalike", "metasystem-devin-lookalike", ""},
		{"supervisor itself", "devin /repo/bin/metasystem internal delegate-supervisor devin follow-up", ""},
	}
	for _, row := range cases {
		t.Run(row.name, func(t *testing.T) {
			if got := Runtime(row.argv, sigs); got != row.want {
				t.Fatalf("Runtime(%q) = %q, want %q", row.argv, got, row.want)
			}
		})
	}
}

// The runtime registry declares exactly the patterns the tests above
// compiled — drift between the fixtures and the registry fails here, not in
// the field.
func TestShippedSignaturesMatchFixtures(t *testing.T) {
	for _, row := range []struct {
		runtime           string
		matches, excludes []string
	}{
		{"devin", devinMatch, devinExcl},
		{"claude", claudeMatch, claudeExcl},
		{"fake", fakeMatch, fakeExcl},
	} {
		text, err := runtimes.SignatureText(row.runtime)
		if err != nil {
			t.Fatal(err)
		}
		matches, excludes := ParseSignatureText(text)
		if len(matches) != len(row.matches) || len(excludes) != len(row.excludes) {
			t.Fatalf("shipped %s signature drifted: %v / %v", row.runtime, matches, excludes)
		}
		for i, m := range row.matches {
			if matches[i] != m {
				t.Fatalf("%s match %d drifted: %q vs %q", row.runtime, i, matches[i], m)
			}
		}
		for i, x := range row.excludes {
			if excludes[i] != x {
				t.Fatalf("%s exclude %d drifted: %q vs %q", row.runtime, i, excludes[i], x)
			}
		}
	}
}
