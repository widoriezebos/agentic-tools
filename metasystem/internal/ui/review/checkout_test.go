package review

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// The shaping desk's source read (g1-s67 D2, §6): the checkout's own files as
// they stand, uncommitted edits included, under the checkout root the document
// reader opens, with Source's path check, binary refusal and bounds.

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", dir}, args...)...)
	command.Env = gittree.ScrubbedEnviron()
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAShapingDeskReadsTheCheckoutAsItStands(t *testing.T) {
	t.Parallel()
	checkout := t.TempDir()
	runGit(t, checkout, "init", "-q", "-b", "main")
	runGit(t, checkout, "config", "user.name", "fixture")
	runGit(t, checkout, "config", "user.email", "fixture@example.invalid")
	writeFile(t, filepath.Join(checkout, "internal", "owner.go"), "package owner\n\nfunc lock() {}\n")
	runGit(t, checkout, "add", ".")
	runGit(t, checkout, "commit", "-qm", "base")
	// Edited and not committed: the desk shows what the checkout has now.
	writeFile(t, filepath.Join(checkout, "internal", "owner.go"), "package owner\n\nfunc lock() { held = true }\n\nvar held bool\n")
	owner := Owner{Git: gittree.Workspace{Dir: checkout}, Checkout: checkout}

	read, err := owner.AsItStands("internal/owner.go", 3, 5)

	testutil.Require(t, "the read", err, nil)
	testutil.Expect(t, "no commit, because nothing is pinned", read.Commit, "")
	testutil.Expect(t, "said to be the checkout's", read.Checkout, true)
	testutil.Expect(t, "the whole file's length, as edited", read.Total, 5)
	testutil.Expect(t, "the edited lines, none marked", read.Lines, []SourceLine{
		{Number: 3, Text: "func lock() { held = true }"},
		{Number: 4, Text: ""},
		{Number: 5, Text: "var held bool"},
	})

	whole, err := owner.AsItStands("internal/owner.go", 0, 0)
	testutil.Require(t, "the whole file", err, nil)
	testutil.Expect(t, "from the first line to the last", []int{whole.From, whole.To}, []int{1, 5})

	writeFile(t, filepath.Join(checkout, "long.go"), strings.Repeat("line\n", 1000))
	long, err := owner.AsItStands("long.go", 10, 900)
	testutil.Require(t, "a long file", err, nil)
	testutil.Expect(t, "four hundred lines at most", []int{long.From, long.To, len(long.Lines)}, []int{10, 409, 400})

	writeFile(t, filepath.Join(checkout, "docs", "shot.png"), "\x89PNG\x00\x00binary")
	outside := t.TempDir()
	writeFile(t, filepath.Join(outside, "secret.go"), "package secret\n")
	if err := os.Symlink(filepath.Join(outside, "secret.go"), filepath.Join(checkout, "escape.go")); err != nil {
		t.Fatal(err)
	}
	for _, refused := range []struct {
		what, path string
		from, to   int
		reason     string
	}{
		{"a path outside the checkout", "../secrets", 1, 2, `"../secrets" is not a path inside the checkout`},
		{"an absolute path", "/etc/passwd", 1, 2, `"/etc/passwd" is not a path inside the checkout`},
		{"a link that leaves the checkout", "escape.go", 1, 2, "escape.go is not a file of the checkout"},
		{"a file the checkout does not hold", "nowhere.go", 1, 2, "nowhere.go is not a file of the checkout"},
		{"a directory", "internal", 1, 2, "internal is not a file of the checkout"},
		{"a range past the file", "internal/owner.go", 9, 12, "internal/owner.go has 5 lines; line 9 is past its end"},
		{"a range backwards", "internal/owner.go", 4, 2, "a range runs forwards: line 4 to line 2 is not one"},
		{"a binary file", "docs/shot.png", 1, 2, "docs/shot.png is a binary file, and the desk shows text"},
	} {
		_, err := owner.AsItStands(refused.path, refused.from, refused.to)
		var refusal *Refusal
		if !errors.As(err, &refusal) || err.Error() != refused.reason {
			t.Fatalf("%s: %v, want the refusal %q", refused.what, err, refused.reason)
		}
	}

	_, err = Owner{Git: gittree.Workspace{Dir: checkout}}.AsItStands("internal/owner.go", 1, 1)
	testutil.Expect(t, "an owner with no checkout says so", err != nil, true)
}
