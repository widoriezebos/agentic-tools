package output

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSpillWritesTheBytesAndReturnsAVerifiedReference(t *testing.T) {
	root := t.TempDir()
	data := []byte("the complete command output\n")
	ref, err := Spill(root, "test-run", "json", data, time.Date(2026, 9, 13, 9, 8, 7, 6, time.FixedZone("offset", 3600)))
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(ref.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, data) {
		t.Fatalf("spilled bytes = %q, want %q", got, data)
	}
	digest := sha256.Sum256(data)
	if ref.OutputMode != "file" || ref.SchemaVersion != 1 || ref.Verb != "test-run" ||
		ref.Bytes != len(data) || ref.Digest != fmt.Sprintf("sha256:%x", digest) || ref.Format != "json" {
		t.Fatalf("reference = %+v", ref)
	}
	dir := filepath.Join(root, filepath.FromSlash(Dir))
	relative, err := filepath.Rel(dir, ref.Path)
	if err != nil || relative == "." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		t.Fatalf("reference path %q is not a file under %q: relative=%q err=%v", ref.Path, dir, relative, err)
	}
}

func TestSpillNamesAreUniqueAndNeverOverwrite(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	originalNonce := randomNonce
	var nonceMu sync.Mutex
	nonceCalls := 0
	randomNonce = func() (string, error) {
		nonceMu.Lock()
		defer nonceMu.Unlock()
		nonceCalls++
		if nonceCalls <= 2 {
			return "deadbeef", nil
		}
		return fmt.Sprintf("%08x", nonceCalls-2), nil
	}
	t.Cleanup(func() { randomNonce = originalNonce })

	first, err := Spill(root, "land", "log", []byte("first\n"), now)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Spill(root, "land", "log", []byte("second\n"), now)
	if err != nil {
		t.Fatal(err)
	}
	if first.Path == second.Path {
		t.Fatalf("a collision overwrote %s", first.Path)
	}
	if got, err := os.ReadFile(first.Path); err != nil || string(got) != "first\n" {
		t.Fatalf("first spill changed after collision: bytes=%q err=%v", got, err)
	}
	randomNonce = originalNonce

	concurrentRoot := t.TempDir()
	paths := make(chan string, 32)
	errors := make(chan error, 32)
	var group sync.WaitGroup
	for i := 0; i < 32; i++ {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			ref, err := Spill(concurrentRoot, "test-run", "txt", []byte(fmt.Sprintf("%d", index)), now)
			if err != nil {
				errors <- err
				return
			}
			paths <- ref.Path
		}(i)
	}
	group.Wait()
	close(paths)
	close(errors)
	for err := range errors {
		t.Errorf("concurrent spill: %v", err)
	}
	unique := map[string]bool{}
	for path := range paths {
		unique[path] = true
	}
	if len(unique) != 32 {
		t.Fatalf("concurrent spills created %d distinct paths, want 32", len(unique))
	}
	files, err := filepath.Glob(filepath.Join(concurrentRoot, filepath.FromSlash(Dir), "test-run-*.txt"))
	if err != nil || len(files) != 32 {
		t.Fatalf("concurrent spills left %d files, want 32: err=%v", len(files), err)
	}
}

func TestDetectDistinguishesTheEnvelopeFromATestResult(t *testing.T) {
	data := []byte(`{"outputMode":"file","schemaVersion":1,"verb":"test-run","path":"/tmp/result.json","bytes":4,"digest":"sha256:abcd","format":"json"}`)
	ref, ok := Detect(data)
	if !ok || ref.Path != "/tmp/result.json" || ref.Verb != "test-run" {
		t.Fatalf("valid reference was not detected: ref=%+v ok=%t", ref, ok)
	}
	if ref, ok := Detect([]byte(`{"schemaVersion":1,"groups":[]}`)); ok {
		t.Fatalf("test result was detected as a reference: ref=%+v", ref)
	}
	if _, ok := Detect([]byte(`{"outputMode":"file","schemaVersion":2}`)); ok {
		t.Fatal("unsupported reference schema was detected")
	}
}

