package proofrun

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

type ResourceCustodyOptions struct {
	Watchdog   WatchdogOptions
	Launcher   identity.Ref
	ControlFD  int
	ReadyFD    int
	MarkerFD   int
	MarkerPath string
	ConfPath   string
}

type resourceCustodyEngineKey struct{}

// WithResourceCustodyExecutable supplies an already-built engine for direct
// callers. Ordinary CLI calls use their own executable and identical custody.
func WithResourceCustodyExecutable(ctx context.Context, executable string) context.Context {
	return context.WithValue(ctx, resourceCustodyEngineKey{}, executable)
}

func resourceCustodyExecutable(ctx context.Context) (string, error) {
	if executable, _ := ctx.Value(resourceCustodyEngineKey{}).(string); executable != "" {
		if !filepath.IsAbs(executable) {
			return "", fmt.Errorf("resource custodian executable must be absolute")
		}
		return executable, nil
	}
	return os.Executable()
}

type resourceCustodyMessage struct {
	Kind  string       `json:"kind"`
	Suite identity.Ref `json:"suite,omitempty"`
}

type resourceCustodyEvent struct {
	message resourceCustodyMessage
	err     error
}

// RunResourceCustodian is the resource-active mode of the existing proof-run
// watchdog. Its own live process anchors the native group before the first
// workload is born. It keeps inherited lease descriptors until the group is
// proven empty, including when the launcher dies or loses the handshake.
func RunResourceCustodian(options ResourceCustodyOptions) error {
	self := os.Getpid()
	group, err := syscall.Getpgid(self)
	if err != nil || group != self || options.ControlFD < 3 || options.ReadyFD < 3 {
		return fmt.Errorf("resource custodian group or handshake is invalid: %v", err)
	}
	prober := identity.KernelProber{}
	actualParent, parentState := int64(os.Getppid()), identity.AliveRef(prober, options.Launcher)
	if options.Launcher.Pid != actualParent || parentState != identity.Alive {
		return fmt.Errorf("resource custodian launcher is not its exact live parent: parent=%d expected=%d identity=%s",
			actualParent, options.Launcher.Pid, parentState)
	}
	control := os.NewFile(uintptr(options.ControlFD), "resource-custody-control")
	ready := os.NewFile(uintptr(options.ReadyFD), "resource-custody-ready")
	defer control.Close()
	defer ready.Close()
	if _, err := io.WriteString(ready, "ready\n"); err != nil {
		return err
	}
	_ = ready.Close()
	// One channel preserves the order of bind, done, and terminal EOF. Two
	// channels let a fast command's EOF race ahead of its queued bind.
	events := make(chan resourceCustodyEvent, 3)
	go func() {
		decoder := json.NewDecoder(control)
		for {
			var message resourceCustodyMessage
			if err := decoder.Decode(&message); err != nil {
				events <- resourceCustodyEvent{err: err}
				return
			}
			events <- resourceCustodyEvent{message: message}
		}
	}()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	var suite identity.Ref
	var watched <-chan error
	var policyError error
	var reason error
	for reason == nil {
		select {
		case event := <-events:
			if event.err != nil {
				if errors.Is(event.err, io.EOF) {
					reason = fmt.Errorf("resource launcher lost its custody channel")
				} else {
					reason = fmt.Errorf("resource custody channel: %w", event.err)
				}
				break
			}
			switch event.message.Kind {
			case "bind":
				if suite.Pid != 0 || event.message.Suite.Pid < 1 || event.message.Suite.Mode() == identity.CompareInvalid {
					reason = fmt.Errorf("resource custodian received an invalid suite binding")
					break
				}
				suite = event.message.Suite
				if options.Watchdog.Suite != "" {
					options.Watchdog.SuiteIdentity = suite
					options.Watchdog.CustodyGroup = int64(self)
					channel := make(chan error, 1)
					watched = channel
					go func() { channel <- RunWatchdog(options.Watchdog) }()
				}
			case "done":
				reason = io.EOF
			default:
				reason = fmt.Errorf("resource custodian received an unknown message")
			}
		case err := <-watched:
			policyError = err
			watched = nil
			reason = io.EOF
		case <-ticker.C:
			if options.Launcher.Pid > 0 && identity.AliveRef(prober, options.Launcher) != identity.Alive {
				reason = fmt.Errorf("resource launcher exact identity was lost")
			} else if suite.Pid > 0 {
				ended, suiteErr := resourceCustodySuiteEnded(prober, suite)
				if suiteErr != nil {
					reason = suiteErr
				} else if ended {
					reason = io.EOF
				} else if options.Watchdog.DonePath != "" && suiteDone(options.Watchdog.DonePath) {
					reason = io.EOF
				}
			}
		}
	}
	// This also unblocks the progress watchdog when the launcher disappears.
	// The watchdog may still be preserving evidence or shutting supervision
	// down after its final done-file check. It must stop creating processes
	// before the final group and detached-fixture censuses begin.
	watchdogBound := 35*time.Second + options.Watchdog.EvidenceTimeout + options.Watchdog.TermGrace + options.Watchdog.KillGrace
	watcherSettled, settleErr := settleResourceWatchdog(watched, options.Watchdog.DonePath, watchdogBound)
	policyError = errors.Join(policyError, settleErr)
	survivors, drainErr := drainResourceGroup(int64(self), prober, 2*time.Second)
	fixtureSurvivors := false
	var fixtureErr error
	if drainErr == nil && options.ConfPath != "" {
		fixtureSurvivors, fixtureErr = drainCustodyFixtures(options, prober)
	}
	if watcherSettled && drainErr == nil && fixtureErr == nil && options.MarkerFD >= 3 && options.MarkerPath != "" {
		marker := os.NewFile(uintptr(options.MarkerFD), options.MarkerPath)
		if marker == nil {
			fixtureErr = fmt.Errorf("resource custodian marker descriptor is invalid")
		} else {
			fixtureErr = MarkHostResourcesCleanCustodian([]*os.File{marker}, options.Launcher)
			_ = marker.Close()
		}
	}
	if reason == io.EOF {
		reason = nil
	}
	if survivors {
		reason = errors.Join(reason, fmt.Errorf("native descendants survived direct worker completion"))
	}
	if fixtureSurvivors {
		reason = errors.Join(reason, fmt.Errorf("declared fixture descendants survived direct worker completion"))
	}
	return errors.Join(reason, policyError, drainErr, fixtureErr)
}

