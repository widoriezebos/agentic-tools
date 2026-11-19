package direct

import "os"

// OnLinux syncs in a file only linux builds.
func OnLinux(f *os.File) error {
	return f.Sync()
}
