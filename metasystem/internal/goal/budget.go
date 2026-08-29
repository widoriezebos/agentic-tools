package goal

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// ParseWorkingDuration accepts one or more positive integer day, hour, and
// minute segments. A working day is eight hours; seconds and fractions are not
// part of the budget grammar.
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

// FormatWorkingDuration is the canonical elapsed-limit token. It preserves
// the working-day convention and never emits seconds.
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

// Budget is the complete limit tuple carried by one goal revision. Spending
// is projected from job records; the goal never stores counters beside these
// limits.
type Budget struct {
	ElapsedLimit            string `json:"elapsedLimit"`
	AttemptLimit            uint64 `json:"attemptLimit"`
	ReservedJobMinutesLimit uint64 `json:"reservedJobMinutesLimit"`
	ActiveJobLimit          uint64 `json:"activeJobLimit"`
}

// NewBudget validates and canonicalizes the complete tuple accepted by the
// command surface. There is no partial form and no numeric default.
func NewBudget(elapsedLimit string, attemptLimit, reservedJobMinutesLimit, activeJobLimit int64) (Budget, error) {
	elapsed, ok := ParseWorkingDuration(elapsedLimit)
	if !ok {
		return Budget{}, fmt.Errorf("elapsedLimit %q is not a positive duration (for example 4h or 1d2h)", elapsedLimit)
	}
	if attemptLimit < 1 {
		return Budget{}, fmt.Errorf("attemptLimit must be a positive integer")
	}
	if reservedJobMinutesLimit < 1 {
		return Budget{}, fmt.Errorf("reservedJobMinutesLimit must be a positive integer")
	}
	if activeJobLimit < 1 {
		return Budget{}, fmt.Errorf("activeJobLimit must be a positive integer")
	}
	return Budget{
		ElapsedLimit:            FormatWorkingDuration(elapsed),
		AttemptLimit:            uint64(attemptLimit),
		ReservedJobMinutesLimit: uint64(reservedJobMinutesLimit),
		ActiveJobLimit:          uint64(activeJobLimit),
	}, nil
}

// Validate checks a tuple read from a goal record without changing its
// spelling. Render/parse therefore stays a fixed point for lawful records.
func (b Budget) Validate() error {
	if _, ok := ParseWorkingDuration(b.ElapsedLimit); !ok {
		return fmt.Errorf("elapsedLimit %q is not a positive duration", b.ElapsedLimit)
	}
	if b.AttemptLimit == 0 {
		return fmt.Errorf("attemptLimit must be a positive integer")
	}
	if b.ReservedJobMinutesLimit == 0 {
		return fmt.Errorf("reservedJobMinutesLimit must be a positive integer")
	}
	if b.ActiveJobLimit == 0 {
		return fmt.Errorf("activeJobLimit must be a positive integer")
	}
	return nil
}

// ElapsedDuration returns the validated elapsed limit as a duration.
func (b Budget) ElapsedDuration() time.Duration {
	duration, _ := ParseWorkingDuration(b.ElapsedLimit)
	return duration
}

// ElapsedBreachDuration returns the stop boundary after applying the configured
// grace percentage. Admission still closes at ElapsedDuration; this later
// boundary is only for breach-stop.
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

func parseBudgetRecord(value string) (Budget, error) {
	record, err := parseKVRecord(value,
		[]string{"elapsedLimit", "attemptLimit", "reservedJobMinutesLimit", "activeJobLimit"}, nil, "")
	if err != nil {
		return Budget{}, err
	}
	positive := func(key string) (uint64, error) {
		number, parseErr := strconv.ParseUint(record[key], 10, 64)
		if parseErr != nil || number == 0 {
			return 0, fmt.Errorf("%s=%q is not a positive integer", key, record[key])
		}
		return number, nil
	}
	attempts, err := positive("attemptLimit")
	if err != nil {
		return Budget{}, err
	}
	reserved, err := positive("reservedJobMinutesLimit")
	if err != nil {
		return Budget{}, err
	}
	active, err := positive("activeJobLimit")
	if err != nil {
		return Budget{}, err
	}
	budget := Budget{
		ElapsedLimit:            record["elapsedLimit"],
		AttemptLimit:            attempts,
		ReservedJobMinutesLimit: reserved,
		ActiveJobLimit:          active,
	}
	if err := budget.Validate(); err != nil {
		return Budget{}, err
	}
	return budget, nil
}

func budgetIntentArgs(b Budget) map[string]string {
	return map[string]string{
		"elapsedLimit":            b.ElapsedLimit,
		"attemptLimit":            strconv.FormatUint(b.AttemptLimit, 10),
		"reservedJobMinutesLimit": strconv.FormatUint(b.ReservedJobMinutesLimit, 10),
		"activeJobLimit":          strconv.FormatUint(b.ActiveJobLimit, 10),
	}
}

func budgetFromIntentArgs(args map[string]string) (Budget, error) {
	if args["elapsedLimit"] == "" || args["attemptLimit"] == "" ||
		args["reservedJobMinutesLimit"] == "" || args["activeJobLimit"] == "" {
		return Budget{}, fmt.Errorf("the stored budget is incomplete; all four fields are required")
	}
	attempts, err := strconv.ParseInt(args["attemptLimit"], 10, 64)
	if err != nil {
		return Budget{}, fmt.Errorf("the stored attemptLimit is invalid: %v", err)
	}
	reserved, err := strconv.ParseInt(args["reservedJobMinutesLimit"], 10, 64)
	if err != nil {
		return Budget{}, fmt.Errorf("the stored reservedJobMinutesLimit is invalid: %v", err)
	}
	active, err := strconv.ParseInt(args["activeJobLimit"], 10, 64)
	if err != nil {
		return Budget{}, fmt.Errorf("the stored activeJobLimit is invalid: %v", err)
	}
	return NewBudget(args["elapsedLimit"], attempts, reserved, active)
}
