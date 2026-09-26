package trunkredmap

import (
	"reflect"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

func TestProofResultReachesLedgerAsTrunkRedRecords(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		groups   []proofrun.GroupResult
		want     []goal.TrunkRedRecordGroup
		manifest [][]string
	}{
		{name: "green run records nothing", groups: []proofrun.GroupResult{
			{ID: "go/unit", Status: "passed", Observed: []proofrun.NativeTestIdentity{{Name: "TestA", Status: "passed"}}},
			{ID: "go/race", Status: "reused", Observed: []proofrun.NativeTestIdentity{{Name: "TestB", Status: "failed"}}},
		}, want: []goal.TrunkRedRecordGroup{}},
		{name: "failed and not-run groups carry their evidence", groups: []proofrun.GroupResult{
			{ID: "go/unit", Status: "passed", Observed: []proofrun.NativeTestIdentity{{Name: "TestGreen", Status: "failed"}}},
			{ID: "go/landing", Status: "failed", LogPath: "logs/landing.log", LogDigest: "sha256:aa",
				InputManifest: []string{"a.go", "b.go"},
				Observed: []proofrun.NativeTestIdentity{
					{Report: "r2.xml", Classname: "pkg/b", Name: "TestB", Status: "failed", Reason: "boom"},
					{Report: "r1.xml", Classname: "pkg/a", Name: "TestPass", Status: "passed"},
					{Report: "r1.xml", Classname: "pkg/a", Name: "TestA", Status: "failed", Reason: "want 1"},
					{Report: "r1.xml", Classname: "pkg/a", Name: "TestSkip", Status: "skipped", Reason: "short"},
				}},
			{ID: "go/deep", Status: "skipped", NotRunReason: "budget exhausted", LogPath: "logs/deep.log", LogDigest: "sha256:bb",
				Observed: []proofrun.NativeTestIdentity{{Name: "TestOnlyPassed", Status: "passed"}}},
		}, want: []goal.TrunkRedRecordGroup{
			{Group: "go/landing", Status: "failed", LogPath: "logs/landing.log", LogDigest: "sha256:aa", Failures: []goal.TrunkRedFailure{
				{Report: "r2.xml", Classname: "pkg/b", Name: "TestB", Status: "failed", Reason: "boom"},
				{Report: "r1.xml", Classname: "pkg/a", Name: "TestA", Status: "failed", Reason: "want 1"},
			}},
			{Group: "go/deep", Status: "skipped", NotRunReason: "budget exhausted", LogPath: "logs/deep.log", LogDigest: "sha256:bb",
				Failures: []goal.TrunkRedFailure{}},
		}, manifest: [][]string{{"a.go", "b.go"}, nil}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			red := ResultToRedGroups(proofrun.TestResult{Groups: tc.groups})
			records := RedGroupsToRecordGroups(red)
			if records == nil || len(records) != len(tc.want) {
				t.Fatalf("records = %#v, want %d groups", records, len(tc.want))
			}
			for i, record := range records {
				want := tc.want[i]
				identity := record.Identity
				record.Identity = ""
				if !reflect.DeepEqual(record, want) {
					t.Fatalf("record %d = %#v, want %#v", i, record, want)
				}
				if !strings.HasPrefix(identity, "tr-"+strings.ReplaceAll(want.Group, "/", "-")+"-") {
					t.Fatalf("record %d identity %q does not name group %q", i, identity, want.Group)
				}
				if !reflect.DeepEqual(red[i].InputManifest, tc.manifest[i]) {
					t.Fatalf("red %d manifest = %#v, want %#v", i, red[i].InputManifest, tc.manifest[i])
				}
			}
			for _, group := range tc.groups {
				for i := range group.InputManifest {
					group.InputManifest[i] = "mutated.go"
				}
			}
			for i := range red {
				if !reflect.DeepEqual(red[i].InputManifest, tc.manifest[i]) {
					t.Fatalf("red %d manifest aliases the proof result: %#v", i, red[i].InputManifest)
				}
			}
		})
	}
}

// A failed group's ledger identity names its failing tests, not their order,
// reasons, or log location; a not-run group's identity names its status.
func TestTrunkRedRecordIdentityFollowsFailingTests(t *testing.T) {
	t.Parallel()
	failing := func(logPath string, observed ...proofrun.NativeTestIdentity) string {
		groups := []proofrun.GroupResult{{ID: "go/landing", Status: "failed", LogPath: logPath, Observed: observed}}
		return RedGroupsToRecordGroups(ResultToRedGroups(proofrun.TestResult{Groups: groups}))[0].Identity
	}
	a := proofrun.NativeTestIdentity{Report: "r", Classname: "pkg", Name: "TestA", Status: "failed", Reason: "x"}
	b := proofrun.NativeTestIdentity{Report: "r", Classname: "pkg", Name: "TestB", Status: "failed", Reason: "y"}
	pass := proofrun.NativeTestIdentity{Report: "r", Classname: "pkg", Name: "TestC", Status: "passed"}
	base := failing("one.log", a, b)
	aOther := a
	aOther.Reason = "different"
	if got := failing("two.log", b, pass, aOther); got != base {
		t.Fatalf("identity changed with order/reason/log/passing tests: %q != %q", got, base)
	}
	if got := failing("one.log", a); got == base {
		t.Fatalf("identity %q ignored a dropped failing test", got)
	}
	notRun := func(status string) string {
		groups := []proofrun.GroupResult{{ID: "go/landing", Status: status}}
		return RedGroupsToRecordGroups(ResultToRedGroups(proofrun.TestResult{Groups: groups}))[0].Identity
	}
	if notRun("skipped") == notRun("blocked") || notRun("skipped") == base {
		t.Fatal("not-run identity does not distinguish its status from other red outcomes")
	}
}