func resourceCustodySuiteEnded(prober identity.Prober, suite identity.Ref) (bool, error) {
	exact, state, err := prober.Probe(suite.Pid)
	if err != nil || state == identity.Unknown {
		return false, fmt.Errorf("resource suite exact identity is uninspectable: %v", err)
	}
	if state == identity.Dead {
		return true, nil
	}
	return !identity.SameIdentity(exact, suite) || exact.Zombie || exact.Exiting, nil
}

func settleResourceWatchdog(watched <-chan error, donePath string, bound time.Duration) (bool, error) {
	var doneErr error
	if donePath != "" {
		doneErr = touchDone(donePath)
	}
	if watched == nil {
		return doneErr == nil, doneErr
	}
	select {
	case err := <-watched:
		return doneErr == nil, errors.Join(doneErr, err)
	case <-time.After(bound):
		return false, errors.Join(doneErr, fmt.Errorf("progress watchdog did not settle before final custody drain"))
	}
}

func drainCustodyFixtures(options ResourceCustodyOptions, prober identity.Prober) (bool, error) {
	return drainCustodyFixtureScans(options, prober, func() ([]identity.FixtureSurvivor, error) {
		processes, err := census.EnumerateConfiguredProcesses(filepath.Dir(options.ConfPath))
		if err != nil {
			return nil, fmt.Errorf("resource custodian fixture census: %w", err)
		}
		// The launcher is normally still alive while blocked in custody.finish.
		// A whole-table survivor scan hides its tagged fixtures, so use the
		// exact owner-scoped view and retain the attempt filter below.
		survivors, err := census.ScanFixtureSurvivors(prober, processes,
			census.FixtureSurvivorSelection{Owner: &options.Launcher, IncludeLiveOwner: true})
		if err != nil {
			return nil, fmt.Errorf("resource custodian fixture survivor scan: %w", err)
		}
		return survivors, nil
	})
}

