package main

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

// removedIntentAliases are the compatibility spellings the verb cleanup
// deleted; each task is reached by its current public form.
var removedIntentAliases = []string{"ready", "decide", "resolve", "recover", "red", "fleet", "doctor", "ui", "fold", "close"}

// TestIntentRemovedAliasesRefuse: every removed spelling, called the way its
// old form was, reaches no public descriptor and no family handler, so it is
// refused before anything runs. ui stays the interface family's name; its
// public boundary belongs to the router.
func TestIntentRemovedAliasesRefuse(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{
		{"ready", bedGoal},
		{"decide", bedGoal, "--finding", "S-1", "--review", "critic-r2", "--reason", "bounded exposure"},
		{"resolve", bedGoal, "--review", "critic-r2", "--finding", "F-1", "--test", "TestIntentReady"},
		{"recover", "--session", "s1"},
		{"red", "own", "tr-1", "--goal", bedGoal},
		{"red", "close", "tr-1", "--reason", "fixed"},
		{"fleet", "--refresh"},
		{"doctor"},
		{"ui"},
		{"ui", "start"},
		{"ui", "stop"},
		{"ui", "status"},
		{"ui", "restart"},
		{"fold", "review", "crit1", "--dispositions", "d.md", "--brief", "b.md"},
		{"fold", "unit", "run-1", "--brief", "b.md"},
		{"close", "crit1", "--dispositions", "d.md", "--reconcile-evidence", "crit2"},
	} {
		if _, ok := findIntentCommand(args[0]); ok {
			t.Errorf("%s is still a public descriptor", args[0])
		}
		if code, stdout, stderr := runCLIHelp(args, families()); code != 2 || stdout != "" || stderr == "" {
			t.Errorf("%v = code %d stdout %q stderr %q; a removed spelling is refused", args, code, stdout, stderr)
		}
	}
	if len(removedIntentAliases) != 10 {
		t.Fatalf("%d removed aliases, want 10", len(removedIntentAliases))
	}
}

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
		{"done", "job", "inv1", "--reason", "finished"},
		{"done", "job", "inv1", "--by", "Wido"},
		{"done", "job", "inv1", "--lineage", "m1"},
		{"done", "job", "inv1", "--goal", bedGoal},
		{"done", bedGoal, "--reason", "finished", "--dispositions", "d.md"},
		{"done", bedGoal, "--reason", "finished", "--evidence", "crit1"},
	} {
		code, result := b.do(args...)
		if code != 2 || result.Outcome != intentRefused || !strings.Contains(result.Summary, "nothing was done") || len(b.calls) != 0 || b.publications() != before {
			t.Fatalf("%v = code %d %+v calls %v", args, code, result, b.calls)
		}
	}
	code, result := b.do("done", "job", "inv1-r2")
	if code != 2 || result.Outcome != intentRefused || result.Next == nil || len(b.calls) != 0 ||
		!slices.Equal(slices.DeleteFunc(slices.Clone(result.Next.Argv), func(word string) bool { return word == "--json" }), []string{"metasystem", "done", "job", "inv1"}) {
		t.Fatalf("a round's done = code %d %+v", code, result)
	}
	for _, ref := range []string{"j1:inv1", "j2:missing"} {
		if code, result := b.do("done", "job", ref); code == 0 || result.Outcome != intentRefused || len(b.calls) != 0 {
			t.Fatalf("an unresolved qualified reference %s = code %d %+v", ref, code, result)
		}
	}
	code, result = b.do("done", "job", "j2:inv1", "--evidence", "crit1")
	if code != 0 || result.Outcome != intentConfirmed || len(b.calls) != 1 {
		t.Fatalf("done job = code %d %+v calls %v", code, result, b.calls)
	}
	if call := b.calls[0]; filepath.Base(call[0]) != "dispatch.sh" || !slices.Equal(call[1:], []string{"close", "--job", "inv1", "--reconcile-evidence", "crit1"}) {
		t.Fatalf("the close owner was called as %v", call)
	}
	if code, result := b.do("done", "job", "inv1"); code != 0 || result.Outcome != intentUnchanged || len(b.calls) != 1 {
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
	for _, ref := range []string{"j1:solo-1", "solo-1"} {
		code, result := b.runJSON(owners, "done", "job", ref)
		if code != 2 || result.Outcome != intentRefused || !strings.Contains(result.Summary, "is a launch; done job reads a dispatch job") || len(b.calls) != 0 || b.publications() != before {
			t.Fatalf("done job %s = code %d %+v", ref, code, result)
		}
	}
}
