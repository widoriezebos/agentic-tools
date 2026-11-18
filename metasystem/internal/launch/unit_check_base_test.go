package launch

import (
	"strings"
	"testing"
)

func TestUnitCheckBaseOverridesInheritedValueOnlyForCheap(t *testing.T) {
	t.Parallel()
	environment := []string{"LANDING_PROOF_BASE=stale"}
	check := UnitCheck{Base: "round-parent", Directory: t.TempDir(), DeclaredEnv: environment, Minutes: 1,
		Cheap: `printf '%s' "$LANDING_PROOF_BASE"`, Audits: `printf '%s' "$LANDING_PROOF_BASE"`}
	_, exits, err := check.Run(check.Directory, t.TempDir(), CheckContext{})
	if err != nil || len(exits) != 2 || exits[0].Output != "round-parent" || exits[1].Output != "stale" {
		t.Fatalf("base environment: %+v, %v", exits, err)
	}
	if environment[len(environment)-1] != "LANDING_PROOF_BASE=stale" || strings.Join(check.DeclaredEnv, "\n") != strings.Join(environment, "\n") {
		t.Fatal("the frozen environment was changed")
	}
}
