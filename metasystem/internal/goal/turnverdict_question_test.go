package goal

import (
	"strings"
	"testing"
)

// questionSeat is the session the question tests stop as.
var questionSeat = TurnVerdictOptions{SeatActor: Actor{Machine: "bed-m1", Lineage: "seat-lineage"}, SeatClaimEpoch: 7}

func openQuestions(questions ...OpenQuestion) func() []OpenQuestion {
	return func() []OpenQuestion { return append([]OpenQuestion(nil), questions...) }
}

func noIdleEscalation(t *testing.T, store *Store) {
	t.Helper()
	store.PrepareIdleContinuation = func(event IdleEscalationEvent) (string, error) {
		t.Errorf("a seat waiting on its question was handed a continuation: %+v", event)
		return "", nil
	}
	store.RecordIdleIncident = func(event IdleEscalationEvent) (string, error) {
		t.Errorf("a seat waiting on its question was recorded idle: %+v", event)
		return "", nil
	}
}

// stops runs n Stops of one session and returns each verdict.
func stops(t *testing.T, store *Store, scan ScanResult, session string, n int) []Verdict {
	t.Helper()
	var verdicts []Verdict
	for stop := 1; stop <= n; stop++ {
		verdict, err := store.TurnVerdict(scan, session, "", "main-1", questionSeat)
		if err != nil {
			t.Fatalf("stop %d: %v", stop, err)
		}
		verdicts = append(verdicts, verdict)
	}
	return verdicts
}

// An open question this session asked lets its turn end over claimable
// backlog, and the Stop says what it waits on. A question another session
// asked, or none, leaves the idle refusal as it is.
func TestQuestionPendingAllowsStopWithClaimableBacklog(t *testing.T) {
	t.Parallel()
	files := func() map[string]*GoalFile {
		first := budgetedQueuedGoal("ready-first", "2026-08-23T00:00:00Z")
		second := budgetedQueuedGoal("ready-second", "2026-08-23T00:00:01Z")
		return map[string]*GoalFile{first.Id: first, second.Id: second}
	}
	mine := OpenQuestion{ID: "q-lane", About: "lane", Machine: "bed-m1", Lineage: "seat-lineage"}

	store, _, _ := fakeServingFixture(t, "bed-m1", files())
	store.OpenQuestions = openQuestions(mine)
	noIdleEscalation(t, store)
	for stop, verdict := range stops(t, store, ScanResult{}, "asked-session", 3) {
		if verdict.ShouldBlock || verdict.IdleRefusal || !strings.Contains(verdict.Display, "WAITING: question q-lane") {
			t.Fatalf("stop %d with this session's question open: %+v", stop+1, verdict)
		}
	}

	for _, leg := range []struct {
		name      string
		questions []OpenQuestion
	}{
		{"another lineage asked", []OpenQuestion{{ID: "q-other", About: "lane", Machine: "bed-m1", Lineage: "another-lineage"}}},
		{"another machine asked", []OpenQuestion{{ID: "q-other", About: "lane", Machine: "bed-m2", Lineage: "seat-lineage"}}},
		{"answered or withdrawn", nil},
	} {
		t.Run(leg.name, func(t *testing.T) {
			t.Parallel()
			store, _, _ := fakeServingFixture(t, "bed-m1", files())
			store.OpenQuestions = openQuestions(leg.questions...)
			first := stops(t, store, ScanResult{}, "control-session", 1)[0]
			if !first.ShouldBlock || !first.IdleRefusal || strings.Contains(first.Display, "WAITING: question") {
				t.Fatalf("the idle refusal stands: %+v", first)
			}
		})
	}
}

// The claimed goal's new revision (an ask changes it) refuses the Stop
// once; with this session's question open, it does not.
func TestQuestionPendingAllowsStopOnTheClaimedGoalsNewRevision(t *testing.T) {
	t.Parallel()
	claimed := func() map[string]*GoalFile {
		live := &GoalFile{
			Id: "working-b", State: StateClaimed, Intent: "Carry the live work", Origin: OriginMain,
			NextStep: "ASKED q-goal: which store?", OpenedAt: "2026-08-23T00:03:00Z", Revision: 3,
			Claimed: &ClaimRecord{Machine: "bed-m1", Lineage: "seat-lineage", At: "2026-08-23T01:03:00Z", Revision: 2, AccountingRevision: 2},
		}
		return map[string]*GoalFile{live.Id: live}
	}
	control, _, _ := fakeServingFixture(t, "bed-m1", claimed())
	if first := stops(t, control, ScanResult{}, "control-session", 1)[0]; !first.ShouldBlock {
		t.Fatalf("the claimed goal's revision refuses the first Stop: %+v", first)
	}
	store, _, _ := fakeServingFixture(t, "bed-m1", claimed())
	store.OpenQuestions = openQuestions(OpenQuestion{ID: "q-goal", Goal: "working-b", Machine: "bed-m1", Lineage: "seat-lineage"})
	if first := stops(t, store, ScanResult{}, "asked-session", 1)[0]; first.ShouldBlock || !strings.Contains(first.Display, "WAITING: question q-goal") {
		t.Fatalf("with the question open: %+v", first)
	}
}

