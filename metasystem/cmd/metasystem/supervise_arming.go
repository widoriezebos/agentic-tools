package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// The arming-side supervision verbs: the reserved-cap ceiling check, the
// owner-identity publication, component-identity reads, and detached launch.

var armTerminalStatuses = map[string]bool{
	"completed": true, "failed": true, "timeout": true, "cancelled": true,
}

// runSuperviseBlockingReservedCap scans the job records and mission fence
// reservations for a non-terminal reservation whose capMin is at or above the
// proposed watcher ceiling, printing the highest blocker as job|cap. A ceiling
// that does not strictly clear every live reservation must not be armed.
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
	reserved := map[string]int64{}
	jobsDir := filepath.Join(*agents, "jobs")
	jobPaths, _ := filepath.Glob(filepath.Join(jobsDir, "*.json"))
	sort.Strings(jobPaths)
	for _, path := range jobPaths {
		record, err := readJSONObject(path)
		if err != nil {
			continue
		}
		cap, capOK := jsonIntField(record["capMin"])
		status, _ := record["status"].(string)
		job, _ := record["jobId"].(string)
		if job == "" {
			job = strings.TrimSuffix(filepath.Base(path), ".json")
		}
		if capOK && cap >= *ceiling && !armTerminalStatuses[status] {
			reserved[job] = cap
		}
	}
	fencePaths, _ := filepath.Glob(filepath.Join(*agents, "missions", "*", "fences.json"))
	sort.Strings(fencePaths)
	for _, path := range fencePaths {
		fences, err := readJSONObject(path)
		if err != nil {
			continue
		}
		reservations, _ := fences["reservations"].(map[string]any)
		for job, raw := range reservations {
			reservation, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			cap, capOK := jsonIntField(reservation["capMin"])
			if !capOK || cap < *ceiling {
				continue
			}
			status := ""
			if record, err := readJSONObject(filepath.Join(jobsDir, job+".json")); err == nil {
				status, _ = record["status"].(string)
			}
			if !armTerminalStatuses[status] {
				if cap > reserved[job] {
					reserved[job] = cap
				}
			}
		}
	}
	if len(reserved) == 0 {
		return 0
	}
	jobs := make([]string, 0, len(reserved))
	for job := range reserved {
		jobs = append(jobs, job)
	}
	sort.Slice(jobs, func(i, j int) bool {
		if reserved[jobs[i]] != reserved[jobs[j]] {
			return reserved[jobs[i]] > reserved[jobs[j]]
		}
		return jobs[i] < jobs[j]
	})
	fmt.Printf("%s|%d\n", jobs[0], reserved[jobs[0]])
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
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	temp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempName, path); err != nil {
		return err
	}
	if dir, err := os.Open(filepath.Dir(path)); err == nil {
		_ = dir.Sync()
		_ = dir.Close()
	}
	return nil
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
	doc, err := readJSONObject(*state)
	if err != nil {
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

// runSuperviseWatchdogReport reads the last census and the supervision state
// and reports what a session should know at turn end: a stale or unsuccessful
// census, untracked agent processes, a fingerprint older than the code, and
// any recorded identity that is no longer running — each with re-arm advice.
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
	const armCmd = "scripts/agents/arm-supervision.sh --repo ."
	supervision := filepath.Join(*repo, "artifacts", "agents", "supervision")
	var lines []string

	last, lastErr := readJSONObject(filepath.Join(supervision, "last-census.json"))
	if lastErr != nil {
		lines = append(lines, "WATCHDOG: it has not reported at all yet. Re-arm it with "+armCmd+" if this persists.")
	} else {
		completed, _ := jsonIntField(last["completedAtEpoch"])
		interval, _ := jsonIntField(last["intervalSec"])
		age := time.Now().Unix() - completed
		window := int64(0)
		if interval >= 1 {
			window = 2 * interval
			if window > 180 {
				window = 180
			}
		}
		verdict, _ := last["verdict"].(string)
		if verdict != "SUCCESS" || interval < 1 || age >= window {
			lines = append(lines, fmt.Sprintf("WATCHDOG: its last report is %ds old or unsuccessful, so it may not be watching. Re-arm it with %s.", age, armCmd))
		}
		if inventory, ok := last["inventory"].([]any); ok {
			for _, raw := range inventory {
				item, _ := raw.(map[string]any)
				if item == nil {
					continue
				}
				if class, _ := item["class"].(string); class == "UNTRACKED" {
					pid, _ := jsonIntField(item["pid"])
					runtime, _ := item["runtime"].(string)
					argv, _ := item["argv"].(string)
					lines = append(lines, fmt.Sprintf("UNTRACKED pid=%d runtime=%s argv=%s", pid, runtime, argv))
				}
			}
		}
	}

	state, stateErr := readJSONObject(filepath.Join(supervision, "state.json"))
	owner, ownerOK := stateOwner(state)
	components, componentsOK := stateComponents(state)
	if stateErr != nil || !ownerOK || !componentsOK {
		lines = append(lines, "WATCHDOG: its record of what it is watching is missing or unreadable. Re-arm it with "+armCmd+".")
		state = nil
	} else if lastErr == nil {
		stateFP, _ := state["fingerprint"].(string)
		lastFP, _ := last["fingerprint"].(string)
		if stateFP != lastFP {
			lines = append(lines, "WATCHDOG: it was started against an older version of this code and is now watching something that has changed. Re-arm it with "+armCmd+".")
		}
	}

	identities := map[string]map[string]any{}
	if owner != nil {
		identities["owner"] = owner
	}
	for name, raw := range components {
		if item, ok := raw.(map[string]any); ok {
			identities[name] = item
		}
	}
	names := make([]string, 0, len(identities))
	for name := range identities {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		item := identities[name]
		pid, pidOK := jsonIntField(item["pid"])
		start, startOK := jsonIntField(item["pidStartedAt"])
		if !pidOK || !startOK || !census.Alive(pid, start) {
			lines = append(lines, fmt.Sprintf("WATCHDOG: its %s part is not running. Re-arm it with %s.", name, armCmd))
		}
	}

	if len(lines) > 20 {
		lines = lines[:20]
	}
	fmt.Println(strings.Join(lines, "\n"))
	return 0
}

func stateOwner(state map[string]any) (map[string]any, bool) {
	if state == nil {
		return nil, false
	}
	owner, ok := state["owner"].(map[string]any)
	return owner, ok
}

func stateComponents(state map[string]any) (map[string]any, bool) {
	if state == nil {
		return nil, false
	}
	components, ok := state["components"].(map[string]any)
	return components, ok
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
