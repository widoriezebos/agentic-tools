package missionrunner

// The covenant gate: the app's covenant binds the mission contract.
// A contract declaring covenant.path must agree with the covenant it
// names — the gate command, the threshold for the battery's metric,
// and the guardrail net as declared, equal in both directions. An app
// CARRYING a covenant at its canonical location cannot be opted out by
// contract omission. Only an app with no covenant at all passes
// untouched: covenants arrive by inception and retrofit, never by
// ambush. The gate runs at start AND resume, because both repin the
// contract and drift between them would otherwise ride a resume.

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/covenant"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
)

func (e *Engine) covenantPreflight(values map[string]string) error {
	declared := strings.TrimSpace(values["covenant.path"])
	if declared == "" {
		if _, err := os.Lstat(filepath.Join(e.Root, covenant.Filename)); err == nil {
			return failf(3, "mission preflight refused: the app carries %s but the contract declares no covenant.path; a covenant is not optional once it exists", covenant.Filename)
		}
		return nil
	}
	// The covenant has ONE home. The declaration opts a mission in; it
	// never relocates the file — a movable covenant is a selectable one,
	// and a second, weaker document must never shadow the canonical.
	if declared != covenant.Filename {
		return failf(3, "mission preflight refused: covenant.path must be %q — the covenant's one home at the app root — not %q", covenant.Filename, declared)
	}
	path := filepath.Join(e.Root, covenant.Filename)
	// The same non-regular-shape ladder the contract itself gets: a
	// symlink must not select an external covenant, and a FIFO must not
	// hang the blocking read.
	if err := contractShapeRefusal(path); err != nil {
		return failf(3, "mission preflight refused: covenant.path %s: %v", declared, err)
	}
	c, err := covenant.Load(path)
	if err != nil {
		return failf(3, "mission preflight refused: the contract declares a covenant at %s and %v", declared, err)
	}
	if gate := values["gate.command"]; gate != c.Battery.Command {
		return failf(3, "mission preflight refused: the contract's gate %q is not the covenant's battery %q; green must mean what the covenant says", gate, c.Battery.Command)
	}
	// The battery binds WHOLE: the threshold for its metric is part of
	// what green means — a matching command with a weakened threshold is
	// the quietest way to hollow a covenant.
	thresholdKey := "gate.threshold." + c.Battery.Metric
	if got := strings.TrimSpace(values[thresholdKey]); got != c.Battery.Threshold {
		return failf(3, "mission preflight refused: the contract's %s %q is not the covenant's threshold %q; green must mean what the covenant says", thresholdKey, got, c.Battery.Threshold)
	}
	if got := strings.TrimSpace(values["gate.direction"]); got != c.Battery.Direction {
		return failf(3, "mission preflight refused: the contract's gate.direction %q is not the covenant's %q; an inverted direction turns every measurement upside down", got, c.Battery.Direction)
	}
	// The threshold SET binds, not only the battery's own row: an
	// extra gate.threshold.* key measures green by a metric the
	// covenant never declared, which changes what green means just as
	// surely as weakening the declared one.
	for key := range values {
		if strings.HasPrefix(key, "gate.threshold.") && key != thresholdKey {
			return failf(3, "mission preflight refused: the contract carries %s but the covenant's battery declares only the metric %q; an undeclared threshold changes what earns green", key, c.Battery.Metric)
		}
	}
	contractNet, violation := mission.ParseGuardrails(mission.ContractGuardrailSubject, values["wall.guardrails"], protectedArtifactPath)
	if violation != "" {
		return failf(3, "mission preflight refused: %s", violation)
	}
	// The nets are EQUAL as declared, in both directions: the covenant
	// is the net's one home, and the contract custodies all of it — a
	// contract inventing entries hides them from the app, and one
	// omitting entries leaves covenant paths on the ordinary lane.
	covenantEntries := map[string]bool{}
	for _, entry := range c.GuardrailSet.Entries() {
		covenantEntries[entry] = true
	}
	contractEntries := map[string]bool{}
	for _, entry := range contractNet.Entries() {
		contractEntries[entry] = true
		if !covenantEntries[entry] {
			return failf(3, "mission preflight refused: the contract custodies the guardrail %q, which the covenant does not declare; the covenant (%v) is the net's one home", entry, c.GuardrailSet.Entries())
		}
	}
	for entry := range covenantEntries {
		if !contractEntries[entry] {
			return failf(3, "mission preflight refused: the covenant declares the guardrail %q and the contract does not custody it; an omitted entry would ride the ordinary authorization lane", entry)
		}
	}
	return nil
}
