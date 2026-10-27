package channel_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
)

// tapClock is the fixed clock of the tap bed: the bed's polls and the fake bot.
var tapClock = time.Date(2026, 10, 26, 9, 30, 0, 0, time.UTC)

// tapBed is the one-bot bed with the seat polled and its acts scripted.
type tapBed struct {
	*inboxBed
	inst  inboxInstallation
	calls [][]string
	// during runs inside each act, before it answers.
	during func()
}

func newTapBed(t *testing.T, codeOff bool) *tapBed {
	t.Helper()
	setup := inboxSetup{now: tapClock}
	if codeOff {
		setup.conf = "channel.human.answer-code=off\n"
	}
	bed := newInboxBed(t, setup)
	return &tapBed{inboxBed: bed, inst: bed.seat}
}

func (b *tapBed) request() channel.AskRequest {
	return channel.AskRequest{RepoRoot: b.inst.root, Goal: "seat-goal", Kind: "other", Machine: b.inst.machine, Lineage: b.inst.name, Facts: []string{"the build of seat-goal stopped"}, Now: b.now.Add(-10 * time.Minute), Contract: true,
		Plain: &channel.Plain{Decision: "seat-goal: the build stopped (a build step failed)", IfNothing: "it waits; nothing builds until you choose"},
		Options: []channel.Option{
			{Label: "Stop here", Consequence: "the work stays where it is; nothing runs. This question stays open."},
			{Label: "Continue", Consequence: "the builder finishes the unit. Undo: metasystem work stop seat-goal.", Act: []string{"metasystem", "work", "revise", "seat-goal", "--reason", "continued on the phone"}},
		},
		Recommendation: "Continue"}
}

func (b *tapBed) askContract(t *testing.T) channel.Question {
	t.Helper()
	q, err := channel.Ask(b.request())
	if err != nil {
		t.Fatal(err)
	}
	return q
}

func (b *tapBed) pollWith(t *testing.T, ctx context.Context) {
	t.Helper()
	c := b.config(b.inst)
	c.RunAct = func(_ context.Context, _ string, argv []string) ([]byte, error) {
		b.calls = append(b.calls, argv)
		if b.during != nil {
			b.during()
		}
		return []byte(`{"data":{"answer":{"state":"started"}}}`), nil
	}
	if _, err := channel.Poll(ctx, c); err != nil {
		t.Fatalf("poll: %v", err)
	}
}

func (b *tapBed) pollOnce(t *testing.T) {
	t.Helper()
	b.pollWith(t, context.Background())
}

