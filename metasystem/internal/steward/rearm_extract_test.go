package steward

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
)

// TestGitArchivedTreeExtractsNativelyAndMatchesTar holds R-138-m1e for the
// re-arm witness digest: with only git reachable on PATH the archived tree is
// still extracted, and the digest equals the one a real tar extraction of the
// same archive produces (the port's equivalence witness).
func TestGitArchivedTreeExtractsNativelyAndMatchesTar(t *testing.T) {
	root := initRearmRepo(t)
	writeRearmFile(t, filepath.Join(root, "cmd", "nested", "deep.txt"), "deep\n")
	writeRearmFile(t, filepath.Join(root, "cmd", "run.sh"), "#!/bin/sh\n")
	if err := os.Chmod(filepath.Join(root, "cmd", "run.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("surface.txt", filepath.Join(root, "cmd", "link")); err != nil {
		t.Fatal(err)
	}
	commitRearmTree(t, root, "archive")
	policy, err := behaviorsurface.Load()
	if err != nil {
		t.Fatal(err)
	}

	archive, err := exec.Command("git", "-C", root, "archive", "--format=tar", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	byTar := t.TempDir()
	extract := exec.Command("tar", "-xf", "-", "-C", byTar)
	extract.Stdin = bytes.NewReader(archive)
	if output, err := extract.CombinedOutput(); err != nil {
		t.Fatalf("reference tar extraction: %v\n%s", err, output)
	}
	want, err := policy.Digest(byTar, behaviorsurface.Engine)
	if err != nil {
		t.Fatal(err)
	}
	empty, err := policy.Digest(t.TempDir(), behaviorsurface.Engine)
	if err != nil {
		t.Fatal(err)
	}
	if want == empty {
		t.Fatal("the fixture sits outside the engine projection; the comparison would be vacuous")
	}

	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	onlyGit := t.TempDir()
	if err := os.Symlink(gitPath, filepath.Join(onlyGit, "git")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", onlyGit)
	got, err := digestArchivedTree(context.Background(), root, "HEAD", policy, SystemRearmClock(), 20)
	if err != nil {
		t.Fatalf("digest with only git on PATH: %v", err)
	}
	if got != want {
		t.Fatalf("native extraction digest %s differs from tar's %s", got, want)
	}
}

// TestArchivedTreeExtractionReportsEachEntryAndRefusesEscapes: every entry
// is progress for the silence bound, as tar -v's per-entry line was, and an
// entry that would land outside the extraction directory is refused.
func TestArchivedTreeExtractionReportsEachEntryAndRefusesEscapes(t *testing.T) {
	write := func(t *testing.T, entries []tar.Header) string {
		t.Helper()
		var buffer bytes.Buffer
		writer := tar.NewWriter(&buffer)
		for _, header := range entries {
			header := header
			body := []byte(nil)
			if header.Typeflag == tar.TypeReg {
				body = []byte("content of " + header.Name)
				header.Size = int64(len(body))
			}
			if err := writer.WriteHeader(&header); err != nil {
				t.Fatal(err)
			}
			if _, err := writer.Write(body); err != nil {
				t.Fatal(err)
			}
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(t.TempDir(), "tree.tar")
		if err := os.WriteFile(path, buffer.Bytes(), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	archive := write(t, []tar.Header{
		{Typeflag: tar.TypeXGlobalHeader, Name: "pax_global_header", PAXRecords: map[string]string{"comment": "0123"}},
		{Typeflag: tar.TypeDir, Name: "cmd/", Mode: 0o755},
		{Typeflag: tar.TypeReg, Name: "cmd/a.txt", Mode: 0o644},
		{Typeflag: tar.TypeReg, Name: "cmd/run", Mode: 0o755},
		{Typeflag: tar.TypeSymlink, Name: "cmd/link", Linkname: "a.txt"},
	})
	destination := t.TempDir()
	progress := 0
	if err := extractArchivedTree(context.Background(), archive, destination, func() { progress++ }); err != nil {
		t.Fatal(err)
	}
	if progress < 4 {
		t.Fatalf("extraction reported %d progress calls for four entries", progress)
	}
	if data, err := os.ReadFile(filepath.Join(destination, "cmd", "a.txt")); err != nil || string(data) != "content of cmd/a.txt" {
		t.Fatalf("regular file = %q, %v", data, err)
	}
	if info, err := os.Stat(filepath.Join(destination, "cmd", "run")); err != nil || info.Mode().Perm()&0o100 == 0 {
		t.Fatalf("executable entry lost its mode: %v %v", info, err)
	}
	if target, err := os.Readlink(filepath.Join(destination, "cmd", "link")); err != nil || target != "a.txt" {
		t.Fatalf("symlink = %q, %v", target, err)
	}

	for _, name := range []string{"../escape", "/absolute", "cmd/../../escape"} {
		escaping := write(t, []tar.Header{{Typeflag: tar.TypeReg, Name: name, Mode: 0o644}})
		err := extractArchivedTree(context.Background(), escaping, t.TempDir(), func() {})
		if err == nil || !strings.Contains(err.Error(), "outside") {
			t.Fatalf("entry %q was not refused: %v", name, err)
		}
	}
	unsupported := write(t, []tar.Header{{Typeflag: tar.TypeFifo, Name: "pipe", Mode: 0o644}})
	if err := extractArchivedTree(context.Background(), unsupported, t.TempDir(), func() {}); err == nil {
		t.Fatal("a FIFO entry was extracted")
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := extractArchivedTree(cancelled, archive, t.TempDir(), func() {}); !errors.Is(err, context.Canceled) {
		t.Fatalf("a cancelled step kept extracting: %v", err)
	}
}
