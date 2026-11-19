// Package broken type-checks on darwin only: the audit refuses what it cannot
// resolve on linux.
package broken

import "os"

// Flush syncs through a helper only darwin defines.
func Flush(f *os.File) error { return flush(f) }
