package proofrun

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
)

// A frozen export lives in this process's scratch root, which the end of any
// in-process dispatch releases when no user holds it. In the cmd/metasystem
// test binary parallel tests end dispatches while another test freezes; an
// export that did not count as a scratch user was removed under it, and the
// freeze reported "the source changed while it was copied" (flaky
// TestFrozenPublicVersionOneCorpusNormalizesSupportedSourceLayouts). These
// witnesses run that release deterministically at the two moments it hurt.
func TestFreezeHoldsTheProcessScratchAgainstAConcurrentRelease(t *testing.T) {
	releaseScratch := func(t *testing.T) {
		t.Helper()
		if err := diskstore.ReleaseProcessScratch(context.Background(), diskstore.WriterDrain{}); err != nil {
			t.Fatalf("release process scratch: %v", err)
		}
	}
	source := func(t *testing.T) string {
		t.Helper()
		root := t.TempDir()
		writeTestFile(t, filepath.Join(root, "metasystem.conf"), []byte("metasystem.version=1\n"), 0o644)
		writeTestFile(t, filepath.Join(root, "internal", "source.go"), []byte("package source\n"), 0o644)
		writeTestFile(t, filepath.Join(root, "plans", "goals", "live.md"), []byte("live goal\n"), 0o644)
		return root
	}

	t.Run("a release while the export is verified", func(t *testing.T) {
		frozen, err := freezeWithHook(source(t), func(string) { releaseScratch(t) })
		if err != nil {
			t.Fatalf("freeze under a concurrent scratch release: %v", err)
		}
		if _, err := os.Stat(filepath.Join(frozen.Root, "internal", "source.go")); err != nil {
			t.Fatalf("frozen export lost its bytes: %v", err)
		}
		if err := frozen.Close(); err != nil {
			t.Fatalf("close frozen export: %v", err)
		}
	})

	t.Run("a release while the published export is in use", func(t *testing.T) {
		frozen, err := Freeze(source(t))
		if err != nil {
			t.Fatal(err)
		}
		releaseScratch(t)
		if _, err := os.Stat(filepath.Join(frozen.Root, "internal", "source.go")); err != nil {
			t.Fatalf("published frozen export removed by a scratch release: %v", err)
		}
		owner := filepath.Dir(frozen.SnapshotRoot)
		if err := frozen.Close(); err != nil {
			t.Fatalf("close frozen export: %v", err)
		}
		if _, err := os.Lstat(owner); !os.IsNotExist(err) {
			t.Fatalf("closed frozen export left %s: %v", owner, err)
		}
		// With the last use done, the owner's release removes the root.
		releaseScratch(t)
	})
}
