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
		`scripts/agents/adapters/devin\.sh`,
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
		{"adapter itself", "bash scripts/agents/adapters/devin.sh probe", ""},
	}
	for _, row := range cases {
		t.Run(row.name, func(t *testing.T) {
			if got := Runtime(row.argv, sigs); got != row.want {
				t.Fatalf("Runtime(%q) = %q, want %q", row.argv, got, row.want)
			}
		})
	}
}

// The shipped adapter emits exactly the patterns the test above compiled —
// drift between the fixture and the adapter fails here, not in the field.
func TestDevinShippedSignatureMatchesFixture(t *testing.T) {
	text, err := SignatureText("../../scripts/agents/adapters/devin.sh")
	if err != nil {
		t.Skipf("shipped adapter unavailable: %v", err)
	}
	matches, excludes := ParseSignatureText(text)
	if len(matches) != len(devinMatch) || len(excludes) != len(devinExcl) {
		t.Fatalf("shipped devin signature drifted: %v / %v", matches, excludes)
	}
	for i, m := range devinMatch {
		if matches[i] != m {
			t.Fatalf("match %d drifted: %q vs %q", i, matches[i], m)
		}
	}
	for i, x := range devinExcl {
		if excludes[i] != x {
			t.Fatalf("exclude %d drifted: %q vs %q", i, excludes[i], x)
		}
	}
}