// tap has user tap the button label on message onMessage.
func (b *tapBed) tap(t *testing.T, onMessage, label string, user int) {
	t.Helper()
	row, err := json.Marshal(map[string]any{"face": "telegram", "user": user, "tap": label, "reply_to": json.Number(onMessage)})
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(filepath.Join(b.fakeDir, "replies.jsonl"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Write(append(row, '\n')); err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
}

// posted asks the contract question and polls once; it returns the question
// with its thread and the sendMessage row that posted it.
func (b *tapBed) posted(t *testing.T) (channel.Question, journalRow) {
	t.Helper()
	q := b.askContract(t)
	b.pollOnce(t)
	q = b.question(t, b.inst, q.ID)
	sent := b.sent(t)
	if q.Thread == nil || len(sent) != 1 {
		t.Fatalf("the question was not posted once: %+v %+v", q.Thread, sent)
	}
	return q, sent[0]
}

func keyboardOf(row journalRow) []string {
	markup, _ := row.Form["reply_markup"].(map[string]any)
	rows, _ := markup["inline_keyboard"].([]any)
	var labels []string
	for _, r := range rows {
		for _, button := range r.([]any) {
			b := button.(map[string]any)
			if b["text"] != b["callback_data"] {
				return []string{"a button whose data is not its label"}
			}
			labels = append(labels, b["text"].(string))
		}
	}
	return labels
}

func messageID(row journalRow) string {
	id, _ := row.Form["message_id"].(float64)
	return strconv.FormatInt(int64(id), 10)
}

func textOf(row journalRow) string { text, _ := row.Form["text"].(string); return text }

// threadLines are the texts posted in reply to message id.
func (b *tapBed) threadLines(t *testing.T, id string) []string {
	t.Helper()
	var out []string
	for _, row := range b.sent(t) {
		if replyParent(row) == id {
			out = append(out, textOf(row))
		}
	}
	return out
}

func (b *tapBed) edits(t *testing.T) []journalRow {
	t.Helper()
	var out []journalRow
	for _, row := range b.journal(t) {
		if row.Method == "editMessageReplyMarkup" {
			out = append(out, row)
		}
	}
	return out
}

func (b *tapBed) recordsBy(t *testing.T, user string) []inboxRecord {
	t.Helper()
	var out []inboxRecord
	for _, r := range b.inbox(t) {
		if r.UserID == user {
			out = append(out, r)
		}
	}
	return out
}

const tapAnswerLine = "Tap a button, or reply: Continue / Stop here"

func TestATapAnswersAContractQuestion(t *testing.T) {
	t.Parallel()
	t.Run("a tap on continue acts once", func(t *testing.T) {
		t.Parallel()
		b := newTapBed(t, true)
		q, row := b.posted(t)
		if got := keyboardOf(row); !slices.Equal(got, []string{"Continue", "Stop here"}) {
			t.Fatalf("keyboard = %q", got)
		}
		if text := textOf(row); !strings.HasSuffix(text, "\n"+tapAnswerLine) || text != strings.TrimSuffix(q.Text, "Reply: Continue / Stop here")+tapAnswerLine {
			t.Fatalf("posted text = %q (stored %q)", text, q.Text)
		}
		b.tap(t, q.Thread.ID, "Continue", 7001)
		b.pollOnce(t)
		r := b.recordFor(t, "Continue")
		if !strings.HasPrefix(r.MessageID, "tap-") || r.Path != "plans/channel/inbox/fleet/telegram-"+r.MessageID+".json" || r.Outcome != "verified" || r.ReplyTo == nil || *r.ReplyTo != q.Thread.ID {
			t.Fatalf("tap record: %+v", r)
		}
		want := append(slices.Clone(q.Options[1].Act), "--answer", q.ID, "--json")
		if len(b.calls) != 1 || !slices.Equal(b.calls[0], want) {
			t.Fatalf("act runs = %q, want %q", b.calls, want)
		}
		got := b.question(t, b.inst, q.ID)
		if got.Answer == nil || got.Answer.Phase != "acted" || got.Answer.UserID != inboxHuman {
			t.Fatalf("answer = %+v", got.Answer)
		}
		b.pollOnce(t)
		if lines := b.threadLines(t, q.Thread.ID); len(lines) != 1 || !strings.HasPrefix(lines[0], "Continue it is. ") {
			t.Fatalf("thread = %q, want the confirmation once", lines)
		}
	})
	t.Run("a tap from someone else acts nothing", func(t *testing.T) {
		t.Parallel()
		b := newTapBed(t, true)
		q, _ := b.posted(t)
		b.tap(t, q.Thread.ID, "Continue", 7002)
		b.pollOnce(t)
		b.pollOnce(t)
		if records := b.recordsBy(t, "7002"); len(records) != 1 || records[0].Outcome != "wrong-user" {
			t.Fatalf("records of the other user: %+v", records)
		}
		if lines := b.threadLines(t, q.Thread.ID); !slices.Equal(lines, []string{"not recorded: wrong user. Reply to the question above with your answer"}) {
			t.Fatalf("thread = %q", lines)
		}
		if got := b.question(t, b.inst, q.ID); got.Answer != nil || got.State != "open" || len(b.calls) != 0 {
			t.Fatalf("another user's tap answered: %+v calls %q", got, b.calls)
		}
		b.tap(t, q.Thread.ID, "Continue", 7001)
		b.pollOnce(t)
		if len(b.calls) != 1 {
			t.Fatalf("the human's tap after another's: act runs %q", b.calls)
		}
	})
	t.Run("stop here keeps the buttons and a later tap acts", func(t *testing.T) {
		t.Parallel()
		b := newTapBed(t, true)
		q, _ := b.posted(t)
		b.tap(t, q.Thread.ID, "Stop here", 7001)
		b.pollOnce(t)
		b.pollOnce(t)
		if got := b.question(t, b.inst, q.ID); got.State != "open" || got.Answer != nil || len(b.calls) != 0 || len(b.edits(t)) != 0 {
			t.Fatalf("stop here: %+v calls %q edits %+v", got, b.calls, b.edits(t))
		}
		b.tap(t, q.Thread.ID, "Continue", 7001)
		b.pollOnce(t)
		if len(b.calls) != 1 || b.question(t, b.inst, q.ID).Answer.Phase != "acted" {
			t.Fatalf("a later tap on continue did not act: %q", b.calls)
		}
	})
	t.Run("a second tap is already done", func(t *testing.T) {
		t.Parallel()
		b := newTapBed(t, true)
		q, _ := b.posted(t)
		b.tap(t, q.Thread.ID, "Continue", 7001)
		b.pollOnce(t)
		b.pollOnce(t)
		b.tap(t, q.Thread.ID, "Continue", 7001)
		b.pollOnce(t)
		b.pollOnce(t)
		lines := b.threadLines(t, q.Thread.ID)
		if len(lines) != 2 || lines[1] != "Already done at "+tapClock.Local().Format("15:04")+" (Continue)." || len(b.calls) != 1 {
			t.Fatalf("thread = %q, act runs %q", lines, b.calls)
		}
	})
	t.Run("the buttons go after the act, even past the duty's deadline", func(t *testing.T) {
		t.Parallel()
		b := newTapBed(t, true)
		q, _ := b.posted(t)
		b.tap(t, q.Thread.ID, "Continue", 7001)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		b.during = cancel
		b.pollWith(t, ctx)
		edits := b.edits(t)
		if len(edits) != 1 || messageID(edits[0]) != q.Thread.ID {
			t.Fatalf("editMessageReplyMarkup rows = %+v, want one on message %s", edits, q.Thread.ID)
		}
		if len(b.calls) != 1 {
			t.Fatalf("act runs = %q", b.calls)
		}
	})
	t.Run("the reminder carries the buttons and loses them after the act", func(t *testing.T) {
		t.Parallel()
		b := newTapBed(t, true)
		b.now = time.Date(2026, 10, 26, 10, 0, 0, 0, time.Local)
		q, _ := b.posted(t)
		b.now = time.Date(2026, 10, 26, 13, 0, 0, 0, time.Local)
		b.pollOnce(t)
		sent := b.sent(t)
		reminded := b.question(t, b.inst, q.ID).Reminded
		if len(sent) != 2 || reminded == nil || !strings.HasPrefix(textOf(sent[1]), "Still waiting: ") || replyParent(sent[1]) != q.Thread.ID {
			t.Fatalf("the reminder was posted as %+v (reminded %+v)", sent, reminded)
		}
		if got := keyboardOf(sent[1]); !slices.Equal(got, []string{"Continue", "Stop here"}) || !strings.HasSuffix(textOf(sent[1]), "\n"+tapAnswerLine) {
			t.Fatalf("reminder keyboard = %q text %q", got, textOf(sent[1]))
		}
		b.tap(t, reminded.ID, "Continue", 7001)
		b.pollOnce(t)
		b.pollOnce(t)
		edits := b.edits(t)
		if len(b.calls) != 1 || len(edits) != 2 || messageID(edits[0]) != q.Thread.ID || messageID(edits[1]) != reminded.ID {
			t.Fatalf("act runs %q, editMessageReplyMarkup rows %+v, want the question's %s and the reminder's %s", b.calls, edits, q.Thread.ID, reminded.ID)
		}
	})
	t.Run("asking with a provider posts the buttons", func(t *testing.T) {
		t.Parallel()
		b := newTapBed(t, true)
		request := b.request()
		request.Provider, request.Destination = b.inst.provider, b.inst.dest
		asked, _, err := channel.AskOrFind(request)
		if err != nil {
			t.Fatal(err)
		}
		sent := b.sent(t)
		if len(sent) != 1 || !slices.Equal(keyboardOf(sent[0]), []string{"Continue", "Stop here"}) || !strings.HasSuffix(textOf(sent[0]), "\n"+tapAnswerLine) {
			t.Fatalf("AskOrFind posted %+v", sent)
		}
		q := b.question(t, b.inst, asked.ID)
		if q.Thread == nil {
			t.Fatal("the asked question has no thread")
		}
		b.pollOnce(t)
		if len(b.sent(t)) != 1 {
			t.Fatalf("the poll posted the question again: %+v", b.sent(t))
		}
		b.tap(t, q.Thread.ID, "Continue", 7001)
		b.pollOnce(t)
		if len(b.calls) != 1 {
			t.Fatalf("a tap on the asked question's message did not act: %q", b.calls)
		}
	})
	t.Run("question retry posts the buttons", func(t *testing.T) {
		t.Parallel()
		b := newTapBed(t, true)
		q := b.askContract(t)
		if _, outcome, err := channel.RetryDelivery(context.Background(), b.inst.root, q.ID, b.inst.provider, b.inst.dest); err != nil || outcome != channel.RetryDelivered {
			t.Fatalf("retry: %s %v", outcome, err)
		}
		sent := b.sent(t)
		if len(sent) != 1 || !slices.Equal(keyboardOf(sent[0]), []string{"Continue", "Stop here"}) || !strings.HasSuffix(textOf(sent[0]), "\n"+tapAnswerLine) {
			t.Fatalf("question retry posted %+v", sent)
		}
	})
	t.Run("an unknown word earns the tap line", func(t *testing.T) {
		t.Parallel()
		b := newTapBed(t, true)
		q, _ := b.posted(t)
		b.reply(t, q.Thread.ID, "maybe", tapClock)
		b.pollOnce(t)
		b.pollOnce(t)
		b.pollOnce(t)
		if lines := b.threadLines(t, q.Thread.ID); !slices.Equal(lines, []string{"I did not catch that. " + tapAnswerLine}) {
			t.Fatalf("thread = %q", lines)
		}
	})
	t.Run("a legacy question gets no buttons", func(t *testing.T) {
		t.Parallel()
		b := newTapBed(t, true)
		q := b.ask(t, b.inst, "seat-goal", "Seat needs a decision")
		b.pollOnce(t)
		sent := b.sent(t)
		if len(sent) != 1 || sent[0].Form["reply_markup"] != nil || textOf(sent[0]) != q.Text {
			t.Fatalf("a legacy question was posted as %+v", sent)
		}
	})
}

func TestNoButtonsWithTheCodeOn(t *testing.T) {
	t.Parallel()
	b := newTapBed(t, false)
	q, row := b.posted(t)
	if row.Form["reply_markup"] != nil || !strings.HasSuffix(textOf(row), "\nReply with the word and your code: Continue / Stop here") {
		t.Fatalf("with the code on the question was posted as %+v", row.Form)
	}
	b.tap(t, q.Thread.ID, "Continue", 7001)
	b.pollOnce(t)
	b.pollOnce(t)
	if records := b.recordsBy(t, inboxHuman); len(records) != 1 || records[0].Outcome != "no-code" {
		t.Fatalf("a tap with the code on: %+v", records)
	}
	if lines := b.threadLines(t, q.Thread.ID); len(lines) != 1 || !strings.HasPrefix(lines[0], "not recorded: no code. ") {
		t.Fatalf("thread = %q", lines)
	}
	if got := b.question(t, b.inst, q.ID); got.Answer != nil || len(b.calls) != 0 {
		t.Fatalf("a tap without a code answered: %+v calls %q", got.Answer, b.calls)
	}
}
