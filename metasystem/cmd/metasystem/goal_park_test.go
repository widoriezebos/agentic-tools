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

// --under belongs to approve and set-budget; the grant flags to grant.
func TestOnlyApproveAndSetBudgetTakeUnder(t *testing.T) {
	root := t.TempDir()
	if _, ok := parseSyncFlags("unpark", []string{"--root", root, "--id", "x", "--under", "e"}); ok {
		t.Fatal("goal unpark accepted --under")
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
