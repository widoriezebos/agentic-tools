package gittree

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func disjointFixtureTrees(t *testing.T, base, chain, main []byte, chainMode, mainMode os.FileMode) (Workspace, string, string, string) {
	t.Helper()
	f := newTreeFixture(t)
	path := filepath.Join(f.w.Dir, "source.txt")
	if err := os.WriteFile(path, base, 0o644); err != nil {
		t.Fatal(err)
	}
	f.git("add", "source.txt")
	f.commit("base source")
	baseTree, err := f.w.HeadTree()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, chain, chainMode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, chainMode); err != nil {
		t.Fatal(err)
	}
	chainTree := f.snapshot()
	if err := os.WriteFile(path, main, mainMode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mainMode); err != nil {
		t.Fatal(err)
	}
	mainTree := f.snapshot()
	return f.w, baseTree, chainTree, mainTree
}

func TestDisjointMergeProof(t *testing.T) {
	successes := []struct {
		name, base, chain, main, want        string
		chainOld, chainNew, mainOld, mainNew HunkRange
	}{
		{"separated replacements", "one\ntwo\nthree\nfour\nfive\n", "ONE\ntwo\nthree\nfour\nfive\n", "one\ntwo\nthree\nfour\nFIVE\n", "ONE\ntwo\nthree\nfour\nFIVE\n", HunkRange{1, 1}, HunkRange{1, 1}, HunkRange{5, 1}, HunkRange{5, 1}},
		{"middle insertion with section suffix", "alpha\nbeta\ngamma\ndelta\n", "alpha\nchain\nbeta\ngamma\ndelta\n", "alpha\nbeta\ngamma\nmain\ndelta\n", "alpha\nchain\nbeta\ngamma\nmain\ndelta\n", HunkRange{1, 0}, HunkRange{2, 1}, HunkRange{3, 0}, HunkRange{4, 1}},
		{"unterminated final line", "alpha\nbeta\ngamma\nlast", "ALPHA\nbeta\ngamma\nlast", "alpha\nbeta\ngamma\nLAST", "ALPHA\nbeta\ngamma\nLAST", HunkRange{1, 1}, HunkRange{1, 1}, HunkRange{4, 1}, HunkRange{4, 1}},
		{"repeated line alignment", "same\nsame\nkeep\nsame\nsame\n", "CHAIN\nsame\nkeep\nsame\nsame\n", "same\nsame\nkeep\nsame\nMAIN\n", "CHAIN\nsame\nkeep\nsame\nMAIN\n", HunkRange{1, 1}, HunkRange{1, 1}, HunkRange{5, 1}, HunkRange{5, 1}},
	}
	for _, test := range successes {
		t.Run(test.name, func(t *testing.T) {
			w, base, chain, main := disjointFixtureTrees(t, []byte(test.base), []byte(test.chain), []byte(test.main), 0o644, 0o644)
			// Hostile repository configuration cannot influence the canonical
			// empty-directory blob comparison.
			_, _ = w.git(nil, "config", "diff.algorithm", "patience")
			_, _ = w.git(nil, "config", "diff.indentHeuristic", "true")
			result, err := w.DisjointMerge(base, chain, main)
			if err != nil {
				t.Fatal(err)
			}
			got, present, err := w.FileAt(result.MergedTree, "source.txt")
			if err != nil || !present {
				t.Fatalf("merged file missing: %v", err)
			}
			if string(got) != test.want {
				t.Fatalf("merged bytes = %q, want %q", got, test.want)
			}
			version, err := w.git(nil, "--version")
			if err != nil {
				t.Fatal(err)
			}
			wantChain := HunkManifestEntry{Path: "source.txt", Side: "chain", OldRange: test.chainOld, NewRange: test.chainNew}
			wantMain := HunkManifestEntry{Path: "source.txt", Side: "main", OldRange: test.mainOld, NewRange: test.mainNew}
			if result.GitVersion != strings.TrimSpace(string(version)) || len(result.Manifest) != 2 ||
				result.Manifest[0] != wantChain || result.Manifest[1] != wantMain {
				t.Fatalf("proof facts are incomplete: %+v", result)
			}
		})
	}

	overlaps := []struct {
		name, base, chain, main string
		chainRange, mainRange   HunkRange
	}{
		{"same line", "a\nb\nc\n", "A\nb\nc\n", "X\nb\nc\n", HunkRange{Start: 1, Count: 1}, HunkRange{Start: 1, Count: 1}},
		{"same gap", "a\nb\nc\n", "a\nchain\nb\nc\n", "a\nmain\nb\nc\n", HunkRange{Start: 1, Count: 0}, HunkRange{Start: 1, Count: 0}},
		{"deletion boundary insertion", "a\nb\nc\nd\n", "a\nd\n", "a\nb\nc\nmain\nd\n", HunkRange{Start: 2, Count: 2}, HunkRange{Start: 3, Count: 0}},
		{"adjacent consuming hunks", "a\nb\nc\nd\n", "A\nb\nc\nd\n", "a\nB\nc\nd\n", HunkRange{Start: 1, Count: 1}, HunkRange{Start: 2, Count: 1}},
	}
	for _, test := range overlaps {
		t.Run(test.name, func(t *testing.T) {
			w, base, chain, main := disjointFixtureTrees(t, []byte(test.base), []byte(test.chain), []byte(test.main), 0o644, 0o644)
			_, err := w.DisjointMerge(base, chain, main)
			var typed *DisjointMergeError
			if !errors.As(err, &typed) || typed.Kind != "overlap" || typed.Path != "source.txt" ||
				typed.ChainRange != test.chainRange || typed.MainRange != test.mainRange {
				t.Fatalf("overlap was not typed with both ranges: %#v", err)
			}
		})
	}

	t.Run("temporary comparison directory inside repository", func(t *testing.T) {
		w, _, _, _ := disjointFixtureTrees(t, []byte("a\nb\n"), []byte("A\nb\n"), []byte("a\nB\n"), 0o644, 0o644)
		t.Setenv("TMPDIR", w.Dir)
		_, _, err := w.canonicalBlobDiff([]byte("a\n"), []byte("A\n"))
		if err == nil || !strings.Contains(err.Error(), "temporary directory is inside a Git repository") {
			t.Fatalf("repository-local temporary comparison was not refused: %v", err)
		}
	})

	t.Run("non text", func(t *testing.T) {
		base := []byte{'a', '\n', 0xff, 0xfe, '\n', 'c', '\n', 'd', '\n'}
		chain := []byte{'A', '\n', 0xff, 0xfe, '\n', 'c', '\n', 'd', '\n'}
		main := []byte{'a', '\n', 0xff, 0xfe, '\n', 'c', '\n', 'D', '\n'}
		if IsRecertifiableText(base) {
			t.Fatal("newline-delimited FF/FE bytes passed IsRecertifiableText")
		}
		w, b, r, target := disjointFixtureTrees(t, base, chain, main, 0o644, 0o644)
		_, err := w.DisjointMerge(b, r, target)
		var typed *DisjointMergeError
		if !errors.As(err, &typed) || typed.Kind != "unproven" || typed.Detail != "non-text-blob" {
			t.Fatalf("non-text refusal = %#v", err)
		}
	})

	t.Run("mode", func(t *testing.T) {
		base := []byte("a\nb\nc\nd\n")
		w, b, r, target := disjointFixtureTrees(t, base, []byte("A\nb\nc\nd\n"), []byte("a\nb\nc\nD\n"), 0o755, 0o644)
		_, err := w.DisjointMerge(b, r, target)
		var typed *DisjointMergeError
		if !errors.As(err, &typed) || typed.Kind != "unproven" || typed.Detail != "unsupported-mode" {
			t.Fatalf("mode refusal = %#v", err)
		}
	})

	t.Run("malformed canonical diff", func(t *testing.T) {
		w, _, _, _ := disjointFixtureTrees(t, []byte("a\nb\n"), []byte("A\nb\n"), []byte("a\nB\n"), 0o644, 0o644)
		base, changed := []byte("a\nb\n"), []byte("A\nb\n")
		diff, _, err := w.canonicalBlobDiff(base, changed)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := parseCanonicalEdits(append(diff, []byte("unexpected-tail\n")...), base, changed); err == nil {
			t.Fatal("malformed canonical diff tail was accepted")
		}
	})

	t.Run("recovery anchor names an exact tree object", func(t *testing.T) {
		w, baseTree, _, _ := disjointFixtureTrees(t, []byte("a\nb\n"), []byte("A\nb\n"), []byte("a\nB\n"), 0o644, 0o644)
		ref := "refs/metasystem/landing/recertifications/test/source"
		if err := w.AnchorRef(ref, baseTree, "tree"); err != nil {
			t.Fatal(err)
		}
		if got, err := w.ResolveTree(ref); err != nil || got != baseTree {
			t.Fatalf("tree anchor = %q, %v; want %q", got, err, baseTree)
		}
		head, err := w.ResolveRef("HEAD")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.git(nil, "update-ref", ref, head); err != nil {
			t.Fatal(err)
		}
		if _, err := w.ResolveTree(ref); err == nil {
			t.Fatal("commit-valued recovery anchor was accepted as a tree anchor")
		}
	})
}
