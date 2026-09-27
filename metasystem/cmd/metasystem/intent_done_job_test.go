package main

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

// TestIntentDoneJobCompletesOnlyTheJob: done job J runs the one close-chain
// owner on the chain's own records. It never reaches the goal: goal options
// are refused before anything is read or written, job options are refused
// on the goal form, a round names its chain's root, a qualified reference
// selects the dispatch store, a launch is refused, and --evidence is handed
// to the owner's existing reconcile option, never accepted on trust.
func TestIntentDoneJobCompletesOnlyTheJob(t *testing.T) {
	t.Parallel()
	b := newDeliveryBed(t)
	b.writeJob(map[string]any{"jobId": "inv1", "role": "investigator", "status": "completed", "round": 1, "parentJob": nil})
	b.writeJob(map[string]any{"jobId": "inv1-r2", "role": "investigator", "status": "completed", "round": 2, "parentJob": "inv1"})
	b.handler = func(process intentProcess) intentProcessResult {
		record := b.job("inv1")
		record["chainClosed"] = true
		b.writeJob(record)
		return intentProcessResult{}
	}
	before, goalBefore := b.publications(), b.goalFile(bedGoal)
	for _, args := range [][]string{
		{"work", "finish", "j2:inv1", "--reason", "finished"},
		{"work", "finish", "j2:inv1", "--by", "Wido"},
		{"work", "finish", "j2:inv1", "--lineage", "m1"},
		{"work", "finish", "j2:inv1", "--goal", bedGoal},
		{"goal", "done", bedGoal, "--reason", "finished", "--dispositions", "d.md"},
		{"goal", "done", bedGoal, "--reason", "finished", "--evidence", "crit1"},
	} {
		code, result := b.do(args...)
		if code != 2 || result.Outcome != intentRefused || !strings.Contains(strings.ToLower(result.Summary), "nothing was done") || len(b.calls) != 0 || b.publications() != before {
			t.Fatalf("%v = code %d %+v calls %v", args, code, result, b.calls)
		}
	}
	code, result := b.do("work", "finish", "j2:inv1-r2")
	if code != 2 || result.Outcome != intentRefused || result.Next == nil || len(b.calls) != 0 ||
		!slices.Equal(slices.DeleteFunc(slices.Clone(result.Next.Argv), func(word string) bool { return word == "--json" }), []string{"metasystem", "work", "finish", "j2:inv1"}) {
		t.Fatalf("a round's done = code %d %+v", code, result)
	}
	for _, ref := range []string{"j1:inv1", "j2:missing"} {
		if code, result := b.do("work", "finish", ref); code == 0 || result.Outcome != intentRefused || len(b.calls) != 0 {
			t.Fatalf("an unresolved qualified reference %s = code %d %+v", ref, code, result)
		}
	}
	code, result = b.do("work", "finish", "j2:inv1", "--evidence", "crit1")
	if code != 0 || result.Outcome != intentConfirmed || len(b.calls) != 1 {
		t.Fatalf("done job = code %d %+v calls %v", code, result, b.calls)
	}
	if call := b.calls[0]; filepath.Base(call[0]) != "dispatch.sh" || !slices.Equal(call[1:], []string{"close", "--job", "inv1", "--reconcile-evidence", "crit1"}) {
		t.Fatalf("the close owner was called as %v", call)
	}
	if code, result := b.do("work", "finish", "j2:inv1"); code != 0 || result.Outcome != intentUnchanged || len(b.calls) != 1 {
		t.Fatalf("a repeated done job = code %d %+v", code, result)
	}
	if after := b.goalFile(bedGoal); b.publications() != before || after.State != goalBefore.State || len(after.History) != len(goalBefore.History) {
		t.Fatalf("done job changed the goal: %d publications (was %d), %s -> %s", b.publications(), before, goalBefore.State, after.State)
	}
}

// TestIntentDoneJobRefusesALaunch: a launch is completed by its own owner;
// done job reads only a dispatch job and refuses before any effect.
func TestIntentDoneJobRefusesALaunch(t *testing.T) {
	t.Parallel()
	b := newDeliveryBed(t)
	manager := &launch.Manager{Store: launch.Store{Root: t.TempDir()}}
	if err := manager.Store.Create(launch.Record{ID: "solo-1", Kind: "read", State: launch.Running}); err != nil {
		t.Fatal(err)
	}
	owners := b.intentBed.owners()
	owners.delivery = b.owners
	owners.processes.launches = func() *launch.Manager { return manager }
	before := b.publications()
	code, result := b.runJSON(owners, "work", "finish", "j1:solo-1")
	if code != 2 || result.Outcome != intentRefused || !strings.Contains(result.Summary, "does not take a launch reference") ||
		!strings.Contains(result.Decision, "metasystem work stop") || len(b.calls) != 0 || b.publications() != before {
		t.Fatalf("work finish j1:solo-1 = code %d %+v", code, result)
	}
	// A bare launch id is not a dispatch job: it is read as a goal name and
	// refused by the goal's own owner, with nothing closed.
	if code, result := b.runJSON(owners, "work", "finish", "solo-1"); code == 0 || result.Outcome != intentRefused || len(b.calls) != 0 || b.publications() != before {
		t.Fatalf("work finish solo-1 = code %d %+v", code, result)
	}
}

// TestIntentCloseRebindsTheChainBudgetFirst (U1d read F-1): the close of a
// chain carries the goal's review-round limit onto its critic registers
// before the close owner runs, and a rebind that fails is refused with the
// owner never started.
func TestIntentCloseRebindsTheChainBudgetFirst(t *testing.T) {
	t.Parallel()
	b := newDeliveryBed(t)
	b.writeJob(map[string]any{"jobId": "inv1", "role": "investigator", "status": "completed", "round": 1, "parentJob": nil})
	var rebound []string
	failing := false
	b.owners.rebind = func(root, job string) (map[string]string, error) {
		rebound = append(rebound, job)
		if failing {
			return nil, errors.New("fixture rebind failure")
		}
		return map[string]string{}, nil
	}
	b.handler = func(process intentProcess) intentProcessResult {
		if !slices.Equal(rebound, []string{"inv1"}) {
			t.Fatalf("the close owner ran before the rebind: %v", rebound)
		}
		record := b.job("inv1")
		record["chainClosed"] = true
		b.writeJob(record)
		return intentProcessResult{}
	}
	failing = true
	if code, result := b.do("work", "finish", "j2:inv1"); code != 1 || result.Outcome != intentRefused || len(b.calls) != 0 ||
		!strings.Contains(result.Summary, "fixture rebind failure") {
		t.Fatalf("a failed rebind = code %d %+v calls %v", code, result, b.calls)
	}
	failing, rebound = false, nil
	if code, result := b.do("work", "finish", "j2:inv1"); code != 0 || result.Outcome != intentConfirmed || len(b.calls) != 1 {
		t.Fatalf("the close after the rebind = code %d %+v calls %v", code, result, b.calls)
	}
}
