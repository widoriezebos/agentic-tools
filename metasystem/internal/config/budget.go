package config

import (
	"fmt"
	"os"
	"strconv"
)

const (
	ElapsedGracePercentKey     = "metasystem.budget.elapsed-grace-percent"
	DefaultElapsedGracePercent = uint64(50)
	MaxElapsedGracePercent     = uint64(200)
	SliceNormHoursKey          = "metasystem.budget.slice-norm-hours"
	DefaultSliceNormHours      = uint64(4)
	GoalNormJobMinutesKey      = "metasystem.budget.goal-norm-job-minutes"
	DefaultGoalNormJobMinutes  = uint64(1440)
)

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

// GoalNormJobMinutes resolves the ordinary total job-minute bound for one
// goal. Like the slice norm, this law is rooted only in committed
// configuration outside explicitly fake fixture roots.
func GoalNormJobMinutes(confPath string) (uint64, error) {
	value, err := budgetLawValue(confPath, GoalNormJobMinutesKey,
		strconv.FormatUint(DefaultGoalNormJobMinutes, 10))
	if err != nil {
		return 0, fmt.Errorf("resolve %s: %w", GoalNormJobMinutesKey, err)
	}
	return parseGoalNormJobMinutes(value)
}

func parseGoalNormJobMinutes(value string) (uint64, error) {
	if !digitsOnlyValue.MatchString(value) {
		return 0, fmt.Errorf("%s must be a positive integer, got %q", GoalNormJobMinutesKey, value)
	}
	minutes, err := strconv.ParseUint(value, 10, 64)
	if err != nil || minutes == 0 {
		return 0, fmt.Errorf("%s must be a positive integer, got %q", GoalNormJobMinutesKey, value)
	}
	return minutes, nil
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
