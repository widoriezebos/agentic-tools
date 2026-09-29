package diskstore

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// restoreFromArchive rebuilds an item's original files from its export
// archive alone: its entries and its recipes resolved inside it.
func restoreFromArchive(t *testing.T, archivePath, manifestPath string) map[string]string {
	t.Helper()
	manifest, err := readExportManifest(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	compressed, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	entries := map[string]string{}
	archive := tar.NewReader(compressed)
	for {
		header, err := archive.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		data, _ := io.ReadAll(archive)
		entries[header.Name] = string(data)
	}
	restored := map[string]string{}
	for name, data := range entries {
		if !strings.HasPrefix(name, ".blobs/") && name != DistilledName && name != OwnerFileName {
			restored[name] = data
		}
	}
	for _, line := range manifest.Recipes {
		switch line.Kind {
		case RecipeBlob:
			restored[line.Path] = entries[".blobs/"+line.SHA256]
		case RecipeGzip:
			delete(restored, line.Replacement)
			reader, err := gzip.NewReader(strings.NewReader(entries[line.Replacement]))
			if err != nil {
				t.Fatal(err)
			}
			plain, _ := io.ReadAll(reader)
			restored[line.Path] = string(plain)
		case RecipeGit:
			delete(restored, line.Replacement)
			reader, err := gzip.NewReader(strings.NewReader(entries[line.Replacement]))
			if err != nil {
				t.Fatal(err)
			}
			inner := tar.NewReader(reader)
			for {
				header, err := inner.Next()
				if errors.Is(err, io.EOF) {
					break
				}
				if header.Typeflag == tar.TypeReg {
					plain, _ := io.ReadAll(inner)
					restored[line.Path+"/"+strings.TrimPrefix(header.Name, "./")] = string(plain)
				}
			}
		}
	}
	return restored
}

func exportRequest(bed distillBed, dir, stage string) ExportRequest {
	return ExportRequest{Item: bed.bundle, Dir: dir, Segment: "107e72c67539", Kind: KindBundle, Blobs: bed.blobs, Now: testNow, Stage: stage}
}

// A distilled bundle whose payload exists only as blobs is exported, then
// the bundle and the host's blob store are deleted, and the archive alone
// restores every original file byte-for-byte (DL4E-01).
func TestAnExportRestoresAloneWithoutTheBundleOrTheBlobStore(t *testing.T) {
	t.Parallel()
	bed := newDistillBed(t)
	if _, err := Distill(context.Background(), bed.bundle, bed.rules("01STAGE0000000000000000001"), testNow); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(realDir(t), "exports")
	result, err := Export(context.Background(), exportRequest(bed, dir, "01E"))
	if err != nil || result.Already {
		t.Fatalf("%+v %v", result, err)
	}
	if err := os.RemoveAll(bed.bundle); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(bed.blobs.Dir); err != nil {
		t.Fatal(err)
	}
	sameFiles(t, restoreFromArchive(t, result.Archive, result.Manifest), bed.original)
}

func TestAnExportIsIdempotentByContent(t *testing.T) {
	t.Parallel()
	bed := newDistillBed(t)
	dir := filepath.Join(realDir(t), "exports")
	first, err := Export(context.Background(), exportRequest(bed, dir, "01E1"))
	if err != nil {
		t.Fatal(err)
	}
	before := treeFiles(t, dir)
	again, err := Export(context.Background(), exportRequest(bed, dir, "01E2"))
	if err != nil || !again.Already || again.Archive != first.Archive {
		t.Fatalf("a repeat reports already exported: %+v %v", again, err)
	}
	sameFiles(t, treeFiles(t, dir), before)
	// A re-landed record changes the item: a new archive beside the old.
	writeBedFile(t, filepath.Join(bed.bundle, "source-003-log", "small.txt"), []byte("changed"))
	changed, err := Export(context.Background(), exportRequest(bed, dir, "01E3"))
	if err != nil || changed.Already || changed.Archive == first.Archive {
		t.Fatalf("changed content is a new archive: %+v %v", changed, err)
	}
	if _, err := os.Stat(first.Archive); err != nil {
		t.Fatalf("the earlier archive is kept: %v", err)
	}
	after := treeFiles(t, dir)
	last, err := Export(context.Background(), exportRequest(bed, dir, "01E4"))
	if err != nil || !last.Already {
		t.Fatalf("%+v %v", last, err)
	}
	sameFiles(t, treeFiles(t, dir), after)
	archives := 0
	for name := range after {
		if strings.HasSuffix(name, ".tar.gz") {
			archives++
		}
	}
	if archives != 2 {
		t.Fatalf("exactly two archives, got %d", archives)
	}
}

func TestAnArchiveThatFailsItsReadBackIsNeverReportedDone(t *testing.T) {
	t.Parallel()
	bed := newDistillBed(t)
	dir := filepath.Join(realDir(t), "exports")
	request := exportRequest(bed, dir, "01E")
	request.corrupt = true
	if _, err := Export(context.Background(), request); err == nil || !strings.Contains(err.Error(), "not exported") {
		t.Fatalf("a flipped byte fails the read-back: %v", err)
	}
	if files := treeFiles(t, dir); len(files) != 0 {
		t.Fatalf("no final or partial file is left: %v", files)
	}
}

func TestAnExportWhoseDirectorySyncFailsIsNotExported(t *testing.T) {
	t.Parallel()
	bed := newDistillBed(t)
	dir := filepath.Join(realDir(t), "exports")
	request := exportRequest(bed, dir, "01E")
	request.Sync = Syncer{Dir: func(string) error { return errors.New("injected: the entry was dropped") }}
	if _, err := Export(context.Background(), request); err == nil || !strings.Contains(err.Error(), "not durable") {
		t.Fatalf("a failed directory sync is not exported: %v", err)
	}
	if files := treeFiles(t, dir); len(files) != 0 {
		t.Fatalf("no final name is left: %v", files)
	}
}

// Reuse re-establishes durability before it authorizes a removal
// (DL4F-02): an existing verified export whose directory cannot be synced
// is not "already exported".
func TestAReusedExportReestablishesDurability(t *testing.T) {
	t.Parallel()
	bed := newDistillBed(t)
	dir := filepath.Join(realDir(t), "exports")
	if _, err := Export(context.Background(), exportRequest(bed, dir, "01E1")); err != nil {
		t.Fatal(err)
	}
	failing := exportRequest(bed, dir, "01E2")
	failing.Sync = Syncer{Dir: func(string) error { return errors.New("injected power loss") }}
	if result, err := Export(context.Background(), failing); err == nil || result.Already {
		t.Fatalf("a reuse whose barrier fails authorizes nothing: %+v %v", result, err)
	}
}

func TestAnExportDirectoryInsideARootOrCheckoutIsDeclined(t *testing.T) {
	t.Parallel()
	// The judgement is by file identity, so the roots must exist: a bed of
	// its own, never the host's real evidence root or checkout.
	bed := t.TempDir()
	root, checkout := filepath.Join(bed, "metasystem-evidence", "agentic-tools"), filepath.Join(bed, "agentic-tools-m1e")
	store := filepath.Join(checkout, "metasystem", "artifacts", "agents", "workspaces", "goal-g", "default")
	for _, dir := range []string{root, store, filepath.Join(bed, "Backup")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	roots, checkouts, stores := []string{root}, []string{checkout}, []string{store}
	for _, dir := range []string{filepath.Join(root, "exports"), filepath.Join(checkout, "exports")} {
		if problem := ExportDirProblem(dir, roots, checkouts, stores); !strings.Contains(problem, "an export must live outside every evidence root and checkout") {
			t.Fatalf("%s: %q", dir, problem)
		}
	}
	if problem := ExportDirProblem(filepath.Join(bed, "Backup", "metasystem-exports"), roots, checkouts, stores); problem != "" {
		t.Fatalf("an external directory is accepted: %q", problem)
	}
}
