package delegation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/adapter"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/authority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatchproc"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/events"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gaterun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/janitor"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/wallclock"
)

// OwnerConfig locates the owners' state. Root is the metasystem installation
// root: the lease, the job records and the runtime registry are all
// resolved beneath it.
type OwnerConfig struct {
	Root string
	// Engine is the engine binary the delegate-supervisor entry and the
	// guard member wrapper run; empty is Root/bin/metasystem.
	Engine string
	// Now is the clock goal binding judges against; nil means the wall
	// clock. A caller that resolved fixture clock authority passes it here.
	Now func() time.Time
	// Host supplies the operations whose owners live in the engine's
	// command layer.
	Host HostOps
}

// NewOwnerPorts wires every port to its real owner.
func NewOwnerPorts(config OwnerConfig) (Ports, error) {
	if config.Root == "" {
		return Ports{}, fmt.Errorf("delegation owner ports need the metasystem root")
	}
	if config.Host == nil {
		return Ports{}, fmt.Errorf("delegation owner ports need the host operations")
	}
	root, err := filepath.Abs(config.Root)
	if err != nil {
		return Ports{}, err
	}
	engine := config.Engine
	if engine == "" {
		engine = filepath.Join(root, "bin", "metasystem")
	}
	now := config.Now
	if now == nil {
		now = time.Now
	}
	emitter := &events.Emitter{Component: "dispatch", Pid: int64(os.Getpid())}
	if started, ok := lease.StartedAt(emitter.Pid, nil); ok {
		emitter.PidStartedAt = started
	}
	eventRoot := root
	if harness := os.Getenv("METASYSTEM_HARNESS_ROOT"); harness != "" {
		eventRoot = harness
	}
	return Ports{
		Lease:   ownerLease{root: root},
		Steward: ownerSteward{root: root},
		Adapter: ownerAdapter{root: root, engine: engine},
		Goal:    ownerGoal{root: root, now: now},
		Records: ownerRecords{root: root},
		Events:  ownerEvents{root: eventRoot, emitter: emitter},
		Process: ownerProcess{root: root},
		Git:     ownerGit{},
		Clock:   wallclock.System(),
		Host:    config.Host,
		Guard:   ownerGuard{},
	}, nil
}

type ownerLease struct{ root string }

func (o ownerLease) Classify(inv Invocation) (lease.ClassifyResult, error) {
	return lease.ClassifyVerb(o.root, inv.CallerPid)
}

func (o ownerLease) RequireHolder(inv Invocation, expectedEpoch *int64) (lease.HolderView, error) {
	return lease.RequireHolder(o.root, inv.CallerPid, expectedEpoch)
}

func (o ownerLease) Renew(inv Invocation) (lease.RenewResult, error) {
	return lease.Renew(o.root, inv.CallerPid)
}

// Held runs fn under the lease lock as run-held gates its child, except for
// a STEWARD caller: a dead worker holds no lease, so the steward's authority
// is enforced per write by the internal entries' checks and fn runs
// ungated (dispatch.sh's lease_run_held).
func (o ownerLease) Held(inv Invocation, expectedEpoch *int64, fn func() error) error {
	if inv.CallerClass == lease.ClassSteward {
		return fn()
	}
	return lease.Held(o.root, inv.CallerPid, expectedEpoch, fn)
}

// Authorize is internal_authority: classify the supplied caller, then judge
// the classification's wire form against the matrix, exactly the JSON the
// shell handed job authority-check.
func (o ownerLease) Authorize(inv Invocation, mode AuthorityMode, job string) error {
	if !authority.ValidMode(string(mode)) {
		return fmt.Errorf("unknown control-plane mode %q", mode)
	}
	classification, err := lease.ClassifyVerb(o.root, inv.CallerPid)
	if err != nil {
		return fmt.Errorf("control-plane write refused: caller classification failed: %w", err)
	}
	encoded, err := json.Marshal(classification)
	if err != nil {
		return err
	}
	var caller map[string]any
	if err := json.Unmarshal(encoded, &caller); err != nil {
		return err
	}
	return authority.Authorize(string(mode), caller, job)
}

