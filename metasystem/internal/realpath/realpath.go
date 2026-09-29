// Package realpath owns the two meanings "where a path really is" has in the
// runner, and the one containment test over them.
//
// Resolve is the strict form: symlinks resolve through the deepest EXISTING
// ancestor and a missing tail is re-attached, so a path that will be created
// compares against the directory it will really land in. Every containment
// check uses it: with the weak form, a not-yet-existing path under a symlink
// pointing out of the boundary stays lexical and reads as inside.
//
// ResolveExisting is the weak form: symlinks resolve only when the whole path
// exists; otherwise the absolute lexical form is returned. It is for naming a
// directory that exists (a checkout, a workspace), never for containment.
//
// Canonical is the refusing form: the absolute, symlink-free path of
// something that must exist (a proof root), or the error saying it does not.
package realpath

import (
	"path/filepath"
	"strings"
)

// Resolve returns the absolute, symlink-free form of path, resolving through
// its deepest existing ancestor and keeping any missing suffix.
func Resolve(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	suffix := ""
	current := path
	for {
		if real, err := filepath.EvalSymlinks(current); err == nil {
			return filepath.Join(real, suffix)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return filepath.Clean(path)
		}
		suffix = filepath.Join(filepath.Base(current), suffix)
		current = parent
	}
}

// ResolveExisting returns the absolute form of path with symlinks resolved
// when the whole path exists, and the absolute lexical form otherwise.
func ResolveExisting(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	if real, err := filepath.EvalSymlinks(path); err == nil {
		return real
	}
	return path
}

// Canonical returns the absolute, cleaned, symlink-free form of an existing
// path, or the error resolving it.
func Canonical(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", err
	}
	return filepath.Clean(resolved), nil
}

// Within reports whether path is root or lies below it, comparing whole path
// segments so /a/bc never counts as inside /a/b. Both should already be in
// the same form (normally Resolve's).
func Within(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
