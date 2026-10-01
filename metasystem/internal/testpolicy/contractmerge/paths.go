package contractmerge

import (
	"fmt"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// AddInputs adds paths to a group's inputs, after the ones it lists; a path
// the group already lists is already added.
func AddInputs(contract testpolicy.Contract, groupID string, paths []string) (testpolicy.Contract, error) {
	return editGroupInputs(contract, groupID, paths, addHelp, func(inputs, paths []string) []string { return appendUnique(inputs, paths...) })
}

// RemoveInputs takes paths out of a group's inputs, the follow-up of deleting
// or moving a file; a path the group no longer lists is already removed.
func RemoveInputs(contract testpolicy.Contract, groupID string, paths []string) (testpolicy.Contract, error) {
	return editGroupInputs(contract, groupID, paths, removeHelp, without)
}

// AddSurfacePaths adds paths to a surface, after the ones it lists; a path
// the surface already lists is already added.
func AddSurfacePaths(contract testpolicy.Contract, surfaceID string, paths []string) (testpolicy.Contract, error) {
	return editSurfacePaths(contract, surfaceID, paths, addHelp, func(current, paths []string) []string { return appendUnique(current, paths...) })
}

// RemoveSurfacePaths takes paths out of a surface; a path the surface no
// longer lists is already removed.
func RemoveSurfacePaths(contract testpolicy.Contract, surfaceID string, paths []string) (testpolicy.Contract, error) {
	return editSurfacePaths(contract, surfaceID, paths, removeHelp, without)
}

// RemoveSurface takes a whole surface out of the contract, with every
// dependsOn naming it. A surface the contract no longer has is already
// removed.
func RemoveSurface(contract testpolicy.Contract, surfaceID string) (testpolicy.Contract, error) {
	surfaces := make([]testpolicy.Surface, 0, len(contract.Surfaces))
	for _, surface := range contract.Surfaces {
		if surface.ID == surfaceID {
			continue
		}
		surface.DependsOn = without(surface.DependsOn, []string{surfaceID})
		surfaces = append(surfaces, surface)
	}
	contract.Surfaces = surfaces
	if err := contract.Validate(); err != nil {
		return testpolicy.Contract{}, invalid(err.Error())
	}
	return contract, nil
}

func editGroupInputs(contract testpolicy.Contract, groupID string, paths []string, help string, edit func([]string, []string) []string) (testpolicy.Contract, error) {
	entity := fmt.Sprintf("group %q", groupID)
	clean, err := cleanPaths(entity, "inputs", paths, help)
	if err != nil {
		return testpolicy.Contract{}, err
	}
	for i := range contract.Groups {
		if contract.Groups[i].ID != groupID {
			continue
		}
		contract.Groups[i].Inputs = edit(append([]string{}, contract.Groups[i].Inputs...), clean)
		if err := contract.Validate(); err != nil {
			return testpolicy.Contract{}, invalid(err.Error())
		}
		return contract, nil
	}
	return testpolicy.Contract{}, unknownGroup(entity)
}

func editSurfacePaths(contract testpolicy.Contract, surfaceID string, paths []string, help string, edit func([]string, []string) []string) (testpolicy.Contract, error) {
	entity := fmt.Sprintf("surface %q", surfaceID)
	clean, err := cleanPaths(entity, "paths", paths, help)
	if err != nil {
		return testpolicy.Contract{}, err
	}
	for i := range contract.Surfaces {
		if contract.Surfaces[i].ID != surfaceID {
			continue
		}
		contract.Surfaces[i].Paths = edit(append([]string{}, contract.Surfaces[i].Paths...), clean)
		if err := contract.Validate(); err != nil {
			return testpolicy.Contract{}, invalid(err.Error())
		}
		return contract, nil
	}
	return testpolicy.Contract{}, addTestsRefusal(entity, "id", "the testing contract has no such surface").withRun(help)
}

func cleanPaths(entity, field string, paths []string, help string) ([]string, error) {
	var clean []string
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			return nil, addTestsRefusal(entity, field, "name each path once, with commas between paths").withRun(help)
		}
		clean = append(clean, path)
	}
	if len(clean) == 0 {
		return nil, addTestsRefusal(entity, field, "name at least one path").withRun(help)
	}
	return clean, nil
}

func without(values, drop []string) []string {
	dropped := map[string]bool{}
	for _, value := range drop {
		dropped[value] = true
	}
	kept := make([]string, 0, len(values))
	for _, value := range values {
		if !dropped[value] {
			kept = append(kept, value)
		}
	}
	return kept
}
