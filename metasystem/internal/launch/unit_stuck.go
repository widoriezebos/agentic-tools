package launch

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// UnitStuckLimits bound the age of each launch kind and a running unit's rounds.
type UnitStuckLimits struct {
	BuildMin, ProofMin, ReadMin, Rounds int
}

// UnitStanding is one running unit's observation. Unreadable prevents a
// partial reading from either opening or clearing an alert.
type UnitStanding struct {
	Goal, Unit, Step, Kind, Launch, Path string
	Minutes, Rounds, MaxRounds, Limit    int
	StartedAt                            time.Time
	Stuck                                bool
	Unreadable                           string
}

// UnitStandings reads every launch of the newest round and chooses the one
// furthest past its own kind's bound. Finished runs are not stuck units.
// A missing root or launch has no started work; other read failures are unknown.
func UnitStandings(unitRoot string, store Store, limits UnitStuckLimits, now time.Time) ([]UnitStanding, error) {
	if unitRoot == "" {
		root, err := store.root()
		if err != nil {
			return nil, err
		}
		unitRoot = filepath.Join(filepath.Dir(root), "unit")
	}
	entries, err := os.ReadDir(unitRoot)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var standings []UnitStanding
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		standing := UnitStanding{Path: filepath.Join(unitRoot, entry.Name(), "run.json")}
		data, err := os.ReadFile(standing.Path)
		if os.IsNotExist(err) {
			continue
		}
		var run UnitRunRecord
		if err == nil {
			err = json.Unmarshal(data, &run)
		}
		if err != nil {
			standing.Unreadable = fmt.Sprintf("%s: %v", standing.Path, err)
			standings = append(standings, standing)
			continue
		}
		if run.State != "running" {
			continue
		}
		standing.Goal, standing.Unit = run.Goal, run.Unit
		standing.Rounds, standing.MaxRounds = len(run.Rounds), run.MaxRounds
		if run.Goal == "" || run.Unit == "" {
			standing.Unreadable = standing.Path + ": the running unit has no goal or unit name"
		}
		if standing.Rounds >= limits.Rounds {
			standing.Stuck, standing.Limit = true, limits.Rounds
		}
		var furthest time.Duration
		if len(run.Rounds) > 0 {
			for _, step := range run.Rounds[len(run.Rounds)-1].Steps {
				if step.LaunchID == "" {
					continue
				}
				launch, err := store.Read(step.LaunchID)
				if os.IsNotExist(err) {
					continue
				}
				if err != nil {
					standing.Unreadable = err.Error()
					continue
				}
				if launch.State != Running {
					continue
				}
				started, err := time.Parse(time.RFC3339, launch.StartedAt)
				limit := map[string]int{"build": limits.BuildMin, "proof": limits.ProofMin, "read": limits.ReadMin}[launch.Kind]
				if err != nil || limit < 1 {
					standing.Unreadable = fmt.Sprintf("launch %s has an unreadable start time or kind", launch.ID)
					continue
				}
				age := now.Sub(started)
				if over := age - time.Duration(limit)*time.Minute; over > furthest {
					furthest = over
					standing.Step, standing.Kind, standing.Launch = step.Name, launch.Kind, launch.ID
					standing.Minutes, standing.Limit, standing.StartedAt = int(age/time.Minute), limit, started
					standing.Stuck = true
				}
			}
		}
		standings = append(standings, standing)
	}
	return standings, nil
}
