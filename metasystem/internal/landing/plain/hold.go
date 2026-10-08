package plain

import (
	"fmt"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

// Exception belongs to one hand-in, ending when it lands or is returned.
type Exception struct {
	Code           string            `json:"code"`
	Reason         string            `json:"reason"`
	By             string            `json:"by"`
	Goal           string            `json:"goal,omitempty"`
	SHA            string            `json:"sha,omitempty"`
	Person         *ActProvenance    `json:"person,omitempty"`
	Incidents      *IncidentCoverage `json:"incidents,omitempty"`
	BindingUnknown bool              `json:"binding-unknown,omitempty"`
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

// IncidentIdentity pins an open generation; later sightings do not change it.
type IncidentIdentity struct {
	Identity  string `json:"identity"`
	Opened    string `json:"opened"`
	FirstSeen string `json:"first-seen"`
}

type IncidentCoverage struct {
	Open []IncidentIdentity `json:"open"`
}

func openIncidents(incidents []goal.TrunkRedEntry) []IncidentIdentity {
	open := []IncidentIdentity{}
	for _, incident := range incidents {
		if incident.Closed != nil || incident.EntryClass() != goal.TrunkRedClassTrunkRed {
			continue
		}
		first := ""
		if len(incident.Sightings) > 0 {
			first = incident.Sightings[0].Opid
		}
		open = append(open, IncidentIdentity{incident.Identity, incident.Opened, first})
	}
	slices.SortFunc(open, func(a, b IncidentIdentity) int {
		return strings.Compare(a.Identity+"/"+a.Opened+"/"+a.FirstSeen, b.Identity+"/"+b.Opened+"/"+b.FirstSeen)
	})
	return slices.Compact(open)
}

// BindException carries a fresh direct proof into this hand-in's metadata.
func BindException(subject GoalSHA, destination lane.Record, proof humanauthority.Proof, root, name, reason string, at time.Time, incidents []goal.TrunkRedEntry, readErr error) (*Exception, error) {
	if proof.Helm != nil || !proof.EnrolledTerminalFor(root) {
		return nil, fmt.Errorf("the exception requires the calling checkout's enrolled terminal")
	}
	if subject.Goal == "" || subject.SHA == "" || destination.Install == "" || destination.CustodyEpoch == 0 || reason == "" || name == "" {
		return nil, fmt.Errorf("the exception needs a known hand-in, lane, person and reason")
	}
	exception := &Exception{Code: goal.LandTrunkRedCode, Reason: reason, By: name, Goal: subject.Goal, SHA: subject.SHA,
		Person: &ActProvenance{Kind: "trunk-exception", Person: name, Root: root, CheckedAt: at.UTC(), TerminalGeneration: proof.TerminalGeneration, TerminalRef: proof.TerminalRef, Destination: destination, Subject: []GoalSHA{subject}}, BindingUnknown: readErr != nil}
	if readErr == nil {
		exception.Incidents = &IncidentCoverage{Open: openIncidents(incidents)}
	}
	return exception, nil
}

func exceptionCovers(entry Entry, incidents []goal.TrunkRedEntry, destination lane.Record) bool {
	e := entry.Exception
	if e == nil || e.Code != goal.LandTrunkRedCode || e.BindingUnknown || e.Incidents == nil || e.Goal != entry.Goal || e.SHA != entry.SHA || e.Reason == "" || e.Person == nil {
		return false
	}
	p := e.Person
	if p.Kind != "trunk-exception" || p.Person != e.By || p.Person == "" || p.Root == "" || p.CheckedAt.IsZero() || p.TerminalGeneration == 0 || p.TerminalRef.PID < 1 || !slices.Equal(p.Subject, []GoalSHA{{Goal: entry.Goal, SHA: entry.SHA}}) || p.Destination.Install == "" || p.Destination.CustodyEpoch == 0 {
		return false
	}
	if destination.Root != "" && p.Destination != destination {
		return false
	}
	for _, incident := range openIncidents(incidents) {
		if !slices.Contains(e.Incidents.Open, incident) {
			return false
		}
	}
	return true
}

// sameException ignores only the repeated observation time of an identical act.
func sameException(a, b *Exception) bool {
	if a == nil || b == nil {
		return a == b
	}
	aa, bb := *a, *b
	if aa.Person != nil && bb.Person != nil {
		ap, bp := *aa.Person, *bb.Person
		ap.CheckedAt, bp.CheckedAt = time.Time{}, time.Time{}
		aa.Person, bb.Person = &ap, &bp
	}
	return reflect.DeepEqual(aa, bb)
}

// TrunkDecision applies current advice to the exact hand-in and incident set.
func TrunkDecision(entry Entry, incidents []goal.TrunkRedEntry, value PolicyValue, readErr error, destination lane.Record) error {
	if entry.Exception != nil && entry.Exception.BindingUnknown {
		return fmt.Errorf("incident binding unknown; a person must bind this hand-in again")
	}
	if exceptionCovers(entry, incidents, destination) {
		return nil
	}
	// The policy decides only while an incident is open; an unreadable value holds nothing else.
	open := openIncidents(incidents)
	if len(open) == 0 {
		return nil
	}
	if readErr != nil {
		return fmt.Errorf("the trunk-red policy cannot be read: %w", readErr)
	}
	if value.Value != "person" {
		for _, incident := range incidents {
			if incident.Closed == nil && incident.EntryClass() == goal.TrunkRedClassTrunkRed && incident.FixGoal == entry.Goal && (entry.Fix == "" || entry.Fix == incident.Identity) {
				return nil
			}
		}
	}
	return fmt.Errorf("main has an open incident (%s); a person must admit this goal's hand-in", open[0].Identity)
}

func ExceptionCommand(goalID, target, name string) []string {
	if name == "" {
		name = "NAME"
	}
	command := []string{"metasystem", "work", "land", goalID, "--exception", goal.LandTrunkRedCode, "--reason", "TEXT", "--by", name}
	if target != "" {
		command = append(command, "--repo", target)
	}
	return command
}

// HoldEntries preserves source holds and adds fresh trunk permission holds.
func HoldEntries(entries []Entry, incidents []goal.TrunkRedEntry, effects ...ProveSeams) []Entry {
	seams := ProveSeams{}
	if len(effects) > 0 {
		seams = effects[0]
	}
	value := PolicyValue{Value: "auto"}
	var policyErr error
	if seams.Policy != nil {
		value, policyErr = seams.Policy("landing.trunk-red")
	}
	destination := lane.Record{}
	var destinationErr error
	if seams.Lane != nil {
		var err error
		destination, err = seams.Lane()
		if err != nil {
			destinationErr = err
		}
	}
	out := append([]Entry(nil), entries...)
	for index, entry := range out {
		if entry.State != StateWaiting {
			continue
		}
		err := destinationErr
		if err == nil {
			err = TrunkDecision(entry, incidents, value, policyErr, destination)
		}
		if err != nil {
			out[index].Held = true
			if out[index].Reason != "" {
				out[index].Reason += "; "
			}
			out[index].Reason += err.Error()
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

// TrunkInputs fetches main before binding permission to its current register.
func TrunkInputs(record lane.Record, seams ProveSeams) ([]goal.TrunkRedEntry, error) {
	if err := seams.fetchMain(record.Root); err != nil {
		return nil, err
	}
	main, err := checkoutGit(record.Root, seams).main()
	if err != nil {
		return nil, err
	}
	return seams.incidents(record.Install, record.Root, main)
}
