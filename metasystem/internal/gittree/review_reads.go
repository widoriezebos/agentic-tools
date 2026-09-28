package gittree

import (
	"bytes"
	"fmt"
	"strings"
)

// The reads a human's review of a goal's built work makes (g1-s65 D4): what
// changed between two commits file by file, one file's text diff, and which
// commits of a branch carry a trailer line. Each is one bounded invocation with
// the package's config pins, so the same two commits read the same way in
// every repository.

// FileCount is one path's change between two commits. A binary file counts no
// lines, and says so.
type FileCount struct {
	Path    string
	Added   int64
	Deleted int64
	Binary  bool
}

// FileCounts answers every path that differs between two commits, in Git's
// order, with its added and deleted line counts.
func (w Workspace) FileCounts(from, to string) ([]FileCount, error) {
	raw, err := w.git(nil, "diff", "--numstat", "-z", "--no-renames",
		"--no-ext-diff", "--no-textconv", "--no-color", "--ignore-submodules=none",
		from, to, "--")
	if err != nil {
		return nil, fmt.Errorf("gittree file counts: %w", err)
	}
	if len(raw) == 0 {
		return []FileCount{}, nil
	}
	if raw[len(raw)-1] != 0 {
		return nil, fmt.Errorf("gittree file counts: numstat output is not NUL-terminated")
	}
	counts := []FileCount{}
	for index, record := range bytes.Split(raw[:len(raw)-1], []byte{0}) {
		fields := bytes.SplitN(record, []byte{'\t'}, 3)
		if len(fields) != 3 || len(fields[2]) == 0 {
			return nil, fmt.Errorf("gittree file counts: numstat record %d is malformed", index+1)
		}
		count := FileCount{Path: string(fields[2])}
		if bytes.Equal(fields[0], []byte("-")) && bytes.Equal(fields[1], []byte("-")) {
			count.Binary = true
			counts = append(counts, count)
			continue
		}
		if count.Added, err = nonnegativeDecimal(fields[0]); err != nil {
			return nil, fmt.Errorf("gittree file counts: numstat record %d has invalid added count: %w", index+1, err)
		}
		if count.Deleted, err = nonnegativeDecimal(fields[1]); err != nil {
			return nil, fmt.Errorf("gittree file counts: numstat record %d has invalid deleted count: %w", index+1, err)
		}
		counts = append(counts, count)
	}
	return counts, nil
}

// PathDiff is one path's unified diff between two commits, as text, with the
// a/ and b/ prefixes forced and every driver a config could inject disabled.
func (w Workspace) PathDiff(from, to, path string) ([]byte, error) {
	patch, err := w.git(nil, "diff", "--no-renames", "--unified=3",
		"--no-ext-diff", "--no-textconv", "--no-color", "--ignore-submodules=none",
		"--src-prefix=a/", "--dst-prefix=b/", from, to, "--", path)
	if err != nil {
		return nil, fmt.Errorf("gittree path diff: %w", err)
	}
	return patch, nil
}

// CommitsCarrying answers the commits reachable from ref whose message carries
// exactly this line, oldest first. Git's grep is a substring match, so every
// message it finds is read again and kept only where one of its lines is the
// line itself: "Goal-Item: g1-s6" is not carried by a commit of g1-s64.
func (w Workspace) CommitsCarrying(ref, line string) ([]string, error) {
	raw, err := w.git(nil, "log", "--reverse", "--format=%H%x00%B%x1e", "--fixed-strings",
		"--grep="+line, ref, "--")
	if err != nil {
		return nil, fmt.Errorf("gittree commits carrying: %w", err)
	}
	commits := []string{}
	for _, entry := range strings.Split(string(raw), "\x1e") {
		entry = strings.TrimLeft(entry, "\n")
		if strings.TrimSpace(entry) == "" {
			continue
		}
		commit, message, found := strings.Cut(entry, "\x00")
		if !found || !treeID.MatchString(commit) {
			return nil, fmt.Errorf("gittree commits carrying: %q is not a commit", commit)
		}
		for _, said := range strings.Split(strings.ReplaceAll(message, "\r\n", "\n"), "\n") {
			if strings.TrimSpace(said) == line {
				commits = append(commits, commit)
				break
			}
		}
	}
	return commits, nil
}
