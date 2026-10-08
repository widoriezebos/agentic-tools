package plain

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

// PolicySubject identifies an act request independently of its notification.
type PolicySubject struct {
	Incidents    []IncidentIdentity `json:"incidents,omitempty"`
	ProofAttempt string             `json:"proof-attempt,omitempty"`
	Tree         string             `json:"tree,omitempty"`
	Lane         lane.Record        `json:"lane"`
	Policy       string             `json:"policy"`
	Act          string             `json:"act"`
	BatchID      string             `json:"batch-id,omitempty"`
	Members      []GoalSHA          `json:"members,omitempty"`
}

type PolicyRequest struct {
	Subject  PolicySubject `json:"subject"`
	Required []string      `json:"required"`
	Evidence string        `json:"evidence"`
}

// PolicyQuestionSatisfied recognizes only the effect recorded for this subject.
// A text answer or a withdrawn notification supplies no selection authority.
func PolicyQuestionSatisfied(install string, q channel.Question) (bool, error) {
	if channel.LaneStopCommand(q) == "" || !strings.HasPrefix(q.Facts[1], "lane policy: ") {
		return false, nil
	}
	var subject PolicySubject
	if err := json.Unmarshal([]byte(strings.TrimPrefix(q.Facts[1], "lane policy: ")), &subject); err != nil {
		return false, err
	}
	if subject.Policy == "landing.trunk-red" && subject.Act == "trunk-exception" && len(subject.Members) == 1 {
		member := subject.Members[0]
		entry, known, err := Latest(install, member.Goal)
		if err != nil || !known || entry.State != StateWaiting || entry.SHA != member.SHA {
			return false, err
		}
		incidents := []goal.TrunkRedEntry{}
		for _, identity := range subject.Incidents {
			incidents = append(incidents, goal.TrunkRedEntry{Identity: identity.Identity, Opened: identity.Opened, Sightings: []goal.TrunkRedSighting{{Opid: identity.FirstSeen}}})
		}
		return exceptionCovers(entry, incidents, subject.Lane), nil
	}
	batch, err := ReadBatch(install)
	if err != nil {
		return false, err
	}
	return subject.Policy == "landing.batch" && subject.Act == "selection" && batch != nil && batch.ID == subject.BatchID && batch.Lane == subject.Lane && batch.Person != nil && batch.Person.Destination == subject.Lane && slices.Equal(batch.Person.Subject, subject.Members) && slices.Equal(batch.Members, subject.Members), nil
}

// PolicyRequests observes pending selections; reading status selects nothing.
func PolicyRequests(install string, effects ...ProveSeams) ([]PolicyRequest, error) {
	batch, err := ReadBatch(install)
	if err != nil {
		return nil, err
	}
	requests := []PolicyRequest{}
	if batch != nil && batch.State == BatchPrepared && batch.Person == nil && batch.Selector.Value == "person" {
		goals := []string{}
		for _, member := range batch.Members {
			goals = append(goals, member.Goal)
		}
		requests = append(requests, PolicyRequest{Subject: PolicySubject{Lane: batch.Lane, Policy: "landing.batch", Act: "selection", BatchID: batch.ID, Members: batch.Members}, Required: []string{"metasystem", "landing", "run", "--goals", strings.Join(goals, ",")}, Evidence: batchPath(install)})
	}
	if len(effects) > 0 && effects[0].Lane != nil {
		seams := effects[0]
		record, err := seams.Lane()
		if err != nil {
			return requests, err
		}
		waiting, err := Waiting(install)
		if err != nil || len(waiting) == 0 {
			return requests, err
		}
		main, err := checkoutGit(record.Root, seams).main()
		if err != nil {
			return requests, err
		}
		waiting, err = Landed(waiting, func(sha string) (bool, error) { return checkoutGit(record.Root, seams).contains(main, sha) })
		if err != nil {
			return requests, err
		}
		incidents, incidentErr := seams.incidents(install, record.Root, main)
		value, policyErr := PolicyValue{Value: "auto"}, error(nil)
		if seams.Policy != nil {
			value, policyErr = seams.Policy("landing.trunk-red")
		}
		name := "NAME"
		if enrollment, err := humanauthority.ReadEnrollment(record.Install); err == nil && enrollment.Human != "" {
			name = enrollment.Human
		}
		for _, entry := range waiting {
			personName := name
			target := record.Root
			if filepath.IsAbs(entry.Seat) {
				target = entry.Seat
				if enrollment, err := humanauthority.ReadEnrollment(entry.Seat); err == nil && enrollment.Human != "" {
					personName = enrollment.Human
				}
			}
			if entry.Exception != nil && entry.Exception.Person != nil {
				personName = entry.Exception.Person.Person
			}
			if entry.State != StateWaiting {
				continue
			}
			err := incidentErr
			if err == nil {
				err = TrunkDecision(entry, incidents, value, policyErr, record)
			}
			if err == nil {
				continue
			}
			requests = append(requests, PolicyRequest{Subject: PolicySubject{Lane: record, Policy: "landing.trunk-red", Act: "trunk-exception", Members: []GoalSHA{{Goal: entry.Goal, SHA: entry.SHA}}, Incidents: openIncidents(incidents)}, Required: ExceptionCommand(entry.Goal, target, personName), Evidence: err.Error()})
		}
	}
	return requests, nil
}

