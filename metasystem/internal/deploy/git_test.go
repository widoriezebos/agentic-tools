package deploy

import (
	"fmt"
	"strings"
	"testing"
)

func TestCheckoutGitContainsReadsAncestry(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		output, err := git(root, args...)
		if err != nil {
			t.Fatal(err)
		}
		return output
	}
	run("init", "-b", "main")
	run("config", "user.name", "Fixture")
	run("config", "user.email", "fixture@example.invalid")
	commit := func(name string) string {
		t.Helper()
		run("-c", "commit.gpgsign=false", "commit", "--allow-empty", "-m", name)
		return run("rev-parse", "HEAD")
	}
	a := commit("a")
	b := commit("b")
	run("checkout", "--detach", a)
	c := commit("c")
	contains := CheckoutGit(root).Contains
	for _, row := range []struct {
		c, d         string
		want, failed bool
	}{{a, b, true, false}, {b, b, true, false}, {b, a, false, false}, {c, b, false, false}, {strings.Repeat("f", 40), b, false, true}} {
		t.Run(fmt.Sprintf("%s-%s", Short(row.c), Short(row.d)), func(t *testing.T) {
			t.Parallel()
			got, err := contains(row.c, row.d)
			if got != row.want || (err != nil) != row.failed {
				t.Fatalf("contains = %t, %v; want %t, error %t", got, err, row.want, row.failed)
			}
		})
	}
}
