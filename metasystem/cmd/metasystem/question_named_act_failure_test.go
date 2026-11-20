package main

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testchannel"
)

type questionActReadFailure struct{ goal.Repository }

func (r questionActReadFailure) Files(tip string, prefixes ...string) (map[string][]byte, error) {
	if len(prefixes) == 1 && prefixes[0] == "records/channel-acts/" {
		return nil, errors.New("shared act records unavailable")
	}
	return r.Repository.Files(tip, prefixes...)
}

func TestQ1QuestionReadsKeepSettlingRepliesAfterMatchingFailure(t *testing.T) {
	t.Parallel()
	for _, verb := range []string{"show", "wait"} {
		t.Run(verb, func(t *testing.T) {
			t.Parallel()
			b := newIntentBed(t, false, nil)
			replies := testchannel.New(t, b.root())
			replies.CommitReply(t, replies.Question)
			q := replies.Question
			q.ID = "pending-act"
			q.Thread = &channel.MessageRef{ID: q.ID, ThreadID: q.ID}
			q.Options = []channel.Option{{Label: "Accept", NamedAct: &channel.NamedAct{Verb: "goal accept-risk", Goal: q.Goal, Target: "review", Finding: "F"}}}
			replies.SaveQuestion(t, q)
			owners := b.owners()
			owners.processes = defaultProcessIntentOwners()
			owners.dependencies.ownerLineage = nil
			owners.dependencies.endpoint = func(root string) (goal.Endpoint, error) {
				ep, err := replies.Resolve(root)
				ep.Repository = questionActReadFailure{ep.Repository}
				return ep, err
			}
			code, out, errout := b.run(owners, "question", verb, replies.Question.ID)
			if code != 0 {
				t.Fatalf("ordinary %s: %d %s %s", verb, code, out, errout)
			}
			replies.AssertAnswer(t)
			code, out, errout = b.run(owners, "question", "show", q.ID)
			if code != 0 || !strings.Contains(out, "could not be checked now: shared") && !strings.Contains(out, "could not be checked now: the local goal ledger") {
				t.Fatalf("diagnostic: %d %s %s", code, out, errout)
			}
			got, err := channel.ReadQuestion(b.root(), q.ID)
			if err != nil || got.State != "open" || got.Answer != nil {
				t.Fatalf("failed match changed question: %+v %v", got, err)
			}
		})
	}
}

func TestQ1DisplayedActIncludesRequiredReason(t *testing.T) {
	t.Parallel()
	b, owners := newQ1CommandBed(t)
	q := q1Ask(t, b, owners, "Accept", "Accept with the displayed command?")
	command := q.Options[0].NamedAct.DisplayCommand()
	if !strings.Contains(q.Text, command) || !strings.Contains(command, "--reason TEXT") {
		t.Fatal("displayed command omits required reason")
	}
	code, out, errout := b.run(owners, shellWords(command)[1:]...)
	if code != 0 {
		t.Fatalf("displayed command refused: %d %s %s", code, out, errout)
	}
	got, err := channel.ReadQuestion(b.root(), q.ID)
	if err != nil || got.State != "closed" || got.Answer == nil {
		t.Fatalf("displayed command did not answer: %+v %v", got, err)
	}
	if strings.Contains(strings.Join(got.Answer.Act.Command, " "), "--reason") {
		t.Fatal("reason entered the identity")
	}
	quoted := channel.NamedAct{Verb: "goal accept-risk", Goal: bedGoal, Target: "review one", Finding: "finding with spaces"}
	words := shellWords(quoted.DisplayCommand())
	if words[5] != quoted.Target || words[7] != quoted.Finding {
		t.Fatal("displayed arguments lost their boundaries")
	}
}

func TestQ1MalformedActStillAllowsQuestionShowToSettleReplies(t *testing.T) {
	t.Parallel()
	b := newIntentBed(t, false, nil)
	replies := testchannel.New(t, b.root())
	replies.CommitReply(t, replies.Question)
	q := replies.Question
	q.ID = "incomplete-act"
	q.Thread = &channel.MessageRef{ID: q.ID, ThreadID: q.ID}
	q.Options = []channel.Option{{Label: "Accept", NamedAct: &channel.NamedAct{Verb: "goal accept-risk", Goal: q.Goal, Target: "review", Finding: "F"}}}
	replies.SaveQuestion(t, q)
	testchannel.Save(t, filepath.Join(b.root(), "artifacts", "agents", "channel", "named-acts", "bad.json"), []byte(`{"id":"incomplete"}`))
	owners := b.owners()
	owners.processes = defaultProcessIntentOwners()
	owners.dependencies.endpoint = replies.Resolve
	code, out, errout := b.run(owners, "question", "show", q.ID)
	if code != 0 || !strings.Contains(out, "skipped incomplete or unsupported act record") {
		t.Fatalf("show: %d %s %s", code, out, errout)
	}
	replies.AssertAnswer(t)
}
