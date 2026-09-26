package channel

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// Targeted question operations, the tolerant open-question walk, and the
// question admission refusals. Each test names what a caller relies on: a
// retry never asks twice, a withdrawal never races a poll, one damaged record
// never hides the others, and an inadmissible question is never written.

// askUndelivered records one open question whose first post failed, so it
// has no thread and one counted undelivered attempt.
func askUndelivered(t *testing.T, root string) Question {
	t.Helper()
	q, err := Ask(AskRequest{RepoRoot: root, Goal: "g", Kind: "other", Machine: "m",
		Facts: []string{"fact"}, Provider: &testProvider{failPosts: 1}, Now: time.Unix(1, 0)})
	if err != nil {
		t.Fatalf("asking the question: %v", err)
	}
	if q.Thread != nil || q.Undelivered != 1 {
		t.Fatalf("the undelivered question = thread %v undelivered %d; want no thread and one miss", q.Thread, q.Undelivered)
	}
	return q
}

// A retry posts exactly the stored question once, counts a failed post
// durably, and once delivered never posts it again.
func TestRetryDeliveryPostsTheStoredQuestionOnlyUntilItIsDelivered(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	asked := askUndelivered(t, root)

	failing := &testProvider{failPosts: 1}
	q, outcome, err := RetryDelivery(context.Background(), root, asked.ID, failing, DestinationConfig{})
	if err != nil || outcome != RetryUndelivered || q.Thread != nil || q.Undelivered != 2 {
		t.Fatalf("a failed retry = %q thread %v undelivered %d err %v; want undelivered twice, no thread", outcome, q.Thread, q.Undelivered, err)
	}
	if stored, _ := ReadQuestion(root, asked.ID); stored.Undelivered != 2 {
		t.Fatalf("the failed retry was not counted durably: %+v", stored)
	}

	delivering := &testProvider{}
	q, outcome, err = RetryDelivery(context.Background(), root, asked.ID, delivering, DestinationConfig{})
	if err != nil || outcome != RetryDelivered || q.Thread == nil || q.Thread.ID != "1" {
		t.Fatalf("a delivering retry = %q thread %v err %v; want delivered in thread 1", outcome, q.Thread, err)
	}
	if len(delivering.posts) != 1 || delivering.posts[0] != renderQuestion(asked) || delivering.postThreads[0] != nil {
		t.Fatalf("the retry posted %q in %v; want the stored question once as a new thread", delivering.posts, delivering.postThreads)
	}
	if stored, _ := ReadQuestion(root, asked.ID); stored.Thread == nil || stored.Thread.ID != "1" {
		t.Fatalf("the delivered thread was not recorded: %+v", stored)
	}

	again := &testProvider{}
	_, outcome, err = RetryDelivery(context.Background(), root, asked.ID, again, DestinationConfig{})
	if err != nil || outcome != RetryAlreadyDelivered || len(again.posts) != 0 {
		t.Fatalf("a retry of a delivered question = %q posts %q err %v; want already-delivered and no post", outcome, again.posts, err)
	}
}

// A retry of a question that is closed, unknown, or that has no provider to
// post through changes nothing.
func TestRetryDeliveryRefusesWhatItCannotDeliver(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	asked := askUndelivered(t, root)

	if _, _, err := RetryDelivery(context.Background(), root, asked.ID, nil, DestinationConfig{}); err == nil {
		t.Fatalf("a retry with no provider was admitted")
	}
	if _, _, err := RetryDelivery(context.Background(), root, "NOSUCHQUESTION", &testProvider{}, DestinationConfig{}); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a retry of an unknown question = %v; want not-exist", err)
	}
	if err := Close(root, asked.ID, "superseded", nil, DestinationConfig{}); err != nil {
		t.Fatalf("closing the question: %v", err)
	}
	p := &testProvider{}
	q, outcome, err := RetryDelivery(context.Background(), root, asked.ID, p, DestinationConfig{})
	if err != nil || outcome != RetryNotOpen || q.State != "closed" || len(p.posts) != 0 {
		t.Fatalf("a retry of a closed question = %q state %q posts %q err %v; want not-open and no post", outcome, q.State, p.posts, err)
	}
	if stored, _ := ReadQuestion(root, asked.ID); stored.Undelivered != 1 {
		t.Fatalf("a refused retry changed the question: %+v", stored)
	}
}

