package fileclone

import (
	"os"

	"golang.org/x/sys/unix"
)

func clone(source *os.File, destination string) (*os.File, error) {
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	if err := unix.IoctlFileClone(int(output.Fd()), int(source.Fd())); err != nil {
		_ = output.Close()
		_ = os.Remove(destination)
		return nil, err
	}
	return output, nil
}
