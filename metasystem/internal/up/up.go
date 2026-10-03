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
	"github.com/widoriezebos/agentic-tools/metasystem/internal/shellquote"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise"
)

// Options are the complete inputs to ordinary and recovery-only arming.
type Options struct {
	Root                  string
	MetasystemRoot        string
	Scope                 string
	Binary                string
	Session               string
	Pid                   int64
	StartTime             int64
	Tag                   string
	Runtime               string
	OwnerLineage          string
	RuntimeSession        string
	NoRuntimeSession      bool
	StartSource           string
	MaxCap                int64
	RecoverOnly           bool
	IfDown                bool
	WaitScaleMilli        int
	CallerPid             int64
	RestampStopCapability func(root, lineage string, claimEpoch int64) (StopCapabilityRestampResult, error)
	// FindSessionAncestor infers the session's main from the caller's
	// runtime-signature ancestry when no --pid/--start-time pair is given;
	// nil means the census's production ancestry walk.
	FindSessionAncestor func(metasystemRoot string, pid int64, runtime string) (census.AgentAncestor, error)
	// ReArmRebuiltEngine, EnsureArmed and EnsureStewardRunner replace the
	// steward re-arm, supervision arming and steward-runner owners for one
	// call. Production leaves them nil and uses the owners directly.
	ReArmRebuiltEngine  func(repoRoot, installationRoot, invokingBinary string) (steward.ReArmOutcome, error)
	EnsureArmed         func(supervise.EnsureOptions) (supervise.EnsureResult, error)
	EnsureStewardRunner func(repoRoot string, enrolled *steward.EnrolledBinary, scaleMilli int) (steward.EnsureRunnerResult, error)
}

// StopCapabilityRestampResult reports the accepted claim inspected during
// arming and whether its capability moved.
type StopCapabilityRestampResult struct {
	GoalID    string
	FromEpoch int64
	ToEpoch   int64
	Restamped bool
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
	RawLines          []string
	Outcome           string
	Authority         string
	ReArmed           string
	DurabilityPending bool
	Holder            string
	Worktree          string
	Failed            string
	Remedy            string
}

// Data is what up's --json envelope carries for a parent to branch on:
// up's outcome, the component it stopped at, and what it re-armed.
type Data struct {
	Outcome string `json:"outcome"`
	Failed  string `json:"failed,omitempty"`
	ReArmed string `json:"reArmed,omitempty"`
}

