package identity

import "time"

// ProcessBirth returns the kernel-recorded start time of a live process.
// Missing, zombie, and unreadable processes have no usable birth time.
func ProcessBirth(pid int64) (time.Time, bool) {
	exact, state, err := (KernelProber{}).ReadStart(pid)
	if err != nil || state != Alive || exact.Zombie || exact.StartedAt.IsZero() {
		return time.Time{}, false
	}
	return exact.StartedAt, true
}