func drainCustodyFixtureScans(options ResourceCustodyOptions, prober identity.Prober, scan func() ([]identity.FixtureSurvivor, error)) (bool, error) {
	deadline := time.Now().Add(2 * time.Second)
	observed := false
	empty := 0
	for time.Now().Before(deadline) {
		survivors, err := scan()
		if err != nil {
			return observed, err
		}
		owned := 0
		for _, survivor := range survivors {
			if !fixtureSurvivorOwnedByAttempt(prober, survivor, fixtureAttemptOwnership{owner: options.Launcher, attemptID: options.Watchdog.AttemptID}) {
				continue
			}
			owned++
			if survivor.Class != identity.FixtureSurvivorCertain {
				return observed, fmt.Errorf("owned fixture survivor %d has uncertain identity", survivor.Ref.Pid)
			}
			observed = true
			if err := identity.SignalExact(prober, survivor.Ref, syscall.SIGKILL, syscall.Kill); err != nil && !errors.Is(err, identity.ErrGone) {
				return observed, fmt.Errorf("stop owned fixture survivor %d: %w", survivor.Ref.Pid, err)
			}
			for {
				exact, state, err := prober.Probe(survivor.Ref.Pid)
				if err != nil || state == identity.Unknown {
					return observed, fmt.Errorf("fixture survivor %d cleanup is uninspectable: %v", survivor.Ref.Pid, err)
				}
				if state == identity.Dead || !identity.SameIdentity(exact, survivor.Ref) || exact.Zombie {
					break
				}
				if time.Now().After(deadline) {
					return observed, fmt.Errorf("fixture survivor %d did not stop", survivor.Ref.Pid)
				}
				time.Sleep(50 * time.Millisecond)
			}
		}
		if owned == 0 {
			empty++
			if empty >= 2 {
				return observed, nil
			}
		} else {
			empty = 0
		}
		time.Sleep(50 * time.Millisecond)
	}
	return observed, fmt.Errorf("declared fixture custody did not drain within 2s")
}

// drainResourceGroup signals only identities actually observed in the live
// custodian group. Two complete empty censuses while its leader is still live
// are required; unknown membership or an uninspectable identity fails closed.
func drainResourceGroup(group int64, prober identity.Prober, limit time.Duration) (bool, error) {
	return drainResourceGroupWith(group, prober, limit, custodyGroupMembers, syscall.Kill, time.Now, func() {
		time.Sleep(50 * time.Millisecond)
	})
}

func drainResourceGroupWith(group int64, prober identity.Prober, limit time.Duration,
	groupMembers func(int64) ([]int64, error), signal identity.SignalFunc, now func() time.Time, pause func(),
) (bool, error) {
	deadline := now().Add(limit)
	observed := false
	empty := 0
	for now().Before(deadline) {
		pids, err := groupMembers(group)
		if err != nil {
			return observed, err
		}
		members := make([]identity.Exact, 0, len(pids))
		for _, pid := range pids {
			exact, state, err := prober.Probe(pid)
			if err != nil || state == identity.Unknown {
				return observed, fmt.Errorf("resource group member %d is uninspectable: %v", pid, err)
			}
			if state == identity.Alive && !exact.Zombie && !exact.Exiting {
				members = append(members, exact)
			}
		}
		if len(members) == 0 {
			empty++
			if empty >= 2 {
				return observed, nil
			}
			pause()
			continue
		}
		empty = 0
		observed = true
		for _, member := range members {
			if err := identity.SignalExact(prober, member.Ref(), syscall.SIGKILL, signal); err != nil && !errors.Is(err, identity.ErrGone) {
				return observed, fmt.Errorf("stop resource group member %d: %w", member.Pid, err)
			}
		}
		pause()
	}
	return observed, fmt.Errorf("resource group %d did not drain within %s", group, limit)
}

type resourceCustody struct {
	command    *exec.Cmd
	control    *os.File
	group      int
	leader     identity.Ref
	spools     *custodySpools
	diagnostic *bytes.Buffer
}

