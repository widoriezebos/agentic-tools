package channel

import (
	"context"
	"testing"
	"time"
)

var landedNow = time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)

const (
	landedSHA      = "0123456789abcdef0123456789abcdef01234567"
	landedSentence = "Seats now ask you on Telegram when they are stuck."
)

// Decision 7, in Wido's words: the message is the plain sentence of what
// was delivered, verbatim and once per landing; a repeat with the same sha
// posts nothing.
func TestPostLandedPostsTheSentenceOncePerSHA(t *testing.T) {
	repo := t.TempDir()
	p := &testProvider{}
	for range 2 {
		if err := PostLanded(context.Background(), repo, p, DestinationConfig{}, landedSentence, landedSHA, landedNow); err != nil {
			t.Fatal(err)
		}
	}
	if len(p.posts) != 1 || p.posts[0] != landedSentence {
		t.Fatalf("posts = %q; want the sentence once, verbatim", p.posts)
	}
}

// A landing with no sentence posts nothing and records nothing.
func TestPostLandedWithoutSentencePostsNothing(t *testing.T) {
	repo := t.TempDir()
	p := &testProvider{}
	if err := PostLanded(context.Background(), repo, p, DestinationConfig{}, " ", landedSHA, landedNow); err != nil {
		t.Fatal(err)
	}
	if len(p.posts) != 0 || len(LoadLandedState(repo).Posted) != 0 {
		t.Fatalf("posts = %q state = %+v; want nothing", p.posts, LoadLandedState(repo))
	}
}

// A failed post is recorded, not lost: the next landing retries it once
// before its own message, and a retry that fails again is not tried a
// third time.
func TestPostLandedFailureIsRetriedOnce(t *testing.T) {
	repo := t.TempDir()
	calls := 0
	p := &testProvider{failPosts: 1, beforePost: func() { calls++ }}
	if err := PostLanded(context.Background(), repo, p, DestinationConfig{}, landedSentence, landedSHA, landedNow); err == nil {
		t.Fatal("a failed post reported no error")
	}
	if pending := LoadLandedState(repo).Pending; len(pending) != 1 || pending[0].SHA != landedSHA || pending[0].Text != landedSentence {
		t.Fatalf("pending = %+v; want the failed landing recorded", pending)
	}
	second, secondSentence := "fedcba9876543210fedcba9876543210fedcba98", "The channel stays quiet unless something landed."
	if err := PostLanded(context.Background(), repo, p, DestinationConfig{}, secondSentence, second, landedNow); err != nil {
		t.Fatal(err)
	}
	if len(p.posts) != 2 || p.posts[0] != landedSentence || p.posts[1] != secondSentence {
		t.Fatalf("posts = %q; want the retried message, then the new one", p.posts)
	}
	if err := RetryLanded(context.Background(), repo, p, DestinationConfig{}); err != nil || calls != 3 {
		t.Fatalf("a retry with nothing pending = %v after %d calls", err, calls)
	}

	repo = t.TempDir()
	calls = 0
	p = &testProvider{failPosts: 2, beforePost: func() { calls++ }}
	if err := PostLanded(context.Background(), repo, p, DestinationConfig{}, landedSentence, landedSHA, landedNow); err == nil {
		t.Fatal("a failed post reported no error")
	}
	if err := RetryLanded(context.Background(), repo, p, DestinationConfig{}); err == nil {
		t.Fatal("a failed retry reported no error")
	}
	if err := RetryLanded(context.Background(), repo, p, DestinationConfig{}); err != nil {
		t.Fatal(err)
	}
	if err := PostLanded(context.Background(), repo, p, DestinationConfig{}, landedSentence, landedSHA, landedNow); err != nil {
		t.Fatal(err)
	}
	if calls != 2 || len(p.posts) != 0 {
		t.Fatalf("%d post calls, posts %q; want the first try and one retry, then nothing", calls, p.posts)
	}
	if state := LoadLandedState(repo); len(state.Pending) != 0 || len(state.Failed) != 1 {
		t.Fatalf("state = %+v; want the landing given up after its one retry", state)
	}
}
