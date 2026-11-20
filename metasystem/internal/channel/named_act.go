package channel

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/project"
)

// NamedAct identifies the completed operation an option asks for. Reasons
// and presentation flags are deliberately absent from its identity.
type NamedAct struct {
	Verb    string `json:"verb"`
	Goal    string `json:"goal"`
	Target  string `json:"target"`
	Finding string `json:"finding,omitempty"`
}

func (a NamedAct) Command() []string {
	if a.Verb == "goal accept-risk" {
		return []string{"metasystem", "goal", "accept-risk", a.Goal, "--review", a.Target, "--finding", a.Finding}
	}
	return []string{"metasystem", "work", "stop", "run:" + a.Target}
}
func (a NamedAct) valid() bool {
	return a.Goal != "" && a.Target != "" && ((a.Verb == "goal accept-risk" && a.Finding != "") || (a.Verb == "work stop" && a.Finding == ""))
}

type ActEvidence struct {
	RecordedAt time.Time            `json:"recordedAt"`
	ID         string               `json:"id"`
	Act        NamedAct             `json:"act"`
	Person     string               `json:"person"`
	Machine    string               `json:"machine"`
	At         time.Time            `json:"at"`
	Source     string               `json:"source"`
	Command    []string             `json:"command"`
	Proof      humanauthority.Proof `json:"proof"`
}

const namedActPrefix = "records/channel-acts/"

// RecordNamedAct is called only after the command has completed successfully.
// Goal acts travel in the existing shared ledger; run stops stay on their host.
func RecordNamedAct(root string, ep goal.Endpoint, act NamedAct, person, machine string, at time.Time, proof humanauthority.Proof) (ActEvidence, error) {
	var evidence ActEvidence
	if !act.valid() {
		return evidence, fmt.Errorf("the completed act has no exact subject")
	}
	direct := proof.Helm == nil && proof.EnrolledTerminalFor(root)
	reply := proof.ChannelWordFor(root)
	if !direct && !reply {
		return evidence, nil
	}
	id, err := goal.NewOperationULID()
	if err != nil {
		return evidence, err
	}
	source := "terminal"
	if reply {
		source = "channel"
	}
	evidence = ActEvidence{RecordedAt: time.Now().UTC(), ID: id, Act: act, Person: person, Machine: machine, At: at.UTC(), Source: source, Command: act.Command(), Proof: proof}
	if person == "" || machine == "" || at.IsZero() {
		return ActEvidence{}, fmt.Errorf("the completed act needs its person, machine and time")
	}
	if act.Verb == "goal accept-risk" {
		data, err := json.Marshal(evidence)
		if err != nil {
			return evidence, err
		}
		result, err := goal.Publish(ep, goal.PublishRequest{Opid: goal.Opid(id, machine, "question-act"), Machine: machine, Lineage: "question-act", Intent: goal.Intent{Verb: "question-act", Targets: []string{act.Goal}}, Message: "record completed human act", Mutate: func(string) ([]goal.Change, error) {
			return []goal.Change{{Path: namedActPrefix + id + ".json", Content: data}}, nil
		}})
		if err != nil {
			return evidence, err
		}
		if result.Outcome != goal.OutcomeConfirmed {
			return evidence, fmt.Errorf("the completed act could not be shared: %s", result.Detail)
		}
	}
	return evidence, writeJSON(filepath.Join(channelRoot(root), "named-acts", id+".json"), evidence)
}

func readNamedActs(root string, resolve func(string) (goal.Endpoint, error), shared bool) ([]ActEvidence, error) {
	files := map[string][]byte{}
	paths, err := filepath.Glob(filepath.Join(channelRoot(root), "named-acts", "*.json"))
	if err != nil {
		return nil, err
	}
	for _, path := range paths {
		data, e := os.ReadFile(path)
		if e != nil {
			return nil, e
		}
		files[path] = data
	}
	if shared {
		ep, e := resolve(root)
		if e != nil {
			return nil, e
		}
		rows, e := goal.ReadRecordFiles(ep, namedActPrefix)
		if e != nil {
			return nil, e
		}
		for path, data := range rows {
			files[path] = data
		}
	}
	byID := map[string]ActEvidence{}
	for _, data := range files {
		var a ActEvidence
		if err := json.Unmarshal(data, &a); err != nil {
			return nil, err
		}
		if a.ID == "" || !a.Act.valid() || a.At.IsZero() || a.Person == "" {
			return nil, fmt.Errorf("a completed act record is incomplete")
		}
		// This checks retained evidence, never grants authority to a reader.
		terminal := a.Proof.Helm == nil && a.Proof.RecordedEnrolled()
		channel := a.Proof.Outcome == humanauthority.OutcomeVerifiedChannel && a.Proof.ChannelUser != "" && a.Proof.ChannelStep > 0
		if terminal || channel {
			byID[a.ID] = a
		}
	}
	out := []ActEvidence{}
	for _, a := range byID {
		out = append(out, a)
	}
	slices.SortFunc(out, func(a, b ActEvidence) int {
		if n := a.At.Compare(b.At); n != 0 {
			return n
		}
		return strings.Compare(a.ID, b.ID)
	})
	return out, nil
}

