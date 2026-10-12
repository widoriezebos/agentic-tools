package plain

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

func flakeStewardFixture() goal.TrunkRedEntry {
	return goal.TrunkRedEntry{ID: "flake", Identity: goal.FlakeIdentity("internal/unit", "TestClock"), Class: goal.TrunkRedClassFlake,
		Group: "internal/unit", TestUnit: "internal/unit", TestName: "TestClock", Opened: bedNow.Add(-time.Hour).Format(time.RFC3339),
		Failures:  []goal.TrunkRedFailure{{Name: "TestClock"}},
		Sightings: []goal.TrunkRedSighting{{Opid: "one", Attempt: "red-1", LogPath: "red-1.log", Load: 22, SeenAt: bedNow.Add(-time.Hour).Format(time.RFC3339), Rerun: &goal.TrunkRedRerun{Attempt: "green-1"}}}}
}

func flakeGoalFixture(id, at string) string {
	return string(goal.RenderFile(&goal.GoalFile{Id: id, State: goal.StateQueued, Intent: "Fix the clock", Origin: goal.OriginMain, OpenedAt: at, Revision: 1,
		History: []goal.HistoryLine{{At: at, Opid: "01J5X0000000000000000000F1-lane-718ba0eb", Verb: "open", Actor: "lane+landing-agent", Targets: []string{id}}}}))
}

func TestLandingStewardFlakeThresholdAndOnce(t *testing.T) {
	t.Parallel()
	b := newStewardBed(t)
	register := flakeStewardFixture()
	b.seams.Incidents = func(string, string, string) ([]goal.TrunkRedEntry, error) { return []goal.TrunkRedEntry{register}, nil }
	calls := 0
	b.seams.Steward.FlakeFix = func(f goal.FlakeFact, fix goal.FlakeFix, now time.Time) (goal.FlakeFact, error) {
		calls++
		if fix.ID != "flake-fix-internal-unit-testclock" || fix.Tier != 3 || fix.Priority != 1 || fix.Identity != register.Identity || now != bedNow.Add(time.Minute) || fix.Next != "fix the test, run it 20 times under -p 4 with the package, hand it in" {
			t.Fatalf("wrong goal request: %+v at %v", fix, now)
		}
		for _, want := range []string{"the test TestClock in internal/unit failed 2 times", "green on every named repeat", "load 22", "load 33", "red-1.log", "red-2.log", "without a wall clock", "R-104: artificial clocks"} {
			if !strings.Contains(fix.Intent, want) {
				t.Fatalf("intent missing %q: %s", want, fix.Intent)
			}
		}
		if !strings.Contains(fix.Basis, "attempt red-1") || !strings.Contains(fix.Basis, "attempt red-2") {
			t.Fatal(fix.Basis)
		}
		register.FixGoal, register.FixOpenedAt = fix.ID, now.UTC().Format(time.RFC3339Nano)
		f.TrunkRedEntry = register
		return f, nil
	}
	git := b.seams.Git
	b.seams.Git = func(dir string, args ...string) (string, error) {
		if args[0] == "show" && strings.Contains(args[1], "flake-fix-") {
			if !strings.HasPrefix(args[1], goal.AcceptedRef+":") {
				t.Fatalf("the report read a stale main instead of the accepted goal: %v", args)
			}
			return flakeGoalFixture(register.FixGoal, register.FixOpenedAt), nil
		}
		return git(dir, args...)
	}
	first := b.step(bedNow)
	if calls != 0 || len(first.Interventions) != 0 || !strings.Contains(first.Line, "confirmed-flake") {
		t.Fatalf("first sighting opened: %+v", first)
	}
	status := ReadStatus(b.home, b.record, b.view, b.seams)
	if row := healthRow(status.Health, "confirmed-flake"); row == nil || row.Act != "one more sighting opens its fix goal" {
		t.Fatalf("row=%+v", row)
	}
	register.Sightings = append(register.Sightings, goal.TrunkRedSighting{Opid: "two", Attempt: "red-2", LogPath: "red-2.log", Load: 33, SeenAt: bedNow.Format(time.RFC3339), Rerun: &goal.TrunkRedRerun{Attempt: "green-2"}})
	second := b.step(bedNow.Add(time.Minute))
	if calls != 1 || len(second.Interventions) != 1 || second.Interventions[0].Row != "confirmed-flake" || second.Interventions[0].What != "opened "+register.FixGoal || second.Interventions[0].Why == "" || second.Interventions[0].How == "" {
		t.Fatalf("second=%+v calls=%d", second, calls)
	}
	register.Sightings = append(register.Sightings, goal.TrunkRedSighting{Opid: "three", Attempt: "red-3", SeenAt: bedNow.Add(2 * time.Minute).Format(time.RFC3339)})
	for _, at := range []time.Time{bedNow.Add(2 * time.Minute), bedNow.Add(2 * time.Hour)} {
		third := b.step(at)
		if calls != 1 || len(third.Interventions) != 0 {
			t.Fatalf("opened twice: %+v calls=%d", third, calls)
		}
	}
	status = ReadStatus(b.home, b.record, b.view, b.seams)
	if row := healthRow(status.Health, "confirmed-flake"); row == nil || !strings.Contains(row.Act, "fix goal "+register.FixGoal+" is open (opened by the steward at ") {
		t.Fatalf("row=%+v", row)
	}
	data, err := os.ReadFile(filepath.Join(Dir(b.install), "steward-report.md"))
	if err != nil || !strings.Contains(string(data), "Flake goal opened "+register.FixGoal) || !strings.Contains(string(data), "Flake fix goal "+register.FixGoal+" still open") {
		t.Fatalf("report=%s err=%v", data, err)
	}
}

