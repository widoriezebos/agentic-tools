package supervise

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/registry"
)

// ArmingOwner is the exact process identity recorded in the repository's
// supervision lock.
type ArmingOwner struct {
	Pid             int64  `json:"pid"`
	PidStartedAt    int64  `json:"pidStartedAt"`
	PidStartTicks   int64  `json:"pidStartTicks,omitempty"`
	BootID          string `json:"bootId,omitempty"`
	InstanceTag     string `json:"instanceTag"`
	AcquiredAt      string `json:"acquiredAt,omitempty"`
	FenceGeneration int64  `json:"fenceGeneration"`
}

// PublishedGeneration is the owner and configuration bound into state.json.
// It is the comparison surface used to decide whether a live owner belongs to
// the engine generation this invocation is arming.
type PublishedGeneration struct {
	Owner       ArmingOwner
	Fingerprint string
	IntervalSec int
	WatcherCap  int
	Generation  int64
}

// EnsureOptions are the complete inputs for one supervision arming attempt.
type EnsureOptions struct {
	Root            string
	MetasystemRoot  string
	Scope           string
	Binary          string
	Command         func(args ...string) (*exec.Cmd, error)
	Fingerprint     string
	IntervalSec     int
	WatcherCap      int64
	OnlyIfDown      bool
	WaitScaleMilli  int
	OwnerTagPrefix  string
	FenceGeneration int64
}

// EnsureResult describes whether arming joined, established, took over, or
// replaced the supervision generation.
type EnsureResult struct {
	Action     string
	Owner      ArmingOwner
	Generation int64
	Inspection ArmedInspection
}

const (
	ShutdownSignalNone = "none"
	ShutdownSignalTerm = "term"
	ShutdownSignalKill = "kill"

	ShutdownStopped     = "stopped"
	ShutdownAlreadyGone = "already-gone"
	ShutdownNotStopped  = "not-stopped"
)

// ComponentOutcome is the identity-bound result of one supervision stop.
type ComponentOutcome struct {
	Component  string
	Identity   identity.Ref
	Tag        string
	Generation int64
	Signal     string
	Result     string
	Reason     string
}

// ShutdownReport preserves one outcome for every supervision identity the
// caller inspected or signalled, in action order.
type ShutdownReport struct {
	Outcomes []ComponentOutcome
	Failures []ShutdownFailure
}

// ShutdownFailure is one processless bookkeeping failure discovered after
// supervision processes were acted on. It remains separate from a component
// outcome so a stopped process is not rewritten as a survivor.
type ShutdownFailure struct {
	Component string
	Path      string
	Reason    string
	Did       string
}

// Line renders one independently counted bookkeeping result.
func (f ShutdownFailure) Line() string {
	target := f.Component
	if f.Path != "" {
		target += " " + f.Path
	}
	return fmt.Sprintf("NOT STOPPED %s: %s; did: %s", target, f.Reason, f.Did)
}

// InventoryItem is one live supervision identity observed without mutation.
type InventoryItem struct {
	Component       string
	Identity        identity.Ref
	Tag             string
	Generation      int64
	FenceGeneration int64
}

// Complete reports whether every inventoried supervision identity was proven
// stopped or already gone.
func (r ShutdownReport) Complete() bool {
	if len(r.Failures) > 0 {
		return false
	}
	for _, outcome := range r.Outcomes {
		if outcome.Result == ShutdownNotStopped {
			return false
		}
	}
	return true
}

// Lines renders the supervision portion of the process-stop line grammar.
func (r ShutdownReport) Lines() []string {
	lines := make([]string, 0, len(r.Outcomes)+len(r.Failures))
	for _, outcome := range r.Outcomes {
		prefix := fmt.Sprintf("%s pid %d", outcome.Component, outcome.Identity.Pid)
		if outcome.Component == "supervision-owner" {
			prefix += fmt.Sprintf(" tag %s generation %d", outcome.Tag, outcome.Generation)
		}
		switch outcome.Result {
		case ShutdownAlreadyGone:
			if outcome.Reason == "" {
				lines = append(lines, prefix+": already gone")
			} else {
				lines = append(lines, fmt.Sprintf("%s: already gone (%s)", prefix, outcome.Reason))
			}
		case ShutdownStopped:
			if outcome.Signal == ShutdownSignalKill {
				if outcome.Component == "supervision-owner" && outcome.Reason == "shutdown-escalated" {
					lines = append(lines, prefix+": killed (TERM ignored; reaped reason=shutdown-escalated)")
				} else {
					lines = append(lines, prefix+": killed (TERM ignored)")
				}
			} else if outcome.Component == "supervision-owner" && outcome.Reason == "shutdown" {
				lines = append(lines, prefix+": stopped (TERM, exited reason=shutdown)")
			} else {
				lines = append(lines, prefix+": stopped (TERM)")
			}
		case ShutdownNotStopped:
			lines = append(lines, fmt.Sprintf("NOT STOPPED %s pid %d started %d: %s; did: left it listed in the fence record",
				outcome.Component, outcome.Identity.Pid, outcome.Identity.StartedAtSec, outcome.Reason))
		}
	}
	for _, failure := range r.Failures {
		lines = append(lines, failure.Line())
	}
	return lines
}

// ComponentFailure preserves which standing process prevented a safe arming
// transition so the operator outcome can name the exact failed component.
type ComponentFailure struct {
	Component string
	Err       error
}

func (e *ComponentFailure) Error() string { return e.Err.Error() }
func (e *ComponentFailure) Unwrap() error { return e.Err }

func ownerPath(root string) string {
	return filepath.Join(SupervisionDir(root), "lock.d", "owner.json")
}

func ownerLockDir(root string) string {
	return filepath.Dir(ownerPath(root))
}

// ReadArmingOwner reads and validates the exact owner record.
func ReadArmingOwner(root string) (ArmingOwner, error) {
	var owner ArmingOwner
	data, err := os.ReadFile(ownerPath(root))
	if err != nil {
		return owner, err
	}
	if err := json.Unmarshal(data, &owner); err != nil {
		return owner, fmt.Errorf("supervision owner record is malformed: %w", err)
	}
	if owner.Pid < 1 || owner.PidStartedAt < 1 || owner.InstanceTag == "" ||
		owner.FenceGeneration < 0 || (owner.PidStartTicks == 0) != (owner.BootID == "") {
		return owner, fmt.Errorf("supervision owner record is incomplete")
	}
	return owner, nil
}