func hasNamedActs(q Question) bool {
	for _, o := range q.Options {
		if o.NamedAct != nil {
			return true
		}
	}
	return false
}

// ReconcileNamedActs runs before callers decide whether a question is open.
func ReconcileNamedActs(root string, resolve func(string) (goal.Endpoint, error)) error {
	if resolve == nil {
		resolve = project.PublicationEndpoint
	}
	questions, _ := WalkQuestions(root)
	needed := false
	for _, q := range questions {
		if q.State == "open" && q.RetiredAt.IsZero() && hasNamedActs(q) {
			needed = true
			break
		}
	}
	if !needed {
		return nil
	}
	return withPollLock(root, func() error { return reconcileNamedActs(root, resolve) })
}
func reconcileNamedActs(root string, resolve func(string) (goal.Endpoint, error)) error {
	questions, _ := WalkQuestions(root)
	needed, shared := false, false
	for _, q := range questions {
		if q.State == "open" && q.RetiredAt.IsZero() && hasNamedActs(q) {
			needed = true
			for _, o := range q.Options {
				if o.NamedAct != nil && o.NamedAct.Verb == "goal accept-risk" {
					shared = true
				}
			}
		}
	}
	if !needed {
		return nil
	}
	acts, err := readNamedActs(root, resolve, shared)
	if err != nil {
		return err
	}
	for _, q := range questions {
		if q.Answer != nil && q.Answer.Act != nil && q.Answer.Phase != "closed" {
			if err := advanceAnswerWithEndpoint(context.Background(), PollConfig{RepoRoot: root, Now: q.Answer.At, Destination: "fleet"}, &q, resolve); err != nil {
				return err
			}
		}
		if q.State != "open" || q.Answer != nil || !q.RetiredAt.IsZero() {
			continue
		}
		for _, a := range acts {
			if (q.ActPerson != "" && !strings.EqualFold(q.ActPerson, a.Person)) || (!q.ActsOpenedAt.IsZero() && a.RecordedAt.Before(q.ActsOpenedAt)) || a.Act.Goal != q.Goal || a.At.Before(q.OpenedAt) || slices.Contains(q.ActsBefore, a.ID) {
				continue
			}
			for _, o := range q.Options {
				if o.NamedAct == nil || !reflect.DeepEqual(*o.NamedAct, a.Act) {
					continue
				}
				id, err := goal.NewOperationULID()
				if err != nil {
					return err
				}
				q.Answer = &Answer{Machine: a.Machine, Lineage: "question-act", Provider: a.Source, Text: o.Label, UserID: a.Person, At: a.At, Step: 1, ULID: id, Opid: goal.Opid(id, a.Machine, "question-act"), Ref: MessageRef{ID: a.ID, ThreadID: q.ID}, Phase: "matched", Act: &a}
				if err := writeJSON(questionPath(root, q.ID), q); err != nil {
					return err
				}
				if err := advanceAnswerWithEndpoint(context.Background(), PollConfig{RepoRoot: root, Now: a.At, Destination: "fleet"}, &q, resolve); err != nil {
					return err
				}
				break
			}
			if q.Answer != nil {
				break
			}
		}
	}
	return nil
}

func validateNamedActs(r AskRequest) error {
	seen := map[NamedAct]bool{}
	for _, o := range r.Options {
		if o.NamedAct == nil {
			continue
		}
		a := *o.NamedAct
		if !a.valid() || a.Goal != r.Goal || r.Goal == "" || r.Kind != "other" {
			return fmt.Errorf("an option's command must name this goal and its exact finding or run")
		}
		if seen[a] {
			return fmt.Errorf("two options cannot name the same command")
		}
		seen[a] = true
	}
	return nil
}

// NamedActIDs freezes the acts already recorded when a question is asked,
// including acts sharing the ask's clock value.
func NamedActIDs(root string, resolve func(string) (goal.Endpoint, error)) ([]string, error) {
	if resolve == nil {
		resolve = project.PublicationEndpoint
	}
	acts, err := readNamedActs(root, resolve, true)
	if err != nil {
		return nil, err
	}
	ids := []string{}
	for _, a := range acts {
		ids = append(ids, a.ID)
	}
	return ids, nil
}
