package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"syscall"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

func runIdentityRef(args []string) int {
	return runIdentityRefWithProber(args, identity.KernelProber{}, os.Stdout)
}

func runIdentityRefWithProber(args []string, prober identity.Prober, output io.Writer) int {
	flags := flag.NewFlagSet("proc ref", flag.ContinueOnError)
	pid := flags.Int64("pid", 0, "process id")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		return 2
	}
	exact, state, err := prober.Probe(*pid)
	if err != nil || state != identity.Alive {
		return 1
	}
	encoded, err := identity.EncodeRef(exact.Ref())
	if err != nil {
		return 1
	}
	if _, err := fmt.Fprintln(output, encoded); err != nil {
		return 1
	}
	return 0
}

func runFixtureCustodian(args []string) int {
	if _, present := os.LookupEnv(identity.FixtureOwnerEnv); present {
		fmt.Fprintln(os.Stderr, "proc custodian: "+identity.FixtureOwnerEnv+" is set; start the custodian without it")
		return 2
	}
	flags := flag.NewFlagSet("proc custodian", flag.ContinueOnError)
	ownerValue := flags.String("owner", "", "exact fixture owner reference")
	logPath := flags.String("log", os.Getenv(identity.FixtureCustodianLogEnv), "custodian log path")
	if flags.Parse(args) != nil {
		return 2
	}
	owner, err := identity.ParseRef(*ownerValue)
	if err != nil {
		fmt.Fprintln(os.Stderr, "proc custodian: invalid --owner:", err)
		return 2
	}
	if *logPath == "" {
		fmt.Fprintln(os.Stderr, "proc custodian: --log or "+identity.FixtureCustodianLogEnv+" is required")
		return 2
	}
	var watchStat unix.Stat_t
	if err := unix.Fstat(3, &watchStat); err != nil {
		fmt.Fprintln(os.Stderr, "proc custodian: watch descriptor 3 is unavailable; it must be a pipe:", err)
		return 2
	}
	if watchStat.Mode&unix.S_IFMT != unix.S_IFIFO {
		fmt.Fprintf(os.Stderr, "proc custodian: watch descriptor 3 is mode %#o, not a pipe\n", watchStat.Mode&unix.S_IFMT)
		return 2
	}
	watchFlags, err := unix.FcntlInt(uintptr(3), unix.F_GETFL, 0)
	if err != nil || watchFlags&unix.O_ACCMODE != unix.O_RDONLY {
		fmt.Fprintf(os.Stderr, "proc custodian: watch descriptor 3 is not a pipe read end: flags=%#x err=%v\n", watchFlags, err)
		return 2
	}
	if _, err := syscall.Setsid(); err != nil {
		sid, sidErr := unix.Getsid(0)
		if sidErr != nil || sid != os.Getpid() {
			fmt.Fprintf(os.Stderr, "proc custodian: create session: %v (current session %d: %v)\n", err, sid, sidErr)
			return 2
		}
	}
	nullFile, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		fmt.Fprintln(os.Stderr, "proc custodian: open null device:", err)
		return 2
	}
	defer nullFile.Close()
	for _, descriptor := range []int{0, 1} {
		if err := unix.Dup2(int(nullFile.Fd()), descriptor); err != nil {
			fmt.Fprintf(os.Stderr, "proc custodian: redirect descriptor %d: %v\n", descriptor, err)
			return 2
		}
	}
	logFile, err := os.OpenFile(*logPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		fmt.Fprintln(os.Stderr, "proc custodian: open log:", err)
		return 2
	}
	defer logFile.Close()
	if err := unix.Dup2(int(logFile.Fd()), 2); err != nil {
		fmt.Fprintln(os.Stderr, "proc custodian: redirect descriptor 2:", err)
		return 2
	}
	if err := closeInheritedDescriptors(); err != nil {
		fmt.Fprintln(os.Stderr, "proc custodian:", err)
		return 2
	}
	watch := os.NewFile(3, "fixture-owner-watch")
	defer watch.Close()
	if err := identity.RunCustodian(owner, watch, logFile); err != nil {
		fmt.Fprintln(os.Stderr, "proc custodian:", err)
		return 1
	}
	return 0
}

func closeInheritedDescriptors() error {
	entries, err := os.ReadDir("/dev/fd")
	if err != nil {
		return fmt.Errorf("list open descriptors: %w", err)
	}
	for _, entry := range entries {
		descriptor, parseErr := strconv.Atoi(entry.Name())
		if parseErr != nil || descriptor <= 3 {
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

// runIdentityProbe prints the full exact identity as JSON, used by
// fixtures and diagnostics.
func runIdentityProbe(args []string) int {
	flags := flag.NewFlagSet("proc probe", flag.ContinueOnError)
	pid := flags.Int64("pid", 0, "process id")
	if flags.Parse(args) != nil {
		return 2
	}
	exact, state, err := identity.KernelProber{}.Probe(*pid)
	result := map[string]any{"pid": *pid, "liveness": state.String()}
	if err != nil {
		result["error"] = err.Error()
	}
	if state == identity.Alive {
		result["startedAt"] = exact.StartedAt.Format("2006-01-02T15:04:05.000000Z07:00")
		result["startedAtUnix"] = exact.StartedAt.Unix()
		result["startedAtUnixMicro"] = exact.StartedAt.UnixMicro()
		result["startTicks"] = exact.StartTicks
		result["bootId"] = exact.BootID
		result["argv"] = exact.Argv
		terminalID, terminalKnown := identity.ControllingTerminalIdentity(*pid)
		result["terminalKnown"] = terminalKnown
		result["terminalId"] = terminalID
		if sessionLeader, sessionErr := unix.Getsid(int(*pid)); sessionErr == nil {
			result["sessionLeaderPid"] = sessionLeader
		} else {
			result["sessionLeaderError"] = sessionErr.Error()
		}
	}
	encoded, marshalErr := json.Marshal(result)
	if marshalErr != nil {
		fmt.Fprintln(os.Stderr, "proc probe:", marshalErr)
		return 1
	}
	fmt.Println(string(encoded))
	if state == identity.Unknown {
		return 1
	}
	return 0
}
