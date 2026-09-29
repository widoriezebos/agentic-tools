package diskstore

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
)

// Export before delete (3.12 "The public actions"; R23; DL4E-01, DL4E-05,
// DL4E-10, DL4F-02): one self-contained compressed archive per item and a
// manifest, named by the item's inventory digest under
// DIR/<segment>/<item>/<sha12>.tar.gz. The archive carries every regular
// file of the item as it stands and every blob its recipes name, so a
// restore needs neither the bundle nor the host's blob store. It is written
// as .partial, read back and verified entry by entry, then published
// durably (verifiedAt written, renamed, the directory chain synced, the
// archive re-opened by its final name and re-hashed); only then is the item
// exported. An existing verified export of the same content is reused only
// after its durability is re-established the same way.

// ExportSchema names an export manifest.
const ExportSchema = "metasystem.evidence-export/1"

// ExportBlob is one blob an archive carries.
type ExportBlob struct {
	Digest string `json:"digest"`
	Entry  string `json:"entry"`
	Size   int64  `json:"size"`
}

// ExportManifest is an archive's manifest.
type ExportManifest struct {
	Schema          string          `json:"schema"`
	Item            string          `json:"item"`
	Kind            string          `json:"kind"`
	Segment         string          `json:"segment"`
	Checkout        string          `json:"checkout,omitempty"`
	EndedAt         string          `json:"endedAt,omitempty"`
	InventoryDigest string          `json:"inventoryDigest"`
	Files           []InventoryFile `json:"files"`
	Blobs           []ExportBlob    `json:"blobs"`
	Recipes         []RecipeLine    `json:"recipes"`
	Archive         string          `json:"archive"`
	ArchiveSHA256   string          `json:"archiveSha256"`
	ArchiveBytes    int64           `json:"archiveBytes"`
	Entries         int             `json:"entries"`
	VerifiedAt      *time.Time      `json:"verifiedAt,omitempty"`
}

// ExportRequest is one item's export.
type ExportRequest struct {
	// Item is the item's absolute path (a directory, or a file).
	Item string
	// Dir is the export directory the person chose.
	Dir                              string
	Segment, Kind, Checkout, EndedAt string
	Blobs                            BlobStore
	Now                              time.Time
	Stage                            string
	Sync                             Syncer
	// corrupt, in tests, flips a byte of the staged archive.
	corrupt bool
}

// ExportResult is what an export did.
type ExportResult struct {
	Archive         string    `json:"archive"`
	Manifest        string    `json:"manifest"`
	ArchiveSHA256   string    `json:"archiveSha256"`
	InventoryDigest string    `json:"inventoryDigest"`
	Already         bool      `json:"already,omitempty"`
	VerifiedAt      time.Time `json:"verifiedAt"`
	Bytes           int64     `json:"bytes"`
}

// Ref is the export block a tombstone and a receipt carry.
func (r ExportResult) Ref(dir string) *ExportRef {
	return &ExportRef{Dir: dir, Archive: r.Archive, ArchiveSHA256: r.ArchiveSHA256, Manifest: r.Manifest, VerifiedAt: r.VerifiedAt}
}

// ExportPaths are where an item with this inventory digest exports.
func ExportPaths(dir, segment, item, digest string) (archive, manifest string) {
	base := filepath.Join(dir, segment, item, digest[:12])
	return base + ".tar.gz", base + ".manifest.json"
}

