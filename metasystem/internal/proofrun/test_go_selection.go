package proofrun

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gopackages"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// ExpandGoPackageGroups turns protected package-selection templates into
// ordinary, stable one-package groups. It must run after contract protection
// and before selection, against the exact change base and candidate tree.
func ExpandGoPackageGroups(contract testpolicy.Contract, projectRoot, baseTree, candidateTree string) (testpolicy.Contract, error) {
	return ExpandGoPackageGroupsWithEnvironment(contract, projectRoot, baseTree, candidateTree, os.Environ())
}

func ExpandGoPackageGroupsWithEnvironment(contract testpolicy.Contract, projectRoot, baseTree, candidateTree string, baseEnvironment []string) (testpolicy.Contract, error) {
	return expandGoPackageGroupsWithSelector(contract, projectRoot, baseTree, candidateTree, baseEnvironment, gopackages.SelectWithEnvironment)
}

func expandGoPackageGroupsWithSelector(contract testpolicy.Contract, projectRoot, baseTree, candidateTree string, baseEnvironment []string, selectPackages func(string, string, string, []string, []string) (gopackages.Selection, error)) (testpolicy.Contract, error) {
	result := contract
	result.Groups = append([]testpolicy.Group(nil), contract.Groups...)
	result.Always.Standard = append([]string(nil), contract.Always.Standard...)
	seen := map[string]bool{}
	for _, group := range contract.Groups {
		seen[group.ID] = true
	}
	selections := map[string]gopackages.Selection{}
	for _, template := range contract.Groups {
		if template.PackageSelection == "" {
			continue
		}
		environment := GroupTestingEnvironment(baseEnvironment, template)
		selectionKey := template.CWD + "\x00" + strings.Join(template.BuildTags, ",") + "\x00" + digestEnvironment(environment)
		selection, ok := selections[selectionKey]
		if !ok {
			moduleRoot := filepath.Join(projectRoot, filepath.FromSlash(template.CWD))
			var err error
			selection, err = selectPackages(moduleRoot, baseTree, candidateTree, template.BuildTags, environment)
			if err != nil {
				return testpolicy.Contract{}, fmt.Errorf("expand Go selector %s: %w", template.ID, err)
			}
			selections[selectionKey] = selection
		}
		for _, pkg := range selection.Packages {
			clone := template
			clone.PackageSelection = ""
			clone.Packages = []string{pkg}
			clone.Inputs = append([]string(nil), template.Inputs...)
			for _, dependency := range selection.InputDirs[pkg] {
				path := template.CWD
				if dependency != "." {
					path = filepath.ToSlash(filepath.Join(path, strings.TrimPrefix(dependency, "./")))
				}
				if path == "." {
					// The shared grammar has no bare ** declaration. These two
					// supported patterns cover root files and every descendant.
					clone.Inputs = append(clone.Inputs, "*", "*/**")
					continue
				}
				clone.Inputs = append(clone.Inputs, path+"/**")
			}
			clone.ID = packageGroupID(template.ID, pkg)
			if seen[clone.ID] {
				return testpolicy.Contract{}, fmt.Errorf("go selector %s produced duplicate group %s", template.ID, clone.ID)
			}
			seen[clone.ID] = true
			result.Groups = append(result.Groups, clone)
			result.Always.Standard = append(result.Always.Standard, clone.ID)
		}
	}
	if err := result.Validate(); err != nil {
		return testpolicy.Contract{}, fmt.Errorf("expanded Go testing contract: %w", err)
	}
	return result, nil
}

func packageGroupID(template, pkg string) string {
	name := strings.TrimPrefix(pkg, "./")
	if pkg == "." {
		name = "root"
	}
	var slug strings.Builder
	for _, r := range name {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			slug.WriteRune(r)
		} else {
			slug.WriteByte('-')
		}
	}
	readable := strings.Trim(slug.String(), "-")
	if len(readable) > 48 {
		readable = readable[len(readable)-48:]
	}
	readable = strings.Trim(readable, "-")
	if readable == "" {
		readable = "root"
	}
	if readable[0] < 'a' || readable[0] > 'z' {
		readable = "p-" + readable
	}
	digest := sha256.Sum256([]byte(pkg))
	return fmt.Sprintf("%s/%s-%x", template, readable, digest[:6])
}
