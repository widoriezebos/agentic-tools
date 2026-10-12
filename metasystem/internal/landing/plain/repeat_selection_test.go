package plain

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestRepeatUsesOnlySelectablePackageTests(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, unit string
		tests      []string
		want       string
	}{
		{"package", "metasystem/internal/p", []string{"TestP", "TestOther"}, "TestP TestOther"},
		{"missing report", "metasystem/internal/p", []string{"TestP(did not report)"}, ""},
		{"subtest", "metasystem/internal/p", []string{"TestP/child"}, ""},
		{"main", "metasystem/internal/p", []string{"TestMain"}, ""},
		{"mixed", "metasystem/internal/p", []string{"TestP", "TestOther(did not report)"}, ""},
		{"non-test", "metasystem/internal/p", []string{"ExampleP"}, ""},
		{"no names", "metasystem/internal/p", nil, ""},
		{"command group", "fast-static-build", []string{"TestP"}, ""},
		{"go group", "go-batchtest", []string{"TestP"}, ""},
		{"section", "section/check", []string{"TestP"}, ""},
		{"contract group with slash", "custom/check", []string{"TestP"}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b := newRepeatBed(t)
			if err := os.MkdirAll(b.install, 0755); err != nil {
				t.Fatal(err)
			}
			// A small contract shape distinguishes a slash-bearing group from a package.
			if err := os.WriteFile(filepath.Join(b.install, "testing.json"), []byte(`{
				"schemaVersion":2,
				"projectRisk":{"severity":1,"exposure":1,"reversibility":"revert","detection":"immediate","recovery":"bounded"},
				"surfaces":[{"id":"app","paths":["metasystem/internal/p/**"],"standard":["custom/check"]}],
				"groups":[{"id":"custom/check","kind":"unit","adapter":"command","cwd":".","phase":"acceptance","environmentMode":"inherit","inputs":["metasystem/internal/p/**"],"platforms":["any"],"targetMs":1,"argv":["app-tests"],"format":"exit-status"}],
				"unknown":["custom/check"]
			}`), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := testpolicy.Load(filepath.Join(b.install, "testing.json")); err != nil {
				t.Fatal(err)
			}
			calls := 0
			b.seams.Command = func(cmd *exec.Cmd) error {
				calls++
				if got := commandEnv(cmd, "LANDING_ONLY_TESTS"); got != tc.want || commandEnv(cmd, "LANDING_ONLY") != tc.unit {
					t.Fatalf("repeat unit=%q names=%q; want unit=%q names=%q", commandEnv(cmd, "LANDING_ONLY"), got, tc.unit, tc.want)
				}
				fmt.Fprint(cmd.Stdout, "LANDING-CHECKED\t0\n")
				return nil
			}
			unit := FailedUnit{Unit: tc.unit, Tests: tc.tests}
			red := Result{Attempt: "selection", Tree: "tree", Result: Red, Failed: []FailedUnit{unit}, Cause: &Cause{Kind: "unclassified"}}
			repeatRed(b.seams, b.install, "fixture", b.install, Running{Attempt: red.Attempt}, scopeDecision{}, red, true)
			if calls != 1 {
				t.Fatalf("repeat ran %d times", calls)
			}
		})
	}
}

// This test certifies the live contract and goal ledger, not an isolated install.
func TestRepeatSelectionRealContract(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..", "..")
	contract, err := testpolicy.Load(filepath.Join(root, "testing.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(contract.Groups) == 0 {
		t.Fatal("the real contract has no groups")
	}
	for _, group := range contract.Groups {
		names, invalid := namedUnitTests(FailedUnit{Unit: group.ID, Tests: []string{"TestP"}}, contract)
		if len(names) != 0 || invalid {
			t.Fatalf("contract group %s selected package tests: %v invalid=%t", group.ID, names, invalid)
		}
	}
	unit := FailedUnit{Unit: "metasystem/internal/landing/plain", Tests: []string{"TestRepeatSelectionRealContract"}}
	if names, invalid := namedUnitTests(unit, contract); invalid || !reflect.DeepEqual(names, unit.Tests) {
		t.Fatalf("real package rejected: %v invalid=%t", names, invalid)
	}
	unit.Tests = []string{"TestP(did not report)"}
	if names, invalid := namedUnitTests(unit, contract); len(names) != 0 || !invalid {
		t.Fatalf("missing report selected: %v invalid=%t", names, invalid)
	}
	t.Logf("real contract: %d groups kept whole", len(contract.Groups))
}
