package testpolicy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const SchemaVersion = 1

var (
	identifier      = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	groupIdentifier = regexp.MustCompile(`^[a-z][a-z0-9-]*(?:/[a-z][a-z0-9-]*)?$`)
)

type Contract struct {
	SchemaVersion     int         `json:"schemaVersion"`
	TailoringRequired bool        `json:"tailoringRequired,omitempty"`
	ProjectRisk       ProjectRisk `json:"projectRisk"`
	Surfaces          []Surface   `json:"surfaces"`
	Groups            []Group     `json:"groups"`
	Always            Always      `json:"always"`
	Unknown           []string    `json:"unknown"`
	Cadence           []string    `json:"cadence"`
}

type ProjectRisk struct {
	Severity      int    `json:"severity"`
	Exposure      int    `json:"exposure"`
	Reversibility string `json:"reversibility"`
	Detection     string `json:"detection"`
	Recovery      string `json:"recovery"`
}

type RiskRaise struct {
	Severity      int    `json:"severity,omitempty"`
	Exposure      int    `json:"exposure,omitempty"`
	Reversibility string `json:"reversibility,omitempty"`
	Detection     string `json:"detection,omitempty"`
	Recovery      string `json:"recovery,omitempty"`
}

type Surface struct {
	ID           string     `json:"id"`
	Paths        []string   `json:"paths"`
	DependsOn    []string   `json:"dependsOn"`
	Standard     []string   `json:"standard"`
	Deep         []string   `json:"deep"`
	Critical     []string   `json:"critical"`
	CrossCutting []string   `json:"crossCutting,omitempty"`
	Risk         *RiskRaise `json:"risk,omitempty"`
}

type Always struct {
	Canary   []string `json:"canary"`
	Standard []string `json:"standard"`
}

type Tool struct {
	ID          string   `json:"id"`
	Executable  string   `json:"executable"`
	VersionArgs []string `json:"versionArgs"`
}

type ExternalInput struct {
	ID   string `json:"id"`
	Path string `json:"path"`
}

type ExpectedTest struct {
	Report    string `json:"report"`
	Classname string `json:"classname"`
	Name      string `json:"name"`
}

type Group struct {
	ID             string            `json:"id"`
	Kind           string            `json:"kind"`
	Adapter        string            `json:"adapter"`
	CWD            string            `json:"cwd"`
	Inputs         []string          `json:"inputs"`
	Outputs        []string          `json:"outputs"`
	Tools          []Tool            `json:"tools"`
	ExternalInputs []ExternalInput   `json:"externalInputs,omitempty"`
	Obligations    []string          `json:"obligations"`
	Platforms      []string          `json:"platforms"`
	TargetMS       int64             `json:"targetMs"`
	Packages       []string          `json:"packages,omitempty"`
	Tests          json.RawMessage   `json:"tests,omitempty"`
	Race           bool              `json:"race,omitempty"`
	Coverage       bool              `json:"coverage,omitempty"`
	Env            map[string]string `json:"env,omitempty"`
	Section        string            `json:"section,omitempty"`
	Argv           []string          `json:"argv,omitempty"`
	Reports        []string          `json:"reports,omitempty"`
	Format         string            `json:"format,omitempty"`
	ExpectedTests  []ExpectedTest    `json:"expectedTests,omitempty"`
}

func Decode(data []byte) (Contract, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var contract Contract
	if err := decoder.Decode(&contract); err != nil {
		return Contract{}, fmt.Errorf("decode testing contract: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return Contract{}, fmt.Errorf("decode testing contract: trailing data")
	}
	if err := contract.Validate(); err != nil {
		return Contract{}, err
	}
	return contract, nil
}

func Load(path string) (Contract, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Contract{}, fmt.Errorf("read testing contract %s: %w", path, err)
	}
	return Decode(data)
}

