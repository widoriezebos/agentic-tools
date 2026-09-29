package main

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/events"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/run"
)

// The run family (the monitor facility): tracked long-running work.
// Mutations classify the caller and run the holder-only matrix exactly
// like the goal family; conclude is record-writer so supervision's
// watcher may conclude; watch and the reads are open.

// runStore binds a Store with the in-lock epoch reader: the CLI
// ALWAYS wires CurrentEpoch so a stale-epoch child cannot mutate
// records after a takeover; only library/test use leaves the seam nil.
func runStore(root string) *run.Store {
	return dispatchcore.NewConcludingRunStore(root, func() (*int64, bool) {
		view, err := classifyVerbCaller(root, int64(os.Getpid()))
		if err != nil {
			return nil, false
		}
		return view.ClaimEpoch, true
	})
}

func runRunWrap(args []string) int {
	flags := newFlagSet("run wrap")
	root := pathFlag(flags, "root", ".", "checkout root")
	id := flags.String("id", "", "run id")
	nonce := flags.String("nonce", "", "launch nonce (in argv by design: the third identity factor)")
	logPath := flags.String("log", "", "log path")
	if flags.Parse(args) != nil {
		return 2
	}
	command := flags.Args()
	if len(command) == 0 {
		fmt.Fprintln(os.Stderr, "run wrap requires -- <command...>")
		return 2
	}
	store := runStore(*root)
	self := int64(os.Getpid())
	// A setsid leader's pgid is its own pid.
	if err := store.Bind(*id, *nonce, self, self); err != nil {
		fmt.Fprintln(os.Stderr, "bind failed:", err)
		return 1
	}
	logFile, err := os.OpenFile(*logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		fmt.Fprintln(os.Stderr, "log unwritable:", err)
		_ = store.WriteSidecar(*id, 1, *nonce, 127)
		return 1
	}
	defer logFile.Close()
	workload := exec.Command(command[0], command[1:]...)
	workload.Env = os.Environ()
	bound, boundErr := store.Read(*id)
	if boundErr != nil || bound == nil {
		fmt.Fprintln(os.Stderr, "bound run record unreadable before workload launch")
		_ = store.WriteSidecar(*id, 1, *nonce, 127)
		return 1
	}
	if bound.Governed != nil {
		workload.Env = append(workload.Env,
			"METASYSTEM_PROOF_RUN_ROOT="+*root,
			"METASYSTEM_PROOF_RUN_ID="+*id,
		)
	}
	workload.Stdout = logFile
	workload.Stderr = logFile
	exitCode := int64(0)
	if err := workload.Run(); err != nil {
		exitCode = 1
		if exit, ok := err.(*exec.ExitError); ok {
			exitCode = int64(exit.ExitCode())
		}
	}
	record, readErr := store.Read(*id)
	generation := 1
	if readErr == nil && record != nil {
		generation = record.Generation
	}
	// The sidecar is the wrapper's LAST act.
	if err := store.WriteSidecar(*id, generation, *nonce, exitCode); err != nil {
		fmt.Fprintln(os.Stderr, "sidecar write failed:", err)
		return 1
	}
	return int(exitCode)
}

func runRunWatch(args []string) int {
	flags := newFlagSet("run watch")
	root := pathFlag(flags, "root", ".", "checkout root")
	id := flags.String("id", "", "run id")
	pollMs := flags.Int("poll-ms", 2000, "poll interval")
	callerPid := flags.Int64("caller-pid", 0, "caller pid")
	if flags.Parse(args) != nil {
		return 2
	}
	_ = pollMs // retained for command-line compatibility; wait owns its cadence.
	if flags.NArg() != 0 {
		return 2
	}
	if *id == "" {
		fmt.Fprintln(os.Stdout, "run  no-record rc=4 log=")
		return run.ExitNoRecord
	}
	pid := *callerPid
	if pid == 0 {
		pid = int64(os.Getppid())
	}
	printer := func(result run.WaitResult, _ bool) {
		if result.ExitCode < run.ExitGreen || result.ExitCode > run.ExitNoRecord {
			return
		}
		outcome := result.SourceOutcome
		switch outcome {
		case run.StatusGreen, run.StatusRed, run.StatusEndedUnknown, run.StatusLaunchFailed:
		case "target-replaced":
		default:
			if result.TargetIncarnation == (run.WaiterTarget{}) {
				outcome = "no-record"
			} else {
				outcome = "record-vanished"
			}
		}
		logPath := ""
		if record, readErr := runStore(*root).Read(*id); readErr == nil && record != nil {
			logPath = record.Log
		}
		fmt.Fprintf(os.Stdout, "run %s %s rc=%d log=%s\n", *id, outcome, result.ExitCode, logPath)
	}
	return compatibilityWaitCommand([]string{"--root", *root, "--run", *id}, nil, pid, printer)
}

func runJobWatchVerb(args []string) int {
	flags := newFlagSet("job watch")
	root := pathFlag(flags, "root", ".", "checkout root")
	job := flags.String("job", "", "job id")
	pollMs := flags.Int("poll-ms", 2000, "poll interval")
	callerPid := flags.Int64("caller-pid", 0, "caller pid")
	progressRoot := flags.String("progress-root", "", "root whose live suite heartbeat prefixes progress notes")
	if flags.Parse(args) != nil {
		return 2
	}
	if flags.NArg() != 0 {
		return 2
	}
	if *job == "" {
		return run.ExitNoRecord
	}
	poll := time.Duration(*pollMs) * time.Millisecond
	if poll <= 0 {
		poll = 2 * time.Second
	}
	stopProgress := startSuiteProgressPrinter(*progressRoot, poll, os.Stderr)
	defer stopProgress()
	pid := *callerPid
	if pid == 0 {
		pid = int64(os.Getppid())
	}
	return compatibilityWaitCommand([]string{"--root", *root, "--job", *job}, nil, pid, func(run.WaitResult, bool) {})
}

var compatibilityWaitCommand = runWaitCommand

// The run package's flight-recorder wiring: component "run", this
// process's identity, one emitter for verbs and watcher alike.
func init() {
	emitter := &events.Emitter{Component: "run", Pid: int64(os.Getpid())}
	if exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid())); err == nil && state == identity.Alive {
		emitter.PidStartedAt = exact.StartedAt.Unix()
	}
	run.SetEmitter(func(root, event string, fields map[string]string) {
		summary := event
		if id, ok := fields["runId"]; ok {
			summary = event + " " + id
		}
		emitter.Emit(root, event, summary, fields)
	})
}
