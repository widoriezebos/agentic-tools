package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/up"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/verbresult"
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
	return upMetasystemRootOf(binary)
}

// upMetasystemRootOf is the installation an engine executable belongs to:
// the directory above its bin directory, which carries metasystem.conf.
func upMetasystemRootOf(binary string) (string, error) {
	binary, err := filepath.EvalSymlinks(binary)
	if err != nil {
		return "", err
	}
	root := filepath.Dir(filepath.Dir(binary))
	if _, err := os.Stat(filepath.Join(root, "metasystem.conf")); err != nil {
		return "", fmt.Errorf("cannot derive the metasystem root from %s; pass --metasystem-root", binary)
	}
	return canonicalPath(root)
}

func upRepositoryScopeWith(supplied string, repositoryTop func(string) (string, error)) (string, error) {
	top, err := repositoryTop(supplied)
	if err != nil {
		return "", fmt.Errorf("--repo is not inside a git repository: %s", supplied)
	}
	return canonicalPath(top)
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

func printUpResult(result up.Result, stdout io.Writer) int {
	for _, line := range result.Lines() {
		fmt.Fprintln(stdout, line)
	}
	return result.ExitCode()
}

// upVerb is the envelope verb of up --json, the answer a parent reads.
const upVerb = "up"

// upEnvelope is up's result as the one envelope a parent reads: its outcome
// and the component it stopped at are typed data, never its words.
func upEnvelope(result up.Result) verbresult.Result {
	envelope := verbresult.FromError(upVerb, result.ExitCode(), nil, result.Data())
	if result.ExitCode() == 0 {
		envelope.Summary = "supervision is " + result.Outcome
		return envelope
	}
	step := result.Failed
	if step == "" {
		step = result.Outcome
	}
	envelope.Summary = "starting supervision stopped at " + step
	if result.Remedy != "" {
		envelope.Details = []string{result.Remedy}
	}
	return envelope
}

// answerUp prints up's result: its lines for a person, and under --json
// those lines on stderr and the envelope alone on stdout.
func answerUp(result up.Result, asJSON bool, stdout, stderr io.Writer) int {
	if !asJSON {
		return printUpResult(result, stdout)
	}
	printUpResult(result, stderr)
	_ = verbresult.Write(stdout, upEnvelope(result))
	return result.ExitCode()
}

func restampStopCapabilityForUp(root, lineage string, claimEpoch int64) (up.StopCapabilityRestampResult, error) {
	if !goal.NewWorld(root) {
		return up.StopCapabilityRestampResult{}, nil
	}
	endpoint, err := goal.ResolveEndpoint(root)
	if err != nil {
		return up.StopCapabilityRestampResult{}, err
	}
	now, err := goalCommandNow(root)
	if err != nil {
		return up.StopCapabilityRestampResult{}, err
	}
	projection, err := goal.Project(endpoint, false, now)
	if err != nil {
		return up.StopCapabilityRestampResult{}, err
	}
	machine, err := goal.ResolveMachine(root)
	if err != nil {
		return up.StopCapabilityRestampResult{}, err
	}
	for _, id := range goal.SortedGoalIds(projection.Tree.Live) {
		file := projection.Tree.Live[id]
		if file.State != goal.StateClaimed || file.Claimed == nil ||
			file.Claimed.Machine != machine || file.Claimed.Lineage != lineage {
			continue
		}
		result := up.StopCapabilityRestampResult{GoalID: id, ToEpoch: claimEpoch}
		if file.StopCapability == nil {
			return result, fmt.Errorf("claimed goal %s has no stop capability", id)
		}
		result.FromEpoch = file.StopCapability.ClaimEpoch
		if result.FromEpoch == claimEpoch {
			return result, nil
		}
		if result.FromEpoch > claimEpoch {
			return result, fmt.Errorf("the stop permission is newer (%d) than this checkout's claim (%d)", result.FromEpoch, claimEpoch)
		}
		classification := lease.ClassifyResult{Class: lease.ClassMain, Holder: true, ClaimEpoch: &claimEpoch}
		request, err := syncReqClassified(root, "", lineage, nil, classification)
		if err != nil {
			return result, err
		}
		published, err := goal.Restamp(request, id)
		if err != nil {
			return result, err
		}
		if published.Outcome != goal.OutcomeConfirmed {
			return result, fmt.Errorf("goal restamp ended %s: %s", published.Outcome, published.Detail)
		}
		result.Restamped = true
		return result, nil
	}
	return up.StopCapabilityRestampResult{}, nil
}

func runUpWith(args []string, repositoryTop func(string) (string, error), stdout, stderr io.Writer) int {
	flags := newFlagSet("up", stdout, stderr)
	repo := pathFlag(flags, "repo", ".", "repository or path inside it")
	metasystemRoot := flags.String("metasystem-root", "", "metasystem checkout root (internal compatibility option)")
	session := flags.String("session", "", "session id (defaults to METASYSTEM_SESSION_ID or session-<pid>)")
	runtimeSession := flags.String("runtime-session", "", "runtime session associated with this lifecycle event")
	noRuntimeSession := flags.Bool("no-runtime-session", false, "record that this lifecycle event supplied no runtime session")
	startSource := flags.String("start-source", "", "session start source")
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
	asJSON := flags.Bool("json", false, "print one result envelope on stdout for a calling process; up's lines go to stderr")
	if flags.Parse(args) != nil {
		return 2
	}
	// refuse answers a refusal before up ran: the words on stderr and, under
	// --json, the envelope a parent reads.
	refuse := func(status int, reason string) int {
		fmt.Fprintln(stderr, reason)
		if *asJSON {
			_ = verbresult.Write(stdout, verbresult.FromError(upVerb, status, errors.New(reason), nil))
		}
		return status
	}
	if flags.NArg() != 0 || *maxCap < 0 || (*runtimeSession != "" && *noRuntimeSession) {
		return refuse(2, "up: flags are invalid")
	}
	modeCount := 0
	for _, selected := range []bool{*printScheduler, *recoverOnly, *retire, *shutdown} {
		if selected {
			modeCount++
		}
	}
	if modeCount > 1 || (*ifDown && !*recoverOnly) {
		return refuse(2, "choose one of: print the schedule, recover, retire or shut down; --if-down needs --recover-only")
	}
	root, err := upMetasystemRoot(*metasystemRoot)
	if err != nil {
		return refuse(2, "up: "+err.Error())
	}
	scope, err := upRepositoryScopeWith(*repo, repositoryTop)
	if err != nil {
		return refuse(2, "up: "+err.Error())
	}
	binary, err := os.Executable()
	if err != nil {
		return refuse(1, "up: "+err.Error())
	}
	binary, err = canonicalPath(binary)
	if err != nil {
		return refuse(1, "up: "+err.Error())
	}
	scale := upWaitScale()
	if scale == 0 {
		return refuse(2, fmt.Sprintf("the test time scale %s is not a whole number above zero", "METASYSTEM_FIXTURE_CAP_SCALE_MILLI"))
	}
	options := up.Options{
		Root: scope, MetasystemRoot: root, Scope: scope, Binary: binary, Session: *session, Pid: *pid,
		StartTime: *start, Tag: *tag, Runtime: *runtimeName, OwnerLineage: *ownerLineage,
		RuntimeSession: *runtimeSession, NoRuntimeSession: *noRuntimeSession, StartSource: *startSource,
		MaxCap: *maxCap, RecoverOnly: *recoverOnly, IfDown: *ifDown, WaitScaleMilli: scale,
		CallerPid:             int64(os.Getppid()),
		RestampStopCapability: restampStopCapabilityForUp,
	}
	if *printScheduler {
		fmt.Fprintln(stdout, up.SchedulerEntry(options))
		return 0
	}
	// Supervision control, enrollment and accounting belong to the
	// authenticated installation. Scope remains the containing application
	// repository so census still observes application processes and sources.
	options.Root = root
	if *retire {
		return answerUp(up.Retire(options), *asJSON, stdout, stderr)
	}
	if *shutdown {
		return answerUp(up.Shutdown(options), *asJSON, stdout, stderr)
	}
	return answerUp(up.Run(options), *asJSON, stdout, stderr)
}
