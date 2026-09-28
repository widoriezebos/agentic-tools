package steward

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// extractArchivedTree unpacks a git-archive tar into destination natively
// (R-138-m1e, Go decides natively; it replaced a `tar -xvf` shell-out). It
// keeps what the witness digest reads: directories, regular files with their
// permission bits, and symlinks with their targets. Every entry is reported
// as progress, as tar's per-entry line was, so the step's silence bound
// measures a stalled extraction and not a large one. A pax global header
// (git's commit comment) is skipped; any other entry kind, and any name that
// would land outside destination, is refused.
func extractArchivedTree(ctx context.Context, archivePath, destination string, progress func()) error {
	archive, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer archive.Close()
	reader := tar.NewReader(archive)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read archive entry: %w", err)
		}
		progress()
		if header.Typeflag == tar.TypeXGlobalHeader {
			continue
		}
		target, err := archiveEntryPath(destination, header.Name)
		if err != nil {
			return err
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			file, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, header.FileInfo().Mode().Perm())
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(file, reader)
			closeErr := file.Close()
			if copyErr != nil {
				return fmt.Errorf("extract %s: %w", header.Name, copyErr)
			}
			if closeErr != nil {
				return closeErr
			}
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			if err := os.Symlink(header.Linkname, target); err != nil {
				return err
			}
		default:
			return fmt.Errorf("archive entry %q has unsupported kind %q", header.Name, header.Typeflag)
		}
	}
}

// archiveEntryPath resolves an entry name under destination and refuses an
// absolute name or one that climbs out of it.
func archiveEntryPath(destination, name string) (string, error) {
	cleaned := filepath.Clean(filepath.FromSlash(name))
	if filepath.IsAbs(cleaned) || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("archive entry %q lands outside the extraction directory", name)
	}
	return filepath.Join(destination, cleaned), nil
}
