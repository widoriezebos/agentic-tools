//go:build darwin

package acp

import (
	"os"
	"path/filepath"
	"strings"
)

// platformCanonical substitutes the directory entries' on-disk
// spelling, making /users/X and /Users/X identical on APFS's
// case-insensitive default. Components that do not exist (or
// cannot be listed) keep their given spelling.
func platformCanonical(path string) string {
	components := strings.Split(strings.TrimPrefix(path, string(filepath.Separator)), string(filepath.Separator))
	result := string(filepath.Separator)
	for _, component := range components {
		if component == "" {
			continue
		}
		entries, err := os.ReadDir(result)
		matched := component
		if err == nil {
			for _, entry := range entries {
				if strings.EqualFold(entry.Name(), component) {
					matched = entry.Name()
					break
				}
			}
		}
		result = filepath.Join(result, matched)
	}
	return result
}
