package gittree

import (
	"os"
	"path/filepath"
)

// OutsideRepository reports whether no directory from dir up to the
// filesystem root holds a .git entry: git's "not a git repository", read
// from the filesystem instead of from git's words. A caller whose git probe
// ran and exited non-zero treats dir as holding no repository only when this is true; a
// failure beside a .git is a repository git cannot read, and fails closed.
func OutsideRepository(dir string) bool {
	current, err := filepath.Abs(dir)
	if err != nil {
		return false
	}
	for {
		if _, err := os.Lstat(filepath.Join(current, ".git")); err == nil || !os.IsNotExist(err) {
			return false
		}
		parent := filepath.Dir(current)
		if parent == current {
			return true
		}
		current = parent
	}
}
