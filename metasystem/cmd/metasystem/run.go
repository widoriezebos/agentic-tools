package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/authority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/run"
)

// The run family (monitor facility, D72): tracked long-running work.
// Mutations classify the caller and run the holder-only matrix exactly
// like the goal family; conclude is record-writer so supervision's
// watcher may conclude; watch and the reads are open.

// runCaller distills the classified invoker.
func runCaller(root string, callerPid int64, mode string) (run.Caller, error) {
	if callerPid == 0 {
		callerPid = int64(os.Getppid())
	}
	view, err := lease.ClassifyVerb(root, callerPid)
	if err != nil {
		return run.Caller{}, fmt.Errorf("caller classification failed: %v", err)
	}
	if mode != "" {
		classification := map[string]any{"class": view.Class, "holder": view.Holder}
		if err := authority.Authorize(mode, classification, ""); err != nil {
			return run.Caller{}, err
		}
	}
	lineage := view.MainId
	if view.Announcement != nil && view.Announcement.MainId != "" {
		lineage = view.Announcement.MainId
	}
	return run.Caller{
		Class: view.Class, MainId: view.MainId,
		OwnerLineage: lineage, ClaimEpoch: view.ClaimEpoch,
	}, nil
}

// watchLine is THE printed waiter command — one grammar, everywhere.
func watchLine(root, id string) string {
	return fmt.Sprintf("bin/metasystem run watch --id %s --root %s", id, root)
}

func runRunLaunch(args []string) int {
	flags := flag.NewFlagSet("run launch", flag.ContinueOnError)
	root := flags.String("root", ".", "checkout root")
	id := flags.String("id", "", "run id")
	kind := flags.String("kind", "custom", "suite|cohort|custom")
	display := flags.String("display", "", "one display line (never derived from argv)")
	log := flags.String("log", "", "log path")
	stale := flags.Int("stale-after-min", 0, "hung threshold minutes")
	windDown := flags.Int("wind-down-min", 0, "drain window minutes")
	expectGreen := flags.String("expect-green", "", "continuation on green")
	expectRed := flags.String("expect-red", "", "continuation on red")
	expectHung := flags.String("expect-hung", "", "continuation on hang")
	expectUnknown := flags.String("expect-unknown", "", "continuation on unknown")
	callerPid := flags.Int64("caller-pid", 0, "caller pid")
	if flags.Parse(args) != nil {
		return 2
	}
	command := flags.Args()
	if len(command) == 0 {
		fmt.Fprintln(os.Stderr, "run launch requires -- <command...>")
		return 2
	}
	caller, err := runCaller(*root, *callerPid, "holder-only")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	store := &run.Store{Root: *root}
	nonce, err := store.Launch(caller, run.LaunchParams{
		Id: *id, Kind: *kind, Display: *display, Log: *log,
		StaleAfterMin: *stale, WindDownMin: *windDown,
		Expect: run.Expect{Green: *expectGreen, Red: *expectRed, Hung: *expectHung, Unknown: *expectUnknown},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	record, err := store.Read(*id)
	if err != nil || record == nil {
		fmt.Fprintln(os.Stderr, "pending record unreadable after launch")
		return 1
	}
	self, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	wrapArgs := append([]string{"run", "wrap",
		"--root", *root, "--id", *id, "--nonce", nonce, "--log", record.Log, "--"}, command...)
	wrapper := exec.Command(self, wrapArgs...)
	wrapper.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	wrapper.Stdout = nil
	wrapper.Stderr = nil
	wrapper.Stdin = nil
	if err := wrapper.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "wrapper spawn failed:", err)
		return 1
	}
	// The wrapper is detached on purpose; the record, not the process
	// tree, is the contract from here.
	_ = wrapper.Process.Release()
	fmt.Printf("run %s launched; watch it with:\n  %s\n", *id, watchLine(*root, *id))
	return 0
}

func runRunWrap(args []string) int {
	flags := flag.NewFlagSet("run wrap", flag.ContinueOnError)
	root := flags.String("root", ".", "checkout root")
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
	store := &run.Store{Root: *root}
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
	flags := flag.NewFlagSet("run watch", flag.ContinueOnError)
	root := flags.String("root", ".", "checkout root")
	id := flags.String("id", "", "run id")
	pollMs := flags.Int("poll-ms", 2000, "poll interval")
	callerPid := flags.Int64("caller-pid", 0, "caller pid")
	if flags.Parse(args) != nil {
		return 2
	}
	caller, err := runCaller(*root, *callerPid, "")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return run.ExitWaiterUnknown
	}
	store := &run.Store{Root: *root}
	return store.Watch(*id, caller, time.Duration(*pollMs)*time.Millisecond)
}

