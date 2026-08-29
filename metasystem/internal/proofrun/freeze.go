package proofrun

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type FrozenExport struct {
	Digest string
	Root   string
}

// Freeze exports one manifest projection and publishes it only when both the
// exported projection and a fresh read of the source still equal the initial
// digest. The gate can therefore use the export without consulting live bytes.
func Freeze(root string) (result FrozenExport, err error) {
	before, err := readManifest(root)
	if err != nil {
		return FrozenExport{}, err
	}
	parent, err := os.MkdirTemp("", "metasystem-witness-freeze-")
	if err != nil {
		return FrozenExport{}, fmt.Errorf("create private frozen-export directory: %w", err)
	}
	if err := os.Chmod(parent, 0700); err != nil {
		os.RemoveAll(parent)
		return FrozenExport{}, fmt.Errorf("protect frozen-export directory: %w", err)
	}
	published := false
	defer func() {
		if !published {
			_ = os.RemoveAll(parent)
		}
	}()

	exportRoot := filepath.Join(parent, "tree")
	if err := os.Mkdir(exportRoot, 0700); err != nil {
		return FrozenExport{}, fmt.Errorf("create frozen-export tree: %w", err)
	}
	if err := exportEntries(root, exportRoot, before.entries); err != nil {
		return FrozenExport{}, err
	}
	exported, err := readManifest(exportRoot)
	if err != nil {
		return FrozenExport{}, fmt.Errorf("verify frozen export: %w", err)
	}
	after, err := readManifest(root)
	if err != nil {
		return FrozenExport{}, fmt.Errorf("re-read source after frozen export: %w", err)
	}
	if before.digest != exported.digest || before.digest != after.digest {
		return FrozenExport{}, fmt.Errorf("frozen export voided because the source changed while it was copied: before %s, export %s, after %s",
			before.digest, exported.digest, after.digest)
	}
	published = true
	return FrozenExport{Digest: before.digest, Root: exportRoot}, nil
}

func exportEntries(sourceRoot, exportRoot string, entries []entry) error {
	directories := make([]entry, 0)
	for _, item := range entries {
		source := filepath.Join(sourceRoot, filepath.FromSlash(item.path))
		destination := filepath.Join(exportRoot, filepath.FromSlash(item.path))
		switch item.kind {
		case 'd':
			if err := os.Mkdir(destination, 0700); err != nil {
				return fmt.Errorf("export directory %q: %w", item.path, err)
			}
			directories = append(directories, item)
		case 'l':
			if err := os.MkdirAll(filepath.Dir(destination), 0700); err != nil {
				return fmt.Errorf("export symlink parent %q: %w", item.path, err)
			}
			target, err := os.Readlink(source)
			if err != nil {
				return fmt.Errorf("export symlink %q: %w", item.path, err)
			}
			if err := os.Symlink(target, destination); err != nil {
				return fmt.Errorf("write exported symlink %q: %w", item.path, err)
			}
		case 'f':
			// Directories are no longer manifest entries wherever the
			// projection skips their bare names; each file creates its
			// own parents.
			if err := os.MkdirAll(filepath.Dir(destination), 0700); err != nil {
				return fmt.Errorf("export file parent %q: %w", item.path, err)
			}
			if err := copyFile(source, destination, exportedMode(item)); err != nil {
				return fmt.Errorf("export file %q: %w", item.path, err)
			}
		default:
			return fmt.Errorf("export entry %q has unknown kind %q", item.path, item.kind)
		}
	}
	for i := len(directories) - 1; i >= 0; i-- {
		item := directories[i]
		if err := os.Chmod(filepath.Join(exportRoot, filepath.FromSlash(item.path)), exportedMode(item)); err != nil {
			return fmt.Errorf("set exported directory mode %q: %w", item.path, err)
		}
	}
	return nil
}

func exportedMode(item entry) os.FileMode {
	if item.kind == 'd' {
		if item.executable {
			return 0755
		}
		return 0644
	}
	if item.executable {
		return 0755
	}
	return 0644
}

func copyFile(source, destination string, mode os.FileMode) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}
