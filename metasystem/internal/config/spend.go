package config

import (
	"fmt"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
)

const (
	SpendModeKey              = "spend.mode"
	SpendCurrencyKey          = "spend.currency"
	SpendCeilingDayTokensKey  = "spend.ceiling.day.tokens"
	SpendCeilingDayMoneyKey   = "spend.ceiling.day.money"
	SpendCeilingGoalTokensKey = "spend.ceiling.goal.tokens"
	SpendCeilingGoalMoneyKey  = "spend.ceiling.goal.money"

	DefaultSpendMode              = "alert"
	DefaultSpendCurrency          = "USD"
	DefaultSpendCeilingDayTokens  = uint64(250000000)
	DefaultSpendCeilingDayMoney   = 750.0
	DefaultSpendCeilingGoalTokens = uint64(125000000)
	DefaultSpendCeilingGoalMoney  = 300.0
)

var (
	spendCurrencyPattern    = regexp.MustCompile(`^[A-Z]{3}$`)
	spendPositiveDecimal    = regexp.MustCompile(`^(?:0\.[0-9]*[1-9][0-9]*|[1-9][0-9]*(?:\.[0-9]+)?)$`)
	spendNonnegativeDecimal = regexp.MustCompile(`^(?:0|[1-9][0-9]*)(?:\.[0-9]+)?$`)
)

// SpendPriceKey identifies one configured price per million tokens.
type SpendPriceKey struct {
	Runtime string
	Model   string
	Class   string
}

// SpendSettings is the committed alert-mode policy used by the meter.
type SpendSettings struct {
	Mode             string
	Currency         string
	DayTokenCeiling  uint64
	DayMoneyCeiling  float64
	GoalTokenCeiling uint64
	GoalMoneyCeiling float64
	Prices           map[SpendPriceKey]float64
}

var spendFixedDefaults = map[string]string{
	SpendModeKey:              DefaultSpendMode,
	SpendCurrencyKey:          DefaultSpendCurrency,
	SpendCeilingDayTokensKey:  strconv.FormatUint(DefaultSpendCeilingDayTokens, 10),
	SpendCeilingDayMoneyKey:   strconv.FormatFloat(DefaultSpendCeilingDayMoney, 'f', -1, 64),
	SpendCeilingGoalTokensKey: strconv.FormatUint(DefaultSpendCeilingGoalTokens, 10),
	SpendCeilingGoalMoneyKey:  strconv.FormatFloat(DefaultSpendCeilingGoalMoney, 'f', -1, 64),
}

// ReadSpendSettings resolves every spend law from committed root
// configuration. A fixture-authorized fake-runtime root may use the ordinary
// local and environment layers.
func ReadSpendSettings(confPath string) (SpendSettings, error) {
	values := map[string]string{}
	for _, key := range []string{
		SpendModeKey, SpendCurrencyKey, SpendCeilingDayTokensKey,
		SpendCeilingDayMoneyKey, SpendCeilingGoalTokensKey, SpendCeilingGoalMoneyKey,
	} {
		value, err := budgetLawValue(confPath, key, spendFixedDefaults[key])
		if err != nil {
			return SpendSettings{}, fmt.Errorf("resolve %s: %w", key, err)
		}
		values[key] = value
	}
	if err := validateSpendFixedValues(values); err != nil {
		return SpendSettings{}, err
	}
	dayTokens, _ := strconv.ParseUint(values[SpendCeilingDayTokensKey], 10, 64)
	goalTokens, _ := strconv.ParseUint(values[SpendCeilingGoalTokensKey], 10, 64)
	dayMoney, _ := strconv.ParseFloat(values[SpendCeilingDayMoneyKey], 64)
	goalMoney, _ := strconv.ParseFloat(values[SpendCeilingGoalMoneyKey], 64)
	settings := SpendSettings{
		Mode: values[SpendModeKey], Currency: values[SpendCurrencyKey],
		DayTokenCeiling: dayTokens, DayMoneyCeiling: dayMoney,
		GoalTokenCeiling: goalTokens, GoalMoneyCeiling: goalMoney,
		Prices: map[SpendPriceKey]float64{},
	}
	if err := refuseUncommittedSpendFamily(confPath); err != nil {
		return SpendSettings{}, err
	}
	for _, key := range Keys(confPath, "spend.price.", nil) {
		priceKey, err := parseSpendPriceKey(key)
		if err != nil {
			return SpendSettings{}, err
		}
		value, err := budgetLawValue(confPath, key, "")
		if err != nil {
			return SpendSettings{}, fmt.Errorf("resolve %s: %w", key, err)
		}
		if !spendNonnegativeDecimal.MatchString(value) {
			return SpendSettings{}, fmt.Errorf("%s must be a non-negative decimal, got %q", key, value)
		}
		price, err := strconv.ParseFloat(value, 64)
		if err != nil || math.IsInf(price, 0) || math.IsNaN(price) {
			return SpendSettings{}, fmt.Errorf("%s must be a non-negative decimal, got %q", key, value)
		}
		settings.Prices[priceKey] = price
	}
	return settings, nil
}

