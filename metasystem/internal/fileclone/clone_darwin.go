package fileclone

import (
	"os"

	"golang.org/x/sys/unix"
)

func clone(source *os.File, destination string) (*os.File, error) {
	if err := unix.Fclonefileat(int(source.Fd()), unix.AT_FDCWD, destination, 0); err != nil {
		return nil, err
	}
	output, err := os.Open(destination)
	if err != nil {
		_ = os.Remove(destination)
	}
	return output, err
}
