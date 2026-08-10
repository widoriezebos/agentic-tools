package validate

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// SessionIsolation prepares a second-session worktree: it copies each
// adapter-declared local-configuration path from the primary checkout
// into the new worktree, then audits that every such path resolves
// inside the new worktree and never back into the primary checkout, so
// the two sessions cannot share runtime state through a stray symlink.
// It returns the new checkout's harness root, refusing when both
// sessions would resolve one metasystem artifacts root.
func SessionIsolation(sourceRoot, destinationRoot, manifestPath, harnessRoot string) (string, error) {
	source := resolvePath(sourceRoot)
	destination := resolvePath(destinationRoot)

	manifest, err := os.ReadFile(manifestPath)
	if err != nil {
		return "", err
	}
	var relatives []string
	for _, raw := range splitLines(string(manifest)) {
		if raw == "" {
			continue
		}
		if filepath.IsAbs(raw) || containsParentStep(raw) {
			return "", fmt.Errorf("adapter local-config-path is unsafe: %s", raw)
		}
		relatives = append(relatives, raw)
	}

	for _, relative := range relatives {
		from := filepath.Join(source, relative)
		to := filepath.Join(destination, relative)
		info, err := os.Stat(from)
		if err != nil {
			continue // nothing to copy
		}
		if _, err := os.Lstat(to); err == nil {
			continue // never overwrite what the new worktree already has
		}
		if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
			return "", err
		}
		if info.IsDir() {
			err = copyTree(from, to)
		} else {
			err = copyFile(from, to)
		}
		if err != nil {
			return "", err
		}
	}

	for _, relative := range relatives {
		target := filepath.Join(destination, relative)
		if _, err := os.Stat(target); err != nil {
			continue
		}
		resolved, err := filepath.EvalSymlinks(target)
		if err != nil || !pathWithin(destination, resolved) {
			return "", fmt.Errorf("isolation audit failed: %s resolves outside the new worktree", relative)
		}
		if pathWithin(source, resolved) {
			return "", fmt.Errorf("isolation audit failed: %s still resolves into the primary checkout", relative)
		}
	}

	harness := resolvePath(harnessRoot)
	relativeHarness, err := filepath.Rel(source, harness)
	if err != nil {
		return "", err
	}
	newHarness := filepath.Join(destination, relativeHarness)
	resolvedHarness, err := filepath.EvalSymlinks(newHarness)
	if err != nil {
		resolvedHarness = filepath.Clean(newHarness)
	}
	if resolvedHarness == harness {
		return "", fmt.Errorf("isolation audit failed: both sessions resolve one metasystem artifacts root")
	}
	if info, err := os.Stat(resolvedHarness); err != nil || !info.IsDir() {
		return "", fmt.Errorf("isolation audit failed: the new harness is absent: %s", resolvedHarness)
	}
	return resolvedHarness, nil
}

// containsParentStep reports whether a relative path names a parent
// directory anywhere; such a manifest entry could write outside the
// worktree and is refused outright.
func containsParentStep(path string) bool {
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		if part == ".." {
			return true
		}
	}
	return false
}

// copyFile copies one regular file, following symlinks at the source
// and preserving its permissions and modification time.
func copyFile(from, to string) error {
	info, err := os.Stat(from)
	if err != nil {
		return err
	}
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(to, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Chtimes(to, info.ModTime(), info.ModTime())
}

// copyTree copies a directory recursively, following symlinks so the
// copy holds real content, and preserves directory permissions and
// modification times after the children land.
func copyTree(from, to string) error {
	info, err := os.Stat(from)
	if err != nil {
		return err
	}
	if err := os.Mkdir(to, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(from)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		childFrom := filepath.Join(from, entry.Name())
		childTo := filepath.Join(to, entry.Name())
		childInfo, err := os.Stat(childFrom)
		if err != nil {
			return err
		}
		if childInfo.IsDir() {
			err = copyTree(childFrom, childTo)
		} else {
			err = copyFile(childFrom, childTo)
		}
		if err != nil {
			return err
		}
	}
	if err := os.Chmod(to, info.Mode().Perm()); err != nil {
		return err
	}
	return os.Chtimes(to, info.ModTime(), info.ModTime())
}
