package gittree

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestChangedLines(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want int64
	}{
		{name: "adds-deletes", data: []byte("2\t3\tfile.txt\x00"), want: 5},
		{name: "empty", data: nil, want: 0},
		{name: "binary", data: []byte("-\t-\tasset.bin\x00"), want: 0},
		{name: "odd-paths", data: []byte("4\t5\ta\tname\n.txt\x00"), want: 9},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := changedLinesFromNumstat(test.data)
			if err != nil || got != test.want {
				t.Fatalf("changedLinesFromNumstat() = %d, %v; want %d, nil", got, err, test.want)
			}
		})
	}
}

func TestChangedLinesRejectsMalformedAndOverflow(t *testing.T) {
	t.Run("missing-nul", func(t *testing.T) {
		assertNumstatError(t, []byte("1\t2\tfile.txt"), "not NUL-terminated")
	})
	t.Run("missing-path", func(t *testing.T) {
		assertNumstatError(t, []byte("1\t2\t\x00"), "empty pathname")
	})
	t.Run("one-tab", func(t *testing.T) {
		assertNumstatError(t, []byte("1\t2\x00"), "no deleted count")
	})
	t.Run("count", func(t *testing.T) {
		for _, data := range [][]byte{
			[]byte("x\t1\tfile\x00"), []byte("1\tx\tfile\x00"),
			[]byte("+1\t0\tfile\x00"), []byte("-1\t0\tfile\x00"),
			[]byte("\t1\tfile\x00"), []byte("9223372036854775808\t0\tfile\x00"),
		} {
			assertNumstatError(t, data, "invalid")
		}
	})
	t.Run("mixed-binary", func(t *testing.T) {
		for _, data := range [][]byte{[]byte("-\t1\tfile\x00"), []byte("1\t-\tfile\x00")} {
			assertNumstatError(t, data, "mixes binary and numeric")
		}
	})
	t.Run("row-overflow", func(t *testing.T) {
		assertNumstatError(t, []byte("9223372036854775807\t1\tfile\x00"), "record 1 count overflow")
	})
	t.Run("total-overflow", func(t *testing.T) {
		data := []byte("9223372036854775807\t0\tfirst\x001\t0\tsecond\x00")
		assertNumstatError(t, data, "total count overflow at record 2")
	})
}

func assertNumstatError(t *testing.T, data []byte, want string) {
	t.Helper()
	if got, err := changedLinesFromNumstat(data); err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("changedLinesFromNumstat(%q) = %d, %v; want error containing %q", data, got, err, want)
	}
}

func TestChangedLinesUsesSuppliedTrees(t *testing.T) {
	t.Run("root", func(t *testing.T) {
		f := newTreeFixture(t)
		from := f.git("rev-parse", "HEAD^{tree}")
		f.write("README.md", "changed\nsecond\n")
		to := f.snapshot()
		f.write("README.md", "later\nworktree\ncontent\n")
		assertWorkspaceChangedLines(t, f.w, from, to, 3)
	})
	t.Run("nested", func(t *testing.T) {
		f := newTreeFixture(t)
		f.write("nested/file.txt", "base\n")
		f.git("add", ".")
		f.commit("add nested workspace")
		from := f.git("rev-parse", "HEAD^{tree}")
		f.write("nested/file.txt", "changed\nsecond\n")
		to := f.snapshot()
		f.write("nested/file.txt", "later\nworktree\ncontent\n")
		assertWorkspaceChangedLines(t, Workspace{Dir: filepath.Join(f.w.Dir, "nested")}, from, to, 3)
	})
	t.Run("rename", func(t *testing.T) {
		f := newTreeFixture(t)
		f.write("old.txt", "one\ntwo\n")
		f.git("add", ".")
		f.commit("add rename source")
		from := f.git("rev-parse", "HEAD^{tree}")
		if err := os.Rename(filepath.Join(f.w.Dir, "old.txt"), filepath.Join(f.w.Dir, "new.txt")); err != nil {
			t.Fatal(err)
		}
		assertWorkspaceChangedLines(t, f.w, from, f.snapshot(), 4)
	})
}

func assertWorkspaceChangedLines(t *testing.T, w Workspace, from, to string, want int64) {
	t.Helper()
	got, err := w.ChangedLines(from, to)
	if err != nil || got != want {
		t.Fatalf("ChangedLines(%s, %s) = %d, %v; want %d, nil", from, to, got, err, want)
	}
}

