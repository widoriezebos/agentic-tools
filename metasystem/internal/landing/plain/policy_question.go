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
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

// PolicySubject identifies an act request independently of its notification.
type PolicySubject struct {
	Lane    lane.Record `json:"lane"`
	Policy  string      `json:"policy"`
	Act     string      `json:"act"`
	BatchID string      `json:"batch-id,omitempty"`
	Members []GoalSHA   `json:"members,omitempty"`
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
	batch, err := ReadBatch(install)
	if err != nil {
		return false, err
	}
	return subject.Policy == "landing.batch" && subject.Act == "selection" && batch != nil && batch.ID == subject.BatchID && batch.Lane == subject.Lane && batch.Person != nil && batch.Person.Destination == subject.Lane && slices.Equal(batch.Person.Subject, subject.Members) && slices.Equal(batch.Members, subject.Members), nil
}

// PolicyRequests observes pending selections; reading status selects nothing.
func PolicyRequests(install string) ([]PolicyRequest, error) {
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
	return requests, nil
}

// SyncPolicyQuestion recovers requests and their closure from authoritative
// effects. Its index is rebuildable; a failed sync never repeats the act.
func SyncPolicyQuestion(install string, machine func(string) (string, error), now time.Time) error {
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
		requests, err := PolicyRequests(install)
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
		seen := map[string]bool{}
		for i := len(stops) - 1; i >= 0; i-- {
			stop := stops[i]
			key := stopKey(stop)
			if seen[key] {
				continue
			}
			seen[key] = true
			if stop.Decision == "stop" && strings.HasPrefix(stop.Handoff, "ask ") {
				data, err := json.Marshal(stop)
				if err != nil {
					return err
				}
				pending = append(pending, []string{stop.Command(), "lane stop: " + string(data), "evidence: " + stop.Evidence})
			}
		}
		questions, _ := channel.WalkQuestions(install)
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
						because = "answered by recorded person selection " + batch.ID
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
