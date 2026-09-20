package proofrun

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/pathpattern"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestGLEPathWildcardInputIdentityTracksContentAndMembership(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, "cmd")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	first := filepath.Join(dir, "landing_batch.go")
	if err := os.WriteFile(first, []byte("one"), 0600); err != nil {
		t.Fatal(err)
	}
	group := testpolicy.Group{Inputs: []string{"cmd/landing_batch*.go"}}
	digest := func() string {
		t.Helper()
		value, err := digestGroupInputs(root, group, nil)
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	initial := digest()
	if err := os.WriteFile(first, []byte("two"), 0600); err != nil {
		t.Fatal(err)
	}
	content := digest()
	if content == initial {
		t.Fatal("matched file content did not invalidate input")
	}
	added := filepath.Join(dir, "landing_batch_land.go")
	if err := os.WriteFile(added, []byte("new"), 0600); err != nil {
		t.Fatal(err)
	}
	addition := digest()
	if addition == content {
		t.Fatal("matched file addition did not invalidate input")
	}
	if err := os.Remove(added); err != nil {
		t.Fatal(err)
	}
	if digest() != content {
		t.Fatal("deletion did not restore prior identity")
	}
	if err := os.Rename(first, added); err != nil {
		t.Fatal(err)
	}
	renamed := digest()
	if renamed == content {
		t.Fatal("rename did not invalidate input")
	}
	if err := os.Rename(added, first); err != nil {
		t.Fatal(err)
	}
	if digest() != content {
		t.Fatal("rename back did not restore identity")
	}
	if err := testexec.Locked(func() error { return os.Chmod(first, 0700) }); err != nil {
		t.Fatal(err)
	}
	if digest() == content {
		t.Fatal("file mode did not invalidate input")
	}
}

func TestGLEPathSymlinkInputTargetChangesIdentity(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "cmd"), 0700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "cmd", "landing_batch_link.go")
	if err := os.Symlink("first.go", link); err != nil {
		t.Fatal(err)
	}
	group := testpolicy.Group{Inputs: []string{"cmd/landing_batch*.go"}}
	before, err := digestGroupInputs(root, group, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("second.go", link); err != nil {
		t.Fatal(err)
	}
	after, err := digestGroupInputs(root, group, nil)
	if err != nil || before == after {
		t.Fatalf("symlink target change = %s, %s, %v", before, after, err)
	}
}

func TestGLEPathOptionalInputAndEscapes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	group := testpolicy.Group{Inputs: []string{"cmd/future?.go"}}
	before, err := digestGroupInputs(root, group, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "cmd"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "cmd", "future1.go"), []byte("future"), 0600); err != nil {
		t.Fatal(err)
	}
	after, err := digestGroupInputs(root, group, nil)
	if err != nil || before == after {
		t.Fatalf("optional addition = %s, %s, %v", before, after, err)
	}
	for _, invalid := range []string{"../secret", "cmd/**/secret", "cmd/[ab].go"} {
		group.Inputs = []string{invalid}
		if _, err := digestGroupInputs(root, group, nil); err == nil {
			t.Errorf("accepted %q", invalid)
		}
	}
	group.Inputs = []string{"cmd/future?.go"}
	if _, err := digestGroupInputsWithImplicit(root, group, nil, []string{"cmd/[literal].go"}); err != nil {
		t.Fatalf("exact discovered path with bracket was refused: %v", err)
	}
	if _, err := digestGroupInputsWithImplicit(root, group, nil, []string{"../outside"}); err == nil {
		t.Fatal("discovered input escaped candidate root")
	}
	if err := os.Symlink(t.TempDir(), filepath.Join(root, "linked")); err != nil {
		t.Fatal(err)
	}
	if _, err := digestGroupInputsWithImplicit(root, group, nil, []string{"linked/outside"}); err == nil {
		t.Fatal("discovered input traversed symlink parent")
	}
}

func TestGLEPathLegacyExactDirectoryAndFileInputs(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "fixtures"), 0700); err != nil {
		t.Fatal(err)
	}
	name := filepath.Join(root, "fixtures", "case.txt")
	if err := os.WriteFile(name, []byte("one"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, declaration := range []string{"fixtures", "fixtures/case.txt"} {
		group := testpolicy.Group{Inputs: []string{declaration}}
		before, err := digestGroupInputs(root, group, nil)
		if err != nil {
			t.Fatalf("%s: %v", declaration, err)
		}
		if err := os.WriteFile(name, []byte("two"), 0600); err != nil {
			t.Fatal(err)
		}
		after, err := digestGroupInputs(root, group, nil)
		if err != nil || before == after {
			t.Fatalf("%s did not bind child content: %s, %s, %v", declaration, before, after, err)
		}
		if err := os.WriteFile(name, []byte("one"), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestGLEPathManifestKeepsDiscoveredLiteralKind(t *testing.T) {
	t.Parallel()
	manifest := mergeInputManifest([]string{"src/*.go"}, []string{"src/[literal].go"})
	if len(manifest) != 2 {
		t.Fatalf("merged input manifest = %v", manifest)
	}
	seenLiteral := false
	for _, entry := range manifest {
		value, literal, err := pathpattern.ManifestEntry(entry)
		if err != nil {
			t.Fatal(err)
		}
		if value == "src/[literal].go" {
			seenLiteral = literal
		}
	}
	if !seenLiteral {
		t.Fatalf("discovered metacharacter filename lost its literal kind: %v", manifest)
	}
}
