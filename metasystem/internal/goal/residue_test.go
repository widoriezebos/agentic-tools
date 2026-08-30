package goal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The R-4 gate runs BEFORE any transaction: a residue-naming conclusion
// without a resolvable goal link refuses at the door.
func TestDoneRefusesUnscheduledResidueInTheConclusion(t *testing.T) {
	root := t.TempDir()
	request := VerbRequest{Endpoint: Endpoint{Root: root}}

	_, err := Done(request, "any-goal", "landed X; the cache half is residue for later")
	if err == nil || !strings.Contains(err.Error(), "residue is a scheduled debt") {
		t.Fatalf("unlinked residue conclusion must refuse by name: %v", err)
	}

	_, err = Done(request, "any-goal", "landed X; residual cache work rides goal:ghost-item")
	if err == nil || !strings.Contains(err.Error(), "goal:ghost-item does not resolve") {
		t.Fatalf("a dangling residue link must refuse by id: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(root, "plans", "goals"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "plans", "goals", "cache-half.md"), []byte("# goal\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = Done(request, "any-goal", "landed X; the cache residue rides goal:cache-half")
	if err != nil && strings.Contains(err.Error(), "residue") {
		t.Fatalf("a linked, resolvable residue must pass the gate: %v", err)
	}

	_, err = Done(request, "any-goal", "landed X cleanly; nothing remains")
	if err != nil && strings.Contains(err.Error(), "residue") {
		t.Fatalf("a residue-free conclusion must not trip the gate: %v", err)
	}
}