type custodySpools struct {
	stdout        *os.File
	stderr        *os.File
	followDone    chan struct{}
	followResults chan error
	finishOnce    sync.Once
	finishErr     error
}

func newCustodySpools(logPath, owner string) (*custodySpools, error) {
	stdout, err := os.CreateTemp(filepath.Dir(logPath), filepath.Base(logPath)+"."+owner+"-stdout.*")
	if err != nil {
		return nil, err
	}
	stderr, err := os.CreateTemp(filepath.Dir(logPath), filepath.Base(logPath)+"."+owner+"-stderr.*")
	if err != nil {
		_ = stdout.Close()
		_ = os.Remove(stdout.Name())
		return nil, err
	}
	return &custodySpools{stdout: stdout, stderr: stderr}, nil
}

func (spools *custodySpools) close() {
	if spools != nil {
		_ = spools.stdout.Close()
		_ = spools.stderr.Close()
	}
}

func (spools *custodySpools) follow(stdout, stderr io.Writer, log *os.File) {
	spools.followDone = make(chan struct{})
	spools.followResults = make(chan error, 2)
	go func() {
		spools.followResults <- followCustodySpool(spools.stdout.Name(), stdout, log, spools.followDone)
	}()
	go func() {
		spools.followResults <- followCustodySpool(spools.stderr.Name(), stderr, log, spools.followDone)
	}()
}

func (spools *custodySpools) finish() error {
	if spools == nil {
		return nil
	}
	spools.finishOnce.Do(func() {
		spools.close()
		if spools.followDone != nil {
			close(spools.followDone)
			spools.finishErr = errors.Join(<-spools.followResults, <-spools.followResults)
		}
	})
	return spools.finishErr
}

func startResourceCustody(options LaunchOptions, parent identity.Ref, donePath string, files []*os.File, spools *custodySpools) (*resourceCustody, error) {
	controlRead, controlWrite, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	readyRead, readyWrite, err := os.Pipe()
	if err != nil {
		controlRead.Close()
		controlWrite.Close()
		return nil, err
	}
	cleanPipes := func() { controlRead.Close(); controlWrite.Close(); readyRead.Close(); readyWrite.Close() }
	command := watchdogCommand(options, identity.Ref{}, donePath, 0)
	if options.Suite == "" {
		engine := options.WatchdogExecutable
		if engine == "" {
			engine, err = os.Executable()
			if err != nil {
				cleanPipes()
				return nil, err
			}
		}
		command = exec.Command(engine, "proof-run", "watchdog")
	}
	command.Args = append(command.Args, "--resource-custody", "--custody-parent-pid", strconv.FormatInt(parent.Pid, 10),
		"--custody-parent-started-at", strconv.FormatInt(parent.StartedAtSec, 10),
		"--custody-parent-start-micro", strconv.FormatInt(parent.StartedAtUnixMicro, 10),
		"--custody-parent-start-ticks", strconv.FormatInt(parent.StartTicks, 10), "--custody-parent-boot-id", parent.BootID,
		"--custody-control-fd", strconv.Itoa(3+len(files)), "--custody-ready-fd", strconv.Itoa(4+len(files)))
	for index, file := range files {
		if strings.HasPrefix(filepath.Base(file.Name()), "lease-") {
			command.Args = append(command.Args, "--custody-marker-fd", strconv.Itoa(3+index), "--custody-marker-path", file.Name())
			break
		}
	}
	command.ExtraFiles = append(append(append([]*os.File{}, files...), controlRead), readyWrite)
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	// A launcher can die while the custodian is proving cleanup. Its stdio
	// therefore belongs to durable regular files, never parent-owned pipes.
	if spools != nil {
		command.Stdout, command.Stderr = spools.stdout, spools.stderr
	} else if options.LogPath != "" {
		custodyLog, openErr := os.OpenFile(options.LogPath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600)
		if openErr != nil {
			cleanPipes()
			return nil, openErr
		}
		defer custodyLog.Close()
		command.Stdout, command.Stderr = custodyLog, custodyLog
	} else {
		diagnostic := &bytes.Buffer{}
		command.Stdout, command.Stderr = io.Discard, diagnostic
		defer func() {
			if command.Process == nil {
				diagnostic.Reset()
			}
		}()
	}
	if err := command.Start(); err != nil {
		cleanPipes()
		spools.close()
		return nil, err
	}
	spools.close()
	controlRead.Close()
	readyWrite.Close()
	readyResult := make(chan error, 1)
	go func() {
		line, err := bufio.NewReader(readyRead).ReadString('\n')
		if err == nil && line != "ready\n" {
			err = fmt.Errorf("resource custodian readiness is invalid")
		}
		readyResult <- err
	}()
	select {
	case err := <-readyResult:
		if err != nil {
			controlWrite.Close()
			readyRead.Close()
			_ = command.Process.Kill()
			_ = command.Wait()
			return nil, err
		}
	case <-time.After(5 * time.Second):
		controlWrite.Close()
		readyRead.Close()
		_ = command.Process.Kill()
		_ = command.Wait()
		return nil, fmt.Errorf("resource custodian did not become ready")
	}
	readyRead.Close()
	leader, state, probeErr := (identity.KernelProber{}).Probe(int64(command.Process.Pid))
	if probeErr != nil || state != identity.Alive {
		controlWrite.Close()
		_ = command.Process.Kill()
		_ = command.Wait()
		return nil, fmt.Errorf("resource custodian exact identity unavailable: %v", probeErr)
	}
	var diagnostic *bytes.Buffer
	if buffer, ok := command.Stderr.(*bytes.Buffer); ok {
		diagnostic = buffer
	}
	return &resourceCustody{command: command, control: controlWrite, group: command.Process.Pid,
		leader: leader.Ref(), spools: spools, diagnostic: diagnostic}, nil
}

