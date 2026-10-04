package testpolicy

import (
	"fmt"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/pathpattern"
)

// AffectedGroup pairs a concrete group with the changed paths its inputs cover.
type AffectedGroup struct {
	Group Group
	Paths []string
}

// AffectedResult separates concrete groups, uncovered paths, and paths covered
// by package selector templates. Groups retain contract order; paths retain
// their supplied order, with overlapping inputs contributing a path only once.
type AffectedResult struct {
	Groups          []AffectedGroup
	Uncovered       []string
	TemplateCovered []string
}

// Affected matches repository-relative paths against declared inputs without
// reading files. Templates cover paths but are never concrete groups to run.
// Invalid inputs return an error rather than a partial selection.
func Affected(contract Contract, paths []string) (AffectedResult, error) {
	var result AffectedResult
	if len(paths) == 0 {
		return result, nil
	}
	covered := make([]bool, len(paths))
	templateCovered := make([]bool, len(paths))
	for _, group := range contract.Groups {
		patterns := make([]pathpattern.Pattern, 0, len(group.Inputs))
		for _, input := range group.Inputs {
			pattern, err := pathpattern.Parse(input)
			if err != nil {
				return AffectedResult{}, fmt.Errorf("testing group %s input %q: %w", group.ID, input, err)
			}
			patterns = append(patterns, pattern)
		}
		var reasons []string
		for i, path := range paths {
			for _, pattern := range patterns {
				if !pattern.Covers(path) {
					continue
				}
				covered[i] = true
				if group.PackageSelection != "" {
					templateCovered[i] = true
				} else {
					reasons = append(reasons, path)
				}
				break
			}
		}
		if len(reasons) != 0 {
			result.Groups = append(result.Groups, AffectedGroup{Group: group, Paths: reasons})
		}
	}
	for i, path := range paths {
		if !covered[i] {
			result.Uncovered = append(result.Uncovered, path)
		}
		if templateCovered[i] {
			result.TemplateCovered = append(result.TemplateCovered, path)
		}
	}
	return result, nil
}
