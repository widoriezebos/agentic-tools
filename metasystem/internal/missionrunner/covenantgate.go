package missionrunner

// The covenant gate: when the sealed contract declares covenant.path,
// preflight loads the app's covenant and refuses any disagreement —
// the contract's gate must BE the covenant's battery, and every
// guardrail the contract custodies must be one the covenant declares.
// A contract with no declaration passes untouched: covenants arrive by
// inception and retrofit, never by ambush, so a covenant-less app
// stays lawful until its covenant exists.

import (
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/covenant"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
)

func (e *Engine) covenantPreflight(values map[string]string) error {
	declared := strings.TrimSpace(values["covenant.path"])
	if declared == "" {
		return nil
	}
	if strings.HasPrefix(declared, "/") || strings.Contains(declared, "..") {
		return failf(3, "mission preflight refused: covenant.path %q must be a repository-relative file", declared)
	}
	c, err := covenant.Load(filepath.Join(e.Root, filepath.FromSlash(declared)))
	if err != nil {
		return failf(3, "mission preflight refused: the contract declares a covenant at %s and %v", declared, err)
	}
	if gate := values["gate.command"]; gate != c.Battery.Command {
		return failf(3, "mission preflight refused: the contract's gate %q is not the covenant's battery %q; green must mean what the covenant says", gate, c.Battery.Command)
	}
	contractNet, violation := mission.ParseGuardrails(values["wall.guardrails"], protectedArtifactPath)
	if violation != "" {
		return failf(3, "mission preflight refused: %s", violation)
	}
	covenantEntries := map[string]bool{}
	for _, entry := range c.GuardrailSet.Entries() {
		covenantEntries[entry] = true
	}
	for _, entry := range contractNet.Entries() {
		if !covenantEntries[entry] {
			return failf(3, "mission preflight refused: the contract custodies the guardrail %q, which the covenant does not declare; the covenant (%v) is the net's one home", entry, c.GuardrailSet.Entries())
		}
	}
	return nil
}
