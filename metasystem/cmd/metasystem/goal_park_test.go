package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

// --blocks and --blocked-by belong to goal open alone: every other verb
// refuses them at the flag edge, and open carries both to the verb. Each is
// repeatable and each takes a comma-separated line, because a human naming
// three goals should not have to learn which of the two spellings this
// command prefers.
func TestOnlyGoalOpenTakesBlocks(t *testing.T) {
	if _, ok := parseSyncFlags("park", []string{"--root", t.TempDir(), "--id", "x", "--because", "y", "--blocks", "z"}); ok {
		t.Fatal("goal park accepted --blocks")
	}
	if _, ok := parseSyncFlags("park", []string{"--root", t.TempDir(), "--id", "x", "--because", "y", "--blocked-by", "z"}); ok {
		t.Fatal("goal park accepted --blocked-by")
	}
	f, ok := parseSyncFlags("open", []string{
		"--root", t.TempDir(), "--id", "x", "--blocks", "z,y", "--blocks", "w", "--blocked-by", "a,b",
	})
	if !ok || strings.Join(f.blocks, "|") != "z,y|w" || strings.Join(f.blockedBy, "|") != "a,b" {
		t.Fatalf("goal open carries both directions: ok=%v blocks=%v blockedBy=%v", ok, f.blocks, f.blockedBy)
	}
}

// --blocker belongs to the two edge verbs, and to nothing else.
func TestOnlyBlockAndUnblockTakeTheBlockerFlag(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, name := range []string{"open", "park", "unpark"} {
		if _, ok := parseSyncFlags(name, []string{"--root", root, "--id", "x", "--because", "y", "--blocker", "g"}); ok {
			t.Fatalf("goal %s accepted --blocker", name)
		}
	}
	for _, name := range []string{"block", "unblock"} {
		f, ok := parseSyncFlags(name, []string{"--root", root, "--id", "x", "--blocker", "g"})
		if !ok || f.id != "x" || f.blocker != "g" {
			t.Fatalf("goal %s carries --id and --blocker: ok=%v %+v", name, ok, f)
		}
	}
}

// --under belongs to the attorney verbs (approve, set-budget and unpark,
// the last with --verified); the grant flags to grant.
func TestUnderBelongsToTheAttorneyVerbs(t *testing.T) {
	root := t.TempDir()
	if _, ok := parseSyncFlags("park", []string{"--root", root, "--id", "x", "--because", "y", "--under", "e"}); ok {
		t.Fatal("goal park accepted --under")
	}
	u, ok := parseSyncFlags("unpark", []string{"--root", root, "--id", "x", "--under", "e", "--verified", "the vendor shipped 1.2"})
	if !ok || u.under != "e" || u.verified != "the vendor shipped 1.2" {
		t.Fatalf("goal unpark carries --under and --verified (R-105-m1e): ok=%v %+v", ok, u)
	}
	if _, ok := parseSyncFlags("open", []string{"--root", root, "--id", "x", "--tiers", "1"}); ok {
		t.Fatal("goal open accepted --tiers")
	}
	f, ok := parseSyncFlags("set-budget", []string{"--root", root, "--id", "x", "--under", "e"})
	if !ok || f.under != "e" {
		t.Fatalf("goal set-budget carries --under: ok=%v under=%q", ok, f.under)
	}
	g, ok := parseSyncFlags("grant", []string{"--root", root, "--by", "Wido", "--tiers", "1", "--verbs", "approve", "--expires", "2026-09-19"})
	if !ok || g.tiers != "1" || g.verbs != "approve" || g.expires != "2026-09-19" {
		t.Fatalf("goal grant carries its flags: ok=%v %+v", ok, g)
	}
}

// goal tier-probe registers its flags and refuses a root with no ledger
// without panicking.
func TestGoalTierProbeFlagRegistrationDoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("goal tier-probe panicked: %v", r)
		}
	}()
	if code := runGoalTierProbe([]string{"--root", t.TempDir(), "--pretty"}); code == 0 {
		t.Fatal("a root with no ledger is not a probe result")
	}
}

func TestGoalLandReadyIsRegisteredAsASyncOnlyVerb(t *testing.T) {
	if runGoalLandReady == nil {
		t.Fatal("goal land-ready is not wired")
	}
	f, ok := parseSyncFlags("land-ready", []string{"--root", t.TempDir(), "--id", "built"})
	if !ok || f.id != "built" {
		t.Fatalf("goal land-ready carries --id: ok=%v id=%q", ok, f.id)
	}
	if _, ok := parseSyncFlags("land-ready", []string{"--root", t.TempDir(), "--id", "built", "--by", "Wido"}); !ok {
		// --by parses on every sync verb; the verb itself refuses a human actor.
		t.Fatal("parse of --by failed at the flag edge")
	}
}

// job cap-continuation refuses without its flags, refuses positional
// arguments, and refuses a parent that is not a capped implementer round.
func TestJobCapContinuationFlagsAndRefusals(t *testing.T) {
	dir := t.TempDir()
	if code := runDispatchCapContinuation([]string{"--root", dir}); code != 2 {
		t.Fatalf("missing flags exit = %d, want 2", code)
	}
	if code := runDispatchCapContinuation([]string{"--root", dir, "--parent", "p.json", "--worktree", dir, "--output", "o.md", "stray"}); code != 2 {
		t.Fatalf("a positional argument was accepted: exit %d", code)
	}
	parent := filepath.Join(dir, "parent.json")
	if err := os.WriteFile(parent, []byte(`{"jobId":"chain","role":"implementer","round":1,"status":"completed","error":null,"capMin":120}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := runDispatchCapContinuation([]string{"--root", dir, "--parent", "parent.json", "--worktree", dir, "--output", "out.md"}); code != 1 {
		t.Fatalf("a completed parent was accepted: exit %d", code)
	}
}
