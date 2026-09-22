package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hooks"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

const stopDeadlineFixtureEventEnvironment = "METASYSTEM_STOP_DEADLINE_EVENT"

func stopDeadlineEvents(ctx context.Context, interval time.Duration) <-chan struct{} {
	events := make(chan struct{}, 1)
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		defer close(events)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				select {
				case events <- struct{}{}:
				default:
				}
			}
		}
	}()
	return events
}

func runHooksStopDeadlineWait(args []string) int {
	flags := flag.NewFlagSet("hooks stop-deadline-wait", flag.ContinueOnError)
	pid := flags.Int64("pid", 0, "Stop worker pid")
	root := pathFlag(flags, "root", "", "checkout root authorizing fixture-only deadline events")
	timeout := flags.Int("timeout-sec", 0, "worker deadline in seconds")
	bootID := flags.String("boot-id", "", "boot identity sampled at worker launch")
	originBootNS := flags.Int64("origin-boot-ns", -1, "monotonic boot nanoseconds at worker launch")
	deadlineBootNS := flags.Int64("deadline-boot-ns", -1, "absolute monotonic worker deadline")
	if flags.Parse(args) != nil {
		return 2
	}
	timeoutDuration := time.Duration(*timeout) * time.Second
	if *pid < 1 || *timeout < 1 || int(timeoutDuration/time.Second) != *timeout || len(flags.Args()) != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem hooks stop-deadline-wait --pid PID --timeout-sec N --boot-id ID --origin-boot-ns N --deadline-boot-ns N [--root ROOT]")
		return 2
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	after := time.After
	if eventPath := os.Getenv(stopDeadlineFixtureEventEnvironment); eventPath != "" {
		if *root == "" || !fixtureauth.FixtureModeRoot(*root) {
			fmt.Fprintln(os.Stderr, "hooks stop-deadline-wait: fixture deadline event requires a fake-runtime root")
			return 1
		}
		if err := validateFixtureOwner(os.Getenv(identity.FixtureOwnerEnv), identity.KernelProber{}); err != nil {
			fmt.Fprintln(os.Stderr, "hooks stop-deadline-wait: fixture deadline event requires its exact owner")
			return 1
		}
		after = func(time.Duration) <-chan time.Time {
			fired := make(chan time.Time, 1)
			go func() {
				ticker := time.NewTicker(20 * time.Millisecond)
				defer ticker.Stop()
				for {
					if _, err := os.Stat(eventPath); err == nil {
						fired <- time.Now()
						return
					}
					select {
					case <-ctx.Done():
						return
					case <-ticker.C:
					}
				}
			}()
			return fired
		}
	}
	if *bootID == "" || *originBootNS < 0 || *deadlineBootNS < *originBootNS ||
		time.Duration(*deadlineBootNS-*originBootNS) != timeoutDuration {
		fmt.Fprintln(os.Stderr, "hooks stop-deadline-wait: invalid boot deadline boundary")
		return 2
	}
	result, err := hooks.WaitForStopDeadline(ctx, *pid, hooks.StopDeadlineBootBoundary{
		BootID: *bootID, Origin: time.Duration(*originBootNS), Deadline: time.Duration(*deadlineBootNS),
	}, hooks.StopDeadlineDependencies{
		Now: time.Now, After: after, Events: stopDeadlineEvents(ctx, 20*time.Millisecond),
		Prober: identity.KernelProber{}, BootClock: identity.BootClock,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "hooks stop-deadline-wait:", err)
		return 1
	}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fmt.Fprintln(os.Stderr, "hooks stop-deadline-wait:", err)
		return 1
	}
	return 0
}

type stopDeadlineCleanupDependencies struct {
	prober    identity.Prober
	parentPid func(int64) (int64, bool)
	parent    int64
	deadline  hooks.StopDeadlineDependencies
}

func cleanupStopDeadlineWorker(ctx context.Context, worker *identity.Ref, grace time.Duration, deps stopDeadlineCleanupDependencies) (hooks.StopDeadlineCleanupResult, error) {
	if worker == nil {
		return hooks.StopDeadlineCleanupResult{State: hooks.StopWorkerUnverified}, nil
	}
	exact, state, err := deps.prober.Probe(worker.Pid)
	switch {
	case state == identity.Dead:
		return hooks.StopDeadlineCleanupResult{State: hooks.StopWorkerGone}, nil
	case err != nil || state == identity.Unknown:
		return hooks.StopDeadlineCleanupResult{State: hooks.StopWorkerUnverified}, nil
	case exact.Zombie || !identity.SameIdentity(exact, *worker):
		return hooks.StopDeadlineCleanupResult{State: hooks.StopWorkerGone}, nil
	}
	if parent, known := deps.parentPid(worker.Pid); !known || parent != deps.parent {
		return hooks.StopDeadlineCleanupResult{State: hooks.StopWorkerUnverified}, nil
	}
	return hooks.StopDeadlineWorker(ctx, worker, grace, deps.deadline), nil
}

func runHooksStopDeadlineCleanup(args []string) int {
	flags := flag.NewFlagSet("hooks stop-deadline-cleanup", flag.ContinueOnError)
	waitResult := flags.String("wait-result", "", "JSON result from stop-deadline-wait")
	pid := flags.Int64("pid", 0, "direct child pid (resolver cleanup)")
	graceMS := flags.Int("term-grace-ms", 200, "TERM grace before KILL")
	if flags.Parse(args) != nil {
		return 2
	}
	if (*waitResult == "") == (*pid == 0) || *pid < 0 || *graceMS < 0 || len(flags.Args()) != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem hooks stop-deadline-cleanup (--wait-result FILE | --pid PID) [--term-grace-ms N]")
		return 2
	}
	var waited hooks.StopDeadlineWaitResult
	if *waitResult != "" {
		file, err := os.Open(*waitResult)
		if err != nil {
			fmt.Fprintln(os.Stderr, "hooks stop-deadline-cleanup:", err)
			return 1
		}
		err = json.NewDecoder(file).Decode(&waited)
		closeErr := file.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil || waited.State != hooks.StopDeadlineReached {
			fmt.Fprintln(os.Stderr, "hooks stop-deadline-cleanup: wait result is not a reached deadline")
			return 1
		}
	} else {
		exact, state, err := (identity.KernelProber{}).Probe(*pid)
		if err != nil || state == identity.Unknown {
			waited.Worker = nil
		} else if state == identity.Alive {
			ref := exact.Ref()
			waited.Worker = &ref
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	prober := identity.KernelProber{}
	deadlineDeps := hooks.StopDeadlineDependencies{
		Now: time.Now, After: time.After, Events: stopDeadlineEvents(ctx, 20*time.Millisecond), Prober: prober,
	}
	result, err := cleanupStopDeadlineWorker(ctx, waited.Worker, time.Duration(*graceMS)*time.Millisecond, stopDeadlineCleanupDependencies{
		prober: prober, parentPid: identity.ParentPid, parent: int64(os.Getppid()), deadline: deadlineDeps,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "hooks stop-deadline-cleanup:", err)
		return 1
	}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fmt.Fprintln(os.Stderr, "hooks stop-deadline-cleanup:", err)
		return 1
	}
	return 0
}
