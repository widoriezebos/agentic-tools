package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

const fixtureLeashEnvironment = "METASYSTEM_FIXTURE_LEASH"

type fixtureLifetimeDependencies struct {
	getenv    func(string) string
	prober    identity.Prober
	after     func(time.Duration) <-chan time.Time
	openLeash func(string) (io.ReadCloser, error)
	ready     func()
}

func defaultFixtureLifetimeDependencies() fixtureLifetimeDependencies {
	return fixtureLifetimeDependencies{
		getenv: os.Getenv, prober: identity.KernelProber{}, after: time.After,
		openLeash: openFixtureLeash, ready: func() {},
	}
}

func openFixtureLeash(path string) (io.ReadCloser, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("read fixture leash: %w", err)
	}
	if info.Mode()&os.ModeNamedPipe == 0 {
		return nil, fmt.Errorf("fixture leash %q is not a pipe", path)
	}
	descriptor, err := unix.Open(path, unix.O_RDONLY|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("open fixture leash: %w", err)
	}
	if err := unix.SetNonblock(descriptor, false); err != nil {
		_ = unix.Close(descriptor)
		return nil, fmt.Errorf("arm fixture leash: %w", err)
	}
	return os.NewFile(uintptr(descriptor), path), nil
}

func validateFixtureOwner(value string, prober identity.Prober) error {
	key, err := identity.ParseKey(value)
	if err != nil {
		return fmt.Errorf("fixture lifetime owner is malformed: %w", err)
	}
	exact, state, err := prober.Probe(key.Owner.Pid)
	if err != nil || state != identity.Alive || !identity.SameIdentity(exact, key.Owner) {
		return fmt.Errorf("fixture lifetime owner does not match the live exact process")
	}
	return nil
}

func fixtureLifetimeContext(parent context.Context, maxSeconds int64, deps fixtureLifetimeDependencies) (context.Context, context.CancelFunc, error) {
	if maxSeconds < 0 || maxSeconds > int64(^uint64(0)>>1)/int64(time.Second) {
		return nil, nil, fmt.Errorf("--max-seconds must be a non-negative integer of seconds")
	}
	owner := deps.getenv(identity.FixtureOwnerEnv)
	leashPath := deps.getenv(fixtureLeashEnvironment)
	if owner != "" {
		if err := validateFixtureOwner(owner, deps.prober); err != nil {
			return nil, nil, err
		}
	}
	if leashPath != "" && owner == "" {
		return nil, nil, fmt.Errorf("fixture leash requires an exact fixture owner")
	}
	if leashPath == "" && maxSeconds == 0 {
		return nil, nil, fmt.Errorf("an exact fixture leash or --max-seconds is required")
	}

	ctx, cancel := context.WithCancel(parent)
	var leash io.ReadCloser
	var leashDone <-chan struct{}
	if leashPath != "" {
		var err error
		leash, err = deps.openLeash(leashPath)
		if err != nil {
			cancel()
			return nil, nil, err
		}
		done := make(chan struct{})
		leashDone = done
		go func() {
			var one [1]byte
			_, _ = leash.Read(one[:])
			close(done)
		}()
	}
	var expiry <-chan time.Time
	if maxSeconds > 0 {
		expiry = deps.after(time.Duration(maxSeconds) * time.Second)
	}
	go func() {
		select {
		case <-ctx.Done():
		case <-leashDone:
		case <-expiry:
		}
		if leash != nil {
			_ = leash.Close()
		}
		cancel()
	}()
	return ctx, func() {
		if leash != nil {
			_ = leash.Close()
		}
		cancel()
	}, nil
}

// runUtilHold parks the process until it is told to stop. Fixtures use it as
// a stand-in child whose whole job is to exist: the --tag value rides in the
// process's command line, so the census and reaper can recognize the process
// by scanning the process table for the tag. A handled signal, owner-leash
// close, or explicit expiry is an orderly stop: the process writes "stopped" to
// --stopped-file when one is given and exits 0. A supervisor can therefore
// distinguish a child that completed its bounded lifetime from one that was
// killed outright.
func runUtilHold(args []string) int {
	return runUtilHoldWithDependencies(args, defaultFixtureLifetimeDependencies())
}

func runUtilHoldWithDependencies(args []string, deps fixtureLifetimeDependencies) int {
	flags := flag.NewFlagSet("util hold", flag.ContinueOnError)
	tag := flags.String("tag", "", "instance tag carried in this process's command line")
	stoppedFile := flags.String("stopped-file", "", "file that receives \"stopped\" on an orderly stop")
	readyFile := flags.String("ready-file", "", "file written after signal handling and lifetime custody are armed")
	maxSecondsText := flags.String("max-seconds", "0", "terminal lifetime when no owner leash closes")
	ignoreTerm := flags.Bool("ignore-term", false, "leave SIGTERM ignored while an owner leash controls lifetime")
	termObservedFile := flags.String("term-observed-file", "", "fixture acknowledgement written after a resisted SIGTERM")
	if flags.Parse(args) != nil {
		return 2
	}
	if *tag == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem util hold --tag TAG [--stopped-file FILE] [--ready-file FILE] [--max-seconds N] [--ignore-term]")
		return 2
	}
	maxSeconds, err := strconv.ParseInt(*maxSecondsText, 10, 64)
	if err != nil || maxSeconds < 0 {
		fmt.Fprintln(os.Stderr, "util hold: --max-seconds must be a non-negative integer of seconds")
		return 2
	}
	if *termObservedFile != "" && !*ignoreTerm {
		fmt.Fprintln(os.Stderr, "util hold: --term-observed-file requires --ignore-term")
		return 2
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, os.Interrupt)
	defer signal.Stop(signals)
	signalContext, stopSignal := context.WithCancel(context.Background())
	defer stopSignal()
	signalFailure := make(chan error, 1)
	go func() {
		for {
			select {
			case received := <-signals:
				if received == syscall.SIGTERM && *ignoreTerm {
					if *termObservedFile != "" {
						if err := os.WriteFile(*termObservedFile, []byte("term-observed\n"), 0o600); err != nil {
							signalFailure <- err
							stopSignal()
							return
						}
					}
					continue
				}
				stopSignal()
				return
			case <-signalContext.Done():
				return
			}
		}
	}()
	ctx, stopLifetime, err := fixtureLifetimeContext(signalContext, maxSeconds, deps)
	if err != nil {
		fmt.Fprintln(os.Stderr, "util hold:", err)
		return 2
	}
	defer stopLifetime()
	deps.ready()
	if *readyFile != "" {
		if err := os.WriteFile(*readyFile, []byte("ready\n"), 0o600); err != nil {
			fmt.Fprintln(os.Stderr, "util hold: publish readiness:", err)
			return 1
		}
	}
	<-ctx.Done()
	select {
	case err := <-signalFailure:
		fmt.Fprintln(os.Stderr, "util hold: acknowledge resisted SIGTERM:", err)
		return 1
	default:
	}

	if *stoppedFile != "" {
		if err := os.WriteFile(*stoppedFile, []byte("stopped\n"), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}
	return 0
}
