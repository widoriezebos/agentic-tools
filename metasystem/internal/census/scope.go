package census

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Scope determination port (process-census.py path_below, argv_paths): a
// process is in a checkout's scope when its resolved cwd is below the repo,
// OR its argv names a path below the repo. This is how the census decides
// which processes belong to a checkout without killing anything.

// realpath resolves symlinks the way python's os.path.realpath does: it
// resolves the LONGEST EXISTING PREFIX and appends the non-existent
// remainder, rather than failing wholesale as filepath.EvalSymlinks does on
// a missing component. This matters because process argvs routinely name
// paths that do not exist, and a checkout scope check must still resolve the
// /var -> /private/var symlink in their existing ancestry.
func realpath(path string) string {
	path = filepath.Clean(path)
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	// Find the longest existing ancestor, resolve it, rejoin the remainder.
	dir, remainder := path, ""
	for {
		parent := filepath.Dir(dir)
		remainder = filepath.Join(filepath.Base(dir), remainder)
		if parent == dir { // reached the root
			return path
		}
		if resolved, err := filepath.EvalSymlinks(parent); err == nil {
			return filepath.Join(resolved, remainder)
		}
		dir = parent
	}
}

// pathFlags are the flags whose following token (or =value) is a path
// argument — mirrors PATH_FLAGS.
var pathFlags = map[string]bool{
	"-C": true, "--cwd": true, "--directory": true, "--path": true,
	"--project-dir": true, "--repo": true, "--root": true,
	"--workspace": true, "--worktree": true,
}

// PathBelow reports whether candidate, once symlink-resolved, is at or below
// root. Faithful to path_below (which resolves the candidate; run_census
// resolves the repo before calling it) — here root is also resolved so the
// comparison is symlink-stable regardless of how the caller spelled it (on
// macOS /var is a symlink to /private/var). An unresolvable candidate is not
// below.
func PathBelow(candidate, root string) bool {
	resolved := realpath(candidate)
	root = realpath(root)
	rel, err := filepath.Rel(root, resolved)
	if err != nil {
		return false
	}
	// relative_to in python succeeds iff resolved is at or under root: no
	// leading "..", and not an absolute escape.
	return rel == "." || (!strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != "..")
}

// ArgvPaths extracts the filesystem paths an argv names, resolving relative
// ones against cwd — the port of argv_paths. A path flag's value, an
// =value on a path flag, and any bare token that looks like a path
// (absolute or ./ ../) are candidates; URLs (containing "://") are skipped.
// Tokenization is POSIX shell splitting; a malformed argv errors.
func ArgvPaths(argv string, cwd string) ([]string, error) {
	tokens, err := shellSplit(argv)
	if err != nil {
		return nil, fmt.Errorf("argv tokenization failed: %w", err)
	}
	var paths []string
	previousPathFlag := false
	for _, token := range tokens {
		var candidate string
		switch {
		case previousPathFlag:
			candidate = token
			previousPathFlag = false
		case pathFlags[token]:
			previousPathFlag = true
			continue
		case strings.HasPrefix(token, "-") && strings.Contains(token, "="):
			flag, value, _ := strings.Cut(token, "=")
			if pathFlags[flag] {
				candidate = value
			}
		case strings.HasPrefix(token, "/") || strings.HasPrefix(token, "./") || strings.HasPrefix(token, "../"):
			candidate = token
		}
		if candidate == "" || strings.Contains(candidate, "://") {
			continue
		}
		path := candidate
		if !filepath.IsAbs(path) {
			if cwd == "" {
				continue
			}
			path = filepath.Join(cwd, path)
		}
		paths = append(paths, realpath(path))
	}
	return paths, nil
}
