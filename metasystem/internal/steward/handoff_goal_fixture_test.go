package steward

import (
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

type handoffGoalFixture struct {
	t     *testing.T
	root  string
	state string
	file  *goal.GoalFile
}

func newHandoffGoalFixture(t *testing.T, state string) *handoffGoalFixture {
	t.Helper()
	root := t.TempDir()
	installHandoffFixtureFiles(t, root)
	canonical, err := canonicalExistingPath(root)
	if err != nil {
		t.Fatal(err)
	}
	fixture := &handoffGoalFixture{t: t, root: canonical, state: state}
	if state != "none" {
		fixture.file = capturedGoal(state)
	}
	return fixture
}

func (fixture *handoffGoalFixture) handoff(root string, caller HandoffCaller, record HandoffRecord, now time.Time, receiptFile string) (HandoffResult, error) {
	return fixture.invoke(root, caller, record, now, receiptFile, true)
}

func (fixture *handoffGoalFixture) handoffBeforeGoal(root string, caller HandoffCaller, record HandoffRecord, now time.Time, receiptFile string) (HandoffResult, error) {
	return fixture.invoke(root, caller, record, now, receiptFile, false)
}

func (fixture *handoffGoalFixture) invoke(root string, caller HandoffCaller, record HandoffRecord, now time.Time, receiptFile string, expectRead bool) (HandoffResult, error) {
	fixture.t.Helper()
	reads := 0
	result, err := handoffWithGoalReader(root, caller, record, now, receiptFile, func(gotRoot string, gotNow time.Time) (handoffGoalSnapshot, error) {
		reads++
		if gotRoot != fixture.root || !gotNow.Equal(now) {
			fixture.t.Errorf("goal reader received root=%q clock=%s; want root=%q clock=%s", gotRoot, gotNow, fixture.root, now)
		}
		snapshot := handoffGoalSnapshot{accepted: make(map[string]*goal.GoalFile)}
		if fixture.file != nil {
			id := fixture.file.Id
			snapshot.accepted[id] = fixture.file
			if fixture.state == "landing" {
				snapshot.landing = []string{id}
			} else {
				snapshot.claimed = []string{id}
			}
		}
		return snapshot, nil
	})
	want := 1
	if !expectRead {
		want = 0
	}
	if reads != want {
		fixture.t.Errorf("goal reader calls=%d, want %d", reads, want)
	}
	return result, err
}
