package testpolicy

// HoldingSurfaces names the surfaces whose paths hold any of the files.
func HoldingSurfaces(contract Contract, files []string) []string {
	holding := map[string]bool{}
	for _, file := range files {
		matched := false
		for _, surface := range contract.Surfaces {
			for _, pattern := range surface.Paths {
				if matchesPath(pattern, file) {
					holding[surface.ID], matched = true, true
				}
			}
		}
		if !matched && contract.Fallback != "" {
			holding[contract.Fallback] = true
		}
	}
	return keys(holding)
}
