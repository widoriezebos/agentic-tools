package main

import "testing"

// goal park's flag setup once registered and-none TWICE (a stray
// f.Bool beside boolAsString) and panicked on every invocation — the
// verb's first real use, parking runtime-install-execution, found it.
// Any non-panicking return proves the registration is sane.
func TestGoalParkFlagRegistrationDoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("goal park panicked at flag registration: %v", r)
		}
	}()
	root := t.TempDir()
	_ = runGoalPark([]string{"--root", root, "--id", "nope", "--because", "x"})
}

// --blocks belongs to goal open alone: every other verb refuses it at the
// flag edge, and open carries it to the verb.
func TestOnlyGoalOpenTakesBlocks(t *testing.T) {
	if _, ok := parseSyncFlags("park", []string{"--root", t.TempDir(), "--id", "x", "--because", "y", "--blocks", "z"}); ok {
		t.Fatal("goal park accepted --blocks")
	}
	f, ok := parseSyncFlags("open", []string{"--root", t.TempDir(), "--id", "x", "--blocks", "z"})
	if !ok || f.blocks != "z" {
		t.Fatalf("goal open carries --blocks: ok=%v blocks=%q", ok, f.blocks)
	}
}
