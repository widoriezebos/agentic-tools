package config

import (
	"fmt"
	"strconv"
	"time"
)

// The host board's two settings (batch-lane design D14-r2, R25). The
// steward's bridge role reads them; board.Write and board.Read never do.
const (
	BoardKeepHoursKey = "board.keep-hours"
	BoardPollSecKey   = "board.poll-sec"
)

// Board is how long a terminal card stays on the board before the bridge
// sweeps it, and the bridge's bounded poll where no kernel watch can be
// established.
type Board struct {
	Keep time.Duration
	Poll time.Duration
}

// DefaultBoard is the compiled board settings.
func DefaultBoard() Board {
	return Board{Keep: time.Duration(intDefault(BoardKeepHoursKey)) * time.Hour, Poll: time.Duration(intDefault(BoardPollSecKey)) * time.Second}
}

// ResolveBoard reads the board settings through the one resolver.
func ResolveBoard(confPath string) (Board, error) {
	read := func(key string) (int, error) {
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
	keep, err := read(BoardKeepHoursKey)
	if err != nil {
		return Board{}, err
	}
	poll, err := read(BoardPollSecKey)
	if err != nil {
		return Board{}, err
	}
	return Board{Keep: time.Duration(keep) * time.Hour, Poll: time.Duration(poll) * time.Second}, nil
}
