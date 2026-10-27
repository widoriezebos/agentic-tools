package channel

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

func contractRequest(root string) AskRequest {
	return AskRequest{RepoRoot: root, Goal: "g", Kind: "other", Machine: "machine", Facts: []string{"the build stopped"}, Now: time.Date(2026, 9, 3, 1, 0, 0, 0, time.UTC), Contract: true,
		Plain: &Plain{Decision: "g: the build of unit u stopped at 01:00 (a build step failed)", IfNothing: "it waits; nothing builds or lands until you choose"},
		Options: []Option{
			{Label: "Stop here", Consequence: "the work stays in its worktree; nothing builds, lands or concludes. This question stays open."},
			{Label: "Continue", Consequence: "the builder corrects and finishes the unit from the prepared brief; one more round of the goal's budget. Undo: metasystem work stop g.", Act: []string{"metasystem", "work", "revise", "g", "--work", "u", "--after", "1", "--brief", "brief.md", "--reason", "the machinery stopped: a build step failed"}},
		},
		Recommendation: "Continue"}
}

func noQuestionFiles(t *testing.T, root string) {
	t.Helper()
	if files, _ := filepath.Glob(filepath.Join(channelRoot(root), "questions", "*.json")); len(files) != 0 {
		t.Fatalf("a refused ask wrote %v", files)
	}
}

func TestContractAskRefusesWordsAPersonCannotRead(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name  string
		amend func(*AskRequest)
	}{
		{"no decision", func(r *AskRequest) { r.Plain.Decision = " " }},
		{"no if-nothing", func(r *AskRequest) { r.Plain.IfNothing = "" }},
		{"an option without a consequence", func(r *AskRequest) { r.Options[0].Consequence = "" }},
		{"a decision of 281 runes", func(r *AskRequest) { r.Plain.Decision = strings.Repeat("é", 281) }},
		{"a consequence of 201 runes", func(r *AskRequest) { r.Options[1].Consequence = strings.Repeat("é", 201) }},
		{"a recommendation that is no label", func(r *AskRequest) { r.Recommendation = "Maybe" }},
		{"a repeated label", func(r *AskRequest) { r.Options[0].Label = "continue" }},
		{"an empty label", func(r *AskRequest) { r.Options[0].Label = "" }},
		{"a label of 65 bytes", func(r *AskRequest) { r.Options[0].Label = strings.Repeat("a", 65) }},
		{"a 10-hex run", func(r *AskRequest) { r.Plain.Decision += " at abcdef0123" }},
		{"a path", func(r *AskRequest) { r.Options[0].Consequence = "the work stays in /Users/x" }},
		{"a pid", func(r *AskRequest) { r.Plain.IfNothing = "pid 6378 waits" }},
		{"a generation", func(r *AskRequest) { r.Plain.Decision = "the seat restarted at generation 2" }},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			request := contractRequest(root)
			row.amend(&request)
			if _, err := Ask(request); err == nil || err.Error() == "" {
				t.Fatalf("the contract ask was accepted: %v", err)
			}
			noQuestionFiles(t, root)
		})
	}
	t.Run("a legacy ask keeps today's text", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		q, err := Ask(AskRequest{RepoRoot: root, About: "lane", Kind: "other", Machine: "m", Facts: []string{"metasystem landing resume"}, Now: time.Unix(10, 0),
			Plain: &Plain{Decision: "the lane on pid 6378 at /Users/x stopped at abcdef0123", Act: "metasystem landing resume", IfNothing: "the lane does not do this"}})
		want := "The lane on pid 6378 at /Users/x stopped at abcdef0123.\nTo go on: metasystem landing resume\nIf you do nothing: the lane does not do this."
		if err != nil || q.Text != want {
			t.Fatalf("legacy ask: %q %v, want %q", q.Text, err, want)
		}
	})
}

