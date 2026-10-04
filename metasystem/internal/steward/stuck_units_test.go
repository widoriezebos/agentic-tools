package steward

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

type stuckFixture struct {
	t        *testing.T
	root     string
	now      time.Time
	pass     StuckUnits
	claims   map[string]bool
	messages []string
}

func newStuckFixture(t *testing.T) *stuckFixture {
	t.Helper()
	f := &stuckFixture{t: t, root: t.TempDir(), now: time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC), claims: map[string]bool{"goal": true}}
	if err := os.WriteFile(filepath.Join(f.root, "metasystem.conf"), []byte(""), 0600); err != nil {
		t.Fatal(err)
	}
	f.pass = StuckUnits{UnitRoot: filepath.Join(f.root, "unit"), Store: launch.Store{Root: filepath.Join(f.root, "launch")},
		Claims:  func(string, time.Time) (string, map[string]bool, error) { return "m1l", f.claims, nil },
		Deliver: func(_ string, message string) error { f.messages = append(f.messages, message); return nil }}
	return f
}

func (f *stuckFixture) write(unit, id, kind string, age, rounds int) {
	f.t.Helper()
	rec := launch.Record{ID: id, Kind: kind, State: launch.Running, StartedAt: f.now.Add(-time.Duration(age) * time.Minute).Format(time.RFC3339)}
	if err := f.pass.Store.Create(rec); err != nil {
		f.t.Fatal(err)
	}
	run := launch.UnitRunRecord{ID: "run-" + unit, Goal: "goal", Unit: unit, State: "running", MaxRounds: 4}
	for i := 0; i < rounds; i++ {
		run.Rounds = append(run.Rounds, launch.UnitRound{Number: i + 1})
	}
	run.Rounds[rounds-1].Steps = []launch.UnitStep{{Name: "r1-s1", LaunchID: id}}
	path := filepath.Join(f.pass.UnitRoot, run.ID, "run.json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		f.t.Fatal(err)
	}
	data, err := json.Marshal(run)
	if err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		f.t.Fatal(err)
	}
}

func (f *stuckFixture) tick() []AlertEpisode {
	f.t.Helper()
	if err := f.pass.Run(f.root, f.now); err != nil {
		f.t.Fatal(err)
	}
	episodes, err := AlertEpisodes(f.root)
	if err != nil {
		f.t.Fatal(err)
	}
	return episodes
}

func (f *stuckFixture) end() {
	f.t.Helper()
	if _, err := f.pass.Store.Update("build", func(r *launch.Record) error { r.State = launch.Completed; return nil }); err != nil {
		f.t.Fatal(err)
	}
}

func TestStuckUnitsOpensOneEpisodePerUnit(t *testing.T) {
	t.Parallel()
	f := newStuckFixture(t)
	f.write("2", "build", "build", 52, 2)
	f.write("3", "read", "read", 40, 1)
	episodes := f.tick()
	if len(episodes) != 2 || len(f.messages) != 2 {
		t.Fatalf("episodes=%+v notifications=%v", episodes, f.messages)
	}
	expected := "Seat m1l has run the build step of unit 2 of goal goal for 52 minutes (limit 45, round 2 of 4): stop it with `metasystem work stop j1:build`, cut the unit, or land with notes (`metasystem work land --message`)."
	found := false
	for _, e := range episodes {
		if e.ScopeID == "goal/2" {
			found = true
			if e.Owner != "pattern:stuck-unit" || strings.Split(e.Message, "\n")[0] != expected || len(e.Evidence) != 1 || e.Evidence[0].Fact != "build r1-s1 52 min, limit 45" {
				t.Fatalf("episode: %+v", e)
			}
		}
	}
	if !found {
		t.Fatal("missing unit 2")
	}
	f.now = f.now.Add(time.Minute)
	episodes = f.tick()
	if len(episodes) != 2 || len(f.messages) != 2 {
		t.Fatal("a repeated stall notified again")
	}
	for _, e := range episodes {
		if e.ScopeID == "goal/2" && (strings.Split(e.Message, "\n")[0] != expected || len(e.Evidence) != 2) {
			t.Fatalf("opening words or evidence changed incorrectly: %+v", e)
		}
	}
	if _, err := os.Stat(PatternStatePath(f.root)); !os.IsNotExist(err) {
		t.Fatalf("stuck pass wrote pattern state: %v", err)
	}
}

func TestStuckUnitsClearsWhenTheStepEnds(t *testing.T) {
	t.Parallel()
	f := newStuckFixture(t)
	f.write("2", "build", "build", 52, 1)
	f.tick()
	f.end()
	episodes := f.tick()
	if len(episodes) != 1 || !episodes[0].Cleared || episodes[0].Suppressed {
		t.Fatalf("not automatically cleared: %+v", episodes)
	}
}

