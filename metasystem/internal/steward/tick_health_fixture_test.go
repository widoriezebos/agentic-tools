package steward

import (
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// tickHealthRoles keeps unrelated role observations stable while the spend
// check produces its own role and typed observation from the supplied ledger.
func tickHealthRoles(t *testing.T, root, machine string, measure spendMeasureFunc) healthRoleEvaluator {
	t.Helper()
	resolveMachine := func(got string) (string, error) {
		t.Helper()
		if got != root {
			t.Fatalf("spend machine lookup root = %q, want %q", got, root)
		}
		return machine, nil
	}
	return func(repoRoot, _ string, now time.Time, _ identity.Prober, _ bool) ([]RoleVerdict, SpendObservation) {
		roles := make([]RoleVerdict, 0, len(healthRoleOrder))
		var spendObservation SpendObservation
		for _, role := range healthRoleOrder {
			if role == RoleSpendFence {
				spendRole, observation := checkSpendFenceWithMeasureAndMachine(repoRoot, now, measure, resolveMachine)
				roles = append(roles, spendRole)
				spendObservation = observation
				continue
			}
			roles = append(roles, roleAlive(role, "the fixture observed this role as stable"))
		}
		return roles, spendObservation
	}
}
