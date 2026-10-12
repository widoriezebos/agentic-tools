package goal

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestRecordFlakeStewardOpensPerTestAndRecovers(t *testing.T) {
	t.Parallel()
	e, client := fakeGoalEndpoint(t)
	args := flakeFixture()
	args.ObserveOnly = true
	args.Tests = []string{"TestOne", "TestTwo"}
	for n := 1; n <= 2; n++ {
		args.Attempt = "red-" + string(rune('0'+n))
		if result, err := RecordFlake(flakeRequest(e, n), args); err != nil || result.FixGoal != "" || result.Sightings != n {
			t.Fatalf("sighting %d: %+v %v", n, result, err)
		}
		tree, _ := acceptedTreeForEndpoint(t, e)
		if len(tree.Live) != 0 {
			t.Fatal("observation opened a goal before the steward")
		}
	}
	tree, _ := acceptedTreeForEndpoint(t, e)
	for i, fact := range FlakeFacts(tree.TrunkRed) {
		id := "flake-fix-metasystem-internal-proofrun-" + strings.ToLower(fact.TestName)
		fix := FlakeFix{Identity: fact.Identity, ID: id, Tier: 3, Priority: 1, Intent: "Fix " + fact.TestName + " with an artificial clock.", Basis: "attempt red-1, log logs/red, load 2.5; attempt red-2, log logs/red, load 2.5", Next: "fix the test, run it 20 times under -p 4 with the package, hand it in"}
		request := flakeRequest(e, i+3)
		result, err := RecordFlake(request, FlakeRecordArgs{Unit: args.Unit, Fix: &fix})
		if err != nil || result.FixGoal != id || result.Sightings != 2 || result.Action != "opened" {
			t.Fatalf("fix: %+v %v", result, err)
		}
		opened, tip := acceptedTreeForEndpoint(t, e)
		f := opened.Live[id]
		if f.Tier != 3 || f.Priority != 1 || f.Risk.Basis != fix.Basis || f.Intent != fix.Intent || f.NextStep != fix.Next {
			t.Fatalf("goal=%+v", f)
		}
		for _, current := range FlakeFacts(opened.TrunkRed) {
			if current.Identity == fact.Identity && (current.FixGoal != id || current.FixOpenedAt != request.stamp()) {
				t.Fatalf("register=%+v", current)
			}
		}
		journal, err := ReadEntry(e.Root, request.opid())
		if err != nil {
			t.Fatal(err)
		}
		recovered, err := requestForEntry(e, journal)
		if err != nil {
			t.Fatal(err)
		}
		_, err = recovered.Mutate(tip)
		var applied AlreadyApplied
		if !errors.As(err, &applied) {
			t.Fatalf("recovery opened twice: %v", err)
		}
		if replay, err := RecordFlake(flakeRequest(e, i+5), FlakeRecordArgs{Unit: args.Unit, Fix: &fix}); err != nil || replay.FixGoal != id || acceptedTipForEndpoint(t, e) != tip {
			t.Fatalf("new tick opened twice: %+v %v", replay, err)
		}
	}
	before := len(client.store.commits)
	args.Attempt = "red-3"
	if _, err := RecordFlake(flakeRequest(e, 7), args); err != nil {
		t.Fatal(err)
	}
	final, _ := acceptedTreeForEndpoint(t, e)
	if len(final.Live) != 2 || len(client.store.commits) != before+1 {
		t.Fatalf("goals=%v commits=%d", final.Live, len(client.store.commits))
	}
	for _, fact := range FlakeFacts(final.TrunkRed) {
		want := "flake-fix-metasystem-internal-proofrun-" + strings.ToLower(fact.TestName)
		if fact.FixGoal != want || len(fact.Sightings) != 3 || strings.Count(final.Live[want].NextStep, "attempt red-3") != 1 {
			t.Fatalf("third sighting crossed tests: %+v", fact)
		}
	}
}

func TestRecordFlakeStewardRejectsOneSighting(t *testing.T) {
	t.Parallel()
	e, _ := fakeGoalEndpoint(t)
	args := flakeFixture()
	args.ObserveOnly = true
	if _, err := RecordFlake(flakeRequest(e, 1), args); err != nil {
		t.Fatal(err)
	}
	before := acceptedTipForEndpoint(t, e)
	fix := FlakeFix{Identity: FlakeIdentity(args.Unit, args.Tests[0]), ID: "flake-fix-one", Tier: 3, Priority: 1, Intent: "Fix", Basis: "one sighting", Next: "Fix"}
	if _, err := RecordFlake(flakeRequest(e, 2), FlakeRecordArgs{Unit: args.Unit, Fix: &fix}); err == nil || acceptedTipForEndpoint(t, e) != before {
		t.Fatalf("one sighting opened: %v", err)
	}
}

func TestRecordFlakeObservationKeepsPersonsGoalAuthority(t *testing.T) {
	t.Parallel()
	file := vGoal("persons-fix", StateClaimed)
	e, _ := fakeGoalEndpoint(t, file)
	seedFlake(t, e, file.Id)
	before, _ := acceptedTreeForEndpoint(t, e)
	args := flakeFixture()
	args.ObserveOnly = true
	result, err := RecordFlake(flakeRequest(e, 1), args)
	if err != nil || result.FixGoal != file.Id || result.Sightings != 2 {
		t.Fatalf("sighting=%+v err=%v", result, err)
	}
	after, _ := acceptedTreeForEndpoint(t, e)
	prior, updated := before.Live[file.Id], after.Live[file.Id]
	if prior.State != updated.State || prior.Tier != updated.Tier || !reflect.DeepEqual(prior.Approved, updated.Approved) || !reflect.DeepEqual(prior.Claimed, updated.Claimed) || !reflect.DeepEqual(prior.Budget, updated.Budget) || !strings.Contains(updated.NextStep, "attempt "+args.Attempt) {
		t.Fatalf("a sighting changed a person's authority: %+v", updated)
	}
}
