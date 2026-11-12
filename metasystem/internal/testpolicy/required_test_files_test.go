package testpolicy_test

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/contractmerge"
)

func requiredSelectorContract() testpolicy.Contract {
	executable := testpolicy.Group{ID: "unit", Kind: "unit", Adapter: "go", CWD: ".", Phase: "acceptance", EnvironmentMode: "inherit", Inputs: []string{"src/**"}, Platforms: []string{"any"}, TargetMS: 1000, Packages: []string{"./src"}, Tests: json.RawMessage(`"all"`)}
	selector := executable
	selector.ID, selector.Tests, selector.RequiredTestFiles = "required", nil, []string{"src/public*_test.go"}
	return testpolicy.Contract{SchemaVersion: 2, ProjectRisk: testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"},
		Groups: []testpolicy.Group{executable, selector}, Surfaces: []testpolicy.Surface{{ID: "app", Paths: []string{"src/**"}, Standard: []string{"unit"}}}, Unknown: []string{"unit"}}
}

func TestRequiredTestFilesContractRules(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name string
		edit func(*testpolicy.Contract)
		want string
	}{
		{"adapter", func(c *testpolicy.Contract) { c.Groups[1].Adapter = "command" }, "requiredTestFiles requires"},
		{"phase", func(c *testpolicy.Contract) { c.Groups[1].Phase = "admission" }, "requiredTestFiles requires"},
		{"tests", func(c *testpolicy.Contract) { c.Groups[1].Tests = json.RawMessage(`"all"`) }, "requiredTestFiles requires"},
		{"packageSelection", func(c *testpolicy.Contract) { c.Groups[1].PackageSelection = "changed-and-consumers" }, "requiredTestFiles requires"},
		{"shards", func(c *testpolicy.Contract) { c.Groups[1].Shards = 2 }, "requiredTestFiles requires"},
		{"coverage", func(c *testpolicy.Contract) { c.Groups[1].Coverage = true }, "requiredTestFiles requires"},
		{"explicit disabled coverage", func(c *testpolicy.Contract) { c.Groups[1].CoverageSet = true }, "requiredTestFiles requires"},
		{"inputs", func(c *testpolicy.Contract) { c.Groups[1].Inputs = nil }, "required"},
		{"invalid pattern", func(c *testpolicy.Contract) { c.Groups[1].RequiredTestFiles = []string{"src/[bad]_test.go"} }, "pattern"},
		{"non-test file", func(c *testpolicy.Contract) { c.Groups[1].RequiredTestFiles = []string{"src/public.go"} }, "pattern"},
		{"recursive pattern", func(c *testpolicy.Contract) { c.Groups[1].RequiredTestFiles = []string{"src/**/public_test.go"} }, "pattern"},
		{"undeclared directory", func(c *testpolicy.Contract) { c.Groups[1].RequiredTestFiles = []string{"other/public_test.go"} }, "pattern"},
		{"no packages", func(c *testpolicy.Contract) { c.Groups[1].Packages = nil }, "pattern"},
		{"always canary", func(c *testpolicy.Contract) { c.Always.Canary = []string{"required"} }, "references required test file selector"},
		{"always standard", func(c *testpolicy.Contract) { c.Always.Standard = []string{"required"} }, "references required test file selector"},
		{"unknown", func(c *testpolicy.Contract) { c.Unknown = []string{"required"} }, "references required test file selector"},
		{"cadence", func(c *testpolicy.Contract) { c.Cadence = []string{"required"} }, "references required test file selector"},
		{"surface standard", func(c *testpolicy.Contract) { c.Surfaces[0].Standard = []string{"required"} }, "references required test file selector"},
		{"surface deep", func(c *testpolicy.Contract) { c.Surfaces[0].Deep = []string{"required"} }, "references required test file selector"},
		{"prerequisite", func(c *testpolicy.Contract) { c.Groups[0].Requires = []string{"required"} }, "requires required test file selector"},
		{"executable without tests", func(c *testpolicy.Contract) { c.Groups[0].Tests = nil }, "go adapter requires packages and tests"},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			c := requiredSelectorContract()
			row.edit(&c)
			if err := c.Validate(); err == nil || !strings.Contains(err.Error(), row.want) {
				t.Fatalf("want %q; got %v", row.want, err)
			}
		})
	}
	c := requiredSelectorContract()
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	encoded, err := contractmerge.Render(c)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := testpolicy.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	again, err := contractmerge.Render(decoded)
	if err != nil || !bytes.Equal(encoded, again) {
		t.Fatalf("selector round trip changed bytes: %v\n%s", err, again)
	}
	candidate := requiredSelectorContract()
	candidate.Groups[1].RequiredTestFiles = []string{"src/new_test.go"}
	protected := testpolicy.ProtectedContract(c, candidate)
	if err := protected.Validate(); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(protected.Groups[1].RequiredTestFiles, []string{"src/public*_test.go", "src/new_test.go"}) {
		t.Fatalf("protected files: %v", protected.Groups[1].RequiredTestFiles)
	}
	protected.Groups[1].RequiredTestFiles[0] = "changed"
	if c.Groups[1].RequiredTestFiles[0] != "src/public*_test.go" || candidate.Groups[1].RequiredTestFiles[0] != "src/new_test.go" {
		t.Fatal("protected files alias input")
	}
	for _, request := range []testpolicy.SelectionRequest{{Groups: []string{"required"}}, {BatchRequirements: []string{"required"}}} {
		_, err := testpolicy.Select(c, request)
		if err == nil || err.Error() != "requested group required selects required test files; it is not run as a group" {
			t.Fatalf("selector accepted as group: %v", err)
		}
	}
}
