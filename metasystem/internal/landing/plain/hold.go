package plain

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// Exception belongs to one hand-in, ending when it lands or is returned.
type Exception struct {
	Code   string `json:"code"`
	Reason string `json:"reason"`
	By     string `json:"by"`
}

// ReadIncidents reads the register at main, never the lane's merged HEAD.
// A checkout that has no register has no incidents.
func ReadIncidents(install, checkout, main string, git func(string, ...string) (string, error)) ([]goal.TrunkRedEntry, error) {
	prefix, err := filepath.Rel(checkout, install)
	if err != nil {
		return nil, err
	}
	path := filepath.ToSlash(filepath.Join(prefix, "plans/goals/trunk-red.json"))
	listed, err := git(checkout, "ls-tree", "--name-only", main, "--", path)
	if err != nil || listed == "" {
		return nil, err
	}
	data, err := git(checkout, "show", main+":"+path)
	if err != nil {
		return nil, err
	}
	entries, problems := goal.ParseTrunkRed([]byte(data))
	if len(problems) != 0 {
		return nil, fmt.Errorf("main's incident register cannot be read: %v", problems)
	}
	return entries, nil
}

func (s ProveSeams) incidents(install, checkout, main string) ([]goal.TrunkRedEntry, error) {
	if s.Incidents != nil {
		return s.Incidents(install, checkout, main)
	}
	return ReadIncidents(install, checkout, main, s.git)
}

// IncidentReason is the landing hold as a person reads it, in local time.
func IncidentReason(entry goal.TrunkRedEntry) string {
	at, _ := time.Parse(time.RFC3339, entry.Opened)
	return fmt.Sprintf("main is red since %s (incident %s), and landing holds until it is fixed", at.Local().Format("15:04"), entry.Identity)
}

// HoldEntries adds the current incident hold to already derived queue states.
// Conflict and environment holds remain even for fix goals and exceptions.
func HoldEntries(entries []Entry, incidents []goal.TrunkRedEntry) []Entry {
	out := append([]Entry(nil), entries...)
	for index, entry := range out {
		if entry.State != StateWaiting || entry.Exception != nil && entry.Exception.Code == goal.LandTrunkRedCode {
			continue
		}
		fixOpen := false
		for _, incident := range incidents {
			if entry.Fix != "" && incident.Identity == entry.Fix && incident.Closed == nil && (incident.FixGoal == "" || incident.FixGoal == entry.Goal) {
				fixOpen = true
				break
			}
		}
		if fixOpen {
			continue
		}
		if red, held := goal.LandingIncident(incidents, entry.Goal); held {
			out[index].Held = true
			if out[index].Reason != "" {
				out[index].Reason += "; "
			}
			out[index].Reason += IncidentReason(red)
		}
	}
	return out
}

// incidentOnlyHolds says every waiting line is held solely by the register.
// A conflict or environment hold cannot be released by fetching main.
func incidentOnlyHolds(before, held []Entry) bool {
	waiting := false
	for index, entry := range before {
		if entry.State != StateWaiting {
			continue
		}
		waiting = true
		if entry.Held || !held[index].Held {
			return false
		}
	}
	return waiting
}
