package config

import (
	"fmt"
	"strconv"
)

const LedgerAttentionStaleMinutesKey = "steward.ledger-attention-stale-minutes"

// DefaultLedgerAttentionStaleMinutes is the compiled default (defaults.go).
var DefaultLedgerAttentionStaleMinutes = uintDefault(LedgerAttentionStaleMinutesKey)

// LedgerAttentionStaleMinutes resolves the maximum age of a shared-ledger
// movement that this machine has not examined. The threshold is operational
// law, so production reads accept only the committed repository value.
func LedgerAttentionStaleMinutes(confPath string) (uint64, error) {
	value, err := budgetLawValue(confPath, LedgerAttentionStaleMinutesKey, MustDefault(LedgerAttentionStaleMinutesKey))
	if err != nil {
		return 0, fmt.Errorf("resolve %s: %w", LedgerAttentionStaleMinutesKey, err)
	}
	return parseLedgerAttentionStaleMinutes(value)
}

func parseLedgerAttentionStaleMinutes(value string) (uint64, error) {
	if !digitsOnlyValue.MatchString(value) {
		return 0, fmt.Errorf("%s must be a positive integer, got %q", LedgerAttentionStaleMinutesKey, value)
	}
	minutes, err := strconv.ParseUint(value, 10, 64)
	if err != nil || minutes == 0 {
		return 0, fmt.Errorf("%s must be a positive integer, got %q", LedgerAttentionStaleMinutesKey, value)
	}
	return minutes, nil
}
