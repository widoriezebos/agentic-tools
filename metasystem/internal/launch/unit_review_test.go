package launch

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// resultGit answers the snapshot's Git calls with a fixed full-id raw
// result and records every argument vector.
type resultGit struct {
	root, index string
	raw         string
	calls       [][]string
}

func (g *resultGit) Run(_ string, _ []string, args ...string) ([]byte, error) {
	g.calls = append(g.calls, args)
	joined := strings.Join(args, " ")
	switch {
	case joined == "rev-parse --show-toplevel":
		return []byte(g.root + "\n"), nil
	case joined == "rev-parse HEAD":
		return []byte(strings.Repeat("1", 40) + "\n"), nil
	case strings.HasSuffix(joined, "--git-path index"):
		return []byte(g.index + "\n"), nil
	case strings.HasSuffix(joined, "--git-path objects"):
		return []byte(filepath.Join(g.root, "objects") + "\n"), nil
	case strings.HasPrefix(joined, "diff --cached --raw"):
		return []byte(g.raw), nil
	}
	return nil, nil
}

// TestUnitResultExactRepresentation (CONN-R1-F4/F5): the retained result
// is Git's NUL-delimited raw diff with full object ids, so two results whose
// object ids share an abbreviation differ, and every path is parsed as its
// exact bytes, including non-ASCII, tab, quote, space and rename pairs.
func TestUnitResultExactRepresentation(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	index := filepath.Join(root, "index")
	os.WriteFile(index, []byte("index"), 0o600)
	shared := "abcdef0123456"
	entry := func(tail, path string) string {
		return ":100644 100644 " + strings.Repeat("0", 40) + " " + shared + tail + " M\x00" + path + "\x00"
	}
	first := &resultGit{root: root, index: index, raw: entry(strings.Repeat("a", 27), "café.txt")}
	second := &resultGit{root: root, index: index, raw: entry(strings.Repeat("b", 27), "café.txt")}
	var results []string
	for _, git := range []*resultGit{first, second} {
		_, result, err := (&UnitRunner{Git: git, Root: root}).WorktreeResult(root)
		if err != nil {
			t.Fatal(err)
		}
		results = append(results, result)
		asked := false
		for _, call := range git.calls {
			if call[0] == "diff" && slices.Contains(call, "--raw") {
				asked = slices.Contains(call, "-z") && slices.Contains(call, "--no-abbrev")
			}
		}
		if !asked {
			t.Fatalf("the snapshot must ask for NUL-delimited full object ids: %v", git.calls)
		}
	}
	if results[0] == results[1] {
		t.Fatal("results sharing an abbreviated object id must not compare equal")
	}
	raw := entry(strings.Repeat("c", 27), "café.txt") + entry(strings.Repeat("d", 27), "tab\tname \"quoted\".txt") +
		":100644 100644 " + strings.Repeat("0", 40) + " " + strings.Repeat("e", 40) + " R087\x00old name.txt\x00new name.txt\x00"
	paths, err := UnitResultPaths(raw)
	if err != nil || !slices.Equal(paths, []string{"café.txt", "tab\tname \"quoted\".txt", "old name.txt", "new name.txt"}) {
		t.Fatalf("exact paths: %q %v", paths, err)
	}
	if _, err := UnitResultPaths(":100644 100644 x y M\x00"); err == nil {
		t.Fatal("a truncated raw entry must be refused")
	}
}
