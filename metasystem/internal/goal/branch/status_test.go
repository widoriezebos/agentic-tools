package branch_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
)

func readUnit(t *testing.T, f *branchFixture, unit, commit string) {
	t.Helper()
	digest, err := branch.UnitDigest(f.root, commit)
	if err != nil {
		t.Fatal(err)
	}
	record := fmt.Sprintf("metasystem/records/misc/goal-a-%s-read.md", unit)
	write(t, f.root, record, "Read commit "+commit+" with unit digest "+digest+" and found it clean.\n")
	if _, _, err := branch.CommitRead(branch.CommitReadRequest{
		Repo: f.root, Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", Units: strings.Split(unit, "+"),
		OpID: "read-" + unit, CheckClaim: claimAllowed, ReaderRecord: record,
		GateRunID: "fast-" + unit, GateTree: unitTree(t, f, commit),
	}); err != nil {
		t.Fatal(err)
	}
}
