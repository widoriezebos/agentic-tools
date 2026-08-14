package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise"
)

// The arming-side supervision verbs: the reserved-cap ceiling check, the
// owner-identity publication, component-identity reads, and detached launch.

// runSuperviseBlockingReservedCap relays `supervise blocking-reserved-cap`:
// the scan/rank decision lives in supervise.BlockingReservedCap (review
// cli-1), and this verb prints the highest blocker as job|cap or the
// refusal by name.
func runSuperviseBlockingReservedCap(args []string) int {
	flags := flag.NewFlagSet("supervise blocking-reserved-cap", flag.ContinueOnError)
	agents := flags.String("agents", "", "artifacts/agents directory")
	ceiling := flags.Int64("ceiling", 0, "proposed watcher ceiling in minutes")
	if flags.Parse(args) != nil {
		return 2
	}
	if *agents == "" || *ceiling < 1 {
		fmt.Fprintln(os.Stderr, "supervise blocking-reserved-cap: --agents and --ceiling are required")
		return 2
	}
	blocker, blocked, err := supervise.BlockingReservedCap(*agents, *ceiling)
	if err != nil {
		fmt.Fprintf(os.Stderr, "supervise blocking-reserved-cap: refusing to arm: %v\n", err)
		return 1
	}
	if blocked {
		fmt.Printf("%s|%d\n", blocker.Job, blocker.Cap)
	}
	return 0
}

