package config

import (
	"fmt"
	"regexp"
	"strings"
)

var nonModelChar = regexp.MustCompile(`[^a-z0-9]+`)

// CanonicalModel is the shared model-key encoding used by adapters and cap
// authorities: lowercase, every run of non-alphanumeric characters collapsed
// to a single dash, and leading/trailing dashes trimmed. So "Claude Opus 4.8"
// and "gpt-5.6-sol" become "claude-opus-4-8" and "gpt-5-6-sol", giving one
// stable key whatever spelling a caller used.
func CanonicalModel(name string) string {
	lowered := strings.ToLower(strings.TrimSpace(name))
	return strings.Trim(nonModelChar.ReplaceAllString(lowered, "-"), "-")
}

// ResolveModelAlias resolves a configured model-family pointer. Alias keys are
// committed policy outside fixture-authorized roots, so machine-local and
// environment values are refused by the same origin rule as budget law.
func ResolveModelAlias(confPath, runtime, model string) (canonical string, aliased bool, err error) {
	if model == "" || model != CanonicalModel(model) {
		return model, false, nil
	}
	key := fmt.Sprintf("runtime.%s.model-alias.%s", runtime, model)
	target, err := budgetLawValue(confPath, key, model)
	if err != nil {
		return "", false, err
	}
	origin, err := KeyOrigin(GetParams{Key: key, ConfPath: confPath})
	if err != nil {
		return "", false, err
	}
	return target, origin != "default", nil
}
