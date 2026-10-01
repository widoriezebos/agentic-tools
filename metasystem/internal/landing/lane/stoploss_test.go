package lane

import (
	"errors"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/custody"
)

// queuedBatch and provingBatch are a lane's batch records: one with a
// member waiting, one that a session left unfinished.
var (
	queuedBatch  = []batch.Record{{BatchID: "b-one", State: batch.StateOpen, Units: []batch.Unit{{GoalID: "g-one", State: batch.UnitJoined}}}}
	provingBatch = []batch.Record{{BatchID: "b-one", State: batch.StateProving, Units: []batch.Unit{{GoalID: "g-one", State: batch.UnitJoined}}}}
)

// startSleeper starts a real kernel execution under custody: a process in
// its own group, recorded before it starts. Cleanup ends it by its own pid.
// The channel closes when it has ended.
func startSleeper(t *testing.T, home string) <-chan struct{} {
	t.Helper()
	command := exec.Command("sleep", "300")
	if _, err := custody.Start(home, custody.KindProve, "batch b-one attempt a1 batch", laneNow, command); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { _ = command.Wait(); close(done) }()
	t.Cleanup(func() {
		_ = command.Process.Kill()
		<-done
	})
	return done
}

// stopCustody is the keeper's Settle on this host: the custody store's own
// stop, with real signals; its grace waits for the execution to end, never
// on the clock.
func stopCustody(home string, ended <-chan struct{}) func(string) error {
	return func(string) error {
		settlement, err := custody.Stop(home, custody.Probes{}, nil, time.Second, func(time.Duration) { <-ended })
		if err != nil {
			return err
		}
		if !settlement.Settled(false) {
			return errors.New(strings.Join(append(settlement.Live, settlement.Unknown...), "; "))
		}
		return nil
	}
}

// TestDeadlineCancelsAndSettles (K10, R8-07): a landing session has 2 h
// from its launch. Before that the keeper leaves it running; at the
// deadline it cancels the session through launch cancellation, ends the
// test run the session left running (a real process in custody) and
// settles its custody, and the hit pauses the lane for a person.
func TestDeadlineCancelsAndSettles(t *testing.T) {
	t.Parallel()
	home, _, module := nestedLaneDirs(t)
	clock := laneNow
	agent := &fakeAgent{}
	keeper := agent.keeper(home, module, &clock, WakeSources{Records: func(string) ([]batch.Record, error) { return queuedBatch, nil }})
	var cancelled []string
	keeper.Cancel = func(id string) error { cancelled = append(cancelled, id); agent.running = ""; return nil }
	var ended <-chan struct{}
	keeper.Settle = func(root string) error { return stopCustody(home, ended)(root) }

	if line := keeper.Step(); len(agent.starts) != 1 {
		t.Fatalf("queued work: %q; want the agent started", line)
	}
	ended = startSleeper(t, home)
	clock = laneNow.Add(AllowanceWindow - time.Minute)
	if line := keeper.Step(); len(cancelled) != 0 || !strings.Contains(line, "running") {
		t.Fatalf("before the deadline: %q, cancelled %v; want it left running", line, cancelled)
	}
	clock = laneNow.Add(AllowanceWindow)
	line := keeper.Step()
	if !slices.Equal(cancelled, []string{"landing-1"}) || !strings.Contains(line, "ran out of its time") {
		t.Fatalf("at the deadline: %q, cancelled %v; want landing-1 cancelled", line, cancelled)
	}
	if settlement, err := custody.Settle(home, custody.Probes{}); err != nil || !settlement.Settled(false) {
		t.Fatalf("custody after the deadline = %+v %v; want the test run ended and settled", settlement, err)
	}
	pause, paused := ReadPause(home)
	store, err := ReadStopLoss(home)
	if !paused || pause.By != StopLossBy || err != nil || len(store.Hits) != 1 || store.Hits[0].Kind != HitDeadline {
		t.Fatalf("after the deadline: pause %+v %t, hits %+v %v; want the lane stopped for a person by the stop-loss", pause, paused, store.Hits, err)
	}
	// The cancelled session is reaped while the lane waits for a person,
	// so its usage is reconciled before any resume.
	if line := keeper.Step(); len(agent.starts) != 1 || !strings.Contains(line, "paused") || !slices.Equal(agent.reaped, []string{"landing-1"}) {
		t.Fatalf("after the hit: %q, starts %d, reaped %v; want the session reaped and nothing started until a person resumes", line, len(agent.starts), agent.reaped)
	}
}

