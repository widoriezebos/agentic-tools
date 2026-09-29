// Package placement recognizes the trees that never belong under an
// evidence root (engine-owns-disk-lifetimes Part B, 3.12 "Placement
// rules", R21): caches and source copies, by their shape. The
// classification is for reports and refusal texts only; nothing is removed
// or skipped by a name match.
package placement

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// The two misplacement kinds a report names.
const (
	Cache      = "cache-under-evidence"
	SourceCopy = "source-copy-under-evidence"
)

// Placement is what a tree is when it is misplaced under an evidence root:
// its kind and a person's words for its shape. The zero value is a tree
// that is neither.
type Placement struct {
	Kind  string `json:"kind,omitempty"`
	Shape string `json:"shape,omitempty"`
}

const (
	goCacheBanner          = "This directory holds cached build artifacts from the Go build system."
	staticcheckCacheBanner = "This directory holds cached build artifacts from staticcheck."
)

// Of classifies one directory (3.12): a Go build cache (its README
// banner with trim.txt), a module cache (cache/download holding an @v
// directory), a staticcheck cache (its README banner), a directory named
// gocache* or gomodcache*; or a source copy (a .git entry, or a
// metasystem/metasystem.conf under it). Anything else, and anything that is
// not a directory, is the zero Placement.
func Of(path string) Placement {
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() {
		return Placement{}
	}
	name := strings.ToLower(filepath.Base(path))
	switch {
	case strings.HasPrefix(name, "gocache"), strings.HasPrefix(name, "gomodcache"):
		return Placement{Kind: Cache, Shape: "a directory named like a Go cache"}
	case readmeBanner(path, goCacheBanner) && present(filepath.Join(path, "trim.txt")):
		return Placement{Kind: Cache, Shape: "a Go build cache"}
	case readmeBanner(path, staticcheckCacheBanner):
		return Placement{Kind: Cache, Shape: "a staticcheck cache"}
	case moduleCache(path):
		return Placement{Kind: Cache, Shape: "a Go module cache"}
	case present(filepath.Join(path, ".git")):
		return Placement{Kind: SourceCopy, Shape: "a git repository or worktree"}
	case present(filepath.Join(path, "metasystem", "metasystem.conf")):
		return Placement{Kind: SourceCopy, Shape: "a copy of a metasystem checkout"}
	}
	return Placement{}
}

func present(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func readmeBanner(dir, banner string) bool {
	file, err := os.Open(filepath.Join(dir, "README"))
	if err != nil {
		return false
	}
	defer file.Close()
	head := make([]byte, 256)
	count, _ := file.Read(head)
	return bytes.HasPrefix(head[:count], []byte(banner))
}

// moduleCache reports cache/download holding an @v directory within a few
// levels (a module path's elements).
func moduleCache(dir string) bool {
	download := filepath.Join(dir, "cache", "download")
	if info, err := os.Lstat(download); err != nil || !info.IsDir() {
		return false
	}
	found := false
	_ = filepath.WalkDir(download, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || found {
			return filepath.SkipDir
		}
		if entry.IsDir() && entry.Name() == "@v" {
			found = true
			return filepath.SkipAll
		}
		if entry.IsDir() && strings.Count(strings.TrimPrefix(path, download), string(filepath.Separator)) >= 6 {
			return filepath.SkipDir
		}
		return nil
	})
	return found
}
