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
		return errors.New("goal done --force works only while you hold the helm: metasystem helm take --reason TEXT")
	}
	if proof == nil || proof.Helm != nil || proof.FixtureOnly || !proof.TerminalValidFor(root) {
		return errors.New("goal done --force is your own act, in the terminal you took the helm in; this shell isn't it")
	}
	return nil
}
