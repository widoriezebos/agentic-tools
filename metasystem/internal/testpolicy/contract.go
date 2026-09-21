package testpolicy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/pathpattern"
)

const SchemaVersion = 1
const ExecutionContractSchemaVersion = 2
const TestWorkersEnvironment = "METASYSTEM_TEST_WORKERS"

var (
	identifier      = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	groupIdentifier = regexp.MustCompile(`^[a-z][a-z0-9-]*(?:/[a-z][a-z0-9-]*)?$`)
	environmentName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

type Contract struct {
	SchemaVersion     int         `json:"schemaVersion"`
	TailoringRequired bool        `json:"tailoringRequired,omitempty"`
	ProjectRisk       ProjectRisk `json:"projectRisk"`
	// Fallback names the surface that owns a changed path by exclusion when no surface path pattern matches.
	Fallback string    `json:"fallback,omitempty"`
	Surfaces []Surface `json:"surfaces"`
	Groups   []Group   `json:"groups"`
	Always   Always    `json:"always"`
	Unknown  []string  `json:"unknown"`
	Cadence  []string  `json:"cadence"`
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
	ID                string          `json:"id"`
	Requires          []string        `json:"requires,omitempty"`
	Phase             string          `json:"phase,omitempty"`
	EnvironmentMode   string          `json:"environmentMode,omitempty"`
	Resources         GroupResources  `json:"resources,omitempty"`
	Freshness         string          `json:"freshness,omitempty"`
	FreshnessMaxAgeMS *int64          `json:"freshnessMaxAgeMs,omitempty"`
	Kind              string          `json:"kind"`
	Adapter           string          `json:"adapter"`
	CWD               string          `json:"cwd"`
	Inputs            []string        `json:"inputs"`
	Outputs           []string        `json:"outputs"`
	Tools             []Tool          `json:"tools"`
	ExternalInputs    []ExternalInput `json:"externalInputs,omitempty"`
	Obligations       []string        `json:"obligations"`
	Platforms         []string        `json:"platforms"`
	TargetMS          int64           `json:"targetMs"`
	CPUBudgetSeconds  *int64          `json:"cpuBudgetSeconds,omitempty"`
	Packages          []string        `json:"packages,omitempty"`
	BuildTags         []string        `json:"buildTags,omitempty"`
	// PackageSelection is a protected template expanded against exact Go trees
	// into one ordinary group per affected package before policy selection.
	PackageSelection string          `json:"packageSelection,omitempty"`
	Tests            json.RawMessage `json:"tests,omitempty"`
	Race             bool            `json:"race,omitempty"`
	Coverage         bool            `json:"coverage,omitempty"`
	// RaceSet and CoverageSet preserve an explicit false across a load/write
	// cycle. In this contract, spelling out a disabled expensive mode is an
	// intentional declaration rather than the same document as omission.
	RaceSet     bool `json:"-"`
	CoverageSet bool `json:"-"`
	// Shards splits a go group's selected discovered tests round-robin
	// into this many concurrent go test launches inside the one group; zero
	// or one runs the group as a single launch. Coverage is merged from the
	// shards' coverage data.
	Shards        int               `json:"shards,omitempty"`
	Env           map[string]string `json:"env,omitempty"`
	Section       string            `json:"section,omitempty"`
	Argv          []string          `json:"argv,omitempty"`
	Reports       []string          `json:"reports,omitempty"`
	Format        string            `json:"format,omitempty"`
	ExpectedTests []ExpectedTest    `json:"expectedTests,omitempty"`
}

// GroupResources declares the local capacity class and exclusive resources
// consumed while this group's native command and its children are live.
type GroupResources struct {
	Class     string   `json:"class,omitempty"`
	Exclusive []string `json:"exclusive,omitempty"`
	// Workers is the adapter-worker share. Omission means one worker,
	// explicit zero means the complete attempt allowance, and a positive
	// value requests that exact share.
	Workers *int `json:"workers,omitempty"`
}

type groupWithoutMethods Group

func (group *Group) UnmarshalJSON(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var decoded groupWithoutMethods
	if err := decoder.Decode(&decoded); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	*group = Group(decoded)
	_, group.RaceSet = fields["race"]
	_, group.CoverageSet = fields["coverage"]
	return nil
}

func (group Group) MarshalJSON() ([]byte, error) {
	value := reflect.ValueOf(groupWithoutMethods(group))
	typ := value.Type()
	var out bytes.Buffer
	out.WriteByte('{')
	written := 0
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		tag := strings.Split(field.Tag.Get("json"), ",")
		if len(tag) == 0 || tag[0] == "" || tag[0] == "-" {
			continue
		}
		omitEmpty := len(tag) > 1 && tag[1] == "omitempty"
		explicitFalse := field.Name == "Race" && group.RaceSet || field.Name == "Coverage" && group.CoverageSet
		if omitEmpty && jsonEmptyValue(value.Field(i)) && !explicitFalse {
			continue
		}
		encoded, err := json.Marshal(value.Field(i).Interface())
		if err != nil {
			return nil, err
		}
		if written > 0 {
			out.WriteByte(',')
		}
		name, _ := json.Marshal(tag[0])
		out.Write(name)
		out.WriteByte(':')
		out.Write(encoded)
		written++
	}
	out.WriteByte('}')
	return out.Bytes(), nil
}

