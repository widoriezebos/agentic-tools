package goal

import (
	"strings"
	"testing"
)

func TestRetroKeepsConcludedNonMaterialNotes(t *testing.T) {
	t.Parallel()
	nonMaterial := false
	file := vGoal("done-notes", StateDone)
	file.ReadItems = []ReadItem{{ID: "critic-1", Read: "critic", Text: "Follow up.", Material: &nonMaterial, State: ReadItemOpen, AddedAt: "2026-09-17T10:00:00Z"}}
	endpoint, _ := fakeGoalEndpoint(t, file)
	lines, present, err := readItemsForRetro(endpoint)
	got := strings.Join(lines, "\n")
	if err != nil || !present || !strings.Contains(got, "goal=done-notes state=done open=1") ||
		!strings.Contains(got, "item goal=done-notes read=critic id=critic-1") || strings.Contains(got, "LEDGER DEFECT") {
		t.Fatalf("retro lost a concluded goal's non-material note: present=%t err=%v lines=%s", present, err, got)
	}
}
