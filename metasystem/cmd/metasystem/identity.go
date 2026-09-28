package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
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

// runIdentityStartedAt prints a pid's start time in epoch seconds on
// stdout, exiting 1 with no output when the pid cannot be read.
func runIdentityStartedAt(args []string) int {
	flags := flag.NewFlagSet("proc started-at", flag.ContinueOnError)
	pid := flags.Int64("pid", 0, "process id")
	emit := flags.String("emit", "seconds", "output form: 'seconds' (default) or 'pair' (SECONDS TICKS BOOTID)")
	if flags.Parse(args) != nil {
		return 2
	}
	exact, state, err := identity.KernelProber{}.Probe(*pid)
	if err != nil || state != identity.Alive {
		return 1
	}
	switch *emit {
	case "seconds":
		fmt.Println(exact.StartedAt.Unix())
	case "pair":
		// SECONDS TICKS BOOTID on one line; TICKS=0 BOOTID="-" on darwin,
		// where the second is already clock-step stable. A "-" boot id is
		// the explicit empty marker so the shell can read three fields.
		boot := exact.BootID
		if boot == "" {
			boot = "-"
		}
		fmt.Printf("%d %d %s\n", exact.StartedAt.Unix(), exact.StartTicks, boot)
	default:
		fmt.Fprintln(os.Stderr, "proc started-at: --emit must be seconds or pair")
		return 2
	}
	return 0
}