func followCustodySpool(path string, output io.Writer, log *os.File, done <-chan struct{}) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	buffer := make([]byte, 32*1024)
	var outputErr, logErr error
	finalEnd := int64(-1)
	var offset int64
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		if finalEnd < 0 {
			select {
			case <-done:
				info, statErr := file.Stat()
				if statErr != nil {
					return errors.Join(outputErr, logErr, statErr)
				}
				finalEnd = info.Size()
			default:
			}
		}
		if finalEnd >= 0 && offset >= finalEnd {
			return errors.Join(outputErr, logErr)
		}
		chunk := buffer
		if finalEnd >= 0 && finalEnd-offset < int64(len(chunk)) {
			chunk = chunk[:finalEnd-offset]
		}
		n, readErr := file.Read(chunk)
		if n > 0 {
			offset += int64(n)
			chunk := chunk[:n]
			if outputErr == nil {
				if _, err := output.Write(chunk); err != nil {
					outputErr = err
				}
			} else {
				_, err := log.Write(chunk)
				logErr = errors.Join(logErr, err)
			}
		}
		if readErr == io.EOF {
			if finalEnd >= 0 {
				return errors.Join(outputErr, logErr, fmt.Errorf("output spool %s truncated before its final byte %d", path, finalEnd))
			}
			select {
			case <-done:
			case <-ticker.C:
			}
			continue
		}
		if readErr != nil {
			return errors.Join(outputErr, logErr, readErr)
		}
	}
}

func (custody *resourceCustody) bind(ref identity.Ref) error {
	return json.NewEncoder(custody.control).Encode(resourceCustodyMessage{Kind: "bind", Suite: ref})
}

func (custody *resourceCustody) signal(signal syscall.Signal, prober identity.Prober, sender identity.SignalFunc) []identity.Ref {
	return signalResourceGroupWith(int64(custody.group), custody.leader, signal, prober, custodyGroupMembers, sender)
}

