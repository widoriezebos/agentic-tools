package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalbudget"
)

const (
	ElapsedGracePercentKey     = "metasystem.budget.elapsed-grace-percent"
	DefaultElapsedGracePercent = uint64(50)
	MaxElapsedGracePercent     = uint64(200)
	SliceNormHoursKey          = "metasystem.budget.slice-norm-hours"
	DefaultSliceNormHours      = uint64(4)
	GoalNormJobMinutesKey      = "metasystem.budget.goal-norm-job-minutes" // retired tombstone only
	ReviewRoundMaxKey          = "metasystem.budget.review-round-max"
	DefaultReviewRoundMax      = uint64(3)
	Tier1BudgetKey             = "metasystem.budget.tier-1"
	Tier2BudgetKey             = "metasystem.budget.tier-2"
	Tier3BudgetKey             = "metasystem.budget.tier-3"
)

var retiredKeys = map[string]string{
	GoalNormJobMinutesKey: "is retired; use " + Tier1BudgetKey + ", " + Tier2BudgetKey + ", and " + Tier3BudgetKey,
}

func refuseRetiredKeys(confPath string) error {
	for key, message := range retiredKeys {
		if _, present, err := ConfLookup(confPath, key); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return err
			}
		} else if present {
			return fmt.Errorf("%s %s", key, message)
		}
		localPath := confPath + ".local"
		if isFile(localPath) {
			if _, present, err := ConfLookup(localPath, key); err != nil {
				return err
			} else if present {
				return fmt.Errorf("%s %s", key, message)
			}
		}
		if _, present := os.LookupEnv(EnvName(key)); present {
			return fmt.Errorf("%s %s", key, message)
		}
	}
	return nil
}

func ReviewRoundMax(confPath string) (uint64, error) {
	value, err := budgetLawValue(confPath, ReviewRoundMaxKey, strconv.FormatUint(DefaultReviewRoundMax, 10))
	if err != nil {
		return 0, fmt.Errorf("resolve %s: %w", ReviewRoundMaxKey, err)
	}
	if !digitsOnlyValue.MatchString(value) {
		return 0, fmt.Errorf("%s must be a non-negative integer", ReviewRoundMaxKey)
	}
	maximum, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be a non-negative integer", ReviewRoundMaxKey)
	}
	return maximum, nil
}

func tierBudgetKey(tier uint8) (string, error) {
	switch tier {
	case 1:
		return Tier1BudgetKey, nil
	case 2:
		return Tier2BudgetKey, nil
	case 3:
		return Tier3BudgetKey, nil
	default:
		return "", fmt.Errorf("tier must be 1, 2, or 3")
	}
}

// TierBox resolves the complete five-member budget assigned at intake.
func TierBox(confPath string, tier uint8) (goalbudget.Budget, error) {
	if err := refuseRetiredKeys(confPath); err != nil {
		return goalbudget.Budget{}, err
	}
	key, err := tierBudgetKey(tier)
	if err != nil {
		return goalbudget.Budget{}, err
	}
	defaults := map[uint8]string{1: "1h/3/360m/1/0", 2: "4h/6/720m/1/2", 3: "8h/10/1200m/1/3"}
	value, err := budgetLawValue(confPath, key, defaults[tier])
	if err != nil {
		return goalbudget.Budget{}, fmt.Errorf("resolve %s: %w", key, err)
	}
	parts := strings.Split(value, "/")
	if len(parts) != 5 || !strings.HasSuffix(parts[2], "m") {
		return goalbudget.Budget{}, fmt.Errorf("%s must use <elapsed>/<attempts>/<minutes>/<active>/<rounds>", key)
	}
	attempts, attemptsErr := strconv.ParseInt(parts[1], 10, 64)
	minutes, minutesErr := strconv.ParseInt(strings.TrimSuffix(parts[2], "m"), 10, 64)
	active, activeErr := strconv.ParseInt(parts[3], 10, 64)
	rounds, roundsErr := strconv.ParseInt(parts[4], 10, 64)
	if attemptsErr != nil || minutesErr != nil || activeErr != nil || roundsErr != nil {
		return goalbudget.Budget{}, fmt.Errorf("%s must use <elapsed>/<attempts>/<minutes>/<active>/<rounds>", key)
	}
	budget, err := goalbudget.New(parts[0], attempts, minutes, active, rounds)
	if err != nil {
		return goalbudget.Budget{}, fmt.Errorf("%s: %w", key, err)
	}
	maximum, err := ReviewRoundMax(confPath)
	if err != nil {
		return goalbudget.Budget{}, err
	}
	if err := budget.Validate(maximum); err != nil {
		return goalbudget.Budget{}, fmt.Errorf("%s: %w (%s=%d)", key, err, ReviewRoundMaxKey, maximum)
	}
	return budget, nil
}