func (contract Contract) Validate() error {
	if contract.SchemaVersion != SchemaVersion {
		return fmt.Errorf("testing contract schemaVersion must be %d", SchemaVersion)
	}
	if contract.TailoringRequired {
		if len(contract.Surfaces) != 0 || len(contract.Groups) != 0 || len(contract.Unknown) != 0 ||
			len(contract.Always.Canary) != 0 || len(contract.Always.Standard) != 0 || len(contract.Cadence) != 0 {
			return fmt.Errorf("TEST_CONTRACT_REQUIRED: the incomplete adoption template cannot contain executable groups or policy")
		}
		return fmt.Errorf("TEST_CONTRACT_REQUIRED: replace the incomplete adoption template with reviewed application test groups")
	}
	if err := validateProjectRisk(contract.ProjectRisk); err != nil {
		return err
	}
	if len(contract.Surfaces) == 0 || len(contract.Groups) == 0 || len(contract.Unknown) == 0 {
		return fmt.Errorf("testing contract requires nonempty surfaces, groups, and unknown")
	}
	groups := make(map[string]Group, len(contract.Groups))
	for _, group := range contract.Groups {
		if !groupIdentifier.MatchString(group.ID) || groups[group.ID].ID != "" {
			return fmt.Errorf("testing group id %q is invalid or duplicated", group.ID)
		}
		if err := validateGroup(group); err != nil {
			return fmt.Errorf("testing group %s: %w", group.ID, err)
		}
		groups[group.ID] = group
	}
	surfaces := make(map[string]Surface, len(contract.Surfaces))
	for _, surface := range contract.Surfaces {
		if !identifier.MatchString(surface.ID) || surfaces[surface.ID].ID != "" || len(surface.Paths) == 0 {
			return fmt.Errorf("testing surface id %q is invalid, duplicated, or has no paths", surface.ID)
		}
		for _, path := range surface.Paths {
			if !validPathDeclaration(path) {
				return fmt.Errorf("testing surface %s has invalid path %q", surface.ID, path)
			}
		}
		if surface.Risk != nil {
			if err := validateRiskRaise(contract.ProjectRisk, *surface.Risk); err != nil {
				return fmt.Errorf("testing surface %s: %w", surface.ID, err)
			}
		}
		surfaces[surface.ID] = surface
	}
	for _, surface := range contract.Surfaces {
		for _, dependency := range surface.DependsOn {
			if _, ok := surfaces[dependency]; !ok {
				return fmt.Errorf("testing surface %s references missing dependency %s", surface.ID, dependency)
			}
		}
		for _, group := range append(append([]string{}, surface.Standard...), surface.Deep...) {
			if _, ok := groups[group]; !ok {
				return fmt.Errorf("testing surface %s references missing group %s", surface.ID, group)
			}
		}
		for _, obligation := range append(append([]string{}, surface.Critical...), surface.CrossCutting...) {
			if len(providers(groups, obligation)) == 0 {
				return fmt.Errorf("testing surface %s obligation %s has no provider", surface.ID, obligation)
			}
		}
	}
	for _, group := range append(append(append([]string{}, contract.Always.Canary...), contract.Always.Standard...), append(contract.Unknown, contract.Cadence...)...) {
		if _, ok := groups[group]; !ok {
			return fmt.Errorf("testing contract references missing group %s", group)
		}
	}
	applicationProvider := false
	for _, surface := range contract.Surfaces {
		for _, id := range surface.Standard {
			group := groups[id]
			if group.Kind != "static" && group.Kind != "build" {
				applicationProvider = true
			}
		}
	}
	if !applicationProvider {
		return fmt.Errorf("testing contract has no standard application test provider")
	}
	return nil
}

