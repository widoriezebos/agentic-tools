package launch

import "golang.org/x/sys/unix"

// groupListed includes zombies: a zombie-only group answers EPERM to a signal
// but is still present in the kernel's process list.
func groupListed(pgid int64) (bool, error) {
	members, err := unix.SysctlKinfoProcSlice("kern.proc.pgrp", int(pgid))
	return len(members) > 0, err
}
