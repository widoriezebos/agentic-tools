package plain

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

func flakePrescription(f goal.FlakeFact, state string) string {
	if f.FixGoal == "" {
		return "one more sighting opens its fix goal"
	}
	if state == "" {
		return "fix goal " + f.FixGoal + " cannot be read"
	}
	if state == goal.StateDone || state == goal.StateAbandoned {
		return fmt.Sprintf("fix goal %s is %s; a new sighting needs its fix goal", f.FixGoal, state)
	}
	if f.FixOpenedAt == "" {
		return "fix goal " + f.FixGoal + " is open"
	}
	return fmt.Sprintf("fix goal %s is open (opened by the steward at %s)", f.FixGoal, lane.LocalText(f.FixOpenedAt))
}

func stewardFlakeReady(h *Health, row Fired) bool {
	for _, f := range h.Flakes {
		if row.Row == "confirmed-flake" && f.Identity == row.Subject {
			for _, parent := range h.Flakes {
				if parent.TestUnit != f.TestUnit || parent.Closed != nil || parent.EntryClass() != goal.TrunkRedClassFlake || !strings.HasPrefix(f.TestName, parent.TestName+"/") || len(parent.Sightings) != len(f.Sightings) {
					continue
				}
				if !slices.ContainsFunc(f.Sightings, func(s goal.TrunkRedSighting) bool {
					return !slices.ContainsFunc(parent.Sightings, func(p goal.TrunkRedSighting) bool { return p.Attempt == s.Attempt })
				}) {
					return false
				}
			}
			return f.FixGoal == "" && len(f.Sightings) >= 2
		}
	}
	return false
}

func stewardFlake(h *Health, row Fired, now time.Time, effects StewardEffects) (Intervention, error) {
	entry := Intervention{At: now.UTC().Format(time.RFC3339Nano), Row: row.Row, Subject: row.Subject, How: "goal open through the Go goal publication path"}
	for i, f := range h.Flakes {
		if f.Identity != row.Subject {
			continue
		}
		var sightings []string
		for _, s := range f.Sightings {
			sightings = append(sightings, fmt.Sprintf("attempt %s, log %s, load %g", s.Attempt, s.LogPath, s.Load))
		}
		basis := strings.Join(sightings, "; ")
		id := "flake-fix-" + strings.Trim(strings.Map(func(r rune) rune {
			if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
				return r
			}
			return '-'
		}, strings.ToLower(f.TestUnit+"-"+f.TestName)), "-")
		fix := goal.FlakeFix{Identity: f.Identity, ID: id, Tier: 3, Priority: 1, Basis: basis,
			Intent: fmt.Sprintf("the test %s in %s failed %d times in the landing lane and was green on every named repeat (%s); make it hold under load without a wall clock (R-104: artificial clocks, never load-fragile tests)", f.TestName, f.TestUnit, len(f.Sightings), basis),
			Next:   "fix the test, run it 20 times under -p 4 with the package, hand it in"}
		entry.What, entry.Why = "opened "+id, basis
		if effects.FlakeFix == nil {
			return entry, fmt.Errorf("the flake fix goal verb is unavailable")
		}
		updated, err := effects.FlakeFix(f, fix, now)
		if err != nil {
			return entry, err
		}
		h.Flakes[i] = updated
		if h.FlakeGoalStates == nil {
			h.FlakeGoalStates = map[string]string{}
		}
		h.FlakeGoalStates[updated.FixGoal] = goal.StateQueued
		entry.What = "opened " + updated.FixGoal
		entry.Outcome = flakePrescription(updated, goal.StateQueued)
		return entry, nil
	}
	return entry, fmt.Errorf("the confirmed flaky test is absent from health")
}

func readFlakeFixGoal(install, checkout, id string, seams ProveSeams) (*goal.GoalFile, error) {
	data, err := seams.git(checkout, "show", goal.AcceptedRef+":"+relativeLanePath(install, checkout, "plans/goals/"+id+".md"))
	if err != nil {
		data, err = seams.git(checkout, "show", goal.AcceptedRef+":"+relativeLanePath(install, checkout, "records/goals/"+id+".md"))
	}
	if err != nil {
		return nil, err
	}
	file, problems := goal.ParseFile([]byte(data))
	if len(problems) != 0 {
		return nil, fmt.Errorf("the flake fix goal cannot be read: %v", problems)
	}
	return file, nil
}