func TestStuckUnitsClearsWhenTheGoalLeavesTheSeat(t *testing.T) {
	t.Parallel()
	f := newStuckFixture(t)
	f.write("2", "build", "build", 52, 1)
	f.tick()
	delete(f.claims, "goal")
	if episodes := f.tick(); len(episodes) != 1 || !episodes[0].Cleared {
		t.Fatalf("claim departure: %+v", episodes)
	}
}

func TestStuckUnitsSkipsUnclaimedGoals(t *testing.T) {
	t.Parallel()
	f := newStuckFixture(t)
	f.write("2", "build", "build", 52, 1)
	delete(f.claims, "goal")
	if episodes := f.tick(); len(episodes) != 0 || len(f.messages) != 0 {
		t.Fatalf("another seat's work alerted: %+v", episodes)
	}
}

func TestStuckUnitsHoldsUnknownOnUnreadableRecords(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"run", "launch", "launch after departure", "root", "ledger", "machine", "settings"} {
		t.Run(kind, func(t *testing.T) {
			f := newStuckFixture(t)
			f.write("2", "build", "build", 52, 1)
			before := f.tick()[0]
			switch kind {
			case "run", "launch", "launch after departure":
				path := filepath.Join(f.pass.UnitRoot, "run-2", "run.json")
				if kind != "run" {
					path = filepath.Join(f.pass.Store.Root, "build", "record.json")
				}
				if err := os.WriteFile(path, []byte("{"), 0600); err != nil {
					t.Fatal(err)
				}
				if kind == "launch after departure" {
					delete(f.claims, "goal")
				}
			case "root":
				f.pass.UnitRoot = filepath.Join(f.pass.UnitRoot, "run-2", "run.json")
			case "ledger":
				f.pass.Claims = func(string, time.Time) (string, map[string]bool, error) {
					return "", nil, errors.New("unreadable ledger")
				}
			case "machine":
				f.pass.Claims = func(string, time.Time) (string, map[string]bool, error) { return "", f.claims, nil }
			case "settings":
				if err := os.WriteFile(filepath.Join(f.root, "metasystem.conf"), []byte("steward.stuck.build-min=bad\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			episodes := f.tick()
			if len(episodes) != 1 || episodes[0].Cleared || episodes[0].Standing != StandingUnreadable || episodes[0].EpisodeID != before.EpisodeID || len(f.messages) != 1 {
				t.Fatalf("unreadable signal changed alert: %+v", episodes)
			}
		})
	}
}

func TestStuckUnitsLimitsComeFromSettings(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		key, kind string
		age       int
	}{{"build-min", "build", 2}, {"proof-min", "proof", 2}, {"read-min", "read", 2}, {"rounds", "build", 0}} {
		t.Run(c.key, func(t *testing.T) {
			f := newStuckFixture(t)
			f.write("2", "build", c.kind, c.age, 1)
			if len(f.tick()) != 0 {
				t.Fatal("default bound unexpectedly stuck")
			}
			if err := os.WriteFile(filepath.Join(f.root, "metasystem.conf"), []byte("steward.stuck."+c.key+"=1\n"), 0600); err != nil {
				t.Fatal(err)
			}
			episodes := f.tick()
			if len(episodes) != 1 || len(f.messages) != 1 {
				t.Fatalf("override ignored: %+v", episodes)
			}
			if c.key == "rounds" && !strings.Contains(episodes[0].Message, "is on round 1 of unit 2 of goal goal (limit 1)") {
				t.Fatalf("round finding: %+v", episodes[0])
			}
		})
	}
}

func TestStuckUnitsAlertsAgainAfterADismissal(t *testing.T) {
	t.Parallel()
	f := newStuckFixture(t)
	f.write("2", "build", "build", 52, 1)
	first := f.tick()[0]
	if _, _, err := ClearAlert(f.root, first.EpisodeID, AlertInvoker{}, f.now); err != nil {
		t.Fatal(err)
	}
	if len(f.tick()) != 1 || len(f.messages) != 1 {
		t.Fatal("dismissed stall notified again")
	}
	f.end()
	episodes := f.tick()
	if episodes[0].Suppressed {
		t.Fatal("clean reading did not release dismissal")
	}
	f.write("2", "again", "build", 52, 1)
	episodes = f.tick()
	if len(episodes) != 2 || len(f.messages) != 2 {
		t.Fatalf("new stall did not open a new episode: %+v", episodes)
	}
}
