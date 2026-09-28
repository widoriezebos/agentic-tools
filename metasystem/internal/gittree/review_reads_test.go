package gittree

import (
	"strings"
	"testing"
)

// The three reads the review room's desk needs (g1-s65 D4), against a stubbed
// Git: the per-file counts between two commits, one path's text diff, and the
// commits reachable from a ref whose message carries one exact line.

func TestFileCountsReadsEachPathAndMarksBinaries(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	w := Workspace{Dir: dir, RawSource: rawPortScript(t, dir, rawPortStep{
		args: []string{"diff", "--numstat", "-z", "--no-renames", "--no-ext-diff", "--no-textconv",
			"--no-color", "--ignore-submodules=none", "base", "tip", "--"},
		result: RawResult{Stdout: []byte("3\t1\tinternal/owner.go\x00-\t-\tdocs/shot.png\x000\t12\tgone.go\x00")},
	})}

	counts, err := w.FileCounts("base", "tip")

	if err != nil {
		t.Fatalf("file counts: %v", err)
	}
	want := []FileCount{
		{Path: "internal/owner.go", Added: 3, Deleted: 1},
		{Path: "docs/shot.png", Binary: true},
		{Path: "gone.go", Deleted: 12},
	}
	if len(counts) != len(want) {
		t.Fatalf("counts = %+v, want %+v", counts, want)
	}
	for at := range want {
		if counts[at] != want[at] {
			t.Fatalf("count %d = %+v, want %+v", at, counts[at], want[at])
		}
	}
}

func TestFileCountsRefusesWhatItCannotRead(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	w := Workspace{Dir: dir, RawSource: rawPortScript(t, dir,
		rawPortStep{result: RawResult{Stdout: []byte("3\t1\tno-terminator")}},
		rawPortStep{result: RawResult{Stdout: []byte("x\t1\tpath\x00")}},
		rawPortStep{result: RawResult{Stderr: []byte("bad revision\n"), ExitCode: 128}},
	)}
	for _, want := range []string{"not NUL-terminated", "invalid added count", "bad revision"} {
		if _, err := w.FileCounts("a", "b"); err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("file counts error = %v, want %q", err, want)
		}
	}
}

func TestPathDiffIsOnePathsTextDiff(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	patch := "diff --git a/x.go b/x.go\n@@ -1 +1 @@\n-old\n+new\n"
	w := Workspace{Dir: dir, RawSource: rawPortScript(t, dir, rawPortStep{
		args: []string{"diff", "--no-renames", "--unified=3", "--no-ext-diff", "--no-textconv", "--no-color",
			"--ignore-submodules=none", "--src-prefix=a/", "--dst-prefix=b/", "base", "tip", "--", "x.go"},
		result: RawResult{Stdout: []byte(patch)},
	})}

	got, err := w.PathDiff("base", "tip", "x.go")

	if err != nil || string(got) != patch {
		t.Fatalf("path diff = %q, %v", got, err)
	}
}

func TestCommitsCarryingReadsExactLinesOldestFirst(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	one, two, near := strings.Repeat("a", 40), strings.Repeat("b", 40), strings.Repeat("c", 40)
	log := one + "\x00land g1-s64\n\nGoal-Item: g1-s64\n\x1e" +
		near + "\x00land g1-s640\n\nGoal-Item: g1-s640\n\x1e" +
		two + "\x00second\n\nMachine: m1e\nGoal-Item: g1-s64\n\x1e"
	w := Workspace{Dir: dir, RawSource: rawPortScript(t, dir, rawPortStep{
		args: []string{"log", "--reverse", "--format=%H%x00%B%x1e", "--fixed-strings",
			"--grep=Goal-Item: g1-s64", "origin/main", "--"},
		result: RawResult{Stdout: []byte(log)},
	})}

	commits, err := w.CommitsCarrying("origin/main", "Goal-Item: g1-s64")

	if err != nil {
		t.Fatalf("commits carrying: %v", err)
	}
	if strings.Join(commits, " ") != one+" "+two {
		t.Fatalf("commits = %v, want the two exact carriers oldest first", commits)
	}
}

func TestCommitsCarryingRefusesAMalformedLog(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	w := Workspace{Dir: dir, RawSource: rawPortScript(t, dir,
		rawPortStep{result: RawResult{Stdout: []byte("not-a-commit\x00body\x1e")}},
		rawPortStep{result: RawResult{Stderr: []byte("unknown revision\n"), ExitCode: 128}},
	)}
	for _, want := range []string{"not a commit", "unknown revision"} {
		if _, err := w.CommitsCarrying("main", "Goal-Item: g"); err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("commits carrying error = %v, want %q", err, want)
		}
	}
}
