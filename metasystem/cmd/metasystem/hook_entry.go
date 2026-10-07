package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/adapter"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/cachedomain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hooks"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/narratordigest"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/receipt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/report"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/up"
)

// runHookEntry is the `hook RUNTIME EVENT` entry the runtime settings run
// directly from the installation directory (plans/designs/verbs-object-action.md
// 3.3): the runtime lifecycle hook body. `hook --accepts` answers whether this
// engine serves the entry at all; `system setup` asks it before connecting a
// checkout's hooks.
func runHookEntry(args []string, stdout, stderr io.Writer) int {
	return runHookEntryWithInputs(args, stdout, stderr, defaultHookEntryInputs)
}

type hookEntryInputs struct {
	invocation hooks.Invocation
	operations hooks.Ops
}

func runHookEntryWithInputs(args []string, stdout, stderr io.Writer, build func(string, string, chan os.Signal, io.Writer) hookEntryInputs) int {
	if len(args) == 1 && args[0] == "--accepts" {
		return 0
	}
	if len(args) > 0 && strings.HasPrefix(args[0], "-") {
		return refuseUnknownOption(stdout, stderr, "hook", args[0], "it takes RUNTIME EVENT, or --accepts")
	}
	runtime, event := "", ""
	if len(args) > 0 {
		runtime = args[0]
	}
	if len(args) > 1 {
		event = args[1]
	}
	// The settings command entered the installation first.
	installation, _ := os.Getwd()
	var signals chan os.Signal
	if event == "start" {
		signals = make(chan os.Signal, 4)
		signal.Notify(signals, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)
		defer signal.Stop(signals)
	}
	inputs := build(installation, event, signals, stderr)
	inputs.invocation.Runtime, inputs.invocation.Event = runtime, event
	inputs.invocation.Stdout, inputs.invocation.Stderr = stdout, stderr
	return hooks.RunRuntimeHook(inputs.invocation, inputs.operations)
}

func defaultHookEntryInputs(installation, event string, signals chan os.Signal, stderr io.Writer) hookEntryInputs {
	origin := time.Now()
	owners := hookOwners{diagnostics: io.Discard}
	if event == "start" {
		owners.diagnostics = stderr
	}
	return hookEntryInputs{invocation: hooks.Invocation{
		Event:  event,
		Stdin:  os.Stdin,
		Lookup: os.LookupEnv, Pid: os.Getpid(), Ppid: os.Getppid(), Installation: installation,
		Now:       time.Now,
		Monotonic: func() time.Duration { return time.Since(origin) },
		After:     time.After, Sleep: time.Sleep, Signals: signals,
		Exec: syscall.Exec, Environ: os.Environ, StartWorker: hooks.LaunchEngineWorker,
		IsExecutable: func(path string) bool {
			info, err := os.Stat(path)
			return err == nil && !info.IsDir() && info.Mode()&0o111 != 0
		},
		Deadline: hooks.DeadlineDeps{
			BootClock: identity.BootClock, Prober: identity.KernelProber{}, ParentPid: identity.ParentPid,
			EventInterval: 20 * time.Millisecond, FixtureDeadline: stopDeadlineFixtureEvent,
		},
	}, operations: owners}
}

// stopDeadlineFixtureEventEnvironment names a file whose appearance a
// fixture uses as the Stop deadline; only a fake-runtime installation whose
// fixture owner is the exact live process may use it.
const stopDeadlineFixtureEventEnvironment = "METASYSTEM_STOP_DEADLINE_EVENT"