// Export writes, verifies and publishes one item's archive.
func Export(ctx context.Context, request ExportRequest) (ExportResult, error) {
	if !filepath.IsAbs(request.Dir) {
		return ExportResult{}, fmt.Errorf("the export directory must be absolute, got %q", request.Dir)
	}
	files, err := Inventory(ctx, request.Item, nil)
	if err != nil {
		return ExportResult{}, err
	}
	digest := InventoryDigest(files)
	name := filepath.Base(request.Item)
	archive, manifestPath := ExportPaths(request.Dir, request.Segment, name, digest)
	result := ExportResult{Archive: archive, Manifest: manifestPath, InventoryDigest: digest}
	if existing, err := readExportManifest(manifestPath); err == nil && existing.VerifiedAt != nil {
		// Reuse re-establishes durability before it authorizes anything
		// (DL4F-02): both files and the directory chain synced, the archive
		// re-opened by its final name and re-hashed.
		if err := request.barrier(ctx, archive, manifestPath, existing.ArchiveSHA256); err == nil {
			result.Already, result.ArchiveSHA256, result.VerifiedAt, result.Bytes = true, existing.ArchiveSHA256, *existing.VerifiedAt, existing.ArchiveBytes
			return result, nil
		}
	}
	manifest := ExportManifest{Schema: ExportSchema, Item: name, Kind: request.Kind, Segment: request.Segment, Checkout: request.Checkout,
		EndedAt: request.EndedAt, InventoryDigest: digest, Archive: filepath.Base(archive), Blobs: []ExportBlob{}, Recipes: []RecipeLine{}}
	for _, file := range files {
		if file.Original != nil {
			manifest.Recipes = append(manifest.Recipes, *file.Original)
			continue
		}
		manifest.Files = append(manifest.Files, file)
	}
	for _, recipe := range manifest.Recipes {
		if recipe.Kind == RecipeBlob && !hasBlob(manifest.Blobs, recipe.SHA256) {
			manifest.Blobs = append(manifest.Blobs, ExportBlob{Digest: recipe.SHA256, Entry: ".blobs/" + recipe.SHA256, Size: recipe.Size})
		}
	}
	if err := os.MkdirAll(filepath.Dir(archive), 0o755); err != nil {
		return result, fmt.Errorf("not exported: %w", err)
	}
	partial := archive + PartialSuffix + request.Stage
	partialManifest := manifestPath + PartialSuffix + request.Stage
	cleanup := func() { _ = os.Remove(partial); _ = os.Remove(partialManifest) }
	entries, err := request.write(ctx, partial, manifest)
	if err != nil {
		cleanup()
		return result, fmt.Errorf("not exported: %w", err)
	}
	if request.corrupt {
		flipByte(partial)
	}
	sum, size, err := FileDigest(ctx, partial)
	if err != nil {
		cleanup()
		return result, fmt.Errorf("not exported: %w", err)
	}
	manifest.ArchiveSHA256, manifest.ArchiveBytes, manifest.Entries = sum, size, entries
	if err := verifyArchive(ctx, partial, manifest); err != nil {
		cleanup()
		return result, fmt.Errorf("not exported: the archive did not verify on read-back: %w", err)
	}
	verified := request.Now.UTC()
	manifest.VerifiedAt = &verified
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		cleanup()
		return result, err
	}
	if err := writeSynced(partialManifest, data); err != nil {
		cleanup()
		return result, fmt.Errorf("not exported: %w", err)
	}
	if err := os.Rename(partial, archive); err != nil {
		cleanup()
		return result, fmt.Errorf("not exported: %w", err)
	}
	if err := os.Rename(partialManifest, manifestPath); err != nil {
		_ = os.Remove(archive)
		cleanup()
		return result, fmt.Errorf("not exported: %w", err)
	}
	if err := request.barrier(ctx, archive, manifestPath, sum); err != nil {
		_ = os.Remove(archive)
		_ = os.Remove(manifestPath)
		return result, fmt.Errorf("not exported: %w", err)
	}
	result.ArchiveSHA256, result.VerifiedAt, result.Bytes = sum, verified, size
	return result, nil
}

func hasBlob(blobs []ExportBlob, digest string) bool {
	for _, blob := range blobs {
		if blob.Digest == digest {
			return true
		}
	}
	return false
}

func readExportManifest(path string) (ExportManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ExportManifest{}, err
	}
	var manifest ExportManifest
	if err := json.Unmarshal(data, &manifest); err != nil || manifest.Schema != ExportSchema {
		if err == nil {
			err = errors.New("not an export manifest")
		}
		return ExportManifest{}, err
	}
	return manifest, nil
}

func writeSynced(path string, data []byte) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	return errors.Join(file.Sync(), file.Close())
}

func syncPath(path string) error {
	file, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	return errors.Join(file.Sync(), file.Close())
}

// barrier is the durability barrier: the archive and manifest synced, every
// directory from the item's export directory up to DIR synced, then the
// archive re-opened by its final name and re-hashed and the manifest re-read.
func (r ExportRequest) barrier(ctx context.Context, archive, manifest, sum string) error {
	for _, path := range []string{archive, manifest} {
		if err := syncPath(path); err != nil {
			return fmt.Errorf("%s could not be synced: %w", path, err)
		}
	}
	for directory := filepath.Dir(archive); ; directory = filepath.Dir(directory) {
		if err := r.Sync.SyncDir(directory); err != nil {
			return fmt.Errorf("the directory %s could not be synced, so the export is not durable: %w", directory, err)
		}
		if directory == r.Dir || directory == filepath.Dir(directory) {
			break
		}
	}
	got, _, err := FileDigest(ctx, archive)
	if err != nil {
		return err
	}
	if got != sum {
		return fmt.Errorf("the archive at %s re-hashes to %s, not %s", archive, got, sum)
	}
	reread, err := readExportManifest(manifest)
	if err != nil || reread.ArchiveSHA256 != sum || reread.VerifiedAt == nil {
		return fmt.Errorf("the manifest %s does not re-read as verified", manifest)
	}
	return nil
}