func jsonEmptyValue(value reflect.Value) bool {
	switch value.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return value.Len() == 0
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Interface, reflect.Pointer:
		return value.IsZero()
	case reflect.Struct:
		return value.IsZero()
	}
	return false
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
	if contract.SchemaVersion != SchemaVersion && contract.SchemaVersion != ExecutionContractSchemaVersion {
		return fmt.Errorf("testing contract schemaVersion must be %d or %d", SchemaVersion, ExecutionContractSchemaVersion)
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
		if contract.SchemaVersion == SchemaVersion && (len(group.Requires) > 0 || group.Phase != "" || group.EnvironmentMode != "" || group.PackageSelection != "" || len(group.BuildTags) != 0 ||
			group.Resources.Class != "" || len(group.Resources.Exclusive) != 0 || group.Resources.Workers != nil || group.Freshness != "" || group.FreshnessMaxAgeMS != nil) {
			return fmt.Errorf("testing group %s: execution contract fields require schemaVersion %d", group.ID, ExecutionContractSchemaVersion)
		}
		if contract.SchemaVersion == ExecutionContractSchemaVersion && (group.Phase != "admission" && group.Phase != "acceptance") {
			return fmt.Errorf("testing group %s: phase must be admission or acceptance", group.ID)
		}
		if contract.SchemaVersion == ExecutionContractSchemaVersion && group.EnvironmentMode != "inherit" && group.EnvironmentMode != "explicit" {
			return fmt.Errorf("testing group %s: environmentMode must be inherit or explicit", group.ID)
		}
		groups[group.ID] = group
	}
	visiting, visited := map[string]bool{}, map[string]bool{}
	var visit func(string) error
	visit = func(id string) error {
		if visiting[id] {
			return fmt.Errorf("testing group prerequisite cycle at %s", id)
		}
		if visited[id] {
			return nil
		}
		visiting[id] = true
		for _, dependency := range groups[id].Requires {
			if _, ok := groups[dependency]; !ok {
				return fmt.Errorf("testing group %s requires missing group %s", id, dependency)
			}
			if err := visit(dependency); err != nil {
				return err
			}
		}
		delete(visiting, id)
		visited[id] = true
		return nil
	}
	for id := range groups {
		if err := visit(id); err != nil {
			return err
		}
	}
	surfaces := make(map[string]Surface, len(contract.Surfaces))
	for _, surface := range contract.Surfaces {
		if !identifier.MatchString(surface.ID) || surfaces[surface.ID].ID != "" || len(surface.Paths) == 0 && surface.ID != contract.Fallback {
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
	if contract.Fallback != "" {
		fallback, ok := surfaces[contract.Fallback]
		if !ok {
			return fmt.Errorf("testing contract references missing fallback surface %s", contract.Fallback)
		}
		// A fallback owns only by exclusion, so it declares neither path
		// patterns nor dependencies that could also activate it for an owned path.
		if len(fallback.Paths) != 0 {
			return fmt.Errorf("testing fallback surface %s must declare no paths", fallback.ID)
		}
		if len(fallback.DependsOn) != 0 {
			return fmt.Errorf("testing fallback surface %s must declare no dependsOn", fallback.ID)
		}
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
	// A selector is policy, not executable evidence. Its generated concrete
	// groups enter always.standard after exact-tree expansion. Keeping the
	// template out of every other policy edge prevents a missing or empty
	// expansion from satisfying an obligation or prerequisite by name.
	for _, template := range contract.Groups {
		if template.PackageSelection == "" {
			continue
		}
		if strings.Contains(template.ID, "/") {
			return fmt.Errorf("testing group %s: package selector template id cannot contain a slash", template.ID)
		}
		if len(template.Obligations) != 0 || len(template.Requires) != 0 {
			return fmt.Errorf("testing group %s: package selector template cannot provide obligations or require groups", template.ID)
		}
		for _, other := range contract.Groups {
			for _, id := range other.Requires {
				if id == template.ID {
					return fmt.Errorf("testing group %s requires package selector template %s", other.ID, template.ID)
				}
			}
		}
		for _, surface := range contract.Surfaces {
			for _, id := range append(append([]string{}, surface.Standard...), surface.Deep...) {
				if id == template.ID {
					return fmt.Errorf("testing surface %s references package selector template %s", surface.ID, template.ID)
				}
			}
		}
		for _, id := range append(append(append([]string{}, contract.Always.Canary...), contract.Always.Standard...), append(contract.Unknown, contract.Cadence...)...) {
			if id == template.ID {
				return fmt.Errorf("testing contract references package selector template %s", template.ID)
			}
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
	if group.Resources.Class != "" && group.Resources.Class != "cheap" && group.Resources.Class != "heavy" {
		return fmt.Errorf("resource class must be cheap or heavy")
	}
	seenResources := map[string]bool{}
	for _, resource := range group.Resources.Exclusive {
		if !identifier.MatchString(resource) || seenResources[resource] {
			return fmt.Errorf("exclusive resources require unique ids")
		}
		seenResources[resource] = true
	}
	if group.Resources.Workers != nil && *group.Resources.Workers < 0 {
		return fmt.Errorf("resource workers must be zero or a positive integer")
	}
	if _, reserved := group.Env[TestWorkersEnvironment]; reserved {
		return fmt.Errorf("environment entry %s is reserved by the test runner", TestWorkersEnvironment)
	}
	if group.Freshness != "" && group.Freshness != "reusable" && group.Freshness != "episode" {
		return fmt.Errorf("freshness must be reusable or episode")
	}
	if group.FreshnessMaxAgeMS != nil && (*group.FreshnessMaxAgeMS <= 0 || group.Freshness != "episode") {
		return fmt.Errorf("freshnessMaxAgeMs requires episode freshness and a positive duration")
	}
	if group.Phase != "" && group.Phase != "admission" && group.Phase != "acceptance" {
		return fmt.Errorf("phase must be admission or acceptance")
	}
	if group.EnvironmentMode != "" && group.EnvironmentMode != "inherit" && group.EnvironmentMode != "explicit" {
		return fmt.Errorf("environmentMode must be inherit or explicit")
	}
	if group.EnvironmentMode == "explicit" {
		for name, value := range group.Env {
			if !environmentName.MatchString(name) || strings.ContainsRune(value, '\x00') {
				return fmt.Errorf("explicit environment has invalid entry %q", name)
			}
		}
	}
	seenPrerequisites := map[string]bool{}
	for _, dependency := range group.Requires {
		if !groupIdentifier.MatchString(dependency) {
			return fmt.Errorf("invalid prerequisite %q", dependency)
		}
		if seenPrerequisites[dependency] {
			return fmt.Errorf("duplicate prerequisite %s", dependency)
		}
		seenPrerequisites[dependency] = true
	}
	kinds := map[string]bool{"unit": true, "component": true, "integration": true, "e2e": true, "performance": true, "static": true, "build": true}
	if !kinds[group.Kind] || group.CWD == "" || !validRelative(group.CWD) || len(group.Inputs) == 0 || len(group.Platforms) == 0 || group.TargetMS <= 0 {
		return fmt.Errorf("kind, cwd, platforms, and positive targetMs are required")
	}
	if group.CPUBudgetSeconds != nil && *group.CPUBudgetSeconds <= 0 {
		return fmt.Errorf("cpuBudgetSeconds must be a positive integer when present")
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
		if group.Resources.Workers != nil {
			return fmt.Errorf("resource workers are declared only by command and section adapters")
		}
		seenTags := map[string]bool{}
		for _, tag := range group.BuildTags {
			if !validGoBuildTag(tag) || seenTags[tag] {
				return fmt.Errorf("go buildTags require unique valid Go tag names")
			}
			seenTags[tag] = true
		}
		if group.PackageSelection != "" && group.PackageSelection != "changed-and-consumers" {
			return fmt.Errorf("unknown go packageSelection %q", group.PackageSelection)
		}
		if group.PackageSelection == "changed-and-consumers" && len(group.Packages) != 0 {
			return fmt.Errorf("changed-and-consumers packageSelection forbids explicit packages")
		}
		if group.PackageSelection == "" && len(group.Packages) == 0 || len(group.Tests) == 0 {
			return fmt.Errorf("go adapter requires packages and tests")
		}
		allTests, _, err := GoTests(group)
		if err != nil {
			return err
		}
		if group.PackageSelection != "" && !allTests {
			return fmt.Errorf("changed-and-consumers packageSelection requires tests=all")
		}
		if group.Coverage && !allTests {
			return fmt.Errorf("whole-package coverage requires tests=all")
		}
		if group.Shards < 0 || group.Shards > 64 {
			return fmt.Errorf("go shards must be 0 through 64")
		}
	case "section":
		if group.PackageSelection != "" {
			return fmt.Errorf("packageSelection requires go adapter")
		}
		if len(group.BuildTags) != 0 {
			return fmt.Errorf("buildTags require go adapter")
		}
		if !identifier.MatchString(group.Section) {
			return fmt.Errorf("section adapter requires one stable section id")
		}
	case "command":
		if group.PackageSelection != "" {
			return fmt.Errorf("packageSelection requires go adapter")
		}
		if len(group.BuildTags) != 0 {
			return fmt.Errorf("buildTags require go adapter")
		}
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
	a, errA := pathpattern.Parse(left)
	b, errB := pathpattern.Parse(right)
	return errA == nil && errB == nil && a.Overlaps(b)
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

// Match go/build/constraint's tag operand rule, which also permits release
// tags such as go1.24 and architecture tags such as 386.
func validGoBuildTag(tag string) bool {
	if tag == "" {
		return false
	}
	for _, character := range tag {
		if !unicode.IsLetter(character) && !unicode.IsDigit(character) && character != '_' && character != '.' {
			return false
		}
	}
	return true
}

func validPathDeclaration(path string) bool {
	_, err := pathpattern.Parse(path)
	return err == nil
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
