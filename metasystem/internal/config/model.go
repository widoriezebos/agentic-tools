package config

import (
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
