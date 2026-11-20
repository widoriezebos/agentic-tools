package channel

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
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
	ID      string               `json:"id"`
	Act     NamedAct             `json:"act"`
	Person  string               `json:"person"`
	Machine string               `json:"machine"`
	At      time.Time            `json:"at"`
	Source  string               `json:"source"`
	Command []string             `json:"command"`
	Proof   humanauthority.Proof `json:"proof"`
}

// CompleteNamedAct answers only questions held by this installation. The act
// has already succeeded; each answer uses the ordinary channel answer owner.
// A missed close is retried by repeating the act, never by reading questions.
func CompleteNamedAct(root, authorityRoot string, act NamedAct, person, machine string, at time.Time, proof humanauthority.Proof, resolve func(string) (goal.Endpoint, error)) []NamedActResult {
	if !act.valid() || person == "" || at.IsZero() {
		return nil
	}
	direct := proof.Helm == nil && proof.EnrolledTerminalFor(authorityRoot)
	reply := proof.ChannelWordFor(authorityRoot)
	if !direct && !reply {
		return nil
	}
	if resolve == nil {
		resolve = project.PublicationEndpoint
	}
	source := "terminal"
	if reply {
		source = "channel"
	}
	questions, _ := WalkQuestions(root)
	var results []NamedActResult
	var evidence *ActEvidence
	for _, prior := range questions {
		if namedActOption(prior, act, person, at) == "" {
			continue
		}
		result := NamedActResult{Question: prior.ID}
		result.Err = withPollLock(root, func() error {
			q, err := ReadQuestion(root, prior.ID)
			if err != nil {
				return err
			}
			option := namedActOption(q, act, person, at)
			if option == "" {
				return nil
			}
			if machine == "" {
				return fmt.Errorf("the acting machine could not be identified")
			}
			if q.Answer == nil {
				if evidence == nil {
					id, err := goal.NewOperationULID()
					if err != nil {
						return err
					}
					evidence = &ActEvidence{ID: id, Act: act, Person: person, Machine: machine, At: at.UTC(), Source: source, Command: act.Command(), Proof: proof}
				}
				id, err := goal.NewOperationULID()
				if err != nil {
					return err
				}
				q.Answer = &Answer{Machine: machine, Lineage: "question-act", Provider: source, Text: option, UserID: person, At: at.UTC(), Step: 1, ULID: id, Opid: goal.Opid(id, machine, "question-act"), Ref: MessageRef{ID: evidence.ID, ThreadID: q.ID}, Phase: "matched", Act: evidence}
				if err := writeQuestion(root, q); err != nil {
					return err
				}
			}
			if err := advanceAnswerWithEndpoint(context.Background(), PollConfig{RepoRoot: root, Now: at, Destination: "fleet"}, &q, resolve); err != nil {
				return err
			}
			result.Option = q.Answer.Text
			return nil
		})
		if result.Err != nil || result.Option != "" {
			results = append(results, result)
		}
	}
	return results
}

type NamedActResult struct {
	Question, Option string
	Err              error
}

func namedActOption(q Question, act NamedAct, person string, at time.Time) string {
	if q.State != "open" || !q.RetiredAt.IsZero() || act.Goal != q.Goal || at.Before(q.OpenedAt) || (q.ActPerson != "" && !strings.EqualFold(q.ActPerson, person)) {
		return ""
	}
	if q.Answer != nil && (q.Answer.Act == nil || q.Answer.Act.Act != act) {
		return ""
	}
	for _, o := range q.Options {
		if o.NamedAct != nil && *o.NamedAct == act {
			return o.Label
		}
	}
	return ""
}

func hasNamedActs(q Question) bool {
	for _, o := range q.Options {
		if o.NamedAct != nil {
			return true
		}
	}
	return false
}

func validateNamedActs(r AskRequest) error {
	seen := map[NamedAct]bool{}
	for _, o := range r.Options {
		if o.NamedAct == nil {
			continue
		}
		if r.Wants != "" {
			return fmt.Errorf("--wants cannot accompany --option-act; the completed command answers only its option")
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