// Open plan work refuses the Stop once per line; with this session's
// question open, it does not.
func TestQuestionPendingAllowsStopWithOpenPlanWork(t *testing.T) {
	t.Parallel()
	scan := ScanResult{Open: []Item{{Kind: "plan", Id: "open", Detail: "OPEN-WORK open: finish it"}}}
	control, _, _ := fakeServingFixture(t, "bed-m1", map[string]*GoalFile{})
	if first := stops(t, control, scan, "control-session", 1)[0]; !first.ShouldBlock {
		t.Fatalf("open plan work refuses the first Stop: %+v", first)
	}
	store, _, _ := fakeServingFixture(t, "bed-m1", map[string]*GoalFile{})
	store.OpenQuestions = openQuestions(OpenQuestion{ID: "q-machine", About: "machine", Machine: "bed-m1", Lineage: "seat-lineage"})
	if first := stops(t, store, scan, "asked-session", 1)[0]; first.ShouldBlock || !strings.Contains(first.Display, "WAITING: question q-machine") {
		t.Fatalf("with the question open: %+v", first)
	}
}

// A wait on a budget-stopped goal is dropped before the Stop judgment sees
// it; the open question the seat asked about that goal still lets the turn
// end while other ready work stands, and nothing is escalated.
func TestQuestionWaitAllowsStopForFencedGoal(t *testing.T) {
	t.Parallel()
	files := func() map[string]*GoalFile {
		fenced := breachStoppedGoalForTest("fenced-a", "bed-m1")
		ready := budgetedQueuedGoal("working-b", "2026-08-23T00:03:00Z")
		return map[string]*GoalFile{fenced.Id: fenced, ready.Id: ready}
	}
	control, _, _ := fakeServingFixture(t, "bed-m1", files())
	if first := stops(t, control, ScanResult{}, "control-session", 1)[0]; !first.ShouldBlock || !first.IdleRefusal {
		t.Fatalf("the ready goal beside the fenced one refuses the Stop: %+v", first)
	}
	store, _, _ := fakeServingFixture(t, "bed-m1", files())
	store.OpenQuestions = openQuestions(OpenQuestion{ID: "q-fenced", Goal: "fenced-a", Machine: "bed-m1", Lineage: "seat-lineage"})
	noIdleEscalation(t, store)
	for stop, verdict := range stops(t, store, ScanResult{}, "fenced-session", 3) {
		if verdict.ShouldBlock || verdict.IdleRefusal || !strings.Contains(verdict.Display, "WAITING: question q-fenced") {
			t.Fatalf("stop %d with the fenced goal's question open: %+v", stop+1, verdict)
		}
	}
}

// A goal an open question names is left to the person by the idle
// continuation, whoever asked; once the question is answered or withdrawn
// it is the continuation's again.
func TestAskedOpenGoalIsLeftByIdleContinuation(t *testing.T) {
	t.Parallel()
	files := func() map[string]*GoalFile {
		asked := budgetedQueuedGoal("asked-first", "2026-08-23T00:00:00Z")
		asked.Priority, asked.Sequence = 1, 1
		ready := budgetedQueuedGoal("ready-second", "2026-08-23T00:00:01Z")
		ready.Priority, ready.Sequence = 1, 2
		return map[string]*GoalFile{asked.Id: asked, ready.Id: ready}
	}
	for _, leg := range []struct {
		name      string
		questions []OpenQuestion
		want      string
	}{
		{"asked open", []OpenQuestion{{ID: "q-first", Goal: "asked-first", Machine: "bed-m2", Lineage: "another-seat"}}, "ready-second"},
		{"answered or withdrawn", nil, "asked-first"},
	} {
		t.Run(leg.name, func(t *testing.T) {
			t.Parallel()
			store, _, _ := fakeServingFixture(t, "bed-m1", files())
			store.OpenQuestions = openQuestions(leg.questions...)
			var prepared IdleEscalationEvent
			store.PrepareIdleContinuation = func(event IdleEscalationEvent) (string, error) {
				prepared = event
				return "intent", nil
			}
			store.RecordIdleIncident = func(IdleEscalationEvent) (string, error) { return "alert", nil }
			verdicts := stops(t, store, ScanResult{}, "idle-session", 3)
			if !verdicts[0].ShouldBlock || prepared.GoalID != leg.want || !prepared.ClaimNeeded {
				t.Fatalf("continuation chose %q, want %q: %+v", prepared.GoalID, leg.want, prepared)
			}
		})
	}
}
