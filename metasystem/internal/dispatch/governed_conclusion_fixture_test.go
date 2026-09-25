package dispatch

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// templateAdmissionBed gives state-root discovery a real template layout.
func templateAdmissionBed(t *testing.T, bed *goalAdmissionBed) {
	t.Helper()
	parent := filepath.Dir(bed.root)
	marker := filepath.Join(parent, "development", "metasystem-design.md")
	if err := os.MkdirAll(filepath.Dir(marker), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(marker, []byte("# template\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(parent, "metasystem")
	if err := os.Rename(bed.root, root); err != nil {
		t.Fatal(err)
	}
	bed.root = root
	bed.reads.NewWorld = func(got string) bool {
		if got != root {
			t.Fatalf("goal world root = %q, want %q", got, root)
		}
		return true
	}
	bed.reads.ResolveEndpoint = func(got string) (goal.Endpoint, error) {
		if got != root {
			return goal.Endpoint{}, fmt.Errorf("goal endpoint root = %q, want %q", got, root)
		}
		return goal.Endpoint{Root: root, Remote: "local", Branch: goal.LocalLedgerBranch, Repository: bed.repository}, nil
	}
	bed.reads.ResolveMachine = func(got string) (string, error) {
		if got != root {
			return "", fmt.Errorf("goal machine root = %q, want %q", got, root)
		}
		return "bed-m1", nil
	}
}

// countGoalAdmissionFacts keeps each fixture's repository reads tied to its own root.
func countGoalAdmissionFacts(t *testing.T, bed *goalAdmissionBed, wantWorld, wantEndpoint, wantMachine int) {
	t.Helper()
	original := bed.reads
	var world, endpoint, machine int
	bed.reads.NewWorld = func(root string) bool {
		world++
		if root != bed.root || world > wantWorld {
			t.Fatalf("unexpected goal world read root=%q count=%d, want root=%q count=%d", root, world, bed.root, wantWorld)
		}
		return original.NewWorld(root)
	}
	bed.reads.ResolveEndpoint = func(root string) (result goal.Endpoint, err error) {
		endpoint++
		if root != bed.root || endpoint > wantEndpoint {
			t.Fatalf("unexpected goal endpoint read root=%q count=%d, want root=%q count=%d", root, endpoint, bed.root, wantEndpoint)
		}
		return original.ResolveEndpoint(root)
	}
	bed.reads.ResolveMachine = func(root string) (machineName string, err error) {
		machine++
		if root != bed.root || machine > wantMachine {
			t.Fatalf("unexpected goal machine read root=%q count=%d, want root=%q count=%d", root, machine, bed.root, wantMachine)
		}
		return original.ResolveMachine(root)
	}
	t.Cleanup(func() {
		if world != wantWorld || endpoint != wantEndpoint || machine != wantMachine {
			t.Errorf("goal fact calls world/endpoint/machine=%d/%d/%d, want %d/%d/%d", world, endpoint, machine, wantWorld, wantEndpoint, wantMachine)
		}
	})
}
