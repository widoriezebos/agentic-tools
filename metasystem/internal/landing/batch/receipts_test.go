package batch

import (
	"encoding/json"
	"slices"
	"testing"
)

func TestPrefixReceiptPlainJSONCompatibility(t *testing.T) {
	plain := PrefixReceipt{GoalID: "goal-a", Tree: "tree-a", AttemptID: "attempt-a", ResultPath: "result-a"}
	data, err := json.Marshal(plain)
	must(t, err)
	want := `{"GoalID":"goal-a","Tree":"tree-a","AttemptID":"attempt-a","ResultPath":"result-a","Reused":null,"Executed":null}`
	if string(data) != want {
		t.Fatalf("plain receipt JSON=%s, want pre-branch bytes %s", data, want)
	}
	branch := PrefixReceipt{GoalID: "goal-b", CommitIDs: []string{"commit-a", "commit-b"}, Units: []string{"10a", "10b"}, LastUnit: "10b"}
	encoded, err := json.Marshal(branch)
	must(t, err)
	var roundTrip PrefixReceipt
	must(t, json.Unmarshal(encoded, &roundTrip))
	if !slices.Equal(roundTrip.CommitIDs, branch.CommitIDs) || !slices.Equal(roundTrip.Units, branch.Units) || roundTrip.LastUnit != branch.LastUnit {
		t.Fatalf("branch receipt did not round-trip: encoded=%s receipt=%+v", encoded, roundTrip)
	}
}
