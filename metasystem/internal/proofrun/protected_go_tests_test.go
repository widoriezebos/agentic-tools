package proofrun

import (
	"encoding/json"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestProtectedGoTestsSkipCommandAndJUnitAdapters(t *testing.T) {
	t.Parallel()
	contract := testpolicy.Contract{
		SchemaVersion: 1,
		ProjectRisk:   testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"},
		Surfaces:      []testpolicy.Surface{{ID: "app", Paths: []string{"app/**"}, Standard: []string{"command", "junit"}, Critical: []string{"command", "junit"}}},
		Groups: []testpolicy.Group{
			{ID: "command", Kind: "static", Adapter: "command", CWD: ".", Inputs: []string{"app/**"}, Obligations: []string{"command"}, Platforms: []string{"any"}, TargetMS: 1,
				Argv: []string{"runner"}, Format: "exit-status", Tests: json.RawMessage(`["TestNamedByAnotherFramework"]`)},
			{ID: "junit", Kind: "integration", Adapter: "command", CWD: ".", Inputs: []string{"app/**"}, Outputs: []string{"reports/**"}, Obligations: []string{"junit"}, Platforms: []string{"any"}, TargetMS: 1,
				Argv: []string{"runner"}, Format: "junit-xml", Reports: []string{"reports"},
				ExpectedTests: []testpolicy.ExpectedTest{{Report: "reports/test.xml", Name: "case"}},
				Tests:         json.RawMessage(`["TestNamedByAnotherFramework"]`)},
		},
		Always: testpolicy.Always{Canary: []string{"command"}}, Unknown: []string{"command", "junit"},
	}
	if err := contract.Validate(); err != nil {
		t.Fatalf("command/JUnit contract is invalid: %v", err)
	}
	if err := CheckProtectedGoTests(gittree.Workspace{Dir: t.TempDir()}, "base", "candidate", contract); err != nil {
		t.Fatalf("non-Go adapters reached Go source inspection: %v", err)
	}
}
