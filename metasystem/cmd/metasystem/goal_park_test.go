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
