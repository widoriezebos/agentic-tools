package main

import (
	"errors"
	"os"
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
			if code != 0 || strings.Contains(out, "could not be checked") {
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
	if code != 0 || strings.Contains(out, "skipped incomplete") {
		t.Fatalf("show: %d %s %s", code, out, errout)
	}
	replies.AssertAnswer(t)
}

// Fail only the ordinary answer publication, after acceptance has succeeded.
type questionAnswerCaptureFailure struct{ goal.Repository }

func (r questionAnswerCaptureFailure) Capture(opid string) (string, error) {
	suffix := goal.Opid("", "", "question-act")
	if strings.HasSuffix(opid, suffix[1:]) {
		return "", errors.New("unavailable")
	}
	return r.Repository.Capture(opid)
}

func TestQ1QuestionCloseFailurePreservesActResult(t *testing.T) {
	t.Parallel()
	b, owners := newQ1CommandBed(t)
	q := q1Ask(t, b, owners, "Accept", "Choose?")
	endpoint := owners.dependencies.endpoint
	owners.dependencies.endpoint = func(root string) (goal.Endpoint, error) {
		ep, err := endpoint(root)
		ep.Repository = questionAnswerCaptureFailure{ep.Repository}
		return ep, err
	}
	code, result := b.runJSON(owners, "goal", "accept-risk", bedGoal, "--finding", "S-1", "--review", "critic-r2", "--reason", "bounded exposure")
	if code != 0 || result.Outcome != intentConfirmed || !strings.Contains(strings.Join(result.Details, " "), "Question "+q.ID+" stays open") {
		t.Fatalf("close failure changed act: %d %+v", code, result)
	}
	got, err := channel.ReadQuestion(b.root(), q.ID)
	if err != nil || got.State != "open" {
		t.Fatalf("question %+v %v", got, err)
	}
	owners.dependencies.endpoint = endpoint
	if code, _, _ := q1Accept(b, owners); code != 0 {
		t.Fatal("repeat failed")
	}
	got, _ = channel.ReadQuestion(b.root(), q.ID)
	if got.State != "closed" {
		t.Fatal("repeat did not close")
	}
}

func TestQ1QuestionCompletionPreservesExistingActFailure(t *testing.T) {
	t.Parallel()
	b, owners := newQ1CommandBed(t)
	q := q1Ask(t, b, owners, "Accept", "Choose?")
	q.Options[0].NamedAct.Finding = "another"
	testchannel.Save(t, filepath.Join(b.root(), "artifacts", "agents", "channel", "questions", "bad.json"), []byte("{"))
	// Keep an unrelated unanswered declaration beside the damaged file.
	bed := testchannel.New(t, b.root())
	bed.SaveQuestion(t, q)
	code, result := b.runJSON(owners, "goal", "accept-risk", bedGoal, "--finding", "S-1", "--review", "critic-r2", "--reason", "bounded exposure")
	// The existing unit-stop recorder already reports damaged questions.
	// Named completion must preserve that result and leave the question alone.
	if code != 1 || result.Outcome != intentPartial || !strings.Contains(strings.Join(result.Details, " "), "risk was recorded, but its questions could not be read") {
		t.Fatalf("existing result changed %d %+v", code, result)
	}
	got, err := channel.ReadQuestion(b.root(), q.ID)
	if err != nil || got.Answer != nil {
		t.Fatal("a partially completed act answered")
	}
}

func TestQ1AcceptRiskIdempotencyAtEnrolledTerminal(t *testing.T) {
	t.Parallel()
	b, owners := newQ1CommandBed(t)
	now, _ := b.commandNow(b.root())
	proof, err := owners.prove(b.root(), 0, nil, "", "", now)
	if err != nil || !proof.EnrolledTerminalFor(b.root()) {
		t.Fatal("witness does not reach the enrolled-terminal path")
	}
	if code, out, errout := q1Accept(b, owners); code != 0 {
		t.Fatalf("first %d %s %s", code, out, errout)
	}
	publications, files := b.publications(), goalLedgerBytes(b)
	journal := strings.Join(goalJournalNames(t, b.root()), ",")
	if code, result := b.runJSON(owners, "goal", "accept-risk", bedGoal, "--finding", "S-1", "--review", "critic-r2", "--reason", "bounded exposure"); code != 0 || result.Outcome != intentUnchanged {
		t.Fatalf("repeat %d %+v", code, result)
	}
	if b.publications() != publications || goalLedgerBytes(b) != files || strings.Join(goalJournalNames(t, b.root()), ",") != journal {
		t.Fatal("repeat changed the ledger or journal")
	}
	if _, err := os.Stat(filepath.Join(b.root(), "artifacts", "agents", "channel", "named-acts")); !os.IsNotExist(err) {
		t.Fatal("completion store created without a question")
	}
}