type ownerSteward struct{ root string }

// AuthorizeDispatch admits exactly one caller class, as the verb does.
func (o ownerSteward) AuthorizeDispatch(inv Invocation, intent string) (steward.DispatchAuthorization, error) {
	classification, err := lease.Classify(o.root, inv.CallerPid)
	if err != nil {
		return steward.DispatchAuthorization{}, err
	}
	if classification.Class != lease.ClassSteward {
		return steward.DispatchAuthorization{}, fmt.Errorf("caller is %s, not the steward; the continuation mode admits exactly one caller", classification.Class)
	}
	return steward.AuthorizeDispatch(o.root, intent)
}

// ownerAdapter drives the runtime adapter through the engine's
// delegate-supervisor entry (internal/adapter/supervisor, U6a): `ENGINE
// delegate-supervisor RUNTIME VERB --root ROOT [flags]`. The long-lived
// round (dispatch, follow-up) is that entry as a detached process of its
// own; its small reads (signature, config-identity, probe, output-stream)
// and the cancel are the same entry run to completion. The retired
// dispatch.sh made these same calls.
type ownerAdapter struct{ root, engine string }

func (o ownerAdapter) argv(runtime, verb string, flags ...string) ([]string, error) {
	if runtime == "" || strings.ContainsAny(runtime, `/\`) || runtime == "." || runtime == ".." || strings.HasPrefix(runtime, "-") {
		return nil, fmt.Errorf("invalid runtime name %q", runtime)
	}
	args := runtimes.SupervisorArgs(runtime, verb, append([]string{"--root", o.root}, flags...)...)
	return append([]string{o.engine}, args...), nil
}

// Installed reports whether the installation resolves the runtime to an
// adapter: the entry's signature read succeeds (dispatch.sh's
// `delegate-supervisor RUNTIME signature` check).
func (o ownerAdapter) Installed(runtime string) bool {
	_, err := o.run(context.Background(), runtime, "signature")
	return err == nil
}

func (o ownerAdapter) run(ctx context.Context, runtime, verb string, flags ...string) (string, error) {
	argv, err := o.argv(runtime, verb, flags...)
	if err != nil {
		return "", err
	}
	command := exec.CommandContext(ctx, argv[0], argv[1:]...)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return "", fmt.Errorf("%s adapter %s failed: %w: %s", runtime, strings.Join(append([]string{verb}, flags...), " "), err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimRight(stdout.String(), "\n"), nil
}

func (o ownerAdapter) ConfigIdentity(ctx context.Context, runtime string) (string, error) {
	return o.run(ctx, runtime, "config-identity")
}

func (o ownerAdapter) Probe(ctx context.Context, runtime string) error {
	_, err := o.run(ctx, runtime, "probe")
	return err
}

func (o ownerAdapter) OutputStream(ctx context.Context, runtime, roundDir string) (string, error) {
	return o.run(ctx, runtime, "output-stream", "--round-dir", roundDir)
}

// Cancel runs the runtime's cancel, which comes back through the delegate
// entry's __cancel-owned callback under the caller's own authority.
func (o ownerAdapter) Cancel(ctx context.Context, runtime, job string) error {
	argv, err := o.argv(runtime, "cancel", "--job", job)
	if err != nil {
		return err
	}
	command := exec.CommandContext(ctx, argv[0], argv[1:]...)
	var output bytes.Buffer
	command.Stdout, command.Stderr = &output, &output
	if err := command.Run(); err != nil {
		return &AdapterCancelError{Code: exitCodeOf(err), Output: strings.TrimSpace(output.String())}
	}
	return nil
}

// Launch is launch_adapter's spawn step: the delegate-supervisor entry runs
// in its own session with the job's commit identity and stdio detached, the
// persistent owner of the round. With an execution guard the session leader
// is the guard member wrapper (the delegate entry's __run-member),
// registered while this launcher is in its ancestry, which runs the
// supervisor as its child and releases its membership when it ends. The
// ownership publication and start-gate release that follow the spawn stay
// with the lifecycle.
func (o ownerAdapter) Launch(ctx context.Context, request AdapterLaunch) (int64, error) {
	if request.Verb != runtimes.SupervisorDispatch && request.Verb != runtimes.SupervisorFollowUp {
		return 0, fmt.Errorf("adapter launch verb must be dispatch or follow-up, not %q", request.Verb)
	}
	if request.Job == "" || request.StartGate == "" || request.InstanceTag == "" || request.LaunchCapability == "" {
		return 0, fmt.Errorf("adapter launch needs a job, start gate, instance tag and launch capability")
	}
	if request.ExecutionGuard != nil && (request.ExecutionGuard.Root == "" || request.ExecutionGuard.Owner == "") {
		return 0, fmt.Errorf("an execution guard needs both its root and its owner")
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	argv, err := o.argv(request.Runtime, request.Verb, "--job", request.Job, "--start-gate", request.StartGate,
		runtimes.SupervisorTagFlag, request.InstanceTag, "--launch-capability", request.LaunchCapability)
	if err != nil {
		return 0, err
	}
	launch := gaterun.DetachedLaunch{
		Dir: o.root,
		Env: []string{"GIT_AUTHOR_NAME=" + request.Job, "GIT_AUTHOR_EMAIL=" + request.Job + "@metasystem.invalid"},
	}
	if guard := request.ExecutionGuard; guard != nil {
		argv = append([]string{o.engine, "internal", "delegate", RunMemberCallback, "--root", guard.Root, "--"}, argv...)
		launch.GuardRoot, launch.GuardOwner = guard.Root, guard.Owner
	}
	launch.Argv = argv
	return gaterun.LaunchDetached(launch)
}

func (o ownerAdapter) ResultPatch(outputPath, failure, phase, usagePath string) error {
	return adapter.WriteResultPatch(outputPath, failure, phase, usagePath)
}

type ownerGoal struct {
	root string
	now  func() time.Time
}

func (o ownerGoal) LedgerIdentity() string { return goal.ExistingLedgerIdentity(o.root) }

func (o ownerGoal) BreachStop(goalID string, revision uint64, now time.Time, orderedBy string) (goal.StopBatch, error) {
	if orderedBy != "" {
		return dispatch.EnsureBreachStopOrderedBy(o.root, goalID, revision, now, orderedBy)
	}
	return dispatch.EnsureBreachStop(o.root, goalID, revision, now)
}

func (o ownerGoal) Binding(goalID string) (GoalBinding, error) {
	clock, _, err := fixtureauth.GoalClock(o.root, func() time.Time { return o.now().UTC() })
	if err != nil {
		return GoalBinding{}, err
	}
	binding, err := dispatch.ResolveGoalBinding(o.root, goalID, clock())
	if err != nil {
		return GoalBinding{}, err
	}
	return GoalBinding{
		GoalID: binding.GoalID, Revision: binding.Revision, Tier: binding.Tier,
		GateWidth: binding.GateWidth, Machine: binding.Machine, Lineage: binding.Lineage,
		Capability: binding.Capability, Fenced: binding.Fence != nil,
	}, nil
}

type ownerRecords struct{ root string }

func (o ownerRecords) Create(job, sourcePath string) error {
	return dispatch.RecordCreate(o.root, job, sourcePath)
}

func (o ownerRecords) Setup(job, sourcePath string) error {
	return dispatch.RecordSetup(o.root, job, sourcePath)
}

func (o ownerRecords) CAS(job, expect, target, patchPath string) (string, error) {
	return dispatch.RecordCAS(o.root, job, expect, target, patchPath)
}

func (o ownerRecords) Read(job string) (map[string]any, error) {
	if job == "" || strings.ContainsAny(job, `/\`) || job == "." || job == ".." {
		return nil, fmt.Errorf("invalid job id %q", job)
	}
	return dispatch.ReadRecordObject(filepath.Join(o.root, "artifacts", "agents", "jobs", job+".json"))
}

type ownerEvents struct {
	root    string
	emitter *events.Emitter
}

func (o ownerEvents) Emit(event, summary string, fields map[string]string) {
	o.emitter.Emit(o.root, event, summary, fields)
}

// ownerProcess is the kernel through the engine's identity owners.
type ownerProcess struct{ root string }

func (p ownerProcess) ClaimProcesses() (ClaimProcesses, error) {
	reader, err := dispatchproc.StartReader(p.root)
	if err != nil {
		return ClaimProcesses{}, err
	}
	return ClaimProcesses{Reader: reader, Scanner: dispatchproc.TaggedProcessScanner{Root: p.root}, Verifier: dispatchproc.ClaimProcessVerifier{Root: p.root}}, nil
}

func (ownerProcess) TagState(pid int64, tag string) string {
	return identity.TagState(identity.KernelProber{}, pid, tag)
}

func (ownerProcess) Exists(pid int64) bool {
	if pid < 1 {
		return false
	}
	err := unix.Kill(int(pid), 0)
	return err == nil || errors.Is(err, unix.EPERM)
}

func (ownerProcess) GroupExists(pgid int64) bool {
	if pgid < 1 {
		return false
	}
	err := unix.Kill(int(-pgid), 0)
	return err == nil || errors.Is(err, unix.EPERM)
}

// GroupOwned is `proc group-owned --root --record`: a verified positioned
// tag, or a fake runtime's recorded trusted-launcher proof the root's
// fixture authorizes.
func (o ownerProcess) GroupOwned(recordPath string, pgid int64, tag string) bool {
	if pgid < 2 || tag == "" {
		return false
	}
	switch janitor.GroupOwnership(pgid, tag) {
	case janitor.GroupOwned:
		return true
	case janitor.GroupNotOwned:
		return false
	}
	if err := unix.Kill(int(-pgid), 0); err != nil && !errors.Is(err, unix.EPERM) {
		return false
	}
	authorization, err := fixtureauth.New(o.root)
	if err != nil {
		return false
	}
	matches, err := dispatch.RecordedGroupProofMatches(recordPath, pgid, tag, authorization.GroupOwnership())
	return err == nil && matches
}

func (ownerProcess) SignalGroup(pgid int64, sig Signal) error {
	if pgid < 2 {
		return fmt.Errorf("refusing to signal process group %d", pgid)
	}
	return unix.Kill(int(-pgid), unix.Signal(sig))
}

func (ownerProcess) CustodyGroups(record map[string]any) ([]int64, error) {
	return dispatch.CustodyGroupTargets(record, func(pid int64) (int64, error) {
		group, err := unix.Getpgid(int(pid))
		return int64(group), err
	})
}

func (ownerProcess) StartedAt(pid int64) (int64, error) {
	exact, state, err := identity.KernelProber{}.Probe(pid)
	if err != nil {
		return 0, err
	}
	if state != identity.Alive {
		return 0, fmt.Errorf("pid %d is not alive", pid)
	}
	return exact.StartedAt.Unix(), nil
}

// ownerGit runs the git binary.
type ownerGit struct{}

func (ownerGit) Run(ctx context.Context, dir string, args ...string) ([]byte, []byte, error) {
	command := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		err = &GitExitError{Code: exit.ExitCode(), Stderr: strings.TrimSpace(stderr.String())}
	}
	return stdout.Bytes(), stderr.Bytes(), err
}

type ownerGuard struct{}

func (ownerGuard) Acquire(root string, pid int64, owner string, wait, progress time.Duration, notes io.Writer) (gaterun.GuardResult, error) {
	return gaterun.AcquireExecutionGuard(root, pid, owner, wait, progress, notes)
}

func (ownerGuard) Release(root string, pid int64) error {
	return gaterun.ReleaseExecutionGuard(root, pid)
}

// RunMemberCallback is the delegate entry's guard member wrapper form.
const RunMemberCallback = "__run-member"

// AdapterCancelError is a runtime cancel that exited non-zero.
type AdapterCancelError struct {
	Code   int
	Output string
}

func (e *AdapterCancelError) Error() string {
	if e.Output != "" {
		return e.Output
	}
	return fmt.Sprintf("adapter cancel exited %d", e.Code)
}

func exitCodeOf(err error) int {
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode()
	}
	return 1
}
