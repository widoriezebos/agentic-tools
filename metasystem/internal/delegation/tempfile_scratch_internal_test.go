package delegation

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
)

// A session with no TMPDIR of its own makes its disposable files in the
// process's registered scratch root, never an unowned os.TempDir() entry
// (Part B U1b-2); a named directory or the session's TMPDIR wins.
func TestTempFileWithoutADirectoryIsInTheProcessScratch(t *testing.T) {
	t.Parallel()
	scratch, err := diskstore.ProcessScratch()
	if err != nil {
		t.Fatal(err)
	}
	path, err := (&session{}).tempFile("", "metasystem-husk-fail")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })
	if filepath.Dir(path) != scratch {
		t.Fatalf("a session without TMPDIR made %s, want a file in the process scratch root %s", path, scratch)
	}
	given := t.TempDir()
	named, err := (&session{env: Env{TempDir: given}}).tempFile("", "x")
	if err != nil || filepath.Dir(named) != given {
		t.Fatalf("a session with TMPDIR %s made %s, %v", given, named, err)
	}
}
