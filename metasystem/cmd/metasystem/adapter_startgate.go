package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/adapter"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

const startGateFixtureExpiryEventEnvironment = "METASYSTEM_START_GATE_EXPIRY_EVENT"

func startGateEvents(ctx context.Context, interval time.Duration) <-chan struct{} {
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

func runAdapterWaitStartGate(args []string) int {
	flags := flag.NewFlagSet("adapter wait-start-gate", flag.ContinueOnError)
	path := flags.String("path", "", "runner-owned gate path")
	root := pathFlag(flags, "root", "", "checkout root authorizing fixture-only expiry events")
	timeoutSec := flags.Int("timeout-sec", 0, "gate deadline in seconds")
	pollMS := flags.Int("poll-ms", 20, "filesystem event observation interval")
	if flags.Parse(args) != nil {
		return 2
	}
	if *path == "" || *timeoutSec < 1 || *pollMS < 1 || len(flags.Args()) != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem adapter wait-start-gate --path FILE --timeout-sec N [--poll-ms N]")
		return 2
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	after := time.After
	if eventPath := os.Getenv(startGateFixtureExpiryEventEnvironment); eventPath != "" {
		if *root == "" || !fixtureauth.FixtureModeRoot(*root) {
			fmt.Fprintln(os.Stderr, "adapter wait-start-gate: fixture expiry event requires a fake-runtime root")
			return 1
		}
		if err := validateFixtureOwner(os.Getenv(identity.FixtureOwnerEnv), identity.KernelProber{}); err != nil {
			fmt.Fprintln(os.Stderr, "adapter wait-start-gate: fixture expiry event requires its exact owner")
			return 1
		}
		after = func(time.Duration) <-chan time.Time {
			fired := make(chan time.Time, 1)
			go func() {
				ticker := time.NewTicker(time.Duration(*pollMS) * time.Millisecond)
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
	_, err := adapter.WaitStartGate(ctx, time.Duration(*timeoutSec)*time.Second, adapter.StartGateDependencies{
		Now: time.Now, After: after, Events: startGateEvents(ctx, time.Duration(*pollMS)*time.Millisecond),
		Exists: func() bool { _, err := os.Stat(*path); return err == nil },
	})
	if errors.Is(err, adapter.ErrStartGateExpired) {
		return 3
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "adapter wait-start-gate:", err)
		return 1
	}
	return 0
}