func signalResourceGroupWith(group int64, leader identity.Ref, signal syscall.Signal, prober identity.Prober,
	groupMembers func(int64) ([]int64, error), sender identity.SignalFunc,
) []identity.Ref {
	if identity.AliveRef(prober, leader) != identity.Alive {
		return nil
	}
	pids, err := groupMembers(group)
	if err != nil {
		return nil
	}
	refs := make([]identity.Ref, 0, len(pids))
	for _, pid := range pids {
		if pid == leader.Pid {
			continue
		}
		exact, state, probeErr := prober.Probe(pid)
		if probeErr != nil || state != identity.Alive || exact.Zombie || exact.Exiting {
			continue
		}
		if exact.EnvironKnown && containsExactEnvironmentEntry(exact.Environ, identity.FixtureCustodianEnv+"=1") {
			continue
		}
		refs = append(refs, exact.Ref())
	}
	if identity.AliveRef(prober, leader) != identity.Alive {
		return nil
	}
	signaled := make([]identity.Ref, 0, len(refs))
	for index := len(refs) - 1; index >= 0; index-- {
		if err := identity.SignalExact(prober, refs[index], signal, sender); err != nil && !errors.Is(err, identity.ErrGone) {
			continue
		}
		signaled = append(signaled, refs[index])
	}
	return signaled
}

func (custody *resourceCustody) finish() error {
	_ = json.NewEncoder(custody.control).Encode(resourceCustodyMessage{Kind: "done"})
	_ = custody.control.Close()
	err := custody.command.Wait()
	if err != nil && custody.diagnostic != nil && strings.TrimSpace(custody.diagnostic.String()) != "" {
		err = fmt.Errorf("%w: %s", err, strings.TrimSpace(custody.diagnostic.String()))
	}
	return errors.Join(err, custody.spools.finish())
}

type custodyBarrier struct {
	ready   *os.File
	release *os.File
}

func prepareCustodyExec(command *exec.Cmd, engine string) (*custodyBarrier, error) {
	readyRead, readyWrite, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	releaseRead, releaseWrite, err := os.Pipe()
	if err != nil {
		readyRead.Close()
		readyWrite.Close()
		return nil, err
	}
	original := append([]string(nil), command.Args...)
	if len(original) == 0 {
		original = []string{command.Path}
	}
	selectedPath := command.Path
	index := len(command.ExtraFiles)
	command.Path = engine
	command.Args = append([]string{engine, "proof-run", "custody-exec", "--ready-fd", strconv.Itoa(3 + index),
		"--release-fd", strconv.Itoa(4 + index), "--path", selectedPath, "--"}, original...)
	command.ExtraFiles = append(command.ExtraFiles, readyWrite, releaseRead)
	return &custodyBarrier{ready: readyRead, release: releaseWrite}, nil
}

func (barrier *custodyBarrier) await() error {
	result := make(chan error, 1)
	go func() {
		line, err := bufio.NewReader(barrier.ready).ReadString('\n')
		if err == nil && line != "ready\n" {
			err = fmt.Errorf("resource worker readiness is invalid")
		}
		result <- err
	}()
	select {
	case err := <-result:
		return err
	case <-time.After(5 * time.Second):
		return fmt.Errorf("resource worker did not reach its start barrier")
	}
}

func (barrier *custodyBarrier) releaseWork() error {
	_, err := barrier.release.Write([]byte{1})
	return errors.Join(err, barrier.release.Close())
}

func (barrier *custodyBarrier) close() {
	_ = barrier.ready.Close()
	_ = barrier.release.Close()
}

// RunResourceCommand executes one preparation or discovery command under the
// same pre-birth custodian as native proof. The caller owns the lease and its
// output writers; this call returns only after ordinary descendants drain.
func RunResourceCommand(ctx context.Context, command *exec.Cmd, lease *HostResourceLease) error {
	return runResourceCommand(ctx, command, lease, nil)
}

type resourceCommandCompletion string

const (
	resourceCommandCompleted resourceCommandCompletion = "completed"
	resourceCommandCancelled resourceCommandCompletion = "cancelled"
)

type resourceCommandEvents struct {
	afterWait func(error)
	selected  func(resourceCommandCompletion)
}

