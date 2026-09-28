package shellquote

import "testing"

func TestQuoteSingleQuotesEveryValue(t *testing.T) {
	for in, want := range map[string]string{
		"": "''", "plain": "'plain'", "a b": "'a b'", "it's": `'it'\''s'`, "$HOME": "'$HOME'",
	} {
		if got := Quote(in); got != want {
			t.Fatalf("Quote(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestWordLeavesSafeWordsBare(t *testing.T) {
	for in, want := range map[string]string{
		"": "''", "/usr/bin/metasystem": "/usr/bin/metasystem", "a,b=c@d%e+f:g": "a,b=c@d%e+f:g",
		"a b": "'a b'", "it's": `'it'\''s'`, "~x": "'~x'",
	} {
		if got := Word(in); got != want {
			t.Fatalf("Word(%q) = %s, want %s", in, got, want)
		}
	}
}

// Token matches bash's printf %q for the paths a refusal lists.
func TestTokenMatchesPrintfQ(t *testing.T) {
	for in, want := range map[string]string{
		"": "''", "a/b.go": "a/b.go", "a b": `a\ b`, "it's": `it\'s`, "~home": `\~home`, "a~b": "a~b",
		"é": "é", "line\nbreak": `$'line\nbreak'`, "tab\there": `$'tab\there'`, "bell\a": `$'bell\007'`,
		"q'\n": `$'q\'\n'`,
	} {
		if got := Token(in); got != want {
			t.Fatalf("Token(%q) = %s, want %s", in, got, want)
		}
	}
}
