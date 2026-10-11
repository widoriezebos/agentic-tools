package rehearsal

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// unpackArchive materializes Git's archive inside the snapshot root.
func unpackArchive(path, directory string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	root, err := os.OpenRoot(directory)
	if err != nil {
		return err
	}
	defer root.Close()
	reader := tar.NewReader(file)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		// Git's global PAX header carries the commit id, not a file.
		if header.Typeflag == tar.TypeXGlobalHeader {
			continue
		}
		if err = root.MkdirAll(filepath.Dir(header.Name), 0700); err != nil {
			return err
		}
		switch header.Typeflag {
		case tar.TypeDir:
			err = root.MkdirAll(header.Name, os.FileMode(header.Mode).Perm())
			if err == nil {
				err = root.Chmod(header.Name, os.FileMode(header.Mode).Perm())
			}
		case tar.TypeReg:
			var output *os.File
			output, err = root.OpenFile(header.Name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, os.FileMode(header.Mode).Perm())
			if err == nil {
				_, copyErr := io.Copy(output, reader)
				err = errors.Join(copyErr, output.Close())
			}
		case tar.TypeSymlink:
			err = root.Symlink(header.Linkname, header.Name)
		default:
			err = fmt.Errorf("unsupported candidate archive entry %s (type %d)", header.Name, header.Typeflag)
		}
		if err != nil {
			return err
		}
	}
}
