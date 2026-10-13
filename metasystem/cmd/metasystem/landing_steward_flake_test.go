package main

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

func TestLandingStewardFlakeLongIDPublishes(t *testing.T) {
	t.Parallel()
	b, ledger := newFlakeEvidenceBed(t, nil)
	name := "TestLandingStewardConfirmedFlakeOpensItsFixGoalWithTheRecordedSightingsAndPreservesPublicationAuthority"
	observation := plainFlakeForSteward("red-1")
	observation.Unit, observation.Tests = "cmd/metasystem", []string{name}
	record := b.owners.landing.landingFlakeRecorder(b.install)
	for _, attempt := range []string{"red-1", "red-2"} {
		observation.Attempt, observation.RepeatAttempt = attempt, "green-"+attempt
		if _, err := record(observation); err != nil {
			t.Fatal(err)
		}
	}
	entries, problems := goal.ParseTrunkRed([]byte(ledger.accepted("plans/goals/trunk-red.json")))
	if len(problems) != 0 {
		t.Fatal(problems)
	}
	fact := goal.FlakeFacts(entries)[0]
	id := "flake-fix-cmd-metasystem-" + strings.ToLower(name)
	want := strings.TrimRight(id[:91], "-") + fmt.Sprintf("-%x", sha256.Sum256([]byte(fact.Identity)))[:9]
	fix := goal.FlakeFix{Identity: fact.Identity, ID: id, Tier: 3, Priority: 1, Intent: "Fix the recorded test.", Basis: "red-1; red-2", Next: "Fix it under load."}
	updated, err := b.owners.landing.landingFlakeFixer(b.install)(fact, fix, laneTestNow)
	if err != nil || updated.FixGoal != want || len(updated.FixGoal) > 100 {
		t.Fatalf("fix=%+v err=%v want=%s", updated, err, want)
	}
	file, problems := goal.ParseFile([]byte(ledger.goalRecord(want)))
	if len(problems) != 0 || file.Id != want {
		t.Fatalf("goal=%+v problems=%v", file, problems)
	}
}

func TestLandingStewardFlakeRecurrenceAfterDoneReopens(t *testing.T) {
	t.Parallel()
	b, ledger := newFlakeEvidenceBed(t, []string{"TestClock"})
	record := b.owners.landing.landingFlakeRecorder(b.install)
	for _, attempt := range []string{"red-1", "red-2"} {
		if _, err := record(plainFlakeForSteward(attempt)); err != nil {
			t.Fatal(err)
		}
	}
	read := func() goal.FlakeFact {
		t.Helper()
		entries, problems := goal.ParseTrunkRed([]byte(ledger.accepted("plans/goals/trunk-red.json")))
		if len(problems) != 0 {
			t.Fatal(problems)
		}
		return goal.FlakeFacts(entries)[0]
	}
	fix := goal.FlakeFix{Identity: read().Identity, ID: "flake-fix-u-a-testclock", Tier: 3, Priority: 1, Intent: "Fix TestClock.", Basis: "red-1; red-2", Next: "Fix it under load."}
	fixer := b.owners.landing.landingFlakeFixer(b.install)
	if _, err := fixer(read(), fix, laneTestNow); err != nil {
		t.Fatal(err)
	}
	ledger.setNow(laneTestNow.Add(time.Minute))
	if code, out, stderr := ledger.public("goal", "done", fix.ID, "--reason", "Fixed.", "--by", "Wido"); code != 0 {
		t.Fatalf("done exit=%d %s %s", code, out, stderr)
	}
	observed, err := record(plainFlakeForSteward("red-3"))
	if err != nil || observed.Goal != "" {
		t.Fatalf("recurrence still carries done fix: %+v %v", observed, err)
	}
	updated, err := fixer(read(), fix, laneTestNow.Add(2*time.Minute))
	if err != nil || updated.FixGoal != fix.ID {
		t.Fatalf("reopen=%+v err=%v", updated, err)
	}
	file, problems := goal.ParseFile([]byte(ledger.goalRecord(fix.ID)))
	if len(problems) != 0 || file.State != goal.StateQueued {
		t.Fatalf("goal=%+v problems=%v", file, problems)
	}
	before := ledger.tip()
	if _, err := fixer(read(), fix, laneTestNow.Add(time.Hour)); err != nil || ledger.tip() != before {
		t.Fatalf("reopen replay changed ledger: %v", err)
	}
	seams := plain.ProveSeams{Now: func() time.Time { return laneTestNow.Add(2 * time.Minute) },
		Machine: func(string) (string, error) { return "landing", nil },
		Incidents: func(string, string, string) ([]goal.TrunkRedEntry, error) {
			entries, problems := goal.ParseTrunkRed([]byte(ledger.accepted("plans/goals/trunk-red.json")))
			if len(problems) != 0 {
				t.Fatal(problems)
			}
			return entries, nil
		},
		Git: func(_ string, args ...string) (string, error) {
			if args[0] == "show" && strings.HasPrefix(args[1], goal.AcceptedRef+":") {
				path := strings.TrimPrefix(strings.TrimPrefix(args[1], goal.AcceptedRef+":"), "metasystem/")
				return ledger.accepted(path), nil
			}
			return "main", nil
		}}
	status := plain.ReadStatus(t.TempDir(), lane.Record{Root: b.root, Install: b.install}, lane.View{Root: &b.root}, seams)
	if status.Health == nil {
		t.Fatal(status.Problems)
	}
	found := false
	for _, row := range status.Health.Rows() {
		if row.Subject == fix.Identity && strings.Contains(row.Act, "fix goal "+fix.ID+" is open") {
			found = true
		}
	}
	if !found {
		t.Fatalf("health does not name the reopened fix: %+v", status.Health.Rows())
	}
}

