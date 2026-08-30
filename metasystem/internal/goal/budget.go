package goal

import (
	"fmt"
	"strconv"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalbudget"
)

// ParseWorkingDuration accepts one or more positive integer day, hour, and
// minute segments. A working day is eight hours; seconds and fractions are not
// part of the budget grammar.
func ParseWorkingDuration(value string) (time.Duration, bool) {
	return goalbudget.ParseWorkingDuration(value)
}

// FormatWorkingDuration is the canonical elapsed-limit token. It preserves
// the working-day convention and never emits seconds.
func FormatWorkingDuration(value time.Duration) string {
	return goalbudget.FormatWorkingDuration(value)
}

// Budget is the complete limit tuple carried by one goal revision. Spending
// is projected from job and governed-run records; the goal never stores
// counters beside these limits.
type Budget = goalbudget.Budget

// NewBudget validates and canonicalizes the complete tuple accepted by the
// command surface. There is no partial form and no numeric default.
func NewBudget(elapsedLimit string, attemptLimit, reservedJobMinutesLimit, activeJobLimit int64) (Budget, error) {
	return goalbudget.New(elapsedLimit, attemptLimit, reservedJobMinutesLimit, activeJobLimit)
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