func TestLandingStewardFlakeKeepsPersonsGoal(t *testing.T) {
	t.Parallel()
	b := newStewardBed(t)
	register := flakeStewardFixture()
	register.FixGoal = "persons-fix"
	register.Sightings = append(register.Sightings, goal.TrunkRedSighting{Opid: "two", Attempt: "red-2"})
	b.seams.Incidents = func(string, string, string) ([]goal.TrunkRedEntry, error) { return []goal.TrunkRedEntry{register}, nil }
	b.seams.Steward.FlakeFix = func(goal.FlakeFact, goal.FlakeFix, time.Time) (goal.FlakeFact, error) {
		t.Fatal("opened over a person's goal")
		return goal.FlakeFact{}, nil
	}
	git := b.seams.Git
	b.seams.Git = func(dir string, args ...string) (string, error) {
		if args[0] == "show" && strings.Contains(args[1], "persons-fix") {
			return flakeGoalFixture("persons-fix", register.Opened), nil
		}
		return git(dir, args...)
	}
	run := b.step(bedNow)
	if len(run.Interventions) != 0 {
		t.Fatal(run)
	}
}

func TestLandingStewardFlakePublicationFailure(t *testing.T) {
	t.Parallel()
	b := newStewardBed(t)
	register := flakeStewardFixture()
	register.Sightings = append(register.Sightings, goal.TrunkRedSighting{Opid: "two", Attempt: "red-2"})
	b.seams.Incidents = func(string, string, string) ([]goal.TrunkRedEntry, error) { return []goal.TrunkRedEntry{register}, nil }
	b.seams.Steward.FlakeFix = func(goal.FlakeFact, goal.FlakeFix, time.Time) (goal.FlakeFact, error) {
		return goal.FlakeFact{}, errors.New("shared ledger unavailable")
	}
	run, err := StewardStep(b.home, bedNow, b.seams)
	if err == nil || register.FixGoal != "" || len(run.Interventions) != 1 || !strings.Contains(run.Interventions[0].Outcome, "failed: shared ledger unavailable") {
		t.Fatalf("run=%+v err=%v", run, err)
	}
}