// TestDeadlineSurvivesRecovery (K10, R8-07): the 2 h are measured from the
// launch of the session that took the work up and cover its composition; a
// recovery session for the unfinished batch keeps that clock. A
// validation session is bound to its own launch.
func TestDeadlineSurvivesRecovery(t *testing.T) {
	t.Parallel()
	home, _, module := nestedLaneDirs(t)
	clock := laneNow
	records := queuedBatch
	agent := &fakeAgent{}
	keeper := agent.keeper(home, module, &clock, WakeSources{Records: func(string) ([]batch.Record, error) { return records, nil }})
	var cancelled []string
	keeper.Cancel = func(id string) error { cancelled = append(cancelled, id); agent.running = ""; return nil }
	keeper.Settle = func(string) error { return nil }
	keeper.Step()
	// The first session dies an hour in, its batch unfinished.
	records, agent.running, clock = provingBatch, "", laneNow.Add(time.Hour)
	if line := keeper.Step(); len(agent.starts) != 2 || agent.running != "landing-2" {
		t.Fatalf("recovery: %q, starts %d; want a recovery session", line, len(agent.starts))
	}
	clock = laneNow.Add(AllowanceWindow)
	if line := keeper.Step(); !slices.Equal(cancelled, []string{"landing-2"}) {
		t.Fatalf("2 h after the first launch: %q, cancelled %v; want the recovery session cancelled on the first session's clock", line, cancelled)
	}

	// A validation session after a person's resume: its own clock.
	if err := Grant(home, "Wido", clock); err != nil {
		t.Fatal(err)
	}
	if _, err := ClearPause(home); err != nil {
		t.Fatal(err)
	}
	records = nil
	keeper.Sources.Validation = func(string, time.Time) (bool, error) { return true, nil }
	clock = clock.Add(time.Minute)
	started := clock
	if keeper.Step(); agent.running != "landing-3" {
		t.Fatalf("validation due: running %q; want a validation session", agent.running)
	}
	clock = started.Add(ValidationWindow - time.Second)
	if keeper.Step(); len(cancelled) != 1 {
		t.Fatalf("inside its own limit the validation session was cancelled: %v", cancelled)
	}
	clock = started.Add(ValidationWindow)
	if keeper.Step(); !slices.Equal(cancelled, []string{"landing-2", "landing-3"}) {
		t.Fatalf("at its own limit: cancelled %v; want the validation session cancelled", cancelled)
	}
}

// TestCrashBreakerCountsOnlyUnsuccessfulRecoveries (K10, R8-08): fresh
// sessions never count, however they end, nor a recovery that completes;
// the third unsuccessful recovery start within 6 h pauses the lane.
func TestCrashBreakerCountsOnlyUnsuccessfulRecoveries(t *testing.T) {
	t.Parallel()
	home, _, _ := nestedLaneDirs(t)
	at := laneNow
	run := func(launch string, reasons []string, completed bool) {
		t.Helper()
		if err := withLock(home, func() error {
			if err := StartLaunchHeld(home, launch, reasons, at); err != nil {
				return err
			}
			state := "failed"
			if completed {
				state = "completed"
			}
			return RecordLaunchEndHeld(home, LaunchEnd{Launch: launch, State: state, Completed: completed, Usage: Usage{Known: true, Tokens: 10, Source: "measure"}}, at.Add(time.Minute))
		}); err != nil {
			t.Fatal(err)
		}
		at = at.Add(10 * time.Minute)
	}
	fresh, recovery := []string{WakeQueued}, []string{WakeUnfinishedBatch}
	for index := 0; index < 4; index++ {
		run("fresh-"+itoa(index), fresh, false)
	}
	run("recovered", recovery, true)
	run("crash-1", recovery, false)
	if err := withLock(home, func() error { return FailedStartHeld(home, recovery, at, "the launcher refused") }); err != nil {
		t.Fatal(err)
	}
	if _, paused := ReadPause(home); paused {
		t.Fatal("two unsuccessful recoveries and four failed fresh sessions tripped the breaker")
	}
	run("crash-2", recovery, false)
	pause, paused := ReadPause(home)
	store, _ := ReadStopLoss(home)
	if !paused || pause.By != StopLossBy || len(store.Hits) != 1 || store.Hits[0].Kind != HitBreaker {
		t.Fatalf("three unsuccessful recoveries: pause %+v %t, hits %+v; want the breaker's stop", pause, paused, store.Hits)
	}

	// Outside the window they no longer count.
	quiet, _, _ := nestedLaneDirs(t)
	home, at = quiet, laneNow
	run("old-1", recovery, false)
	run("old-2", recovery, false)
	at = laneNow.Add(BreakerWindow + time.Minute)
	run("new-1", recovery, false)
	if _, paused := ReadPause(home); paused {
		t.Fatal("failures older than 6 h tripped the breaker")
	}
}

