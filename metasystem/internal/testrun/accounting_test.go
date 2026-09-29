package testrun

import (
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// TestCadencePreflightAccountsToNoGoal: the cadence preflight plans the
// fetched tree before the tick claims standing authority, so it accounts to
// no goal; a preflight of any other purpose is refused.
func TestCadencePreflightAccountsToNoGoal(t *testing.T) {
	t.Parallel()
	request := SelectionRequest{Root: "landing", Tree: "tree", Mode: testpolicy.ModeDeep, Purpose: testpolicy.PurposeCadence, CadencePreflight: true}
	if accounts, err := accountsToGoal(request); err != nil || accounts {
		t.Fatalf("cadence preflight accounts-to-goal=%t err=%v", accounts, err)
	}
	request.Purpose = testpolicy.PurposeDelivery
	if _, err := accountsToGoal(request); err == nil {
		t.Fatal("a delivery preflight was accepted as cadence")
	}
}
