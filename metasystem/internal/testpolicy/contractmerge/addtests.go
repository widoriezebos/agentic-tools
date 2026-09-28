package contractmerge

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// AddTests adds named Go tests after proving that each name is declared by a
// top-level function in at least one of the group's package directories.
func AddTests(contract testpolicy.Contract, contractPath, groupID string, additions []string) (testpolicy.Contract, error) {
	groupIndex := -1
	for i := range contract.Groups {
		if contract.Groups[i].ID == groupID {
			groupIndex = i
			break
		}
	}
	if groupIndex < 0 {
		return testpolicy.Contract{}, addTestsRefusal(fmt.Sprintf("group %q", groupID), "id", "unknown group")
	}
	group := contract.Groups[groupIndex]
	all, _, err := testpolicy.GoTests(group)
	if err != nil || group.Adapter != "go" || all {
		return testpolicy.Contract{}, addTestsRefusal(fmt.Sprintf("group %q", groupID), "tests", "group must use a named Go test list")
	}
	var names []string
	if err := json.Unmarshal(group.Tests, &names); err != nil {
		return testpolicy.Contract{}, addTestsRefusal(fmt.Sprintf("group %q", groupID), "tests", "group must use a named Go test list")
	}
	requested := make([]string, 0, len(additions))
	seen := map[string]bool{}
	for _, name := range additions {
		name = strings.TrimSpace(name)
		if name == "" || strings.Contains(name, ",") {
			return testpolicy.Contract{}, addTestsRefusal(fmt.Sprintf("group %q", groupID), "tests", "test names must be nonempty comma-separated identifiers")
		}
		if !seen[name] {
			requested = append(requested, name)
			seen[name] = true
		}
	}
	if len(requested) == 0 {
		return testpolicy.Contract{}, addTestsRefusal(fmt.Sprintf("group %q", groupID), "tests", "at least one test name is required")
	}
	available, err := declaredFunctions(contractPath, group)
	if err != nil {
		return testpolicy.Contract{}, addTestsRefusal(fmt.Sprintf("group %q", groupID), "packages", err.Error())
	}
	for _, name := range requested {
		if !available[name] {
			return testpolicy.Contract{}, addTestsRefusal(fmt.Sprintf("group %q", groupID), "tests", fmt.Sprintf("no matching func %s( in the group's packages", name))
		}
	}
	merged := appendUnique(names, requested...)
	encoded, _ := json.Marshal(merged)
	contract.Groups[groupIndex].Tests = encoded
	if err := contract.Validate(); err != nil {
		return testpolicy.Contract{}, invalid(err.Error())
	}
	return contract, nil
}

func addTestsRefusal(entity, field, detail string) error {
	return &Refusal{Code: AddTestsCode, Entity: entity, Field: field, Detail: detail}
}

func declaredFunctions(contractPath string, group testpolicy.Group) (map[string]bool, error) {
	cwd, err := resolveGroupCWD(contractPath, group)
	if err != nil {
		return nil, err
	}
	result := map[string]bool{}
	for _, pkg := range group.Packages {
		clean, err := exactPackagePath(pkg)
		if err != nil {
			return nil, err
		}
		directory := filepath.Join(cwd, filepath.FromSlash(clean))
		entries, err := os.ReadDir(directory)
		if err != nil {
			return nil, fmt.Errorf("read package %q: %w", pkg, err)
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
				continue
			}
			file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(directory, entry.Name()), nil, 0)
			if err != nil {
				return nil, fmt.Errorf("parse package %q file %s: %w", pkg, entry.Name(), err)
			}
			for _, declaration := range file.Decls {
				function, ok := declaration.(*ast.FuncDecl)
				if ok && function.Recv == nil {
					result[function.Name.Name] = true
				}
			}
		}
	}
	return result, nil
}

