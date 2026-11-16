// Package fileclone copies open files into independent, synced files, sharing
// data blocks where the filesystem supports copy-on-write clones.
package fileclone

import (
	"errors"
	"io"
	"os"
)

// Copy creates destination exclusively with mode and the full contents of
// source. It never reopens source's pathname. Any clone failure falls back to
// copying from that descriptor. The caller owns publication of the new file.
func Copy(source *os.File, destination string, mode os.FileMode) error {
	return copyWith(source, destination, mode, clone)
}

func copyWith(source *os.File, destination string, mode os.FileMode, clone func(*os.File, string) (*os.File, error)) (err error) {
	output, err := clone(source, destination)
	if err != nil {
		output, err = os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		if _, err = source.Seek(0, io.SeekStart); err == nil {
			_, err = io.Copy(output, source)
		}
	}
	defer func() {
		err = errors.Join(err, output.Close())
		if err != nil {
			_ = os.Remove(destination)
		}
	}()
	if err != nil {
		return err
	}
	if err = output.Chmod(mode); err != nil {
		return err
	}
	return output.Sync()
}
