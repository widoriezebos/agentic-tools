package config

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	// SeatPresenceStaleMinutesKey is the reader's own window on a seat's
	// presence. The threshold a standing is judged against is this window or
	// three of the writer's own ticks, whichever is longer.
	SeatPresenceStaleMinutesKey     = "seat.presence-stale-min"
	DefaultSeatPresenceStaleMinutes = uint64(30)

	// SeatPresenceNamespaceKey pins the presence ladder to one namespace for
	// an operator who knows the host. Unset, the publisher climbs the ladder
	// by itself, starting at the metasystem ref.
	SeatPresenceNamespaceKey = "seat.presence-namespace"

	seatMetasystemNamespace = "refs/metasystem/presence"
	seatBranchNamespace     = "refs/heads/presence"
)

// SeatPresenceStaleMinutes resolves the presence stale window. Like the other
// steward thresholds it is operational law, so production reads accept only
// the committed repository value.
func SeatPresenceStaleMinutes(confPath string) (uint64, error) {
	value, err := budgetLawValue(confPath, SeatPresenceStaleMinutesKey,
		strconv.FormatUint(DefaultSeatPresenceStaleMinutes, 10))
	if err != nil {
		return 0, fmt.Errorf("resolve %s: %w", SeatPresenceStaleMinutesKey, err)
	}
	return parseSeatPresenceStaleMinutes(value)
}

func parseSeatPresenceStaleMinutes(value string) (uint64, error) {
	if !digitsOnlyValue.MatchString(value) {
		return 0, fmt.Errorf("%s must be a positive integer, got %q", SeatPresenceStaleMinutesKey, value)
	}
	minutes, err := strconv.ParseUint(value, 10, 64)
	if err != nil || minutes == 0 {
		return 0, fmt.Errorf("%s must be a positive integer, got %q", SeatPresenceStaleMinutesKey, value)
	}
	return minutes, nil
}

// SeatPresenceNamespace resolves the pinned presence namespace. The empty
// string is the ordinary answer: no pin, and the publisher finds its own rung.
func SeatPresenceNamespace(confPath string) (string, error) {
	value, err := budgetLawValue(confPath, SeatPresenceNamespaceKey, "")
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", SeatPresenceNamespaceKey, err)
	}
	return parseSeatPresenceNamespace(value)
}

func parseSeatPresenceNamespace(value string) (string, error) {
	trimmed := strings.TrimSuffix(strings.TrimSpace(value), "/")
	switch trimmed {
	case "", seatMetasystemNamespace, seatBranchNamespace:
		return trimmed, nil
	}
	return "", fmt.Errorf("%s must be %s or %s, got %q",
		SeatPresenceNamespaceKey, seatMetasystemNamespace, seatBranchNamespace, value)
}