// While a poll holds the channel lock, a targeted retry or withdrawal is
// refused as busy and leaves the question exactly as it was. The lock is held
// by this test through the same owner the poll uses, so nothing waits on time.
func TestTargetedOperationsAreBusyWhilePollHoldsTheLock(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	asked := askUndelivered(t, root)
	p := &testProvider{}

	err := withPollLock(root, func() error {
		if _, _, err := RetryDelivery(context.Background(), root, asked.ID, p, DestinationConfig{}); !errors.Is(err, ErrChannelBusy) {
			return fmt.Errorf("a retry during a poll = %v; want busy", err)
		}
		if _, err := Withdraw(root, asked.ID, "no longer needed", p, DestinationConfig{}); !errors.Is(err, ErrChannelBusy) {
			return fmt.Errorf("a withdrawal during a poll = %v; want busy", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	stored, err := ReadQuestion(root, asked.ID)
	if err != nil || stored.State != "open" || stored.Undelivered != 1 || len(p.posts) != 0 {
		t.Fatalf("a busy refusal changed the question: %+v posts %q err %v", stored, p.posts, err)
	}
}

// A withdrawal closes the question and its answer, tells its thread why, and
// answers the closed record.
func TestWithdrawClosesTheQuestionAndSaysWhyInItsThread(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	p := &testProvider{}
	asked, err := Ask(AskRequest{RepoRoot: root, Goal: "g", Kind: "other", Machine: "m",
		Facts: []string{"fact"}, Provider: p, Now: time.Unix(1, 0)})
	if err != nil || asked.Thread == nil {
		t.Fatalf("asking a delivered question: %+v %v", asked, err)
	}
	stored, _ := ReadQuestion(root, asked.ID)
	stored.Answer = &Answer{Phase: "matched"}
	if err := writeJSON(questionPath(root, asked.ID), stored); err != nil {
		t.Fatalf("recording an answer in flight: %v", err)
	}

	q, err := Withdraw(root, asked.ID, "the goal was parked", p, DestinationConfig{})

	if err != nil || q.State != "closed" || q.Answer == nil || q.Answer.Phase != "closed" {
		t.Fatalf("the withdrawn question = %+v err %v; want closed with a closed answer", q, err)
	}
	if len(p.posts) != 2 || p.posts[1] != "closed: the goal was parked" || p.postThreads[1] == nil || p.postThreads[1].ID != asked.Thread.ID {
		t.Fatalf("the withdrawal posted %q in %v; want the reason in the question's thread", p.posts, p.postThreads)
	}
	if _, err := Withdraw(root, "NOSUCHQUESTION", "gone", p, DestinationConfig{}); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("withdrawing an unknown question = %v; want not-exist", err)
	}
}

// A channel whose directory cannot be made refuses targeted operations with
// that error rather than acting unlocked.
func TestTargetedOperationsRefuseAChannelThatCannotBeLocked(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "artifacts", "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(channelRoot(root), []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	ran := false
	err := withPollLock(root, func() error { ran = true; return nil })
	if err == nil || errors.Is(err, ErrChannelBusy) || ran {
		t.Fatalf("locking an unmakeable channel = %v ran %v; want the filesystem error and no operation", err, ran)
	}
}

// The open-question walk returns every readable open question newest first
// and names each unreadable or incomplete record on its own channel, so one
// damaged file never hides the rest.
func TestWalkOpenQuestionsNamesDamagedRecordsWithoutHidingOthers(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	older := Question{ID: "OLDER", Goal: "g", Kind: "other", State: "open", OpenedAt: time.Unix(10, 0).UTC()}
	newer := Question{ID: "NEWER", Goal: "g", Kind: "other", State: "open", OpenedAt: time.Unix(20, 0).UTC()}
	closed := Question{ID: "CLOSED", Goal: "g", Kind: "other", State: "closed", OpenedAt: time.Unix(30, 0).UTC()}
	incomplete := Question{ID: "INCOMPLETE", Goal: "g", Kind: "other", State: "open"}
	for _, q := range []Question{older, newer, closed, incomplete} {
		if err := writeJSON(questionPath(root, q.ID), q); err != nil {
			t.Fatal(err)
		}
	}
	if err := writeDurable(questionPath(root, "DAMAGED"), []byte("{not json")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(questionPath(root, "ADIRECTORY"), 0o755); err != nil {
		t.Fatal(err)
	}

	open, unreadable := WalkOpenQuestions(root)

	if len(open) != 2 || open[0].ID != "NEWER" || open[1].ID != "OLDER" {
		t.Fatalf("the open questions = %+v; want NEWER then OLDER", open)
	}
	if len(unreadable) != 3 {
		t.Fatalf("the unreadable records = %q; want the damaged, incomplete and unreadable ones", unreadable)
	}
	named := strings.Join(unreadable, "\n")
	for _, want := range []string{"ADIRECTORY.json", "DAMAGED.json", "INCOMPLETE.json: missing required question fields"} {
		if !strings.Contains(named, want) {
			t.Fatalf("the unreadable records %q do not name %q", unreadable, want)
		}
	}
	if open, unreadable := WalkOpenQuestions(t.TempDir()); len(open) != 0 || len(unreadable) != 0 {
		t.Fatalf("a channel with no questions walked %+v %q; want nothing", open, unreadable)
	}
}

// A question that its kind cannot carry is refused before anything is written
// or posted: a missing fact, an unknown kind, a budget on a kind without one,
// a carry without its exact token or with a budget, and a budget question
// without a valid tuple.
func TestAskRefusesInadmissibleQuestionsBeforeWritingOrPosting(t *testing.T) {
	t.Parallel()

	for name, request := range map[string]AskRequest{
		"no fact":                        {Goal: "g", Kind: "other"},
		"no goal":                        {Kind: "other", Facts: []string{"fact"}},
		"an unknown kind":                {Goal: "g", Kind: "whim", Facts: []string{"fact"}},
		"a budget on an ordinary kind":   {Goal: "g", Kind: "stop", Facts: []string{"fact"}, Budget: &goal.Budget{}},
		"a carry with a budget":          {Goal: "g", Kind: "carry", Facts: []string{"fact"}, Budget: &goal.Budget{}},
		"a carry without its token":      {Goal: "g", Kind: "carry", Facts: []string{"fact"}, Wants: "carry it please"},
		"a budget question with none":    {Goal: "g", Kind: "budget-above-norm", Facts: []string{"fact"}},
		"a budget question with a blank": {Goal: "g", Kind: "budget-above-norm", Facts: []string{"fact"}, Budget: &goal.Budget{}},
	} {
		root := t.TempDir()
		p := &testProvider{}
		request.RepoRoot, request.Provider, request.Now = root, p, time.Unix(1, 0)
		if q, err := Ask(request); err == nil {
			t.Errorf("%s was asked as %+v; want a refusal", name, q)
			continue
		}
		if matches, _ := filepath.Glob(filepath.Join(channelRoot(root), "questions", "*.json")); len(matches) != 0 || len(p.posts) != 0 {
			t.Errorf("%s was refused but left records %v and posts %q", name, matches, p.posts)
		}
	}
}

// Every provider failure kind is recognized through wrapping by its own kind
// and by no other, which is how callers decide between retrying, reporting
// busy, and asking for configuration.
func TestProviderErrorKindsAreRecognizedThroughWrapping(t *testing.T) {
	t.Parallel()

	made := map[ErrorKind]error{
		Unconfigured:  ErrUnconfigured("no token"),
		SendFailed:    ErrSendFailed("503"),
		ReceiveFailed: ErrReceiveFailed("timeout"),
		Busy:          ErrBusy("rate limited"),
	}
	for kind, err := range made {
		wrapped := fmt.Errorf("posting the status: %w", err)
		if err.Error() != string(kind)+": "+err.(*ProviderError).Problem {
			t.Errorf("the %s error reads %q", kind, err.Error())
		}
		for other := range made {
			if got := IsKind(wrapped, other); got != (other == kind) {
				t.Errorf("IsKind(%s error, %s) = %v", kind, other, got)
			}
		}
	}
	if IsKind(errors.New("plain"), Busy) {
		t.Fatalf("a plain error was recognized as a provider error")
	}
}