// ElapsedGracePercent resolves the grace band from the committed root. A root
// that explicitly declares the fake runtime may use fixture overrides.
func ElapsedGracePercent(confPath string) (uint64, error) {
	value, err := budgetLawValue(confPath, ElapsedGracePercentKey,
		strconv.FormatUint(DefaultElapsedGracePercent, 10))
	if err != nil {
		return 0, fmt.Errorf("resolve %s: %w", ElapsedGracePercentKey, err)
	}
	return parseElapsedGracePercent(value)
}

func parseElapsedGracePercent(value string) (uint64, error) {
	if !digitsOnlyValue.MatchString(value) {
		return 0, fmt.Errorf("%s must be an integer between 0 and %d, got %q",
			ElapsedGracePercentKey, MaxElapsedGracePercent, value)
	}
	percent, err := strconv.ParseUint(value, 10, 64)
	if err != nil || percent > MaxElapsedGracePercent {
		return 0, fmt.Errorf("%s must be an integer between 0 and %d, got %q",
			ElapsedGracePercentKey, MaxElapsedGracePercent, value)
	}
	return percent, nil
}

// SliceNormHours resolves the ordinary per-job slice norm. The norm is an
// admission boundary, so a malformed configured value refuses loudly instead
// of silently replacing the human's word with the default.
func SliceNormHours(confPath string) (uint64, error) {
	value, err := budgetLawValue(confPath, SliceNormHoursKey,
		strconv.FormatUint(DefaultSliceNormHours, 10))
	if err != nil {
		return 0, fmt.Errorf("resolve %s: %w", SliceNormHoursKey, err)
	}
	return parseSliceNormHours(value)
}

func parseSliceNormHours(value string) (uint64, error) {
	if !digitsOnlyValue.MatchString(value) {
		return 0, fmt.Errorf("%s must be a positive integer, got %q", SliceNormHoursKey, value)
	}
	hours, err := strconv.ParseUint(value, 10, 64)
	if err != nil || hours == 0 {
		return 0, fmt.Errorf("%s must be a positive integer, got %q", SliceNormHoursKey, value)
	}
	return hours, nil
}

func budgetLawValue(confPath, key, fallback string) (string, error) {
	if fixtureBudgetLawRoot(confPath) {
		value, _, err := Get(GetParams{
			Key: key, ConfPath: confPath, Default: fallback, DefaultSet: true,
		})
		return value, err
	}
	if _, present := os.LookupEnv(EnvName(key)); present {
		return "", fmt.Errorf("%s accepts only committed root configuration outside a fixture-authorized root; environment source %s is refused",
			key, EnvName(key))
	}
	localPath := confPath + ".local"
	if isFile(localPath) {
		_, present, err := ConfLookup(localPath, key)
		if err != nil {
			return "", err
		}
		if present {
			return "", fmt.Errorf("%s accepts only committed root configuration outside a fixture-authorized root; .local source %s is refused",
				key, localPath)
		}
	}
	value, present, err := ConfLookup(confPath, key)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fallback, nil
		}
		return "", err
	}
	if present {
		return value, nil
	}
	return fallback, nil
}

// fixtureBudgetLawRoot mirrors the fixture clock gate: only the committed
// root declaration can authorize local or environment fixture inputs.
func fixtureBudgetLawRoot(confPath string) bool {
	return ConfValue(confPath, "metasystem.runtimes", "") == "fake"
}