func stopDeadlineFixtureEvent(ctx context.Context, installation stateroot.Installation) (<-chan time.Time, error) {
	eventPath := os.Getenv(stopDeadlineFixtureEventEnvironment)
	if eventPath == "" {
		return nil, nil
	}
	if installation == "" || !fixtureauth.FixtureModeRoot(installation.Path()) {
		return nil, fmt.Errorf("fixture deadline event requires a fake-runtime root")
	}
	if err := validateFixtureOwner(os.Getenv(identity.FixtureOwnerEnv), identity.KernelProber{}); err != nil {
		return nil, fmt.Errorf("fixture deadline event requires its exact owner")
	}
	fired := make(chan time.Time, 1)
	go func() {
		ticker := time.NewTicker(20 * time.Millisecond)
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
	return fired, nil
}

// hookOwners binds the runtime hook's operations to their owners. Each
// method returns what the former verb printed and its status; start's
// diagnostics reach the hook's stderr as the verbs' did.
type hookOwners struct {
	diagnostics io.Writer
	// engineBuild makes the bootstrap build command, run in the
	// installation; nil is `go run ./cmd/devgate build`. Tests stand in for
	// the Go toolchain here.
	engineBuild   func() *exec.Cmd
	upInputs      upCommandInputs
	repositoryTop func(string) (string, error)
}

func (o hookOwners) diagnose(format string, args ...any) {
	fmt.Fprintf(o.diagnostics, format+"\n", args...)
}

func jsonLine(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(encoded) + "\n"
}

func (o hookOwners) RuntimeNames() (string, int) {
	var out strings.Builder
	for _, name := range runtimes.Names() {
		out.WriteString(name + "\n")
	}
	return out.String(), 0
}

func (o hookOwners) StateRoot(installation stateroot.Installation) (string, int) {
	root, err := stateroot.RootForCandidate(installation.Path())
	if err != nil {
		o.diagnose("%v", err)
		return "", 1
	}
	return root.Path() + "\n", 0
}

func (o hookOwners) HookDelegate(root string, metasystemRoot stateroot.Installation, job string, callerPid int) (string, int) {
	if root == "" || metasystemRoot == "" || callerPid < 1 {
		o.diagnose("hook delegate custody: a root, an installation and a caller are required")
		return "", 2
	}
	result, err := lease.HookDelegate(root, metasystemRoot.Path(), job, int64(callerPid))
	if err != nil {
		o.diagnose("%v", err)
		return "", 1
	}
	if !result.Delegate {
		return "", 3
	}
	return jsonLine(result), 0
}

func (o hookOwners) FindAncestor(installation stateroot.Installation, pid int, runtime string, allHosts bool) (string, int) {
	var ancestor census.AgentAncestor
	var err error
	if allHosts {
		ancestor, err = census.FindAncestorAllHosts(installation.Path(), int64(pid))
	} else {
		ancestor, err = census.FindAncestorProduction(installation.Path(), int64(pid), runtime)
	}
	if err != nil {
		o.diagnose("%v", err)
		return "", 1
	}
	return jsonLine(ancestor), 0
}

func (o hookOwners) Classify(root string, metasystemRoot stateroot.Installation, callerPid int) (string, int) {
	installation := metasystemRoot.Path()
	if installation == "" {
		installation = root
	}
	out, err := lease.ClassifyVerbAt(root, installation, int64(callerPid))
	if err != nil {
		o.diagnose("%v", err)
		return "", 1
	}
	return jsonLine(out), 0
}

func (o hookOwners) StartContext(runtime string) (string, int) {
	declaration, ok := runtimes.Lookup(runtime)
	if !ok {
		o.diagnose("unknown runtime: %s", runtime)
		return "", 1
	}
	if declaration.StartContextField == "" {
		o.diagnose("no start context declared for %s", declaration.Name)
		return "", 1
	}
	return fmt.Sprintf("field=%s event=%s bytes=%d sources=%s\n", declaration.StartContextField,
		declaration.StartContextEventName, declaration.StartContextBytes,
		strings.Join(declaration.StartContextSources, ",")), 0
}

// PeerSeat is the checkout's enrolled nickname: the seat its peer messages
// are addressed to (batch-lane design D14-r3, R26).
func (o hookOwners) PeerSeat(repo string) (string, int) {
	return peerSeatLine(goal.ResolveMachine, repo)
}

// PeerClaims is the live claims of the checkout's accepted ledger, the
// ownership a goal's peer message is delivered by; never a card.
func (o hookOwners) PeerClaims(repo string) (string, int) {
	return peerClaimsLine(func() (board.Ownership, error) { return goal.PeerOwnership(repo) })
}

// peerSeatLine is the nickname on one line, status 1 when none is enrolled.
func peerSeatLine(resolve func(string) (string, error), repo string) (string, int) {
	machine, err := resolve(repo)
	if err != nil || strings.TrimSpace(machine) == "" {
		return "", 1
	}
	return strings.TrimSpace(machine) + "\n", 0
}

// peerClaimsLine is the ownership as one JSON object, or the reason the
// ledger is unreadable with status 1.
func peerClaimsLine(read func() (board.Ownership, error)) (string, int) {
	ownership, err := read()
	if err != nil {
		return err.Error() + "\n", 1
	}
	if ownership.Live == nil {
		ownership.Live = map[string]string{}
	}
	if ownership.Concluded == nil {
		ownership.Concluded = map[string]string{}
	}
	return jsonLine(ownership), 0
}

func (o hookOwners) StewardPending(repo string) (string, int) {
	pending, err := steward.PendingNotifications(repo)
	if err != nil {
		o.diagnose("steward pending: %v", err)
		return "", 1
	}
	if len(pending) == 0 {
		return "", 0
	}
	return fmt.Sprintf("%d undelivered; newest: %s\n", len(pending), pending[len(pending)-1].Message), 0
}

func (o hookOwners) BrainBoot(_ context.Context, root, repo string, bytes, deadlineMS int) (string, string, int) {
	if bytes < minimumBrainContextBytes {
		return "", fmt.Sprintf("refused: the context bound %d is below the minimum 2048\n", bytes), 2
	}
	if deadlineMS < 1 {
		return "", "brain boot: --deadline-ms must be positive\n", 2
	}
	output, err := composeBrainBootMode(root, repo, bytes, deadlineMS, true)
	if err != nil {
		return "", "brain boot: " + err.Error() + "\n", 1
	}
	if !output.Declared {
		return jsonLine(map[string]any{"declared": false}), "", 0
	}
	return jsonLine(output), "", 0
}

func (o hookOwners) BrainStartDelivered(root, repo, declarationSHA, digestCursor, digestPrefix string) int {
	cursor := int64(-1)
	if digestCursor != "" {
		parsed, err := strconv.ParseInt(digestCursor, 10, 64)
		if err != nil {
			return 2
		}
		cursor = parsed
	}
	if err := brainStartDelivered(root, repo, declarationSHA, cursor, digestPrefix, goal.ExistingLedgerIdentity, stateroot.ResolveLayout); err != nil {
		return 1
	}
	return 0
}

func (o hookOwners) Up(request hooks.UpRequest, stdout, stderr io.Writer) int {
	var pid, start int64
	var err error
	if request.Pid != "" {
		if pid, err = strconv.ParseInt(request.Pid, 10, 64); err != nil {
			fmt.Fprintln(stderr, "up: flags are invalid")
			return 2
		}
	}
	if request.StartTime != "" {
		if start, err = strconv.ParseInt(request.StartTime, 10, 64); err != nil {
			fmt.Fprintln(stderr, "up: flags are invalid")
			return 2
		}
	}
	root, err := upMetasystemRoot(request.MetasystemRoot.Path())
	if err != nil {
		fmt.Fprintln(stderr, "up:", err)
		return 2
	}
	inputs := o.upInputs.defaults()
	top := o.repositoryTop
	if top == nil {
		top = stateroot.RepositoryTop
	}
	scope, err := upRepositoryScopeWith(request.Repo, top)
	if err != nil {
		fmt.Fprintln(stderr, "up:", err)
		return 2
	}
	binary, err := inputs.executable()
	if err == nil {
		binary, err = canonicalPath(binary)
	}
	if err != nil {
		fmt.Fprintln(stderr, "up:", err)
		return 1
	}
	scale := upWaitScale()
	if scale == 0 {
		fmt.Fprintf(stderr, "the test time scale %s is not a whole number above zero\n", "METASYSTEM_FIXTURE_CAP_SCALE_MILLI")
		return 2
	}
	// Supervision control, enrollment and accounting belong to the
	// authenticated installation; scope stays the containing repository so
	// census still observes application processes and sources. The caller
	// is the hook process, as it was when `up` ran as its child.
	options := up.Options{
		Root: root, MetasystemRoot: root, Scope: scope, Binary: binary, Session: request.Session, Pid: pid,
		StartTime: start, Tag: request.Tag, Runtime: request.Runtime,
		RuntimeSession: request.RuntimeSession, NoRuntimeSession: request.NoRuntimeSession, StartSource: request.StartSource,
		RecoverOnly: request.RecoverOnly, IfDown: request.IfDown, WaitScaleMilli: scale,
		CallerPid:             int64(request.CallerPid),
		RestampStopCapability: restampStopCapabilityWith(inputs.dependencies, inputs.clock, false),
	}
	var result up.Result
	if request.Retire {
		result = up.Retire(options)
	} else {
		result = inputs.run(options)
	}
	for _, line := range result.Lines() {
		fmt.Fprintln(stdout, line)
	}
	return result.ExitCode()
}

func (o hookOwners) SessionStart(root, session string, stdout, stderr io.Writer) int {
	return sessionStartRecovery(root, session, stdout, stderr)
}

func (o hookOwners) SessionEnd(root, session string) int {
	if strings.TrimSpace(session) == "" {
		return 2
	}
	if err := (&goal.Store{Root: root}).EndSessionStop(session); err != nil {
		return 1
	}
	return 0
}

func (o hookOwners) HookAttempt(repo string, pid int, turnKey string) (string, int) {
	if repo == "" || pid < 1 || turnKey == "" {
		return "", 2
	}
	line, err := beginHookAttempt(repo, int64(pid), turnKey)
	if err != nil {
		return "", 1
	}
	return line + "\n", 0
}

func (o hookOwners) HookComplete(request hooks.HookCompletion) int {
	return completeHookAttempt(request, io.Discard)
}

func (o hookOwners) HookExpire(repo string, elapsedSec int64) int {
	if repo == "" || elapsedSec < 0 {
		return 2
	}
	if _, err := steward.ExpireHookAttempt(repo, elapsedSec, time.Now()); err != nil {
		return 1
	}
	return 0
}

func (o hookOwners) HealthPreview(repo string, metasystemRoot stateroot.Installation) (string, int) {
	var out bytes.Buffer
	status := writeHookHealthPreview(repo, metasystemRoot.Path(), true, &out, io.Discard)
	return out.String(), status
}

func (o hookOwners) DigestPending(repo string) (string, int) {
	pending, err := narratordigest.Pending(repo)
	if err != nil {
		return fmt.Sprintf("steward digest-pending: %v\n", err), 1
	}
	return jsonLine(pending), 0
}

func (o hookOwners) DigestAdvance(repo, cursor, prefix string) int {
	parsed, err := strconv.ParseInt(cursor, 10, 64)
	if err != nil || parsed < 0 || prefix == "" {
		return 2
	}
	if err := narratordigest.Advance(repo, parsed, prefix); err != nil {
		return 1
	}
	return 0
}

func (o hookOwners) ReceiptCheck(root string) (string, string, int) {
	options := receipt.Options{Root: root, Skills: "none", Verify: "skipped", Corrections: "0", StopLoss: "no"}
	receiptRoot, err := stateroot.StateRoot(stateroot.Receipts)
	if err != nil {
		return "", "receipt: " + err.Error() + "\n", 1
	}
	options.File = receiptRoot.Path("receipts.log")
	result := receipt.Check(options)
	var stdout, stderr strings.Builder
	for _, line := range result.Out {
		stdout.WriteString(line + "\n")
	}
	for _, line := range result.Err {
		stderr.WriteString(line + "\n")
	}
	return stdout.String(), stderr.String(), result.Code
}

func (o hookOwners) ProtocolGrowth(root, mainID string) (string, int) {
	out, err := lease.ProtocolGrowth(root, mainID)
	if err != nil {
		return "", 1
	}
	return jsonLine(out), 0
}

func (o hookOwners) ProtocolAdvance(root, mainID string, callerPid int, counts string) int {
	if err := lease.ProtocolAdvance(root, mainID, int64(callerPid), counts); err != nil {
		return 1
	}
	return 0
}

func (o hookOwners) RenewLease(root string, callerPid int) int {
	if _, err := lease.Renew(root, int64(callerPid)); err != nil {
		return 1
	}
	return 0
}

func (o hookOwners) WatchdogReport(repo string) (string, int) {
	lines := supervise.WatchdogReport(repo, time.Now())
	if len(lines) == 0 {
		return "", 0
	}
	return strings.Join(lines, "\n") + "\n", 0
}

func (o hookOwners) TurnVerdict(request hooks.TurnVerdictRequest) (string, string, int) {
	var stdout, stderr bytes.Buffer
	status := reportTurnVerdict(request, &stdout, &stderr, nil, nil)
	return stdout.String(), stderr.String(), status
}

func (o hookOwners) StopBlock(request hooks.StopBlockRequest) (string, int) {
	block, status, err := renderStopBlock(request, time.Now())
	if err != nil {
		o.diagnose("%v", err)
	}
	if status != 0 {
		return "", status
	}
	return jsonLine(block), 0
}

func (o hookOwners) StopInput(request hooks.StopInputRequest, stderr io.Writer) int {
	claimEpoch, err := strconv.ParseInt(request.ClaimEpoch, 10, 64)
	if err != nil && request.ClaimEpoch != "" {
		fmt.Fprintln(stderr, "the stop report input is not valid: its checkout claim is not a whole number")
		return 2
	}
	if request.Root == "" || request.Runtime == "" || request.Session == "" || request.Attempt == "" ||
		request.OutputFile == "" || request.Advisor == (request.VerdictFile != "") {
		fmt.Fprintln(stderr, "the stop report input lacks one of root, runtime, session, attempt, output or its verdict")
		return 2
	}
	err = report.ComposeStopPresentationInput(report.StopPresentationCollection{
		Root: request.Root, Runtime: request.Runtime, Session: request.Session, Attempt: request.Attempt, MainID: request.MainID,
		Machine: request.Machine, Lineage: request.Lineage, ClaimEpoch: claimEpoch, Advisor: request.Advisor,
		VerdictFile: request.VerdictFile, FactsFile: request.FactsFile, CompletionFile: request.CompletionFile, HealthFile: request.HealthFile,
		DigestFile: request.DigestFile, DigestCursorPrefix: request.DigestCursorPrefix,
		ReceiptFile: request.ReceiptFile, ReceiptStderrFile: request.ReceiptStderrFile, ReceiptExit: request.ReceiptExit,
		ArmingFile: request.ArmingFile, ArmingStderrFile: request.ArmingStderrFile, ArmingExit: request.ArmingExit,
		NoticeFile: request.NoticeFile, FailureFile: request.FailureFile, OutputFile: request.OutputFile,
	}, time.Now())
	return stopPresentationStatus("report stop-input", err, stderr)
}

func stopPresentationStatus(name string, err error, stderr io.Writer) int {
	if err == nil {
		return 0
	}
	fmt.Fprintln(stderr, name+":", err)
	var validation report.StopPresentationValidationError
	if errors.As(err, &validation) {
		return 2
	}
	return 1
}

func (o hookOwners) StopPresent(root, inputFile, outputFile string, stderr io.Writer) int {
	_, err := report.PresentStop(root, inputFile, outputFile, time.Now())
	return stopPresentationStatus("report stop-present", err, stderr)
}

func (o hookOwners) StopOutput(runtime, inputFile, outputFile string, stderr io.Writer) int {
	if err := adapter.MapStopOutput(runtime, inputFile, outputFile); err != nil {
		fmt.Fprintln(stderr, "adapter stop-output:", err)
		return 1
	}
	return 0
}

func (o hookOwners) EvidenceGC(installation stateroot.Installation, output io.Writer) int {
	return evidenceGC(installation.Path(), "", evidenceGCDefaultGrace(), output, output)
}

func (o hookOwners) Slug(value string) string { return lease.Slug(value) }

func (o hookOwners) TokenHex(size int) (string, error) {
	buffer := make([]byte, size)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}

// hookGitSteering are the inherited Git variables that must not redirect
// the hook's checkout identification.
var hookGitSteering = []string{
	"GIT_DIR", "GIT_WORK_TREE", "GIT_COMMON_DIR", "GIT_INDEX_FILE",
	"GIT_CEILING_DIRECTORIES", "GIT_DISCOVERY_ACROSS_FILESYSTEM",
	"GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES",
	"GIT_CONFIG", "GIT_CONFIG_PARAMETERS", "GIT_CONFIG_COUNT",
	"GIT_CONFIG_GLOBAL", "GIT_CONFIG_SYSTEM", "GIT_CONFIG_NOSYSTEM",
	"GIT_GRAFT_FILE", "GIT_SHALLOW_FILE", "GIT_REPLACE_REF_BASE",
	"GIT_IMPLICIT_WORK_TREE", "GIT_NO_REPLACE_OBJECTS", "GIT_PREFIX",
}

func (o hookOwners) Git(args ...string) (string, error) {
	command := exec.Command("git", args...)
	environment := os.Environ()
	command.Env = environment[:0:0]
	for _, entry := range environment {
		name, _, _ := strings.Cut(entry, "=")
		steering := false
		for _, blocked := range hookGitSteering {
			if name == blocked {
				steering = true
			}
		}
		if !steering {
			command.Env = append(command.Env, entry)
		}
	}
	out, err := command.Output()
	return string(out), err
}

// EngineBehind is the hook side of a generation cutover: the enrolled
// engine no longer owns the ENGINE projection at the checkout's HEAD, the
// checkout is clean in engine inputs, and no proof attempt is live.
func (o hookOwners) EngineBehind(enrolled stateroot.Installation, repo string) (bool, error) {
	installation := enrolled.Path()
	pinned, err := steward.OpenEnrolledBinary(installation)
	if err != nil {
		return false, nil
	}
	defer pinned.Close()
	head, err := o.Git("-C", installation, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		return false, err
	}
	verifyErr := pinned.VerifySourceAtDestinationWithClock(steward.SystemRearmClock(), installation, strings.TrimSpace(head))
	if verifyErr == nil {
		return false, nil
	}
	if !errors.Is(verifyErr, steward.ErrNotOwned) {
		return false, verifyErr
	}
	prefix, err := o.Git("-C", installation, "rev-parse", "--show-prefix")
	if err != nil {
		return false, err
	}
	seconds := steward.RearmResolveSeconds(installation)
	dirty, err := testrun.DirtyEnginePaths(context.Background(), steward.SystemRearmClock(), seconds, installation, strings.TrimSuffix(strings.TrimSpace(prefix), "/"))
	if err != nil || len(dirty) > 0 {
		return false, err
	}
	attempts, err := proofrun.ReadAttempts(installation)
	if err != nil {
		return false, err
	}
	for _, attempt := range attempts {
		if attempt.Terminal == nil {
			return false, nil
		}
	}
	return true, nil
}

// UnmigratableRetainedPlans is the seam for the retained-plan scan of the
// cutover (verbs-object-action section 7, unit U1c). Until that scan lands
// no retained plan holds the rearm.
func (o hookOwners) UnmigratableRetainedPlans(string) ([]string, error) { return nil, nil }

// StartEngineRebuild starts the installation's engine build detached from
// the hook: in its own session, logging to the bootstrap log, holding the
// proof mutation lock (handed to the build as an inherited descriptor, so no
// attempt is admitted while the engine changes) and named by the bootstrap
// fence. It returns once the build has started through the Go bootstrap,
// `go run ./cmd/devgate build`; an installation without cmd/devgate cannot
// rebuild and is refused. A live fence means a rebuild already runs; a held
// proof lock refuses without waiting.
func (o hookOwners) StartEngineRebuild(installation stateroot.Installation) error {
	if hooks.BootstrapFenceHeld(installation, hookProcessAlive) {
		return nil
	}
	if info, err := os.Stat(installation.Path("cmd", "devgate")); err != nil || !info.IsDir() {
		return errors.New("the engine cannot be rebuilt here: this installation has no cmd/devgate\nfrom a complete metasystem tree, run: go run ./cmd/devgate build")
	}
	lock, err := proofrun.TryAcquireMutation(installation.Path())
	if err != nil {
		return fmt.Errorf("another engine build or check run holds the installation: %w", err)
	}
	defer func() { _ = lock.Release() }()
	claimed, err := hooks.ClaimBootstrapFence(installation, os.Getpid(), hookProcessAlive)
	if err != nil || !claimed {
		return err
	}
	logFile, err := os.OpenFile(installation.Path(filepath.FromSlash(hooks.BootstrapLogPath)), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer logFile.Close()
	argv := testrun.DevgateBootstrapBuildArgv()
	command := exec.Command(argv[0], argv[1:]...)
	if o.engineBuild != nil {
		command = o.engineBuild()
	}
	command.Dir = installation.Path()
	// The engine cache is set explicitly from the authenticated domain,
	// never guessed by a nested go.
	environment, err := cachedomain.Carry(os.Environ(), installation.Path())
	if err != nil {
		fmt.Fprintf(logFile, "hook bootstrap: %v\n", err)
		return err
	}
	command.Env = environment
	command.Stdout, command.Stderr = logFile, logFile
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	locked := lock.Handoff()
	defer locked.Close()
	command.ExtraFiles = []*os.File{locked}
	fmt.Fprintf(logFile, "hook bootstrap: SessionStart starts the engine build (%s)\n", strings.Join(command.Args, " "))
	if err := command.Start(); err != nil {
		return err
	}
	pointErr := hooks.PointBootstrapFence(installation, command.Process.Pid)
	_ = command.Process.Release()
	return pointErr
}

// hookProcessAlive reports whether pid names a process: one this user
// cannot signal still exists.
func hookProcessAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}
