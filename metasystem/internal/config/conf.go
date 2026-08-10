// Package config reads the metasystem.conf settings file. This is the
// CONF-ONLY reader (no metasystem.conf.local precedence — that belongs to
// dispatch's config_get): last matching key wins, comments and blanks
// skipped.
package config

import (
	"os"
	"strings"
)

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
	for _, raw := range strings.Split(string(content), "\n") {
		stripped := strings.TrimSpace(raw)
		if stripped == "" || strings.HasPrefix(stripped, "#") || !strings.Contains(raw, "=") {
			continue
		}
		name, val, _ := strings.Cut(raw, "=")
		if strings.TrimSpace(name) == key {
			value = strings.TrimSpace(val)
			found = true
		}
	}
	if !found {
		return def
	}
	return value
}