// runSuperviseWriteOwnerIdentity atomically writes the owner-identity record
// {pid, pidStartedAt, instanceTag, acquiredAt}.
func runSuperviseWriteOwnerIdentity(args []string) int {
	flags := flag.NewFlagSet("supervise write-owner-identity", flag.ContinueOnError)
	path := flags.String("path", "", "identity file path")
	pid := flags.Int64("pid", 0, "owner pid")
	start := flags.Int64("start", 0, "owner start epoch seconds")
	tag := flags.String("tag", "", "owner instance tag")
	acquiredAt := flags.String("acquired-at", "", "acquisition timestamp")
	if flags.Parse(args) != nil {
		return 2
	}
	if *path == "" || *pid == 0 || *tag == "" {
		fmt.Fprintln(os.Stderr, "supervise write-owner-identity: --path, --pid, and --tag are required")
		return 2
	}
	value := map[string]any{
		"pid": *pid, "pidStartedAt": *start, "instanceTag": *tag, "acquiredAt": *acquiredAt,
	}
	if err := writeIdentityJSON(*path, value); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// writeIdentityJSON writes indented, key-sorted JSON atomically: temp in the
// target directory, fsync, rename, directory fsync.
func writeIdentityJSON(path string, value any) error {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	// Through the durable-write owner (go-production-grade B5); the
	// empty anchor preserves this writer's previous behavior exactly
	// until its caller is converted to the two-outcome contract.
	_, writeErr := atomicfile.WriteText(path, string(encoded), "")
	return writeErr
}

func readJSONObject(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	return doc, nil
}

func jsonIntField(v any) (int64, bool) {
	f, ok := v.(float64)
	if !ok || f != float64(int64(f)) {
		return 0, false
	}
	return int64(f), true
}

// runSuperviseComponentIdentity prints "pid start tag" for one recorded
// component, failing as a unit when the component or any field is absent.
func runSuperviseComponentIdentity(args []string) int {
	flags := flag.NewFlagSet("supervise component-identity", flag.ContinueOnError)
	state := flags.String("state", "", "supervision state path")
	component := flags.String("component", "", "component name")
	if flags.Parse(args) != nil {
		return 2
	}
	if *state == "" || *component == "" {
		fmt.Fprintln(os.Stderr, "supervise component-identity: --state and --component are required")
		return 2
	}
	doc, err := readJSONObject(*state)
	if err != nil {
		// Named on stderr (cli-8): this runs in detached supervision where
		// stderr is the only diagnostic channel. An absent component or
		// field below stays silent by design — the callers probe with
		// || true and an absent entry is an ordinary outcome, not a fault.
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	components, _ := doc["components"].(map[string]any)
	entry, _ := components[*component].(map[string]any)
	if entry == nil {
		return 1
	}
	pid, pidOK := jsonIntField(entry["pid"])
	start, startOK := jsonIntField(entry["pidStartedAt"])
	tag, tagOK := entry["instanceTag"].(string)
	if !pidOK || !startOK || !tagOK {
		return 1
	}
	fmt.Printf("%d %d %s\n", pid, start, tag)
	return 0
}

// runSuperviseLaunchDetached starts a command fully detached — stdin from
// /dev/null, stdout/stderr appended to the log, its own session — and prints
// the child pid.
func runSuperviseLaunchDetached(args []string) int {
	flags := flag.NewFlagSet("supervise launch-detached", flag.ContinueOnError)
	log := flags.String("log", "", "log file the child's output appends to (default /dev/null)")
	cwd := flags.String("cwd", "", "working directory for the child (optional)")
	var env []string
	flags.Func("env", "KEY=VALUE to add to the child's environment (repeatable)", func(value string) error {
		env = append(env, value)
		return nil
	})
	if flags.Parse(args) != nil {
		return 2
	}
	argv := flags.Args()
	if len(argv) == 0 {
		fmt.Fprintln(os.Stderr, "supervise launch-detached: a command is required")
		return 2
	}
	logPath := *log
	if logPath == "" {
		logPath = os.DevNull
	}
	logFile, err := os.OpenFile(logPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer logFile.Close()
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer devNull.Close()
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdin = devNull
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Dir = *cwd
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println(cmd.Process.Pid)
	// The child is its own session; it is not waited on here.
	_ = cmd.Process.Release()
	return 0
}

// runSuperviseWatchdogReport relays `supervise watchdog-report`: the health
// judgment lives in supervise.WatchdogReport (review cli-2), and this verb
// prints its lines — nothing when everything is healthy.
func runSuperviseWatchdogReport(args []string) int {
	flags := flag.NewFlagSet("supervise watchdog-report", flag.ContinueOnError)
	repo := flags.String("repo", "", "checkout root")
	if flags.Parse(args) != nil {
		return 2
	}
	if *repo == "" {
		fmt.Fprintln(os.Stderr, "supervise watchdog-report: --repo is required")
		return 2
	}
	if lines := supervise.WatchdogReport(*repo, time.Now()); len(lines) > 0 {
		fmt.Println(strings.Join(lines, "\n"))
	}
	return 0
}

// runSuperviseHeartbeat writes a component heartbeat: the process identity
// (pid + kernel start second), its function and tag, and the observation time,
// atomically.
func runSuperviseHeartbeat(args []string) int {
	flags := flag.NewFlagSet("supervise heartbeat", flag.ContinueOnError)
	path := flags.String("path", "", "heartbeat file path")
	function := flags.String("function", "", "component function name")
	pid := flags.Int64("pid", 0, "component pid")
	tag := flags.String("tag", "", "instance tag")
	if flags.Parse(args) != nil {
		return 2
	}
	if *path == "" || *function == "" || *pid < 1 || *tag == "" {
		fmt.Fprintln(os.Stderr, "supervise heartbeat: --path, --function, --pid, and --tag are required")
		return 2
	}
	exact, state, err := identity.KernelProber{}.Probe(*pid)
	if err != nil || state != identity.Alive {
		fmt.Fprintln(os.Stderr, "supervise heartbeat: pid identity unreadable")
		return 1
	}
	value := map[string]any{
		"function": *function, "pid": *pid, "pidStartedAt": exact.StartedAt.Unix(),
		"instanceTag": *tag, "observedAtEpoch": time.Now().Unix(),
	}
	if err := writeIdentityJSON(*path, value); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
