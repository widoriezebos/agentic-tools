package main

import "testing"

// The public resolve-taint parser speaks exactly the amended design
// grammar (slice-6 successor round-3 finding 2): one typed act — either
// --restore <treeId> or --adopt — beside the taint id; anything else is
// a usage refusal (exit 2) before any engine work.
func TestResolveTaintParserGrammar(t *testing.T) {
	refusals := [][]string{
		{},
		{"--root", "/nonexistent", "--mission", "alpha", "--taint", "1"},
		{"--root", "/nonexistent", "--mission", "alpha", "--taint", "1", "--restore", "abc", "--adopt"},
		{"--root", "/nonexistent", "--mission", "alpha", "--taint", "0", "--adopt"},
		{"--root", "/nonexistent", "--mission", "alpha", "--taint", "x", "--adopt"},
		{"--root", "/nonexistent", "--mission", "alpha", "--restore", "abc"},
		{"--root", "/nonexistent", "--mission", "alpha", "--taint", "1", "--variant", "restore", "--tree", "abc"},
		{"--root", "/nonexistent", "--mission", "alpha", "--taint", "1", "--restore", "abc", "--reason", "r"},
		{"--root", "/nonexistent", "--mission", "alpha", "--taint", "1", "--restore", "abc", "--by", "Wido"},
		{"--root", "/nonexistent", "--mission", "alpha", "--taint", "1", "--adopt", "--by", "Wido", "--reason", "r"},
		{"--root", "/nonexistent", "--mission", "alpha", "--taint", "1", "--adopt", "--by", "Wido", "--reason", "r", "--waives", "   "},
		{"--root", "/nonexistent", "--mission", "alpha", "--taint", "1", "--restore", "abc", "--by", "Wido", "--reason", "r", "--waives", "claim"},
		{"--root", "/nonexistent", "--mission", "alpha", "--taint", "1", "--adopt", "--by", "  ", "--reason", "r", "--waives", "claim"},
		{"--root", "/nonexistent", "--mission", "alpha", "--taint", "1", "--adopt", "--by", "--reason", "--reason", "restored", "--waives", "claim"},
		{"--root", "/nonexistent", "--mission", "alpha", "--taint", "1", "--adopt", "--by", "\uFEFF", "--reason", "r", "--waives", "claim"},
		{"--root", "/nonexistent", "--mission", "alpha", "--taint", "1", "--restore", "not-a-tree", "--by", "Wido", "--reason", "r"},
		{"--root", "/nonexistent", "--mission", "alpha", "--taint", "1", "--restore", "--reason", "--restore", "0123456789012345678901234567890123456789", "--by", "Wido", "--reason", "restored"},
		{"--root", "/nonexistent", "--mission", "alpha", "--taint", "1", "--adopt", "--by", "--adopt", "Wido", "--reason", "r", "--waives", "claim"},
		{"--root", "/nonexistent", "--mission", "alpha", "--taint", "1", "--adopt", "--adopt", "--by", "Wido", "--reason", "r", "--waives", "claim"},
		{"--root", "/nonexistent", "--mission", "alpha", "--taint", "1", "--adopt", "--by", "\u2061", "--reason", "r", "--waives", "claim"},
	}
	for _, args := range refusals {
		if code := runMissionRunnerResolveTaint(args); code != 2 {
			t.Fatalf("args %v must refuse as usage (2), got %d", args, code)
		}
	}
	// Both typed acts PARSE — the engine then refuses on the fake root
	// with its own exit (3), proving the grammar accepted the shape.
	accepted := [][]string{
		{"--root", "/nonexistent", "--mission", "alpha", "--taint", "1", "--restore", "0123456789012345678901234567890123456789", "--by", "Wido", "--reason", "r"},
		{"--root", "/nonexistent", "--mission", "alpha", "--taint", "1", "--adopt", "--by", "Wido", "--reason", "r", "--waives", "claim"},
	}
	for _, args := range accepted {
		if code := runMissionRunnerResolveTaint(args); code == 2 {
			t.Fatalf("args %v must parse past usage, got 2", args)
		}
	}
}