func runRunRegister(args []string) int {
	flags := flag.NewFlagSet("run register", flag.ContinueOnError)
	root := flags.String("root", ".", "checkout root")
	id := flags.String("id", "", "run id")
	kind := flags.String("kind", "custom", "suite|cohort|custom")
	display := flags.String("display", "", "one display line")
	log := flags.String("log", "", "log path")
	pid := flags.Int64("pid", 0, "the already-running leader pid")
	pattern := flags.String("verdict-pattern", "", "RE2 over the log tail (adopted records only)")
	stale := flags.Int("stale-after-min", 0, "hung threshold minutes")
	callerPid := flags.Int64("caller-pid", 0, "caller pid")
	if flags.Parse(args) != nil {
		return 2
	}
	caller, err := runCaller(*root, *callerPid, "holder-only")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	store := &run.Store{Root: *root}
	if err := store.Register(caller, run.LaunchParams{
		Id: *id, Kind: *kind, Display: *display, Log: *log, StaleAfterMin: *stale,
	}, *pid, *pattern); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	record, _ := store.Read(*id)
	fmt.Printf("run %s registered (%s); watch it with:\n  %s\n", *id, record.Custody, watchLine(*root, *id))
	return 0
}

func runRunAdopt(args []string) int {
	flags := flag.NewFlagSet("run adopt", flag.ContinueOnError)
	root := flags.String("root", ".", "checkout root")
	id := flags.String("id", "", "run id")
	pid := flags.Int64("pid", 0, "the successor leader pid")
	callerPid := flags.Int64("caller-pid", 0, "caller pid")
	if flags.Parse(args) != nil {
		return 2
	}
	caller, err := runCaller(*root, *callerPid, "holder-only")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	store := &run.Store{Root: *root}
	if err := store.Adopt(caller, *id, *pid); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("run %s adopted; watch it with:\n  %s\n", *id, watchLine(*root, *id))
	return 0
}

func runRunAck(args []string) int {
	flags := flag.NewFlagSet("run ack", flag.ContinueOnError)
	root := flags.String("root", ".", "checkout root")
	id := flags.String("id", "", "run id")
	callerPid := flags.Int64("caller-pid", 0, "caller pid")
	if flags.Parse(args) != nil {
		return 2
	}
	caller, err := runCaller(*root, *callerPid, "holder-only")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	store := &run.Store{Root: *root}
	if err := store.Ack(caller, *id); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("run %s acknowledged\n", *id)
	return 0
}

func runRunConclude(args []string) int {
	flags := flag.NewFlagSet("run conclude", flag.ContinueOnError)
	root := flags.String("root", ".", "checkout root")
	id := flags.String("id", "", "run id")
	callerPid := flags.Int64("caller-pid", 0, "caller pid")
	if flags.Parse(args) != nil {
		return 2
	}
	if _, err := runCaller(*root, *callerPid, "record-writer"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	store := &run.Store{Root: *root}
	result, err := store.Assess(*id)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if result.Transitioned {
		fmt.Printf("run %s: %s -> %s\n", *id, result.From, result.To)
	} else {
		fmt.Printf("run %s: no transition\n", *id)
	}
	for _, line := range result.Unreadable {
		fmt.Println("unreadable: " + line)
	}
	return 0
}

func runRunPrune(args []string) int {
	flags := flag.NewFlagSet("run prune", flag.ContinueOnError)
	root := flags.String("root", ".", "checkout root")
	callerPid := flags.Int64("caller-pid", 0, "caller pid")
	if flags.Parse(args) != nil {
		return 2
	}
	caller, err := runCaller(*root, *callerPid, "holder-only")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	store := &run.Store{Root: *root}
	dropped, err := store.Prune(caller)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("pruned %d run record(s)\n", len(dropped))
	for _, line := range dropped {
		fmt.Println("dropped: " + line)
	}
	return 0
}

func runRunList(args []string) int {
	flags := flag.NewFlagSet("run list", flag.ContinueOnError)
	root := flags.String("root", ".", "checkout root")
	if flags.Parse(args) != nil {
		return 2
	}
	store := &run.Store{Root: *root}
	records, unreadable := store.List()
	printJSON(map[string]any{"schemaVersion": 1, "runs": records, "unreadable": unreadable})
	return 0
}

func runRunStatus(args []string) int {
	flags := flag.NewFlagSet("run status", flag.ContinueOnError)
	root := flags.String("root", ".", "checkout root")
	id := flags.String("id", "", "run id")
	if flags.Parse(args) != nil {
		return 2
	}
	store := &run.Store{Root: *root}
	record, err := store.Read(*id)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if record == nil {
		fmt.Fprintf(os.Stderr, "no run record %s\n", *id)
		return 4
	}
	printJSON(record)
	return 0
}
