package proofrun

import (
	"errors"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gopackages"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func loadGoExpansionContract(t *testing.T) testpolicy.Contract {
	t.Helper()
	contract, err := testpolicy.Load(filepath.Join("..", "..", "testing.json"))
	if err != nil {
		t.Fatal(err)
	}
	return contract
}

func goExpansionTemplate(t *testing.T, contract testpolicy.Contract) testpolicy.Group {
	t.Helper()
	for _, group := range contract.Groups {
		if group.ID == "go-affected" {
			if group.PackageSelection != "changed-and-consumers" {
				t.Fatalf("Go selector template changed: %+v", group)
			}
			return group
		}
	}
	t.Fatal("missing Go selector template")
	return testpolicy.Group{}
}

func concreteGoExpansionGroup(t *testing.T, contract testpolicy.Contract, templateID, pkg string) testpolicy.Group {
	t.Helper()
	var matches []testpolicy.Group
	for _, group := range contract.Groups {
		if strings.HasPrefix(group.ID, templateID+"/") && slices.Equal(group.Packages, []string{pkg}) {
			matches = append(matches, group)
		}
	}
	if len(matches) != 1 {
		t.Fatalf("wanted one concrete %s group for %s, got %d", templateID, pkg, len(matches))
	}
	group := matches[0]
	if group.PackageSelection != "" || !slices.Contains(contract.Always.Standard, group.ID) {
		t.Fatalf("concrete group is not an ordinary standard group: %+v", group)
	}
	return group
}

func TestGoExpansionPreservesGroupsAcrossPackageInventoryChanges(t *testing.T) {
	contract := loadGoExpansionContract(t)
	template := goExpansionTemplate(t, contract)
	if !slices.Contains(template.Inputs, "metasystem/scripts/**") {
		t.Fatalf("Go template lost shared script declaration: %v", template.Inputs)
	}
	root := t.TempDir()
	const baseTree = "opaque-base-inventory-tree"
	const firstTree = "opaque-first-inventory-tree"
	const secondTree = "opaque-second-inventory-tree"
	baseEnvironment := []string{"PATH=/selector-tools", "GOOS=linux", "GOARCH=arm64", "GOWORK=off"}
	wantEnvironment := []string{"GOARCH=arm64", "GOOS=linux", "GOWORK=off", "PATH=/selector-tools"}
	selections := []struct {
		tree      string
		selection gopackages.Selection
	}{
		{firstTree, gopackages.Selection{
			Tree: firstTree, ModulePath: "example.invalid/host",
			Changed: []string{"./base"}, Dependents: []string{"./consumer"},
			Packages: []string{"./base", "./consumer"},
			InputDirs: map[string][]string{
				"./base": {"./base"}, "./consumer": {"./consumer", "./base"},
			},
		}},
		{secondTree, gopackages.Selection{
			Tree: secondTree, ModulePath: "example.invalid/host",
			Changed: []string{"./added", "./base", "./internal/..."}, Dependents: []string{"./consumer"},
			Packages: []string{"./added", "./base", "./consumer", "./internal/live"},
			InputDirs: map[string][]string{
				"./added": {"./added"}, "./base": {"./base"},
				"./consumer": {"./consumer", "./base"}, "./internal/live": {"./internal/live"},
			},
		}},
	}
	called := 0
	selector := func(moduleRoot, base, candidate string, tags, environment []string) (gopackages.Selection, error) {
		if called >= len(selections) {
			t.Fatalf("selector called more than %d times", len(selections))
		}
		want := selections[called]
		called++
		if moduleRoot != filepath.Join(root, "metasystem") || base != baseTree || candidate != want.tree ||
			!slices.Equal(tags, template.BuildTags) || !slices.Equal(environment, wantEnvironment) {
			t.Fatalf("selector call %d: root=%q base=%q candidate=%q tags=%v environment=%v", called, moduleRoot, base, candidate, tags, environment)
		}
		return want.selection, nil
	}
	first, err := expandGoPackageGroupsWithSelector(contract, root, baseTree, firstTree, baseEnvironment, selector)
	if err != nil {
		t.Fatal(err)
	}
	second, err := expandGoPackageGroupsWithSelector(contract, root, baseTree, secondTree, baseEnvironment, selector)
	if err != nil {
		t.Fatal(err)
	}
	if called != len(selections) {
		t.Fatalf("selector calls = %d, want %d consumed", called, len(selections))
	}
	for _, expanded := range []testpolicy.Contract{first, second} {
		if err := expanded.Validate(); err != nil {
			t.Fatalf("invalid expanded contract: %v", err)
		}
	}
	base := concreteGoExpansionGroup(t, first, template.ID, "./base")
	consumer := concreteGoExpansionGroup(t, first, template.ID, "./consumer")
	if base.ID == consumer.ID || !strings.HasPrefix(base.ID, template.ID+"/") || !strings.HasPrefix(consumer.ID, template.ID+"/") {
		t.Fatalf("base and consumer lack distinct concrete IDs: %q %q", base.ID, consumer.ID)
	}
	if !slices.Equal(base.Inputs, append(slices.Clone(template.Inputs), "metasystem/base/**")) ||
		!slices.Equal(consumer.Inputs, append(slices.Clone(template.Inputs), "metasystem/consumer/**", "metasystem/base/**")) {
		t.Fatalf("package inputs: base=%v consumer=%v", base.Inputs, consumer.Inputs)
	}
	if !reflect.DeepEqual(base, concreteGoExpansionGroup(t, second, template.ID, "./base")) {
		t.Fatal("unrelated package inventory changed the base group")
	}
	for _, pkg := range selections[1].selection.Packages {
		concreteGoExpansionGroup(t, second, template.ID, pkg)
	}
	for _, tc := range []struct {
		contract testpolicy.Contract
		want     int
	}{{first, 2}, {second, 4}} {
		count := 0
		for _, group := range tc.contract.Groups {
			if strings.HasPrefix(group.ID, template.ID+"/") {
				count++
				if slices.Equal(group.Packages, []string{"./internal/old"}) {
					t.Fatal("deleted package remained in current inventory")
				}
			}
		}
		if count != tc.want {
			t.Fatalf("concrete groups = %d, want %d", count, tc.want)
		}
	}
}

