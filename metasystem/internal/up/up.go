// Package up owns the session-start arming transaction. It coordinates the
// existing lease, supervision, and steward owners and renders their decisions
// as one typed operator outcome.
package up

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	processidentity "github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise"
)

// Options are the complete inputs to ordinary and recovery-only arming.
type Options struct {
	Root           string
	MetasystemRoot string
	Scope          string
	Binary         string
	Session        string
	Pid            int64
	StartTime      int64
	Tag            string
	Runtime        string
	OwnerLineage   string
	MaxCap         int64
	RecoverOnly    bool
	IfDown         bool
	WaitScaleMilli int
	CallerPid      int64
}

// ComponentOutcome is one typed and actionable component result.
type ComponentOutcome struct {
	Component string
	Outcome   string
	Detail    string
	Remedy    string
}

// Result carries every component line and the one aggregate outcome.
type Result struct {
	Components        []ComponentOutcome
	Outcome           string
	Authority         string
	ReArmed           string
	DurabilityPending bool
	Holder            string
	Worktree          string
	Failed            string
	Remedy            string
}

// ExitCode is zero for armed, advisor, and successful recovery outcomes.
func (r Result) ExitCode() int {
	if r.Outcome == "failed" || r.Outcome == "recovery-partial" || r.Outcome == "ENROLLMENT_DRIFT" {
		return 1
	}
	return 0
}

func quoteField(value string) string {
	return strconv.Quote(value)
}

// Lines renders stable key/value records for operators and fixtures.
func (r Result) Lines() []string {
	lines := make([]string, 0, len(r.Components)+1)
	for _, component := range r.Components {
		line := fmt.Sprintf("component=%s outcome=%s", component.Component, component.Outcome)
		if component.Detail != "" {
			line += " detail=" + quoteField(component.Detail)
		}
		if component.Remedy != "" {
			line += " remedy=" + quoteField(component.Remedy)
		}
		lines = append(lines, line)
	}
	aggregate := "up outcome=" + r.Outcome
	if r.Authority != "" {
		aggregate += " authority=" + r.Authority
	}
	if r.ReArmed != "" {
		aggregate += " re-armed=" + quoteField(r.ReArmed)
	}
	if r.Holder != "" {
		aggregate += " holder=" + quoteField(r.Holder)
	}
	if r.Worktree != "" {
		aggregate += " worktree=" + quoteField(r.Worktree)
	}
	if r.Failed != "" {
		aggregate += " component=" + r.Failed
	}
	if r.Remedy != "" {
		aggregate += " remedy=" + quoteField(r.Remedy)
	}
	lines = append(lines, aggregate)
	return lines
}

func failure(components []ComponentOutcome, component string, err error, remedy string) Result {
	components = append(components, ComponentOutcome{
		Component: component, Outcome: "failed", Detail: err.Error(), Remedy: remedy,
	})
	return Result{Components: components, Outcome: "failed", Failed: component, Remedy: remedy}
}

type sessionIdentity struct {
	Session      string
	Pid          int64
	StartTime    int64
	StartTicks   int64
	BootID       string
	Tag          string
	Runtime      string
	OwnerLineage string
	Provenance   lease.IdentityProvenance
}

var sessionParentPid = processidentity.ParentPid

var stewardEnsureRunner = steward.EnsureRunner
var enrolledCommand = func(enrolled *steward.EnrolledBinary) func(args ...string) (*exec.Cmd, error) {
	return enrolled.Command
}

func installationRoot(options Options) string {
	if options.MetasystemRoot != "" {
		return options.MetasystemRoot
	}
	return options.Root
}

func sameAuthenticatedProcess(left, right census.ProcIdentity) bool {
	if left.Pid != right.Pid || left.PidStartedAt != right.PidStartedAt {
		return false
	}
	if left.PidStartTicks > 0 && left.BootID != "" && right.PidStartTicks > 0 && right.BootID != "" {
		return left.PidStartTicks == right.PidStartTicks && left.BootID == right.BootID
	}
	return true
}

