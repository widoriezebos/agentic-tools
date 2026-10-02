package delegation

import (
	"bytes"
	"testing"
)

// Design review is allowed from tier 2 (Wido 2026-10-01); tier 1 refuses
// every critic role.
func TestGoalTierLadderAllowsDesignCriticFromTierTwo(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		tier  uint8
		role  string
		allow bool
	}{
		{1, "design-critic", false},
		{1, "code-critic", false},
		{1, "implementer", true},
		{2, "design-critic", true},
		{2, "code-critic", true},
		{3, "design-critic", true},
	} {
		s := &session{stderr: &bytes.Buffer{}}
		err := s.requireGoalTierLadder(&subject{goal: "g", goalTier: tc.tier, role: tc.role})
		if (err == nil) != tc.allow {
			t.Fatalf("tier %d role %s: err=%v; want allowed=%v", tc.tier, tc.role, err, tc.allow)
		}
	}
}
