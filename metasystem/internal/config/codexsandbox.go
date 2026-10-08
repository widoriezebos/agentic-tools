package config

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// The host's Codex sandbox mode: one setting per host, in its local
// settings. workspace-write keeps each envelope's own read-only or
// workspace-write mapping; danger-full-access runs every Codex launch
// unsandboxed, on a host the person declares trusted.
const (
	CodexSandboxKey            = "launch.codex.sandbox"
	CodexSandboxWorkspaceWrite = "workspace-write"
	CodexSandboxFullAccess     = "danger-full-access"
	// CodexSandboxWidening is the widenedBy value of a Codex job's requested
	// envelope admitted under full access: the record names the setting that
	// left the job uncontained.
	CodexSandboxWidening = CodexSandboxKey + "=" + CodexSandboxFullAccess
)

// ParseCodexSandbox accepts the two modes and refuses anything else, naming
// both.
func ParseCodexSandbox(value string) (string, error) {
	switch value {
	case CodexSandboxWorkspaceWrite, CodexSandboxFullAccess:
		return value, nil
	}
	return "", fmt.Errorf("%s must be %s or %s, not %q", CodexSandboxKey, CodexSandboxWorkspaceWrite, CodexSandboxFullAccess, value)
}

// CodexSandbox resolves the host's Codex sandbox mode the way every launch
// setting resolves; an empty confPath reads the compiled default under the
// environment.
func CodexSandbox(confPath string, lookupEnv func(string) (string, bool)) (string, error) {
	value, _, err := Get(GetParams{Key: CodexSandboxKey, ConfPath: confPath, LookupEnv: lookupEnv})
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", CodexSandboxKey, err)
	}
	return ParseCodexSandbox(value)
}

// SettingValueProblem is settings set's check of one value before it is
// written: a key with a closed set of values refuses one outside it. Every
// other key is written as given and judged by settings check.
func SettingValueProblem(key, value string) error {
	if key == "host.load-max" {
		limit, err := strconv.ParseFloat(value, 64)
		if err != nil || math.IsNaN(limit) || math.IsInf(limit, 0) || limit <= 0 {
			return fmt.Errorf("host.load-max must be a positive finite number")
		}
	}
	if key == "proof.deadline" {
		minutes, err := strconv.ParseInt(value, 10, 64)
		if err != nil || minutes <= 0 || minutes > int64((1<<63-1)/time.Minute) {
			return fmt.Errorf("%s, the check deadline, must be positive minutes within the timer range", key)
		}
	}
	if PolicyScope(key) != "" {
		return policyValueProblem(key, value)
	}
	if CommittedOnly(key) && (strings.TrimSpace(value) == "" || strings.ContainsAny(value, "\r\n")) {
		return fmt.Errorf("%s must be a non-empty, one-line command in metasystem.conf", key)
	}
	if key == CodexSandboxKey {
		_, err := ParseCodexSandbox(value)
		return err
	}
	return nil
}