func TestGoalLandingGoAssetOnlyChangeExpandsConcretePackage(t *testing.T) {
	contract := loadGoExpansionContract(t)
	template := goExpansionTemplate(t, contract)
	root := t.TempDir()
	const baseTree = "opaque-base-asset-tree"
	const candidateTree = "opaque-candidate-asset-tree"
	baseEnvironment := []string{"PATH=/selector-tools", "GOOS=linux", "GOARCH=arm64", "GOWORK=off"}
	wantEnvironment := []string{"GOARCH=arm64", "GOOS=linux", "GOWORK=off", "PATH=/selector-tools"}
	selection := gopackages.Selection{
		Tree: candidateTree, ModulePath: "example.invalid/assets",
		Changed: []string{"./app"}, Packages: []string{"./app"},
		InputDirs: map[string][]string{"./app": {"./app"}},
	}
	calls := 0
	selector := func(moduleRoot, base, candidate string, tags, environment []string) (gopackages.Selection, error) {
		calls++
		if calls != 1 || moduleRoot != filepath.Join(root, "metasystem") || base != baseTree || candidate != candidateTree ||
			!slices.Equal(tags, template.BuildTags) || !slices.Equal(environment, wantEnvironment) {
			t.Fatalf("unexpected selector call %d: root=%q base=%q candidate=%q tags=%v environment=%v", calls, moduleRoot, base, candidate, tags, environment)
		}
		return selection, nil
	}
	expanded, err := expandGoPackageGroupsWithSelector(contract, root, baseTree, candidateTree, baseEnvironment, selector)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("selector calls = %d, want one consumed call", calls)
	}
	if err := expanded.Validate(); err != nil {
		t.Fatalf("expanded contract is invalid: %v", err)
	}
	group := concreteGoExpansionGroup(t, expanded, template.ID, "./app")
	if !slices.Contains(group.Inputs, "metasystem/app/**") {
		t.Fatalf("asset-only group lacks its native input directory: %+v", group)
	}
}