func TestChangedLinesCommandPins(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "argv")
	shim := filepath.Join(dir, "git")
	script := "#!/bin/sh\n: > \"$NUMSTAT_ARGV\"\nfor arg do\n  printf '%s\\n' \"$arg\" >> \"$NUMSTAT_ARGV\"\ndone\n"
	if err := os.WriteFile(shim, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("NUMSTAT_ARGV", logPath)
	if got, err := (Workspace{Dir: dir}).ChangedLines("from-tree", "to-tree"); err != nil || got != 0 {
		t.Fatalf("ChangedLines with recording git = %d, %v; want 0, nil", got, err)
	}
	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	argv := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
	diff := -1
	for i, arg := range argv {
		if arg == "diff" {
			diff = i
			break
		}
	}
	if diff < 0 {
		t.Fatalf("runner argv has no diff command: %q", argv)
	}
	got := argv[diff:]
	want := []string{"diff", "--numstat", "-z", "--no-renames", "--no-ext-diff", "--no-textconv", "--no-color", "--ignore-submodules=none", "from-tree", "to-tree", "--"}
	for _, pin := range []struct {
		name  string
		index int
	}{
		{name: "numstat", index: 1}, {name: "nul", index: 2},
		{name: "no-renames", index: 3}, {name: "no-ext-diff", index: 4},
		{name: "no-textconv", index: 5}, {name: "no-color", index: 6},
		{name: "submodules", index: 7}, {name: "end-options", index: 10},
	} {
		t.Run(pin.name, func(t *testing.T) {
			if !reflect.DeepEqual(got, want) || got[pin.index] != want[pin.index] {
				t.Fatalf("diff argv = %q; want %q", got, want)
			}
		})
	}
}

func TestChangedLinesHostileDiffConfig(t *testing.T) {
	f := newTreeFixture(t)
	hostile := filepath.Join(t.TempDir(), "hostile-diff")
	if err := os.WriteFile(hostile, []byte("#!/bin/sh\nexit 77\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	f.write(".gitattributes", "README.md diff=hostile\n")
	f.write("old.txt", "rename\n")
	f.git("add", ".gitattributes", "old.txt")
	f.commit("add diff attributes")
	f.git("config", "diff.external", hostile)
	f.git("config", "diff.hostile.command", hostile)
	f.git("config", "diff.hostile.textconv", hostile)
	f.git("config", "diff.renames", "true")
	f.git("config", "color.ui", "always")
	f.git("config", "diff.ignoreSubmodules", "all")
	from := f.git("rev-parse", "HEAD^{tree}")
	f.write("README.md", "changed\nsecond\n")
	if err := os.Rename(filepath.Join(f.w.Dir, "old.txt"), filepath.Join(f.w.Dir, "new.txt")); err != nil {
		t.Fatal(err)
	}
	assertWorkspaceChangedLines(t, f.w, from, f.snapshot(), 5)

	t.Run("replace-ref", func(t *testing.T) {
		f := newTreeFixture(t)
		f.write("README.md", "base one\nbase two\n")
		f.git("add", "README.md")
		f.commit("expand readme")
		from := f.git("rev-parse", "HEAD^{tree}")
		originalBlob := f.git("rev-parse", "HEAD:README.md")
		f.write("README.md", "changed\n")
		to := f.snapshot()

		replacementPath := filepath.Join(t.TempDir(), "replacement")
		if err := os.WriteFile(replacementPath, []byte(strings.Repeat("replacement\n", 10)), 0o644); err != nil {
			t.Fatal(err)
		}
		replacementBlob := f.git("hash-object", "-w", replacementPath)
		f.git("replace", originalBlob, replacementBlob)
		assertWorkspaceChangedLines(t, f.w, from, to, 3)
	})

	t.Run("config-environment", func(t *testing.T) {
		f := newTreeFixture(t)
		from := f.git("rev-parse", "HEAD^{tree}")
		f.write("README.md", "changed\nsecond\n")
		to := f.snapshot()
		t.Setenv("GIT_CONFIG_PARAMETERS", "'core.bigfilethreshold'='1'")
		assertWorkspaceChangedLines(t, f.w, from, to, 3)
	})
}

func TestChangedLinesGitFailure(t *testing.T) {
	f := newTreeFixture(t)
	if got, err := f.w.ChangedLines("not-a-tree", f.git("rev-parse", "HEAD^{tree}")); err == nil || got != 0 {
		t.Fatalf("ChangedLines with invalid tree = %d, %v; want 0 and an error", got, err)
	}
}