// IncompleteTemplate is the explicit first-adoption state. It is valid JSON
// in the current schema but deliberately cannot pass contract validation or
// claim application readiness until the project replaces it with real groups.
func IncompleteTemplate() ([]byte, error) {
	value := Contract{SchemaVersion: SchemaVersion, TailoringRequired: true,
		ProjectRisk: ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"},
		Surfaces:    []Surface{}, Groups: []Group{}, Always: Always{Canary: []string{}, Standard: []string{}}, Unknown: []string{}, Cadence: []string{}}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func validateGroup(group Group) error {
	kinds := map[string]bool{"unit": true, "component": true, "integration": true, "e2e": true, "performance": true, "static": true, "build": true}
	if !kinds[group.Kind] || group.CWD == "" || !validRelative(group.CWD) || len(group.Inputs) == 0 || len(group.Platforms) == 0 || group.TargetMS <= 0 {
		return fmt.Errorf("kind, cwd, platforms, and positive targetMs are required")
	}
	for _, path := range append(append([]string{}, group.Inputs...), group.Outputs...) {
		if !validPathDeclaration(path) {
			return fmt.Errorf("invalid input or output path %q", path)
		}
	}
	for _, input := range group.Inputs {
		for _, output := range group.Outputs {
			if pathDeclarationsOverlap(input, output) {
				return fmt.Errorf("input %q overlaps output %q", input, output)
			}
		}
	}
	toolIDs := map[string]bool{}
	for _, tool := range group.Tools {
		if !identifier.MatchString(tool.ID) || toolIDs[tool.ID] || tool.Executable == "" || len(tool.VersionArgs) == 0 {
			return fmt.Errorf("tools require unique ids, executables, and version arguments")
		}
		toolIDs[tool.ID] = true
	}
	externalIDs := map[string]bool{}
	for _, external := range group.ExternalInputs {
		if !identifier.MatchString(external.ID) || externalIDs[external.ID] || !validExternalLocator(external.Path) {
			return fmt.Errorf("external inputs require unique ids and nonempty locators")
		}
		externalIDs[external.ID] = true
	}
	switch group.Adapter {
	case "go":
		if len(group.Packages) == 0 || len(group.Tests) == 0 {
			return fmt.Errorf("go adapter requires packages and tests")
		}
		allTests, _, err := GoTests(group)
		if err != nil {
			return err
		}
		if group.Coverage && !allTests {
			return fmt.Errorf("whole-package coverage requires tests=all")
		}
	case "section":
		if !identifier.MatchString(group.Section) {
			return fmt.Errorf("section adapter requires one stable section id")
		}
	case "command":
		if len(group.Argv) == 0 || group.Argv[0] == "" {
			return fmt.Errorf("command adapter requires argv")
		}
		if group.Argv[0] == "true" || (len(group.Argv) >= 3 && strings.HasSuffix(group.Argv[0], "sh") && group.Argv[1] == "-c" && strings.TrimSpace(group.Argv[2]) == "true") {
			return fmt.Errorf("blanket success commands are not testing evidence")
		}
		if group.Format == "junit-xml" {
			if len(group.Reports) == 0 || len(group.ExpectedTests) == 0 || len(group.Outputs) == 0 {
				return fmt.Errorf("junit-xml command requires expectedTests")
			}
			for _, report := range group.Reports {
				if !validRelative(report) {
					return fmt.Errorf("junit-xml command has invalid report directory")
				}
				owned := false
				for _, output := range group.Outputs {
					plain := strings.TrimSuffix(output, "/**")
					if report == plain || strings.HasPrefix(report, plain+"/") {
						owned = true
					}
				}
				if !owned {
					return fmt.Errorf("junit-xml report directory %s is outside declared outputs", report)
				}
			}
			seen := map[string]bool{}
			for _, expected := range group.ExpectedTests {
				if expected.Name == "" || !validRelative(expected.Report) {
					return fmt.Errorf("invalid expected test identity")
				}
				owned := false
				for _, report := range group.Reports {
					if strings.HasPrefix(expected.Report, report+"/") {
						owned = true
					}
				}
				if !owned {
					return fmt.Errorf("expected test report is outside declared report directories")
				}
				key := expected.Report + "\x00" + expected.Classname + "\x00" + expected.Name
				if seen[key] {
					return fmt.Errorf("duplicate expected test identity")
				}
				seen[key] = true
			}
		} else if group.Format != "exit-status" || (group.Kind != "static" && group.Kind != "build") {
			return fmt.Errorf("command format must be junit-xml, or exit-status for static/build")
		}
	default:
		return fmt.Errorf("adapter must be go, section, or command")
	}
	return nil
}

var externalEnvironmentLocator = regexp.MustCompile(`^\$\{[A-Z_][A-Z0-9_]*\}(?:/[^\x00]*)?$`)

func validExternalLocator(locator string) bool {
	if filepath.IsAbs(locator) {
		return filepath.Clean(locator) == locator && !strings.ContainsRune(locator, '\x00')
	}
	return externalEnvironmentLocator.MatchString(locator)
}

func pathDeclarationsOverlap(left, right string) bool {
	left = strings.TrimSuffix(left, "/**")
	right = strings.TrimSuffix(right, "/**")
	return left == right || strings.HasPrefix(left, right+"/") || strings.HasPrefix(right, left+"/")
}

func GoTests(group Group) (all bool, names []string, err error) {
	var keyword string
	if json.Unmarshal(group.Tests, &keyword) == nil {
		if keyword != "all" {
			return false, nil, fmt.Errorf("go tests string must be all")
		}
		return true, nil, nil
	}
	if json.Unmarshal(group.Tests, &names) != nil || len(names) == 0 {
		return false, nil, fmt.Errorf("go tests must be all or a nonempty name list")
	}
	seen := map[string]bool{}
	for _, name := range names {
		if name == "" || seen[name] {
			return false, nil, fmt.Errorf("go test names must be nonempty and unique")
		}
		seen[name] = true
	}
	sort.Strings(names)
	return false, names, nil
}

func validPathDeclaration(path string) bool {
	if path == "" || filepath.IsAbs(path) || strings.ContainsRune(path, '\x00') {
		return false
	}
	plain := strings.TrimSuffix(path, "/**")
	return validRelative(plain) && (plain == path || strings.HasSuffix(path, "/**"))
}

func validRelative(path string) bool {
	if path == "." {
		return true
	}
	if path == "" || filepath.IsAbs(path) || filepath.ToSlash(filepath.Clean(path)) != path {
		return false
	}
	for _, component := range strings.Split(path, "/") {
		if component == "" || component == "." || component == ".." {
			return false
		}
	}
	return true
}

func providers(groups map[string]Group, obligation string) []string {
	var result []string
	for id, group := range groups {
		for _, owned := range group.Obligations {
			if owned == obligation {
				result = append(result, id)
			}
		}
	}
	sort.Strings(result)
	return result
}
