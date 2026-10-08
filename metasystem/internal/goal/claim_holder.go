package goal

import "fmt"

// ClaimHolderFacts is a fresh lease and process observation, read at the
// publication boundary. A recorded lease alone does not prove a live holder.
type ClaimHolderFacts struct {
	Lineage  string
	Epoch    int64
	LiveMain bool
}

func authenticateClaimHolder(r VerbRequest) error {
	if r.Endpoint.ClaimHolder == nil {
		return coded("REBIND_EPOCH_UNAUTHENTICATED", fmt.Errorf("the live checkout holder cannot be authenticated; run metasystem session start"))
	}
	holder, err := r.Endpoint.ClaimHolder(r.Endpoint.Root)
	if err != nil {
		return coded("REBIND_EPOCH_UNAUTHENTICATED", fmt.Errorf("the checkout holder cannot be authenticated: %w", err))
	}
	if !holder.LiveMain || holder.Lineage != r.Actor.Lineage || holder.Epoch != r.ClaimEpoch || holder.Epoch < 1 {
		return coded("REBIND_EPOCH_UNAUTHENTICATED", fmt.Errorf("the live checkout holder no longer matches this claim renewal; run metasystem session start"))
	}
	return nil
}
