package config

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const (
	ContextCeilingTokensKey                 = "context.ceiling.tokens"
	ContextHandoffMarginTokensKey           = "context.handoff.margin.tokens"
	ContextHandoffNoteDirectoryPrefix       = "context.handoff.note-directory."
	ContextToolGateModeKey                  = "context.toolgate.mode"
	ContextToolGateReserveCallsKey          = "context.toolgate.reserve.calls"
	DefaultContextCeilingTokens       int64 = 250000
	DefaultContextHandoffMarginTokens int64 = 145000
	// The construction line is 150000 proof tokens minus 3 handoff calls of at most 14454 tokens.
	ContextConstructionLineTokens int64 = 106638
)

type Budget struct{ Ceiling, Margin, Trigger, Reserve int64 }

var contextKeys = map[string]struct{}{
	ContextCeilingTokensKey:        {},
	ContextHandoffMarginTokensKey:  {},
	ContextToolGateModeKey:         {},
	ContextToolGateReserveCallsKey: {},
}

var contextRuntimeName = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)

// ContextBudget reads the committed context law for an installation root.
func ContextBudget(root string) (Budget, error) {
	return contextBudgetFromConf(filepath.Join(root, "metasystem.conf"))
}

func contextBudgetFromConf(confPath string) (Budget, error) {
	if err := validateContextKeys(confPath); err != nil {
		return Budget{}, err
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
	reserve, err := contextOptionalPositiveValue(confPath, ContextToolGateReserveCallsKey)
	if err != nil {
		return Budget{}, err
	}
	budget := Budget{Ceiling: ceiling, Margin: margin, Trigger: ceiling - margin, Reserve: reserve}
	constructionLine := ContextConstructionLine(reserve)
	if budget.Trigger > constructionLine {
		return Budget{}, contextConfigInvalid(ContextCeilingTokensKey, "trigger %d exceeds construction line %d", budget.Trigger, constructionLine)
	}
	return budget, nil
}

// ContextConstructionLine returns the latest safe trigger for a reserve size.
func ContextConstructionLine(reserve int64) int64 {
	return 150000 - (3+reserve)*14454
}

func validateContextKeys(confPath string) error {
	for _, key := range Keys(confPath, "context.", nil) {
		_, known := contextKeys[key]
		runtime := strings.TrimPrefix(key, ContextHandoffNoteDirectoryPrefix)
		if !known && (!strings.HasPrefix(key, ContextHandoffNoteDirectoryPrefix) || !contextRuntimeName.MatchString(runtime)) {
			return contextConfigInvalid(key, "unknown context key")
		}
	}
	return nil
}

// ToolGateMode reads whether the Claude tool gate observes or denies calls.
func ToolGateMode(root string) (string, error) {
	confPath := filepath.Join(root, "metasystem.conf")
	if err := validateContextKeys(confPath); err != nil {
		return "", err
	}
	mode, err := budgetLawValue(confPath, ContextToolGateModeKey, "observe")
	if err != nil {
		return "", contextConfigInvalid(ContextToolGateModeKey, "%v", err)
	}
	if mode != "observe" && mode != "deny" {
		return "", contextConfigInvalid(ContextToolGateModeKey, "must be observe or deny, got %q", mode)
	}
	return mode, nil
}

// ContextHandoffNoteDirectory resolves the directory that may contain a
// runtime's handoff note. Claude uses its discovered project memory directory
// when the committed configuration does not name a different directory.
func ContextHandoffNoteDirectory(root, runtime, claudeDefault string) (string, error) {
	if !contextRuntimeName.MatchString(runtime) {
		return "", contextConfigInvalid(ContextHandoffNoteDirectoryPrefix+runtime, "invalid runtime")
	}
	key := ContextHandoffNoteDirectoryPrefix + runtime
	fallback := ""
	if runtime == "claude" {
		fallback = claudeDefault
	}
	value, err := budgetLawValue(filepath.Join(root, "metasystem.conf"), key, fallback)
	if err != nil {
		return "", contextConfigInvalid(key, "%v", err)
	}
	if value == "" {
		return "", contextConfigInvalid(key, "must name the runtime's handoff-note directory")
	}
	if !filepath.IsAbs(value) {
		return "", contextConfigInvalid(key, "must be an absolute path, got %q", value)
	}
	return filepath.Clean(value), nil
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

func contextOptionalPositiveValue(confPath, key string) (int64, error) {
	const absent = "\x00"
	raw, err := budgetLawValue(confPath, key, absent)
	if err != nil {
		return 0, contextConfigInvalid(key, "%v", err)
	}
	if raw == absent {
		return 0, nil
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