func TestGoExpansionIncludesCompletePackageInputs(t *testing.T) {
	cases := []struct {
		name       string
		cwd        string
		pkg        string
		inputDirs  []string
		addedInput []string
	}{
		{name: "root", cwd: ".", pkg: ".", inputDirs: []string{"."}, addedInput: []string{"*", "*/**"}},
		{name: "newpkg", cwd: "metasystem", pkg: "./newpkg", inputDirs: []string{"./newpkg"}, addedInput: []string{"metasystem/newpkg/**"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			contract := loadGoExpansionContract(t)
			for i := range contract.Groups {
				if contract.Groups[i].ID == "go-affected" {
					contract.Groups[i].CWD = tc.cwd
				}
			}
			template := goExpansionTemplate(t, contract)
			root := t.TempDir()
			baseTree := "opaque-base-" + tc.name
			candidateTree := "opaque-candidate-" + tc.name
			baseEnvironment := []string{"PATH=/selector-tools", "GOOS=linux", "GOARCH=arm64", "GOWORK=off"}
			wantEnvironment := []string{"GOARCH=arm64", "GOOS=linux", "GOWORK=off", "PATH=/selector-tools"}
			selection := gopackages.Selection{
				Tree: candidateTree, ModulePath: "example.invalid/" + tc.name,
				Changed: []string{tc.pkg}, Packages: []string{tc.pkg},
				InputDirs: map[string][]string{tc.pkg: tc.inputDirs},
			}
			calls := 0
			selector := func(moduleRoot, base, candidate string, tags, environment []string) (gopackages.Selection, error) {
				calls++
				if calls != 1 || moduleRoot != filepath.Join(root, tc.cwd) || base != baseTree || candidate != candidateTree ||
					!slices.Equal(tags, template.BuildTags) || !slices.Equal(environment, wantEnvironment) {
					t.Fatalf("selector call %d: root=%q base=%q candidate=%q tags=%v environment=%v",
						calls, moduleRoot, base, candidate, tags, environment)
				}
				return selection, nil
			}
			expanded, err := expandGoPackageGroupsWithSelector(contract, root, baseTree, candidateTree, baseEnvironment, selector)
			if err != nil {
				t.Fatal(err)
			}
			if calls != 1 {
				t.Fatalf("selector calls = %d, want one consumed call", calls)
			}
			if err := expanded.Validate(); err != nil {
				t.Fatalf("expanded contract is invalid: %v", err)
			}
			group := concreteGoExpansionGroup(t, expanded, template.ID, tc.pkg)
			wantInputs := append(append([]string(nil), template.Inputs...), tc.addedInput...)
			if !slices.Equal(group.Inputs, wantInputs) {
				t.Fatalf("%s package inputs = %v, want %v", tc.name, group.Inputs, wantInputs)
			}
		})
	}
}

