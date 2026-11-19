package direct

import "os"

// OnDarwin syncs in a file only darwin builds.
func OnDarwin(f *os.File) error {
	return f.Sync()
}