func resolveGroupCWD(contractPath string, group testpolicy.Group) (string, error) {
	absolute, err := filepath.Abs(contractPath)
	if err != nil {
		return "", err
	}
	for root := filepath.Dir(absolute); ; root = filepath.Dir(root) {
		cwd := filepath.Join(root, filepath.FromSlash(group.CWD))
		allPresent := true
		for _, pkg := range group.Packages {
			clean, err := exactPackagePath(pkg)
			if err != nil {
				return "", err
			}
			if info, err := os.Stat(filepath.Join(cwd, filepath.FromSlash(clean))); err != nil || !info.IsDir() {
				allPresent = false
				break
			}
		}
		if allPresent {
			return cwd, nil
		}
		parent := filepath.Dir(root)
		if parent == root {
			break
		}
	}
	return "", fmt.Errorf("cannot locate cwd %q and its packages from %s", group.CWD, contractPath)
}

func exactPackagePath(pkg string) (string, error) {
	if pkg == "./" {
		return ".", nil
	}
	clean := strings.TrimPrefix(pkg, "./")
	if clean == "." {
		return clean, nil
	}
	if clean == "" || strings.Contains(clean, "...") || filepath.IsAbs(clean) || filepath.ToSlash(filepath.Clean(clean)) != clean || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("package %q is not an exact relative package", pkg)
	}
	return clean, nil
}

// RemoveTests takes named Go tests out of a group's named test list: the
// follow-up of deleting a test. A name the group no longer lists is already
// removed; the names need not exist in the group's packages.
func RemoveTests(contract testpolicy.Contract, groupID string, removals []string) (testpolicy.Contract, error) {
	groupIndex := -1
	for i := range contract.Groups {
		if contract.Groups[i].ID == groupID {
			groupIndex = i
			break
		}
	}
	if groupIndex < 0 {
		return testpolicy.Contract{}, addTestsRefusal(fmt.Sprintf("group %q", groupID), "id", "unknown group")
	}
	group := contract.Groups[groupIndex]
	all, _, err := testpolicy.GoTests(group)
	var names []string
	if err != nil || group.Adapter != "go" || all || json.Unmarshal(group.Tests, &names) != nil {
		return testpolicy.Contract{}, addTestsRefusal(fmt.Sprintf("group %q", groupID), "tests", "group must use a named Go test list")
	}
	drop := map[string]bool{}
	for _, name := range removals {
		name = strings.TrimSpace(name)
		if name == "" {
			return testpolicy.Contract{}, addTestsRefusal(fmt.Sprintf("group %q", groupID), "tests", "test names must be nonempty comma-separated identifiers")
		}
		drop[name] = true
	}
	if len(drop) == 0 {
		return testpolicy.Contract{}, addTestsRefusal(fmt.Sprintf("group %q", groupID), "tests", "at least one test name is required")
	}
	kept := make([]string, 0, len(names))
	for _, name := range names {
		if !drop[name] {
			kept = append(kept, name)
		}
	}
	encoded, _ := json.Marshal(kept)
	contract.Groups[groupIndex].Tests = encoded
	if err := contract.Validate(); err != nil {
		return testpolicy.Contract{}, invalid(err.Error())
	}
	return contract, nil
}

// RemoveGroup takes a whole group out of the contract, with every surface,
// always, unknown and cadence reference to it. A group the contract no
// longer has is already removed.
func RemoveGroup(contract testpolicy.Contract, groupID string) (testpolicy.Contract, error) {
	without := func(ids []string) []string {
		kept := make([]string, 0, len(ids))
		for _, id := range ids {
			if id != groupID {
				kept = append(kept, id)
			}
		}
		return kept
	}
	groups := make([]testpolicy.Group, 0, len(contract.Groups))
	for _, group := range contract.Groups {
		if group.ID != groupID {
			groups = append(groups, group)
		}
	}
	contract.Groups = groups
	for i := range contract.Surfaces {
		surface := &contract.Surfaces[i]
		surface.Standard, surface.Deep, surface.Critical = without(surface.Standard), without(surface.Deep), without(surface.Critical)
		if surface.CrossCutting != nil {
			surface.CrossCutting = without(surface.CrossCutting)
		}
	}
	contract.Always.Canary, contract.Always.Standard = without(contract.Always.Canary), without(contract.Always.Standard)
	contract.Unknown, contract.Cadence = without(contract.Unknown), without(contract.Cadence)
	if err := contract.Validate(); err != nil {
		return testpolicy.Contract{}, invalid(err.Error())
	}
	return contract, nil
}