func proveCallerDescendsFromTarget(callerPid, targetPid int64) error {
	current := callerPid
	seen := map[int64]bool{}
	for current > 1 && !seen[current] {
		if current == targetPid {
			return nil
		}
		seen[current] = true
		parent, ok := sessionParentPid(current)
		if !ok || parent == current {
			break
		}
		current = parent
	}
	return fmt.Errorf("explicit session pid is not the caller or one of its ancestors")
}

func resolveSessionIdentity(options Options) (sessionIdentity, error) {
	explicitPid := options.Pid != 0
	explicitStart := options.StartTime != 0
	if explicitPid != explicitStart {
		return sessionIdentity{}, fmt.Errorf("--pid and --start-time are a recorded identity pair and must be passed together")
	}
	pid, started, runtimeName := options.Pid, options.StartTime, options.Runtime
	startTicks, bootID := int64(0), ""
	if runtimeName == "" {
		runtimeName = os.Getenv("METASYSTEM_AGENT_RUNTIME")
	}
	if !explicitPid {
		// This is the named L7 seam for the runtime-signature registry that
		// L8 will own. Up consumes the census proof without defining a second
		// registry or signature grammar.
		ancestor, err := census.FindAncestorProduction(installationRoot(options), int64(os.Getppid()), runtimeName)
		if err != nil {
			return sessionIdentity{}, fmt.Errorf("runtime-signature ancestry proof failed: %w", err)
		}
		pid, started, runtimeName = ancestor.Pid, ancestor.PidStartedAt, ancestor.Runtime
		startTicks, bootID = ancestor.PidStartTicks, ancestor.BootID
	}
	if pid < 1 || started < 1 {
		return sessionIdentity{}, fmt.Errorf("session identity must carry a positive pid and start time")
	}
	if explicitPid {
		authorization, err := fixtureauth.New(installationRoot(options))
		if err != nil {
			return sessionIdentity{}, err
		}
		authIdentity, err := census.AuthIdentity(pid, authorization.Identity())
		if err != nil {
			return sessionIdentity{}, fmt.Errorf("session pid identity is not live and readable: %w", err)
		}
		// An explicit --pid/--start-time pair is the recorded fallback, so
		// its second must match. The exact pair is then carried from this same
		// authenticated process observation into announcement and lease work.
		if authIdentity.PidStartedAt != started {
			return sessionIdentity{}, fmt.Errorf("session pid start time does not match the live process")
		}
		callerPid := options.CallerPid
		if callerPid == 0 {
			callerPid = int64(os.Getpid())
		}
		callerIdentity, err := census.AuthIdentity(callerPid, authorization.Identity())
		if err != nil {
			return sessionIdentity{}, fmt.Errorf("calling process identity is not live and readable: %w", err)
		}
		if err := proveCallerDescendsFromTarget(callerPid, pid); err != nil {
			return sessionIdentity{}, err
		}
		recheckedCaller, err := census.AuthIdentity(callerPid, authorization.Identity())
		if err != nil || !sameAuthenticatedProcess(callerIdentity, recheckedCaller) {
			return sessionIdentity{}, fmt.Errorf("calling process identity changed during ancestry proof")
		}
		recheckedTarget, err := census.AuthIdentity(pid, authorization.Identity())
		if err != nil || !sameAuthenticatedProcess(authIdentity, recheckedTarget) {
			return sessionIdentity{}, fmt.Errorf("session pid identity changed during ancestry proof")
		}
		startTicks, bootID = authIdentity.PidStartTicks, authIdentity.BootID
		return sessionIdentity{
			Session: sessionValue(options.Session, pid), Pid: pid, StartTime: started,
			StartTicks: startTicks, BootID: bootID, Tag: sessionTag(options.Tag, runtimeName, sessionValue(options.Session, pid)),
			Runtime: runtimeValue(runtimeName), OwnerLineage: ownerLineage(options.OwnerLineage),
			Provenance: lease.IdentityProvenance{
				Source: "explicit-ancestry-fallback", CallerPid: callerIdentity.Pid,
				CallerPidStartedAt: callerIdentity.PidStartedAt,
			},
		}, nil
	}
	session := sessionValue(options.Session, pid)
	runtimeName = runtimeValue(runtimeName)
	tag := sessionTag(options.Tag, runtimeName, session)
	lineage := ownerLineage(options.OwnerLineage)
	return sessionIdentity{
		Session: session, Pid: pid, StartTime: started, StartTicks: startTicks,
		BootID: bootID, Tag: tag, Runtime: runtimeName, OwnerLineage: lineage,
		Provenance: lease.IdentityProvenance{Source: "runtime-signature-ancestry"},
	}, nil
}