func TestLandingStewardFlakeFoldsSubtestsWithSameAttempts(t *testing.T) {
	t.Parallel()
	for _, sameAttempts := range []bool{true, false} {
		t.Run(fmt.Sprint(sameAttempts), func(t *testing.T) {
			t.Parallel()
			b := newStewardBed(t)
			parent := flakeStewardFixture()
			parent.Sightings = append(parent.Sightings, goal.TrunkRedSighting{Opid: "two", Attempt: "red-2"})
			entries := []goal.TrunkRedEntry{parent}
			for _, name := range []string{"TestClock/first", "TestClock/second"} {
				child := parent
				child.ID, child.Identity = name, goal.FlakeIdentity(parent.TestUnit, name)
				child.TestName, child.Failures = name, []goal.TrunkRedFailure{{Name: name}}
				if !sameAttempts {
					child.Sightings = []goal.TrunkRedSighting{{Opid: name + "-one", Attempt: name + "-red-1"}, {Opid: name + "-two", Attempt: name + "-red-2"}}
				}
				entries = append(entries, child)
			}
			b.seams.Incidents = func(string, string, string) ([]goal.TrunkRedEntry, error) { return entries, nil }
			calls := 0
			b.seams.Steward.FlakeFix = func(f goal.FlakeFact, fix goal.FlakeFix, now time.Time) (goal.FlakeFact, error) {
				calls++
				f.FixGoal, f.FixOpenedAt = fix.ID, now.Format(time.RFC3339)
				return f, nil
			}
			b.seams.Git = func(_ string, args ...string) (string, error) {
				if args[0] == "show" && strings.Contains(args[1], "flake-fix-") {
					id := strings.TrimSuffix(filepath.Base(args[1]), ".md")
					return flakeGoalFixture(id, parent.Opened), nil
				}
				return "main", nil
			}
			run := b.step(bedNow)
			want := 3
			if sameAttempts {
				want = 1
			}
			if calls != want || len(run.Interventions) != want {
				t.Fatalf("opened %d goals with %d interventions, want %d: %+v", calls, len(run.Interventions), want, run)
			}
			if sameAttempts && run.Interventions[0].Subject != parent.Identity {
				t.Fatalf("subtests did not fold into their parent: %+v", run)
			}
		})
	}
}

func TestLandingStewardFlakePrescriptionRequiresLiveGoal(t *testing.T) {
	t.Parallel()
	for _, state := range []string{goal.StateQueued, goal.StateDone, "unreadable"} {
		t.Run(state, func(t *testing.T) {
			t.Parallel()
			b := newStewardBed(t)
			entry := flakeStewardFixture()
			entry.FixGoal = "old-fix"
			b.seams.Incidents = func(string, string, string) ([]goal.TrunkRedEntry, error) { return []goal.TrunkRedEntry{entry}, nil }
			git := b.seams.Git
			b.seams.Git = func(dir string, args ...string) (string, error) {
				if args[0] == "show" && strings.Contains(args[1], "old-fix.md") {
					if state == "unreadable" {
						return "", errors.New("missing goal")
					}
					if state == goal.StateDone && strings.Contains(args[1], "plans/goals/") {
						return "", errors.New("archived goal")
					}
					file, _ := goal.ParseFile([]byte(flakeGoalFixture(entry.FixGoal, entry.Opened)))
					file.State = state
					if state == goal.StateDone {
						file.Conclude = "Fixed."
					}
					return string(goal.RenderFile(file)), nil
				}
				return git(dir, args...)
			}
			status := ReadStatus(b.home, b.record, b.view, b.seams)
			row := healthRow(status.Health, "confirmed-flake")
			if row == nil {
				t.Fatal(status.Problems)
			}
			if strings.Contains(row.Act, " is open") != (state == goal.StateQueued) {
				t.Fatalf("state=%s row=%+v", state, row)
			}
			if state == "unreadable" && !strings.Contains(row.Act, "cannot be read") {
				t.Fatal(row.Act)
			}
			data, err := json.Marshal(status.Health)
			if err != nil {
				t.Fatal(err)
			}
			var saved Health
			if err := json.Unmarshal(data, &saved); err != nil {
				t.Fatal(err)
			}
			if restored := healthRow(&saved, "confirmed-flake"); restored == nil || restored.Act != row.Act {
				t.Fatalf("serialized health lost the fix state: %s", data)
			}
		})
	}
}
