package gittree

import (
	"fmt"
	"strings"
)

// SingleParent returns the sole parent of a commit. A root commit and a
// merge are both ambiguous inputs for an exact inverse and are rejected.
func (w Workspace) SingleParent(rev string) (string, error) {
	line, err := w.gitLine(nil, "rev-list", "--parents", "-n", "1", rev)
	if err != nil {
		return "", fmt.Errorf("gittree parent of %s: %w", rev, err)
	}
	fields := strings.Fields(line)
	if len(fields) != 2 {
		return "", fmt.Errorf("gittree parent of %s: commit must have exactly one parent", rev)
	}
	if !treeID.MatchString(fields[0]) || !treeID.MatchString(fields[1]) {
		return "", fmt.Errorf("gittree parent of %s: git returned malformed object ids", rev)
	}
	return fields[1], nil
}
