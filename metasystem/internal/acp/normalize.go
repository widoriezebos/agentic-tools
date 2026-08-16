package acp

import (
	"os"
	"path/filepath"
)

// Canonicalize is the normalizer's path stage: symlinks resolved,
// and on case-insensitive hosts the ON-DISK component spelling
// substituted, so that allInside's byte comparison is a real
// identity check. Both roots and request paths MUST pass through
// here before Decide — the pure function assumes canonical inputs
// and never touches the filesystem itself. Nonexistent trailing
// components keep their given spelling (a write target may not
// exist yet); every existing prefix is canonical.
func Canonicalize(path string) (string, error) {
	if !filepath.IsAbs(path) {
		return "", &os.PathError{Op: "canonicalize", Path: path, Err: os.ErrInvalid}
	}
	return platformCanonical(deepestResolvable(path)), nil
}

// deepestResolvable resolves symlinks over the longest existing
// prefix and reattaches the nonexistent tail verbatim.
func deepestResolvable(path string) string {
	remainder := ""
	current := filepath.Clean(path)
	for {
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			if remainder == "" {
				return resolved
			}
			return filepath.Join(resolved, remainder)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return filepath.Join(current, remainder)
		}
		if remainder == "" {
			remainder = filepath.Base(current)
		} else {
			remainder = filepath.Join(filepath.Base(current), remainder)
		}
		current = parent
	}
}
