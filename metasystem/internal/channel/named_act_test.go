package channel

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/governance"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testgoal"
)

type namedActBed struct {
	root  string
	ep    goal.Endpoint
	now   time.Time
	proof humanauthority.Proof
	act   NamedAct
}

func newNamedActBed(t *testing.T) *namedActBed {
	t.Helper()
	repo, _, old, now := pollLedgerBed(t)
	if err := os.Remove(questionPath(repo.root, old.ID)); err != nil {
		t.Fatal(err)
	}
	ep := repo.endpoint
	ep.Repository = testgoal.New(copyPollFiles(repo.commits[repo.canonical].files), now, strings.Repeat("1", 40))
	proof, err := humanauthority.VerifiedChannelAnswerProof(repo.root, governance.RecordedChannelAuthority{Outcome: governance.AuthorityOutcomeVerifiedChannelAnswer, Provider: "fake", UserID: "Wido", MessageRef: "other/answer", ContextID: "other", Step: 42}, now)
	if err != nil {
		t.Fatal(err)
	}
	return &namedActBed{repo.root, ep, now, proof, NamedAct{Verb: "goal accept-risk", Goal: "g", Target: "review", Finding: "F"}}
}
func (b *namedActBed) resolve(string) (goal.Endpoint, error) { return b.ep, nil }
func (b *namedActBed) ask(t *testing.T, facts string, amend func(*AskRequest)) Question {
	t.Helper()
	r := AskRequest{RepoRoot: b.root, Goal: "g", Kind: "other", Machine: "asking", Now: b.now, Facts: []string{facts}, Options: []Option{{Label: "Accept", Consequence: "keep the risk", NamedAct: &b.act}, {Label: "Wait", Consequence: "wait"}}}
	if amend != nil {
		amend(&r)
	}
	q, err := Ask(r)
	if err != nil {
		t.Fatal(err)
	}
	q.Thread = &MessageRef{ID: q.ID, ThreadID: q.ID}
	if err := writeJSON(questionPath(b.root, q.ID), q); err != nil {
		t.Fatal(err)
	}
	return q
}
func (b *namedActBed) complete(t *testing.T, act NamedAct, at time.Time) []NamedActResult {
	t.Helper()
	results := CompleteNamedAct(b.root, b.root, act, "Wido", "acting", at, b.proof, b.resolve)
	for _, result := range results {
		if result.Err != nil {
			t.Fatal(result.Err)
		}
	}
	return results
}
func (b *namedActBed) read(t *testing.T, q Question) Question {
	t.Helper()
	q, err := ReadQuestion(b.root, q.ID)
	if err != nil {
		t.Fatal(err)
	}
	return q
}

func TestQ1CompletedActAnswersLocalQuestions(t *testing.T) {
	t.Parallel()
	b := newNamedActBed(t)
	q := b.ask(t, "Accept?", nil)
	other := b.ask(t, "Another seat asks", nil)
	b.complete(t, b.act, b.now.Add(time.Second))
	a := b.read(t, q).Answer.Act
	for _, q := range []Question{q, other} {
		got := b.read(t, q)
		if got.State != "closed" || got.Answer == nil || got.Answer.Text != "Accept" || got.Answer.Act.ID != a.ID || !got.Answer.At.Equal(a.At) {
			t.Fatalf("answer %+v", got)
		}
	}
	b.complete(t, b.act, b.now.Add(2*time.Second))
	tip, _, err := b.ep.Repository.Accepted()
	if err != nil {
		t.Fatal(err)
	}
	files, err := b.ep.Repository.Files(tip, "plans/goals/")
	if err != nil {
		t.Fatal(err)
	}
	f, problems := goal.ParseFile(files["plans/goals/g.md"])
	if len(problems) > 0 {
		t.Fatal(problems)
	}
	answers := 0
	for _, h := range f.History {
		if h.Verb == "answer" {
			answers++
		}
	}
	if answers != 2 {
		t.Fatalf("answer events %d, want one per question", answers)
	}
	if got := GoalOpenQuestions(b.root); len(got) != 0 {
		t.Fatalf("steward still held: %+v", got)
	}
	provider := &testProvider{}
	c := PollConfig{RepoRoot: b.root, Destination: "fleet", ProviderName: "fake", HumanUserID: "Wido", TOTPSecret: "JBSWY3DPEHPK3PXP", Machine: "asking", Lineage: "seat", Provider: provider, Now: b.now.Add(time.Minute)}
	// Drive the actual poll's outbox twice; retries must not post again.
	for range 2 {
		if _, err := pollWithEndpoint(context.Background(), c, b.resolve); err != nil {
			t.Fatal(err)
		}
	}
	if len(provider.posts) != 2 {
		t.Fatalf("posts %v", provider.posts)
	}
	for _, line := range provider.posts {
		if !strings.Contains(line, "Answered at the channel: Accept") || !strings.Contains(line, "Wido") {
			t.Fatalf("notice %s", line)
		}
	}
	code, err := TOTPCode(c.TOTPSecret, c.Now)
	if err != nil {
		t.Fatal(err)
	}
	provider.inbound = []Inbound{{Ref: MessageRef{ID: "late-reply", ThreadID: q.ID}, ThreadID: q.ID, UserID: "Wido", Text: "Wait " + code, SentAt: c.Now, Ack: "late"}}
	for range 2 {
		if _, err := pollWithEndpoint(context.Background(), c, b.resolve); err != nil {
			t.Fatal(err)
		}
	}
	if len(provider.posts) != 3 || !strings.Contains(provider.posts[2], "Already answered") || b.read(t, q).Answer.Text != "Accept" {
		t.Fatalf("late reply changed answer or was not refused: %v", provider.posts)
	}

	withdrawn, err := Withdraw(b.root, q.ID, "late withdrawal", provider, DestinationConfig{}, b.now.Add(time.Minute))
	if err != nil || withdrawn.Withdrawal != nil || withdrawn.Answer.Text != "Accept" {
		t.Fatalf("withdraw %+v %v", withdrawn, err)
	}
}

