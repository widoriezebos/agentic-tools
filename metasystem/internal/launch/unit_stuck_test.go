package launch

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestUnitStandingsReadRunningRuns(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	limits := UnitStuckLimits{BuildMin: 45, ProofMin: 30, ReadMin: 35, Rounds: 3}
	for _, c := range []struct {
		name, kind, state string
		age, rounds       int
		missing, corrupt  bool
		stuck, unread     bool
	}{
		{name: "build over", kind: "build", age: 46, rounds: 1, stuck: true},
		{name: "proof under", kind: "proof", age: 29, rounds: 1},
		{name: "read boundary", kind: "read", age: 35, rounds: 1},
		{name: "round limit", kind: "build", age: 1, rounds: 3, stuck: true},
		{name: "judgement", kind: "build", state: "awaiting-judgement", age: 46, rounds: 3},
		{name: "missing launch", kind: "build", age: 46, rounds: 1, missing: true},
		{name: "corrupt launch", kind: "build", age: 46, rounds: 1, corrupt: true, unread: true},
	} {
		t.Run(c.name, func(t *testing.T) {
			root := t.TempDir()
			store := Store{Root: filepath.Join(root, "launch")}
			runner := UnitRunner{Root: filepath.Join(root, "unit")}
			state := c.state
			if state == "" {
				state = "running"
			}
			run := UnitRunRecord{ID: "run", Goal: "goal", Unit: "2", State: state, MaxRounds: 4}
			for i := 0; i < c.rounds; i++ {
				run.Rounds = append(run.Rounds, UnitRound{Number: i + 1})
			}
			run.Rounds[len(run.Rounds)-1].Steps = []UnitStep{{Name: "r1-s1", LaunchID: "run-r1-s1"}}
			if err := runner.save(run); err != nil {
				t.Fatal(err)
			}
			if !c.missing {
				if err := store.Create(Record{ID: "run-r1-s1", Kind: c.kind, State: Running, StartedAt: now.Add(-time.Duration(c.age) * time.Minute).Format(time.RFC3339)}); err != nil {
					t.Fatal(err)
				}
			}
			if c.corrupt {
				if err := os.WriteFile(filepath.Join(store.Root, "run-r1-s1", "record.json"), []byte("{"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			got, err := UnitStandings(runner.Root, store, limits, now)
			if err != nil {
				t.Fatal(err)
			}
			if state != "running" {
				if len(got) != 0 {
					t.Fatalf("finished run: %+v", got)
				}
				return
			}
			if len(got) != 1 || got[0].Stuck != c.stuck || (got[0].Unreadable != "") != c.unread {
				t.Fatalf("standing: %+v", got)
			}
			if c.name == "build over" && (got[0].Launch != "run-r1-s1" || got[0].Minutes != 46 || got[0].Limit != 45) {
				t.Fatalf("wrong stop target or age: %+v", got[0])
			}
		})
	}
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".inputs"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "bad"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "bad", "run.json"), []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := UnitStandings(root, Store{Root: t.TempDir()}, limits, now)
	if err != nil || len(got) != 1 || got[0].Unreadable == "" {
		t.Fatalf("corrupt run: %+v %v", got, err)
	}
	if _, err := UnitStandings(filepath.Join(root, "bad", "run.json"), Store{}, limits, now); err == nil {
		t.Fatal("unlistable root was read as empty")
	}
}

func TestUnitStandingsNamesTheRunningParallelRead(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	root := t.TempDir()
	store := Store{Root: filepath.Join(root, "launch")}
	runner := UnitRunner{Root: filepath.Join(root, "unit")}
	for _, rec := range []Record{
		{ID: "build", Kind: "build", State: Running, StartedAt: now.Add(-48 * time.Minute).Format(time.RFC3339)},
		{ID: "earlier", Kind: "read", State: Running, StartedAt: now.Add(-40 * time.Minute).Format(time.RFC3339)},
		{ID: "later", Kind: "read", State: Completed, StartedAt: now.Add(-39 * time.Minute).Format(time.RFC3339)},
	} {
		if err := store.Create(rec); err != nil {
			t.Fatal(err)
		}
	}
	run := UnitRunRecord{ID: "run", Goal: "goal", Unit: "2", State: "running", MaxRounds: 4, Rounds: []UnitRound{{Steps: []UnitStep{{Name: "build", LaunchID: "build"}, {Name: "read-a", LaunchID: "earlier"}, {Name: "read-b", LaunchID: "later"}}}}}
	if err := runner.save(run); err != nil {
		t.Fatal(err)
	}
	got, err := UnitStandings(runner.Root, store, UnitStuckLimits{BuildMin: 45, ProofMin: 30, ReadMin: 35, Rounds: 3}, now)
	if err != nil || len(got) != 1 {
		t.Fatalf("read: %+v %v", got, err)
	}
	if !got[0].Stuck || got[0].Launch != "earlier" || got[0].Step != "read-a" || got[0].Kind != "read" || got[0].Limit != 35 || got[0].Minutes != 40 {
		t.Fatalf("wrong parallel read: %+v", got[0])
	}
}
