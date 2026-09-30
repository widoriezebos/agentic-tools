package lane

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// fakeOwner is the lane owner as the keeper sees it: alive or dead, and the
// starts the keeper asked for.
type fakeOwner struct {
	alive  bool
	starts []string
	fail   error
}

func (o *fakeOwner) keeper(home string, clock *time.Time) Keeper {
	return Keeper{Home: home, Now: func() time.Time { return *clock },
		Inspect: func(string) (bool, error) { return o.alive, nil },
		Start: func(root string) error {
			o.starts = append(o.starts, root)
			return o.fail
		}}
}

func TestKeeperDoesNothingWithoutALane(t *testing.T) {
	t.Parallel()
	home, _, _ := laneDirs(t)
	clock := laneNow
	owner := &fakeOwner{}
	keeper := owner.keeper(home, &clock)
	keeper.Inspect = func(string) (bool, error) { t.Fatal("inspected an owner with no lane registered"); return false, nil }
	if line := keeper.Step(); line != "" || len(owner.starts) != 0 {
		t.Fatalf("no lane: line %q, starts %v", line, owner.starts)
	}
}

// A dead owner is restarted on the next cycle; a running one is left alone.
func TestKeeperRestartsDeadOwner(t *testing.T) {
	t.Parallel()
	home, root, _ := laneDirs(t)
	if _, err := Resolve(home, root, "m1e", laneNow, true); err != nil {
		t.Fatal(err)
	}
	clock := laneNow
	owner := &fakeOwner{alive: true}
	keeper := owner.keeper(home, &clock)
	if line := keeper.Step(); len(owner.starts) != 0 || !strings.Contains(line, "running") {
		t.Fatalf("alive: %q, starts %v", line, owner.starts)
	}
	owner.alive = false
	clock = clock.Add(10 * time.Minute)
	line := keeper.Step()
	if len(owner.starts) != 1 || owner.starts[0] != resolved(root) || !strings.Contains(line, "asked its supervision to start it (restart 1)") {
		t.Fatalf("dead: %q, starts %v; want one restart of %s", line, owner.starts, root)
	}
	owner.alive = true
	clock = clock.Add(10 * time.Minute)
	keeper.Step()
	if state := ReadKeeper(home); state != (KeeperState{}) {
		t.Fatalf("a running owner left keeper state %+v", state)
	}
}

// Two seats' stewards find the owner dead in the same minute: one starts it.
func TestKeeperTwoStewardsStartOnce(t *testing.T) {
	t.Parallel()
	home, root, _ := laneDirs(t)
	if _, err := Resolve(home, root, "m1e", laneNow, true); err != nil {
		t.Fatal(err)
	}
	clock := laneNow
	owner := &fakeOwner{}
	first, second := owner.keeper(home, &clock), owner.keeper(home, &clock)
	first.Step()
	clock = clock.Add(20 * time.Second)
	if line := second.Step(); !strings.Contains(line, "starting") {
		t.Fatalf("second steward: %q; want the start in progress named", line)
	}
	if len(owner.starts) != 1 {
		t.Fatalf("starts = %d; want one", len(owner.starts))
	}
}

// An owner that dies on every start is restarted with backoff, then given up
// on at the fifth death with a line a person can act on; landing start's
// reset lets the keeper restart it again.
func TestKeeperGivesUpAfterFiveAndRestartResets(t *testing.T) {
	t.Parallel()
	home, root, _ := laneDirs(t)
	if _, err := Resolve(home, root, "m1e", laneNow, true); err != nil {
		t.Fatal(err)
	}
	clock := laneNow
	owner := &fakeOwner{}
	keeper := owner.keeper(home, &clock)
	keeper.Step() // death 1: restarted at once
	clock = clock.Add(90 * time.Second)
	keeper.Step() // death 2: its restart waits one base interval after the last
	if len(owner.starts) != 1 {
		t.Fatalf("death 2 restarted before its backoff: %d starts", len(owner.starts))
	}
	clock = clock.Add(90 * time.Second)
	keeper.Step()
	if len(owner.starts) != 2 {
		t.Fatalf("death 2's restart after its backoff: %d starts", len(owner.starts))
	}
	var line string
	for i := 0; i < 20 && ReadKeeper(home).GaveUp == ""; i++ {
		clock = clock.Add(10 * time.Minute)
		line = keeper.Step()
	}
	state := ReadKeeper(home)
	if state.GaveUp == "" || state.Failures != GiveUpAt || len(owner.starts) != GiveUpAt-1 {
		t.Fatalf("state %+v, %d starts; want given up at %d deaths after %d restarts", state, len(owner.starts), GiveUpAt, GiveUpAt-1)
	}
	for _, want := range []string{"died 5 times", "restarted 4 times", resolved(root) + "/artifacts/agents/supervision/landing-owner.last-error", "metasystem landing start"} {
		if !strings.Contains(line, want) {
			t.Errorf("give-up line %q lacks %q", line, want)
		}
	}
	clock = clock.Add(time.Hour)
	if keeper.Step(); len(owner.starts) != GiveUpAt-1 {
		t.Fatalf("restarted after giving up")
	}
	if err := ResetKeeper(home); err != nil {
		t.Fatal(err)
	}
	clock = clock.Add(time.Minute)
	if keeper.Step(); len(owner.starts) != GiveUpAt {
		t.Fatalf("after the reset: %d starts; want one more", len(owner.starts))
	}
}

// A start that fails is a death like any other, and its error is kept for
// the person.
func TestKeeperKeepsStartError(t *testing.T) {
	t.Parallel()
	home, root, _ := laneDirs(t)
	if _, err := Resolve(home, root, "m1e", laneNow, true); err != nil {
		t.Fatal(err)
	}
	clock := laneNow
	owner := &fakeOwner{fail: errors.New("up refused")}
	keeper := owner.keeper(home, &clock)
	line := keeper.Step()
	if state := ReadKeeper(home); state.LastError != "up refused" || !strings.Contains(line, "up refused") {
		t.Fatalf("state %+v line %q; want the start error kept", state, line)
	}
}

// A person's pause holds the keeper until landing start resumes.
func TestKeeperHonoursPause(t *testing.T) {
	t.Parallel()
	home, root, _ := laneDirs(t)
	if _, err := Resolve(home, root, "m1e", laneNow, true); err != nil {
		t.Fatal(err)
	}
	clock := laneNow
	owner := &fakeOwner{}
	keeper := owner.keeper(home, &clock)
	if changed, err := SetPause(home, "Wido", laneNow); err != nil || !changed {
		t.Fatalf("pause = %v %v", changed, err)
	}
	if changed, err := SetPause(home, "Wido", laneNow.Add(time.Minute)); err != nil || changed {
		t.Fatalf("repeat pause = %v %v; want unchanged", changed, err)
	}
	line := keeper.Step()
	if len(owner.starts) != 0 || !strings.Contains(line, "paused by Wido") || !strings.Contains(line, "metasystem landing start") {
		t.Fatalf("paused: %q, starts %v", line, owner.starts)
	}
	if changed, err := ClearPause(home); err != nil || !changed {
		t.Fatalf("resume = %v %v", changed, err)
	}
	if changed, _ := ClearPause(home); changed {
		t.Fatalf("repeat resume changed something")
	}
	if keeper.Step(); len(owner.starts) != 1 {
		t.Fatalf("resumed: %d starts; want one", len(owner.starts))
	}
}
