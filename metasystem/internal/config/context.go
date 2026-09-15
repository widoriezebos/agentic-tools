package config

import (
	"fmt"
	"path/filepath"
	"strconv"
)

const (
	ContextCeilingTokensKey                 = "context.ceiling.tokens"
	ContextHandoffMarginTokensKey           = "context.handoff.margin.tokens"
	DefaultContextCeilingTokens       int64 = 250000
	DefaultContextHandoffMarginTokens int64 = 145000
	// The construction line is 150000 proof tokens minus 3 handoff calls of at most 14454 tokens.
	ContextConstructionLineTokens int64 = 106638
)

type Budget struct{ Ceiling, Margin, Trigger int64 }

var contextKeys = map[string]struct{}{
	ContextCeilingTokensKey:       {},
	ContextHandoffMarginTokensKey: {},
}

// ContextBudget reads the committed context law for an installation root.
func ContextBudget(root string) (Budget, error) {
	return contextBudgetFromConf(filepath.Join(root, "metasystem.conf"))
}

func contextBudgetFromConf(confPath string) (Budget, error) {
	for _, key := range Keys(confPath, "context.", nil) {
		if _, known := contextKeys[key]; !known {
			return Budget{}, contextConfigInvalid(key, "unknown context key")
		}
	}
	ceiling, err := contextPositiveValue(confPath, ContextCeilingTokensKey, DefaultContextCeilingTokens)
	if err != nil {
		return Budget{}, err
	}
	margin, err := contextPositiveValue(confPath, ContextHandoffMarginTokensKey, DefaultContextHandoffMarginTokens)
	if err != nil {
		return Budget{}, err
	}
	if margin >= ceiling {
		return Budget{}, contextConfigInvalid(ContextHandoffMarginTokensKey, "must be below %s=%d", ContextCeilingTokensKey, ceiling)
	}
	budget := Budget{Ceiling: ceiling, Margin: margin, Trigger: ceiling - margin}
	if budget.Trigger > ContextConstructionLineTokens {
		return Budget{}, contextConfigInvalid(ContextCeilingTokensKey, "trigger %d exceeds construction line %d", budget.Trigger, ContextConstructionLineTokens)
	}
	return budget, nil
}

func contextPositiveValue(confPath, key string, fallback int64) (int64, error) {
	raw, err := budgetLawValue(confPath, key, strconv.FormatInt(fallback, 10))
	if err != nil {
		return 0, contextConfigInvalid(key, "%v", err)
	}
	value, parseErr := strconv.ParseInt(raw, 10, 64)
	if parseErr != nil || value < 1 {
		return 0, contextConfigInvalid(key, "must be a positive integer, got %q", raw)
	}
	return value, nil
}

func contextConfigInvalid(key, format string, args ...any) error {
	return fmt.Errorf("CONTEXT_CONFIG_INVALID key=%s reason=%s", key, fmt.Sprintf(format, args...))
}
