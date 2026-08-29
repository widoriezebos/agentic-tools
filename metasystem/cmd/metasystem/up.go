package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/up"
)

func canonicalPath(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if resolved, resolveErr := filepath.EvalSymlinks(absolute); resolveErr == nil {
		return resolved, nil
	}
	return filepath.Clean(absolute), nil
}

func upMetasystemRoot(explicit string) (string, error) {
	if explicit != "" {
		return canonicalPath(explicit)
	}
	binary, err := os.Executable()
	if err != nil {
		return "", err
	}
	root := filepath.Dir(filepath.Dir(binary))
	if _, err := os.Stat(filepath.Join(root, "metasystem.conf")); err != nil {
		return "", fmt.Errorf("cannot derive the metasystem root from %s; pass --metasystem-root", binary)
	}
	return canonicalPath(root)
}

func upRepositoryScope(supplied string) (string, error) {
	command := exec.Command("git", "-C", supplied, "rev-parse", "--show-toplevel")
	out, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("--repo is not inside a git repository: %s", supplied)
	}
	return canonicalPath(strings.TrimSpace(string(out)))
}

func upWaitScale() int {
	value := os.Getenv("METASYSTEM_FIXTURE_CAP_SCALE_MILLI")
	if value == "" {
		return 1000
	}
	scale, err := strconv.Atoi(value)
	if err != nil || scale < 1 {
		return 0
	}
	return scale
}

func printUpResult(result up.Result) int {
	for _, line := range result.Lines() {
		fmt.Println(line)
	}
	return result.ExitCode()
}

func runUp(args []string) int {
	flags := flag.NewFlagSet("up", flag.ContinueOnError)
	repo := flags.String("repo", ".", "repository or path inside it")
	metasystemRoot := flags.String("metasystem-root", "", "metasystem checkout root (internal compatibility option)")
	session := flags.String("session", "", "session id (defaults to METASYSTEM_SESSION_ID or session-<pid>)")
	pid := flags.Int64("pid", 0, "explicit session pid fallback; requires --start-time")
	start := flags.Int64("start-time", 0, "explicit session start epoch fallback; requires --pid")
	tag := flags.String("tag", "", "session instance tag")
	runtimeName := flags.String("runtime", "", "runtime whose signature proves the session ancestor")
	ownerLineage := flags.String("owner-lineage", "", "logical owner lineage")
	maxCap := flags.Int64("max-cap", 0, "declared maximum delegate cap in minutes")
	printScheduler := flags.Bool("print-scheduler-entry", false, "print, but never install, an optional recovery-only cron entry")
	recoverOnly := flags.Bool("recover-only", false, "restricted scheduler recovery: no announcement or lease")
	ifDown := flags.Bool("if-down", false, "with --recover-only, start only missing repository rings")
	retire := flags.Bool("retire", false, "retire this session announcement (internal compatibility option)")
	shutdown := flags.Bool("shutdown", false, "stop supervision (internal fixture compatibility option)")
	_ = flags.Bool("rearm", false, "deprecated compatibility spelling; ordinary up replaces an older generation automatically")
	if flags.Parse(args) != nil {
		return 2
	}
	if flags.NArg() != 0 || *maxCap < 0 {
		fmt.Fprintln(os.Stderr, "up: flags are invalid")
		return 2
	}
	modeCount := 0
	for _, selected := range []bool{*printScheduler, *recoverOnly, *retire, *shutdown} {
		if selected {
			modeCount++
		}
	}
	if modeCount > 1 || (*ifDown && !*recoverOnly) {
		fmt.Fprintln(os.Stderr, "up: scheduler printing, recovery, retirement, and shutdown modes cannot be combined; --if-down requires --recover-only")
		return 2
	}
	root, err := upMetasystemRoot(*metasystemRoot)
	if err != nil {
		fmt.Fprintln(os.Stderr, "up:", err)
		return 2
	}
	scope, err := upRepositoryScope(*repo)
	if err != nil {
		fmt.Fprintln(os.Stderr, "up:", err)
		return 2
	}
	binary, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "up:", err)
		return 1
	}
	binary, err = canonicalPath(binary)
	if err != nil {
		fmt.Fprintln(os.Stderr, "up:", err)
		return 1
	}
	scale := upWaitScale()
	if scale == 0 {
		fmt.Fprintln(os.Stderr, "up: METASYSTEM_FIXTURE_CAP_SCALE_MILLI must be a positive integer")
		return 2
	}
	options := up.Options{
		Root: scope, MetasystemRoot: root, Scope: scope, Binary: binary, Session: *session, Pid: *pid,
		StartTime: *start, Tag: *tag, Runtime: *runtimeName, OwnerLineage: *ownerLineage,
		MaxCap: *maxCap, RecoverOnly: *recoverOnly, IfDown: *ifDown, WaitScaleMilli: scale,
		CallerPid: int64(os.Getppid()),
	}
	if *printScheduler {
		fmt.Println(up.SchedulerEntry(options))
		return 0
	}
	stateRoot, err := stateroot.RootForInstallation(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "up:", err)
		return 1
	}
	options.Root = stateRoot
	if *retire {
		return printUpResult(up.Retire(options))
	}
	if *shutdown {
		return printUpResult(up.Shutdown(options))
	}
	return printUpResult(up.Run(options))
}