// TestDailyCeilingResetsAtLocalMidnight (K10, R8-06): the landing agent's
// reconciled usage since local midnight holds the next launch at the
// ceiling; the next local day starts from zero, and a person's resume
// counts from the resume.
func TestDailyCeilingResetsAtLocalMidnight(t *testing.T) {
	t.Parallel()
	home, _, _ := nestedLaneDirs(t)
	midnight := time.Date(2026, 10, 1, 0, 0, 0, 0, time.Local)
	spend := func(launch string, tokens int64, at time.Time) {
		t.Helper()
		if err := withLock(home, func() error {
			if err := StartLaunchHeld(home, launch, []string{WakeQueued}, at.Add(-time.Minute)); err != nil {
				return err
			}
			return RecordLaunchEndHeld(home, LaunchEnd{Launch: launch, State: "completed", Completed: true, Usage: Usage{Known: true, Tokens: tokens, Source: "measure"}}, at)
		}); err != nil {
			t.Fatal(err)
		}
	}
	hold := func(at time.Time) string {
		t.Helper()
		var reason string
		if err := withLock(home, func() error {
			var err error
			reason, err = StopLossHoldHeld(home, at, 1000)
			return err
		}); err != nil {
			t.Fatal(err)
		}
		return reason
	}
	spend("yesterday", 900, midnight.Add(-time.Hour))
	spend("morning", 600, midnight.Add(2*time.Hour))
	if reason := hold(midnight.Add(3 * time.Hour)); reason != "" {
		t.Fatalf("600 of 1000 today (900 yesterday): held %q", reason)
	}
	spend("noon", 400, midnight.Add(12*time.Hour))
	if reason := hold(midnight.Add(13 * time.Hour)); !strings.Contains(reason, "1000") {
		t.Fatalf("1000 of 1000 today: held %q; want the ceiling", reason)
	}
	if _, paused := ReadPause(home); !paused {
		t.Fatal("the ceiling did not stop the lane for a person")
	}
	if reason := hold(midnight.Add(24*time.Hour + time.Minute)); reason != "" {
		t.Fatalf("after local midnight: held %q; want a fresh day", reason)
	}
	if err := Grant(home, "Wido", midnight.Add(14*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if reason := hold(midnight.Add(15 * time.Hour)); reason != "" {
		t.Fatalf("after a person's resume: held %q; want the count from the resume", reason)
	}
}

// TestResumeGrantsAFreshAllowanceAndKeepsHistory (K10, R8-08): a batch's
// fifth execution is refused and owes an alert, without pausing the lane;
// a person's grant gives a fresh allowance of four, and the history of the
// first is kept.
func TestResumeGrantsAFreshAllowanceAndKeepsHistory(t *testing.T) {
	t.Parallel()
	home, _, _ := nestedLaneDirs(t)
	charge := func() error {
		return Gate(home, OpProve, AuthorityAgent, func(Record) error { return ChargeHeld(home, "b-one", laneNow) })
	}
	for index := 0; index < AllowanceExecutions; index++ {
		if err := charge(); err != nil {
			t.Fatalf("execution %d: %v", index+1, err)
		}
	}
	var refusal *Refusal
	if err := charge(); !errors.As(err, &refusal) || refusal.Code != CodeAllowanceSpent {
		t.Fatalf("the fifth execution = %v; want the allowance's refusal", err)
	}
	if _, paused := ReadPause(home); paused {
		t.Fatal("a spent allowance paused the lane")
	}
	if batch, spent, err := AllowanceSpent(home); err != nil || !spent || batch != "b-one" {
		t.Fatalf("spent allowance = %q %t %v; want b-one", batch, spent, err)
	}
	before, _ := ReadStopLoss(home)
	if err := Grant(home, "Wido", laneNow); err != nil {
		t.Fatal(err)
	}
	if _, spent, _ := AllowanceSpent(home); spent {
		t.Fatal("a fresh grant still reads the allowance spent")
	}
	for index := 0; index < AllowanceExecutions; index++ {
		if err := charge(); err != nil {
			t.Fatalf("after the resume, execution %d: %v", index+1, err)
		}
	}
	after, _ := ReadStopLoss(home)
	if after.Grant != before.Grant+1 || len(after.History) <= len(before.History) || !slices.Equal(after.History[:len(before.History)], before.History) ||
		len(after.Hits) != 1 {
		t.Fatalf("after the resume: grant %d (was %d), history %d (was %d), hits %+v; want a new grant over the kept history",
			after.Grant, before.Grant, len(after.History), len(before.History), after.Hits)
	}
}

// TestOneFreshSessionPerBatch (D4): a session takes up one batch; begin of
// another batch in the same session is refused, and the next session
// takes it up.
func TestOneFreshSessionPerBatch(t *testing.T) {
	t.Parallel()
	home, _, module := nestedLaneDirs(t)
	clock := laneNow
	agent := &fakeAgent{}
	keeper := agent.keeper(home, module, &clock, WakeSources{Records: func(string) ([]batch.Record, error) { return queuedBatch, nil }})
	keeper.Step()
	begin := func(batchID string) error {
		return Gate(home, OpBegin, AuthorityAgent, func(Record) error { return BindBatchHeld(home, batchID, clock) })
	}
	if err := begin("b-one"); err != nil {
		t.Fatal(err)
	}
	if err := begin("b-one"); err != nil {
		t.Fatalf("the same batch begun again: %v", err)
	}
	var refusal *Refusal
	if err := begin("b-two"); !errors.As(err, &refusal) || refusal.Code != CodeFreshSession {
		t.Fatalf("a second batch in one session = %v; want refused", err)
	}
	// The session ends; the next batch is queued: a fresh session starts at
	// once, without the cooldown of the same reasons, and takes it up.
	agent.running, clock = "", clock.Add(time.Minute)
	if line := keeper.Step(); len(agent.starts) != 2 {
		t.Fatalf("after the session that landed its batch: %q; want a fresh session at once", line)
	}
	if err := begin("b-two"); err != nil {
		t.Fatalf("the fresh session's batch: %v", err)
	}
}

// TestHitAlertsOpenOnceThroughTheKeeper (K10): a hit's alert is opened by
// the lane's own keeper, once, and stays owed while opening fails.
func TestHitAlertsOpenOnceThroughTheKeeper(t *testing.T) {
	t.Parallel()
	home, checkout, module := nestedLaneDirs(t)
	clock := laneNow
	agent := &fakeAgent{}
	keeper := agent.keeper(home, module, &clock, WakeSources{Records: noBatches})
	var opened []Hit
	failing := true
	keeper.Alert = func(root string, hit Hit) error {
		if root != checkout {
			t.Errorf("the alert was asked for %s, not the lane %s", root, root)
		}
		if failing {
			return errors.New("the alert store is unwritable")
		}
		opened = append(opened, hit)
		return nil
	}
	if err := withLock(home, func() error {
		return RecordLaunchEndHeld(home, LaunchEnd{Launch: "landing-x", State: "cancelled", Usage: Usage{Why: "no transcript"}}, clock)
	}); err != nil {
		t.Fatal(err)
	}
	if line := keeper.Step(); !strings.Contains(line, "could not be opened") || len(opened) != 0 {
		t.Fatalf("a failing alert: %q", line)
	}
	failing = false
	keeper.Step()
	keeper.Step()
	if len(opened) != 1 || opened[0].Kind != HitUsageUnknown || !strings.Contains(opened[0].Message, "landing-x") {
		t.Fatalf("opened %+v; want the usage hit's alert once", opened)
	}
	other := agent.keeper(home, t.TempDir(), &clock, WakeSources{Records: noBatches})
	other.Alert = func(string, Hit) error { t.Error("another checkout's steward opened the lane's alert"); return nil }
	other.Step()
}

// TestResumeKeepsALiveSessionsDeadline (K10, critique N-5): a session whose
// deadline cancel failed keeps its clock across a person's resume, so the
// keeper cancels it again rather than letting it run unbounded.
func TestResumeKeepsALiveSessionsDeadline(t *testing.T) {
	t.Parallel()
	home, _, module := nestedLaneDirs(t)
	clock := laneNow
	agent := &fakeAgent{}
	keeper := agent.keeper(home, module, &clock, WakeSources{Records: func(string) ([]batch.Record, error) { return queuedBatch, nil }})
	var cancels int
	keeper.Cancel = func(string) error { cancels++; return errors.New("the session could not be stopped") }
	keeper.Settle = func(string) error { return nil }
	keeper.Step()
	clock = laneNow.Add(AllowanceWindow)
	if line := keeper.Step(); cancels != 1 || !strings.Contains(line, "could not") {
		t.Fatalf("a failed cancel: %q, cancels %d", line, cancels)
	}
	if err := Grant(home, "Wido", clock); err != nil {
		t.Fatal(err)
	}
	if _, err := ClearPause(home); err != nil {
		t.Fatal(err)
	}
	clock = clock.Add(time.Minute)
	if keeper.Step(); cancels != 2 {
		t.Fatalf("after the resume the still-running session was not cancelled again: cancels %d", cancels)
	}
	if _, paused := ReadPause(home); !paused {
		t.Fatal("the session still runs past its deadline and the lane is not stopped")
	}
}

// TestDeadlineWaitsForAPushInFlight (K10, critique N-4): the deadline
// cancel does not cut a push to main in half; it waits for the push, at
// most PushGrace past the deadline.
func TestDeadlineWaitsForAPushInFlight(t *testing.T) {
	t.Parallel()
	home, checkout, module := nestedLaneDirs(t)
	clock := laneNow
	agent := &fakeAgent{}
	keeper := agent.keeper(home, module, &clock, WakeSources{Records: func(string) ([]batch.Record, error) { return queuedBatch, nil }})
	var cancelled []string
	keeper.Cancel = func(id string) error { cancelled = append(cancelled, id); agent.running = ""; return nil }
	keeper.Settle = func(string) error { return nil }
	keeper.Step()
	// A publication's push runs: its token is minted by a live process.
	if err := Gate(home, OpPublish, AuthorityAgent, func(Record) error {
		_, err := mint(home, Tuple{Repo: checkout}, OpPublish, AuthorityAgent)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	clock = laneNow.Add(AllowanceWindow)
	if line := keeper.Step(); len(cancelled) != 0 || !strings.Contains(line, "push") {
		t.Fatalf("a push in flight at the deadline: %q, cancelled %v; want the cancel to wait", line, cancelled)
	}
	clock = laneNow.Add(AllowanceWindow + PushGrace)
	if keeper.Step(); len(cancelled) != 1 {
		t.Fatalf("PushGrace past the deadline: cancelled %v; want the session cancelled", cancelled)
	}
}

// TestRepeatedBindLeavesTheStoreUnchanged (R-129-ui): begin repeated for the
// batch its session already took up is success with no second record: the
// stop-loss store is not rewritten, with or without a running session.
func TestRepeatedBindLeavesTheStoreUnchanged(t *testing.T) {
	t.Parallel()
	for _, withSession := range []bool{false, true} {
		home, _, module := nestedLaneDirs(t)
		clock := laneNow
		if withSession {
			agent := &fakeAgent{}
			keeper := agent.keeper(home, module, &clock, WakeSources{Records: func(string) ([]batch.Record, error) { return queuedBatch, nil }})
			keeper.Step()
		}
		begin := func() error {
			return Gate(home, OpBegin, AuthorityAgent, func(Record) error { return BindBatchHeld(home, "b-one", clock) })
		}
		if err := begin(); err != nil {
			t.Fatal(err)
		}
		first, err := os.ReadFile(stopLossPath(home))
		if err != nil {
			t.Fatal(err)
		}
		clock = clock.Add(time.Minute)
		if err := begin(); err != nil {
			t.Fatalf("session=%v: the same batch begun again: %v", withSession, err)
		}
		again, err := os.ReadFile(stopLossPath(home))
		if err != nil {
			t.Fatal(err)
		}
		if string(again) != string(first) {
			t.Fatalf("session=%v: a repeated begin rewrote the stop-loss store:\n%s\nwas\n%s", withSession, again, first)
		}
	}
}
