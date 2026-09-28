package main

import (
	"fmt"
	"os"
	"strconv"

	"golang.org/x/sys/unix"
)

// closeInheritedDescriptorsAbove closes every descriptor above floor that
// lacks close-on-exec. Go opens its own descriptors close-on-exec and hands a
// child only 0-2 and ExtraFiles, so a descriptor above floor without the flag
// was inherited from an ancestor by accident: a shell's flock descriptor, for
// one, which would otherwise be held by this process and every child it
// starts for as long as any of them lives.
func closeInheritedDescriptorsAbove(floor int) error {
	entries, err := os.ReadDir("/dev/fd")
	if err != nil {
		return fmt.Errorf("list open descriptors: %w", err)
	}
	for _, entry := range entries {
		descriptor, parseErr := strconv.Atoi(entry.Name())
		if parseErr != nil || descriptor <= floor {
			continue
		}
		flags, flagErr := unix.FcntlInt(uintptr(descriptor), unix.F_GETFD, 0)
		if flagErr == unix.EBADF {
			continue
		}
		if flagErr != nil {
			return fmt.Errorf("inspect descriptor %d: %w", descriptor, flagErr)
		}
		if flags&unix.FD_CLOEXEC == 0 {
			if err := unix.Close(descriptor); err != nil && err != unix.EBADF {
				return fmt.Errorf("close inherited descriptor %d: %w", descriptor, err)
			}
		}
	}
	return nil
}
