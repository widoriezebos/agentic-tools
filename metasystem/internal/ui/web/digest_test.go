package web

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// The fixture and its digest are written in the design; the bundle script
// asserts the same literal from its own implementation of the rule, so the two
// cannot drift apart without one of them failing.
const fixtureDigest = "sha256:f8c8104462398072962c697d4b860b1e76da903a0fa41933ffc541bcbb3e1399"

func writeFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write(t, filepath.Join(dir, ".nvmrc"), []byte("24.21.0\n"))
	write(t, filepath.Join(dir, "a.txt"), []byte("x\r\n"))
	write(t, filepath.Join(dir, "b", "c.txt"), []byte("y\n"))
	write(t, filepath.Join(dir, "d.bin"), []byte{0x0d, 0x0a})
	write(t, filepath.Join(dir, ".hidden"), []byte("z"))
	write(t, filepath.Join(dir, "b", "node_modules", "q.txt"), []byte("q"))
	return dir
}

func write(t *testing.T, path string, content []byte) {
	t.Helper()
	testutil.Require(t, "prepare "+path, os.MkdirAll(filepath.Dir(path), 0o755), nil)
	testutil.Require(t, "write "+path, os.WriteFile(path, content, 0o644), nil)
}

func TestSourceDigestFixture(t *testing.T) {
	t.Parallel()

	t.Run("the fixture digest", func(t *testing.T) {
		t.Parallel()
		digest, files, err := SourceDigest(writeFixture(t))
		testutil.Require(t, "source digest", err, nil)
		testutil.Expect(t, "digest", digest, fixtureDigest)
		testutil.Expect(t, "files", files, []FileDigest{
			{Path: ".nvmrc", SHA256: "73fb1b615e2043a933be1c0895cde4358036acc28d785692509b822aa53c761f"},
			{Path: "a.txt", SHA256: "73cb3858a687a8494ca3323053016282f3dad39d42cf62ca4e79dda2aac7d9ac"},
			{Path: "b/c.txt", SHA256: "3bb2abb69ebb27fbfe63c7639624c6ec5e331b841a5bc8c3ebc10b9285e90877"},
			{Path: "d.bin", SHA256: "7eb70257593da06f682a3ddda54a9d260d4fc514f645237f5ca74b08f8da61a6"},
		})
	})

	// A CRLF file and a binary file holding the same two bytes must not digest
	// alike: normalisation is by kind, never by content.
	t.Run("text is normalised and bytes are not", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		write(t, filepath.Join(dir, "same.txt"), []byte{0x0d, 0x0a})
		write(t, filepath.Join(dir, "same.bin"), []byte{0x0d, 0x0a})
		_, files, err := SourceDigest(dir)
		testutil.Require(t, "source digest", err, nil)
		testutil.Require(t, "file count", len(files), 2)
		testutil.Expect(t, "raw bytes", files[0].SHA256, "7eb70257593da06f682a3ddda54a9d260d4fc514f645237f5ca74b08f8da61a6")
		testutil.Expect(t, "normalised text", files[1].SHA256, "01ba4719c80b6fe911b091a7c05124b64eeece964e09c058ef8f9805daca546b")
	})

	t.Run("a symbolic link is an error", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		write(t, filepath.Join(dir, "a.txt"), []byte("x\n"))
		testutil.Require(t, "link", os.Symlink(filepath.Join(dir, "a.txt"), filepath.Join(dir, "b.txt")), nil)
		_, _, err := SourceDigest(dir)
		testutil.Require(t, "source digest refuses", err != nil, true)
		testutil.Expect(t, "refusal names the link", strings.Contains(err.Error(), "b.txt is a symbolic link"), true)
	})

	// The dependency tree is full of links; skipping it by name comes first, so
	// an installed checkout digests the same as a fresh one.
	t.Run("a link inside node_modules is not reached", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		write(t, filepath.Join(dir, "a.txt"), []byte("x\n"))
		write(t, filepath.Join(dir, "node_modules", "p", "index.js"), []byte("export {};\n"))
		testutil.Require(t, "make .bin", os.MkdirAll(filepath.Join(dir, "node_modules", ".bin"), 0o755), nil)
		testutil.Require(t, "link", os.Symlink("../p/index.js", filepath.Join(dir, "node_modules", ".bin", "p")), nil)
		_, files, err := SourceDigest(dir)
		testutil.Require(t, "source digest", err, nil)
		testutil.Expect(t, "files", files, []FileDigest{{Path: "a.txt", SHA256: "73cb3858a687a8494ca3323053016282f3dad39d42cf62ca4e79dda2aac7d9ac"}})
	})

	t.Run("a non-ASCII path is an error", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		write(t, filepath.Join(dir, "späce.txt"), []byte("x\n"))
		_, _, err := SourceDigest(dir)
		testutil.Require(t, "source digest refuses", err != nil, true)
		testutil.Expect(t, "refusal names the path", strings.Contains(err.Error(), "is not an ASCII path"), true)
	})

	t.Run("an absent tree is an error", func(t *testing.T) {
		t.Parallel()
		_, _, err := SourceDigest(filepath.Join(t.TempDir(), "absent"))
		testutil.Expect(t, "source digest refuses", err != nil, true)
	})
}
