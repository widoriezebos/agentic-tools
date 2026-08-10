package census

import "testing"

// The real adapter patterns (verified against the shipped adapters), so the
// tests exercise the exact ERE shapes production uses.
var (
	claudeMatch = []string{`^([^[:space:]]*/)?claude([[:space:]]|$)`}
	claudeExcl  = []string{`claude-session-signal\.py`, `supervision-hook\.sh`, `scripts/agents/adapters/claude\.sh`}
	fakeMatch   = []string{`(^|[[:space:]/-])metasystem-fake-agent([[:space:]]|$)`}
	fakeExcl    = []string{`supervision-hook\.sh`, `scripts/agents/adapters/fake\.sh`}
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
		// KI-14: a tool shell quoting a claude path in an excluded file is
		// NOT claude.
		{"excluded session-signal", "python3 claude-session-signal.py", ""},
		{"excluded supervision hook", "bash supervision-hook.sh claude stop", ""},
		{"excluded adapter", "bash scripts/agents/adapters/claude.sh probe", ""},
		// A shell whose argv merely CONTAINS 'claude' mid-word is not
		// matched (the word-boundary anchor).
		{"claude substring not a word", "echo declaudetest", ""},
		{"fake agent", "metasystem-fake-agent first", "fake"},
		{"fake agent with leading path", "/tool/metasystem-fake-agent second", "fake"},
		{"fake excluded adapter", "bash scripts/agents/adapters/fake.sh signature", ""},
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