// Data is the result's typed facts for the envelope.
func (r Result) Data() Data {
	return Data{Outcome: r.Outcome, Failed: r.Failed, ReArmed: r.ReArmed}
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
	lines := make([]string, 0, len(r.Components)+len(r.RawLines)+1)
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
	lines = append(lines, r.RawLines...)
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

func stopCapabilityOutcome(options Options, lineage string, claimEpoch int64) ComponentOutcome {
	if options.RestampStopCapability == nil {
		return ComponentOutcome{Component: "stop-capability", Outcome: "no-claimed-goal"}
	}
	result, err := options.RestampStopCapability(options.Root, lineage, claimEpoch)
	if err != nil {
		goalID := result.GoalID
		if goalID == "" {
			goalID = "<goal>"
		}
		return ComponentOutcome{
			Component: "stop-capability", Outcome: "deferred", Detail: err.Error(),
			Remedy: "metasystem session start",
		}
	}
	if result.GoalID == "" {
		return ComponentOutcome{Component: "stop-capability", Outcome: "no-claimed-goal"}
	}
	if result.Restamped {
		return ComponentOutcome{Component: "stop-capability", Outcome: "restamped",
			Detail: fmt.Sprintf("from %d to %d", result.FromEpoch, result.ToEpoch)}
	}
	return ComponentOutcome{Component: "stop-capability", Outcome: "current",
		Detail: fmt.Sprintf("goal=%s epoch=%d", result.GoalID, result.ToEpoch)}
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
	return errors.New("the session process you named is not this process or one that started it")
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
		findAncestor := options.FindSessionAncestor
		if findAncestor == nil {
			findAncestor = census.FindAncestorProduction
		}
		ancestor, err := findAncestor(installationRoot(options), int64(os.Getppid()), runtimeName)
		if err != nil {
			return sessionIdentity{}, fmt.Errorf("this process could not be traced back to its session: %w", err)
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
			return sessionIdentity{}, errors.New("this process changed while it was being traced back to its session")
		}
		recheckedTarget, err := census.AuthIdentity(pid, authorization.Identity())
		if err != nil || !sameAuthenticatedProcess(authIdentity, recheckedTarget) {
			return sessionIdentity{}, errors.New("the session process changed while it was being checked")
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
		session = claudeSession(pid)
	}
	if session == "" {
		session = fmt.Sprintf("session-%d", pid)
	}
	return session
}

// claudeSession is the session id Claude Code exports to the commands its
// session runs, so a session start run by hand inside a Claude session names
// the session as its SessionStart hook does. It counts only when CLAUDE_PID is
// the session process itself: the variables are inherited, so a process
// beneath another session process can carry an outer session's values.
func claudeSession(pid int64) string {
	if os.Getenv("CLAUDE_PID") != strconv.FormatInt(pid, 10) {
		return ""
	}
	return os.Getenv("CLAUDE_CODE_SESSION_ID")
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

func associateRuntimeSession(options Options, mainID, event string) error {
	if options.NoRuntimeSession || options.RuntimeSession == "" {
		return nil
	}
	return lease.AssociateSession(options.Root, mainID, options.RuntimeSession, event, options.StartSource)
}

func ownerLineage(value string) string {
	lineage := value
	if lineage == "" {
		lineage = os.Getenv("METASYSTEM_OWNER_LINEAGE")
	}
	return lineage
}

// ProductionCommand is one command the production scripts exec, with the
// Debian-family package that provides it. The inventory is a contract (the
// supported-platforms rule in docs/project-rules.md): arming and adoption
// name anything missing up front instead of failing mid-operation.
type ProductionCommand struct {
	Name, Package string
}

// ProductionCommands is the command inventory. shasum is deliberately absent:
// production hashing is the engine's own Go.
var ProductionCommands = []ProductionCommand{
	{"git", "git"}, {"ps", "procps"}, {"pgrep", "procps"}, {"awk", "mawk or gawk"},
	{"sed", "sed"}, {"grep", "grep"}, {"tar", "tar"}, {"date", "coreutils"},
	{"mktemp", "coreutils"}, {"stat", "coreutils"}, {"cksum", "coreutils"}, {"find", "findutils"},
	{"install", "coreutils"}, {"ln", "coreutils"}, {"tr", "coreutils"}, {"sort", "coreutils"},
	{"wc", "coreutils"}, {"head", "coreutils"}, {"tail", "coreutils"}, {"cut", "coreutils"},
	{"tee", "coreutils"}, {"uname", "coreutils"}, {"basename", "coreutils"}, {"dirname", "coreutils"},
	{"touch", "coreutils"}, {"chmod", "coreutils"}, {"mkdir", "coreutils"}, {"rmdir", "coreutils"},
	{"cat", "coreutils"}, {"cp", "coreutils"}, {"mv", "coreutils"}, {"rm", "coreutils"},
}

// MissingProductionCommands returns the inventory commands lookPath cannot
// find, in inventory order.
func MissingProductionCommands(lookPath func(string) (string, error)) []ProductionCommand {
	var missing []ProductionCommand
	for _, command := range ProductionCommands {
		if _, err := lookPath(command.Name); err != nil {
			missing = append(missing, command)
		}
	}
	return missing
}

func preflightCommands() error {
	var missing []string
	for _, command := range MissingProductionCommands(exec.LookPath) {
		missing = append(missing, command.Name)
	}
	if len(missing) > 0 {
		return fmt.Errorf("required production commands are missing: %s", strings.Join(missing, ", "))
	}
	return nil
}

func watchInterval(root string) (int, error) {
	value, _, err := config.Get(config.GetParams{
		Key: "watch.interval-sec", ConfPath: filepath.Join(root, "metasystem.conf"),
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
		failed := failure(components, "supervision-owner", err, "fix the named supervision configuration or fingerprint input, then rerun metasystem session start")
		return nil, supervise.EnsureResult{}, &failed
	}
	fence, err := stopfence.Read(options.Root)
	if err != nil {
		failed := failure(components, "supervision-owner", err, "repair the process-creation fence, then rerun metasystem system start")
		return nil, supervise.EnsureResult{}, &failed
	}
	exact, state, probeErr := (processidentity.KernelProber{}).Probe(int64(os.Getpid()))
	if probeErr != nil || state != processidentity.Alive {
		failed := failure(components, "supervision-owner", fmt.Errorf("cannot prove the arming process identity: %v", probeErr), "retry from the same terminal")
		return nil, supervise.EnsureResult{}, &failed
	}
	claim, err := stopfence.Creating(options.Root, "supervision-owner", fence.Generation, exact.Ref())
	if err != nil {
		failed := failure(components, "supervision-owner", err, "repair the creation-claim directory, then rerun metasystem system start")
		return nil, supervise.EnsureResult{}, &failed
	}
	defer claim.Close()
	armingOptions.FenceGeneration = fence.Generation
	armingOptions.Command = enrolledCommand(enrolled)
	ensureArmed := supervise.EnsureArmed
	if options.EnsureArmed != nil {
		ensureArmed = options.EnsureArmed
	}
	result, err := ensureArmed(armingOptions)
	if err != nil {
		if errors.Is(err, steward.ErrEnrollmentDrift) {
			drift := enrollmentDrift(components, fmt.Errorf("supervision-owner launch: %w", err), installationRoot(options), options.Root)
			return nil, supervise.EnsureResult{}, &drift
		}
		component := "supervision-owner"
		remedy := "inspect artifacts/agents/supervision/owner.log, repair the named blocker, then rerun metasystem session start"
		var componentFailure *supervise.ComponentFailure
		if errors.As(err, &componentFailure) {
			component = componentFailure.Component
			remedy = "prove the recorded component identity and process group are gone, then rerun metasystem session start"
		}
		failed := failure(components, component, err, remedy)
		return nil, supervise.EnsureResult{}, &failed
	}
	second, err := stopfence.Read(options.Root)
	if err != nil || second.State == stopfence.StateClosed || second.Generation != fence.Generation {
		prefix := "metasystem-supervision-owner-" + lease.Slug(options.Scope) + "-"
		_, _ = supervise.ShutdownAt(options.Root, installationRoot(options), options.Root, prefix, options.WaitScaleMilli)
		remedy := "repair the process-creation fence, then rerun metasystem session start"
		if err == nil && second.State == stopfence.StateClosed {
			description, renderErr := stopfence.ClosedDescription(second, options.Scope)
			command, commandErr := stopfence.ClosedCommand(second, options.Scope)
			if renderErr != nil {
				err = renderErr
			} else if commandErr != nil {
				err = commandErr
			} else {
				err = fmt.Errorf("%s; while the supervision owner started, it has been ended", description)
				remedy = "at an agent-free terminal, run: " + command
			}
		} else if err == nil {
			err = fmt.Errorf("%s was restarted while its supervisor started, so this supervisor was ended; run the command again", options.Scope)
			remedy = "retry metasystem session start"
		}
		failed := failure(components, "supervision-owner", err, remedy)
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
			"inspect artifacts/agents/supervision/owner.log and rerun metasystem session start after the component can complete one pass")
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
	ensureRunner := stewardEnsureRunner
	if options.EnsureStewardRunner != nil {
		ensureRunner = options.EnsureStewardRunner
	}
	result, err := ensureRunner(options.Root, enrolled, options.WaitScaleMilli)
	if err != nil {
		if errors.Is(err, steward.ErrEnrollmentDrift) {
			drift := enrollmentDrift(components, fmt.Errorf("steward-runner launch: %w", err), installationRoot(options), options.Root)
			return nil, &drift
		}
		failed := failure(components, "steward-runner", err,
			"configure a working notification channel, inspect artifacts/agents/steward/runner.log, then rerun metasystem session start")
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
	remedy := fmt.Sprintf("this engine is not eligible for automatic re-arm; from an agent-free terminal run metasystem system start --repo %s, or relay the human's recorded word with --temporary-human-word and --review-by", repoRoot)
	var landing *steward.LandingRefError
	isLanding := errors.As(err, &landing)
	if isLanding && landing.Unresolved {
		remedy = fmt.Sprintf("fetch or pull the configured remote once so its remote-tracking landing ref resolves, or from an agent-free terminal run metasystem system start --repo %s", repoRoot)
	} else if errors.Is(err, steward.ErrNoLandingRef) {
		remedy = fmt.Sprintf("run git -C %s config --local metasystem.steward.landing-ref refs/remotes/<remote>/<branch> once on this machine, or re-arm at the terminal", installationRoot)
	} else if remote, ok := landingRemote(landing, isLanding); ok {
		remedy = fmt.Sprintf("run git -C %s fetch %s once, then rerun metasystem session start, or from an agent-free terminal run metasystem system start --repo %s", installationRoot, remote, repoRoot)
	}
	components = append(components, ComponentOutcome{
		Component: "accepted-engine", Outcome: "ENROLLMENT_DRIFT", Detail: err.Error(), Remedy: remedy,
	})
	return Result{Components: components, Outcome: "ENROLLMENT_DRIFT", Failed: "accepted-engine", Remedy: remedy}
}

// landingRemote is the remote a not-landed judgment's ref names.
func landingRemote(landing *steward.LandingRefError, found bool) (string, bool) {
	if !found {
		return "", false
	}
	return landing.Remote()
}

func beforeMintRemedy(err error, repoRoot string) string {
	var step *steward.ArmStepError
	if !errors.As(err, &step) {
		return "repair the named enrollment publication failure, then rerun metasystem session start"
	}
	switch step.Step {
	case steward.ArmStepNotify:
		return fmt.Sprintf("set the notification command with git -C %s config --local metasystem.steward.notify-command <command>, then rerun metasystem session start", repoRoot)
	case steward.ArmStepRunnerDirectory:
		return fmt.Sprintf("make the steward runner directory %s writable, then rerun metasystem session start", filepath.Join(repoRoot, "artifacts", "agents", "steward"))
	case steward.ArmStepLock:
		return fmt.Sprintf("make the steward arm lock file %s creatable, openable, and lockable by this user by checking its directory permissions, free space, and the open-file limit, then rerun metasystem session start", filepath.Join(repoRoot, "artifacts", "agents", "steward", "arm.flock"))
	case steward.ArmStepIdentity:
		return "repair the enrollment identity publication, then rerun metasystem session start"
	}
	return "repair the named enrollment publication failure, then rerun metasystem session start"
}

func openInvokingEnrollment(options Options, allowReArm bool) (*steward.EnrolledBinary, steward.ReArmOutcome, error) {
	var rearmed steward.ReArmOutcome
	enrolled, err := steward.OpenEnrolledBinary(options.Root)
	if allowReArm && errors.Is(err, steward.ErrEngineRebuilt) {
		reArm := steward.ReArmRebuiltEngine
		if options.ReArmRebuiltEngine != nil {
			reArm = options.ReArmRebuiltEngine
		}
		rearmed, err = reArm(options.Root, installationRoot(options), options.Binary)
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
			detail := fmt.Errorf("the new engine %s (landed %s on %s) is not in use: the old steward (pid %d) did not stop\n%w; install %d stays",
				rearmed.EngineBuild, shortCommit(rearmed.LandedCommit), rearmed.LandingRef,
				rearmed.StoppedRunnerPid, err, rearmed.PreviousGeneration)
			return finish(failure(components, "accepted-engine", detail,
				fmt.Sprintf("prove runner pid %d is gone in artifacts/agents/steward/runner.json, then rerun metasystem session start", rearmed.StoppedRunnerPid)))
		case steward.StageStopped:
			return finish(failure(components, "accepted-engine",
				fmt.Errorf("runner pid %d was confirmed stopped, but the new enrollment could not be minted: %w", rearmed.StoppedRunnerPid, err),
				"rerun metasystem session start; the stopped runner will be replaced after a durable mint"))
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
				"inspect artifacts/agents/steward/runner.log and engine-pins, then rerun metasystem session start"))
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
		return finish(failure(components, "session-announcement", err, "repair the named announcement or lease state, then rerun metasystem session start"))
	}
	components = append(components, ComponentOutcome{
		Component: "session-announcement", Outcome: "verified", Detail: announcement,
	})
	appendArmingLog(options.Root, fmt.Sprintf("announcement-written registry=%s pid=%d start=%d", announcement, session.Pid, session.StartTime))
	view, err := lease.ClassifyVerbAt(options.Root, installationRoot(options), session.Pid)
	if err != nil {
		return finish(failure(components, "checkout-lease", err, "repair the checkout lease and rerun metasystem session start"))
	}
	event := "stop"
	if options.StartSource != "" {
		event = "start"
	}
	if options.RuntimeSession == "" && !options.NoRuntimeSession {
		// An announcement made under the pid alone gains its runtime session
		// here; without one the session cannot register a wait.
		options.RuntimeSession = claudeSession(session.Pid)
	}
	if err := associateRuntimeSession(options, view.MainId, event); err != nil {
		return finish(failure(components, "session-announcement", err, "repair the named announcement and rerun metasystem session start"))
	}
	authority := "writer"
	holderName := view.MainId
	if !view.Holder {
		authority = "read-only"
		holder, holderErr := lease.CurrentHolder(options.Root)
		if holderErr != nil {
			return finish(failure(components, "checkout-lease", holderErr, "repair the checkout lease and rerun metasystem session start"))
		}
		holderName = holder.MainId
		if holder.SessionId != "" {
			holderName = holder.SessionId + " (" + holder.MainId + ")"
		}
		components = append(components, ComponentOutcome{
			Component: "checkout-lease", Outcome: "advisor",
			Detail: "holder=" + holderName + "; this session has reading authority only",
			Remedy: "run metasystem session isolate to create an isolated writer worktree",
		})
	} else {
		components = append(components, ComponentOutcome{
			Component: "checkout-lease", Outcome: "holder", Detail: "main=" + holderName,
		})
		lineage := session.OwnerLineage
		if lineage == "" {
			lineage = view.MainId
		}
		if view.ClaimEpoch == nil {
			components = append(components, ComponentOutcome{
				Component: "stop-capability", Outcome: "deferred",
				Detail: "the holder classification has no claim epoch",
				Remedy: "metasystem session start",
			})
		} else {
			components = append(components, stopCapabilityOutcome(options, lineage, *view.ClaimEpoch))
		}
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
			Worktree: "metasystem session isolate",
		})
	}
	return finish(Result{Components: components, Outcome: "armed", Authority: authority})
}

