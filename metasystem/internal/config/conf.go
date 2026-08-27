// Package config owns metasystem configuration reading at three depths:
// ConfValue in this file is the never-fails hot-path reader of one
// conf-format file (last matching key wins, comments and blanks
// skipped), Get in resolve.go is the full layered resolution (flag →
// env → .local → mode-scoped → committed → default) behind the config
// verbs, and Validate checks the whole domain against a repository.
package config

import (
	"os"
	"strconv"
	"strings"
)

// DefaultAppetiteOverrunGracePercent is the shipped fallback when an older
// configuration has not yet materialized the appetite grace key.
const DefaultAppetiteOverrunGracePercent = 25

// parseSettings walks conf content applying THE line rule exactly once
// (review foundations-7): a setting is a non-blank, non-comment line
// containing '='; the key is left of the FIRST '=', both sides trimmed.
// A non-blank, non-comment line without '=' visits with ok=false so
// strict consumers can name it; tolerant consumers skip it. Divergent
// duplicate-key semantics live in the consumers, each stating why.
func parseSettings(content string, visit func(lineNo int, key, value string, ok bool)) {
	for number, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.Contains(line, "=") {
			visit(number+1, "", "", false)
			continue
		}
		name, val, _ := strings.Cut(line, "=")
		visit(number+1, strings.TrimSpace(name), strings.TrimSpace(val), true)
	}
}

// ConfValue returns the value of key from the given metasystem.conf path, or
// def when the file is unreadable or the key is absent. A line is a setting
// when it is non-blank, not a comment, and contains '='; the LAST such line
// for a key wins.
func ConfValue(confPath, key, def string) string {
	content, err := os.ReadFile(confPath)
	if err != nil {
		return def
	}
	value := def
	found := false
	// Last duplicate wins HERE, deliberately: hot-path readers must not
	// fail on a conf a strict verb would refuse; validate names the
	// duplicate, ConfLookup errors on it.
	parseSettings(string(content), func(_ int, name, val string, ok bool) {
		if ok && name == key {
			value = val
			found = true
		}
	})
	if !found {
		return def
	}
	return value
}

// AppetiteOverrunGracePercent is the never-fails runtime reader for the
// appetite grace band. Strict validation names malformed configuration; the
// hot path retains the built-in bound instead of silently disabling it.
func AppetiteOverrunGracePercent(confPath string) int {
	raw := ConfValue(confPath, "appetite.overrun-grace-percent", strconv.Itoa(DefaultAppetiteOverrunGracePercent))
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 || value > 100 {
		return DefaultAppetiteOverrunGracePercent
	}
	return value
}
