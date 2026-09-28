package steward

import (
	"path/filepath"
	"testing"
	"time"
)

// EM-07: the context budget of a root that does not exist read "alive (no
// announced holder: checkout lease is absent at ...)" with no error, so the
// status command exited 0 for a place with no installation. It is unknown,
// with the error naming the root.
func TestContextBudgetOfAMissingRootIsAnError(t *testing.T) {
	t.Parallel()
	missing := filepath.Join(t.TempDir(), "nonexistent")
	role, _, err := ContextBudgetLine(missing, missing, time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC), ContextOptions{})
	want := missing + " does not exist, so there is no installation to read; nothing was read"
	if err == nil || err.Error() != want {
		t.Fatalf("missing root err = %v, want %q", err, want)
	}
	if role.Status != HealthUnknown || role.Reason != want || role.Remedy != "" {
		t.Fatalf("missing root role = %+v", role)
	}
}