func TestContractQuestionRendersInContractOrder(t *testing.T) {
	t.Parallel()
	for _, codeOff := range []bool{false, true} {
		root := t.TempDir()
		answer := "Reply with the word and your code: Continue / Stop here"
		if codeOff {
			answer = "Reply: Continue / Stop here"
			if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("channel.human.answer-code=off\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		request := contractRequest(root)
		request.Plain.Decision = "D" + strings.Repeat("é", 278) + "."
		q, err := Ask(request)
		if err != nil {
			t.Fatal(err)
		}
		want := strings.Join([]string{request.Plain.Decision, "",
			"★ Continue — " + request.Options[1].Consequence,
			"Stop here — " + request.Options[0].Consequence, "",
			"If you do nothing: it waits; nothing builds or lands until you choose.", answer}, "\n")
		if q.Text != want {
			t.Fatalf("contract text (code off %t):\n%s\nwant:\n%s", codeOff, q.Text, want)
		}
		if got := ReplyInstructionsAt(root, q); got != answer {
			t.Fatalf("question show's answer line = %q, want %q", got, answer)
		}
	}
}

// actScript stands in for the act's verb: it records each argv and answers
// with the envelope the row scripts.
type actScript struct {
	calls [][]string
	reply func(argv []string) string
}

func (s *actScript) run(_ context.Context, _ string, argv []string) ([]byte, error) {
	s.calls = append(s.calls, argv)
	return []byte(s.reply(argv)), nil
}

type choiceBed struct {
	t        *testing.T
	repo     *pollRepository
	provider *testProvider
	act      *actScript
	q        Question
	now      time.Time
	sent     int
}

func newChoiceBed(t *testing.T) *choiceBed {
	t.Helper()
	repo, provider, legacy, now := pollLedgerBed(t)
	if err := os.Remove(questionPath(repo.root, legacy.ID)); err != nil {
		t.Fatal(err)
	}
	conf := filepath.Join(repo.root, "metasystem.conf")
	data, _ := os.ReadFile(conf)
	if err := os.WriteFile(conf, append(data, []byte("channel.human.answer-code=off\n")...), 0o600); err != nil {
		t.Fatal(err)
	}
	request := contractRequest(repo.root)
	request.UnitStop = &UnitStopQuestion{Loop: "unit-build", Subject: "g/u/run", Attempt: 1, Finding: "build-partial", Needs: "metasystem work revise g --by Wido", AcceptableActs: []string{"work-revise"}}
	q, err := Ask(request)
	if err != nil {
		t.Fatal(err)
	}
	b := &choiceBed{t: t, repo: repo, provider: provider, q: q, now: now}
	b.act = &actScript{reply: func([]string) string { return b.started() }}
	b.poll()
	if b.question().Thread == nil {
		t.Fatal("the question was not posted")
	}
	return b
}

// started is the act closing its own typed question, as work revise's
// record does, and saying so with a later stop's outcome word.
func (b *choiceBed) started() string {
	act := UnitStopAct{ID: "revise-" + b.q.ID, Goal: "g", Loop: "unit-build", Subject: "g/u/run", Attempt: 1, Findings: []string{"build-partial"}, Kind: "work-revise", Reason: "continued", At: b.now}
	if err := RecordUnitStopAct(b.repo.root, act); err != nil {
		b.t.Fatal(err)
	}
	return `{"outcome":"refused","summary":"the next round stopped","data":{"answer":{"state":"started"}}}`
}

func (b *choiceBed) config() PollConfig {
	c := pollBedConfig(b.repo, b.provider, b.now)
	c.RunAct = b.act.run
	return c
}

func (b *choiceBed) poll() {
	b.t.Helper()
	if _, err := b.repo.poll(context.Background(), b.config()); err != nil {
		b.t.Fatal(err)
	}
}

// reply is the person's new message in the question's thread.
func (b *choiceBed) reply(text string) {
	b.sent++
	id := "reply-" + string(rune('a'+b.sent))
	b.provider.inbound = append(b.provider.inbound, Inbound{Ref: MessageRef{ID: id, ThreadID: "1"}, ThreadID: "1", UserID: "UWIDO", Text: text, SentAt: b.now})
}

func (b *choiceBed) question() Question {
	q, err := ReadQuestion(b.repo.root, b.q.ID)
	if err != nil {
		b.t.Fatal(err)
	}
	return q
}

func (b *choiceBed) lastAttempt() Attempt {
	attempts := b.question().Attempts
	if len(attempts) == 0 {
		b.t.Fatal("no attempt recorded")
	}
	return attempts[len(attempts)-1]
}

// posted is every line posted after the question itself.
func (b *choiceBed) posted() []string { return b.provider.posts[1:] }

func (b *choiceBed) wantPosted(lines ...string) {
	b.t.Helper()
	if !slices.Equal(b.posted(), lines) {
		b.t.Fatalf("thread = %q, want %q", b.posted(), lines)
	}
}

func TestChannelChoiceRunsItsActInTheFullPoll(t *testing.T) {
	t.Parallel()
	confirmation := func(b *choiceBed) string {
		return `Continue it is. The builder's next round of "g" started at ` + b.now.Local().Format("15:04") + ". If it stops again, I ask here. Undo: metasystem work stop g."
	}
	t.Run("continue acts once and a second continue is already done", func(t *testing.T) {
		t.Parallel()
		b := newChoiceBed(t)
		b.reply(" continue ")
		b.poll()
		want := append(slices.Clone(b.q.Options[1].Act), "--answer", b.q.ID, "--json")
		q := b.question()
		if len(b.act.calls) != 1 || !slices.Equal(b.act.calls[0], want) {
			t.Fatalf("act runs = %q, want %q", b.act.calls, want)
		}
		if q.Answer == nil || q.Answer.Phase != "acted" || q.State != "closed" || b.lastAttempt().Outcome != "acted" || b.lastAttempt().Label != "Continue" {
			t.Fatalf("after the act: %+v", q)
		}
		b.poll()
		b.wantPosted(confirmation(b))
		b.reply("Continue")
		b.poll()
		b.poll()
		b.wantPosted(confirmation(b), "Already done at "+b.now.Local().Format("15:04")+" (Continue).")
		if len(b.act.calls) != 1 {
			t.Fatalf("a repeat reply ran the act again: %q", b.act.calls)
		}
		b.poll()
		if len(b.posted()) != 2 {
			t.Fatalf("a handled reply was read again: %q", b.posted())
		}
	})
	t.Run("stop here keeps the question and a later continue acts", func(t *testing.T) {
		t.Parallel()
		b := newChoiceBed(t)
		b.reply("stop here")
		b.poll()
		q := b.question()
		if q.State != "open" || q.Answer != nil || b.lastAttempt().Outcome != "chosen" || len(b.act.calls) != 0 || len(GoalOpenQuestions(b.repo.root)) != 1 {
			t.Fatalf("stop here: %+v calls=%q", q, b.act.calls)
		}
		b.poll()
		b.wantPosted("Stop here it is. Nothing runs until you choose again; this question stays open.")
		b.reply("continue")
		b.poll()
		if len(b.act.calls) != 1 || b.question().Answer.Phase != "acted" {
			t.Fatalf("a later continue did not act: %q %+v", b.act.calls, b.question())
		}
	})
	t.Run("an unknown word is not caught and never read again", func(t *testing.T) {
		t.Parallel()
		b := newChoiceBed(t)
		b.reply("maybe")
		b.poll()
		b.poll()
		b.poll()
		q := b.question()
		if q.Answer != nil || q.State != "open" || len(q.Attempts) != 0 || len(b.act.calls) != 0 {
			t.Fatalf("an unknown word was recorded: %+v", q)
		}
		b.wantPosted("I did not catch that. Reply: Continue / Stop here")
		b.reply("continue")
		b.poll()
		if len(b.act.calls) != 1 {
			t.Fatalf("the next reply was not matched: %q", b.act.calls)
		}
	})
	t.Run("a refused act leaves the question answerable", func(t *testing.T) {
		t.Parallel()
		b := newChoiceBed(t)
		b.act.reply = func([]string) string {
			return `{"outcome":"refused","summary":"the goal's budget is closed (5 days used)","data":{"answer":{"state":"refused"}}}`
		}
		b.reply("continue")
		b.poll()
		q := b.question()
		if q.Answer != nil || q.State != "open" || b.lastAttempt().Outcome != "refused" || b.lastAttempt().Message != goal.ChannelInboxPath("fleet", &goal.ChannelInbound{Provider: "fake", MessageID: "reply-b"}) {
			t.Fatalf("refused: %+v", q)
		}
		b.reply("stop here")
		b.poll()
		b.act.reply = func([]string) string { return b.started() }
		b.poll()
		b.poll()
		if len(b.act.calls) != 1 {
			t.Fatalf("the original continue ran again once the act could: %q", b.act.calls)
		}
		b.wantPosted("I could not continue: the goal's budget is closed (5 days used). The question above is still open; reply Continue again once that is fixed.",
			"Stop here it is. Nothing runs until you choose again; this question stays open.")
		b.reply("continue")
		b.poll()
		if len(b.act.calls) != 2 || b.question().Answer.Phase != "acted" {
			t.Fatalf("a new continue did not act: %q", b.act.calls)
		}
	})
	t.Run("a refusal in words a person cannot read is said plainly", func(t *testing.T) {
		t.Parallel()
		b := newChoiceBed(t)
		b.act.reply = func([]string) string { return `{"outcome":"refused","summary":"run 20261024abcdef0123 is busy"}` }
		b.reply("continue")
		b.poll()
		b.poll()
		b.wantPosted("I could not continue: the command refused it; metasystem status g at your terminal says why. The question above is still open; reply Continue again once that is fixed.")
	})
	t.Run("the settlement pass records and never acts", func(t *testing.T) {
		t.Parallel()
		b := newChoiceBed(t)
		b.reply("continue")
		c := b.config()
		c.answerCodeOff, c.MaxDispositions = true, 5
		endpoint := func() (goal.Endpoint, error) { return b.repo.endpoint, nil }
		if err := receiveToInbox(context.Background(), c, nil, endpoint, &PollResult{}); err != nil {
			t.Fatal(err)
		}
		settle := b.config()
		settle.Provider = nil
		if _, err := SettleReplies(context.Background(), settle, b.repo.resolve); err != nil {
			t.Fatal(err)
		}
		if q := b.question(); q.Answer == nil || q.Answer.Phase != "recorded" || len(b.act.calls) != 0 {
			t.Fatalf("the settlement pass: %+v calls=%q", q, b.act.calls)
		}
		b.poll()
		b.poll()
		if len(b.act.calls) != 1 || b.question().Answer.Phase != "acted" {
			t.Fatalf("the full poll did not act once: %q", b.act.calls)
		}
	})
	t.Run("an accepted act waits for a fresh reply", func(t *testing.T) {
		t.Parallel()
		b := newChoiceBed(t)
		b.act.reply = func([]string) string {
			return `{"outcome":"refused","summary":"refused","data":{"answer":{"state":"accepted","hold":"the goal's budget is used up"}}}`
		}
		b.reply("continue")
		b.poll()
		if q := b.question(); q.State != "open" || q.Answer == nil || q.Answer.Phase != "accepted" || b.lastAttempt().Outcome != "accepted" {
			t.Fatalf("accepted: %+v", q)
		}
		b.act.reply = func([]string) string { return b.started() }
		b.poll()
		b.poll()
		if len(b.act.calls) != 1 {
			t.Fatalf("a poll restarted the held request by itself: %q", b.act.calls)
		}
		b.wantPosted("Continue it is; it waits: the goal's budget is used up. When that is fixed, reply Continue again.")
		b.reply("continue")
		b.poll()
		b.poll()
		if len(b.act.calls) != 2 || b.question().Answer.Phase != "acted" {
			t.Fatalf("the fresh reply did not act: %q %+v", b.act.calls, b.question())
		}
		b.wantPosted("Continue it is; it waits: the goal's budget is used up. When that is fixed, reply Continue again.", confirmation(b))
	})
	t.Run("retry and accept the size say what went on", func(t *testing.T) {
		t.Parallel()
		for label, did := range map[string]string{"Retry": `The failed step of "g" started again`, "Accept the size": `The checks of "g" went on`} {
			b := newChoiceBed(t)
			q := b.question()
			q.Options[1].Label, q.Recommendation = label, label
			if err := writeJSON(questionPath(b.repo.root, q.ID), q); err != nil {
				t.Fatal(err)
			}
			b.reply(strings.ToLower(label))
			b.poll()
			b.poll()
			b.wantPosted(label + " it is. " + did + " at " + b.now.Local().Format("15:04") + ". If it stops again, I ask here. Undo: metasystem work stop g.")
		}
	})
	t.Run("one more day names the new time box", func(t *testing.T) {
		t.Parallel()
		b := newChoiceBed(t)
		q := b.question()
		q.Options[1].Label, q.Recommendation = "One more day", "One more day"
		if err := writeJSON(questionPath(b.repo.root, q.ID), q); err != nil {
			t.Fatal(err)
		}
		b.act.reply = func([]string) string {
			return `{"outcome":"confirmed","data":{"answer":{"state":"started"},"budget":{"box":"7d4h/4/240m/2/3"}}}`
		}
		b.reply("one more day")
		b.poll()
		b.poll()
		b.wantPosted(`One more day it is. "g" has one more working day and goes on (its time box is now 7d4h) at ` + b.now.Local().Format("15:04") + ". If it stops again, I ask here. Undo: metasystem work stop g.")
	})
	t.Run("an act that found the goal going on says so", func(t *testing.T) {
		t.Parallel()
		b := newChoiceBed(t)
		b.act.reply = func([]string) string { return `{"outcome":"unchanged","data":{"answer":{"state":"settled"}}}` }
		b.reply("continue")
		b.poll()
		b.poll()
		if q := b.question(); q.Answer == nil || q.Answer.Phase != "acted" {
			t.Fatalf("a settled act is not done: %+v", q)
		}
		b.wantPosted(`Continue it is. "g" is already going on; nothing more was needed.`)
	})
	t.Run("a reply to the executor's own line is a reply to the question", func(t *testing.T) {
		t.Parallel()
		b := newChoiceBed(t)
		b.act.reply = func([]string) string { return `{"outcome":"refused","summary":"the goal's budget is closed"}` }
		b.reply("continue")
		b.poll()
		b.poll()
		// The test provider names each post by its count; the refusal line
		// is the last post.
		notice := fmt.Sprint(len(b.provider.posts))
		if len(b.posted()) != 1 || notice == b.question().Thread.ID {
			t.Fatalf("the refusal line was not posted: %q", b.posted())
		}
		b.act.reply = func([]string) string { return b.started() }
		b.provider.inbound = append(b.provider.inbound, Inbound{Ref: MessageRef{ID: "to-the-notice", ThreadID: notice}, ThreadID: notice, UserID: "UWIDO", Text: "continue", SentAt: b.now})
		b.poll()
		if len(b.act.calls) != 2 || b.question().Answer.Phase != "acted" {
			t.Fatalf("a reply to the refusal line did not act: %q %+v", b.act.calls, b.question())
		}
	})
	t.Run("a reply to a question its unit closed says when", func(t *testing.T) {
		t.Parallel()
		for _, kept := range []bool{true, false} {
			b := newChoiceBed(t)
			// Another open question keeps the inbox read.
			other := contractRequest(b.repo.root)
			other.Facts = []string{"another stop"}
			if _, err := Ask(other); err != nil {
				t.Fatal(err)
			}
			closed := b.now.Add(-time.Hour)
			if err := RecordUnitStopAct(b.repo.root, UnitStopAct{ID: "closure", Goal: "g", Loop: "unit-build", Subject: "g/u/run", Attempt: 1, Findings: []string{"build-partial"}, Kind: "work-revise", Reason: "continued at the terminal", At: closed}); err != nil {
				t.Fatal(err)
			}
			want := "This was settled at " + closed.Local().Format("15:04") + ": continued at the terminal."
			if !kept {
				q := b.question()
				q.ClosedAt = time.Time{}
				if err := writeJSON(questionPath(b.repo.root, q.ID), q); err != nil {
					t.Fatal(err)
				}
				want = "This was settled: continued at the terminal."
			}
			if q := b.question(); q.State != "closed" || kept && !q.ClosedAt.Equal(closed) {
				t.Fatalf("the closure kept no time: %+v", q)
			}
			b.reply("continue")
			b.poll()
			b.poll()
			if posted := b.posted(); len(posted) == 0 || posted[len(posted)-1] != want || len(b.act.calls) != 0 {
				t.Fatalf("settled line = %q, want %q (calls %q)", posted, want, b.act.calls)
			}
		}
	})
	t.Run("a reply after the act is answered for one day", func(t *testing.T) {
		t.Parallel()
		for _, late := range []bool{false, true} {
			b := newChoiceBed(t)
			b.reply("continue")
			b.poll()
			b.poll()
			acted := b.now
			b.now = acted.Add(24*time.Hour - time.Minute)
			if late {
				b.now = acted.Add(24*time.Hour + time.Minute)
			}
			b.reply("continue")
			b.poll()
			b.poll()
			want := []string{confirmation(&choiceBed{now: acted}), "Already done at " + acted.Local().Format("15:04") + " (Continue)."}
			if late {
				want = want[:1]
			}
			b.wantPosted(want...)
		}
	})
}

func TestDefaultActRunnerRunsTheEngineInTheRepository(t *testing.T) {
	t.Parallel()
	engine, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	var name string
	var args []string
	var cancelled error
	runner := engineActRunner(func(ctx context.Context, path string, words ...string) *exec.Cmd {
		name, args, cancelled = path, words, ctx.Err()
		return exec.CommandContext(ctx, "true")
	})
	duty, cancel := context.WithCancel(context.Background())
	cancel()
	dir := t.TempDir()
	if _, err := runner(duty, dir, []string{"metasystem", "work", "revise", "g"}); err != nil && !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if name != engine || !slices.Equal(args, []string{"work", "revise", "g"}) || cancelled != nil {
		t.Fatalf("default act runner: %q %q cancelled=%v", name, args, cancelled)
	}
	var seenDir string
	runner = engineActRunner(func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		cmd := exec.CommandContext(ctx, "pwd")
		cmd.Dir = "elsewhere"
		return cmd
	})
	out, err := runner(context.Background(), dir, []string{"metasystem"})
	seenDir = strings.TrimSpace(string(out))
	resolved, _ := filepath.EvalSymlinks(dir)
	if err != nil || (seenDir != dir && seenDir != resolved) {
		t.Fatalf("the act ran in %q (%v), want %q", seenDir, err, dir)
	}
}
