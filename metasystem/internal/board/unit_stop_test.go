package board

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestBoardRetainsAndDisplaysReviewStop(t *testing.T) {
	t.Parallel()
	seat := seatOf("m1b")
	stop := &ReviewStop{Decision: "stop", Handoff: "split successor", Class: "repeated regression at source.go", Attempt: 2, Budget: 3}
	view := NewView([]Seat{seat}, Picture{Cards: []Card{{Seat: seat, Goal: "goal-x", Stage: StageJudgement, Stop: stop}}})
	bytes, err := json.Marshal(view)
	if err != nil || !strings.Contains(string(bytes), `"handoff":"split successor"`) {
		t.Fatalf("stop projection %s %v", bytes, err)
	}
	line, ok := view.GoalLine("goal-x", t0, time.UTC)
	if !ok || !strings.Contains(line, "review stop at attempt 2 of 3") || !strings.Contains(line, "split successor") || !strings.Contains(line, "repeated regression") {
		t.Fatalf("stop display %q", line)
	}
}
