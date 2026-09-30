package main

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/ownercall"
)

// TestGoalSyncOwnersRunInThisProcess is the U9a witness that goal sync's
// publish, refresh and upgrade reach the goal owners in this process (design
// 6.2): the bed fails any engine child, every owner call supplies this
// process as its caller, and the owner's own report is the result.
func TestGoalSyncOwnersRunInThisProcess(t *testing.T) {
	t.Parallel()
	b := newDeliveryBed(t)
	calls := defaultIntentOwnerCalls()
	var supplied []ownercall.Process
	var reached [][]string
	realReconcile := calls.goalReconcile
	calls.goalReconcile = func(dependencies syncRequestDependencies, stdout, stderr io.Writer, dir string, args []string) int {
		supplied = append(supplied, dependencies.authorityFacts.caller)
		reached = append(reached, append([]string{"reconcile"}, args...))
		if slices.Contains(args, "--refresh-only") {
			return realReconcile(dependencies, stdout, stderr, dir, args)
		}
		// The bed proves no person; the named human's proof is the owner's
		// own business, witnessed at the owner.
		io.WriteString(stderr, "goal reconcile: the bed proves no person\n")
		return 1
	}
	calls.goalMigrate = func(dependencies syncRequestDependencies, stdout, stderr io.Writer, dir string, args []string) int {
		supplied = append(supplied, dependencies.authorityFacts.caller)
		reached = append(reached, append([]string{"migrate"}, args...))
		io.WriteString(stderr, "goal migrate: the bed proves no person\n")
		return 1
	}
	b.owners.calls = calls

	// The bed names no lineage and no person: the owner's own refusal comes
	// back as the result, not on this process's standard error.
	if code, result := b.do("goal", "sync", "--refresh"); code == 0 || !strings.Contains(result.Summary, "no session is named") {
		t.Fatalf("goal sync --refresh = %d %+v", code, result)
	}
	if _, result := b.do("goal", "sync", "--publish", "--goal", "g", "--by", "Wido"); result.Summary != "goal reconcile: the bed proves no person" {
		t.Fatalf("goal sync --publish = %+v", result)
	}
	legacy := filepath.Join(b.root(), "plans", "goals.md")
	source := []byte("# Goals\n")
	if err := os.WriteFile(legacy, source, 0o644); err != nil {
		t.Fatal(err)
	}
	if code, result := b.do("goal", "sync", "--upgrade", "--by", "Wido", "--source-digest", goal.SourceDigestOf(source)); code == 0 || result.Outcome != intentRefused {
		t.Fatalf("goal sync --upgrade without a person's proof = %d %+v", code, result)
	}
	if want := [][]string{{"reconcile", "--root", b.root(), "--refresh-only"}, {"reconcile", "--root", b.root(), "--by", "Wido", "--id", "g"},
		{"migrate", "--root", b.root(), "--source-digest", goal.SourceDigestOf(source), "--by", "Wido"}}; !reflect.DeepEqual(reached, want) {
		t.Fatalf("owner argv = %q, want %q", reached, want)
	}
	if len(b.calls) != 0 {
		t.Fatalf("an engine child ran: %v", b.calls)
	}
	if len(supplied) != 3 {
		t.Fatalf("owner calls = %d, want refresh, publish and upgrade", len(supplied))
	}
	for _, caller := range supplied {
		if caller.Pid != int64(os.Getpid()) {
			t.Fatalf("an owner call supplied %+v, want this process %d", caller, os.Getpid())
		}
	}
}
