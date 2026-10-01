package lane

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
)

// nestedLaneDirs registers a nested landing checkout (its module root below
// its top) on a fresh home and returns the home, the checkout and the module.
func nestedLaneDirs(t *testing.T) (home, checkout, module string) {
	t.Helper()
	home, checkout, _ = laneDirs(t)
	module = filepath.Join(checkout, "metasystem")
	if err := os.MkdirAll(module, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(module, "go.mod"), []byte("module fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	register(t, home, checkout)
	return home, resolved(checkout), resolved(module)
}

// fakeAgent is the landing agent as the keeper sees it: the launch that
// runs, the starts it was asked for with their wake, and the reaps.
type fakeAgent struct {
	running string
	starts  []Wake
	roots   []string
	reaped  []string
	fail    error
}

func (a *fakeAgent) keeper(home, self string, clock *time.Time, sources WakeSources) AgentKeeper {
	return AgentKeeper{Home: home, Self: self, Now: func() time.Time { return *clock }, Sources: sources,
		Running: func() (string, bool, error) { return a.running, a.running != "", nil },
		Start: func(root string, wake Wake) (string, error) {
			if a.fail != nil {
				return "", a.fail
			}
			a.starts, a.roots = append(a.starts, wake), append(a.roots, root)
			a.running = "landing-" + itoa(len(a.starts))
			return a.running, nil
		},
		Reap: []func(string) error{func(id string) error { a.reaped = append(a.reaped, id); return nil }}}
}

// noBatches is a lane with no batch records, read without the disk.
func noBatches(string) ([]batch.Record, error) { return nil, nil }

// TestKeeperWakesForDueValidation (A-a, §3 Wake): an idle lane with no
// batch runs no model; when validation falls due the keeper starts the
// landing agent on the lane checkout, once, naming the reason, and starts no
// second one while it runs. A pause, a hold and a steward that is not the
// lane's own start nothing; an ended agent is reaped once, before the next
// start.
func TestKeeperWakesForDueValidation(t *testing.T) {
	t.Parallel()
	home, checkout, module := nestedLaneDirs(t)
	clock := laneNow
	due := false
	var askedRoot string
	sources := WakeSources{Records: noBatches,
		Validation: func(root string, _ time.Time) (bool, error) { askedRoot = root; return due, nil }}
	agent := &fakeAgent{}
	keeper := agent.keeper(home, module, &clock, sources)

	if line := keeper.Step(); len(agent.starts) != 0 || !strings.Contains(line, "idle") {
		t.Fatalf("an idle lane: %q, starts %v; want no model run", line, agent.starts)
	}
	due = true
	line := keeper.Step()
	if len(agent.starts) != 1 || !slices.Equal(agent.starts[0].Reasons, []string{WakeValidationDue}) || agent.roots[0] != checkout {
		t.Fatalf("validation due: %q, starts %+v on %v; want one start on %s for %s", line, agent.starts, agent.roots, checkout, WakeValidationDue)
	}
	if askedRoot != module || !strings.Contains(line, "validation") {
		t.Fatalf("the due read asked %q, line %q", askedRoot, line)
	}
	if line := keeper.Step(); len(agent.starts) != 1 || !strings.Contains(line, "running") {
		t.Fatalf("a running agent: %q, starts %d; want no second start", line, len(agent.starts))
	}

	// The agent ended with validation still due: it is reaped once, and a
	// fresh agent starts.
	agent.running = ""
	clock = clock.Add(time.Minute)
	if line := keeper.Step(); len(agent.starts) != 2 || !slices.Equal(agent.reaped, []string{"landing-1"}) {
		t.Fatalf("after the agent ended: %q, starts %d, reaped %v; want one reap and a fresh agent", line, len(agent.starts), agent.reaped)
	}
	agent.running = ""
	due = false
	keeper.Step()
	keeper.Step()
	if !slices.Equal(agent.reaped, []string{"landing-1", "landing-2"}) || len(agent.starts) != 2 {
		t.Fatalf("an ended agent is reaped once: reaped %v, starts %d", agent.reaped, len(agent.starts))
	}

	// Paused: nothing starts, whatever is due; an unreadable pause is a pause.
	due = true
	if _, err := SetPause(home, "a-person", laneNow); err != nil {
		t.Fatal(err)
	}
	if line := keeper.Step(); len(agent.starts) != 2 || !strings.Contains(line, "paused") {
		t.Fatalf("paused: %q, starts %d", line, len(agent.starts))
	}
	if err := os.WriteFile(pausePath(home), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if line := keeper.Step(); len(agent.starts) != 2 || !strings.Contains(line, "paused") {
		t.Fatalf("an unreadable pause: %q, starts %d; want it held as paused", line, len(agent.starts))
	}
	if _, err := ClearPause(home); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(pausePath(home)); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}

	// A hold (the budget, usage or custody gates) starts nothing; one that
	// cannot be read holds too.
	held := agent.keeper(home, module, &clock, sources)
	held.Holds = []func(string) (string, error){func(string) (string, error) { return "custody of proof p1 is unknown", nil }}
	if line := held.Step(); len(agent.starts) != 2 || !strings.Contains(line, "custody of proof p1 is unknown") {
		t.Fatalf("held: %q, starts %d", line, len(agent.starts))
	}
	held.Holds = []func(string) (string, error){func(string) (string, error) { return "", errors.New("the budget store is unreadable") }}
	if line := held.Step(); len(agent.starts) != 2 || !strings.Contains(line, "budget store is unreadable") {
		t.Fatalf("an unreadable hold: %q, starts %d", line, len(agent.starts))
	}

	// Another checkout's steward keeps no agent: only the lane's own does.
	other := agent.keeper(home, t.TempDir(), &clock, sources)
	if other.Step(); len(agent.starts) != 2 {
		t.Fatalf("another checkout's steward started the landing agent: %d", len(agent.starts))
	}
	// The lane's own steward, found by the checkout top, starts it.
	top := agent.keeper(home, checkout, &clock, sources)
	if top.Step(); len(agent.starts) != 3 {
		t.Fatalf("the lane's own steward (by its top) did not start the agent: %d", len(agent.starts))
	}
}

// TestKeeperStartsOutsideTheLaneLock (A-a, critique F-2 and F-3): the start
// runs outside the lane flock, so a person's landing stop is never kept
// waiting by it; a pause that lands while the agent starts stops it again,
// and a person's resume wakes a fresh one. A failed start claims nothing and
// the next cycle tries again.
func TestKeeperStartsOutsideTheLaneLock(t *testing.T) {
	t.Parallel()
	home, _, module := nestedLaneDirs(t)
	clock := laneNow
	reasons := []batch.Record{}
	sources := WakeSources{Records: func(string) ([]batch.Record, error) { return reasons, nil },
		Validation: func(string, time.Time) (bool, error) { return true, nil }}
	agent := &fakeAgent{}
	keeper := agent.keeper(home, module, &clock, sources)
	var cancelled []string
	keeper.Cancel = func(id string) error { cancelled = append(cancelled, id); agent.running = ""; return nil }
	start := keeper.Start
	keeper.Start = func(root string, wake Wake) (string, error) {
		// A person's landing stop during the start takes the flock at once.
		held, err := lock.File(LockPath(home), 0o600, lock.TryExclusive)
		if err != nil {
			t.Errorf("the lane flock is held during the start: %v", err)
			return start(root, wake)
		}
		_ = held.Release()
		if _, err := SetPause(home, "a-person", clock); err != nil {
			t.Errorf("landing stop during the start: %v", err)
		}
		return start(root, wake)
	}
	line := keeper.Step()
	if len(agent.starts) != 1 || !slices.Equal(cancelled, []string{"landing-1"}) || !strings.Contains(line, "stopped again") {
		t.Fatalf("paused during the start: %q, starts %d, cancelled %v", line, len(agent.starts), cancelled)
	}
	if _, err := ClearPause(home); err != nil {
		t.Fatal(err)
	}
	keeper.Start = start
	clock = clock.Add(time.Minute)
	// The person's resume wakes a fresh agent at once.
	if line := keeper.Step(); len(agent.starts) != 2 {
		t.Fatalf("after the resume: %q, starts %d; want a fresh agent", line, len(agent.starts))
	}
	agent.running = ""
	reasons = []batch.Record{{BatchID: "b-open", State: batch.StateOpen, Units: []batch.Unit{{GoalID: "g-one", State: batch.UnitJoined}}}}
	agent.fail = errors.New("the launcher refused")
	if line := keeper.Step(); len(agent.starts) != 2 || !strings.Contains(line, "the launcher refused") {
		t.Fatalf("a failed start: %q", line)
	}
	agent.fail = nil
	if line := keeper.Step(); len(agent.starts) != 3 || !slices.Equal(agent.starts[2].Reasons, []string{WakeQueued, WakeValidationDue}) {
		t.Fatalf("the next step after a failed start: %q, starts %+v; want a start at once", line, agent.starts)
	}
}

// TestWakeReasonsFromLaneState (A-a, §3 Wake): queued work, an unfinished
// batch, due validation and a pending finalization are each a reason; a
// source that cannot be read is named and is no reason; the reasons are the
// ones landing status --json carries as "wake".
func TestWakeReasonsFromLaneState(t *testing.T) {
	t.Parallel()
	home, checkout, module := nestedLaneDirs(t)
	registered := Record{Root: checkout, Install: module}
	joined := batch.Unit{GoalID: "g-one", State: batch.UnitJoined}
	records := []batch.Record{
		{BatchID: "b-empty", State: batch.StateOpen},
		{BatchID: "b-landed", State: batch.StateLanded, Units: []batch.Unit{joined}},
	}
	sources := WakeSources{Records: func(string) ([]batch.Record, error) { return records, nil },
		Validation:   func(string, time.Time) (bool, error) { return false, nil },
		Finalization: func(string) (bool, error) { return false, nil }}
	if wake := ReadWake(registered, laneNow, sources); len(wake.Reasons) != 0 || len(wake.Unread) != 0 {
		t.Fatalf("an empty open batch and a landed one wake nothing: %+v", wake)
	}
	records = append(records, batch.Record{BatchID: "b-open", State: batch.StateOpen, Units: []batch.Unit{joined}},
		batch.Record{BatchID: "b-held", State: batch.StateHeldTrunkRed, Units: []batch.Unit{joined}})
	sources.Validation = func(string, time.Time) (bool, error) { return true, nil }
	sources.Finalization = func(string) (bool, error) { return true, nil }
	wake := ReadWake(registered, laneNow, sources)
	if !slices.Equal(wake.Reasons, []string{WakeQueued, WakeUnfinishedBatch, WakeValidationDue, WakeFinalizationPending}) {
		t.Fatalf("reasons = %v", wake.Reasons)
	}
	sources.Records = func(string) ([]batch.Record, error) { return nil, errors.New("batch store unreadable") }
	sources.Validation = func(string, time.Time) (bool, error) { return false, errors.New("ledger unreadable") }
	sources.Finalization = nil
	wake = ReadWake(registered, laneNow, sources)
	if len(wake.Reasons) != 0 || len(wake.Unread) != 2 || !strings.Contains(strings.Join(wake.Unread, "; "), "ledger unreadable") {
		t.Fatalf("unread sources: %+v", wake)
	}

	// landing status --json carries the same reasons as "wake".
	view := BuildView(ViewSources{Home: home, Now: laneNow, Owner: func(string) (OwnerProbe, error) { return OwnerProbe{}, nil },
		Records:    func(string) ([]batch.Record, error) { return records, nil },
		Validation: func(string, time.Time) (bool, error) { return true, nil }})
	if view.Wake == nil || !slices.Equal(view.Wake.Reasons, []string{WakeQueued, WakeUnfinishedBatch, WakeValidationDue}) {
		t.Fatalf("view wake = %+v", view.Wake)
	}
}

// TestUnreadableKeeperRecordHoldsUntilAPersonStarts (A-a, re-review 2
// NB-1): a landing agent keeper record that cannot be read holds every
// start (the agent's and, through AgentStarting, the owner's) with a line
// naming the file and the command that repairs it; landing status shows it;
// a person's landing start (ClearAgentCooldown) replaces it and the hold
// ends.
func TestUnreadableKeeperRecordHoldsUntilAPersonStarts(t *testing.T) {
	t.Parallel()
	home, _, module := nestedLaneDirs(t)
	clock := laneNow
	agent := &fakeAgent{}
	keeper := agent.keeper(home, module, &clock, WakeSources{Records: noBatches, Validation: func(string, time.Time) (bool, error) { return true, nil }})
	if err := os.WriteFile(agentStatePath(home), []byte("{torn"), 0o600); err != nil {
		t.Fatal(err)
	}
	line := keeper.Step()
	if len(agent.starts) != 0 || !strings.Contains(line, agentStatePath(home)) || !strings.Contains(line, "run: metasystem landing start") {
		t.Fatalf("an unreadable record: %q, starts %d; want held, naming the file and the repair", line, len(agent.starts))
	}
	if _, _, err := AgentStarting(home, clock); err == nil {
		t.Fatal("an owner start reads an unreadable keeper record as no agent starting")
	}
	view := BuildView(ViewSources{Home: home, Now: laneNow, Owner: func(string) (OwnerProbe, error) { return OwnerProbe{}, nil }, Records: noBatches})
	if view.Wake == nil || !strings.Contains(strings.Join(view.Wake.Unread, "; "), "run: metasystem landing start") {
		t.Fatalf("landing status does not show the unreadable record: %+v", view.Wake)
	}
	if err := ClearAgentCooldown(home); err != nil {
		t.Fatal(err)
	}
	if keeper.Step(); len(agent.starts) != 1 {
		t.Fatalf("after a person's start: %d starts; want the agent", len(agent.starts))
	}
}

// TestKeeperLaunchesOnlyForAQueueNotPausedNoneAlive (simple lane §1, rail 2;
// unit B): the keeper launches the landing agent when the queue is
// non-empty, the lane is not paused and none is alive, and only then. An
// agent that ended with work still queued is followed by a fresh one at the
// next step: no cooldown or budget holds it.
func TestKeeperLaunchesOnlyForAQueueNotPausedNoneAlive(t *testing.T) {
	t.Parallel()
	home, checkout, module := nestedLaneDirs(t)
	clock := laneNow
	var records []batch.Record
	agent := &fakeAgent{}
	keeper := agent.keeper(home, module, &clock, WakeSources{Records: func(string) ([]batch.Record, error) { return records, nil }})

	if line := keeper.Step(); len(agent.starts) != 0 || !strings.Contains(line, "idle") {
		t.Fatalf("an empty queue: %q, starts %d; want no launch", line, len(agent.starts))
	}
	records = []batch.Record{{BatchID: "b-open", State: batch.StateOpen, Units: []batch.Unit{{GoalID: "g-one", State: batch.UnitJoined}}}}
	if line := keeper.Step(); len(agent.starts) != 1 || agent.roots[0] != checkout || !slices.Equal(agent.starts[0].Reasons, []string{WakeQueued}) {
		t.Fatalf("a queued member: %q, starts %+v on %v; want one launch on %s", line, agent.starts, agent.roots, checkout)
	}
	if line := keeper.Step(); len(agent.starts) != 1 || !strings.Contains(line, "running") {
		t.Fatalf("an agent alive: %q, starts %d; want no second launch", line, len(agent.starts))
	}
	// The agent ended with the member still queued: the next step launches
	// a fresh one at once.
	agent.running = ""
	clock = clock.Add(time.Minute)
	if line := keeper.Step(); len(agent.starts) != 2 || !slices.Equal(agent.reaped, []string{"landing-1"}) {
		t.Fatalf("after the agent ended with work queued: %q, starts %d, reaped %v; want a fresh launch", line, len(agent.starts), agent.reaped)
	}
	agent.running = ""
	if _, err := SetPause(home, "a-person", clock); err != nil {
		t.Fatal(err)
	}
	if line := keeper.Step(); len(agent.starts) != 2 || !strings.Contains(line, "paused") {
		t.Fatalf("paused: %q, starts %d; want no launch", line, len(agent.starts))
	}
}
