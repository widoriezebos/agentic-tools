package channel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/project"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/shellquote"
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
func (a NamedAct) DisplayCommand() string {
	words := a.Command()
	if a.Verb == "goal accept-risk" {
		words = append(words, "--reason", "TEXT")
	}
	for i := range words {
		words[i] = shellquote.Word(words[i])
	}
	return strings.Join(words, " ")
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
	if act.Verb == "work stop" {
		questions, unreadable := WalkQuestions(root)
		if len(unreadable) > 0 {
			return evidence, fmt.Errorf("the open questions could not be read")
		}
		var needed []Question
		var pending *ActEvidence
		for _, q := range questions {
			if q.State != "open" || !q.RetiredAt.IsZero() {
				continue
			}
			if q.Answer != nil {
				if q.Answer.Act != nil && q.Answer.Act.Act == act {
					pending = q.Answer.Act
				}
				continue
			}
			for _, o := range q.Options {
				if o.NamedAct != nil && *o.NamedAct == act {
					needed = append(needed, q)
					break
				}
			}
		}
		if len(needed) == 0 {
			if pending != nil {
				return *pending, nil
			}
			return evidence, nil
		}
		acts, _ := readNamedActs(root, nil, false)
		for _, a := range acts {
			if a.Act != act {
				continue
			}
			coversAll := true
			for _, q := range needed {
				if !namedActMatches(q, a) {
					coversAll = false
					break
				}
			}
			if coversAll {
				return a, nil
			}
		}
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
	var diagnostics error
	paths, err := filepath.Glob(filepath.Join(channelRoot(root), "named-acts", "*.json"))
	if err != nil {
		return nil, err
	}
	for _, path := range paths {
		data, e := os.ReadFile(path)
		if e != nil {
			diagnostics = errors.Join(diagnostics, fmt.Errorf("act record %s could not be read", filepath.Base(path)))
			continue
		}
		files[path] = data
	}
	if shared {
		ep, e := resolve(root)
		if e == nil {
			var rows map[string][]byte
			rows, e = goal.ReadRecordFiles(ep, namedActPrefix)
			for path, data := range rows {
				files[path] = data
			}
		}
		if e != nil {
			diagnostics = errors.Join(diagnostics, fmt.Errorf("the local goal ledger could not be read: %w", e))
		}
	}
	byID := map[string]ActEvidence{}
	for path, data := range files {
		var a ActEvidence
		if err := json.Unmarshal(data, &a); err != nil {
			diagnostics = errors.Join(diagnostics, fmt.Errorf("skipped unreadable act record %s", filepath.Base(path)))
			continue
		}
		if a.ID == "" || !a.Act.valid() || a.At.IsZero() || a.Person == "" {
			diagnostics = errors.Join(diagnostics, fmt.Errorf("skipped incomplete or unsupported act record %s", filepath.Base(path)))
			continue
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
	return out, diagnostics
}

func hasNamedActs(q Question) bool {
	for _, o := range q.Options {
		if o.NamedAct != nil {
			return true
		}
	}
	return false
}

func namedActMatches(q Question, a ActEvidence) bool {
	if (q.ActPerson != "" && !strings.EqualFold(q.ActPerson, a.Person)) || (!q.ActsOpenedAt.IsZero() && a.RecordedAt.Before(q.ActsOpenedAt)) || a.Act.Goal != q.Goal || a.At.Before(q.OpenedAt) || slices.Contains(q.ActsBefore, a.ID) {
		return false
	}
	for _, o := range q.Options {
		if o.NamedAct != nil && *o.NamedAct == a.Act {
			return true
		}
	}
	return false
}

// ReconcileNamedActs reads locally accepted evidence. Publication is bounded
// and runs outside the channel lock, so other questions remain usable.
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
	held, err := lock.File(filepath.Join(channelRoot(root), "named-acts.lock"), 0o644, lock.TryExclusive)
	if err != nil {
		if lock.Busy(err) {
			return ErrChannelBusy
		}
		return err
	}
	defer held.Release()
	deadline := time.Now().Add(5 * time.Second)
	var failures error
	for _, prior := range questions {
		if prior.State != "open" || !prior.RetiredAt.IsZero() || !hasNamedActs(prior) {
			continue
		}
		shared := false
		for _, o := range prior.Options {
			if o.NamedAct != nil && o.NamedAct.Verb == "goal accept-risk" {
				shared = true
			}
		}
		acts, readErr := readNamedActs(root, resolve, shared)
		var matched Question
		err := withPollLock(root, func() error {
			q, err := ReadQuestion(root, prior.ID)
			if err != nil {
				return err
			}
			if q.State != "open" || !q.RetiredAt.IsZero() || (q.Answer != nil && q.Answer.Act == nil) {
				return nil
			}
			if q.Answer == nil {
				for _, a := range acts {
					if !namedActMatches(q, a) {
						continue
					}
					for _, o := range q.Options {
						if o.NamedAct == nil || *o.NamedAct != a.Act {
							continue
						}
						id, err := goal.NewOperationULID()
						if err != nil {
							return err
						}
						q.Answer = &Answer{Machine: a.Machine, Lineage: "question-act", Provider: a.Source, Text: o.Label, UserID: a.Person, At: a.At, Step: 1, ULID: id, Opid: goal.Opid(id, a.Machine, "question-act"), Ref: MessageRef{ID: a.ID, ThreadID: q.ID}, Phase: "matched", Act: &a}
						break
					}
					break
				}
			}
			diagnostic := ""
			if readErr != nil {
				diagnostic = "could not be checked now: " + readErr.Error()
			}
			changed := q.ActCheck != diagnostic || (q.Answer != nil && prior.Answer == nil)
			q.ActCheck = diagnostic
			if changed {
				if err := writeJSON(questionPath(root, q.ID), q); err != nil {
					return err
				}
			}
			if q.Answer != nil {
				matched = q
			}
			return nil
		})
		if err != nil {
			failures = errors.Join(failures, err)
			continue
		}
		if matched.Answer == nil {
			continue
		}
		a := matched.Answer
		var publishErr error
		if a.Phase == "matched" {
			ep, e := resolve(root)
			publishErr = e
			if e == nil {
				published, e := goal.Answer(goal.VerbRequest{Endpoint: ep.WithNetworkDeadline(deadline), Actor: goal.Actor{Machine: a.Machine, Lineage: a.Lineage}, Ulid: a.ULID, Now: a.At}, matched.Goal, matched.ID, a.Text, matched.Wants, goal.AnswerProof{Provider: a.Provider, User: a.UserID, Ref: a.Ref.ThreadID + "/" + a.Ref.ID, Step: a.Step, OperationID: a.Opid})
				publishErr = e
				if e == nil && published.Outcome != goal.OutcomeConfirmed {
					publishErr = goal.Coded(published.Code, fmt.Errorf("goal answer was not confirmed: %s", published.Detail))
				}
			}
			// The completed act remains an answer even after its goal ends.
			if goal.RefusalCode(publishErr) == goal.AnswerGoalClosedCode {
				publishErr = nil
			}
		}
		err = withPollLock(root, func() error {
			q, err := ReadQuestion(root, matched.ID)
			if err != nil {
				return err
			}
			if q.State != "open" || q.Answer == nil || q.Answer.Opid != a.Opid {
				return nil
			}
			if publishErr != nil {
				diagnostic := "could not be checked now: " + publishErr.Error()
				if q.ActCheck == diagnostic {
					return nil
				}
				q.ActCheck = diagnostic
				return writeJSON(questionPath(root, q.ID), q)
			}
			q.ActCheck = ""
			q.Answer.Phase = "recorded"
			return advanceAnswerWithEndpoint(context.Background(), PollConfig{RepoRoot: root, Now: a.At, Destination: "fleet"}, &q, resolve)
		})
		failures = errors.Join(failures, publishErr, err)
	}
	return failures
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