func sessionValue(value string, pid int64) string {
	session := value
	if session == "" {
		session = os.Getenv("METASYSTEM_SESSION_ID")
	}
	if session == "" {
		session = fmt.Sprintf("session-%d", pid)
	}
	return session
}

func runtimeValue(value string) string {
	if value == "" {
		return "unknown"
	}
	return value
}

func sessionTag(value, runtimeName, session string) string {
	tag := value
	if tag == "" {
		tag = os.Getenv("METASYSTEM_INSTANCE_TAG")
	}
	if tag == "" {
		tag = "metasystem-main-" + runtimeName + "-" + lease.Slug(session)
	}
	return tag
}

func ownerLineage(value string) string {
	lineage := value
	if lineage == "" {
		lineage = os.Getenv("METASYSTEM_OWNER_LINEAGE")
	}
	return lineage
}

var requiredCommands = []string{
	"git", "ps", "pgrep", "awk", "sed", "grep", "tar", "date", "mktemp", "stat", "cksum",
	"find", "install", "ln", "tr", "sort", "wc", "head", "tail", "cut", "tee", "uname",
	"basename", "dirname", "touch", "chmod", "mkdir", "rmdir", "cat", "cp", "mv", "rm",
}

func preflightCommands() error {
	var missing []string
	for _, command := range requiredCommands {
		if _, err := exec.LookPath(command); err != nil {
			missing = append(missing, command)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("required production commands are missing: %s", strings.Join(missing, ", "))
	}
	return nil
}

func watchInterval(root string) (int, error) {
	value, _, err := config.Get(config.GetParams{
		Key: "watch.interval-sec", ConfPath: filepath.Join(root, "metasystem.conf"),
		Default: "60", DefaultSet: true,
	})
	if err != nil {
		return 0, err
	}
	interval, err := strconv.Atoi(value)
	if err != nil || interval < 1 {
		return 0, fmt.Errorf("watch.interval-sec must be a positive integer")
	}
	return interval, nil
}

func appendArmingLog(root, message string) {
	path := filepath.Join(supervise.SupervisionDir(root), "arming.log")
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer file.Close()
	_, _ = fmt.Fprintf(file, "%s %s\n", time.Now().UTC().Format(time.RFC3339), message)
}

func supervisionOptions(options Options) (supervise.EnsureOptions, error) {
	metasystemRoot := installationRoot(options)
	interval, err := watchInterval(metasystemRoot)
	if err != nil {
		return supervise.EnsureOptions{}, err
	}
	ceiling, err := supervise.DeriveCeiling(filepath.Join(metasystemRoot, "metasystem.conf"), options.MaxCap, os.Environ())
	if err != nil {
		return supervise.EnsureOptions{}, err
	}
	fingerprint, err := census.Fingerprint(metasystemRoot, options.Scope)
	if err != nil {
		return supervise.EnsureOptions{}, err
	}
	return supervise.EnsureOptions{
		Root: options.Root, MetasystemRoot: metasystemRoot, Scope: options.Scope, Binary: options.Binary,
		Fingerprint: fingerprint, IntervalSec: interval, WatcherCap: ceiling,
		OnlyIfDown: options.RecoverOnly && options.IfDown, WaitScaleMilli: options.WaitScaleMilli,
		OwnerTagPrefix: "metasystem-supervision-owner-" + lease.Slug(options.Scope) + "-",
	}, nil
}

func ensureSupervision(options Options, enrolled *steward.EnrolledBinary, components []ComponentOutcome) ([]ComponentOutcome, supervise.EnsureResult, *Result) {
	armingOptions, err := supervisionOptions(options)
	if err != nil {
		failed := failure(components, "supervision-owner", err, "fix the named supervision configuration or fingerprint input, then rerun metasystem up")
		return nil, supervise.EnsureResult{}, &failed
	}
	armingOptions.Command = enrolledCommand(enrolled)
	result, err := supervise.EnsureArmed(armingOptions)
	if err != nil {
		if errors.Is(err, steward.ErrEnrollmentDrift) {
			drift := enrollmentDrift(components, fmt.Errorf("supervision-owner launch: %w", err), installationRoot(options), options.Root)
			return nil, supervise.EnsureResult{}, &drift
		}
		component := "supervision-owner"
		remedy := "inspect artifacts/agents/supervision/owner.log, repair the named blocker, then rerun metasystem up"
		var componentFailure *supervise.ComponentFailure
		if errors.As(err, &componentFailure) {
			component = componentFailure.Component
			remedy = "prove the recorded component identity and process group are gone, then rerun metasystem up"
		}
		failed := failure(components, component, err, remedy)
		return nil, supervise.EnsureResult{}, &failed
	}
	ownerOutcome := ComponentOutcome{
		Component: "supervision-owner", Outcome: result.Action,
		Detail: fmt.Sprintf("pid=%d generation=%d", result.Owner.Pid, result.Generation),
	}
	if !result.Inspection.Armed() {
		if result.Inspection.Component != "supervision-owner" {
			components = append(components, ownerOutcome)
		}
		failed := failure(components, result.Inspection.Component, fmt.Errorf("%s", result.Inspection.Reason),
			"inspect artifacts/agents/supervision/owner.log and rerun metasystem up after the component can complete one pass")
		return nil, result, &failed
	}
	components = append(components, ownerOutcome)
	watcherOutcome := "verified"
	if result.Action == "not-needed" {
		watcherOutcome = "owned"
	}
	components = append(components,
		ComponentOutcome{Component: "repo-watcher", Outcome: watcherOutcome, Detail: fmt.Sprintf("generation=%d", result.Generation)},
		ComponentOutcome{Component: "job-reaper", Outcome: watcherOutcome, Detail: fmt.Sprintf("generation=%d", result.Generation)},
	)
	return components, result, nil
}

func ensureStewardRunner(options Options, enrolled *steward.EnrolledBinary, components []ComponentOutcome) ([]ComponentOutcome, *Result) {
	result, err := stewardEnsureRunner(options.Root, enrolled, options.WaitScaleMilli)
	if err != nil {
		if errors.Is(err, steward.ErrEnrollmentDrift) {
			drift := enrollmentDrift(components, fmt.Errorf("steward-runner launch: %w", err), installationRoot(options), options.Root)
			return nil, &drift
		}
		failed := failure(components, "steward-runner", err,
			"configure a working notification channel, inspect artifacts/agents/steward/runner.log, then rerun metasystem up")
		return nil, &failed
	}
	detail := fmt.Sprintf("pid=%d generation=%d", result.Pid, result.Generation)
	if result.Action == "excluded" {
		detail = "standing runner is excluded by fixture or linked-worktree policy"
	}
	components = append(components, ComponentOutcome{
		Component: "steward-runner", Outcome: result.Action, Detail: detail,
	})
	return components, nil
}

func enrollmentDrift(components []ComponentOutcome, err error, installationRoot, repoRoot string) Result {
	remedy := fmt.Sprintf("this engine is not eligible for automatic re-arm; from an agent-free terminal run metasystem steward restart --repo %s (steward arm when no runner is live), or relay the human's recorded word with --temporary-human-word and --review-by", repoRoot)
	if strings.Contains(err.Error(), "owns no resolving remote-tracking landing ref") {
		remedy = fmt.Sprintf("fetch or pull the configured remote once so its remote-tracking landing ref resolves, or from an agent-free terminal run metasystem steward restart --repo %s", repoRoot)
	} else if strings.Contains(err.Error(), "owns no remote-tracking landing ref") {
		remedy = fmt.Sprintf("run git -C %s config --local metasystem.steward.landing-ref refs/remotes/<remote>/<branch> once on this machine, or re-arm at the terminal", installationRoot)
	} else if remote, ok := notLandedRemote(err.Error()); ok {
		remedy = fmt.Sprintf("run git -C %s fetch %s once, then rerun metasystem up, or from an agent-free terminal run metasystem steward restart --repo %s", installationRoot, remote, repoRoot)
	}
	components = append(components, ComponentOutcome{
		Component: "accepted-engine", Outcome: "ENROLLMENT_DRIFT", Detail: err.Error(), Remedy: remedy,
	})
	return Result{Components: components, Outcome: "ENROLLMENT_DRIFT", Failed: "accepted-engine", Remedy: remedy}
}

func notLandedRemote(message string) (string, bool) {
	for _, marker := range []string{"not proven landed on ", "not landed on "} {
		at := strings.Index(message, marker)
		if at < 0 {
			continue
		}
		fields := strings.Fields(message[at+len(marker):])
		if len(fields) == 0 {
			continue
		}
		ref := strings.TrimSuffix(fields[0], ":")
		tail := strings.TrimPrefix(ref, "refs/remotes/")
		remote, branch, qualified := strings.Cut(tail, "/")
		if tail != ref && qualified && remote != "" && branch != "" {
			return remote, true
		}
	}
	return "", false
}

func beforeMintRemedy(err error, repoRoot string) string {
	message := err.Error()
	switch {
	case strings.Contains(message, "no notification channel is configured"):
		return fmt.Sprintf("set the notification command with git -C %s config --local metasystem.steward.notify-command <command>, then rerun metasystem up", repoRoot)
	case strings.Contains(message, "create runner directory"):
		return fmt.Sprintf("make the steward runner directory %s writable, then rerun metasystem up", filepath.Join(repoRoot, "artifacts", "agents", "steward"))
	case strings.Contains(message, "open arm lock"), strings.Contains(message, "take arm lock"):
		return fmt.Sprintf("make the steward arm lock file %s creatable, openable, and lockable by this user by checking its directory permissions, free space, and the open-file limit, then rerun metasystem up", filepath.Join(repoRoot, "artifacts", "agents", "steward", "arm.flock"))
	case strings.Contains(message, "re-publish identity with durability pending"):
		return "repair the enrollment identity publication, then rerun metasystem up"
	default:
		return "repair the named enrollment publication failure, then rerun metasystem up"
	}
}

func openInvokingEnrollment(options Options, allowReArm bool) (*steward.EnrolledBinary, steward.ReArmOutcome, error) {
	var rearmed steward.ReArmOutcome
	enrolled, err := steward.OpenEnrolledBinary(options.Root)
	if allowReArm && errors.Is(err, steward.ErrEngineRebuilt) {
		rearmed, err = steward.ReArmRebuiltEngine(options.Root, installationRoot(options), options.Binary)
		if err == nil {
			enrolled, err = steward.OpenEnrolledBinary(options.Root)
		}
	}
	if err != nil {
		return nil, rearmed, err
	}
	if canonicalRuntimePath(enrolled.Install.InstallPath) != canonicalRuntimePath(options.Binary) {
		_ = enrolled.Close()
		return nil, rearmed, fmt.Errorf("%w: invoking engine %q is not enrolled engine %q",
			steward.ErrEnrollmentDrift, canonicalRuntimePath(options.Binary), enrolled.Install.InstallPath)
	}
	return enrolled, rearmed, nil
}

type rearmFact struct {
	value             string
	durabilityPending bool
}

func factFromRearm(outcome steward.ReArmOutcome) rearmFact {
	if outcome.Status != "re-armed" {
		return rearmFact{}
	}
	value := fmt.Sprintf("generation=%d previous=%d engine=%s landed=%s",
		outcome.Generation, outcome.PreviousGeneration, outcome.EngineBuild, shortCommit(outcome.LandedCommit))
	if outcome.DurabilityPending {
		value += " (durability pending)"
	}
	return rearmFact{value: value, durabilityPending: outcome.DurabilityPending}
}

func shortCommit(commit string) string {
	if len(commit) > 7 {
		return commit[:7]
	}
	return commit
}

func acceptedEngine(enrolled *steward.EnrolledBinary, rearmed steward.ReArmOutcome) ComponentOutcome {
	if rearmed.Status == "re-armed" {
		detail := fmt.Sprintf("the enrolled engine was rebuilt; armed (runner pid %d) (generation=%d previous=%d engine=%s landed=%s ref=%s path=%s)",
			rearmed.RunnerPid, rearmed.Generation, rearmed.PreviousGeneration, rearmed.EngineBuild,
			shortCommit(rearmed.LandedCommit), rearmed.LandingRef, enrolled.Install.InstallPath)
		if rearmed.DurabilityPending {
			detail += " (durability pending)"
		}
		if rearmed.NoticeErr != nil {
			detail += "; pending notification not written: " + rearmed.NoticeErr.Error()
		}
		return ComponentOutcome{Component: "accepted-engine", Outcome: "re-armed", Detail: detail}
	}
	if rearmed.Status == "already-current" {
		return ComponentOutcome{
			Component: "accepted-engine", Outcome: "verified",
			Detail: fmt.Sprintf("generation=%d path=%s; brought current by a concurrent arm", enrolled.Install.Generation, enrolled.Install.InstallPath),
		}
	}
	return ComponentOutcome{
		Component: "accepted-engine", Outcome: "verified",
		Detail: fmt.Sprintf("generation=%d path=%s", enrolled.Install.Generation, enrolled.Install.InstallPath),
	}
}

func ordinary(options Options) Result {
	result, fact := ordinaryBody(options)
	result.ReArmed = fact.value
	result.DurabilityPending = fact.durabilityPending
	return result
}

func ordinaryBody(options Options) (Result, rearmFact) {
	components := []ComponentOutcome{}
	fact := rearmFact{}
	finish := func(result Result) (Result, rearmFact) { return result, fact }
	if err := preflightCommands(); err != nil {
		return finish(failure(components, "host-preflight", err, "install the named commands and rerun metasystem up"))
	}
	components = append(components, ComponentOutcome{Component: "host-preflight", Outcome: "verified"})
	enrolled, rearmed, err := openInvokingEnrollment(options, true)
	if rearmed.Stage == steward.StageMinted {
		fact = factFromRearm(rearmed)
		logLine := fmt.Sprintf("engine-re-armed generation=%d previous=%d engine=%s landed=%s ref=%s",
			rearmed.Generation, rearmed.PreviousGeneration, rearmed.EngineBuild, shortCommit(rearmed.LandedCommit), rearmed.LandingRef)
		if rearmed.NoticeErr != nil {
			logLine += fmt.Sprintf(" notice-error=%q", rearmed.NoticeErr.Error())
		}
		if rearmed.DurabilityPending {
			logLine += " durability-pending=true"
		}
		appendArmingLog(options.Root, logLine)
	}
	if err != nil {
		switch rearmed.Stage {
		case steward.StageStopAttempted:
			detail := fmt.Errorf("re-arm eligible (engine %s landed %s on %s); stopping runner pid %d for replacement failed: %w; the enrollment still names generation %d",
				rearmed.EngineBuild, shortCommit(rearmed.LandedCommit), rearmed.LandingRef,
				rearmed.StoppedRunnerPid, err, rearmed.PreviousGeneration)
			return finish(failure(components, "accepted-engine", detail,
				fmt.Sprintf("prove runner pid %d is gone in artifacts/agents/steward/runner.json, then rerun metasystem up", rearmed.StoppedRunnerPid)))
		case steward.StageStopped:
			return finish(failure(components, "accepted-engine",
				fmt.Errorf("runner pid %d was confirmed stopped, but the new enrollment could not be minted: %w", rearmed.StoppedRunnerPid, err),
				"rerun metasystem up; the stopped runner will be replaced after a durable mint"))
		case steward.StageMinted:
			components = append(components, ComponentOutcome{
				Component: "accepted-engine", Outcome: "re-armed",
				Detail: fmt.Sprintf("generation=%d previous=%d engine=%s landed=%s ref=%s", rearmed.Generation,
					rearmed.PreviousGeneration, rearmed.EngineBuild, shortCommit(rearmed.LandedCommit), rearmed.LandingRef),
			})
			if errors.Is(err, steward.ErrEnrollmentDrift) {
				return finish(enrollmentDrift(components, err, installationRoot(options), options.Root))
			}
			return finish(failure(components, "steward-runner", fmt.Errorf("after re-arm: %w", err),
				"inspect artifacts/agents/steward/runner.log and engine-pins, then rerun metasystem up"))
		default:
			if errors.Is(err, steward.ErrEnrollmentDrift) {
				return finish(enrollmentDrift(components, err, installationRoot(options), options.Root))
			}
			return finish(failure(components, "accepted-engine", err, beforeMintRemedy(err, options.Root)))
		}
	}
	defer enrolled.Close()
	components = append(components, acceptedEngine(enrolled, rearmed))

	session, err := resolveSessionIdentity(options)
	if err != nil {
		return finish(failure(components, "session-identity", err,
			"pass --pid <session-pid> and --start-time <epoch-seconds>, or configure a runtime signature and invoke up from that session"))
	}
	if err := enrolled.PrepareForExecution(); err != nil {
		return finish(enrollmentDrift(components, err, installationRoot(options), options.Root))
	}
	components = append(components, ComponentOutcome{
		Component: "session-identity", Outcome: "verified",
		Detail: fmt.Sprintf("runtime=%s pid=%d start=%d", session.Runtime, session.Pid, session.StartTime),
	})
	announcement, err := lease.AnnounceWithProofAt(options.Root, installationRoot(options), session.Session, session.Pid, session.StartTime,
		session.StartTicks, session.BootID, session.Tag, session.Runtime, session.OwnerLineage, &session.Provenance)
	if err != nil {
		return finish(failure(components, "session-announcement", err, "repair the named announcement or lease state, then rerun metasystem up"))
	}
	components = append(components, ComponentOutcome{
		Component: "session-announcement", Outcome: "verified", Detail: announcement,
	})
	appendArmingLog(options.Root, fmt.Sprintf("announcement-written registry=%s pid=%d start=%d", announcement, session.Pid, session.StartTime))
	view, err := lease.ClassifyVerbAt(options.Root, installationRoot(options), session.Pid)
	if err != nil {
		return finish(failure(components, "checkout-lease", err, "repair the checkout lease and rerun metasystem up"))
	}
	authority := "writer"
	holderName := view.MainId
	if !view.Holder {
		authority = "read-only"
		holder, holderErr := lease.CurrentHolder(options.Root)
		if holderErr != nil {
			return finish(failure(components, "checkout-lease", holderErr, "repair the checkout lease and rerun metasystem up"))
		}
		holderName = holder.MainId
		if holder.SessionId != "" {
			holderName = holder.SessionId + " (" + holder.MainId + ")"
		}
		components = append(components, ComponentOutcome{
			Component: "checkout-lease", Outcome: "advisor",
			Detail: "holder=" + holderName + "; this session has reading authority only",
			Remedy: "run scripts/agents/second-session.sh to create an isolated writer worktree",
		})
	} else {
		components = append(components, ComponentOutcome{
			Component: "checkout-lease", Outcome: "holder", Detail: "main=" + holderName,
		})
	}
	components, supervision, failed := ensureSupervision(options, enrolled, components)
	if failed != nil {
		return finish(*failed)
	}
	appendArmingLog(options.Root, fmt.Sprintf("first-census-complete repo=%s owner=%d", options.Scope, supervision.Owner.Pid))
	components, stewardFailed := ensureStewardRunner(options, enrolled, components)
	if stewardFailed != nil {
		return finish(*stewardFailed)
	}
	if authority == "read-only" {
		return finish(Result{
			Components: components, Outcome: "advisor", Authority: authority, Holder: holderName,
			Worktree: "scripts/agents/second-session.sh",
		})
	}
	return finish(Result{Components: components, Outcome: "armed", Authority: authority})
}

func recovery(options Options) Result {
	components := []ComponentOutcome{}
	if !options.IfDown {
		return failure(components, "recovery-mode", fmt.Errorf("--recover-only requires --if-down"),
			"invoke ordinary metasystem up from a session, or add --if-down for the scheduler recovery path")
	}
	enrolled, _, err := openInvokingEnrollment(options, false)
	if err != nil {
		result := enrollmentDrift(components, err, installationRoot(options), options.Root)
		if errors.Is(err, steward.ErrEngineRebuilt) {
			result.Remedy = "run metasystem up from a session, which re-arms a rebuilt engine at its enrolled path; or run metasystem steward arm at an agent-free terminal"
			result.Components[len(result.Components)-1].Remedy = result.Remedy
		}
		return result
	}
	defer enrolled.Close()
	if err := enrolled.PrepareForExecution(); err != nil {
		return enrollmentDrift(components, err, installationRoot(options), options.Root)
	}
	options.Binary = enrolled.Install.InstallPath
	components = append(components, ComponentOutcome{
		Component: "accepted-engine", Outcome: "verified",
		Detail: fmt.Sprintf("generation=%d path=%s", enrolled.Install.Generation, enrolled.Install.InstallPath),
	})
	components, supervision, failed := ensureSupervision(options, enrolled, components)
	if failed != nil {
		failed.Outcome = "recovery-partial"
		return *failed
	}
	repair, err := steward.RepairEnrolledRunner(options.Root)
	if err != nil {
		result := failure(components, "steward-runner", err, "run ordinary metasystem up from a session to repair steward enrollment")
		result.Outcome = "recovery-partial"
		return result
	}
	if repair.Status == "NOT_ENROLLED" || repair.Status == "ENROLLMENT_CHANGED" || repair.Status == steward.AutoHealEnded {
		result := failure(components, "steward-runner", fmt.Errorf("recovery stopped with %s", repair.Status),
			"run ordinary metasystem up from a session to establish the steward generation")
		result.Outcome = "recovery-partial"
		return result
	}
	components = append(components, ComponentOutcome{
		Component: "steward-runner", Outcome: strings.ToLower(repair.Status),
		Detail: fmt.Sprintf("pid=%d generation=%d", repair.ReplacementPid, repair.Generation),
	})
	outcome := "recovery-not-needed"
	if supervision.Action != "not-needed" || repair.Status == "RESTORED" {
		outcome = "recovery-started"
	}
	return Result{Components: components, Outcome: outcome, Authority: "none"}
}

func canonicalRuntimePath(path string) string {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	if resolved, err := filepath.EvalSymlinks(absolute); err == nil {
		return resolved
	}
	return filepath.Clean(absolute)
}

func clearInheritedExecutionID() {
	// Up is a session/ring transaction, never a delegated mission action.
	// Clear inherited attribution before announcements, lease events, or any
	// detached supervision and steward children are created.
	_ = os.Unsetenv("METASYSTEM_EXECUTION_ID")
}

// Run performs ordinary session arming or the restricted recovery-only path.
func Run(options Options) Result {
	clearInheritedExecutionID()
	if options.RecoverOnly {
		return recovery(options)
	}
	return ordinary(options)
}

// Retire removes only this session's announcement.
func Retire(options Options) Result {
	clearInheritedExecutionID()
	session, err := resolveSessionIdentity(options)
	if err != nil {
		return failure(nil, "session-identity", err,
			"pass --pid <session-pid> and --start-time <epoch-seconds>")
	}
	if err := lease.Retire(options.Root, session.Session, session.Pid, session.StartTime); err != nil {
		return failure(nil, "session-announcement", err, "inspect the announcement registry and retry retirement")
	}
	return Result{Components: []ComponentOutcome{{Component: "session-announcement", Outcome: "retired"}}, Outcome: "retired"}
}

// Shutdown stops supervision for fixture and administrative cleanup. It is a
// compatibility operation, not part of the daily operator surface.
func Shutdown(options Options) Result {
	clearInheritedExecutionID()
	if _, err := lease.RequireHolderAt(options.Root, installationRoot(options), int64(os.Getppid()), nil); err != nil {
		return failure(nil, "checkout-lease", err, "run shutdown from the checkout holder")
	}
	prefix := "metasystem-supervision-owner-" + lease.Slug(options.Scope) + "-"
	if err := supervise.ShutdownAt(options.Root, installationRoot(options), options.Root, prefix, options.WaitScaleMilli); err != nil {
		return failure(nil, "supervision-owner", err, "inspect the recorded owner identity before retrying shutdown")
	}
	return Result{Components: []ComponentOutcome{{Component: "supervision-owner", Outcome: "stopped"}}, Outcome: "stopped"}
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

// SchedulerEntry prints the optional operator-owned recovery entry. It has no
// filesystem side effects and the command it prints carries no session or
// lease authority.
func SchedulerEntry(options Options) string {
	return fmt.Sprintf("0 * * * * cd %s && %s up --metasystem-root %s --repo %s --recover-only --if-down",
		shellQuote(options.Scope), shellQuote(options.Binary), shellQuote(installationRoot(options)), shellQuote(options.Scope))
}
