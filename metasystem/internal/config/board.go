package config

import (
	"fmt"
	"strconv"
	"time"
)

// The host board's settings (batch-lane design D14-r2, R25; the mailbox's
// keep-days D14-r3, R26; the handover's claim-lock wait, R26's N-2). The
// steward's bridge role reads the first three and a goal handover the
// fourth; board.Write, board.Read and the mailbox never do, they take them
// as values.
const (
	BoardKeepHoursKey           = "board.keep-hours"
	BoardPollSecKey             = "board.poll-sec"
	BoardMailboxKeepDaysKey     = "board.mailbox-keep-days"
	BoardHandoverLockWaitSecKey = "board.handover-lock-wait-sec"
)

// Board is how long a terminal card stays on the board before the bridge
// sweeps it, and the bridge's bounded poll where no kernel watch can be
// established, and how long a closed peer-message thread whose every
// message was offered stays in its mailbox after it closed, and how long a
// handover waits for the offers holding its goal's claim lock.
type Board struct {
	Keep             time.Duration
	Poll             time.Duration
	MailboxKeep      time.Duration
	HandoverLockWait time.Duration
}

// DefaultBoard is the compiled board settings.
func DefaultBoard() Board {
	return Board{Keep: time.Duration(intDefault(BoardKeepHoursKey)) * time.Hour, Poll: time.Duration(intDefault(BoardPollSecKey)) * time.Second,
		MailboxKeep:      time.Duration(intDefault(BoardMailboxKeepDaysKey)) * 24 * time.Hour,
		HandoverLockWait: time.Duration(intDefault(BoardHandoverLockWaitSecKey)) * time.Second}
}

// positiveBoardSetting reads one board key through the one resolver.
func positiveBoardSetting(confPath, key string) (int, error) {
	value, _, err := Get(GetParams{Key: key, ConfPath: confPath})
	if err != nil {
		return 0, fmt.Errorf("resolve %s: %w", key, err)
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return 0, fmt.Errorf("%s must be a positive integer, got %q", key, value)
	}
	return parsed, nil
}

// ResolveHandoverLockWait reads the handover's claim-lock wait alone, so a
// handover is never refused over another board key.
func ResolveHandoverLockWait(confPath string) (time.Duration, error) {
	seconds, err := positiveBoardSetting(confPath, BoardHandoverLockWaitSecKey)
	return time.Duration(seconds) * time.Second, err
}

// ResolveBoard reads the board settings through the one resolver.
func ResolveBoard(confPath string) (Board, error) {
	read := func(key string) (int, error) { return positiveBoardSetting(confPath, key) }
	keep, err := read(BoardKeepHoursKey)
	if err != nil {
		return Board{}, err
	}
	poll, err := read(BoardPollSecKey)
	if err != nil {
		return Board{}, err
	}
	mailboxKeep, err := read(BoardMailboxKeepDaysKey)
	if err != nil {
		return Board{}, err
	}
	handoverWait, err := ResolveHandoverLockWait(confPath)
	if err != nil {
		return Board{}, err
	}
	return Board{Keep: time.Duration(keep) * time.Hour, Poll: time.Duration(poll) * time.Second, MailboxKeep: time.Duration(mailboxKeep) * 24 * time.Hour,
		HandoverLockWait: handoverWait}, nil
}