func TestGoExpansionSelectorBoundary(t *testing.T) {
	cases := []struct {
		name string
		run  func(*testing.T)
	}{
		{"argument_and_environment_forwarding", func(t *testing.T) {
			contract := loadGoExpansionContract(t)
			for i := range contract.Groups {
				if contract.Groups[i].ID == "go-affected" {
					contract.Groups[i].BuildTags = []string{"linux", "integration"}
					contract.Groups[i].Env = map[string]string{"GOOS": "darwin", "CUSTOM": "proof"}
				}
			}
			template := goExpansionTemplate(t, contract)
			root := t.TempDir()
			baseEnvironment := []string{"PATH=/selector-tools", "GOOS=linux", "GOARCH=arm64"}
			wantEnvironment := []string{"CUSTOM=proof", "GOARCH=arm64", "GOOS=darwin", "PATH=/selector-tools"}
			calls := 0
			selector := func(moduleRoot, base, candidate string, tags, environment []string) (gopackages.Selection, error) {
				calls++
				if calls != 1 || moduleRoot != filepath.Join(root, template.CWD) || base != "opaque-base-forward" || candidate != "opaque-candidate-forward" ||
					!slices.Equal(tags, []string{"linux", "integration"}) || !slices.Equal(environment, wantEnvironment) {
					t.Fatalf("selector arguments: call=%d root=%q base=%q candidate=%q tags=%v environment=%v", calls, moduleRoot, base, candidate, tags, environment)
				}
				return gopackages.Selection{Packages: []string{"./app"}, InputDirs: map[string][]string{"./app": {"./app"}}}, nil
			}
			expanded, err := expandGoPackageGroupsWithSelector(contract, root, "opaque-base-forward", "opaque-candidate-forward", baseEnvironment, selector)
			if err != nil || calls != 1 {
				t.Fatalf("expand: calls=%d err=%v", calls, err)
			}
			if err := expanded.Validate(); err != nil {
				t.Fatal(err)
			}
			group := concreteGoExpansionGroup(t, expanded, template.ID, "./app")
			if !slices.Equal(group.BuildTags, template.BuildTags) || !slices.Contains(group.Inputs, "metasystem/app/**") {
				t.Fatalf("selector facts were not carried into the concrete group: %+v", group)
			}
		}},
		{"same_key_callback_reuse", func(t *testing.T) {
			contract := loadGoExpansionContract(t)
			copyTemplate := goExpansionTemplate(t, contract)
			copyTemplate.ID = "go-affected-reuse"
			contract.Groups = append(contract.Groups, copyTemplate)
			if err := contract.Validate(); err != nil {
				t.Fatal(err)
			}
			calls := 0
			selector := func(string, string, string, []string, []string) (gopackages.Selection, error) {
				calls++
				if calls != 1 {
					t.Fatalf("same-key selector called %d times", calls)
				}
				return gopackages.Selection{Packages: []string{"./app"}, InputDirs: map[string][]string{"./app": {"./app"}}}, nil
			}
			expanded, err := expandGoPackageGroupsWithSelector(contract, t.TempDir(), "opaque-base-reuse", "opaque-candidate-reuse", []string{"PATH=/selector-tools"}, selector)
			if err != nil || calls != 1 {
				t.Fatalf("expand: calls=%d err=%v", calls, err)
			}
			if err := expanded.Validate(); err != nil {
				t.Fatal(err)
			}
			for _, id := range []string{"go-affected", copyTemplate.ID} {
				group := concreteGoExpansionGroup(t, expanded, id, "./app")
				if !slices.Contains(group.Inputs, "metasystem/app/**") {
					t.Fatalf("%s lost the selected input directory", id)
				}
			}
		}},
		{"selector_error_wraps_template_id", func(t *testing.T) {
			contract := loadGoExpansionContract(t)
			template := goExpansionTemplate(t, contract)
			cause := errors.New("declared selector failure")
			calls := 0
			selector := func(string, string, string, []string, []string) (gopackages.Selection, error) {
				calls++
				return gopackages.Selection{}, cause
			}
			expanded, err := expandGoPackageGroupsWithSelector(contract, t.TempDir(), "opaque-base-error", "opaque-candidate-error", []string{"PATH=/selector-tools"}, selector)
			if calls != 1 || err == nil || !errors.Is(err, cause) || !strings.Contains(err.Error(), template.ID) || !reflect.DeepEqual(expanded, testpolicy.Contract{}) {
				t.Fatalf("selector error: calls=%d contract=%+v err=%v", calls, expanded, err)
			}
		}},
		{"input_contract_is_unchanged", func(t *testing.T) {
			contract := loadGoExpansionContract(t)
			original := loadGoExpansionContract(t)
			selector := func(string, string, string, []string, []string) (gopackages.Selection, error) {
				return gopackages.Selection{Packages: []string{"./app"}, InputDirs: map[string][]string{"./app": {"./app"}}}, nil
			}
			expanded, err := expandGoPackageGroupsWithSelector(contract, t.TempDir(), "opaque-base-input", "opaque-candidate-input", []string{"PATH=/selector-tools"}, selector)
			if err != nil {
				t.Fatal(err)
			}
			if err := expanded.Validate(); err != nil {
				t.Fatal(err)
			}
			concreteGoExpansionGroup(t, expanded, "go-affected", "./app")
			if !reflect.DeepEqual(contract, original) {
				t.Fatal("expansion mutated its input contract")
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, tc.run)
	}
}