// WriteArmingOwner publishes the owner identity atomically after the caller
// has acquired lock.d.
func WriteArmingOwner(root string, owner ArmingOwner) error {
	if owner.Pid < 1 || owner.PidStartedAt < 1 || owner.InstanceTag == "" ||
		owner.FenceGeneration < 0 || (owner.PidStartTicks == 0) != (owner.BootID == "") {
		return fmt.Errorf("supervision owner identity is incomplete")
	}
	data, err := json.MarshalIndent(owner, "", "  ")
	if err != nil {
		return err
	}
	_, err = atomicfile.WriteText(ownerPath(root), string(append(data, '\n')), root)
	return err
}

// ReadPublishedGeneration reads the generation configuration the live owner
// published. An absent or malformed document is not treated as an older
// generation; callers must distinguish establishment from replacement.
func ReadPublishedGeneration(root string) (PublishedGeneration, error) {
	var document stateDocument
	data, err := os.ReadFile(filepath.Join(SupervisionDir(root), "state.json"))
	if err != nil {
		return PublishedGeneration{}, err
	}
	if err := json.Unmarshal(data, &document); err != nil {
		return PublishedGeneration{}, fmt.Errorf("supervision state is malformed: %w", err)
	}
	return PublishedGeneration{
		Owner: ArmingOwner{
			Pid: document.Owner.Pid, PidStartedAt: document.Owner.PidStartedAt,
			PidStartTicks: document.Owner.PidStartTicks, BootID: document.Owner.BootID,
			InstanceTag: document.Owner.InstanceTag,
		},
		Fingerprint: document.Fingerprint,
		IntervalSec: document.IntervalSec,
		WatcherCap:  document.DerivedWatcherCapMin,
		Generation:  document.Generation,
	}, nil
}

func sameArmingOwner(left, right ArmingOwner) bool {
	if left.Pid != right.Pid || left.InstanceTag != right.InstanceTag {
		return false
	}
	if left.PidStartTicks > 0 && left.BootID != "" && right.PidStartTicks > 0 && right.BootID != "" {
		return left.PidStartTicks == right.PidStartTicks && left.BootID == right.BootID
	}
	return left.PidStartedAt == right.PidStartedAt
}

func ownerLiveness(owner ArmingOwner) identity.Liveness {
	ref := identity.Ref{
		Pid: owner.Pid, StartedAtSec: owner.PidStartedAt,
		StartTicks: owner.PidStartTicks, BootID: owner.BootID,
	}
	return identity.AliveTaggedRef(identity.KernelProber{}, ref, owner.InstanceTag)
}

var armingOwnerLiveness = ownerLiveness

func scaledWait(baseSeconds int, scaleMilli int) time.Duration {
	if scaleMilli < 1 {
		scaleMilli = 1000
	}
	seconds := (baseSeconds*scaleMilli + 999) / 1000
	if seconds < 1 {
		seconds = 1
	}
	return time.Duration(seconds) * time.Second
}

