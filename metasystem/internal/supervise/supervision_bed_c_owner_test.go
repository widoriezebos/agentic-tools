package supervise

// Ported from scripts/agents/supervision-fixtures.sh, scenario
// census-lifecycle, S4-4: continuous supervision has one owner and one
// cadence. A dead watcher, and then a dead reaper, is detected on the owner's
// next observation and the WHOLE instance set is replaced, never the dead
// member alone; the owner narrates the failing observation and the relaunch
// on the trace it writes to owner.ndjson. The components are a programmable
// world; no process is killed and no cycle is waited for.

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// supCDyingWorld reports exactly the named component tags as dead.
type supCDyingWorld struct {
	*fakeWorld
	dead map[string]bool
}

func (w *supCDyingWorld) Observe(held Held) Observation {
	if w.dead[held.Tag] {
		return Failing
	}
	return Healthy
}

func supCCurrentSet(owner *Owner) map[Component]Held {
	set := map[Component]Held{}
	for _, held := range owner.currentGenerationHeld() {
		set[held.Component] = held
	}
	return set
}

func TestSupCDeadComponentReplacesTheWholeSetAndNarratesIt(t *testing.T) {
	t.Parallel()
	world := &supCDyingWorld{fakeWorld: newWorld(), dead: map[string]bool{}}
	owner := newOwner(world.fakeWorld)
	owner.Components = world
	var narration []string
	owner.Narrate = func(trace CycleTrace) {
		line, err := json.Marshal(trace)
		if err != nil {
			t.Errorf("trace encoding: %v", err)
			return
		}
		narration = append(narration, string(line))
	}
	establish(t, owner, world.fakeWorld)
	now := time.Unix(1786000000, 0)

	for round, victim := range []Component{Watcher, Reaper} {
		before := supCCurrentSet(owner)
		if len(before) != len(productionComponentSet) {
			t.Fatalf("round %d: the established set is incomplete: %+v", round, before)
		}
		world.dead[before[victim].Tag] = true
		published := world.published
		if exit := owner.Cycle(now); exit != nil {
			t.Fatalf("round %d: a dead %s exited the owner: %+v", round, victim, exit)
		}
		after := supCCurrentSet(owner)
		if len(after) != len(before) {
			t.Fatalf("round %d: replacement set is incomplete: %+v", round, after)
		}
		for component, old := range before {
			replacement := after[component]
			if replacement.Tag == old.Tag || replacement.Identity.Pid == old.Identity.Pid || replacement.Generation <= old.Generation {
				t.Fatalf("round %d: a dead %s did not replace %s: before=%+v after=%+v", round, victim, component, old, replacement)
			}
		}
		if world.published != published+1 {
			t.Fatalf("round %d: the replacement set was not published", round)
		}
		// The replacement set runs healthy for one interval, as the owner's
		// cadence observes it before the next death.
		now = now.Add(time.Second)
		if exit := owner.Cycle(now); exit != nil || owner.Breaker.Consecutive != 0 {
			t.Fatalf("round %d: the replacement set did not observe healthy: exit=%+v breaker=%d", round, exit, owner.Breaker.Consecutive)
		}
		now = now.Add(time.Second)
	}

	failing, relaunches := 0, 0
	for _, line := range narration {
		if strings.Contains(line, `"observation":"failing"`) {
			failing++
		}
		if strings.Contains(line, `"relaunch"`) {
			relaunches++
		}
	}
	if failing < 2 || relaunches < 2 {
		t.Fatalf("component deaths were not surfaced in the owner narration (failing=%d relaunch=%d):\n%s",
			failing, relaunches, strings.Join(narration, "\n"))
	}
}
