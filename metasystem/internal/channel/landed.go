package channel

// A landing on main is the one piece of news the channel carries without
// asking for a response (Decision 7 of the blocked-agent-asks-the-human
// design). Its message is the plain sentence of what was delivered, written
// by the agent that did the work, posted verbatim once per sha; a landing
// with no sentence posts nothing (Wido, 2026-10-02: "a meaningful message
// about what was delivered"). A post that fails is kept and retried once,
// by the next landing or the next tick; it never fails the landing itself.

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
)

// LandedNotice is one landing's line.
type LandedNotice struct {
	SHA string `json:"sha"`
	// Text is the message: the plain sentences of what landed.
	Text string    `json:"text"`
	At   time.Time `json:"at"`
	// Error is why its last post failed.
	Error string `json:"error,omitempty"`
}

// LandedState is landed.json: the shas already posted, the failed posts
// waiting for their one retry, and those whose retry failed too.
type LandedState struct {
	Posted  []string       `json:"posted"`
	Pending []LandedNotice `json:"pending,omitempty"`
	Failed  []LandedNotice `json:"failed,omitempty"`
}

func landedPath(repo string) string { return filepath.Join(channelRoot(repo), "landed.json") }

// LoadLandedState reads landed.json; a missing or unreadable file is empty.
func LoadLandedState(repo string) LandedState {
	var s LandedState
	if b, err := os.ReadFile(landedPath(repo)); err == nil {
		_ = json.Unmarshal(b, &s)
	}
	return s
}

// PostLanded posts a landing's message, the plain sentences of what it
// delivered, unless its sha was posted, is waiting for its retry or was
// given up; empty text posts and records nothing. A pending earlier
// landing is retried first. A failed post is recorded for one retry and
// returned.
func PostLanded(ctx context.Context, repo string, p Provider, d DestinationConfig, text, sha string, now time.Time) error {
	text = strings.TrimSpace(text)
	if text == "" || sha == "" {
		return RetryLanded(ctx, repo, p, d)
	}
	return withLandedState(repo, func(s *LandedState) error {
		retryErr := retryPending(ctx, s, p, d)
		if s.known(sha) {
			return retryErr
		}
		notice := LandedNotice{SHA: sha, Text: text, At: now.UTC()}
		if _, err := p.Post(ctx, d, text, nil); err != nil {
			notice.Error = err.Error()
			s.Pending = append(s.Pending, notice)
			return errors.Join(retryErr, err)
		}
		s.Posted = append(s.Posted, sha)
		return retryErr
	})
}

// RetryLanded retries each pending landing's post once.
func RetryLanded(ctx context.Context, repo string, p Provider, d DestinationConfig) error {
	if len(LoadLandedState(repo).Pending) == 0 {
		return nil
	}
	return withLandedState(repo, func(s *LandedState) error { return retryPending(ctx, s, p, d) })
}

// retryPending posts each pending line once more: posted, or given up.
func retryPending(ctx context.Context, s *LandedState, p Provider, d DestinationConfig) error {
	var problems []error
	for _, notice := range s.Pending {
		if _, err := p.Post(ctx, d, notice.Text, nil); err != nil {
			notice.Error = err.Error()
			s.Failed = append(s.Failed, notice)
			problems = append(problems, err)
			continue
		}
		s.Posted = append(s.Posted, notice.SHA)
	}
	s.Pending = nil
	return errors.Join(problems...)
}

func (s LandedState) known(sha string) bool {
	if slices.Contains(s.Posted, sha) {
		return true
	}
	for _, notice := range append(slices.Clone(s.Pending), s.Failed...) {
		if notice.SHA == sha {
			return true
		}
	}
	return false
}

// withLandedState runs fn over landed.json under its own lock and saves
// what fn left, even when fn reports a failed post.
func withLandedState(repo string, fn func(*LandedState) error) error {
	if err := os.MkdirAll(channelRoot(repo), 0o755); err != nil {
		return err
	}
	held, err := lock.File(filepath.Join(channelRoot(repo), "landed.lock"), 0o644, lock.Exclusive)
	if err != nil {
		return err
	}
	defer held.Release()
	s := LoadLandedState(repo)
	err = fn(&s)
	return errors.Join(err, writeJSON(landedPath(repo), s))
}
