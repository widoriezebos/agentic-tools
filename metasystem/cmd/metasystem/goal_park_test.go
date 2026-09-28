package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
