package channel

import (
	"context"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

// localAt is 2026-09-03 plus days at hh:mm on this computer's clock.
func localAt(days, hh, mm int) time.Time {
	return time.Date(2026, 9, 3+days, hh, mm, 0, 0, time.Local)
}

// reminders are the "Still waiting:" posts of the bed, with their threads.
func (b *choiceBed) reminders() ([]string, []*MessageRef) {
	var texts []string
	var threads []*MessageRef
	for i, text := range b.provider.posts {
		if strings.HasPrefix(text, "Still waiting: ") {
			texts, threads = append(texts, text), append(threads, b.provider.postThreads[i])
		}
	}
	return texts, threads
}

func (b *choiceBed) pollAt(at time.Time) int {
	b.t.Helper()
	b.now = at
	b.poll()
	texts, _ := b.reminders()
	return len(texts)
}

func (b *choiceBed) wantReminders(at time.Time, want int) {
	b.t.Helper()
	if got := b.pollAt(at); got != want {
		b.t.Fatalf("at %s: %d reminders, want %d (posts %q)", at.Format("Jan 2 15:04"), got, want, b.provider.posts)
	}
}

func TestAReminderComesOnceInWakingHours(t *testing.T) {
	t.Parallel()
	t.Run("once, three hours after the ask, in the question's thread", func(t *testing.T) {
		t.Parallel()
		b := newChoiceBedOpened(t, localAt(0, 10, 0), nil)
		q := b.question()
		ifNothing := "If you do nothing: it waits; nothing builds or lands until you choose. I ask once more at 13:00."
		if !strings.Contains(q.Text, "\n"+ifNothing+"\n") {
			t.Fatalf("question text = %q, want the line %q", q.Text, ifNothing)
		}
		b.wantReminders(localAt(0, 12, 59), 0)
		b.wantReminders(localAt(0, 13, 0), 1)
		texts, threads := b.reminders()
		want := "Still waiting: " + strings.Replace(q.Text, " I ask once more at 13:00.", " This is the one reminder.", 1)
		if texts[0] != want || strings.Contains(texts[0], "I ask once more") {
			t.Fatalf("reminder = %q, want %q", texts[0], want)
		}
		if threads[0] == nil || threads[0].ID != q.Thread.ID {
			t.Fatalf("reminder thread = %+v, want a reply to %s", threads[0], q.Thread.ID)
		}
		reminded := b.question()
		if reminded.Reminded == nil || !slices.Contains(reminded.Notices, reminded.Reminded.ID) {
			t.Fatalf("reminded = %+v notices %q", reminded.Reminded, reminded.Notices)
		}
		b.wantReminders(localAt(0, 16, 0), 1)
		b.provider.inbound = append(b.provider.inbound, Inbound{Ref: MessageRef{ID: "reply-r", ThreadID: reminded.Reminded.ID}, ThreadID: reminded.Reminded.ID, UserID: "UWIDO", Text: "continue", SentAt: b.now})
		b.poll()
		b.poll()
		if len(b.act.calls) != 1 || b.question().Answer == nil || b.question().Answer.Phase != "acted" {
			t.Fatalf("a reply to the reminder: act runs %q, question %+v", b.act.calls, b.question())
		}
	})
	t.Run("asked in the evening, asked again the next morning", func(t *testing.T) {
		t.Parallel()
		b := newChoiceBedOpened(t, localAt(0, 20, 30), nil)
		if text := b.question().Text; !strings.Contains(text, "until you choose. I ask once more tomorrow at 08:00.\n") {
			t.Fatalf("question text = %q", text)
		}
		b.wantReminders(localAt(0, 23, 30), 0)
		b.wantReminders(localAt(1, 7, 59), 0)
		b.wantReminders(localAt(1, 8, 0), 1)
	})
	t.Run("a reminder that failed at 21:59 waits for the morning", func(t *testing.T) {
		t.Parallel()
		b := newChoiceBedOpened(t, localAt(0, 18, 0), nil)
		b.provider.failPosts = 1
		b.wantReminders(localAt(0, 21, 59), 0)
		if b.question().Reminded != nil {
			t.Fatal("a failed reminder was recorded as posted")
		}
		b.wantReminders(localAt(0, 22, 1), 0)
		b.wantReminders(localAt(1, 7, 59), 0)
		b.wantReminders(localAt(1, 8, 0), 1)
	})
	t.Run("an answer that waits gets no reminder", func(t *testing.T) {
		t.Parallel()
		b := newChoiceBedOpened(t, localAt(0, 10, 0), nil)
		b.act.reply = func([]string) string {
			return `{"outcome":"accepted","data":{"answer":{"state":"accepted","hold":"the lane is busy"}}}`
		}
		b.reply("continue")
		b.poll()
		if q := b.question(); q.Answer == nil || q.Answer.Phase != "accepted" {
			t.Fatalf("the answer does not wait: %+v", q.Answer)
		}
		b.wantReminders(localAt(0, 14, 0), 0)
	})
	t.Run("after stop here no reminder comes", func(t *testing.T) {
		t.Parallel()
		b := newChoiceBedOpened(t, localAt(0, 10, 0), nil)
		b.reply("stop here")
		b.poll()
		if q := b.question(); q.Answer != nil || len(q.Attempts) != 1 {
			t.Fatalf("stop here: %+v", q)
		}
		b.wantReminders(localAt(0, 14, 0), 0)
	})
	t.Run("a contract question with no act says no time and gets no reminder", func(t *testing.T) {
		t.Parallel()
		b := newChoiceBedOpened(t, localAt(0, 10, 0), func(r *AskRequest) { r.Options[1].Act = nil })
		if text := b.question().Text; strings.Contains(text, "I ask once more") || !strings.Contains(text, "until you choose.\nReply: ") {
			t.Fatalf("question text = %q", text)
		}
		b.wantReminders(localAt(0, 14, 0), 0)
	})
	t.Run("a closed question gets no reminder", func(t *testing.T) {
		t.Parallel()
		b := newChoiceBedOpened(t, localAt(0, 10, 0), nil)
		q := b.question()
		q.State = "closed"
		if err := writeJSON(questionPath(b.repo.root, q.ID), q); err != nil {
			t.Fatal(err)
		}
		b.wantReminders(localAt(0, 14, 0), 0)
	})
	t.Run("a legacy question with an act gets no reminder", func(t *testing.T) {
		t.Parallel()
		repo, provider, q, _ := pollLedgerBed(t)
		q.OpenedAt, q.Text = localAt(0, 10, 0), "Goal g needs your answer."
		q.Options = []Option{{Label: "Continue", Consequence: "it goes on", Act: []string{"metasystem", "work", "revise", "g"}}}
		if err := writeJSON(questionPath(repo.root, q.ID), q); err != nil {
			t.Fatal(err)
		}
		if _, err := repo.poll(context.Background(), pollBedConfig(repo, provider, localAt(0, 14, 0))); err != nil {
			t.Fatal(err)
		}
		if len(provider.posts) != 0 {
			t.Fatalf("a legacy question was reminded: %q", provider.posts)
		}
	})
}

// failingReceive is the test provider whose receive fails.
type failingReceive struct{ *testProvider }

func (failingReceive) Receive(context.Context, DestinationConfig, []MessageRef, Cursor) ([]Inbound, Cursor, error) {
	return nil, "", os.ErrDeadlineExceeded
}

func TestBackOnlineIsSaidOnce(t *testing.T) {
	t.Parallel()
	// contractBed is the poll ledger bed with one undelivered contract question.
	// Its clock is in waking hours, when the back-online line is posted.
	day := localAt(1, 10, 0)
	contractBed := func(t *testing.T) (*pollRepository, *testProvider, Question, time.Time) {
		repo, provider, legacy, _ := pollLedgerBed(t)
		if err := os.Remove(questionPath(repo.root, legacy.ID)); err != nil {
			t.Fatal(err)
		}
		request := contractRequest(repo.root)
		request.Now = day
		q, err := Ask(request)
		if err != nil {
			t.Fatal(err)
		}
		return repo, provider, q, day
	}
	poll := func(t *testing.T, repo *pollRepository, p Provider, now time.Time) PollResult {
		t.Helper()
		r, err := repo.poll(context.Background(), pollBedConfig(repo, p, now))
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	down := func(t *testing.T, repo *pollRepository) (bool, bool) {
		t.Helper()
		ledger, err := readNotices(noticesPath(pollBedConfig(repo, nil, time.Time{})))
		_, statErr := os.Stat(noticesPath(pollBedConfig(repo, nil, time.Time{})))
		if err != nil {
			t.Fatal(err)
		}
		return !ledger.Down.IsZero(), statErr == nil
	}
	backOnline := func(now time.Time, q Question) string {
		return "Back online at " + now.Local().Format("15:04") + "; on machine, 1 question waits for you, newest first:\n- " +
			strings.SplitN(q.Text, "\n", 2)[0] + " (asked " + q.OpenedAt.Local().Format("15:04") + ")"
	}
	t.Run("a failed post is followed by one back-online line", func(t *testing.T) {
		t.Parallel()
		repo, p, q, now := contractBed(t)
		p.failPosts = 1
		if r := poll(t, repo, p, now); r.Undelivered != 1 {
			t.Fatalf("result = %+v", r)
		}
		if isDown, _ := down(t, repo); !isDown {
			t.Fatal("the failed post did not mark the channel down")
		}
		later := now.Add(2 * time.Minute)
		poll(t, repo, p, later)
		if len(p.posts) != 2 || p.posts[0] != q.Text || p.posts[1] != backOnline(later, q) || p.postThreads[1] != nil {
			t.Fatalf("posts = %q threads %v", p.posts, p.postThreads)
		}
		poll(t, repo, p, later.Add(2*time.Minute))
		if isDown, _ := down(t, repo); len(p.posts) != 2 || isDown {
			t.Fatalf("a third poll: posts %q down %t", p.posts, isDown)
		}
	})
	t.Run("a failure at night is followed by the line in the morning, once", func(t *testing.T) {
		t.Parallel()
		repo, p, legacy, _ := pollLedgerBed(t)
		if err := os.Remove(questionPath(repo.root, legacy.ID)); err != nil {
			t.Fatal(err)
		}
		request := contractRequest(repo.root)
		request.Now = localAt(0, 18, 0)
		q, err := Ask(request)
		if err != nil {
			t.Fatal(err)
		}
		poll(t, repo, p, localAt(0, 18, 0))
		p.failPosts = 1
		if r := poll(t, repo, p, localAt(0, 21, 59)); r.Undelivered != 1 {
			t.Fatalf("the reminder's post did not fail: %+v", r)
		}
		poll(t, repo, p, localAt(0, 22, 1))
		poll(t, repo, p, localAt(1, 7, 59))
		if isDown, _ := down(t, repo); len(p.posts) != 1 || !isDown {
			t.Fatalf("at night: posts %q down %t", p.posts, isDown)
		}
		poll(t, repo, p, localAt(1, 8, 0))
		poll(t, repo, p, localAt(1, 8, 2))
		if len(p.posts) != 3 || !strings.HasPrefix(p.posts[1], "Still waiting: ") || p.posts[2] != backOnline(localAt(1, 8, 0), q) {
			t.Fatalf("in the morning: posts %q", p.posts)
		}
	})
	t.Run("a question whose ask could not post is followed by one back-online line", func(t *testing.T) {
		t.Parallel()
		repo, p, legacy, _ := pollLedgerBed(t)
		if err := os.Remove(questionPath(repo.root, legacy.ID)); err != nil {
			t.Fatal(err)
		}
		request, now := contractRequest(repo.root), day
		request.Now, request.Provider = day, p
		p.failPosts = 1
		q, _, err := AskOrFind(request)
		if err != nil || q.Undelivered != 1 {
			t.Fatalf("ask: %+v %v", q, err)
		}
		poll(t, repo, p, now)
		poll(t, repo, p, now.Add(2*time.Minute))
		if len(p.posts) != 2 || p.posts[0] != q.Text || p.posts[1] != backOnline(now, q) {
			t.Fatalf("posts = %q", p.posts)
		}
	})
	t.Run("a receive error alone says nothing", func(t *testing.T) {
		t.Parallel()
		repo, p, _, _ := pollLedgerBed(t)
		now := day
		if _, err := repo.poll(context.Background(), pollBedConfig(repo, failingReceive{p}, now)); err == nil {
			t.Fatal("the receive error was not returned")
		}
		poll(t, repo, p, now.Add(2*time.Minute))
		if isDown, exists := down(t, repo); isDown || exists || len(p.posts) != 0 {
			t.Fatalf("down %t notices.json %t posts %q", isDown, exists, p.posts)
		}
	})
	t.Run("a poll without a failure writes no notices", func(t *testing.T) {
		t.Parallel()
		repo, p, _, _ := pollLedgerBed(t)
		poll(t, repo, p, day)
		if _, exists := down(t, repo); exists || len(p.posts) != 0 {
			t.Fatalf("notices.json %t posts %q", exists, p.posts)
		}
	})
	t.Run("an outage that ends with nothing open says nothing", func(t *testing.T) {
		t.Parallel()
		repo, p, q, now := contractBed(t)
		p.failPosts = 1
		poll(t, repo, p, now)
		closed, err := ReadQuestion(repo.root, q.ID)
		if err != nil {
			t.Fatal(err)
		}
		closed.State = "closed"
		if err := writeJSON(questionPath(repo.root, q.ID), closed); err != nil {
			t.Fatal(err)
		}
		poll(t, repo, p, now.Add(2*time.Minute))
		if isDown, _ := down(t, repo); isDown || len(p.posts) != 0 {
			t.Fatalf("down %t posts %q", isDown, p.posts)
		}
	})
}
