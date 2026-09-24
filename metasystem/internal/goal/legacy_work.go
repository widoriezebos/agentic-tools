package goal

import "github.com/widoriezebos/agentic-tools/metasystem/internal/identity"

// ReadLegacyClaimableWork reads legacy goal work after the caller has selected
// the legacy world. It retains the ledger and process liveness checks.
func ReadLegacyClaimableWork(root string, prober identity.Prober) (ClaimableBudgetedWork, error) {
	return readLegacyClaimableWork(root, prober)
}
