package branch

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// TestChangeDigestGitAdapter checks nested snapshot paths and Git's patch bytes
// when the same edit is applied to different bases.
func TestChangeDigestGitAdapter(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	git := func(args ...string) string { return mergeDriverGit(t, root, args...) }
	write := func(path, body string) { mergeDriverWrite(t, root, "metasystem/"+path, body) }
	git("init", "-q")
	git("config", "user.name", "fixture")
	git("config", "user.email", "fixture@example.invalid")
	var lines []string
	for i := 1; i <= 30; i++ {
		lines = append(lines, fmt.Sprintf("line %02d\n", i))
	}
	source := strings.Join(lines, "")
	write("source.txt", source)
	write("generated/out.txt", source)
	write("testing.json", `{"unknown":{"future":true},"generated":[{"paths":["generated/**"],"command":["make"]}]}`)
	git("add", ".")
	git("commit", "-qm", "base")
	base := git("rev-parse", "HEAD")
	write("source.txt", strings.ReplaceAll(source, "line 15\n", "unit edit\n"))
	write("generated/out.txt", strings.ReplaceAll(source, "line 15\n", "generated edit\n"))
	git("commit", "-qam", "unit")
	unit := git("rev-parse", "HEAD")
	repo := filepath.Join(root, "metasystem")
	reads := gitAttestationReads{}
	digest, err := changeDigestWithReads(reads, repo, unit)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := UnitDigest(repo, unit)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, old string
		equal     bool
	}{
		{"distant base change", "line 01\n", true},
		{"nearby context change", "line 13\n", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			git("checkout", "-q", "--detach", base)
			mainSource := strings.ReplaceAll(source, test.old, "main changed\n")
			mainGenerated := strings.ReplaceAll(source, "line 14\n", "main generated\n")
			write("source.txt", mainSource)
			write("generated/out.txt", mainGenerated)
			git("commit", "-qam", "main change")
			write("source.txt", strings.ReplaceAll(mainSource, "line 15\n", "unit edit\n"))
			write("generated/out.txt", strings.ReplaceAll(mainGenerated, "line 15\n", "generated edit\n"))
			git("commit", "-qam", "unit")
			rebased := git("rev-parse", "HEAD")
			got, err := changeDigestWithReads(reads, repo, rebased)
			if err != nil || (got == digest) != test.equal {
				t.Fatalf("change equality = %t, want %t: %v", got == digest, test.equal, err)
			}
			if test.equal {
				var normalized [2][]byte
				for i, commit := range []string{unit, rebased} {
					patch, err := reads.ChangePatch(repo, commit)
					if err != nil {
						t.Fatal(err)
					}
					normalized[i], err = normalizeChangePatch(patch, func(string) bool { return false })
					if err != nil {
						t.Fatal(err)
					}
				}
				if bytes.Equal(normalized[0], normalized[1]) {
					t.Fatal("patches retaining generated files are equal across the base change")
				}
			}
			newRaw, err := UnitDigest(repo, rebased)
			if err != nil || newRaw == raw {
				t.Fatalf("rebased raw change = %s, old = %s: %v", newRaw, raw, err)
			}
		})
	}
	if _, err := reads.ChangePatch(repo, base); err == nil {
		t.Fatal("root commit yielded a change")
	}
}

func TestChangePatchGitAdapterIgnoresConfig(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	git := func(args ...string) string { return mergeDriverGit(t, root, args...) }
	git("init", "-q")
	git("config", "user.name", "fixture")
	git("config", "user.email", "fixture@example.invalid")
	var lines []string
	for i := 1; i <= 30; i++ {
		lines = append(lines, fmt.Sprintf("line %02d\n", i))
	}
	source := strings.ReplaceAll(strings.Join(lines, ""), "line 04\n", "\n")
	mergeDriverWrite(t, root, "metasystem/café.txt", source)
	mergeDriverWrite(t, root, "metasystem/a.txt", "base\n")
	git("add", ".")
	git("commit", "-qm", "base")
	changed := strings.ReplaceAll(source, "line 05\n", "first edit\n")
	changed = strings.ReplaceAll(changed, "line 15\n", "second edit\n")
	mergeDriverWrite(t, root, "metasystem/café.txt", changed)
	mergeDriverWrite(t, root, "metasystem/a.txt", "unit edit\n")
	git("commit", "-qam", "unit")
	commit := git("rev-parse", "HEAD")
	repo := filepath.Join(root, "metasystem")
	reads := gitAttestationReads{}
	want, err := reads.ChangePatch(repo, commit)
	if err != nil {
		t.Fatal(err)
	}
	mergeDriverWrite(t, root, "order", "metasystem/café.txt\nmetasystem/a.txt\n")
	for _, setting := range [][2]string{
		{"core.quotePath", "false"}, {"diff.suppressBlankEmpty", "true"},
		{"diff.algorithm", "histogram"}, {"diff.indentHeuristic", "false"},
		{"diff.interHunkContext", "10"}, {"diff.orderFile", filepath.Join(root, "order")},
		{"diff.noprefix", "true"}, {"diff.relative", "true"}, {"color.ui", "always"},
	} {
		git("config", "--local", setting[0], setting[1])
	}
	got, err := reads.ChangePatch(repo, commit)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("Git configuration changed patch bytes:\nwant:\n%s\ngot:\n%s", want, got)
	}
}
