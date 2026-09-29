package pathclass

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// These tests port scripts/agents/path-class-fixtures.sh's manifest legs:
// TestPathClassVerbAnswersFromManifest (resolution after discovery, with the
// repository top and the executable supplied by the test instead of Git) and
// TestDeletedListsHaveNoReader (the static reader scan over the manifest's
// behavior roots).

func pathClassBedWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// deletedListTokens name the deleted landing tables. They are written by
// concatenation so this file stays clean under its own scan.
var deletedListTokens = []string{"register-carriage-" + "paths", "instruction-bearing-" + "paths", "neverDirect" + "Fix"}

func readsDeletedList(text string) bool {
	for _, token := range deletedListTokens {
		if strings.Contains(text, token) {
			return true
		}
	}
	return false
}

type deletedListSearch struct {
	excludeDirs  map[string]bool
	excludeFiles map[string]bool
}

var (
	// installReaderSearch skips review records, the journey narrative and any
	// installed frontend dependency tree.
	installReaderSearch = deletedListSearch{
		excludeDirs:  map[string]bool{"reviews": true, "node_modules": true},
		excludeFiles: map[string]bool{"journey.md": true},
	}
	repoReaderSearch = deletedListSearch{excludeDirs: map[string]bool{"node_modules": true}}
)

// readers returns every "path:line:text" hit of deletedListTokens under the
// given roots, relative to from. Text files only; symbolic links met during
// the walk are not followed.
func (s deletedListSearch) readers(from string, roots []string) ([]string, error) {
	var hits []string
	scan := func(path string) error {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if bytes.IndexByte(data, 0) >= 0 || !readsDeletedList(string(data)) {
			return nil
		}
		relative, err := filepath.Rel(from, path)
		if err != nil {
			return err
		}
		for number, line := range strings.Split(string(data), "\n") {
			if readsDeletedList(line) {
				hits = append(hits, filepath.ToSlash(relative)+":"+strconv.Itoa(number+1)+":"+line)
			}
		}
		return nil
	}
	for _, root := range roots {
		start := filepath.Join(from, filepath.FromSlash(root))
		info, err := os.Stat(start)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			if !s.excludeFiles[filepath.Base(start)] {
				if err := scan(start); err != nil {
					return nil, err
				}
			}
			continue
		}
		err = filepath.WalkDir(start, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				if path != start && s.excludeDirs[entry.Name()] {
					return filepath.SkipDir
				}
				return nil
			}
			if !entry.Type().IsRegular() || s.excludeFiles[entry.Name()] {
				return nil
			}
			return scan(path)
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(hits)
	return hits, nil
}

// behaviorRoots lists the manifest's behavior rows of one namespace that
// exist under base.
func behaviorRoots(t *testing.T, manifest []byte, namespace, base string) []string {
	t.Helper()
	var roots []string
	for _, line := range strings.Split(string(manifest), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[1] != string(Behavior) || !strings.HasPrefix(fields[0], namespace+":") {
			continue
		}
		key := strings.TrimPrefix(fields[0], namespace+":")
		if _, err := os.Lstat(filepath.Join(base, filepath.FromSlash(key))); err == nil {
			roots = append(roots, key)
		} else if !errors.Is(err, fs.ErrNotExist) {
			t.Fatal(err)
		}
	}
	return roots
}

func TestDeletedListsHaveNoReader(t *testing.T) {
	t.Parallel()
	// The exclusion carries its own verdict: a planted dependency tree is not
	// a reader, while a planted ordinary file is.
	planted := t.TempDir()
	token := "neverDirect" + "Fix\n"
	pathClassBedWrite(t, filepath.Join(planted, "cmd", "seen.txt"), token)
	pathClassBedWrite(t, filepath.Join(planted, "internal", "x", "node_modules", "p", "notes.txt"), token)
	hits, err := installReaderSearch.readers(planted, []string{"cmd", "internal"})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || !strings.HasPrefix(hits[0], "cmd/seen.txt:") {
		t.Fatalf("the reader search no longer excludes an installed dependency tree: %q", hits)
	}

	installation, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	manifest := Source()
	installRoots := behaviorRoots(t, manifest, "install", installation)
	if len(installRoots) == 0 {
		t.Fatal("no installation behavior root exists on disk")
	}
	hits, err = installReaderSearch.readers(installation, installRoots)
	if err != nil {
		t.Fatalf("the installation behavior source search itself failed: %v", err)
	}
	if len(hits) != 0 {
		t.Fatalf("an installation behavior source still reads a deleted table:\n%s", strings.Join(hits, "\n"))
	}

	repository := filepath.Dir(installation)
	repoRoots := behaviorRoots(t, manifest, "repo", repository)
	if len(repoRoots) == 0 {
		return
	}
	hits, err = repoReaderSearch.readers(repository, repoRoots)
	if err != nil {
		t.Fatalf("the repository behavior source search itself failed: %v", err)
	}
	if len(hits) != 0 {
		t.Fatalf("a repository behavior source still reads a deleted table:\n%s", strings.Join(hits, "\n"))
	}
}
