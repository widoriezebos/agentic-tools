package seat

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestTheVerdictPrecedenceCaseByCase(t *testing.T) {
	t.Parallel()
	window := 30 * time.Minute
	fresh := FormatTime(fixtureClock.Add(-2 * time.Minute))
	stale := FormatTime(fixtureClock.Add(-2 * time.Hour))

	cases := []struct {
		name       string
		state      PublicationState
		readable   bool
		readErr    error
		wantStatus string
		wantReason string
	}{
		{
			name: "an unreadable publication state", readErr: fmt.Errorf("torn"),
			wantStatus: StatusUnknown, wantReason: "unreadable",
		},
		{
			name: "no attempt since arming", readable: false,
			wantStatus: StatusAlive, wantReason: "first publish pending",
		},
		{
			name:     "a skip for no nickname",
			state:    PublicationState{LastAttemptAt: fresh, LastOutcome: OutcomeSkipped + ": " + SkipNoNickname},
			readable: true, wantStatus: StatusAlive, wantReason: "publishes no presence: no machine nickname",
		},
		{
			name: "a conflict",
			state: PublicationState{LastAttemptAt: fresh, LastSuccessAt: fresh, TickSeconds: 600,
				LastOutcome: OutcomeSkipped + ": " + SkipConflict + ": two checkouts"},
			readable: true, wantStatus: StatusDead, wantReason: SkipConflict,
		},
		{
			name: "a last success older than the threshold",
			state: PublicationState{LastAttemptAt: fresh, LastSuccessAt: stale, TickSeconds: 600,
				LastOutcome: OutcomeFailed + ": the remote refused", Detail: "the remote refused"},
			readable: true, wantStatus: StatusDead, wantReason: "presence not published since " + stale,
		},
		{
			name: "a fresh success",
			state: PublicationState{LastAttemptAt: fresh, LastSuccessAt: fresh, TickSeconds: 600,
				LastOutcome: OutcomePublished, Rung: 2},
			readable: true, wantStatus: StatusAlive, wantReason: "presence published 2 min ago on rung 2, a branch per machine",
		},
		{
			name: "a failed newest attempt over a still-fresh success",
			state: PublicationState{LastAttemptAt: FormatTime(fixtureClock), LastSuccessAt: fresh, TickSeconds: 600,
				LastOutcome: OutcomeFailed + ": the remote hiccupped", Detail: "the remote hiccupped", Rung: 1},
			readable: true, wantStatus: StatusAlive, wantReason: "newest attempt failed: the remote hiccupped",
		},
		{
			name:     "an unarmed skip with no success behind it",
			state:    PublicationState{LastAttemptAt: fresh, LastOutcome: OutcomeSkipped + ": " + SkipUnarmed, Detail: SkipUnarmed},
			readable: true, wantStatus: StatusDead, wantReason: "presence not published since arming",
		},
	}
	for _, test := range cases {
		verdict := Health(test.state, test.readable, test.readErr, fixtureClock, window)
		if verdict.Status != test.wantStatus || !strings.Contains(verdict.Reason, test.wantReason) {
			t.Errorf("%s = %s (%q); want %s containing %q", test.name, verdict.Status, verdict.Reason, test.wantStatus, test.wantReason)
		}
	}
}

func TestADeadVerdictCarriesTheRemedyThatFitsIt(t *testing.T) {
	t.Parallel()
	conflict := PublicationState{LastAttemptAt: FormatTime(fixtureClock), TickSeconds: 600,
		LastOutcome: OutcomeSkipped + ": " + SkipConflict + ": another checkout"}
	if verdict := Health(conflict, true, nil, fixtureClock, 30*time.Minute); verdict.Remedy != RemedyOneCheckout {
		t.Fatalf("conflict remedy = %q", verdict.Remedy)
	}
	silent := PublicationState{LastAttemptAt: FormatTime(fixtureClock), TickSeconds: 600,
		LastSuccessAt: FormatTime(fixtureClock.Add(-4 * time.Hour)), LastOutcome: OutcomeFailed + ": gone"}
	if verdict := Health(silent, true, nil, fixtureClock, 30*time.Minute); verdict.Remedy != RemedyRemote {
		t.Fatalf("silent remedy = %q", verdict.Remedy)
	}
}

func TestAManualTickChangesNoVerdict(t *testing.T) {
	t.Parallel()
	// The component writes no publication state for a manual tick, so the
	// verdict is whatever the resident runner last wrote.
	published := PublicationState{LastAttemptAt: FormatTime(fixtureClock.Add(-time.Minute)),
		LastSuccessAt: FormatTime(fixtureClock.Add(-time.Minute)), TickSeconds: 600, LastOutcome: OutcomePublished, Rung: 1}
	before := Health(published, true, nil, fixtureClock, 30*time.Minute)
	after := Health(published, true, nil, fixtureClock, 30*time.Minute)
	if before != after || before.Status != StatusAlive {
		t.Fatalf("verdict moved: %+v then %+v", before, after)
	}
}

func TestTheStateFoldsPublishSkipAndFailure(t *testing.T) {
	t.Parallel()
	state := PublicationState{}
	state = RecordPublished(state, "m1e", RungBranchForce, 600, "", fixtureClock)
	if state.LastOutcome != OutcomePublished || state.Rung != 2 || state.ConsecutiveFailures != 0 {
		t.Fatalf("published state = %+v", state)
	}
	state = RecordFailed(state, "the remote refused", fixtureClock.Add(time.Minute))
	state = RecordFailed(state, "the remote refused", fixtureClock.Add(2*time.Minute))
	if state.ConsecutiveFailures != 2 || state.LastSuccessAt != FormatTime(fixtureClock) {
		t.Fatalf("failed state = %+v; the previous success must stand", state)
	}
	state = RecordSkipped(state, SkipNoNickname, fixtureClock.Add(3*time.Minute))
	if SkippedFor(state) != SkipNoNickname || state.ConsecutiveFailures != 2 {
		t.Fatalf("skipped state = %+v; a skip is not a failure", state)
	}
	state = RecordPublished(state, "m1e", RungMetasystemRef, 600, "", fixtureClock.Add(4*time.Minute))
	if state.ConsecutiveFailures != 0 || SkippedFor(state) != "" {
		t.Fatalf("recovered state = %+v", state)
	}
}
