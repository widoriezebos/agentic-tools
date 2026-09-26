package channel

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
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
	lock, err := os.OpenFile(filepath.Join(channelRoot(repo), "lock"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := unix.Flock(int(lock.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		if err == unix.EWOULDBLOCK {
			return ErrChannelBusy
		}
		return err
	}
	defer unix.Flock(int(lock.Fd()), unix.LOCK_UN)
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
// stored question is rendered as Poll renders it, and its answer handling
// stays with Poll.
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
		ref, postErr := p.Post(ctx, d, renderQuestion(q), nil)
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
// poll never both write the question.
func Withdraw(repo, id, because string, p Provider, d DestinationConfig) (Question, error) {
	var q Question
	err := withPollLock(repo, func() error {
		if err := Close(repo, id, because, p, d); err != nil {
			return err
		}
		var err error
		q, err = ReadQuestion(repo, id)
		return err
	})
	return q, err
}
