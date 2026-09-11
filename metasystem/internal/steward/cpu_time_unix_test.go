//go:build unix

package steward

import (
	"syscall"
	"time"
)

// processCPUTime is the user plus system CPU time this process has consumed;
// tests that bound a cost use it instead of the wall clock.
func processCPUTime() time.Duration {
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
		return 0
	}
	return time.Duration(usage.Utime.Nano() + usage.Stime.Nano())
}