// write streams the item's files and its recipes' blobs into a tar.gz at
// path, synced; a blob missing from the store or not hashing to its name
// fails the export.
func (r ExportRequest) write(ctx context.Context, path string, manifest ExportManifest) (int, error) {
	out, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return 0, err
	}
	compressed := gzip.NewWriter(out)
	archive := tar.NewWriter(compressed)
	entries := 0
	root := r.Item
	single := false
	if info, err := os.Lstat(root); err == nil && !info.IsDir() {
		single = true
	}
	writeFile := func(entry, source string, file InventoryFile) error {
		if file.Link != "" {
			entries++
			return archive.WriteHeader(&tar.Header{Name: entry, Typeflag: tar.TypeSymlink, Linkname: file.Link, Mode: 0o777})
		}
		in, err := os.Open(source)
		if err != nil {
			return err
		}
		defer in.Close()
		if err := archive.WriteHeader(&tar.Header{Name: entry, Typeflag: tar.TypeReg, Size: file.Size, Mode: 0o644}); err != nil {
			return err
		}
		entries++
		_, err = copyChunks(ctx, archive, in)
		return err
	}
	for _, file := range manifest.Files {
		source := filepath.Join(root, filepath.FromSlash(file.Path))
		if single {
			source = root
		}
		if err := writeFile(file.Path, source, file); err != nil {
			return entries, errors.Join(err, archive.Close(), compressed.Close(), out.Close())
		}
	}
	for _, blob := range manifest.Blobs {
		present, err := r.Blobs.Verify(ctx, blob.Digest)
		if err != nil || !present {
			return entries, errors.Join(fmt.Errorf("blob %s missing", blob.Digest), archive.Close(), compressed.Close(), out.Close())
		}
		if err := writeFile(blob.Entry, r.Blobs.Path(blob.Digest), InventoryFile{Size: blob.Size}); err != nil {
			return entries, errors.Join(err, archive.Close(), compressed.Close(), out.Close())
		}
	}
	return entries, errors.Join(archive.Close(), compressed.Close(), out.Sync(), out.Close())
}

// verifyArchive reads an archive back: every entry inflated and hashed
// against the manifest, every blob entry against its digest, every recipe's
// replacement or blob found among the entries, the entry count checked.
func verifyArchive(ctx context.Context, path string, manifest ExportManifest) error {
	want := map[string]string{}
	for _, file := range manifest.Files {
		if file.Link == "" {
			want[file.Path] = file.SHA256
		} else {
			want[file.Path] = "link:" + file.Link
		}
	}
	for _, blob := range manifest.Blobs {
		want[blob.Entry] = blob.Digest
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	compressed, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer compressed.Close()
	archive := tar.NewReader(compressed)
	seen := map[string]bool{}
	count := 0
	for {
		header, err := archive.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		count++
		expected, listed := want[header.Name]
		if !listed {
			return fmt.Errorf("entry %s is not in the manifest", header.Name)
		}
		switch header.Typeflag {
		case tar.TypeSymlink:
			if expected != "link:"+header.Linkname {
				return fmt.Errorf("entry %s links to %s", header.Name, header.Linkname)
			}
		default:
			got, _, err := readerDigest(ctx, archive)
			if err != nil {
				return err
			}
			if got != expected {
				return fmt.Errorf("entry %s hashes to %s, not %s", header.Name, got, expected)
			}
		}
		seen[header.Name] = true
	}
	if count != manifest.Entries || len(seen) != len(want) {
		return fmt.Errorf("the archive holds %d entries, the manifest %d", count, len(want))
	}
	for _, recipe := range manifest.Recipes {
		switch recipe.Kind {
		case RecipeBlob:
			if !seen[".blobs/"+recipe.SHA256] {
				return fmt.Errorf("recipe %s names blob %s, which the archive lacks", recipe.Path, recipe.SHA256)
			}
		case RecipeGzip, RecipeGit:
			if !seen[recipe.Replacement] {
				return fmt.Errorf("recipe %s names %s, which the archive lacks", recipe.Path, recipe.Replacement)
			}
		}
	}
	return nil
}

func flipByte(path string) {
	data, err := os.ReadFile(path)
	if err != nil || len(data) < 64 {
		return
	}
	data[len(data)/2] ^= 0xff
	_ = os.WriteFile(path, data, 0o644)
}

// ExportDirProblem is the damage an export directory would do (H1: the
// only refusal): inside an evidence root it would be measured and disposed
// of as evidence; inside a checkout or a registered store it would be
// removed with them. roots, checkouts and stores are absolute paths.
func ExportDirProblem(dir string, roots, checkouts, stores []string) string {
	for _, group := range [][]string{roots, checkouts, stores} {
		for _, inside := range group {
			if under(dir, inside) {
				return "an export must live outside every evidence root and checkout; " + dir + " is inside " + inside +
					"; choose an external volume or a directory such as /Volumes/Backup/metasystem-exports and set " + config.DiskEvidenceExportDirKey + " in metasystem.conf.local so --to can be omitted"
			}
		}
	}
	return ""
}
