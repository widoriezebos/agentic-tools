//go:build !darwin && !linux

package direct

import "os"

// Elsewhere syncs in a file no audited platform builds.
func Elsewhere(f *os.File) error {
	return f.Sync()
}
