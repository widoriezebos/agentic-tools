package testpolicy

import "fmt"

// DeepOnlySectionGroups returns section-adapter groups that no canary or
// standard selection can reach. It asks the selection policy for every
// surface so dependency consumers, critical providers, cross-cutting
// providers, the fallback surface, and the always lists keep one owner.
func DeepOnlySectionGroups(contract Contract) ([]Group, error) {
	standard := map[string]bool{}
	for _, surface := range contract.Surfaces {
		changedPath, err := deepOnlyProbePath(contract, surface)
		if err != nil {
			return nil, err
		}
		plan, err := Select(contract, SelectionRequest{
			ChangedPaths:  []string{changedPath},
			GoalRisk:      GoalRisk{Severity: 3, Novelty: 3, Exposure: 3, Accumulation: 3},
			RequestedMode: ModeStandard,
			Purpose:       PurposeDiagnostic,
		})
		if err != nil {
			return nil, fmt.Errorf("select standard groups for surface %s: %w", surface.ID, err)
		}
		addAll(standard, plan.SelectedGroups)
	}

	var result []Group
	for _, group := range contract.Groups {
		if group.Adapter == "section" && !standard[group.ID] {
			result = append(result, group)
		}
	}
	return result, nil
}

// DeepOnlySectionGroupIDs is the stable identifier view used by cadence
// orchestration and status reporting.
func DeepOnlySectionGroupIDs(contract Contract) ([]string, error) {
	groups, err := DeepOnlySectionGroups(contract)
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(groups))
	for index, group := range groups {
		ids[index] = group.ID
	}
	return ids, nil
}

func deepOnlyProbePath(contract Contract, surface Surface) (string, error) {
	if surface.ID != contract.Fallback {
		if len(surface.Paths) == 0 {
			return "", fmt.Errorf("testing surface %s has no path to select", surface.ID)
		}
		path := surface.Paths[0]
		if len(path) >= 3 && path[len(path)-3:] == "/**" {
			return path[:len(path)-2] + "deep-only-inventory-probe", nil
		}
		return path, nil
	}

	path := "deep-only-inventory-probe/unowned"
	for {
		matched := false
		for _, candidate := range contract.Surfaces {
			if candidate.ID == contract.Fallback {
				continue
			}
			for _, pattern := range candidate.Paths {
				if matchesPath(pattern, path) {
					matched = true
					break
				}
			}
			if matched {
				break
			}
		}
		if !matched {
			return path, nil
		}
		path = "x/" + path
	}
}
