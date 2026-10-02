package channel

import (
	"context"
	"testing"
	"time"
)

var landedNow = time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)

const landedSHA = "0123456789abcdef0123456789abcdef01234567"

// Decision 7: a landing on main posts one line naming its goals and short
// sha, once per sha: a repeat with the same sha posts nothing.
func TestPostLandedOncePerSHA(t *testing.T) {
	repo := t.TempDir()
	p := &testProvider{}
	for range 2 {
		if err := PostLanded(context.Background(), repo, p, DestinationConfig{}, []string{"goal-a", "goal-b"}, landedSHA, landedNow); err != nil {
			t.Fatal(err)
		}
	}
	if len(p.posts) != 1 || p.posts[0] != "landed: goal-a, goal-b at 0123456789ab" {
		t.Fatalf("posts = %q; want one landing line", p.posts)
	}
}

// A landing with no named goal still says what landed.
func TestLandedTextWithoutGoal(t *testing.T) {
	if got := LandedText(nil, landedSHA); got != "landed: main at 0123456789ab" {
		t.Fatalf("text = %q", got)
	}
}

// A failed post is recorded, not lost: the next landing retries it once
// before its own line, and a retry that fails again is not tried a third
// time.
func TestPostLandedFailureIsRetriedOnce(t *testing.T) {
	repo := t.TempDir()
	calls := 0
	p := &testProvider{failPosts: 1, beforePost: func() { calls++ }}
	if err := PostLanded(context.Background(), repo, p, DestinationConfig{}, []string{"goal-a"}, landedSHA, landedNow); err == nil {
		t.Fatal("a failed post reported no error")
	}
	if pending := LoadLandedState(repo).Pending; len(pending) != 1 || pending[0].SHA != landedSHA {
		t.Fatalf("pending = %+v; want the failed landing recorded", pending)
	}
	second := "fedcba9876543210fedcba9876543210fedcba98"
	if err := PostLanded(context.Background(), repo, p, DestinationConfig{}, []string{"goal-b"}, second, landedNow); err != nil {
		t.Fatal(err)
	}
	if len(p.posts) != 2 || p.posts[0] != "landed: goal-a at 0123456789ab" || p.posts[1] != "landed: goal-b at fedcba987654" {
		t.Fatalf("posts = %q; want the retried line, then the new one", p.posts)
	}
	if err := RetryLanded(context.Background(), repo, p, DestinationConfig{}); err != nil || calls != 3 {
		t.Fatalf("a retry with nothing pending = %v after %d calls", err, calls)
	}

	repo = t.TempDir()
	calls = 0
	p = &testProvider{failPosts: 2, beforePost: func() { calls++ }}
	if err := PostLanded(context.Background(), repo, p, DestinationConfig{}, []string{"goal-a"}, landedSHA, landedNow); err == nil {
		t.Fatal("a failed post reported no error")
	}
	if err := RetryLanded(context.Background(), repo, p, DestinationConfig{}); err == nil {
		t.Fatal("a failed retry reported no error")
	}
	if err := RetryLanded(context.Background(), repo, p, DestinationConfig{}); err != nil {
		t.Fatal(err)
	}
	if err := PostLanded(context.Background(), repo, p, DestinationConfig{}, []string{"goal-a"}, landedSHA, landedNow); err != nil {
		t.Fatal(err)
	}
	if calls != 2 || len(p.posts) != 0 {
		t.Fatalf("%d post calls, posts %q; want the first try and one retry, then nothing", calls, p.posts)
	}
	if state := LoadLandedState(repo); len(state.Pending) != 0 || len(state.Failed) != 1 {
		t.Fatalf("state = %+v; want the landing given up after its one retry", state)
	}
}