func TestNewestSinceReturnsOnlyANewerRegularFile(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, filepath.FromSlash(Dir))
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	since := time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC)
	writeAt := func(name, contents string, at time.Time) string {
		t.Helper()
		path := filepath.Join(directory, name)
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, at, at); err != nil {
			t.Fatal(err)
		}
		return path
	}
	writeAt("older.log", "old", since.Add(-time.Second))
	writeAt("equal.log", "equal", since)
	tieZulu := writeAt("zulu.log", "zulu", since.Add(time.Minute))
	tieAlpha := writeAt("alpha.log", "alpha", since.Add(time.Minute))
	newest := writeAt("newest.log", "newest contents", since.Add(2*time.Minute))
	oldDirectory := filepath.Join(directory, "directory.log")
	if err := os.Mkdir(oldDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(oldDirectory, since.Add(3*time.Minute), since.Add(3*time.Minute)); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(directory, "symlink.log")
	if err := os.Symlink(newest, link); err != nil {
		t.Fatal(err)
	}

	path, bytes, ok := NewestSince(root, since)
	if !ok || path != newest || bytes != int64(len("newest contents")) {
		t.Fatalf("newest spill = path %q bytes %d ok %t, want %q", path, bytes, ok, newest)
	}
	if err := os.Remove(newest); err != nil {
		t.Fatal(err)
	}
	path, _, ok = NewestSince(root, since)
	if !ok || path != tieAlpha || path == tieZulu {
		t.Fatalf("lexical tie winner = path %q ok %t, want %q", path, ok, tieAlpha)
	}
	if _, _, ok := NewestSince(root, since.Add(4*time.Minute)); ok {
		t.Fatal("a directory, symlink, equal file, or older file was returned")
	}
	if _, _, ok := NewestSince(t.TempDir(), time.Time{}); ok {
		t.Fatal("a missing output directory returned a spill")
	}
	unreadableRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(unreadableRoot, "artifacts", "agents"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(unreadableRoot, filepath.FromSlash(Dir)), []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := NewestSince(unreadableRoot, time.Time{}); ok {
		t.Fatal("an unreadable output directory returned a spill")
	}
}

func TestPruneRemovesOnlyFilesOlderThanTheWindow(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, filepath.FromSlash(Dir))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	oldTime := now.Add(-8 * 24 * time.Hour)
	oldFile := filepath.Join(dir, "old.log")
	freshFile := filepath.Join(dir, "fresh.log")
	subdir := filepath.Join(dir, "old-directory")
	target := filepath.Join(root, "outside-old.log")
	link := filepath.Join(dir, "old-link.log")
	for _, path := range []string{oldFile, freshFile, target} {
		if err := os.WriteFile(path, []byte(path), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(subdir, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{oldFile, subdir, target} {
		if err := os.Chtimes(path, oldTime, oldTime); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	removed, err := Prune(root, 7*24*time.Hour, now)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(removed, []string{oldFile}) {
		t.Fatalf("removed = %v, want only %s", removed, oldFile)
	}
	for _, path := range []string{freshFile, subdir, link, target} {
		if _, err := os.Lstat(path); err != nil {
			t.Errorf("survivor %s: %v", path, err)
		}
	}

	linkedRoot := t.TempDir()
	externalDir := t.TempDir()
	externalFile := filepath.Join(externalDir, "outside.log")
	if err := os.WriteFile(externalFile, []byte("outside\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(externalFile, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(linkedRoot, "artifacts", "agents"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(externalDir, filepath.Join(linkedRoot, filepath.FromSlash(Dir))); err != nil {
		t.Fatal(err)
	}
	if _, err := Prune(linkedRoot, 7*24*time.Hour, now); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("symlinked output directory was followed: %v", err)
	}
	if got, err := os.ReadFile(externalFile); err != nil || string(got) != "outside\n" {
		t.Fatalf("symlink target was changed: bytes=%q err=%v", got, err)
	}
}
