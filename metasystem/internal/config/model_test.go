package config

import "testing"

func TestCanonicalModel(t *testing.T) {
	cases := map[string]string{
		"Claude Opus 4.8":   "claude-opus-4-8",
		"  GPT-5.6-sol ":    "gpt-5-6-sol",
		"already-canonical": "already-canonical",
		"___":               "",
		"":                  "",
		"UPPER.case_MIX":    "upper-case-mix",
	}
	for in, want := range cases {
		if got := CanonicalModel(in); got != want {
			t.Errorf("CanonicalModel(%q) = %q, want %q", in, got, want)
		}
	}
}
