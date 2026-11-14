package proofrun

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestNativeDiscoveryPassesOverRequiredSelectors(t *testing.T) {
	t.Parallel()
	executable := testpolicy.Group{ID: "app", Adapter: "go", CWD: ".", Packages: []string{"."}, Tests: json.RawMessage(`["TestPresent"]`)}
	selector := executable
	selector.ID, selector.Tests, selector.RequiredTestFiles = "required", nil, []string{"missing_test.go"}
	packageSelection := executable
	packageSelection.ID, packageSelection.PackageSelection = "changed", "changed-and-consumers"
	contract := testpolicy.Contract{Groups: []testpolicy.Group{executable, selector, packageSelection}}
	if got, want := nativeDiscoveryGroups(contract), []testpolicy.Group{executable}; !reflect.DeepEqual(got, want) {
		t.Fatalf("native discovery groups = %v, want %v", got, want)
	}
}
