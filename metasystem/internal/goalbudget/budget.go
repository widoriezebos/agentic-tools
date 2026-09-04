// Package goalbudget owns the one complete goal budget tuple. The goal ledger
// re-exports this type; run records embed the same type rather than defining a
// second tuple family.
package goalbudget

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

const SetupRefusalReleaseRule = "setup-refusal-release"

// ReservationConsumesBudget applies the setup-refusal-release rule. A job
// that terminates before an agent starts consumes neither an attempt nor its
// reserved minutes; every other published reservation consumes both.
func ReservationConsumesBudget(terminal bool, phase, refusalClass string) bool {
	return !terminal || phase != "setup" || refusalClass != "setup"
}

func ParseWorkingDuration(value string) (time.Duration, bool) {
	if value == "" {
		return 0, false
	}
	var total time.Duration
	for i := 0; i < len(value); {
		start := i
		for i < len(value) && value[i] >= '0' && value[i] <= '9' {
			i++
		}
		if start == i || i >= len(value) {
			return 0, false
		}
		n, err := strconv.ParseInt(value[start:i], 10, 64)
		if err != nil || n <= 0 {
			return 0, false
		}
		var unit time.Duration
		switch value[i] {
		case 'm':
			unit = time.Minute
		case 'h':
			unit = time.Hour
		case 'd':
			unit = 8 * time.Hour
		default:
			return 0, false
		}
		if time.Duration(n) > (time.Duration(1<<63-1)-total)/unit {
			return 0, false
		}
		total += time.Duration(n) * unit
		i++
	}
	return total, total > 0
}

func FormatWorkingDuration(value time.Duration) string {
	minutes := int64(value / time.Minute)
	if minutes <= 0 {
		return ""
	}
	days := minutes / (8 * 60)
	minutes %= 8 * 60
	hours := minutes / 60
	minutes %= 60
	var b strings.Builder
	if days > 0 {
		fmt.Fprintf(&b, "%dd", days)
	}
	if hours > 0 {
		fmt.Fprintf(&b, "%dh", hours)
	}
	if minutes > 0 {
		fmt.Fprintf(&b, "%dm", minutes)
	}
	return b.String()
}

type Budget struct {
	ElapsedLimit            string `json:"elapsedLimit"`
	AttemptLimit            uint64 `json:"attemptLimit"`
	ReservedJobMinutesLimit uint64 `json:"reservedJobMinutesLimit"`
	ActiveJobLimit          uint64 `json:"activeJobLimit"`
	ReviewRoundLimit        int64  `json:"reviewRoundLimit"`
}

func New(elapsedLimit string, attemptLimit, reservedJobMinutesLimit, activeJobLimit, reviewRoundLimit int64) (Budget, error) {
	elapsed, ok := ParseWorkingDuration(elapsedLimit)
	if !ok {
		return Budget{}, fmt.Errorf("elapsedLimit %q is not a positive duration (for example 4h or 1d2h)", elapsedLimit)
	}
	if attemptLimit < 1 || reservedJobMinutesLimit < 1 || activeJobLimit < 1 {
		return Budget{}, fmt.Errorf("attemptLimit, reservedJobMinutesLimit, and activeJobLimit must be positive integers")
	}
	if reviewRoundLimit < 0 {
		return Budget{}, fmt.Errorf("reviewRoundLimit must be a non-negative integer")
	}
	return Budget{ElapsedLimit: FormatWorkingDuration(elapsed), AttemptLimit: uint64(attemptLimit),
		ReservedJobMinutesLimit: uint64(reservedJobMinutesLimit), ActiveJobLimit: uint64(activeJobLimit),
		ReviewRoundLimit: reviewRoundLimit}, nil
}

// Validate checks the tuple's intrinsic shape and, when supplied, the
// configured review-round ceiling. A zero ceiling deliberately means that
// the non-negative round member has no upper bound.
func (b Budget) Validate(reviewRoundMax ...uint64) error {
	if _, ok := ParseWorkingDuration(b.ElapsedLimit); !ok {
		return fmt.Errorf("elapsedLimit %q is not a positive duration", b.ElapsedLimit)
	}
	if b.AttemptLimit == 0 || b.ReservedJobMinutesLimit == 0 || b.ActiveJobLimit == 0 {
		return fmt.Errorf("attemptLimit, reservedJobMinutesLimit, and activeJobLimit must be positive integers")
	}
	if b.ReviewRoundLimit < 0 {
		return fmt.Errorf("reviewRoundLimit must be a non-negative integer")
	}
	if len(reviewRoundMax) > 1 {
		return fmt.Errorf("reviewRoundLimit validation accepts at most one configured ceiling")
	}
	if len(reviewRoundMax) == 1 && reviewRoundMax[0] > 0 && uint64(b.ReviewRoundLimit) > reviewRoundMax[0] {
		return fmt.Errorf("reviewRoundLimit %d exceeds configured maximum %d", b.ReviewRoundLimit, reviewRoundMax[0])
	}
	return nil
}

func (b Budget) ElapsedDuration() time.Duration {
	duration, _ := ParseWorkingDuration(b.ElapsedLimit)
	return duration
}

func (b Budget) ElapsedBreachDuration(gracePercent uint64) (time.Duration, error) {
	limit := b.ElapsedDuration()
	if limit <= 0 {
		return 0, fmt.Errorf("elapsedLimit %q is not a positive duration", b.ElapsedLimit)
	}
	limitNanos := uint64(limit)
	remaining := uint64(math.MaxInt64) - limitNanos
	wholeHundreds := limitNanos / 100
	if gracePercent > 0 && wholeHundreds > remaining/gracePercent {
		return 0, fmt.Errorf("elapsedLimit %q with grace percent %d exceeds the duration range", b.ElapsedLimit, gracePercent)
	}
	extra := wholeHundreds * gracePercent
	remainder := limitNanos % 100
	if gracePercent > 0 && remainder > math.MaxUint64/gracePercent {
		return 0, fmt.Errorf("elapsedLimit %q with grace percent %d exceeds the duration range", b.ElapsedLimit, gracePercent)
	}
	fraction := remainder * gracePercent / 100
	if fraction > remaining-extra {
		return 0, fmt.Errorf("elapsedLimit %q with grace percent %d exceeds the duration range", b.ElapsedLimit, gracePercent)
	}
	return time.Duration(limitNanos + extra + fraction), nil
}
