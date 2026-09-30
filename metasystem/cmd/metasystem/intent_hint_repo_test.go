package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// A hint's --repo naming the checkout the person already stands in is noise
// on the page: the text drops it, and --json keeps the argv whole.
func TestHintDropsARepoTheCommandRunsIn(t *testing.T) {
	t.Parallel()
	checkout := t.TempDir()
	inside := filepath.Join(checkout, "metasystem")
	if err := os.MkdirAll(inside, 0o755); err != nil {
		t.Fatal(err)
	}
	other := t.TempDir()
	inv := &intentInvocation{cwd: inside, layout: stateroot.Layout{GitRoot: checkout}}
	next := &intentNext{Argv: []string{"metasystem", "ui", "status", "--repo", checkout}, Reason: "what that interface is doing now"}
	if hint := inv.hintFor(next); !slices.Equal(hint.Argv, []string{"metasystem", "ui", "status"}) || hint.Reason != next.Reason {
		t.Fatalf("inside the checkout the hint is %v", hint)
	}
	if !slices.Contains(next.Argv, "--repo") {
		t.Fatal("the result's own argv, which --json prints, keeps --repo")
	}
	elsewhere := &intentNext{Argv: []string{"metasystem", "ui", "status", "--repo", other}}
	if hint := inv.hintFor(elsewhere); !slices.Equal(hint.Argv, elsewhere.Argv) {
		t.Fatalf("another checkout's --repo stays: %v", hint)
	}
	outside := &intentInvocation{cwd: other, layout: stateroot.Layout{GitRoot: checkout}}
	if hint := outside.hintFor(next); !slices.Equal(hint.Argv, next.Argv) {
		t.Fatalf("run from outside, the --repo stays: %v", hint)
	}
}
