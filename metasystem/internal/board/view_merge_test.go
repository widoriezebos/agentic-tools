package board

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestGoalViewRetainsReservationAndReviewStop(t *testing.T) {
	t.Parallel()
	seat := seatOf("m1b")
	stop := &ReviewStop{Decision: "stop", Handoff: "split successor", Class: "repeated regression", Attempt: 2, Budget: 3}
	view := NewView([]Seat{seat}, Picture{Cards: []Card{{Seat: seat, Goal: "goal", Stage: StageJudgement, Stop: stop}}})
	view.Seats[0].Goals[0].Reserved = true
	encoded, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	var stored View
	if err := json.Unmarshal(encoded, &stored); err != nil {
		t.Fatal(err)
	}
	got := stored.Seats[0].Goals[0]
	if !got.Reserved || !reflect.DeepEqual(got.Stop, stop) {
		t.Fatalf("view lost reservation or stop: %s", encoded)
	}
}