func validateSpendFixedValues(values map[string]string) error {
	mode := values[SpendModeKey]
	if mode == "enforce" {
		return fmt.Errorf("spend.mode=enforce is refused until step 2 lands on Wido's word (R-60-m1)")
	}
	if mode != "alert" {
		return fmt.Errorf("%s must be alert, got %q", SpendModeKey, mode)
	}
	if !spendCurrencyPattern.MatchString(values[SpendCurrencyKey]) {
		return fmt.Errorf("%s must be three uppercase letters, got %q", SpendCurrencyKey, values[SpendCurrencyKey])
	}
	for _, key := range []string{SpendCeilingDayTokensKey, SpendCeilingGoalTokensKey} {
		if !positiveInteger.MatchString(values[key]) {
			return fmt.Errorf("%s must be a positive integer, got %q", key, values[key])
		}
		if _, err := strconv.ParseUint(values[key], 10, 64); err != nil {
			return fmt.Errorf("%s must be a positive integer, got %q", key, values[key])
		}
	}
	for _, key := range []string{SpendCeilingDayMoneyKey, SpendCeilingGoalMoneyKey} {
		if !spendPositiveDecimal.MatchString(values[key]) {
			return fmt.Errorf("%s must be a positive decimal, got %q", key, values[key])
		}
		value, err := strconv.ParseFloat(values[key], 64)
		if err != nil || math.IsInf(value, 0) || math.IsNaN(value) {
			return fmt.Errorf("%s must be a positive decimal, got %q", key, values[key])
		}
	}
	return nil
}

func parseSpendPriceKey(key string) (SpendPriceKey, error) {
	parts := strings.Split(key, ".")
	if len(parts) != 5 || parts[0] != "spend" || parts[1] != "price" {
		return SpendPriceKey{}, fmt.Errorf("unsupported spend price key %s", key)
	}
	if parts[2] == "" || parts[3] == "" || CanonicalModel(parts[3]) != parts[3] {
		return SpendPriceKey{}, fmt.Errorf("non-canonical spend price key %s", key)
	}
	switch parts[4] {
	case "input", "cached", "output", "reasoning":
	default:
		return SpendPriceKey{}, fmt.Errorf("unsupported spend price key %s", key)
	}
	return SpendPriceKey{Runtime: parts[2], Model: parts[3], Class: parts[4]}, nil
}

func refuseUncommittedSpendFamily(confPath string) error {
	if fixtureBudgetLawRoot(confPath) {
		return nil
	}
	localPath := confPath + ".local"
	if content, err := os.ReadFile(localPath); err == nil {
		var found string
		parseSettings(string(content), func(_ int, key, _ string, ok bool) {
			if found == "" && ok && strings.HasPrefix(key, "spend.") {
				found = key
			}
		})
		if found != "" {
			return fmt.Errorf("%s accepts only committed root configuration outside a fixture-authorized root; .local source %s is refused", found, localPath)
		}
	}
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(name, "METASYSTEM_SPEND_") {
			return fmt.Errorf("spend settings accept only committed root configuration outside a fixture-authorized root; environment source %s is refused", name)
		}
	}
	return nil
}
