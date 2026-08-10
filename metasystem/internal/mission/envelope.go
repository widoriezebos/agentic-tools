package mission

import (
	"fmt"
	"path/filepath"
)

// DispatchEnvelopeAllows reports whether a mission's signed contract carries
// the exact runtime:model pair in its envelope.dispatch-allow — the signed
// grant that lets a dispatch escalate past the roster without a TTY approval.
// The contract must be sealed, its approval must cover the sealed bytes, and
// the seal must be on the fetched origin; an unsealed or unverifiable
// contract allows nothing.
func DispatchEnvelopeAllows(projectRoot, missionID, requestedPair string) error {
	path := filepath.Join(projectRoot, "plans", fmt.Sprintf("mission-%s.contract.md", missionID))
	doc, err := contractRead(path)
	if err != nil {
		return fmt.Errorf("signed dispatch envelope unavailable: %v", err)
	}
	if len(doc.sealed) == 0 {
		return fmt.Errorf("signed dispatch envelope unavailable: dispatch-allow envelope is not sealed")
	}
	if err := doc.verifyApproval(); err != nil {
		return fmt.Errorf("signed dispatch envelope unavailable: %v", err)
	}
	repo, err := contractRepositoryFor(path)
	if err != nil {
		return fmt.Errorf("signed dispatch envelope unavailable: %v", err)
	}
	if err := doc.verifyOrigin(repo); err != nil {
		return fmt.Errorf("signed dispatch envelope unavailable: %v", err)
	}
	value, present := doc.values["envelope.dispatch-allow"]
	if !present {
		return fmt.Errorf("requested pair %s is not in the signed dispatch envelope", requestedPair)
	}
	allowed, err := contractValidateDispatchAllow(value)
	if err != nil {
		return fmt.Errorf("signed dispatch envelope unavailable: %v", err)
	}
	for _, pair := range allowed {
		if pair == requestedPair {
			return nil
		}
	}
	return fmt.Errorf("requested pair %s is not in the signed dispatch envelope", requestedPair)
}
