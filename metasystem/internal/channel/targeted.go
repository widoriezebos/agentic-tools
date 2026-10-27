package channel

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
)

// ErrChannelBusy is returned when a poll or another targeted operation
// holds the channel lock; nothing was changed.
var ErrChannelBusy = errors.New("the channel is busy with another poll; nothing was changed")

// withPollLock runs one question operation under the lock Poll holds, so a
// targeted retry or withdrawal never interleaves with a poll's delivery or
// answer handling of the same question.
func withPollLock(repo string, operation func() error) error {
	if err := os.MkdirAll(channelRoot(repo), 0o755); err != nil {
		return err
	}
	held, err := lock.File(filepath.Join(channelRoot(repo), "lock"), 0o644, lock.TryExclusive)
	if err != nil {
		if lock.Busy(err) {
			return ErrChannelBusy
		}
		return err
	}
	defer held.Release()
	return operation()
}

// RetryOutcome is what one targeted delivery retry did.
type RetryOutcome string

const (
	RetryDelivered        RetryOutcome = "delivered"
	RetryUndelivered      RetryOutcome = "undelivered"
	RetryAlreadyDelivered RetryOutcome = "already-delivered"
	RetryNotOpen          RetryOutcome = "not-open"
)

// RetryDelivery posts one open question that has no thread yet, and only
// that question, under the poll lock. It never asks a new question: the
// stored text is posted as Poll posts it, and its answer handling stays
// with Poll.
func RetryDelivery(ctx context.Context, repo, id string, p Provider, d DestinationConfig) (Question, RetryOutcome, error) {
	var q Question
	var outcome RetryOutcome
	err := withPollLock(repo, func() error {
		var err error
		if q, err = ReadQuestion(repo, id); err != nil {
			return err
		}
		switch {
		case q.State != "open":
			outcome = RetryNotOpen
			return nil
		case q.Thread != nil:
			outcome = RetryAlreadyDelivered
			return nil
		case p == nil:
			return errors.New("no channel provider is configured")
		}
		if err = ensureText(repo, &q); err != nil {
			return err
		}
		ref, postErr := postQuestion(ctx, p, d, q, answerCodeOffOrOn(repo))
		if postErr != nil {
			q.Undelivered++
			outcome = RetryUndelivered
		} else {
			q.Thread = &ref
			outcome = RetryDelivered
		}
		return writeJSON(questionPath(repo, q.ID), q)
	})
	return q, outcome, err
}

// Withdraw is Close under the poll lock, so a withdrawal and a concurrent
// poll never both write the question. The question keeps the caller's
// clock, when given, as its closing time.
func Withdraw(repo, id, because string, p Provider, d DestinationConfig, at ...time.Time) (Question, error) {
	var q Question
	err := withPollLock(repo, func() error {
		if err := closeAt(repo, id, because, p, d, append(at, time.Now())[0]); err != nil {
			return err
		}
		var err error
		q, err = ReadQuestion(repo, id)
		return err
	})
	return q, err
}
