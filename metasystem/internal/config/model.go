package config

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	DefaultCodexModel   = "gpt-6.2-astra"
	CodexBuildEffortKey = "launch.build.effort.codex"
	BuildEffortKey      = "launch.build.effort"
)

// CodexBuildEffort resolves the runtime pin first, then an existing shared
// installation pin, then the Codex default. An explicit shared xhigh is a pin.
func CodexBuildEffort(conf string, lookup func(string) (string, bool)) (string, error) {
	p := GetParams{ConfPath: conf, LookupEnv: lookup, Key: CodexBuildEffortKey}
	source, err := KeyOrigin(p)
	if err != nil {
		return "", err
	}
	if source == "default" {
		p.Key = BuildEffortKey
		source, err = KeyOrigin(p)
		if err != nil {
			return "", err
		}
		if source == "default" {
			p.Key = CodexBuildEffortKey
		}
	}
	value, _, err := Get(p)
	if err == nil && strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("%s is empty", p.Key)
	}
	return value, err
}

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
	// A compiled alias (defaults.go) is as much an alias as a written one:
	// the pointer is aliased when it resolves to another model.
	return target, target != model, nil
}
