package goadapter

import (
	"go/build"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
)

// Git freezes the base so declaration discovery must read candidate working bytes.
func TestRequiredTestFilesExpandCandidateDeclarations(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write := func(path, source string) {
		t.Helper()
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.invalid/required\n\ngo 1.27\n")
	write("one/public_test.go", "package one\nimport \"testing\"\nfunc TestOriginal(t *testing.T) {}\n")
	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"add", "."}, {"commit", "-qm", "base"}} {
		cmd := exec.Command("git", append([]string{"-C", root, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid"}, args...)...)
		cmd.Env = gittree.ScrubbedEnviron()
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	write("one/public_test.go", "package one\nimport \"testing\"\nfunc TestRenamed(t *testing.T) {}\nconst decoy = `\nfunc TestDecoy(t *testing.T) {}\n`\ntype Receiver struct{}\nfunc (Receiver) TestMethod() {}\nfunc TestMain(m *testing.M) {}\n")
	write("one/added_test.go", "package one\nimport \"testing\"\nfunc TestAdded(t *testing.T) {}\n")
	write("two/public_test.go", "package two\nimport \"testing\"\nfunc TestOther(t *testing.T) {}\n")
	write("two/excluded_test.go", "//go:build never_on_this_host\n\npackage two\nimport \"testing\"\nfunc TestExcluded(t *testing.T) {}\n")
	context := build.Default
	context.BuildTags = nil
	got, excluded, err := RequiredTests(root, &context, []string{"one/*_test.go", "one/public_test.go", "two/*_test.go"})
	want := map[string][]string{"./one": {"TestAdded", "TestRenamed"}, "./two": {"TestOther"}}
	if err != nil || excluded != 1 || !reflect.DeepEqual(got, want) {
		t.Fatalf("required=%v excluded=%d err=%v; want %v and 1", got, excluded, err, want)
	}
	write("two/broken_test.go", "package two\nfunc TestBroken(\n")
	for _, pattern := range []string{"missing/*_test.go", "two/broken_test.go"} {
		if _, _, err := RequiredTests(root, &context, []string{pattern}); err == nil || !strings.Contains(err.Error(), pattern) {
			t.Fatalf("want error naming %s: %v", pattern, err)
		}
	}
}
