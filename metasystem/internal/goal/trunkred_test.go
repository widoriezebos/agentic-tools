package goal

import (
	"encoding/json"
	"testing"
)

func TestTrunkRedBranchReferenceRoundTripsAndValidates(t *testing.T) {
	branch := TrunkRedBranch{Name: "fix/red", Commit: "abc123", State: TrunkRedBranchOpen}
	data, err := json.Marshal(branch)
	if err != nil || string(data) != `{"name":"fix/red","commit":"abc123","state":"open"}` {
		t.Fatalf("marshal branch: %s, %v", data, err)
	}
	var decoded TrunkRedBranch
	if err := json.Unmarshal(data, &decoded); err != nil || decoded != branch || decoded.Validate() != nil {
		t.Fatalf("round trip: %+v, %v", decoded, err)
	}
	for _, invalid := range []TrunkRedBranch{{Commit: "abc"}, {State: TrunkRedBranchOpen}, {Name: "fix", Commit: "abc"}, {Name: "fix", State: "unknown"}} {
		if invalid.Validate() == nil {
			t.Fatalf("accepted invalid branch %+v", invalid)
		}
	}
	if err := (TrunkRedBranch{}).Validate(); err != nil {
		t.Fatalf("empty branch: %v", err)
	}
}
func TestTrunkRedVerdictSplitsByOwnerMachine(t *testing.T) {
	closed := &TrunkRedClosure{At: "closed"}
	tree := &TreeGoals{Live: map[string]*GoalFile{}, TrunkRed: []TrunkRedEntry{
		{ID: "mine", Owner: TrunkRedOwner{Machine: "m1"}},
		{ID: "theirs", Owner: TrunkRedOwner{Machine: "m2"}},
		{ID: "closed", Owner: TrunkRedOwner{Machine: "m1"}, Closed: closed},
	}}
	verdict, err := Next(Projection{Root: t.TempDir(), Tree: tree}, "m1")
	if err != nil || len(verdict.TrunkRedOwned) != 1 || verdict.TrunkRedOwned[0].ID != "mine" || len(verdict.TrunkRedElsewhere) != 1 || verdict.TrunkRedElsewhere[0].ID != "theirs" {
		t.Fatalf("split: %+v, %v", verdict, err)
	}
}
func TestTrunkRedDoesNotChangeSelection(t *testing.T) {
	frontier := NextVerdict{Ready: []string{"ready"}}
	want := SelectNext(frontier)
	frontier.TrunkRedOwned = []TrunkRedEntry{{ID: "red"}}
	if got := SelectNext(frontier); got != want {
		t.Fatalf("selection changed from %+v to %+v", want, got)
	}
}
