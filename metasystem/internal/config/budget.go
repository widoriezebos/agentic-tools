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
	RiskGateKey                = "metasystem.budget.risk-gate"
	RiskGateMark               = "mark"
	RiskGateEnforce            = "enforce"
)

func RiskGate(confPath string) (string, error) {
	value, err := budgetLawValue(confPath, RiskGateKey, RiskGateMark)
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", RiskGateKey, err)
	}
	if value != RiskGateMark && value != RiskGateEnforce {
		return "", fmt.Errorf("%s must be mark or enforce", RiskGateKey)
	}
	return value, nil
}

var retiredKeys = map[string]string{
	GoalNormJobMinutesKey: "is retired; use " + Tier1BudgetKey + ", " + Tier2BudgetKey + ", and " + Tier3BudgetKey,
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

type strictBudgetSettings map[string][]string

func parseStrictBudgetSettings(content []byte) strictBudgetSettings {
	settings := strictBudgetSettings{}
	parseSettings(string(content), func(_ int, key, value string, ok bool) {
		if ok {
			settings[key] = append(settings[key], value)
		}
	})
	return settings
}

func (s strictBudgetSettings) lookup(key string) (string, bool, error) {
	values := s[key]
	if len(values) > 1 {
		return "", false, fmt.Errorf("duplicate metasystem configuration key: %s", key)
	}
	if len(values) == 1 {
		return values[0], true, nil
	}
	return "", false, nil
}

func (s strictBudgetSettings) last(key string) string {
	values := s[key]
	if len(values) == 0 {
		return ""
	}
	return values[len(values)-1]
}

// TierBoxSet is one immutable read of the configuration sources used by goal
// norm admission. Resolving several tiers from a set performs no further file
// reads, so one frontier cannot observe a different law for each candidate.
type TierBoxSet struct {
	confPath   string
	fixture    bool
	committed  strictBudgetSettings
	local      strictBudgetSettings
	localFound bool
}

// LoadTierBoxSet reads the committed configuration and its regular .local
// sibling at most once each, retaining the existing source-authority rules.
func LoadTierBoxSet(confPath string) (*TierBoxSet, error) {
	return loadTierBoxSet(confPath, os.ReadFile)
}

func loadTierBoxSet(confPath string, readFile func(string) ([]byte, error)) (*TierBoxSet, error) {
	committed := strictBudgetSettings{}
	content, err := readFile(confPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("cannot read metasystem configuration: %s: %w", confPath, err)
		}
	} else {
		committed = parseStrictBudgetSettings(content)
	}

	set := &TierBoxSet{confPath: confPath, fixture: committed.last("metasystem.runtimes") == "fake", committed: committed}
	localPath := confPath + ".local"
	if isFile(localPath) {
		local, readErr := readFile(localPath)
		if readErr != nil {
			return nil, fmt.Errorf("cannot read metasystem configuration: %s: %w", localPath, readErr)
		}
		set.local, set.localFound = parseStrictBudgetSettings(local), true
	}
	if err := set.refuseRetiredKeys(); err != nil {
		return nil, err
	}
	return set, nil
}

func (s *TierBoxSet) refuseRetiredKeys() error {
	for key, message := range retiredKeys {
		if _, present, err := s.committed.lookup(key); err != nil {
			return err
		} else if present {
			return fmt.Errorf("%s %s", key, message)
		}
		if s.localFound {
			if _, present, err := s.local.lookup(key); err != nil {
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

func (s *TierBoxSet) lawValue(key, fallback string) (string, error) {
	if s.fixture {
		if value, present := os.LookupEnv(EnvName(key)); present {
			return value, nil
		}
		if s.localFound {
			if value, present, err := s.local.lookup(key); err != nil {
				return "", err
			} else if present {
				return value, nil
			}
		}
		if value, present, err := s.committed.lookup(key); err != nil {
			return "", err
		} else if present {
			return value, nil
		}
		return fallback, nil
	}
	if _, present := os.LookupEnv(EnvName(key)); present {
		return "", fmt.Errorf("%s accepts only committed root configuration outside a fixture-authorized root; environment source %s is refused", key, EnvName(key))
	}
	if s.localFound {
		if _, present, err := s.local.lookup(key); err != nil {
			return "", err
		} else if present {
			return "", fmt.Errorf("%s accepts only committed root configuration outside a fixture-authorized root; .local source %s is refused", key, s.confPath+".local")
		}
	}
	if value, present, err := s.committed.lookup(key); err != nil {
		return "", err
	} else if present {
		return value, nil
	}
	return fallback, nil
}

func (s *TierBoxSet) reviewRoundMax() (uint64, error) {
	value, err := s.lawValue(ReviewRoundMaxKey, strconv.FormatUint(DefaultReviewRoundMax, 10))
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

// TierBox resolves one complete five-member budget from this immutable set.
func (s *TierBoxSet) TierBox(tier uint8) (goalbudget.Budget, error) {
	key, err := tierBudgetKey(tier)
	if err != nil {
		return goalbudget.Budget{}, err
	}
	defaults := map[uint8]string{1: "1h/3/360m/1/0", 2: "4h/6/720m/1/2", 3: "8h/10/1200m/1/3"}
	value, err := s.lawValue(key, defaults[tier])
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
	maximum, err := s.reviewRoundMax()
	if err != nil {
		return goalbudget.Budget{}, err
	}
	if err := budget.Validate(maximum); err != nil {
		return goalbudget.Budget{}, fmt.Errorf("%s: %w (%s=%d)", key, err, ReviewRoundMaxKey, maximum)
	}
	return budget, nil
}

// TierBox resolves the complete five-member budget assigned at intake.
func TierBox(confPath string, tier uint8) (goalbudget.Budget, error) {
	set, err := LoadTierBoxSet(confPath)
	if err != nil {
		return goalbudget.Budget{}, err
	}
	return set.TierBox(tier)
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
