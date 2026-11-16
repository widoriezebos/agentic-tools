//go:build !darwin && !linux

package fileclone

import (
	"errors"
	"os"
)

func clone(source *os.File, destination string) (*os.File, error) {
	return nil, errors.New("file cloning unavailable")
}
