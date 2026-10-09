package goal

import (
	"reflect"
	"strings"
	"testing"
)

func TestRecordUnitDropRebasePreservesScope(t *testing.T) {
	t.Parallel()
	for _, restored := range []bool{false, true} {
		t.Run(map[bool]string{false: "excluded", true: "restored"}[restored], func(t *testing.T) {
			t.Parallel()
			file := vGoal("drop-scope", StateClaimed)
			before := UnitDrop{Unit: "U", Operation: "drop-U", Loop: "read-U", Subject: "subject-U", Attempt: 1,
				Revision: file.Revision, Covered: []string{strings.Repeat("a", 40)}, Findings: []string{"read-U:1"},
				Commit: strings.Repeat("b", 40), Tree: strings.Repeat("c", 40), Proof: "drop-proof",
				Decisions: strings.Repeat("d", 64), Requirements: strings.Repeat("e", 64), Actor: "Wido",
				Reason: "Remove required scope", Impact: "Required behavior will be missing", At: file.Claimed.At}
			scope := ScopeExclusion{Unit: before.Unit, Operation: before.Operation, Requirements: before.Requirements,
				Result: before.Commit, Proof: before.Proof, Actor: before.Actor, Authority: "SIGNED_IN_SESSION",
				Reason: before.Reason, Impact: before.Impact, At: before.At, Designs: []string{strings.Repeat("f", 64)}}
			if restored {
				scope.RestoredAt, scope.RestoredBy = "2026-08-20T11:00:00Z", "Wido"
			}
			file.UnitDrops, file.ScopeExclusions = []UnitDrop{before}, []ScopeExclusion{scope}
			endpoint, _ := fakeGoalEndpoint(t, file)
			request := verbReqFor(endpoint, "01J5X00000000000000000DS01", file.Claimed.Machine)
			after := before
			after.Commit, after.Tree, after.Proof, after.Covered = strings.Repeat("1", 40), strings.Repeat("2", 40), "rebased-proof", []string{strings.Repeat("3", 40)}
			result, err := RecordUnitDrop(request, file.Id, after, nil, before)
			if err != nil || result.Outcome != OutcomeConfirmed {
				t.Fatalf("rebase publication: %+v %v", result, err)
			}
			tree, err := loadTreeFor(endpoint, result.Tip)
			if err != nil {
				t.Fatal(err)
			}
			current := tree.Live[file.Id]
			want := scope
			want.Result, want.Proof = after.Commit, after.Proof
			if len(current.ScopeExclusions) != 1 || !reflect.DeepEqual(current.ScopeExclusions[0], want) || current.ExcludesScope(before.Unit, "") == restored {
				t.Fatalf("rebase changed scope authority or restoration: %+v", current.ScopeExclusions)
			}
			result, err = RecordUnitDrop(request, file.Id, after, nil, before)
			if err != nil || result.Outcome != OutcomeConfirmed {
				t.Fatalf("replayed publication: %+v %v", result, err)
			}
		})
	}
}
