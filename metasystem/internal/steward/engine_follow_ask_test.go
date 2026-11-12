package steward

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
)

func TestFollowFailureAsksOnceAndRetires(t *testing.T) {
	t.Parallel()
	b := newPersonWaitBed(t, false)
	r := &FollowRefusal{Commit: strings.Repeat("a", 40), Command: "cd " + b.root + " && cp -p /fixture/artifact bin/.metasystem.hand && mv bin/.metasystem.hand bin/metasystem && bin/metasystem system restart"}
	pass := func() {
		t.Helper()
		if refused, err := TellFollow(b.root, "ui", r, b.now); err != nil || refused != (r != nil && !r.Waiting) {
			t.Fatalf("refused=%v err=%v", refused, err)
		}
		b.poll()
		b.now = b.now.Add(time.Minute)
	}
	for i := range 10 {
		r.Reason = fmt.Sprintf("artifact unreadable on pass %d", i)
		pass()
	}
	qs := b.questions()
	if len(qs) != 1 || qs[0].Decision == nil || *qs[0].Decision != (channel.Decision{Kind: "engine-follow", Subject: "ui", State: r.Commit}) || qs[0].Wants != r.Command || !qs[0].RetiredAt.IsZero() || len(b.channel.posts(t)) != 1 || qs[0].Plain.Decision != "Seat ui could not install the deployed engine aaaaaaaaaaaa: artifact unreadable on pass 0" {
		t.Fatalf("persistent failure questions=%+v posts=%q", qs, b.channel.posts(t))
	}
	first := qs[0]
	row, _ := json.Marshal(map[string]any{"thread_ts": first.Thread.ThreadID, "user": "UWIDO", "text": "I will check the artifact"})
	if err := os.WriteFile(filepath.Join(b.channel.dir, "replies.jsonl"), append(row, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	b.poll()
	if q := b.questions()[0]; q.Answer == nil || !q.RetiredAt.IsZero() {
		t.Fatalf("reply not retained: %+v", q)
	}
	for range 10 {
		pass()
	}
	if len(b.questions()) != 1 {
		t.Fatal("the answered refusal asked again")
	}
	r = nil
	pass()
	if q := b.questions()[0]; q.RetiredAt.IsZero() || q.State != "closed" {
		t.Fatalf("successful follow did not retire: %+v", q)
	}
	r = &FollowRefusal{Commit: strings.Repeat("b", 40), Reason: "unreadable", Command: first.Wants}
	pass()
	if len(b.questions()) != 2 {
		t.Fatal("new deployment did not ask anew")
	}
	second := b.questions()[0]
	r.Commit = strings.Repeat("c", 40)
	pass()
	q, err := channel.ReadQuestion(b.root, second.ID)
	if err != nil || q.RetiredAt.IsZero() || q.State != "closed" || len(b.questions()) != 3 {
		t.Fatalf("superseded refusal=%+v err=%v", q, err)
	}
}

func TestFollowWaitingLeavesQuestionsAlone(t *testing.T) {
	t.Parallel()
	b := newPersonWaitBed(t, false)
	r := &FollowRefusal{Commit: "old", Reason: "unreadable", Command: "inspect the artifact"}
	if _, err := TellFollow(b.root, "ui", r, b.now); err != nil {
		t.Fatal(err)
	}
	r.Commit, r.Waiting = "new", true
	if refused, err := TellFollow(b.root, "ui", r, b.now); refused || err != nil {
		t.Fatalf("waiting refusal=%v err=%v", refused, err)
	}
	if qs := b.questions(); len(qs) != 1 || !qs[0].RetiredAt.IsZero() {
		t.Fatalf("waiting changed questions: %+v", qs)
	}
	writeStewardRecord(t, filepath.Join(b.root, "artifacts", "agents", "channel", "questions", "broken.json"), map[string]any{"invalid": true})
	if refused, err := TellFollow(b.root, "ui", r, b.now); refused || err != nil {
		t.Fatalf("waiting read the question store: %v %v", refused, err)
	}
	if refused, err := TellFollow(b.root, "ui", nil, b.now); refused || err == nil {
		t.Fatalf("unreadable question store accepted: %v %v", refused, err)
	}
}

func TestFollowUnavailableAsksOnce(t *testing.T) {
	t.Parallel()
	for _, reason := range []string{"no active line", "unreadable record", "no contract"} {
		t.Run(reason, func(t *testing.T) {
			t.Parallel()
			b := newPersonWaitBed(t, false)
			r := &FollowRefusal{Commit: FollowDeploymentUnavailable, Reason: reason, Command: "cd " + b.root + " && bin/metasystem deploy status"}
			for range 10 {
				if refused, err := TellFollow(b.root, "ui", r, b.now); !refused || err != nil {
					t.Fatalf("unavailable refusal=%v err=%v", refused, err)
				}
				b.poll()
				b.now = b.now.Add(time.Minute)
			}
			qs := b.questions()
			if len(qs) != 1 || qs[0].Decision == nil || qs[0].Decision.State != FollowDeploymentUnavailable || qs[0].Wants != r.Command || len(b.channel.posts(t)) != 1 || qs[0].Plain.Decision != "Seat ui cannot read this project's deployed engine: "+reason {
				t.Fatalf("unavailable questions=%+v posts=%q", qs, b.channel.posts(t))
			}
		})
	}
}

func TestRunnerDeliversFollowFailureAtHelmAndNormally(t *testing.T) {
	t.Parallel()
	for _, held := range []bool{false, true} {
		t.Run(fmt.Sprint(held), func(t *testing.T) {
			t.Parallel()
			b := newPersonWaitBed(t, false)
			if held {
				takeHelmFixture(t, b.root)
			}
			deps := idleRunnerDependencies(&b.now)
			passes, duties := 0, 0
			deps.Tick = func(string, TickConfig, WorkerCensus) (TickResult, error) {
				passes++
				return TickResult{}, nil
			}
			deps.Channel = func(context.Context, string) (int, error) { duties++; b.poll(); return 0, nil }
			deps.SettleReplies = func(context.Context, string) (int, error) { return 0, nil }
			sleep := deps.Sleep
			deps.Sleep = func(d time.Duration) {
				sleep(d)
				if passes == 10 {
					if err := stopRunnerLoop(b.root); err != nil {
						t.Fatal(err)
					}
				}
			}
			r := &FollowRefusal{Commit: "deployed", Reason: "artifact unreadable", Command: "inspect the artifact"}
			cfg := TickConfig{Prober: seatBusyDeadProber{}, FollowDeployedEngine: func() (bool, bool, error) {
				refused, err := TellFollow(b.root, "ui", r, b.now)
				return false, refused, err
			}}
			if err := runLoopWithDependencies(b.root, fakeCensus{}, nil, time.Second, cfg, deps); err != nil {
				t.Fatal(err)
			}
			if passes != 10 || duties != 10 || len(b.questions()) != 1 || len(b.channel.posts(t)) != 1 {
				t.Fatalf("held=%v passes=%d duties=%d questions=%+v posts=%q", held, passes, duties, b.questions(), b.channel.posts(t))
			}
		})
	}
}
