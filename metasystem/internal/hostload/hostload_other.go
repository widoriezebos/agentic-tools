//go:build !darwin && !linux

package hostload

import "errors"

func readLoad() (float64, float64, float64, error) {
	return 0, 0, 0, errors.New("host load is not readable on this platform")
}
