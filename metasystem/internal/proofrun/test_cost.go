package proofrun

import "fmt"

var requiredCostScenarios = map[string][]string{
	"dispatcher": {"adapter-selftest", "steward-continuation"},
	"adoption":   {"filled-target-delivery", "copied-registration-setup", "copied-registration-positive", "copy-drift-source", "copy-drift-registration"},
}

var fixtureScenarioCatalog = map[string][]string{
	"dispatcher": {"dispatch", "mission-runner", "adapter-selftest", "steward-continuation", "brain-delegate-refuses", "brain-cancel-close-reap-refuse", "brain-breach-stop-exempt", "brain-absent-node-proceeds", "brain-fence-helper-fails", "seat-refused"},
}

// FixtureScenarios returns the ordinary complete scenario catalog or the
// accepted recurring-cost selection. The Go owner keeps comparison calls from
// silently dropping or adding a scenario while the shell remains the real
// process-owning parent.
func FixtureScenarios(family, selection string) ([]string, error) {
	var selected []string
	switch selection {
	case "all":
		selected = fixtureScenarioCatalog[family]
	case "comparison":
		selected = requiredCostScenarios[family]
	default:
		return nil, fmt.Errorf("unknown fixture selection %q", selection)
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("unknown fixture family %q", family)
	}
	return append([]string(nil), selected...), nil
}