// SyncPolicyQuestion recovers requests and their closure from authoritative
// effects. Its index is rebuildable; a failed sync never repeats the act.
func SyncPolicyQuestion(install string, machine func(string) (string, error), now time.Time, effects ...ProveSeams) error {
	return withLock(install, func() error {
		batch, err := ReadBatch(install)
		if err != nil {
			return err
		}
		if batch != nil && batch.Person != nil && batch.Person.BarrenStopAt != "" {
			if err := closeMatchingStopsLocked(install, "person selected batch "+batch.ID, batch.Person.CheckedAt, func(stop Stop) bool {
				return stop.Loop == "lane-return" && stop.Subject == "lane" && stop.At == batch.Person.BarrenStopAt
			}); err != nil {
				return err
			}
		}
		// Recorded effects recover closure after an interrupted question sync.
		returns, err := readLines[Line](queuePath(install))
		if err != nil {
			return err
		}
		for _, returned := range returns {
			latest, known, readErr := Latest(install, returned.Goal)
			if readErr != nil {
				return readErr
			}
			if known && latest.State == StateReturned && latest.SHA == returned.SHA && latest.ReturnedAt == returned.At && returned.Outcome == StateReturned && returned.ReturnOrigin != nil {
				if err := closeGoalStopsLocked(install, returned.Goal, "recorded return "+returned.Goal, now, returned.SHA); err != nil {
					return err
				}
			}
		}
		for _, path := range []string{resultsPath(install), gatesPath(install)} {
			results, err := readLines[Result](path)
			if err != nil {
				return err
			}
			for _, result := range results {
				if result.ClassificationPerson != nil && !result.ClassificationPending {
					if err := closeSubjectStopsLocked(install, "lane-classify", result.Attempt, "recorded person classification", now); err != nil {
						return err
					}
				}
			}
		}
		requests, err := PolicyRequests(install, effects...)
		if err != nil {
			return err
		}
		pending := [][]string{}
		for _, request := range requests {
			data, err := json.Marshal(request.Subject)
			if err != nil {
				return err
			}
			pending = append(pending, []string{strings.Join(request.Required, " "), "lane policy: " + string(data), "evidence: " + request.Evidence})
		}
		stops, err := readLines[Stop](stopsPath(install))
		if err != nil {
			return err
		}
		seen := stopSet{}
		for i := len(stops) - 1; i >= 0; i-- {
			stop := stops[i]
			if seen.contains(stop) {
				continue
			}
			seen.add(stop)
			if stop.Decision == "stop" && strings.HasPrefix(stop.Handoff, "ask ") {
				data, err := json.Marshal(stop)
				if err != nil {
					return err
				}
				pending = append(pending, []string{stop.Command(), "lane stop: " + string(data), "evidence: " + stop.Evidence})
			}
		}
		questions, _ := channel.WalkQuestions(install)
		if len(effects) == 0 {
			for _, q := range questions {
				if q.State == "closed" || channel.LaneStopCommand(q) == "" || !strings.HasPrefix(q.Facts[1], "lane policy: ") {
					continue
				}
				var subject PolicySubject
				if json.Unmarshal([]byte(strings.TrimPrefix(q.Facts[1], "lane policy: ")), &subject) != nil || subject.Policy != "landing.trunk-red" {
					continue
				}
				satisfied, err := PolicyQuestionSatisfied(install, q)
				if err != nil {
					return err
				}
				if !satisfied && len(subject.Members) == 1 {
					current, known, err := Latest(install, subject.Members[0].Goal)
					if err != nil {
						return err
					}
					if known && current.State == StateWaiting && current.SHA == subject.Members[0].SHA {
						pending = append(pending, q.Facts)
					}
				}
			}
		}
		if len(pending) == 0 && !slices.ContainsFunc(questions, func(q channel.Question) bool { return channel.LaneStopCommand(q) != "" && q.State != "closed" }) {
			return nil
		}
		name, err := machine(install)
		if err != nil {
			return err
		}
		index := map[string]string{}
		for _, q := range questions {
			if channel.LaneStopCommand(q) == "" || q.Machine != name {
				continue
			}
			matching := slices.ContainsFunc(pending, func(facts []string) bool { return slices.Equal(q.Facts, facts) })
			if matching && q.State != "closed" {
				index[q.Facts[1]] = q.ID
			} else if !matching && q.State != "closed" {
				because := "superseded by current lane state"
				if strings.HasPrefix(q.Facts[1], "lane policy: ") {
					answered, err := PolicyQuestionSatisfied(install, q)
					if err != nil {
						return err
					}
					if answered {
						because = "answered by the recorded person act"
					}
				}
				if _, err := channel.Withdraw(install, q.ID, because, nil, channel.DestinationConfig{}); err != nil {
					return err
				}
			}
		}
		// The subject index is rebuilt from the questions. A readable old index
		// also identifies a pending question whose own JSON became unreadable.
		reference := filepath.Join(Dir(install), "stop-question")
		if raw, err := os.ReadFile(reference); err == nil && len(raw) > 0 && len(pending) > 0 {
			prior := map[string]string{}
			_ = json.Unmarshal(raw, &prior)
			for subject, id := range prior {
				if !slices.ContainsFunc(pending, func(facts []string) bool { return facts[1] == subject }) {
					continue
				}
				if _, err := channel.ReadQuestion(install, id); err != nil {
					return fmt.Errorf("the lane's stop question %s cannot be read: %w", id, err)
				}
			}
		}
		for _, facts := range pending {
			if index[facts[1]] != "" {
				continue
			}
			q, err := channel.Ask(channel.AskRequest{RepoRoot: install, About: "lane", Kind: "other", Machine: name, Lineage: lane.AgentLineage, Facts: facts, Recommendation: "Run the command above; its recorded effect closes this request.", Now: now})
			if err != nil {
				return err
			}
			index[facts[1]] = q.ID
		}
		if len(pending) == 0 {
			if err := os.Remove(reference); err != nil && !os.IsNotExist(err) {
				return err
			}
			return nil
		}
		data, err := json.Marshal(index)
		if err != nil {
			return err
		}
		_, err = atomicfile.WriteFile(reference, data, 0600, "")
		return err
	})
}