func TestQ1ExactMatchAndOldQuestions(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"goal", "finding", "review", "run", "verb", "before", "same clock before", "no declaration", "withdrawn", "retired", "answered", "exact", "stop exact"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b := newNamedActBed(t)
			act := b.act
			at := b.now.Add(time.Second)
			if name == "same clock before" {
				b.complete(t, act, b.now)
			}
			q := b.ask(t, "Choose", func(r *AskRequest) {
				if name == "no declaration" {
					r.Options[0].NamedAct = nil
					r.Options[0].Consequence = "metasystem goal accept-risk g --review review --finding F"
				}
				if name == "run" || name == "stop exact" {
					r.Options[0].NamedAct = &NamedAct{Verb: "work stop", Goal: "g", Target: "run-x"}
				}
			})
			switch name {
			case "goal":
				act.Goal = "other"
			case "finding":
				act.Finding = "other"
			case "review":
				act.Target = "other"
			case "run":
				act = NamedAct{Verb: "work stop", Goal: "g", Target: "run-y"}
			case "stop exact":
				act = NamedAct{Verb: "work stop", Goal: "g", Target: "run-x"}
			case "verb":
				act = NamedAct{Verb: "work stop", Goal: "g", Target: "review"}
			case "before":
				at = b.now.Add(-time.Second)
			case "withdrawn":
				if _, err := Withdraw(b.root, q.ID, "done", nil, DestinationConfig{}, b.now); err != nil {
					t.Fatal(err)
				}
			case "retired":
				q.RetiredAt = b.now
				if err := writeJSON(questionPath(b.root, q.ID), q); err != nil {
					t.Fatal(err)
				}
			case "answered":
				q.State = "closed"
				q.Answer = &Answer{Text: "Wait", Phase: "closed"}
				if err := writeJSON(questionPath(b.root, q.ID), q); err != nil {
					t.Fatal(err)
				}
			}
			if name != "same clock before" {
				b.complete(t, act, at)
			}
			got := b.read(t, q)
			want := name == "exact" || name == "stop exact"
			if (got.Answer != nil && got.Answer.Text == "Accept") != want {
				t.Fatalf("unexpected answer %+v", got)
			}
		})
	}
}

func TestQ1InterruptedAnswerNeedsRepeatedAct(t *testing.T) {
	t.Parallel()
	b := newNamedActBed(t)
	q := b.ask(t, "Choose", nil)
	failed := func(string) (goal.Endpoint, error) { return goal.Endpoint{}, errors.New("unavailable") }
	results := CompleteNamedAct(b.root, b.root, b.act, "Wido", "acting", b.now.Add(time.Second), b.proof, failed)
	if len(results) != 1 || results[0].Err == nil {
		t.Fatal("publication failure hidden")
	}
	before, err := os.ReadFile(questionPath(b.root, q.ID))
	if err != nil {
		t.Fatal(err)
	}
	// Reads and ordinary reply settlement do not repair the missed close.
	for range 2 {
		WalkOpenQuestions(b.root)
		if _, err := SettleReplies(context.Background(), PollConfig{RepoRoot: b.root, Now: b.now}, b.resolve); err != nil {
			t.Fatal(err)
		}
	}
	after, err := os.ReadFile(questionPath(b.root, q.ID))
	if err != nil || string(before) != string(after) {
		t.Fatal("a read repaired the act")
	}
	fresh := b.ask(t, "Choose again", nil)
	b.complete(t, b.act, b.now.Add(2*time.Second))
	for _, question := range []Question{q, fresh} {
		if got := b.read(t, question); got.State != "closed" || got.Answer.Phase != "closed" {
			t.Fatalf("repeat did not close %+v", got)
		}
	}
}

func TestQ1RelayedAndUnprovenActsDoNotAnswer(t *testing.T) {
	t.Parallel()
	b := newNamedActBed(t)
	q := b.ask(t, "Choose", nil)
	for _, proof := range []humanauthority.Proof{{}, {Outcome: humanauthority.OutcomeProven}, {TemporaryHumanWord: "yes", Departure: "R-32-m1"}} {
		results := CompleteNamedAct(b.root, b.root, b.act, "Wido", "acting", b.now, proof, b.resolve)
		if len(results) != 0 {
			t.Fatal("unproved act answered")
		}
	}
	if got := b.read(t, q); got.Answer != nil {
		t.Fatalf("unproved answer %+v", got)
	}
}

func TestQ2NamedActRefusesWantedAuthority(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	_, err := Ask(AskRequest{RepoRoot: root, Goal: "g", Kind: "other", Facts: []string{"Accept?"}, Wants: "approval token", Options: []Option{{Label: "Accept", Consequence: "keep the risk", NamedAct: &NamedAct{Verb: "goal accept-risk", Goal: "g", Target: "review", Finding: "F"}}}})
	if err == nil || !strings.Contains(err.Error(), "--wants") {
		t.Fatalf("mixed authority: %v", err)
	}
	questions, bad := WalkQuestions(root)
	if len(questions) != 0 || len(bad) != 0 {
		t.Fatalf("refusal wrote questions: %d unreadable: %d", len(questions), len(bad))
	}
}