func TestLandingStewardFlakeFixUsesGoalPublication(t *testing.T) {
	t.Parallel()
	b, ledger := newFlakeEvidenceBed(t, []string{"TestClock"})
	owner := b.owners.landing
	record := owner.landingFlakeRecorder(b.install)
	for _, attempt := range []string{"red-1", "red-2"} {
		observed, err := record(plainFlakeForSteward(attempt))
		if err != nil || observed.Goal != "" {
			t.Fatalf("observation=%+v err=%v", observed, err)
		}
	}
	entries, problems := goal.ParseTrunkRed([]byte(ledger.accepted("plans/goals/trunk-red.json")))
	if len(problems) != 0 {
		t.Fatal(problems)
	}
	fact := goal.FlakeFacts(entries)[0]
	fix := goal.FlakeFix{Identity: fact.Identity, ID: "flake-fix-u-a-testclock", Tier: 3, Priority: 1, Intent: "Fix TestClock under load with an artificial clock.", Basis: "red-1 at load 22; red-2 at load 22", Next: "fix the test, run it 20 times under -p 4 with the package, hand it in"}
	fixer := owner.landingFlakeFixer(b.install)
	updated, err := fixer(fact, fix, laneTestNow)
	if err != nil || updated.FixGoal != fix.ID || updated.FixOpenedAt == "" {
		t.Fatalf("fix=%+v err=%v", updated, err)
	}
	file, problems := goal.ParseFile([]byte(ledger.goalRecord(fix.ID)))
	if len(problems) != 0 || file.Tier != 3 || file.Priority != 1 || file.Intent != fix.Intent || file.Risk.Basis != fix.Basis {
		t.Fatalf("goal=%+v problems=%v", file, problems)
	}
	before := ledger.tip()
	replay, err := fixer(fact, fix, laneTestNow.Add(time.Hour))
	if err != nil || ledger.tip() != before || replay.FixOpenedAt != updated.FixOpenedAt {
		t.Fatalf("replay changed goal or its opening time: %+v %v", replay, err)
	}
	t.Logf("observed shared goal %s, tier %d, priority %d, opening time retained on replay", file.Id, file.Tier, file.Priority)
}

func plainFlakeForSteward(attempt string) plain.FlakeRecord {
	return plain.FlakeRecord{FailedUnit: plain.FailedUnit{Unit: "u/a", Tests: []string{"TestClock"}}, Attempt: attempt, Commit: "main", Tree: "main-tree", Log: "red.log", Load: 22, Repeat: "alone", RepeatAttempt: "green-" + attempt, RepeatLog: "green.log"}
}

// TestLandingFlakeFixerFromEmptyOwnersResolvesTheMachine: the landing agent
// wires the steward's flake fixer from empty owners (landing_agent.go), so the
// machine seam must default to the checkout's machine instead of dereferencing
// nil (engine incident 20261013T151839-run on 5793734b8).
func TestLandingFlakeFixerFromEmptyOwnersResolvesTheMachine(t *testing.T) {
	t.Parallel()
	b, ledger := newFlakeEvidenceBed(t, []string{"TestClock"})
	owner := b.owners.landing
	record := owner.landingFlakeRecorder(b.install)
	for _, attempt := range []string{"red-1", "red-2"} {
		if _, err := record(plainFlakeForSteward(attempt)); err != nil {
			t.Fatal(err)
		}
	}
	entries, problems := goal.ParseTrunkRed([]byte(ledger.accepted("plans/goals/trunk-red.json")))
	if len(problems) != 0 {
		t.Fatal(problems)
	}
	fact := goal.FlakeFacts(entries)[0]
	fix := goal.FlakeFix{Identity: fact.Identity, ID: "flake-fix-u-a-testclock", Tier: 3, Priority: 1, Intent: "Fix TestClock under load with an artificial clock.", Basis: "red-1; red-2", Next: "Fix it."}
	owner.machine = nil
	wantMachine, wantErr := goal.ResolveMachine(b.install)
	updated, err := owner.landingFlakeFixer(b.install)(fact, fix, laneTestNow)
	if wantErr != nil {
		if err == nil || err.Error() != wantErr.Error() {
			t.Fatalf("empty owners: err=%v want %v", err, wantErr)
		}
		return
	}
	if err != nil || updated.FixGoal != fix.ID || updated.FixOpenedAt == "" {
		t.Fatalf("empty owners: fix=%+v err=%v", updated, err)
	}
	if _, problems := goal.ParseFile([]byte(ledger.goalRecord(fix.ID))); len(problems) != 0 {
		t.Fatal(problems)
	}
	t.Logf("empty owners resolved machine %q from the checkout and opened %s", wantMachine, fix.ID)
}