func acquireCapAuthorityLock(root string, scaleMilli int) (func() error, error) {
	directory := filepath.Join(SupervisionDir(root), "cap-authority.lock.d")
	if err := os.MkdirAll(filepath.Dir(directory), 0o755); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(scaledWait(10, scaleMilli))
	for {
		err := dispatch.OwnerLockClaim(directory, int64(os.Getpid()), "metasystem up")
		if err == nil {
			return func() error {
				return dispatch.OwnerLockRelease(directory, int64(os.Getpid()), "metasystem up")
			}, nil
		}
		if !errors.Is(err, dispatch.ErrOwnerLockBusy) {
			return nil, err
		}
		if !time.Now().Before(deadline) {
			return nil, fmt.Errorf("repository cap-authority lock remained busy")
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func requireCeilingClear(root, metasystemRoot string, ceiling int64) error {
	blocker, blocked, err := BlockingReservedCapAt(filepath.Join(root, "artifacts", "agents"), metasystemRoot, ceiling)
	if err != nil {
		return err
	}
	if blocked {
		return fmt.Errorf("derived %dm watcher ceiling does not strictly clear reserved cap %dm for job %s", ceiling, blocker.Cap, blocker.Job)
	}
	return nil
}

func publishedForOwner(root string, owner ArmingOwner, wait time.Duration) (PublishedGeneration, error) {
	deadline := time.Now().Add(wait)
	var lastErr error
	for {
		generation, err := ReadPublishedGeneration(root)
		if err == nil && sameArmingOwner(generation.Owner, owner) {
			return generation, nil
		}
		if err != nil {
			lastErr = err
		} else {
			lastErr = fmt.Errorf("supervision state names another owner")
		}
		if !time.Now().Before(deadline) {
			return PublishedGeneration{}, lastErr
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func generationMatches(generation PublishedGeneration, options EnsureOptions) bool {
	return generation.Fingerprint == options.Fingerprint &&
		generation.IntervalSec == options.IntervalSec &&
		generation.WatcherCap == int(options.WatcherCap)
}

func waitUntilArmed(options EnsureOptions, owner ArmingOwner) ArmedInspection {
	deadline := time.Now().Add(scaledWait(options.IntervalSec+10, options.WaitScaleMilli))
	inspection := ArmedInspection{Component: "repo-watcher", Reason: "the supervision generation has not completed its first census"}
	for time.Now().Before(deadline) {
		inspection = InspectArmedAt(filepath.Join(options.Root, "artifacts", "agents"), options.MetasystemRoot, owner.Pid,
			owner.PidStartedAt, owner.InstanceTag, int64(options.IntervalSec), time.Now())
		if inspection.Armed() {
			return inspection
		}
		time.Sleep(50 * time.Millisecond)
	}
	return inspection
}

func writeShutdownIntent(root string, owner ArmingOwner, requester string) error {
	record := intentRecord{
		TargetPid: owner.Pid, TargetPidStartedAt: owner.PidStartedAt,
		TargetInstanceTag: owner.InstanceTag, Requester: requester,
		WrittenAt: time.Now().UTC().Format(time.RFC3339),
	}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	_, err = atomicfile.WriteText(filepath.Join(ownerLockDir(root), "shutdown-intent.json"), string(append(data, '\n')), root)
	return err
}

func signalGroup(pid int64, signal syscall.Signal) error {
	if err := syscall.Kill(-int(pid), signal); err == nil || err == syscall.ESRCH {
		return nil
	} else {
		return err
	}
}

var (
	armingOwnerSignal = signalGroup
	armingNow         = time.Now
	armingSleep       = time.Sleep
)

func scaledDuration(base time.Duration, scaleMilli int) time.Duration {
	if scaleMilli < 1 {
		scaleMilli = 1000
	}
	scaled := time.Duration((int64(base)*int64(scaleMilli) + 999) / 1000)
	if scaled < time.Millisecond {
		return time.Millisecond
	}
	return scaled
}

func publishedOwnerState(root string, expected ...ArmingOwner) (generation int64, teardownCeilingSeconds int) {
	data, err := os.ReadFile(filepath.Join(SupervisionDir(root), "state.json"))
	if err == nil {
		var document stateDocument
		if json.Unmarshal(data, &document) == nil {
			if len(expected) > 0 {
				published := ArmingOwner{
					Pid: document.Owner.Pid, PidStartedAt: document.Owner.PidStartedAt,
					PidStartTicks: document.Owner.PidStartTicks, BootID: document.Owner.BootID,
					InstanceTag: document.Owner.InstanceTag,
				}
				if !sameArmingOwner(published, expected[0]) {
					return 0, 41
				}
			}
			generation = document.Generation
			if document.TeardownCeilingSec > 0 {
				return generation, document.TeardownCeilingSec
			}
		}
	}
	return generation, 41
}

func registryShowsOwnerExited(path, ownerTag string) bool {
	frames, err := registry.ReadFrames(path)
	if err != nil {
		return false
	}
	reduction, err := registry.Reduce(frames)
	if err != nil {
		return false
	}
	claim := reduction.Claims[ownerTag]
	return claim != nil && claim.Closed && claim.ClosedBy == registry.EventExited
}

func stopOwner(root string, owner ArmingOwner, scaleMilli int, requester string) (ComponentOutcome, error) {
	generation, ceilingSeconds := publishedOwnerState(root, owner)
	outcome := ComponentOutcome{
		Component: "supervision-owner", Identity: identity.Ref{
			Pid: owner.Pid, StartedAtSec: owner.PidStartedAt,
			StartTicks: owner.PidStartTicks, BootID: owner.BootID,
		}, Tag: owner.InstanceTag, Generation: generation, Signal: ShutdownSignalNone,
	}
	if armingOwnerLiveness(owner) == identity.Dead {
		outcome.Result = ShutdownAlreadyGone
		return outcome, nil
	}
	if armingOwnerLiveness(owner) == identity.Unknown {
		outcome.Result = ShutdownNotStopped
		outcome.Reason = "owner identity is uninspectable; replacement is not authorized"
		return outcome, fmt.Errorf("%s", outcome.Reason)
	}
	if err := writeShutdownIntent(root, owner, requester); err != nil {
		outcome.Result = ShutdownNotStopped
		outcome.Reason = fmt.Sprintf("write shutdown intent: %v", err)
		return outcome, fmt.Errorf("%s", outcome.Reason)
	}
	// Re-authenticate immediately beside signalling: the replacement caller
	// did not spawn this owner and the shutdown-intent write created a race
	// window in which the recorded pid could have changed.
	switch armingOwnerLiveness(owner) {
	case identity.Dead:
		outcome.Result = ShutdownAlreadyGone
		return outcome, nil
	case identity.Unknown:
		outcome.Result = ShutdownNotStopped
		outcome.Reason = "owner identity became uninspectable before signalling; replacement is not authorized"
		return outcome, fmt.Errorf("%s", outcome.Reason)
	}
	if err := armingOwnerSignal(owner.Pid, syscall.SIGTERM); err != nil {
		outcome.Result = ShutdownNotStopped
		outcome.Reason = fmt.Sprintf("signal owner: %v", err)
		return outcome, fmt.Errorf("%s", outcome.Reason)
	}
	outcome.Signal = ShutdownSignalTerm
	registryPath, _ := registry.DefaultPath()
	deadline := armingNow().Add(scaledWait(ceilingSeconds, scaleMilli))
	for armingNow().Before(deadline) {
		if armingOwnerLiveness(owner) == identity.Dead {
			outcome.Result = ShutdownStopped
			if registryPath != "" && registryShowsOwnerExited(registryPath, owner.InstanceTag) {
				outcome.Reason = "shutdown"
			}
			return outcome, nil
		}
		if registryPath != "" && registryShowsOwnerExited(registryPath, owner.InstanceTag) {
			outcome.Result = ShutdownStopped
			outcome.Reason = "shutdown"
			return outcome, nil
		}
		armingSleep(50 * time.Millisecond)
	}
	if registryPath != "" && registryShowsOwnerExited(registryPath, owner.InstanceTag) {
		outcome.Result = ShutdownStopped
		outcome.Reason = "shutdown"
		return outcome, nil
	}
	switch armingOwnerLiveness(owner) {
	case identity.Dead:
		outcome.Result = ShutdownStopped
		if registryPath != "" && registryShowsOwnerExited(registryPath, owner.InstanceTag) {
			outcome.Reason = "shutdown"
		}
		return outcome, nil
	case identity.Unknown:
		outcome.Result = ShutdownNotStopped
		outcome.Reason = "owner identity became uninspectable before SIGKILL; replacement is not authorized"
		return outcome, fmt.Errorf("%s", outcome.Reason)
	}
	if err := armingOwnerSignal(owner.Pid, syscall.SIGKILL); err != nil {
		outcome.Result = ShutdownNotStopped
		outcome.Reason = fmt.Sprintf("kill owner: %v", err)
		return outcome, fmt.Errorf("%s", outcome.Reason)
	}
	outcome.Signal = ShutdownSignalKill
	deadline = armingNow().Add(scaledDuration(componentPostKillProofInterval, scaleMilli))
	for armingNow().Before(deadline) {
		if armingOwnerLiveness(owner) == identity.Dead {
			outcome.Result = ShutdownStopped
			outcome.Reason = "shutdown-escalated"
			return outcome, nil
		}
		armingSleep(20 * time.Millisecond)
	}
	outcome.Result = ShutdownNotStopped
	outcome.Reason = fmt.Sprintf("owner pid %d did not stop after KILL", owner.Pid)
	return outcome, nil
}

func recordedHeld(root string) ([]Held, error) {
	var document stateDocument
	data, err := os.ReadFile(filepath.Join(SupervisionDir(root), "state.json"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read recorded supervision components: %w", err)
	}
	if err := json.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("recorded supervision state is malformed: %w", err)
	}
	var held []Held
	for _, kind := range []Component{Watcher, Reaper} {
		component, exists := document.Components[string(kind)]
		if !exists {
			continue
		}
		if component.Pid < 1 || component.PidStartedAt < 1 || component.InstanceTag == "" ||
			(component.PidStartTicks == 0) != (component.BootID == "") {
			return nil, fmt.Errorf("recorded %s identity is incomplete", kind)
		}
		held = append(held, Held{
			Component: kind, Tag: component.InstanceTag, Generation: document.Generation,
			Identity: identity.Ref{
				Pid: component.Pid, StartedAtSec: component.PidStartedAt,
				StartTicks: component.PidStartTicks, BootID: component.BootID,
			},
		})
	}
	return held, nil
}

type recordedComponentControl struct {
	prober      identity.Prober
	groupAbsent func(int64) (bool, error)
	signalGroup func(int64, syscall.Signal) error
}

func kernelGroupAbsent(pgid int64) (bool, error) {
	err := syscall.Kill(-int(pgid), 0)
	switch err {
	case nil, syscall.EPERM:
		return false, nil
	case syscall.ESRCH:
		return true, nil
	default:
		return false, err
	}
}

func authenticateRecordedComponent(control recordedComponentControl, held Held) error {
	switch identity.AliveTaggedRef(control.prober, held.Identity, held.Tag) {
	case identity.Alive:
		return nil
	case identity.Unknown:
		return fmt.Errorf("recorded %s identity is uninspectable; takeover is not authorized", held.Component)
	default:
		absent, err := control.groupAbsent(held.Identity.Pid)
		if err != nil {
			return fmt.Errorf("prove recorded %s process group absent: %w", held.Component, err)
		}
		if absent {
			return errRecordedComponentGone
		}
		return fmt.Errorf("recorded %s identity is gone or no longer tag-authenticated while process group %d remains", held.Component, held.Identity.Pid)
	}
}

var errRecordedComponentGone = errors.New("recorded component is already gone")

func waitForRecordedGroupAbsence(control recordedComponentControl, pgid int64, wait time.Duration) (bool, error) {
	deadline := time.Now().Add(wait)
	for {
		absent, err := control.groupAbsent(pgid)
		if err != nil || absent || !time.Now().Before(deadline) {
			return absent, err
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func supervisionComponentName(component Component) string {
	if component == Reaper {
		return "job-reaper"
	}
	return "repo-watcher"
}

func stopRecordedComponent(control recordedComponentControl, held Held, scaleMilli int) (ComponentOutcome, error) {
	outcome := ComponentOutcome{
		Component: supervisionComponentName(held.Component), Identity: held.Identity,
		Tag: held.Tag, Generation: held.Generation, Signal: ShutdownSignalNone,
	}
	if err := authenticateRecordedComponent(control, held); err != nil {
		if errors.Is(err, errRecordedComponentGone) {
			outcome.Result = ShutdownAlreadyGone
			return outcome, nil
		}
		outcome.Result = ShutdownNotStopped
		outcome.Reason = err.Error()
		return outcome, err
	}
	// The identity and tag proof is immediately adjacent to this signal.
	// These detached processes were not spawned by this up invocation.
	if err := control.signalGroup(held.Identity.Pid, syscall.SIGTERM); err != nil {
		err = fmt.Errorf("signal recorded %s process group: %w", held.Component, err)
		outcome.Result = ShutdownNotStopped
		outcome.Reason = err.Error()
		return outcome, err
	}
	outcome.Signal = ShutdownSignalTerm
	if absent, err := waitForRecordedGroupAbsence(control, held.Identity.Pid, scaledWait(5, scaleMilli)); err != nil {
		err = fmt.Errorf("prove recorded %s process group stopped: %w", held.Component, err)
		outcome.Result = ShutdownNotStopped
		outcome.Reason = err.Error()
		return outcome, err
	} else if absent {
		outcome.Result = ShutdownStopped
		return outcome, nil
	}
	// Re-authenticate beside escalation. If the leader exited or changed
	// identity while its group survived, custody is no longer sufficient to
	// risk signalling whatever now occupies the recorded group id.
	if err := authenticateRecordedComponent(control, held); err != nil {
		err = fmt.Errorf("refuse SIGKILL for %s: %w", held.Component, err)
		outcome.Result = ShutdownNotStopped
		outcome.Reason = err.Error()
		return outcome, err
	}
	if err := control.signalGroup(held.Identity.Pid, syscall.SIGKILL); err != nil {
		err = fmt.Errorf("kill recorded %s process group: %w", held.Component, err)
		outcome.Result = ShutdownNotStopped
		outcome.Reason = err.Error()
		return outcome, err
	}
	outcome.Signal = ShutdownSignalKill
	absent, err := waitForRecordedGroupAbsence(control, held.Identity.Pid, scaledDuration(componentPostKillProofInterval, scaleMilli))
	if err != nil {
		err = fmt.Errorf("prove recorded %s process group killed: %w", held.Component, err)
		outcome.Result = ShutdownNotStopped
		outcome.Reason = err.Error()
		return outcome, err
	}
	if !absent {
		outcome.Result = ShutdownNotStopped
		outcome.Reason = fmt.Sprintf("death unproven after KILL for process group %d", held.Identity.Pid)
		return outcome, nil
	}
	outcome.Result = ShutdownStopped
	return outcome, nil
}

var enumerateTakeoverProcesses = census.EnumerateConfiguredProcesses

func processArgument(fields []string, name string) string {
	for index := 0; index+1 < len(fields); index++ {
		if fields[index] == name {
			return fields[index+1]
		}
	}
	return ""
}

func taggedTakeoverComponents(root, ownerTag string, processes []census.Process) ([]Held, error) {
	var held []Held
	for _, process := range processes {
		if !process.Alive || !strings.Contains(process.Argv, ownerTag+"-") {
			continue
		}
		fields := strings.Fields(process.Argv)
		componentName := processArgument(fields, "--component")
		tag := processArgument(fields, "--tag")
		repo := processArgument(fields, "--repo")
		component := Component(componentName)
		if (component != Watcher && component != Reaper) ||
			!strings.HasPrefix(tag, ownerTag+"-"+componentName+"-") {
			continue
		}
		if canonicalPathForTakeover(repo) != canonicalPathForTakeover(root) {
			return nil, fmt.Errorf("tagged %s pid %d names another repository %q", component, process.Pid, repo)
		}
		if process.Pid < 1 || process.Started < 1 || process.PGID != process.Pid ||
			(process.StartTicks == 0) != (process.BootID == "") {
			return nil, fmt.Errorf("tagged %s pid %d has incomplete process-group identity", component, process.Pid)
		}
		generation, err := strconv.ParseInt(processArgument(fields, "--generation"), 10, 64)
		if err != nil || generation < 1 {
			return nil, fmt.Errorf("tagged %s pid %d has an invalid generation", component, process.Pid)
		}
		held = append(held, Held{Component: component, Tag: tag, Generation: generation, Identity: identity.Ref{
			Pid: process.Pid, StartedAtSec: process.Started,
			StartTicks: process.StartTicks, BootID: process.BootID,
		}})
	}
	return held, nil
}

func canonicalPathForTakeover(path string) string {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	if resolved, err := filepath.EvalSymlinks(absolute); err == nil {
		return resolved
	}
	return filepath.Clean(absolute)
}

func mergeTakeoverComponents(recorded, discovered []Held) ([]Held, error) {
	byPid := map[int64]Held{}
	for _, component := range append(recorded, discovered...) {
		if prior, exists := byPid[component.Identity.Pid]; exists {
			if prior.Tag != component.Tag || prior.Component != component.Component || prior.Identity != component.Identity || prior.Generation != component.Generation {
				return nil, fmt.Errorf("supervision component pid %d has conflicting recorded and census identities", component.Identity.Pid)
			}
			continue
		}
		byPid[component.Identity.Pid] = component
	}
	held := make([]Held, 0, len(byPid))
	for _, component := range byPid {
		held = append(held, component)
	}
	sort.Slice(held, func(i, j int) bool {
		rank := func(component Component) int {
			if component == Watcher {
				return 0
			}
			return 1
		}
		if rank(held[i].Component) != rank(held[j].Component) {
			return rank(held[i].Component) < rank(held[j].Component)
		}
		if held[i].Generation != held[j].Generation {
			return held[i].Generation < held[j].Generation
		}
		return held[i].Identity.Pid < held[j].Identity.Pid
	})
	return held, nil
}

func takeoverComponents(root, metasystemRoot, ownerTag string) ([]Held, error) {
	held, err := recordedHeld(root)
	if err != nil {
		return nil, err
	}
	// Without an owner tag there is no safe prefix for the census half of the
	// sweep. Recorded identities remain sufficient for the ordinary
	// crashed-owner path and are authenticated again beside every signal.
	if ownerTag == "" {
		return held, nil
	}
	processes, err := enumerateTakeoverProcesses(metasystemRoot)
	if err != nil {
		return nil, fmt.Errorf("enumerate tagged supervision components: %w", err)
	}
	discovered, err := taggedTakeoverComponents(root, ownerTag, processes)
	if err != nil {
		return nil, err
	}
	return mergeTakeoverComponents(held, discovered)
}

// ReadInventory returns live owner and component identities from the caller's
// single process snapshot. It performs no writes and sends no signals.
func ReadInventory(root string, processes []census.Process) ([]InventoryItem, error) {
	owner, ownerErr := ReadArmingOwner(root)
	if ownerErr != nil && !os.IsNotExist(ownerErr) {
		return nil, ownerErr
	}
	var items []InventoryItem
	ownerTag := ""
	fenceGeneration := int64(0)
	if ownerErr == nil {
		ownerTag = owner.InstanceTag
		fenceGeneration = owner.FenceGeneration
		switch armingOwnerLiveness(owner) {
		case identity.Unknown:
			return nil, fmt.Errorf("owner identity is uninspectable")
		case identity.Alive:
			generation, _ := publishedOwnerState(root, owner)
			items = append(items, InventoryItem{
				Component: "supervision-owner",
				Identity: identity.Ref{
					Pid: owner.Pid, StartedAtSec: owner.PidStartedAt,
					StartTicks: owner.PidStartTicks, BootID: owner.BootID,
				},
				Tag: ownerTag, Generation: generation, FenceGeneration: fenceGeneration,
			})
		}
	}
	recorded, err := recordedHeld(root)
	if err != nil {
		return nil, err
	}
	if ownerTag == "" {
		var document stateDocument
		if data, readErr := os.ReadFile(filepath.Join(SupervisionDir(root), "state.json")); readErr == nil && json.Unmarshal(data, &document) == nil {
			ownerTag = document.Owner.InstanceTag
		}
	}
	var discovered []Held
	if ownerTag != "" {
		discovered, err = taggedTakeoverComponents(root, ownerTag, processes)
		if err != nil {
			return nil, err
		}
	}
	held, err := mergeTakeoverComponents(recorded, discovered)
	if err != nil {
		return nil, err
	}
	control := takeoverComponentControl()
	for _, component := range held {
		authErr := authenticateRecordedComponent(control, component)
		if errors.Is(authErr, errRecordedComponentGone) {
			continue
		}
		if authErr != nil {
			return nil, authErr
		}
		items = append(items, InventoryItem{
			Component: supervisionComponentName(component.Component), Identity: component.Identity,
			Tag: component.Tag, Generation: component.Generation, FenceGeneration: fenceGeneration,
		})
	}
	return items, nil
}

var takeoverComponentControl = func() recordedComponentControl {
	return recordedComponentControl{
		prober: identity.KernelProber{}, groupAbsent: kernelGroupAbsent,
		signalGroup: func(pgid int64, signal syscall.Signal) error {
			if err := syscall.Kill(-int(pgid), signal); err != nil && err != syscall.ESRCH {
				return err
			}
			return nil
		},
	}
}

func stopTakeoverComponents(root, metasystemRoot, ownerTag string, scaleMilli int, ownerOrderly bool) ([]ComponentOutcome, error) {
	held, err := takeoverComponents(root, metasystemRoot, ownerTag)
	if err != nil {
		return nil, err
	}
	control := takeoverComponentControl()
	var outcomes []ComponentOutcome
	var failures []error
	for _, component := range held {
		outcome, stopErr := stopRecordedComponent(control, component, scaleMilli)
		if ownerOrderly && outcome.Result == ShutdownAlreadyGone {
			outcome.Reason = "by the owner"
		}
		outcomes = append(outcomes, outcome)
		if stopErr != nil {
			failures = append(failures, &ComponentFailure{Component: outcome.Component, Err: stopErr})
		}
	}
	return outcomes, errors.Join(failures...)
}

func releaseDeadOwnerLock(root string, expected ArmingOwner) error {
	current, err := ReadArmingOwner(root)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil && !sameArmingOwner(current, expected) {
		return fmt.Errorf("another owner won the supervision lock")
	}
	if err == nil {
		if err := os.Remove(ownerPath(root)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	_ = os.Remove(filepath.Join(ownerLockDir(root), "shutdown-intent.json"))
	if err := os.Remove(ownerLockDir(root)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func withoutExecutionID(environment []string) []string {
	filtered := make([]string, 0, len(environment))
	for _, entry := range environment {
		if strings.HasPrefix(entry, "METASYSTEM_EXECUTION_ID=") {
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered
}

var releaseLaunchedOwner = func(command *exec.Cmd) error {
	return command.Process.Release()
}

func launchOwner(options EnsureOptions, tag string) (ArmingOwner, error) {
	supervisionDir := SupervisionDir(options.Root)
	logFile, err := os.OpenFile(filepath.Join(supervisionDir, "owner.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return ArmingOwner{}, err
	}
	defer logFile.Close()
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		return ArmingOwner{}, err
	}
	defer devNull.Close()
	gateFile, err := os.CreateTemp(supervisionDir, ".owner-gate-*")
	if err != nil {
		return ArmingOwner{}, err
	}
	gate := gateFile.Name()
	gateFile.Close()
	if err := os.Remove(gate); err != nil {
		return ArmingOwner{}, err
	}
	argv := []string{"supervise", "owner", "--repo", options.Root, "--metasystem-root", options.MetasystemRoot, "--scope", options.Scope,
		"--gate", gate, "--tag", tag, "--interval", strconv.Itoa(options.IntervalSec),
		"--fingerprint", options.Fingerprint, "--watcher-cap", strconv.FormatInt(options.WatcherCap, 10)}
	for _, variable := range []string{
		"METASYSTEM_GO_OWNER_IGNORE_TERM",
		"METASYSTEM_GO_COMPONENT_CRASH_ON_START",
		"METASYSTEM_GO_COMPONENT_IGNORE_TERM",
		"METASYSTEM_GO_COMPONENT_SLOW_STOP",
	} {
		if os.Getenv(variable) != "" && !fixtureauth.FixtureModeRoot(options.MetasystemRoot) {
			return ArmingOwner{}, fmt.Errorf("supervision fixture seam %s is fixture-only", variable)
		}
	}
	if os.Getenv("METASYSTEM_GO_OWNER_IGNORE_TERM") != "" {
		argv = append(argv, "--ignore-term")
	}
	var cmd *exec.Cmd
	if options.Command != nil {
		cmd, err = options.Command(argv...)
		if err != nil {
			return ArmingOwner{}, err
		}
	} else {
		cmd = exec.Command(options.Binary, argv...)
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = devNull, logFile, logFile
	cmd.Env = withoutExecutionID(os.Environ())
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return ArmingOwner{}, err
	}
	pid := int64(cmd.Process.Pid)
	deadline := time.Now().Add(scaledWait(5, options.WaitScaleMilli))
	var exact identity.Exact
	for time.Now().Before(deadline) {
		var state identity.Liveness
		exact, state, _ = (identity.KernelProber{}).Probe(pid)
		if state == identity.Alive {
			break
		}
		if state == identity.Dead {
			return ArmingOwner{}, fmt.Errorf("owner process died before publishing its identity")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if exact.Pid == 0 {
		_ = cmd.Process.Kill()
		return ArmingOwner{}, fmt.Errorf("owner process identity was not readable before the start deadline")
	}
	owner := ArmingOwner{
		Pid: pid, PidStartedAt: exact.StartedAt.Unix(), PidStartTicks: exact.StartTicks, BootID: exact.BootID, InstanceTag: tag,
		AcquiredAt: time.Now().UTC().Format(time.RFC3339), FenceGeneration: options.FenceGeneration}
	if err := WriteArmingOwner(options.Root, owner); err != nil {
		_ = cmd.Process.Kill()
		return ArmingOwner{}, err
	}
	if err := os.WriteFile(gate, []byte("start\n"), 0o600); err != nil {
		_ = cmd.Process.Kill()
		return ArmingOwner{}, err
	}
	_ = releaseLaunchedOwner(cmd)
	return owner, nil
}

func newOwnerTag(prefix string) string {
	return fmt.Sprintf("%s%d-%d", prefix, time.Now().Unix(), os.Getpid())
}

func requireOwnerCheckoutPath(requestedStateRoot string, owner ArmingOwner) error {
	requestedCheckout := canonicalPathForTakeover(requestedStateRoot)
	registryPath, err := registry.DefaultPath()
	if err != nil {
		return fmt.Errorf("resolve supervision registry for checkout %q: %w", requestedCheckout, err)
	}
	recordedCheckout, found, err := registry.OwnerCheckoutPath(registryPath, owner.InstanceTag)
	if err != nil {
		return fmt.Errorf("read supervision registry before acting for checkout %q: %w", requestedCheckout, err)
	}
	if !found {
		return fmt.Errorf("supervision request for checkout %q names owner tag %q with no registry checkout; refusing to select an owner by tag", requestedCheckout, owner.InstanceTag)
	}
	recordedCheckout = canonicalPathForTakeover(recordedCheckout)
	if recordedCheckout != requestedCheckout {
		return fmt.Errorf("supervision request for checkout %q names owner tag %q recorded for checkout %q; refusing to act on another repository", requestedCheckout, owner.InstanceTag, recordedCheckout)
	}
	return nil
}

func requireOwnerTagPrefix(requestedStateRoot, expectedTagPrefix string, owner ArmingOwner) error {
	requestedCheckout := canonicalPathForTakeover(requestedStateRoot)
	if !strings.HasPrefix(owner.InstanceTag, expectedTagPrefix) {
		return fmt.Errorf("supervision request for checkout %q names owner tag %q outside that checkout's prefix %q; refusing to act on another repository", requestedCheckout, owner.InstanceTag, expectedTagPrefix)
	}
	return nil
}

func appendShutdownEscalated(root string, owner ArmingOwner, outcomes []ComponentOutcome, forceSweepPending bool, scaleMilli int) error {
	registryPath, err := registry.DefaultPath()
	if err != nil {
		return fmt.Errorf("resolve supervision registry for escalated shutdown: %w", err)
	}
	if registryShowsOwnerExited(registryPath, owner.InstanceTag) {
		return nil
	}
	exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		return fmt.Errorf("read shutdown caller identity for escalated registry row")
	}
	killed := make([]string, 0, len(outcomes))
	sweepPending := forceSweepPending
	for _, outcome := range outcomes {
		if outcome.Signal == ShutdownSignalKill {
			killed = append(killed, fmt.Sprintf("pid %d (%s)", outcome.Identity.Pid, outcome.Component))
		}
		if outcome.Result == ShutdownNotStopped {
			sweepPending = true
		}
	}
	record := map[string]any{
		"schemaVersion": 1,
		"event":         registry.EventReaped,
		"checkoutPath":  root,
		"at":            time.Now().UTC().Format(time.RFC3339),
		"ownerTag":      owner.InstanceTag,
		"reason":        "shutdown-escalated",
		"killed":        killed,
		"sweepPending":  sweepPending,
		"engine":        "go",
		"engineBuild":   BuildStamp,
	}
	payload, err := EncodeRecord(record)
	if err != nil {
		return fmt.Errorf("encode escalated shutdown registry row: %w", err)
	}
	self := lock.Identity{
		Pid: exact.Pid, PidStartedAt: exact.StartedAt.Unix(),
		Tag: fmt.Sprintf("metasystem-shutdown-%d", exact.Pid),
	}
	probe := func(who lock.Identity) lock.Liveness {
		switch identity.AliveRef(identity.KernelProber{}, identity.Ref{Pid: who.Pid, StartedAtSec: who.PidStartedAt}) {
		case identity.Alive:
			return lock.Alive
		case identity.Dead:
			return lock.Dead
		default:
			return lock.Unknown
		}
	}
	if err := registry.LockedAppend(registryPath, self, payload,
		scaledDuration(registryAppendLockWait, scaleMilli), 25*time.Millisecond, probe); err != nil {
		return fmt.Errorf("append escalated shutdown registry row: %w", err)
	}
	return nil
}

// EnsureArmed establishes exactly one current supervision owner and waits
// until its watcher, reaper, and first generation-bound census all verify.
// A dead owner is taken over only after exact death; a live older generation
// is stopped through its intent channel and replaced under the same cap fence.
func EnsureArmed(options EnsureOptions) (result EnsureResult, err error) {
	if options.MetasystemRoot == "" {
		options.MetasystemRoot = options.Root
	}
	if options.Root == "" || options.MetasystemRoot == "" || options.Scope == "" || (options.Binary == "" && options.Command == nil) || options.Fingerprint == "" || options.IntervalSec < 1 || options.WatcherCap < 1 || options.OwnerTagPrefix == "" || options.FenceGeneration < 0 {
		return EnsureResult{}, fmt.Errorf("supervision arming options are incomplete")
	}
	if options.WaitScaleMilli < 1 {
		options.WaitScaleMilli = 1000
	}
	if err := os.MkdirAll(SupervisionDir(options.Root), 0o755); err != nil {
		return EnsureResult{}, err
	}
	release, err := acquireCapAuthorityLock(options.Root, options.WaitScaleMilli)
	if err != nil {
		return EnsureResult{}, err
	}
	defer func() {
		if releaseErr := release(); err == nil && releaseErr != nil {
			result = EnsureResult{}
			err = fmt.Errorf("release repository cap-authority lock: %w", releaseErr)
		}
	}()

	action := "started"
	for attempts := 0; attempts < 4; attempts++ {
		if err := os.Mkdir(ownerLockDir(options.Root), 0o755); err == nil {
			if err := requireCeilingClear(options.Root, options.MetasystemRoot, options.WatcherCap); err != nil {
				_ = os.Remove(ownerLockDir(options.Root))
				return EnsureResult{}, err
			}
			owner, err := launchOwner(options, newOwnerTag(options.OwnerTagPrefix))
			if err != nil {
				_ = os.Remove(ownerLockDir(options.Root))
				return EnsureResult{}, err
			}
			inspection := waitUntilArmed(options, owner)
			generation, _ := ReadPublishedGeneration(options.Root)
			return EnsureResult{Action: action, Owner: owner, Generation: generation.Generation, Inspection: inspection}, nil
		} else if !os.IsExist(err) {
			return EnsureResult{}, err
		}

		owner, err := ReadArmingOwner(options.Root)
		if os.IsNotExist(err) {
			deadline := time.Now().Add(scaledWait(5, options.WaitScaleMilli))
			for os.IsNotExist(err) && time.Now().Before(deadline) {
				time.Sleep(20 * time.Millisecond)
				owner, err = ReadArmingOwner(options.Root)
			}
		}
		if err != nil {
			return EnsureResult{}, fmt.Errorf("supervision lock has no provable owner: %w", err)
		}
		switch armingOwnerLiveness(owner) {
		case identity.Unknown:
			return EnsureResult{}, fmt.Errorf("supervision owner pid %d is uninspectable; takeover is not authorized", owner.Pid)
		case identity.Dead:
			if err := requireCeilingClear(options.Root, options.MetasystemRoot, options.WatcherCap); err != nil {
				return EnsureResult{}, err
			}
			if _, err := stopTakeoverComponents(options.Root, options.MetasystemRoot, owner.InstanceTag, options.WaitScaleMilli, false); err != nil {
				return EnsureResult{}, fmt.Errorf("dead-owner takeover refused: %w", err)
			}
			if err := releaseDeadOwnerLock(options.Root, owner); err != nil {
				continue
			}
			action = "taken-over"
			continue
		}
		if err := requireOwnerCheckoutPath(options.Root, owner); err != nil {
			return EnsureResult{}, err
		}
		// Tags never select a victim. A mismatch can only veto the
		// checkout-path selection made above.
		if err := requireOwnerTagPrefix(options.Root, options.OwnerTagPrefix, owner); err != nil {
			return EnsureResult{}, err
		}

		if options.OnlyIfDown {
			generation, err := publishedForOwner(options.Root, owner, scaledWait(5, options.WaitScaleMilli))
			if err != nil {
				return EnsureResult{}, fmt.Errorf("live owner did not publish a verifiable generation: %w", err)
			}
			inspection := waitUntilArmed(options, owner)
			return EnsureResult{
				Action: "not-needed", Owner: owner, Generation: generation.Generation,
				Inspection: inspection,
			}, nil
		}
		generation, err := publishedForOwner(options.Root, owner, scaledWait(5, options.WaitScaleMilli))
		if err != nil {
			return EnsureResult{}, fmt.Errorf("live owner did not publish a verifiable generation: %w", err)
		}
		if generationMatches(generation, options) {
			inspection := waitUntilArmed(options, owner)
			return EnsureResult{Action: "verified", Owner: owner, Generation: generation.Generation, Inspection: inspection}, nil
		}
		if err := requireCeilingClear(options.Root, options.MetasystemRoot, options.WatcherCap); err != nil {
			return EnsureResult{}, err
		}
		if _, err := stopOwner(options.Root, owner, options.WaitScaleMilli, "metasystem up engine-generation replacement"); err != nil {
			return EnsureResult{}, err
		}
		if _, err := stopTakeoverComponents(options.Root, options.MetasystemRoot, owner.InstanceTag, options.WaitScaleMilli, false); err != nil {
			return EnsureResult{}, fmt.Errorf("generation replacement refused: %w", err)
		}
		if err := releaseDeadOwnerLock(options.Root, owner); err != nil {
			continue
		}
		action = "replaced"
	}
	return EnsureResult{}, fmt.Errorf("supervision owner changed repeatedly while arming")
}

// Shutdown stops the exact owner recorded for this checkout. The expected tag
// prefix prevents a copied lock from signalling another repository's owner.
func Shutdown(root, requestedStateRoot, expectedTagPrefix string, scaleMilli int) (ShutdownReport, error) {
	return ShutdownAt(root, root, requestedStateRoot, expectedTagPrefix, scaleMilli)
}

// ShutdownAt stops repository supervision while using the installed
// metasystem's authorized census source for the final tag sweep.
func ShutdownAt(root, metasystemRoot, requestedStateRoot, expectedTagPrefix string, scaleMilli int) (ShutdownReport, error) {
	var report ShutdownReport
	owner, err := ReadArmingOwner(root)
	if os.IsNotExist(err) {
		// A crashed owner can disappear before recording an outcome while its
		// published watcher and reaper remain. The caller owns the final sweep:
		// reuse the identity-bound component ladder instead of treating a
		// missing owner record as proof that the components are gone.
		ownerTag := ""
		if published, publishedErr := ReadPublishedGeneration(root); publishedErr == nil {
			ownerTag = published.Owner.InstanceTag
		} else if !os.IsNotExist(publishedErr) {
			return report, publishedErr
		}
		componentOutcomes, componentErr := stopTakeoverComponents(root, metasystemRoot, ownerTag, scaleMilli, false)
		report.Outcomes = append(report.Outcomes, componentOutcomes...)
		return report, componentErr
	}
	if err != nil {
		return report, err
	}
	// A copied lock never authorizes this checkout to act on another
	// repository, regardless of whether its recorded owner is still alive.
	if err := requireOwnerTagPrefix(requestedStateRoot, expectedTagPrefix, owner); err != nil {
		return report, err
	}
	// A dead owner cannot be selected as a signal target. Preserve the
	// pre-custody behavior for its cleanup path even when it died before its
	// first registry row or its recorded path is stale after a checkout move.
	switch armingOwnerLiveness(owner) {
	case identity.Alive:
		if err := requireOwnerCheckoutPath(requestedStateRoot, owner); err != nil {
			return report, err
		}
	}
	ownerOutcome, ownerErr := stopOwner(root, owner, scaleMilli, "metasystem up shutdown")
	report.Outcomes = append(report.Outcomes, ownerOutcome)
	ownerOrderly := ownerOutcome.Result == ShutdownStopped && ownerOutcome.Signal == ShutdownSignalTerm
	componentOutcomes, componentErr := stopTakeoverComponents(root, metasystemRoot, owner.InstanceTag, scaleMilli, ownerOrderly)
	report.Outcomes = append(report.Outcomes, componentOutcomes...)
	if ownerOutcome.Signal == ShutdownSignalKill {
		if err := appendShutdownEscalated(root, owner, report.Outcomes, componentErr != nil, scaleMilli); err != nil {
			registryPath, _ := registry.DefaultPath()
			report.Failures = append(report.Failures, ShutdownFailure{
				Component: "supervision-registry", Path: registryPath, Reason: err.Error(),
				Did: "left the registry without the shutdown-escalated row",
			})
			// The process outcome remains a successful KILL, but it may not
			// claim the registry row whose publication just failed.
			if ownerOutcome.Result == ShutdownStopped && ownerOutcome.Reason == "shutdown-escalated" {
				ownerOutcome.Reason = ""
				report.Outcomes[0] = ownerOutcome
			}
			componentErr = errors.Join(componentErr, err)
		}
	}
	if ownerOutcome.Result == ShutdownNotStopped {
		return report, errors.Join(ownerErr, componentErr)
	}
	if err := releaseDeadOwnerLock(root, owner); err != nil {
		report.Failures = append(report.Failures, ShutdownFailure{
			Component: "supervision-lock", Path: ownerLockDir(root), Reason: err.Error(), Did: "left the lock",
		})
		return report, errors.Join(ownerErr, componentErr, err)
	}
	return report, errors.Join(ownerErr, componentErr)
}
