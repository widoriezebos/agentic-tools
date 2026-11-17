package goal

import (
	"bufio"
	"container/list"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
)

const committedTreeLimit = 16

type committedTreeKey struct {
	common, directory, commit, prefixes string
}

type committedTreeEntry struct {
	key   committedTreeKey
	files map[string][]byte
}

// Only complete, unreplaced object reads are retained. Returned maps and bytes
// are immutable; parsing remains the caller's job on every request.
type committedTreeMemo struct {
	mu           sync.Mutex
	entries      map[committedTreeKey]*list.Element
	lru          list.List
	reading      map[committedTreeKey]chan struct{}
	hits, misses uint64
}

var committedTrees committedTreeMemo

func (m *committedTreeMemo) read(ctx context.Context, key committedTreeKey, load func() (map[string][]byte, error)) (map[string][]byte, error) {
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		m.mu.Lock()
		if entry := m.entries[key]; entry != nil {
			m.hits++
			m.lru.MoveToFront(entry)
			files := entry.Value.(committedTreeEntry).files
			m.mu.Unlock()
			return files, nil
		}
		if done := m.reading[key]; done != nil {
			m.mu.Unlock()
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-done:
				continue
			}
		}
		if m.entries == nil {
			m.entries = make(map[committedTreeKey]*list.Element)
			m.reading = make(map[committedTreeKey]chan struct{})
		}
		done := make(chan struct{})
		m.reading[key] = done
		m.misses++
		m.mu.Unlock()

		files, err := load()
		if ctx.Err() != nil {
			err = ctx.Err()
		}
		m.mu.Lock()
		if err == nil {
			m.entries[key] = m.lru.PushFront(committedTreeEntry{key, files})
			if m.lru.Len() > committedTreeLimit {
				old := m.lru.Back()
				delete(m.entries, old.Value.(committedTreeEntry).key)
				m.lru.Remove(old)
			}
		}
		delete(m.reading, key)
		close(done)
		m.mu.Unlock()
		return files, err
	}
}

// Resolve the on-disk repository on every lookup without starting Git. A
// missing or unreadable layout bypasses the memo and leaves errors to Git.
func committedFilesKey(root, commit string, prefixes []string) (committedTreeKey, bool) {
	if len(commit) != 40 && len(commit) != 64 {
		return committedTreeKey{}, false
	}
	if _, err := hex.DecodeString(commit); err != nil {
		return committedTreeKey{}, false
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return committedTreeKey{}, false
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return committedTreeKey{}, false
	}
	for top := root; ; top = filepath.Dir(top) {
		gitdir := filepath.Join(top, ".git")
		info, err := os.Stat(gitdir)
		if err == nil {
			if !info.IsDir() {
				data, err := os.ReadFile(gitdir)
				pointer, ok := strings.CutPrefix(strings.TrimSpace(string(data)), "gitdir:")
				if err != nil || !ok || strings.TrimSpace(pointer) == "" {
					return committedTreeKey{}, false
				}
				gitdir = strings.TrimSpace(pointer)
				if !filepath.IsAbs(gitdir) {
					gitdir = filepath.Join(top, gitdir)
				}
			}
			common := gitdir
			if data, err := os.ReadFile(filepath.Join(gitdir, "commondir")); err == nil {
				common = strings.TrimSpace(string(data))
				if common == "" {
					return committedTreeKey{}, false
				}
				if !filepath.IsAbs(common) {
					common = filepath.Join(gitdir, common)
				}
			} else if !os.IsNotExist(err) {
				return committedTreeKey{}, false
			}
			common, err = filepath.EvalSymlinks(common)
			if err != nil {
				return committedTreeKey{}, false
			}
			directory, err := filepath.Rel(top, root)
			if err != nil {
				return committedTreeKey{}, false
			}
			paths := slices.Clone(prefixes)
			slices.Sort(paths)
			paths = slices.Compact(paths)
			return committedTreeKey{common, directory, strings.ToLower(commit), strings.Join(paths, "\x00")}, true
		}
		if !os.IsNotExist(err) || filepath.Dir(top) == top {
			return committedTreeKey{}, false
		}
	}
}

type committedGitRun func(input []byte, args ...string) (string, error)

func readCommittedFiles(ctx context.Context, root, commit string, prefixes []string, run committedGitRun) (map[string][]byte, error) {
	return committedTrees.files(ctx, root, commit, prefixes, run)
}

func (m *committedTreeMemo) files(ctx context.Context, root, commit string, prefixes []string, run committedGitRun) (map[string][]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	load := func() (map[string][]byte, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		args := append([]string{"--no-replace-objects", "ls-tree", "-r", "--name-only", commit, "--"}, prefixes...)
		out, err := run(nil, args...)
		if err != nil {
			return nil, fmt.Errorf("cannot list committed files at %s: %w", commit, err)
		}
		var paths []string
		var input strings.Builder
		for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
			if path := strings.TrimSpace(line); path != "" {
				paths = append(paths, path)
				input.WriteString(commit + ":./" + path + "\n")
			}
		}
		if len(paths) == 0 {
			return map[string][]byte{}, nil
		}
		out, err = run([]byte(input.String()), "--no-replace-objects", "cat-file", "--batch")
		if err != nil {
			return nil, fmt.Errorf("read ledger blobs at %s: %w", commit, err)
		}
		return parseCommittedBlobs(commit, paths, out)
	}
	if key, ok := committedFilesKey(root, commit, prefixes); ok {
		return m.read(ctx, key, load)
	}
	return load()
}

func parseCommittedBlobs(commit string, paths []string, out string) (map[string][]byte, error) {
	files := make(map[string][]byte, len(paths))
	reader := bufio.NewReader(strings.NewReader(out))
	for _, path := range paths {
		header, err := reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("cannot read %s at %s: malformed git cat-file header: %w", path, commit, err)
		}
		fields := strings.Fields(header)
		if len(fields) != 3 || fields[1] != "blob" {
			return nil, fmt.Errorf("cannot read %s at %s: unexpected git cat-file header %q", path, commit, strings.TrimSpace(header))
		}
		size, err := strconv.ParseInt(fields[2], 10, 64)
		if err != nil || size < 0 {
			return nil, fmt.Errorf("cannot read %s at %s: invalid git cat-file size %q", path, commit, fields[2])
		}
		content := make([]byte, size)
		if _, err := io.ReadFull(reader, content); err != nil {
			return nil, fmt.Errorf("cannot read %s at %s: truncated git cat-file content: %w", path, commit, err)
		}
		if separator, err := reader.ReadByte(); err != nil || separator != '\n' {
			return nil, fmt.Errorf("cannot read %s at %s: malformed git cat-file separator", path, commit)
		}
		files[path] = content
	}
	return files, nil
}
