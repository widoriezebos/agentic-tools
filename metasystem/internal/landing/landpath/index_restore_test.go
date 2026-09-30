package landpath

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
)

// realGitRoot is a repository with one commit holding a.txt and b.txt, both
// then edited in the working tree, and the bed's Git replaced by real git in
// it: the index this test compares is git's own.
func realGitRoot(t *testing.T, b *bed) string {
	t.Helper()
	root := b.root
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = gittree.ScrubbedEnviron()
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	run("config", "user.name", "index fixture")
	run("config", "user.email", "index@example.invalid")
	for _, name := range []string{"a.txt", "b.txt", ".gitignore"} {
		body := name + " one\n"
		if name == ".gitignore" {
			body = "message\n"
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run("add", "--", "a.txt", "b.txt", ".gitignore")
	run("commit", "-q", "-m", "base")
	for _, name := range []string{"a.txt", "b.txt"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(name+" two\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	b.owners.Git = func(call GitCall) GitResult {
		cmd := exec.Command("git", call.Args...)
		cmd.Dir = call.Dir
		cmd.Env = append(gittree.ScrubbedEnviron(), call.Env...)
		cmd.Stdin = bytes.NewReader(call.Stdin)
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		code := 0
		if err := cmd.Run(); err != nil {
			code = 1
			if exit, ok := err.(*exec.ExitError); ok {
				code = exit.ExitCode()
			}
		}
		return GitResult{Stdout: stdout.Bytes(), Stderr: stderr.Bytes(), Code: code}
	}
	return root
}

func indexBytes(t *testing.T, root string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, ".git", "index"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// TestARefusedPathModeLandingGivesTheIndexBackAsItFoundIt: the switch-on
// trial's defect. A path-mode landing stages the caller's paths and is then
// refused before its commit (unstaged changes remain; a receipt that does
// not name the candidate). The index is byte-for-byte the one the landing
// found, the working tree keeps its edits, and the same command run again
// is not refused for a non-empty index.
func TestARefusedPathModeLandingGivesTheIndexBackAsItFoundIt(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name    string
		request func(b *bed) LandRequest
		drift   bool
		text    string
	}{
		{"unstaged changes remain", func(*bed) LandRequest {
			return LandRequest{Pathspecs: []string{"a.txt"}, SkipTransport: true}
		}, true, "other changed files would stay behind uncommitted: b.txt"},
		{"receipt names another tree", func(b *bed) LandRequest {
			return LandRequest{Pathspecs: []string{"a.txt", "b.txt"}, Chain: "c1", TestReceipt: filepath.Join(b.root, "absent.json"), SkipTransport: true}
		}, false, "the test receipt can't be read (missing)"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			b := newBed(t)
			root := realGitRoot(t, b)
			b.owners.Drift = func(_ string, _ bool, stdout, _ io.Writer) int {
				if !c.drift {
					return 0
				}
				fmt.Fprintln(stdout, "unstaged\t M\tb.txt")
				return 1
			}
			before := indexBytes(t, root)
			b.expect(b.land(c.request(b)), 2, c.text)
			if after := indexBytes(t, root); !bytes.Equal(before, after) {
				t.Fatalf("the refused landing left the index changed\nstdout:\n%s\nstderr:\n%s", b.stdout.String(), b.stderr.String())
			}
			for _, name := range []string{"a.txt", "b.txt"} {
				if data, _ := os.ReadFile(filepath.Join(root, name)); string(data) != name+" two\n" {
					t.Fatalf("the working tree was touched: %s holds %q", name, data)
				}
			}
			b.stdout.Reset()
			b.stderr.Reset()
			b.land(c.request(b))
			if strings.Contains(b.stderr.String()+b.stdout.String(), "requires an empty index") {
				t.Fatalf("the retry was refused for the index the first attempt left:\n%s", b.stderr.String())
			}
		})
	}
}