func runResourceCommand(ctx context.Context, command *exec.Cmd, lease *HostResourceLease, events *resourceCommandEvents) error {
	finish, _, _, err := startResourceCommand(ctx, command, lease)
	if err != nil {
		return err
	}
	waited := make(chan error, 1)
	go func() {
		waitErr := command.Wait()
		if events != nil && events.afterWait != nil {
			events.afterWait(waitErr)
		}
		waited <- waitErr
	}()
	var commandErr error
	select {
	case commandErr = <-waited:
		if events != nil && events.selected != nil {
			events.selected(resourceCommandCompleted)
		}
	case <-ctx.Done():
		if events != nil && events.selected != nil {
			events.selected(resourceCommandCancelled)
		}
		custodyErr := finish()
		commandErr = errors.Join(ctx.Err(), <-waited)
		if custodyErr != nil && commandErr == nil {
			return fmt.Errorf("resource command custody: %w", custodyErr)
		}
		return errors.Join(commandErr, custodyErr)
	}
	custodyErr := finish()
	if custodyErr != nil && commandErr == nil {
		return fmt.Errorf("resource command custody: %w", custodyErr)
	}
	return errors.Join(commandErr, custodyErr)
}

// startResourceCommand leaves command.Process as the exact worker PID, so the
// ordinary CPU/output supervisor can continue sampling it. Its finish
// closure may run before command.Wait to make owned cleanup close inherited
// output; it must run exactly once on every successful start.
func startResourceCommand(ctx context.Context, command *exec.Cmd, lease *HostResourceLease) (func() error, func(syscall.Signal, identity.Prober, identity.SignalFunc) []identity.Ref, identity.Ref, error) {
	if command == nil {
		return nil, nil, identity.Ref{}, fmt.Errorf("resource command is nil")
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, identity.Ref{}, err
	}
	engine, err := resourceCustodyExecutable(ctx)
	if err != nil {
		return nil, nil, identity.Ref{}, err
	}
	parent, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		return nil, nil, identity.Ref{}, fmt.Errorf("resource launcher identity unavailable: %v", err)
	}
	var files []*os.File
	if lease != nil {
		files = lease.Files()
	}
	if len(files) != 0 {
		if err := MarkHostResourcesDirty(files); err != nil {
			return nil, nil, identity.Ref{}, err
		}
	}
	custody, err := startResourceCustody(LaunchOptions{WatchdogExecutable: engine}, parent.Ref(), "", files, nil)
	if err != nil {
		if len(files) != 0 {
			_ = MarkHostResourcesClean(files)
		}
		return nil, nil, identity.Ref{}, err
	}
	originalPath, originalArgs := command.Path, append([]string(nil), command.Args...)
	originalExtra := append([]*os.File(nil), command.ExtraFiles...)
	barrier, err := prepareCustodyExec(command, engine)
	finish := func() error {
		command.Path, command.Args, command.ExtraFiles = originalPath, originalArgs, originalExtra
		if barrier != nil {
			barrier.close()
		}
		custodyErr := custody.finish()
		if custodyErr == nil && len(files) != 0 {
			custodyErr = MarkHostResourcesClean(files)
		}
		return custodyErr
	}
	if err != nil {
		return nil, nil, identity.Ref{}, errors.Join(err, finish())
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, identity.Ref{}, errors.Join(err, finish())
	}
	attributes := syscall.SysProcAttr{}
	if command.SysProcAttr != nil {
		attributes = *command.SysProcAttr
	}
	attributes.Setpgid, attributes.Pgid = true, custody.group
	command.SysProcAttr = &attributes
	if err := command.Start(); err != nil {
		return nil, nil, identity.Ref{}, errors.Join(err, finish())
	}
	for _, file := range command.ExtraFiles[len(command.ExtraFiles)-2:] {
		_ = file.Close()
	}
	abort := func(err error) (func() error, func(syscall.Signal, identity.Prober, identity.SignalFunc) []identity.Ref, identity.Ref, error) {
		custodyErr := finish()
		return nil, nil, identity.Ref{}, errors.Join(err, command.Wait(), custodyErr)
	}
	if err := barrier.await(); err != nil {
		return abort(err)
	}
	exact, state, err := (identity.KernelProber{}).Probe(int64(command.Process.Pid))
	if err != nil || state != identity.Alive {
		return abort(fmt.Errorf("resource worker exact identity unavailable: %v", err))
	}
	if err := custody.bind(exact.Ref()); err != nil {
		return abort(err)
	}
	if err := ctx.Err(); err != nil {
		return abort(err)
	}
	if err := barrier.releaseWork(); err != nil {
		return abort(err)
	}
	return finish, custody.signal, exact.Ref(), nil
}
