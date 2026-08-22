package validate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlanConsistencyAcceptsExplainedMentions(t *testing.T) {
	plans := t.TempDir()
	writeFile(t, filepath.Join(plans, "a.md"), "RETIRED: frobnicate -- the spindle pass\n")
	writeFile(t, filepath.Join(plans, "b.md"), "frobnicate was replaced by the spindle pass.\n")
	retired, violations, err := PlanConsistency(plans)
	if err != nil {
		t.Fatal(err)
	}
	if retired != 1 {
		t.Fatalf("retired = %d, want 1", retired)
	}
	if len(violations) != 0 {
		t.Fatalf("explained mention flagged: %v", violations)
	}
}

func TestPlanConsistencyRejectsPrescribedRetiredTerm(t *testing.T) {
	plans := t.TempDir()
	writeFile(t, filepath.Join(plans, "a.md"), "RETIRED: frobnicate -- the spindle pass\n")
	writeFile(t, filepath.Join(plans, "b.md"), "intro\nAlways Frobnicate the widget first.\n")
	_, violations, err := PlanConsistency(plans)
	if err != nil {
		t.Fatal(err)
	}
	want := "b.md:2: prescribes 'frobnicate', retired in a.md in favour of the spindle pass"
	if len(violations) != 1 || violations[0] != want {
		t.Fatalf("violations = %v, want [%s]", violations, want)
	}
}

func TestPlanConsistencyMissingDirectoryErrors(t *testing.T) {
	if _, _, err := PlanConsistency(filepath.Join(t.TempDir(), "absent")); err == nil {
		t.Fatal("a missing plans directory must error")
	}
}

// Gone is fine, unreadable is not — a skipped plan may
// hold the very declaration or violation this gate exists to catch.
func TestPlanConsistencyRefusesUnreadablePlan(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission bits cannot bite as root")
	}
	plans := t.TempDir()
	writeFile(t, filepath.Join(plans, "a.md"), "RETIRED: frobnicate -- gone\n")
	writeFile(t, filepath.Join(plans, "b.md"), "clean\n")
	if err := os.Chmod(filepath.Join(plans, "b.md"), 0o000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(filepath.Join(plans, "b.md"), 0o644)
	_, _, err := PlanConsistency(plans)
	if err == nil || !strings.Contains(err.Error(), "plan file unreadable") {
		t.Fatalf("an unreadable plan must refuse the gate: %v", err)
	}
}