func recovery(options Options) Result {
	components := []ComponentOutcome{}
	if !options.IfDown {
		return failure(components, "recovery-mode", fmt.Errorf("--recover-only requires --if-down"),
			"invoke ordinary metasystem session start from a session, or add --if-down for the scheduler recovery path")
	}
	enrolled, _, err := openInvokingEnrollment(options, false)
	if err != nil {
		result := enrollmentDrift(components, err, installationRoot(options), options.Root)
		if errors.Is(err, steward.ErrEngineRebuilt) {
			result.Remedy = "run metasystem session start from a session, which re-arms a rebuilt engine at its enrolled path; or a person runs metasystem system start at an agent-free terminal"
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
		result := failure(components, "steward-runner", err, "run ordinary metasystem session start from a session to repair steward enrollment")
		result.Outcome = "recovery-partial"
		return result
	}
	if repair.Status == "NOT_ENROLLED" || repair.Status == "ENROLLMENT_CHANGED" || repair.Status == steward.AutoHealEnded {
		result := failure(components, "steward-runner", fmt.Errorf("recovery stopped with %s", repair.Status),
			"run ordinary metasystem session start from a session to establish the steward generation")
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

// closedFenceResult is the standing answer of a checkout whose process
// creation fence is closed, or false when arming may proceed.
func closedFenceResult(options Options) (Result, bool) {
	closed, record, err := stopfence.Closed(options.Root)
	if err != nil {
		return failure(nil, "stopped", err, "repair the stop fence before starting the metasystem"), true
	}
	if !closed {
		return Result{}, false
	}
	detail := "since " + record.ChangedAt
	if !stopfence.Completed(record) {
		detail, err = stopfence.ClosedDescription(record, options.Scope)
		if err != nil {
			return failure(nil, "stopped", err, "repair the stop fence before starting the metasystem"), true
		}
	}
	remedy, renderErr := stopfence.ClosedCommand(record, options.Scope)
	if renderErr != nil {
		return failure(nil, "stopped", renderErr, "repair the stop fence before starting the metasystem"), true
	}
	return Result{
		Components: []ComponentOutcome{{
			Component: "stopped", Outcome: "standing",
			Detail: detail,
		}},
		Outcome: "stopped",
		Remedy:  remedy,
	}, true
}

// Run performs ordinary session arming or the restricted recovery-only path.
func Run(options Options) Result {
	clearInheritedExecutionID()
	if err := preflightCommands(); err != nil {
		return failure(nil, "host-preflight", err, "install the named commands and rerun metasystem session start")
	}
	if result, closed := closedFenceResult(options); closed {
		return result
	}
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
	view, err := lease.ClassifyVerbAt(options.Root, installationRoot(options), session.Pid)
	if err != nil {
		return failure(nil, "session-announcement", err, "inspect the announcement registry and retry retirement")
	}
	if err := associateRuntimeSession(options, view.MainId, "end"); err != nil {
		return failure(nil, "session-announcement", err, "inspect the announcement registry and retry retirement")
	}
	retireSession := session.Session
	if options.RuntimeSession != "" && !options.NoRuntimeSession {
		retireSession = options.RuntimeSession
	}
	if err := lease.Retire(options.Root, retireSession, session.Pid, session.StartTime); err != nil {
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
	report, err := supervise.ShutdownAt(options.Root, installationRoot(options), options.Root, prefix, options.WaitScaleMilli)
	if err != nil {
		return failure(nil, "supervision-owner", err, "inspect the recorded owner identity before retrying shutdown")
	}
	if !report.Complete() {
		return Result{RawLines: report.Lines(), Components: []ComponentOutcome{{
			Component: "supervision-owner", Outcome: "failed", Detail: "one or more recorded supervision identities were not stopped",
			Remedy: "inspect the recorded identities before retrying shutdown",
		}}, Outcome: "failed", Failed: "supervision-owner", Remedy: "inspect the recorded identities before retrying shutdown"}
	}
	return Result{RawLines: report.Lines(), Outcome: "stopped"}
}

// SchedulerEntry prints the optional operator-owned recovery entry. It has no
// filesystem side effects and the command it prints carries no session or
// lease authority.
func SchedulerEntry(options Options) string {
	return fmt.Sprintf("0 * * * * cd %s && %s up --metasystem-root %s --repo %s --recover-only --if-down",
		shellquote.Quote(options.Scope), shellquote.Quote(options.Binary), shellquote.Quote(installationRoot(options)), shellquote.Quote(options.Scope))
}
