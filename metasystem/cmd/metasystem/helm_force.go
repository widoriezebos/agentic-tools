package main

import (
	"errors"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
)

// forceAdmission decides goal done --force: only at the helm, and only from
// the person's own proof at the enrolled terminal. The proof kind decides:
// the helm's admission of the agent beside the person, a fixture proof or a
// session lineage (no proof) never force.
func forceAdmission(state helm.State, proof *humanauthority.Proof, root string) error {
	if !state.Active {
		return errors.New("goal done --force works only at the helm: take it first (metasystem helm take --reason TEXT), or close what blocks the conclusion")
	}
	if proof == nil || proof.Helm != nil || proof.FixtureOnly || !proof.TerminalValidFor(root) {
		why := "no person proof"
		if proof != nil && proof.Helm != nil {
			why = "admitted by the helm"
		}
		return errors.New("goal done --force is the person's own act: this shell was not proven at the enrolled terminal (" + why + "); run it yourself at the terminal you took the helm at (a terminal is enrolled once with " + humanauthority.EnrollCommand + ")")
	}
	return nil
}
