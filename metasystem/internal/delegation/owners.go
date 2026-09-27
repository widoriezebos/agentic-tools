package delegation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/adapter"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/authority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/events"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gaterun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

// OwnerConfig locates the owners' state. Root is the metasystem installation
// root dispatch.sh calls $root: the lease, the job records and the adapter
// scripts are all resolved beneath it.
type OwnerConfig struct {
	Root string
	// Now is the clock goal binding judges against; nil means the wall
	// clock. A caller that resolved fixture clock authority passes it here.
	Now func() time.Time
}

// NewOwnerPorts wires every port to its real owner.
func NewOwnerPorts(config OwnerConfig) (Ports, error) {
	if config.Root == "" {
		return Ports{}, fmt.Errorf("delegation owner ports need the metasystem root")
	}
	root, err := filepath.Abs(config.Root)
	if err != nil {
		return Ports{}, err
	}
	now := config.Now
	if now == nil {
		now = time.Now
	}
	emitter := &events.Emitter{Component: "dispatch", Pid: int64(os.Getpid())}
	if started, ok := lease.StartedAt(emitter.Pid, nil); ok {
		emitter.PidStartedAt = started
	}
	return Ports{
		Lease:   ownerLease{root: root},
		Steward: ownerSteward{root: root},
		Adapter: ownerAdapter{root: root},
		Goal:    ownerGoal{root: root, now: now},
		Records: ownerRecords{root: root},
		Events:  ownerEvents{root: root, emitter: emitter},
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

func (o ownerLease) Held(inv Invocation, expectedEpoch *int64, fn func() error) error {
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

// ownerAdapter drives the runtime's adapter script, the adapter owner until
// U6a replaces it with the Go runtime registry.
type ownerAdapter struct{ root string }

func (o ownerAdapter) script(runtime string) (string, error) {
	if runtime == "" || strings.ContainsAny(runtime, `/\`) || runtime == "." || runtime == ".." {
		return "", fmt.Errorf("invalid runtime name %q", runtime)
	}
	path := filepath.Join(o.root, "scripts", "agents", "adapters", runtime+".sh")
	info, err := os.Stat(path)
	if err != nil || info.IsDir() || info.Mode()&0o111 == 0 {
		return "", fmt.Errorf("runtime adapter is not installed: %s", runtime)
	}
	return path, nil
}

func (o ownerAdapter) run(ctx context.Context, runtime string, args ...string) (string, error) {
	path, err := o.script(runtime)
	if err != nil {
		return "", err
	}
	command := exec.CommandContext(ctx, path, args...)
	command.Dir = o.root
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return "", fmt.Errorf("%s adapter %s failed: %w: %s", runtime, strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
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

func (o ownerAdapter) Cancel(ctx context.Context, runtime, job string) error {
	_, err := o.run(ctx, runtime, "cancel", "--job", job)
	return err
}

// Launch is launch_adapter's spawn step: the adapter runs in its own session
// with the job's commit identity, stdio detached, and joins the checkout
// execution guard when one is named. The ownership publication and start-gate
// release that follow the spawn stay with the lifecycle.
func (o ownerAdapter) Launch(ctx context.Context, request AdapterLaunch) (int64, error) {
	if request.Verb != "dispatch" && request.Verb != "follow-up" {
		return 0, fmt.Errorf("adapter launch verb must be dispatch or follow-up, not %q", request.Verb)
	}
	if request.Job == "" || request.StartGate == "" || request.InstanceTag == "" || request.LaunchCapability == "" {
		return 0, fmt.Errorf("adapter launch needs a job, start gate, instance tag and launch capability")
	}
	if request.ExecutionGuard != nil && (request.ExecutionGuard.Root == "" || request.ExecutionGuard.Owner == "") {
		return 0, fmt.Errorf("an execution guard needs both its root and its owner")
	}
	path, err := o.script(request.Runtime)
	if err != nil {
		return 0, err
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return 0, err
	}
	defer devNull.Close()
	command := exec.Command(path, request.Verb, "--job", request.Job, "--start-gate", request.StartGate,
		"--instance-tag", request.InstanceTag, "--launch-capability", request.LaunchCapability)
	command.Dir = o.root
	command.Stdin, command.Stdout, command.Stderr = devNull, devNull, devNull
	command.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME="+request.Job, "GIT_AUTHOR_EMAIL="+request.Job+"@metasystem.invalid")
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := command.Start(); err != nil {
		return 0, err
	}
	pid := int64(command.Process.Pid)
	if request.ExecutionGuard != nil {
		if err := gaterun.RegisterSpawnedExecutionGuardMember(request.ExecutionGuard.Root, pid, request.ExecutionGuard.Owner); err != nil {
			_ = command.Process.Kill()
			_, _ = command.Process.Wait()
			return 0, err
		}
	}
	// The supervisor is its own session and outlives this call; release the
	// handle without waiting, as launch-detached does.
	_ = command.Process.Release()
	return pid, nil
}

func (o ownerAdapter) ResultPatch(outputPath, failure, phase, usagePath string) error {
	return adapter.WriteResultPatch(outputPath, failure, phase, usagePath)
}

type ownerGoal struct {
	root string
	now  func() time.Time
}

func (o ownerGoal) Binding(goalID string) (GoalBinding, error) {
	binding, err := dispatch.ResolveGoalBinding(o.root, goalID, o.now())
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
