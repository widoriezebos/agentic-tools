// Package testchannel supplies private reply transactions for channel entrypoint tests.
package testchannel

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testgoal"
)

type Replies struct {
	Root     string
	Endpoint goal.Endpoint
	Question channel.Question
	Sent     time.Time
}

func New(t *testing.T, root string) *Replies {
	t.Helper()
	at := time.Date(2026, 10, 15, 9, 13, 0, 0, time.Local).UTC()
	f := &goal.GoalFile{Id: "reply-goal", State: goal.StateQueued, Tier: 1, Intent: "Ask a question.", Origin: "main", NextStep: "Wait.", OpenedAt: at.Format(time.RFC3339), Revision: 1,
		History: []goal.HistoryLine{{At: at.Format(time.RFC3339), Opid: goal.Opid("01J5X0000000000000000000F0", "asking", "session"), Verb: "open", Actor: "asking+session", Targets: []string{"reply-goal"}, Keep: -1}}}
	files := map[string][]byte{"plans/goals/backlog.md": goal.RenderRoot(&goal.RootRecord{Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "1", SyncMode: goal.SyncLocal, Revision: 1}), "plans/goals/reply-goal.md": goal.RenderFile(f)}
	r := &Replies{Root: root, Sent: at, Endpoint: goal.Endpoint{Root: root, Remote: "local", Branch: goal.LocalLedgerBranch, Repository: testgoal.New(files, at, strings.Repeat("1", 40))},
		Question: channel.Question{ID: "01J5X0000000000000000000Q0", Goal: f.Id, Kind: "other", Machine: "asking", Lineage: "asking-session", State: "open", OpenedAt: at.Add(-time.Hour), Facts: []string{"Proceed?"}, Thread: &channel.MessageRef{ID: "1", ThreadID: "1"}}}
	Save(t, filepath.Join(root, "metasystem.conf"), []byte("channel.destination.fleet.adapter=fake\nmetasystem.budget.tier-1=1h/3/360m/1/0\nmetasystem.budget.review-round-max=3\n"))
	r.SaveQuestion(t, r.Question)
	return r
}

func Save(t *testing.T, path string, value any) {
	t.Helper()
	data, ok := value.([]byte)
	if !ok {
		var err error
		data, err = json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}
func (r *Replies) SaveQuestion(t *testing.T, q channel.Question) {
	t.Helper()
	Save(t, filepath.Join(r.Root, "artifacts/agents/channel/questions", q.ID+".json"), q)
}
func (r *Replies) Resolve(string) (goal.Endpoint, error) { return r.Endpoint, nil }
func (r *Replies) Config() channel.PollConfig {
	return channel.PollConfig{RepoRoot: r.Root, Destination: "fleet", ProviderName: "fake", Machine: "different-caller", Lineage: "retry-session", Now: r.Sent.Add(time.Hour)}
}
func (r *Replies) CommitReply(t *testing.T, q channel.Question) {
	t.Helper()
	ulid, err := goal.NewOperationULID()
	if err != nil {
		t.Fatal(err)
	}
	opid := goal.Opid(ulid, "receiving", "other-session")
	step := int64(42)
	message := "2"
	if q.Thread.ID != "1" {
		message += "-" + q.Thread.ID
		step++
	}
	in := &goal.ChannelInbound{Provider: "fake", Destination: "fleet", MessageID: message, ReplyTo: &q.Thread.ID, UserID: "person", SentAt: r.Sent.Format(time.RFC3339), Text: "yes", Step: &step, Outcome: "verified", Question: "unmatched", Opid: opid, ReceivedBy: "receiving", ReceivedAt: r.Sent.Add(time.Minute).Format(time.RFC3339)}
	data, err := goal.RenderChannelInbound(in)
	if err != nil {
		t.Fatal(err)
	}
	result, err := goal.Publish(r.Endpoint, goal.PublishRequest{Opid: opid, Machine: "receiving", Lineage: "other-session", Intent: goal.Intent{Verb: "inbox"}, Message: "channel inbox fake-2", Mutate: func(string) ([]goal.Change, error) {
		return []goal.Change{{Path: goal.ChannelInboxPath("fleet", in), Content: data}}, nil
	}})
	if err != nil || result.Outcome != goal.OutcomeConfirmed {
		t.Fatalf("commit reply: %+v %v", result, err)
	}
}
func (r *Replies) Interrupt(t *testing.T, phase string) {
	t.Helper()
	c := r.Config()
	c.Machine, c.Lineage = "original-caller", "original-session"
	c.FailurePoint = func(p string) error {
		if p == phase {
			return errors.New("interrupted at " + phase)
		}
		return nil
	}
	if _, err := channel.SettleReplies(context.Background(), c, r.Resolve); err == nil {
		t.Fatal("interruption did not fire")
	}
}
func (r *Replies) AssertAnswer(t *testing.T) {
	t.Helper()
	q, err := channel.ReadQuestion(r.Root, r.Question.ID)
	if err != nil || q.State != "closed" || q.Answer == nil || q.Answer.Phase != "closed" || q.Answer.Text != "yes" || q.Answer.Ref.ID != "2" || !q.Answer.At.Equal(r.Sent) {
		t.Fatalf("completed answer: %+v %v", q, err)
	}
	tip, err := r.Endpoint.Repository.Capture("check-answer")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Endpoint.Repository.Release("check-answer")
	files, err := r.Endpoint.Repository.Files(tip, "plans/goals/")
	if err != nil {
		t.Fatal(err)
	}
	f, problems := goal.ParseFile(files["plans/goals/reply-goal.md"])
	if len(problems) > 0 {
		t.Fatal(problems)
	}
	answers := 0
	for _, h := range f.History {
		if h.Verb == "answer" {
			answers++
			if h.Opid != q.Answer.Opid {
				t.Fatalf("publication changed operation identity: %+v %+v", h, q.Answer)
			}
		}
	}
	if answers != 1 || strings.Count(f.NextStep, "ANSWERED "+q.ID) != 1 {
		t.Fatalf("answer history=%d next=%q", answers, f.NextStep)
	}
	if got := channel.AnswerSummary(q); got != "answered 'yes' at 09:13" {
		t.Fatal(got)
	}
}
