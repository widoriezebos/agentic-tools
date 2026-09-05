package supervise

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/registry"
)

// ArmingOwner is the exact process identity recorded in the repository's
// supervision lock.
type ArmingOwner struct {
	Pid           int64  `json:"pid"`
	PidStartedAt  int64  `json:"pidStartedAt"`
	PidStartTicks int64  `json:"pidStartTicks,omitempty"`
	BootID        string `json:"bootId,omitempty"`
	InstanceTag   string `json:"instanceTag"`
	AcquiredAt    string `json:"acquiredAt,omitempty"`
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
	Root           string
	MetasystemRoot string
	Scope          string
	Binary         string
	Command        func(args ...string) (*exec.Cmd, error)
	Fingerprint    string
	IntervalSec    int
	WatcherCap     int64
	OnlyIfDown     bool
	WaitScaleMilli int
	OwnerTagPrefix string
}

// EnsureResult describes whether arming joined, established, took over, or
// replaced the supervision generation.
type EnsureResult struct {
	Action     string
	Owner      ArmingOwner
	Generation int64
	Inspection ArmedInspection
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
		(owner.PidStartTicks == 0) != (owner.BootID == "") {
		return owner, fmt.Errorf("supervision owner record is incomplete")
	}
	return owner, nil
}

// WriteArmingOwner publishes the owner identity atomically after the caller
// has acquired lock.d.
func WriteArmingOwner(root string, owner ArmingOwner) error {
	if owner.Pid < 1 || owner.PidStartedAt < 1 || owner.InstanceTag == "" ||
		(owner.PidStartTicks == 0) != (owner.BootID == "") {
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

func stopOwner(root string, owner ArmingOwner, scaleMilli int, requester string) error {
	if armingOwnerLiveness(owner) == identity.Dead {
		return nil
	}
	if armingOwnerLiveness(owner) == identity.Unknown {
		return fmt.Errorf("owner identity is uninspectable; replacement is not authorized")
	}
	if err := writeShutdownIntent(root, owner, requester); err != nil {
		return fmt.Errorf("write shutdown intent: %w", err)
	}
	// Re-authenticate immediately beside signalling: the replacement caller
	// did not spawn this owner and the shutdown-intent write created a race
	// window in which the recorded pid could have changed.
	switch armingOwnerLiveness(owner) {
	case identity.Dead:
		return nil
	case identity.Unknown:
		return fmt.Errorf("owner identity became uninspectable before signalling; replacement is not authorized")
	}
	if err := signalGroup(owner.Pid, syscall.SIGTERM); err != nil {
		return fmt.Errorf("signal owner: %w", err)
	}
	deadline := time.Now().Add(scaledWait(5, scaleMilli))
	for time.Now().Before(deadline) {
		if armingOwnerLiveness(owner) == identity.Dead {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	switch armingOwnerLiveness(owner) {
	case identity.Dead:
		return nil
	case identity.Unknown:
		return fmt.Errorf("owner identity became uninspectable before SIGKILL; replacement is not authorized")
	}
	if err := signalGroup(owner.Pid, syscall.SIGKILL); err != nil {
		return fmt.Errorf("kill owner: %w", err)
	}
	deadline = time.Now().Add(scaledWait(1, scaleMilli))
	for time.Now().Before(deadline) {
		if armingOwnerLiveness(owner) == identity.Dead {
			return nil
		}
		time.Sleep(20 * time.Millisecond)
	}
	return fmt.Errorf("owner pid %d did not stop", owner.Pid)
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

func stopRecordedComponent(control recordedComponentControl, held Held, scaleMilli int) error {
	if err := authenticateRecordedComponent(control, held); err != nil {
		if errors.Is(err, errRecordedComponentGone) {
			return nil
		}
		return err
	}
	// The identity and tag proof is immediately adjacent to this signal.
	// These detached processes were not spawned by this up invocation.
	if err := control.signalGroup(held.Identity.Pid, syscall.SIGTERM); err != nil {
		return fmt.Errorf("signal recorded %s process group: %w", held.Component, err)
	}
	if absent, err := waitForRecordedGroupAbsence(control, held.Identity.Pid, scaledWait(5, scaleMilli)); err != nil {
		return fmt.Errorf("prove recorded %s process group stopped: %w", held.Component, err)
	} else if absent {
		return nil
	}
	// Re-authenticate beside escalation. If the leader exited or changed
	// identity while its group survived, custody is no longer sufficient to
	// risk signalling whatever now occupies the recorded group id.
	if err := authenticateRecordedComponent(control, held); err != nil {
		return fmt.Errorf("refuse SIGKILL for %s: %w", held.Component, err)
	}
	if err := control.signalGroup(held.Identity.Pid, syscall.SIGKILL); err != nil {
		return fmt.Errorf("kill recorded %s process group: %w", held.Component, err)
	}
	absent, err := waitForRecordedGroupAbsence(control, held.Identity.Pid, scaledWait(1, scaleMilli))
	if err != nil {
		return fmt.Errorf("prove recorded %s process group killed: %w", held.Component, err)
	}
	if !absent {
		return fmt.Errorf("recorded %s process group %d did not stop", held.Component, held.Identity.Pid)
	}
	return nil
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
		held = append(held, Held{Component: component, Tag: tag, Identity: identity.Ref{
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

func takeoverComponents(root, metasystemRoot, ownerTag string) ([]Held, error) {
	held, err := recordedHeld(root)
	if err != nil {
		return nil, err
	}
	processes, err := enumerateTakeoverProcesses(metasystemRoot)
	if err != nil {
		return nil, fmt.Errorf("enumerate tagged supervision components: %w", err)
	}
	discovered, err := taggedTakeoverComponents(root, ownerTag, processes)
	if err != nil {
		return nil, err
	}
	byPid := map[int64]Held{}
	for _, component := range append(held, discovered...) {
		if prior, exists := byPid[component.Identity.Pid]; exists {
			if prior.Tag != component.Tag || prior.Component != component.Component || prior.Identity != component.Identity {
				return nil, fmt.Errorf("supervision component pid %d has conflicting recorded and census identities", component.Identity.Pid)
			}
			continue
		}
		byPid[component.Identity.Pid] = component
	}
	held = held[:0]
	for _, component := range byPid {
		held = append(held, component)
	}
	return held, nil
}

func stopTakeoverComponents(root, metasystemRoot, ownerTag string, scaleMilli int) error {
	held, err := takeoverComponents(root, metasystemRoot, ownerTag)
	if err != nil {
		return err
	}
	control := recordedComponentControl{
		prober: identity.KernelProber{}, groupAbsent: kernelGroupAbsent,
		signalGroup: func(pgid int64, signal syscall.Signal) error {
			if err := syscall.Kill(-int(pgid), signal); err != nil && err != syscall.ESRCH {
				return err
			}
			return nil
		},
	}
	for _, component := range held {
		if err := stopRecordedComponent(control, component, scaleMilli); err != nil {
			name := "repo-watcher"
			if component.Component == Reaper {
				name = "job-reaper"
			}
			return &ComponentFailure{Component: name, Err: err}
		}
	}
	return nil
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
		AcquiredAt: time.Now().UTC().Format(time.RFC3339)}
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

func requireOwnerCheckout(requestedRoot, stateRoot, expectedTagPrefix string, owner ArmingOwner) error {
	requestedCheckout := canonicalPathForTakeover(requestedRoot)
	stateCheckout := canonicalPathForTakeover(stateRoot)
	registryPath, err := registry.DefaultPath()
	if err != nil {
		return fmt.Errorf("resolve supervision registry for checkout %q: %w", requestedCheckout, err)
	}
	recordedCheckout, found, err := registry.OwnerCheckoutPath(registryPath, owner.InstanceTag)
	if err != nil {
		return fmt.Errorf("read supervision registry before acting for checkout %q: %w", requestedCheckout, err)
	}
	if found {
		recordedCheckout = canonicalPathForTakeover(recordedCheckout)
		if recordedCheckout != requestedCheckout {
			return fmt.Errorf("supervision request for checkout %q names owner tag %q recorded for checkout %q; refusing to act on another repository", requestedCheckout, owner.InstanceTag, recordedCheckout)
		}
	} else if stateCheckout != requestedCheckout {
		// An owner has no registry row during the bounded interval before
		// its first write-ahead launch record. The lock's canonical state
		// root is the only checkout path available in that interval. It may
		// authorize only the same canonical checkout, never a shared
		// installation serving another requested scope.
		return fmt.Errorf("supervision request for checkout %q found unregistered owner tag %q under state checkout %q; refusing to act on another repository", requestedCheckout, owner.InstanceTag, stateCheckout)
	}
	// Tags never select a victim. A mismatch can only veto the checkout-path
	// selection made above, including during the pre-registry interval.
	if !strings.HasPrefix(owner.InstanceTag, expectedTagPrefix) {
		return fmt.Errorf("supervision request for checkout %q names owner tag %q outside that checkout's prefix %q; refusing to act on another repository", requestedCheckout, owner.InstanceTag, expectedTagPrefix)
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
	if options.Root == "" || options.MetasystemRoot == "" || options.Scope == "" || (options.Binary == "" && options.Command == nil) || options.Fingerprint == "" || options.IntervalSec < 1 || options.WatcherCap < 1 || options.OwnerTagPrefix == "" {
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
		if err := requireOwnerCheckout(options.Root, options.Root, options.OwnerTagPrefix, owner); err != nil {
			return EnsureResult{}, err
		}
		switch armingOwnerLiveness(owner) {
		case identity.Unknown:
			return EnsureResult{}, fmt.Errorf("supervision owner pid %d is uninspectable; takeover is not authorized", owner.Pid)
		case identity.Dead:
			if err := requireCeilingClear(options.Root, options.MetasystemRoot, options.WatcherCap); err != nil {
				return EnsureResult{}, err
			}
			if err := stopTakeoverComponents(options.Root, options.MetasystemRoot, owner.InstanceTag, options.WaitScaleMilli); err != nil {
				return EnsureResult{}, fmt.Errorf("dead-owner takeover refused: %w", err)
			}
			if err := releaseDeadOwnerLock(options.Root, owner); err != nil {
				continue
			}
			action = "taken-over"
			continue
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
		if err := stopOwner(options.Root, owner, options.WaitScaleMilli, "metasystem up engine-generation replacement"); err != nil {
			return EnsureResult{}, err
		}
		if err := stopTakeoverComponents(options.Root, options.MetasystemRoot, owner.InstanceTag, options.WaitScaleMilli); err != nil {
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
func Shutdown(root, expectedTagPrefix string, scaleMilli int) error {
	return ShutdownAt(root, root, expectedTagPrefix, scaleMilli)
}

// ShutdownAt stops repository supervision while using the installed
// metasystem's authorized census source for the final tag sweep.
func ShutdownAt(root, metasystemRoot, expectedTagPrefix string, scaleMilli int) error {
	owner, err := ReadArmingOwner(root)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := requireOwnerCheckout(root, root, expectedTagPrefix, owner); err != nil {
		return err
	}
	if err := stopOwner(root, owner, scaleMilli, "metasystem up shutdown"); err != nil {
		return err
	}
	if err := stopTakeoverComponents(root, metasystemRoot, owner.InstanceTag, scaleMilli); err != nil {
		return err
	}
	return releaseDeadOwnerLock(root, owner)
}
