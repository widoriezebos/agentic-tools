package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/registry"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise"
)

// procfsMounts is the mount table the restricted-procfs arming refusal reads; a test
// points it at a fixture to prove the refusal wiring end to end.
var procfsMounts = "/proc/self/mounts"

// runSuperviseOwner runs the Go owner loop for one checkout: it
// assembles the disk, process, ledger, and intent adapters behind the
// tested owner and drives it. Launched by arm-supervision.sh under the
// engine switch; it supervises until an exit condition (D-1) fires,
// tears down by held identity, and records its terminal.
//
// Observability: every cycle narrates one JSON line to the owner log
// (the extreme-observability ruling), and the terminal exit is both
// logged and appended to the registry.
func runSuperviseOwnerLoop(args []string) int {
	flags := flag.NewFlagSet("supervise owner", flag.ContinueOnError)
	repo := flags.String("repo", "", "checkout root")
	scope := flags.String("scope", "", "census scope (git toplevel); defaults to --repo")
	tag := flags.String("tag", "", "owner instance tag")
	intervalSec := flags.Int("interval", 60, "base observation interval seconds")
	fingerprint := flags.String("fingerprint", "", "supervision fingerprint the armer computed; published in state.json and matched against the watcher's census verdicts")
	watcherCap := flags.Int("watcher-cap", 0, "derived watcher cap ceiling in minutes; published as derivedWatcherCapMin and loaded into the watcher heartbeat attestation")
	registryPath := flags.String("registry", defaultRegistryPath(), "machine-wide registry file")
	gate := flags.String("gate", "", "start-gate file: wait for it to appear, then delete it, before supervising (the armer publishes the lock, then signals the gate — avoids the lock/pid chicken-and-egg)")
	if flags.Parse(args) != nil {
		return 2
	}
	if *repo == "" || *tag == "" {
		fmt.Fprintln(os.Stderr, "supervise owner: --repo and --tag are required")
		return 2
	}
	// Restricted procfs breaks the three-way liveness guarantee (a live
	// foreign process reads as ENOENT-dead), so supervision refuses to arm
	// under it. Configuration-based, never
	// privilege-based: root's hidepid exemption does not relax this.
	if value, restricted := identity.RestrictedProcfsAt(procfsMounts); restricted {
		fmt.Fprintf(os.Stderr, "supervise owner: refusing to arm: /proc is mounted hidepid=%s, which makes another user's live process indistinguishable from a dead one and breaks identity's three-way liveness guarantee\n", value)
		return 1
	}
	if *scope == "" {
		*scope = *repo
	}

	// Wait for the armer's start gate: it captures our pid, publishes
	// owner.json naming us, then touches the gate. Until then our lock
	// currency would read NoLock and the first establish would churn.
	if *gate != "" {
		deadline := time.Now().Add(30 * time.Second)
		for {
			if _, err := os.Stat(*gate); err == nil {
				_ = os.Remove(*gate)
				break
			}
			if time.Now().After(deadline) {
				fmt.Fprintln(os.Stderr, "supervise owner: start gate never appeared")
				return 1
			}
			time.Sleep(20 * time.Millisecond)
		}
	}

	self := identity.Ref{Pid: int64(os.Getpid())}
	exact, state, err := identity.KernelProber{}.Probe(self.Pid)
	if err != nil || state != identity.Alive {
		fmt.Fprintln(os.Stderr, "supervise owner: cannot read own identity")
		return 1
	}
	self.StartedAtSec = exact.StartedAt.Unix()

	supervisionDir := filepath.Join(*repo, "artifacts", "agents", "supervision")
	ownerTag := *tag

	checkout := &supervise.DiskCheckout{
		Root: *repo, Self: self, SelfTag: ownerTag,
		IntervalSec: *intervalSec,
		Fingerprint: *fingerprint,
		WatcherCap:  *watcherCap,
	}
	prober := identity.KernelProber{}
	lockSelf := lock.Identity{Pid: self.Pid, PidStartedAt: self.StartedAtSec, Tag: ownerTag}
	ledger := &supervise.RegistryLedger{
		CheckoutPath: *repo, OwnerTag: ownerTag,
		Append: func(record map[string]any) error {
			payload, err := supervise.EncodeRecord(record)
			if err != nil {
				return err
			}
			return registry.LockedAppend(*registryPath, lockSelf, payload,
				30*time.Second, 25*time.Millisecond, kernelProbe(prober))
		},
	}
	components := &supervise.ProcComponents{
		SupervisionDir: supervisionDir,
		Prober:         prober,
		IntervalSec:    *intervalSec,
		StopCeiling:    5 * time.Second,
		Command: func(component supervise.Component, componentTag, heartbeatPath string) []string {
			self, _ := os.Executable()
			argv := []string{self, "supervise", "component",
				"--component", string(component), "--tag", componentTag,
				"--heartbeat", heartbeatPath, "--interval", fmt.Sprint(*intervalSec),
				"--cap-min", fmt.Sprint(*watcherCap),
				// The component operates on this checkout: the watcher censuses
				// the scope and the reaper sweeps this checkout's job records.
				"--repo", *repo, "--scope", *scope}
			// Fixture-only crash-loop injection (D-2 breaker proof): the
			// component crashes on start and never beats, so the owner
			// sees Failing every observation.
			if os.Getenv("METASYSTEM_GO_COMPONENT_CRASH_ON_START") != "" {
				argv = append(argv, "--crash-on-start")
			}
			return argv
		},
	}
	intents := &supervise.DiskIntents{
		Root: *repo, Self: self, SelfTag: ownerTag,
		LatchWindow: 20 * time.Second,
	}

	logPath := filepath.Join(supervisionDir, "owner.ndjson")
	owner := &supervise.Owner{
		Checkout: checkout, Components: components, Ledger: ledger, Intents: intents,
		BaseInterval:  time.Duration(*intervalSec) * time.Second,
		Ceiling:       12,
		Breaker:       supervise.Breaker{GiveUpAt: 5, BaseInterval: time.Duration(*intervalSec) * time.Second, BackoffCap: 10 * time.Minute},
		Establishment: supervise.Establishment{Deadline: 5},
		TagPrefix:     ownerTag,
		Narrate:       narrator(logPath),
		// Generations stay monotone across owner restarts (the fingerprint
		// harness's staleness discipline): continue from the last published
		// state rather than restarting at one.
		SeedGeneration: checkout.PriorGeneration(),
	}
	// TERM/INT take the ExitOnSignal path (D-1): latch the shutdown intent,
	// tear the held set down BY IDENTITY, append the terminal. The check
	// lives in the injected Sleep so it runs between cycles, never
	// concurrently with one — an unhandled TERM would kill the owner with
	// the default action and leak its detached components forever.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	owner.Sleep = func(d time.Duration) {
		select {
		case <-time.After(d):
		case <-stop:
			exit := owner.ExitOnSignal()
			fmt.Printf("owner exit: reason=%s teardownComplete=%v\n", exit.Reason, exit.TeardownComplete)
			os.Exit(0)
		}
	}
	exit := owner.Run()
	fmt.Printf("owner exit: reason=%s teardownComplete=%v\n", exit.Reason, exit.TeardownComplete)
	return 0
}

// narrator appends one JSON line per cycle to the owner log — the
// flight-recorder-style spine for supervision decisions. Never-fail:
// a log write error must never disturb the loop.
func narrator(path string) func(supervise.CycleTrace) {
	return func(trace supervise.CycleTrace) {
		line, err := json.Marshal(trace)
		if err != nil {
			return
		}
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o644)
		if err != nil {
			return
		}
		defer file.Close()
		_, _ = file.Write(append(line, '\n'))
	}
}

func kernelProbe(prober identity.Prober) lock.Probe {
	return func(who lock.Identity) lock.Liveness {
		switch identity.AliveRef(prober, identity.Ref{Pid: who.Pid, StartedAtSec: who.PidStartedAt}) {
		case identity.Alive:
			return lock.Alive
		case identity.Dead:
			return lock.Dead
		default:
			return lock.Unknown
		}
	}
}

func defaultRegistryPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".metasystem/armed-checkouts.jsonl"
	}
	return filepath.Join(home, ".metasystem", "armed-checkouts.jsonl")
}
