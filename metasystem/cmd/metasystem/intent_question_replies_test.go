package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testchannel"
)

func TestIntentQuestionSettlesReplies(t *testing.T) {
	t.Parallel()
	for _, verb := range []string{"list", "show", "answer"} {
		t.Run(verb, func(t *testing.T) {
			t.Parallel()
			for _, phase := range []string{"fresh", "matched", "recorded-commit", "recorded", "closed"} {
				t.Run(phase, func(t *testing.T) {
					t.Parallel()
					b := newIntentBed(t, false, nil)
					replies := testchannel.New(t, b.root())
					replies.CommitReply(t, replies.Question)
					if phase != "fresh" {
						replies.Interrupt(t, phase)
					}
					owners := b.owners()
					owners.processes = defaultProcessIntentOwners()
					owners.dependencies.endpoint = replies.Resolve
					// Use the production identity path with no supplied owner lineage.
					owners.dependencies.ownerLineage = nil
					args := []string{"question", verb}
					if verb != "list" {
						args = append(args, replies.Question.ID)
					}
					for i := 0; i < 2; i++ {
						code, stdout, stderr := b.run(owners, args...)
						if code != 0 || !strings.Contains(strings.Join(strings.Fields(stdout), " "), "answered 'yes' at 09:13") || strings.Contains(stdout, "withdrawn") {
							t.Fatalf("%v exit %d: %s %s", args, code, stdout, stderr)
						}
					}
					replies.AssertAnswer(t)
				})
			}
		})
	}
}

func TestIntentQuestionActionReplyStaysOutstanding(t *testing.T) {
	t.Parallel()
	b := newIntentBed(t, false, nil)
	replies := testchannel.New(t, b.root())
	// One synthetic saved stop, reconciled by the real landing request owner.
	stopPath := filepath.Join(plain.Dir(b.root()), "stops.jsonl")
	testchannel.Save(t, stopPath, []byte(`{"loop":"lane-return","subject":"lane","attempt":2,"decision":"stop","handoff":"ask lane","at":"2026-10-15T08:00:00Z","tree":"candidate","proof-attempt":"attempt"}`+"\n"))
	if err := plain.SyncPolicyQuestion(b.root(), func(string) (string, error) { return "asking", nil }, replies.Sent); err != nil {
		t.Fatal(err)
	}
	questions, unreadable := channel.WalkOpenQuestions(b.root())
	var request channel.Question
	for _, q := range questions {
		if channel.LaneStopCommand(q) != "" {
			request = q
		}
	}
	if request.ID == "" || len(unreadable) > 0 {
		t.Fatalf("landing request: %v %v", questions, unreadable)
	}
	request.Thread = &channel.MessageRef{ID: "10", ThreadID: "10"}
	replies.SaveQuestion(t, request)
	before := map[string]string{}
	_ = filepath.WalkDir(plain.Dir(b.root()), func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			t.Fatal(err)
		}
		if !entry.IsDir() {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			before[path] = string(data)
		}
		return nil
	})
	replies.CommitReply(t, request)
	owners := b.owners()
	owners.processes = defaultProcessIntentOwners()
	owners.dependencies.endpoint = replies.Resolve
	for _, verb := range []string{"list", "show"} {
		args := []string{"question", verb}
		if verb == "show" {
			args = append(args, request.ID)
		}
		code, out, errout := b.run(owners, args...)
		want := "replied 'yes' at 09:13; the act is still needed: " + request.Facts[0]
		if code != 0 || !strings.Contains(strings.Join(strings.Fields(out), " "), want) || strings.Contains(out, "lane stop:") || strings.Contains(out, "evidence:") {
			t.Fatalf("%v exit %d: %s %s; want %q", args, code, out, errout, want)
		}
	}
	got, err := channel.ReadQuestion(b.root(), request.ID)
	if err != nil || got.State != "open" || got.Answer == nil || got.Answer.Phase != "recorded" || got.Answer.Text != "yes" || got.Answer.Ref.ID != "2-10" || got.Answer.Ref.ThreadID != "10" {
		t.Fatalf("request: %+v %v", got, err)
	}
	after := map[string]string{}
	_ = filepath.WalkDir(plain.Dir(b.root()), func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			t.Fatal(err)
		}
		if !entry.IsDir() {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			after[path] = string(data)
		}
		return nil
	})
	if len(before) != len(after) {
		t.Fatalf("landing records changed: %v %v", before, after)
	}
	for path, data := range before {
		if after[path] != data {
			t.Fatalf("landing record changed: %s", path)
		}
	}
	// Five complete replies must not exhaust the allowance for a fresh reply.
	for i := 0; i < 4; i++ {
		copy := got
		copy.ID = string(rune('a'+i)) + "-action"
		replies.SaveQuestion(t, copy)
	}
	// Replace the shared message with a new committed reply to the goal question.
	replies.CommitReply(t, replies.Question)
	for i := 0; i < 3; i++ {
		result, err := channel.SettleReplies(context.Background(), replies.Config(), replies.Resolve)
		want := 0
		if i == 0 {
			want = 1
		}
		if err != nil || result.Dispositions != want {
			t.Fatalf("settlement %d: %+v %v", i, result, err)
		}
	}
	replies.AssertAnswer(t)
}

func TestIntentQuestionWithdrawnStillRendersWithdrawn(t *testing.T) {
	t.Parallel()
	b := newIntentBed(t, false, nil)
	replies := testchannel.New(t, b.root())
	q := replies.Question
	q.State = "closed"
	replies.SaveQuestion(t, q)
	owners := b.owners()
	owners.processes = defaultProcessIntentOwners()
	owners.dependencies.endpoint = replies.Resolve
	code, out, errout := b.run(owners, "question", "show", q.ID)
	if code != 0 || !strings.Contains(out, "withdrawn") || strings.Contains(out, "answered") {
		t.Fatalf("exit %d: %s %s", code, out, errout)
	}
}

func TestIntentQuestionSettlementUsesInstallation(t *testing.T) {
	t.Parallel()
	b := newIntentBed(t, false, nil)
	replies := testchannel.New(t, t.TempDir())
	replies.CommitReply(t, replies.Question)
	owners := b.owners()
	owners.processes = defaultProcessIntentOwners()
	owners.dependencies.endpoint = replies.Resolve
	inv := &intentInvocation{stateRoot: b.root(), owners: owners, layout: stateroot.Layout{InstallationRoot: stateroot.Installation(replies.Root)}}
	if problem := inv.settleQuestionReplies(); problem != nil {
		t.Fatalf("settlement: %+v", problem)
	}
	q, problem := inv.resolveQuestion(replies.Question.ID)
	if problem != nil {
		t.Fatalf("question read outside installation: %+v", problem)
	}
	if result := inv.questionView(q); !strings.Contains(result.Summary, "answered 'yes' at 09:13") {
		t.Fatal(result.Summary)
	}
	replies.AssertAnswer(t)
	if _, err := channel.ReadQuestion(b.root(), q.id); !os.IsNotExist(err) {
		t.Fatalf("question appeared in project state: %v", err)
	}
}
