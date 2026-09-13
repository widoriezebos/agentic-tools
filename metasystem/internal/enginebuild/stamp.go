// Package enginebuild owns the grammar of source-linked engine build stamps.
package enginebuild

import "strings"

// StampCommit extracts the source commit from clean and dirty build stamps.
func StampCommit(stamp string) (commit string, dirty bool, ok bool) {
	if commitShaped(stamp) {
		return stamp, false, true
	}
	const prefix = "dev-"
	const suffix = "-dirty"
	if strings.HasPrefix(stamp, prefix) && strings.HasSuffix(stamp, suffix) {
		commit = strings.TrimSuffix(strings.TrimPrefix(stamp, prefix), suffix)
		if commitShaped(commit) {
			return commit, true, true
		}
	}
	return "", false, false
}

func commitShaped(commit string) bool {
	if len(commit) != 40 {
		return false
	}
	for _, character := range commit {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}
