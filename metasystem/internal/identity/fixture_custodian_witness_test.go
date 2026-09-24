package identity

import (
	"errors"
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

const (
	launcherWitnessMode       = "FIXTURE_LAUNCHER_WITNESS_MODE"
	exitingWitnessMode        = "FIXTURE_EXITING_WITNESS_MODE"
	platformBinaryWitnessMode = "FIXTURE_PLATFORM_BINARY_WITNESS_MODE"
	launcherBasePath          = "PATH=/usr/bin:/bin"
	witnessCustodianPoll      = 50 * time.Millisecond
	witnessCustodianBound     = 5 * time.Second
)

func witnessEnvironment(environment []string) []string {
	return append(environment,
		FixtureCustodianPollEnv+"="+witnessCustodianPoll.String(),
		FixtureCustodianBoundEnv+"="+witnessCustodianBound.String(),
	)
}

func witnessKernelBound(poll, bound time.Duration) time.Duration {
	cleanupScans := time.Duration(custodianSettledScans + 1)
	ownerProof := bound + custodianHaltMargin + poll
	leash := fixtureCustodianLeashBound(bound) + poll
	cleanup := cleanupScans * (bound + custodianHaltMargin + poll)
	return 10 * (ownerProof + leash + cleanup)
}

func TestWitnessBoundDerivesFromCustodianTiming(t *testing.T) {
	t.Parallel()

	baseline := witnessKernelBound(50*time.Millisecond, 5*time.Second)
	cleanupScans := time.Duration(custodianSettledScans + 1)
	minimum := 10 * ((5*time.Second + custodianHaltMargin + 50*time.Millisecond) +
		(10*5*time.Second + 50*time.Millisecond) +
		cleanupScans*(5*time.Second+custodianHaltMargin+50*time.Millisecond))
	if baseline < minimum {
		t.Fatalf("witness bound %s is less than tenfold production timing %s", baseline, minimum)
	}
	if changed := witnessKernelBound(100*time.Millisecond, 7*time.Second); changed == baseline {
		t.Fatalf("witness bound stayed %s after poll and bound changed", changed)
	}
}

func TestKernelProberReportsExitingBeforeProcessDeath(t *testing.T) {
	t.Parallel()

	if os.Getenv(exitingWitnessMode) == "child" {
		_, _ = io.Copy(io.Discard, os.Stdin)
		os.Exit(0)
	}

	dir := t.TempDir()
	stderrPath := filepath.Join(dir, "exiting.stderr")
	command := exec.Command(os.Args[0], "-test.run=^"+t.Name()+"$", "-test.count=1")
	command.Env = append(os.Environ(), exitingWitnessMode+"=child")
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatalf("create exiting child stdin: %v; state=%s zombie=false exiting=false running-started-at=%s zombie-started-at=%s output=%q",
			err, Unknown, (Exact{}).StartedAt, (Exact{}).StartedAt, "")
	}
	startWitnessCommand(t, command, stderrPath, true)
	reaped := false
	t.Cleanup(func() {
		_ = stdin.Close()
		if reaped {
			return
		}
		_ = command.Process.Kill()
		_ = command.Wait()
		reaped = true
	})

	var running, zombie, observed Exact
	state := Unknown
	var probeErr error
	fail := func(message string) {
		output, _ := os.ReadFile(stderrPath)
		t.Fatalf("%s; pid=%d state=%s zombie=%t exiting=%t running-started-at=%s zombie-started-at=%s probe=%v output=%q",
			message, command.Process.Pid, state, observed.Zombie, observed.Exiting, running.StartedAt, zombie.StartedAt, probeErr, output)
	}

	observed, state, probeErr = (KernelProber{}).Probe(int64(command.Process.Pid))
	running = observed
	if probeErr != nil || state != Alive || observed.Zombie || observed.Exiting {
		fail("running child did not report Alive without zombie or exiting flags")
	}

	watch, err := armExitWatch(command.Process.Pid)
	if err != nil {
		fail(fmt.Sprintf("arm exit watch: %v", err))
	}
	if err := stdin.Close(); err != nil {
		watch.close()
		fail(fmt.Sprintf("close exiting child stdin: %v", err))
	}
	if err := watch.wait(); err != nil {
		fail(fmt.Sprintf("wait for exiting child: %v", err))
	}

	observed, state, probeErr = (KernelProber{}).Probe(int64(command.Process.Pid))
	zombie = observed
	if probeErr != nil || state != Alive || !observed.Zombie || !observed.Exiting || !observed.StartedAt.Equal(running.StartedAt) {
		fail("exited unreaped child did not report Alive with zombie and exiting flags and the original start time")
	}

	waitErr := command.Wait()
	reaped = true
	if waitErr != nil {
		fail(fmt.Sprintf("reap exiting child: %v", waitErr))
	}
	observed, state, probeErr = (KernelProber{}).Probe(int64(command.Process.Pid))
	if probeErr != nil || state != Dead {
		fail("reaped child did not report Dead")
	}
}

func TestCustodianObservesAPlatformBinaryChild(t *testing.T) {
	t.Parallel()

	if dir := os.Getenv(platformBinaryWitnessMode); dir != "" {
		_, state, err := (KernelProber{}).Probe(int64(os.Getpid()))
		if err != nil || state != Alive {
			t.Fatalf("probe platform-binary owner: state=%s err=%v", state, err)
		}
		checkWitness(t, publishWitnessPID(filepath.Join(dir, "ownerpid"), os.Getpid()))
		registry := os.Getenv("METASYSTEM_SUPERVISION_REGISTRY_HOME")
		if registry == "" {
			t.Fatal("platform-binary owner has no test registry home")
		}
		checkWitness(t, os.WriteFile(filepath.Join(dir, "logpath"), []byte(fmt.Sprintf("%s.custodian-%d.log", registry, os.Getpid())), 0o600))
		command := exec.Command("/bin/sh", "-c", "exec tail -f /dev/null")
		command.Env = slices.DeleteFunc(os.Environ(), func(entry string) bool { return strings.HasPrefix(entry, FixtureOwnerEnv+"=") })
		startWitnessCommand(t, command, filepath.Join(dir, "platform.stderr"), false)
		checkWitness(t, publishWitnessPID(filepath.Join(dir, "platformpid"), command.Process.Pid))
		refValue := liveWitnessProcessRef(t, int64(command.Process.Pid), 0)
		ref, err := ParseRef(refValue)
		checkWitness(t, err)
		t.Cleanup(func() { _ = SignalExact(KernelProber{}, ref, syscall.SIGKILL); _, _ = command.Process.Wait() })
		holdWitnessProcess(t)
		return
	}

	dir := t.TempDir()
	command := exec.Command(os.Args[0], "-test.run=^"+t.Name()+"$", "-test.count=1")
	command.Env = witnessEnvironment(append(os.Environ(), platformBinaryWitnessMode+"="+dir))
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	startWitnessCommand(t, command, filepath.Join(dir, "owner.stderr"), true)
	t.Cleanup(func() { _ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL); _, _ = command.Process.Wait() })
	owner := waitWitnessRef(t, dir, filepath.Join(dir, "ownerpid"), filepath.Join(dir, "owner.stderr"))
	child := waitWitnessRef(t, dir, filepath.Join(dir, "platformpid"), filepath.Join(dir, "owner.stderr"), filepath.Join(dir, "platform.stderr"))
	t.Cleanup(func() { _ = SignalExact(KernelProber{}, child, syscall.SIGKILL) })
	custodian := waitWitnessCustodian(t, dir, owner)
	t.Cleanup(func() { _ = SignalExact(KernelProber{}, custodian, syscall.SIGKILL) })
	logPath, err := os.ReadFile(filepath.Join(dir, "logpath"))
	checkWitness(t, err)
	t.Cleanup(func() { _ = os.Remove(string(logPath)) })
	waitWitnessCustodianObserved(t, dir, string(logPath), child)
}

func TestLauncherDeathKillsTheTest(t *testing.T) {
	if !runLauncherWitnessMode(t) {
		runLauncherDeathWitness(t, false)
	}
}

func TestCustodianWatchesTheExportedLauncher(t *testing.T) {
	if runLauncherWitnessMode(t) {
		return
	}
	dead, err := EncodeRef(fixtureExact(1<<30, 1).Ref())
	checkWitness(t, err)
	for _, value := range []string{"malformed", dead, launcherProcessRef(t)} {
		t.Run(value, func(t *testing.T) {
			sink := filepath.Join(t.TempDir(), "signal-sink")
			dir := filepath.Dir(sink)
			checkWitness(t, os.WriteFile(sink, nil, 0o600))
			command := exec.Command(os.Args[0], "-test.run=^TestCustodianWatchesTheExportedLauncher$", "-test.count=1")
			command.Env = witnessEnvironment([]string{launcherBasePath, launcherWitnessMode + "=owner|" + dir, RunOwnerEnv + "=" + value})
			output, runErr := command.CombinedOutput()
			contents, _ := os.ReadFile(sink)
			if runErr == nil || !strings.Contains(string(output), value) || len(contents) != 0 {
				t.Fatalf("invalid run owner %q: err=%v output=%q sink=%q", value, runErr, output, contents)
			}
		})
	}
	runLauncherDeathWitness(t, true)
}

func launcherProcessRef(t *testing.T) string {
	command := exec.Command("tail", "-f", "/dev/null")
	checkWitness(t, command.Start())
	t.Cleanup(func() { _ = command.Process.Kill(); _, _ = command.Process.Wait() })
	return liveWitnessProcessRef(t, int64(command.Process.Pid), 0)
}

func liveWitnessProcessRef(t *testing.T, pid, disallowed int64) string {
	exact, state, probeErr := (KernelProber{}).Probe(pid)
	value, encodeErr := EncodeRef(exact.Ref())
	if probeErr != nil || state != Alive || encodeErr != nil || exact.Pid == disallowed {
		t.Fatalf("probe process %d: disallowed=%d state=%s probe=%v encode=%v", exact.Pid, disallowed, state, probeErr, encodeErr)
	}
	return value
}

func runLauncherWitnessMode(t *testing.T) bool {
	mode, dir, _ := strings.Cut(os.Getenv(launcherWitnessMode), "|")
	switch mode {
	case "owner":
		if sink := filepath.Join(dir, "signal-sink"); func() bool { _, err := os.Stat(sink); return err == nil }() {
			checkWitness(t, os.WriteFile(sink, []byte("test ran"), 0o600))
			return true
		}
		runWitnessOwner(t, dir)
	case "launcher":
		environment, err := ExportRunOwner(witnessEnvironment([]string{launcherBasePath, launcherWitnessMode + "=owner|" + dir}))
		checkWitness(t, err)
		launcherLog := fmt.Sprintf("%s.custodian-%d.log", os.Getenv("METASYSTEM_SUPERVISION_REGISTRY_HOME"), os.Getpid())
		checkWitness(t, os.WriteFile(filepath.Join(dir, "launcher-logpath"), []byte(launcherLog), 0o600))
		command := exec.Command("/bin/sh", "-c", `"$1" -test.run="^$2$" -test.count=1; exec tail -f /dev/null`, "sh", os.Args[0], t.Name())
		command.Env = environment
		checkWitness(t, command.Start())
		checkWitness(t, publishWitnessPID(filepath.Join(dir, "shellpid"), command.Process.Pid))
		holdWitnessProcess(t)
	default:
		return false
	}
	return true
}

func runLauncherDeathWitness(t *testing.T, exported bool) {
	dir := t.TempDir()
	var command *exec.Cmd
	if exported {
		command = exec.Command(os.Args[0], "-test.run=^"+t.Name()+"$", "-test.count=1")
		command.Env = witnessEnvironment([]string{launcherBasePath, launcherWitnessMode + "=launcher|" + dir})
	} else {
		command = exec.Command("/bin/sh", "-c", `"$1" -test.run="^$2$" -test.count=1; exit $?`, "sh", os.Args[0], t.Name())
		command.Env = witnessEnvironment([]string{launcherBasePath, launcherWitnessMode + "=owner|" + dir})
	}
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	startWitnessCommand(t, command, filepath.Join(dir, "launcher.stderr"), false)
	t.Cleanup(func() { _ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL); _, _ = command.Process.Wait() })
	owner := waitWitnessRef(t, dir, filepath.Join(dir, "ownerpid"), filepath.Join(dir, "launcher.stderr"))
	child := waitWitnessRef(t, dir, filepath.Join(dir, "pid0"), filepath.Join(dir, "launcher.stderr"))
	t.Cleanup(func() { _ = SignalExact(KernelProber{}, child, syscall.SIGKILL) })
	logPath, _ := os.ReadFile(filepath.Join(dir, "logpath"))
	t.Cleanup(func() { _ = os.Remove(string(logPath)) })
	other := waitWitnessRef(t, dir, filepath.Join(dir, "pid1"), filepath.Join(dir, "launcher.stderr"))
	t.Cleanup(func() { _ = SignalExact(KernelProber{}, other, syscall.SIGKILL) })
	custodian := waitWitnessCustodian(t, dir, owner)
	t.Cleanup(func() { _ = SignalExact(KernelProber{}, custodian, syscall.SIGKILL) })
	waitWitnessCustodianObserved(t, dir, string(logPath), child, other)
	var shell Ref
	if exported {
		shell = waitWitnessRef(t, dir, filepath.Join(dir, "shellpid"), filepath.Join(dir, "launcher.stderr"))
		launcherLog, err := os.ReadFile(filepath.Join(dir, "launcher-logpath"))
		checkWitness(t, err)
		t.Cleanup(func() { _ = os.Remove(string(launcherLog)) })
		waitWitnessCustodianObserved(t, dir, string(launcherLog), shell)
	}
	launcherValue := liveWitnessProcessRef(t, int64(command.Process.Pid), owner.Pid)
	deaths := []witnessDeath{
		armWitnessDeath(t, owner),
		armWitnessDeath(t, child),
		armWitnessDeath(t, other),
	}
	if exported {
		deaths = append(deaths, armWitnessDeath(t, shell))
	}
	_ = command.Process.Kill()
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	_ = waitWitnessCommand(t, dir, command, done, func() string {
		data, _ := os.ReadFile(filepath.Join(dir, "launcher.stderr"))
		return string(data)
	})
	for _, death := range deaths {
		waitWitnessDead(t, dir, death)
	}
	waitWitnessLog(t, dir, string(logPath), "dead-launcher="+launcherValue)
}

func TestKilledTestBinaryLeavesNoFixtureChild(t *testing.T) { custodianWitness(t, false) }
func TestCustodianReapsStoppedAndDetached(t *testing.T)     { custodianWitness(t, true) }

func TestQuietCustodianRemovesItsLog(t *testing.T) {
	if dir := os.Getenv("FIXTURE_QUIET_CUSTODIAN_WITNESS"); dir != "" {
		exact, state, err := (KernelProber{}).Probe(int64(os.Getpid()))
		if err != nil || state != Alive {
			t.Fatalf("probe quiet owner: state=%s err=%v", state, err)
		}
		logPath := fmt.Sprintf("%s.custodian-%d.log", os.Getenv("METASYSTEM_SUPERVISION_REGISTRY_HOME"), os.Getpid())
		recordsPath := fmt.Sprintf("%s.fixture-refs-%d", os.Getenv("METASYSTEM_SUPERVISION_REGISTRY_HOME"), os.Getpid())
		checkWitness(t, func() error { _, err := os.Stat(logPath); return err }())
		checkWitness(t, os.WriteFile(filepath.Join(dir, "logpath"), []byte(logPath), 0o600))
		checkWitness(t, os.WriteFile(filepath.Join(dir, "recordspath"), []byte(recordsPath), 0o600))
		checkWitness(t, recordWitnessRef(dir, "owner", exact.Ref()))
		custodian := waitWitnessCustodian(t, dir, exact.Ref())
		checkWitness(t, publishWitnessPID(filepath.Join(dir, "custodianpid"), int(custodian.Pid)))
		waitWitnessFile(t, dir, filepath.Join(dir, "release"))
		return
	}
	dir := t.TempDir()
	command := exec.Command(os.Args[0], "-test.run=^"+t.Name()+"$", "-test.count=1")
	command.Env = witnessEnvironment(append(os.Environ(), "FIXTURE_QUIET_CUSTODIAN_WITNESS="+dir))
	startWitnessCommand(t, command, filepath.Join(dir, "owner.stderr"), true)
	t.Cleanup(func() { _ = command.Process.Kill(); _, _ = command.Process.Wait() })
	custodian := waitWitnessRef(t, dir, filepath.Join(dir, "custodianpid"), filepath.Join(dir, "owner.stderr"))
	t.Cleanup(func() { _ = SignalExact(KernelProber{}, custodian, syscall.SIGKILL) })
	logPath, err := os.ReadFile(filepath.Join(dir, "logpath"))
	checkWitness(t, err)
	recordsPath, err := os.ReadFile(filepath.Join(dir, "recordspath"))
	checkWitness(t, err)
	t.Cleanup(func() { _ = os.Remove(string(logPath)) })
	waitWitnessFile(t, dir, string(logPath))
	waitWitnessFile(t, dir, string(recordsPath))
	custodianDeath := armWitnessDeath(t, custodian)
	checkWitness(t, os.WriteFile(filepath.Join(dir, "release"), nil, 0o600))
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	if err := waitWitnessCommand(t, dir, command, done, func() string {
		output, _ := os.ReadFile(filepath.Join(dir, "owner.stderr"))
		return string(output)
	}); err != nil {
		output, _ := os.ReadFile(filepath.Join(dir, "owner.stderr"))
		t.Fatalf("quiet owner exit: %v output=%q", err, output)
	}
	waitWitnessDead(t, dir, custodianDeath)
	_, logState := os.Lstat(string(logPath))
	_, recordsState := os.Lstat(string(recordsPath))
	if !os.IsNotExist(logState) || !os.IsNotExist(recordsState) {
		log, _ := os.ReadFile(string(logPath))
		t.Fatalf("quiet custodian artifacts remained: log=%s state=%v records=%s state=%v content=%q",
			logPath, logState, recordsPath, recordsState, log)
	}
}
func TestCustodianRejectsRuntimePollerAsWatch(t *testing.T) {
	exact, state, err := (KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != Alive {
		t.Fatalf("probe custodian owner: state=%s err=%v", state, err)
	}
	owner, err := EncodeRef(exact.Ref())
	checkWitness(t, err)
	dir := t.TempDir()
	output, err := os.CreateTemp(dir, "custodian-output")
	checkWitness(t, err)
	defer output.Close()
	checkWitness(t, unix.SetNonblock(int(output.Fd()), true))
	command := exec.Command(os.Args[0])
	command.Env = witnessEnvironment(append(os.Environ(), FixtureCustodianEnv+"=1", FixtureCustodianOwnerEnv+"="+owner))
	command.Stdout, command.Stderr = output, output
	checkWitness(t, command.Start())
	t.Cleanup(func() { _ = command.Process.Kill() })
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	waitErr := waitWitnessCommand(t, dir, command, done, func() string {
		data, _ := os.ReadFile(output.Name())
		return string(data)
	})
	exit, ok := waitErr.(*exec.ExitError)
	data, _ := os.ReadFile(output.Name())
	if !ok || exit.ExitCode() != 2 || !strings.Contains(string(data), "descriptor 3") {
		t.Fatalf("custodian exit=%v output=%q; want exit 2 naming descriptor 3", waitErr, data)
	}
}

func TestCustodianRejectsMissingReadyDescriptor(t *testing.T) {
	dir := t.TempDir()
	owner := liveWitnessProcessRef(t, int64(os.Getpid()), 0)
	watch, held, err := os.Pipe()
	checkWitness(t, err)
	t.Cleanup(func() { _ = watch.Close(); _ = held.Close() })
	var output strings.Builder
	command := exec.Command(os.Args[0])
	command.Env = witnessEnvironment(append(os.Environ(), FixtureCustodianEnv+"=1", FixtureCustodianOwnerEnv+"="+owner))
	command.ExtraFiles, command.Stderr = []*os.File{watch}, &output
	checkWitness(t, command.Start())
	t.Cleanup(func() { _ = command.Process.Kill(); _, _ = command.Process.Wait() })
	started, _ := ParseRef(liveWitnessProcessRef(t, int64(command.Process.Pid), 0))
	checkWitness(t, recordWitnessRef(dir, "custodian", started))
	t.Cleanup(func() { _ = SignalExact(KernelProber{}, started, syscall.SIGKILL) })
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	waitErr := waitWitnessCommand(t, dir, command, done, output.String)
	exit, ok := waitErr.(*exec.ExitError)
	if !ok || exit.ExitCode() != 2 || !strings.Contains(output.String(), "descriptor 4") {
		t.Fatalf("custodian exit=%v output=%q; want exit 2 naming descriptor 4", waitErr, output.String())
	}
}

func custodianWitness(t *testing.T, hard bool) {
	if dir := os.Getenv("FIXTURE_CUSTODIAN_WITNESS"); dir != "" {
		runWitnessOwner(t, dir)
		return
	}
	dir := t.TempDir()
	command := exec.Command(os.Args[0], "-test.run=^"+t.Name()+"$", "-test.count=1")
	command.Env = witnessEnvironment(append(os.Environ(), "FIXTURE_CUSTODIAN_WITNESS="+dir))
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	startWitnessCommand(t, command, filepath.Join(dir, "owner.stderr"), true)
	owner, state, err := (KernelProber{}).Probe(int64(command.Process.Pid))
	if err != nil || state != Alive {
		t.Fatalf("probe witness owner: state=%s err=%v", state, err)
	}
	ownerValue, err := EncodeRef(owner.Ref())
	checkWitness(t, err)
	checkWitness(t, recordWitnessRef(dir, "owner", owner.Ref()))
	t.Cleanup(func() { _ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL); _, _ = command.Process.Wait() })
	refs := make([]Ref, 3)
	var logPath []byte
	for i := range refs {
		pidPath := filepath.Join(dir, "pid"+strconv.Itoa(i))
		refs[i] = waitWitnessRef(t, dir, pidPath, filepath.Join(dir, "owner.stderr"), pidPath+".stderr")
		t.Cleanup(func() { _ = SignalExact(KernelProber{}, refs[i], syscall.SIGKILL) })
		if i == 0 {
			logPath, _ = os.ReadFile(filepath.Join(dir, "logpath"))
			t.Cleanup(func() { _ = os.Remove(string(logPath)) })
		}
	}
	custodian := waitWitnessCustodian(t, dir, owner.Ref())
	t.Cleanup(func() { _ = SignalExact(KernelProber{}, custodian, syscall.SIGKILL) })
	waitWitnessCustodianObserved(t, dir, string(logPath), refs...)
	deaths := make([]witnessDeath, 0, len(refs)+1)
	for _, ref := range refs {
		deaths = append(deaths, armWitnessDeath(t, ref))
	}
	deaths = append(deaths, armWitnessDeath(t, custodian))
	if hard {
		checkWitness(t, os.WriteFile(filepath.Join(dir, "stop"), nil, 0o600))
		waitWitnessFile(t, dir, filepath.Join(dir, "stopped"))
		_ = syscall.Kill(-command.Process.Pid, syscall.SIGTERM)
	} else {
		_ = command.Process.Kill()
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	_ = waitWitnessCommand(t, dir, command, done, func() string {
		data, _ := os.ReadFile(filepath.Join(dir, "owner.stderr"))
		return string(data)
	})
	for _, death := range deaths {
		waitWitnessDead(t, dir, death)
	}
	log, err := os.ReadFile(string(logPath))
	if err != nil {
		t.Fatalf("read completed custodian log: %v log=%q", err, log)
	}
	if want := "owner=" + ownerValue + " action=complete"; !strings.Contains(string(log), want) {
		t.Fatalf("custodian log %q does not contain %q", log, want)
	}
	for _, ref := range refs {
		if !strings.Contains(string(log), "action=kill pid="+strconv.FormatInt(ref.Pid, 10)+" ") {
			t.Fatalf("custodian log %q omits pid %d", log, ref.Pid)
		}
	}
	if !strings.Contains(string(log), "action=kill pid="+strconv.FormatInt(refs[2].Pid, 10)+" carrier=record") {
		t.Fatalf("custodian log %q does not reap untagged pid %d through its record", log, refs[2].Pid)
	}
}

func runWitnessOwner(t *testing.T, dir string) {
	exact, state, err := (KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != Alive {
		t.Fatalf("probe owner: state=%s err=%v", state, err)
	}
	checkWitness(t, publishWitnessPID(filepath.Join(dir, "ownerpid"), os.Getpid()))
	word := fixtureWord(t, FixtureKey{Owner: exact.Ref(), Test: t.Name(), Nonce: "1234abcd"})
	script := filepath.Join(dir, "detached.sh")
	body := `if [ -n "${METASYSTEM_FIXTURE_OWNER-}" ]; then tag="METASYSTEM_FIXTURE_OWNER=$METASYSTEM_FIXTURE_OWNER"; [ "${1-}" = "$tag" ] || exec /bin/sh "$0" "$tag" "$@"; shift; fi; tmp="$1.tmp.$$"; printf %s $$ > "$tmp"; mv "$tmp" "$1"; trap '' TERM; exec tail -f /dev/null`
	checkWitness(t, testexec.WriteFile(script, []byte(body), 0o700))
	detached := exec.Command("/bin/sh", "-c", `exec /bin/sh "$1" "$2"`, "sh", script, filepath.Join(dir, "pid0"))
	detached.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	commands := []*exec.Cmd{
		detached,
		exec.Command("/bin/sh", "-c", `tmp="$2.tmp.$$"; printf %s $$ > "$tmp"; mv "$tmp" "$2"; exec tail -f /dev/null`, "sh", word, filepath.Join(dir, "pid1")),
		exec.Command("/bin/sh", "-c", `tmp="$1.tmp.$$"; printf %s $$ > "$tmp"; mv "$tmp" "$1"; exec tail -f /dev/null`, "sh", filepath.Join(dir, "pid2")),
	}
	commands[1].SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	registry := os.Getenv("METASYSTEM_SUPERVISION_REGISTRY_HOME")
	if registry == "" {
		t.Fatal("witness owner has no test registry home")
	}
	checkWitness(t, os.WriteFile(filepath.Join(dir, "logpath"), []byte(fmt.Sprintf("%s.custodian-%d.log", registry, os.Getpid())), 0o600))
	for i, command := range commands {
		command.Env = slices.DeleteFunc(os.Environ(), func(entry string) bool { return strings.HasPrefix(entry, FixtureOwnerEnv+"=") })
		if i < 2 {
			command.Env = append(command.Env, word)
		}
		startWitnessCommand(t, command, filepath.Join(dir, "pid"+strconv.Itoa(i)+".stderr"), false)
		refValue := liveWitnessProcessRef(t, int64(command.Process.Pid), 0)
		ref, err := ParseRef(refValue)
		checkWitness(t, err)
		t.Cleanup(func() { _ = SignalExact(KernelProber{}, ref, syscall.SIGKILL); _, _ = command.Process.Wait() })
		if i == 2 {
			records, err := os.OpenFile(fmt.Sprintf("%s.fixture-refs-%d", registry, os.Getpid()), os.O_WRONLY|os.O_APPEND, 0)
			checkWitness(t, err)
			_, writeErr := records.WriteString("+" + refValue + "\n")
			checkWitness(t, errors.Join(writeErr, records.Close()))
		}
	}
	waitWitnessFile(t, dir, filepath.Join(dir, "stop"))
	checkWitness(t, commands[1].Process.Signal(syscall.SIGSTOP))
	var status syscall.WaitStatus
	_, err = syscall.Wait4(commands[1].Process.Pid, &status, syscall.WUNTRACED, nil)
	checkWitness(t, err)
	if status.Exited() || status.Signaled() {
		t.Fatalf("fixture child stop status = %v", status)
	}
	checkWitness(t, os.WriteFile(filepath.Join(dir, "stopped"), nil, 0o600))
	holdWitnessProcess(t)
}

func holdWitnessProcess(t *testing.T) {
	t.Helper()
	reader, writer, err := os.Pipe()
	checkWitness(t, err)
	defer reader.Close()
	defer writer.Close()
	if err := waitWitnessRelease(reader); err != nil {
		t.Fatalf("witness holding pipe returned: %v", err)
	}
	t.Fatal("witness holding pipe returned without an error")
}

func waitWitnessRelease(reader io.Reader) error {
	var oneByte [1]byte
	_, err := reader.Read(oneByte[:])
	return err
}

type controlledWitnessReader struct {
	entered  chan struct{}
	released chan error
}

func (reader controlledWitnessReader) Read([]byte) (int, error) {
	close(reader.entered)
	return 0, <-reader.released
}

func TestWitnessHolderWaitsForKernelRelease(t *testing.T) {
	t.Parallel()

	marker := errors.New("released")
	reader := controlledWitnessReader{entered: make(chan struct{}), released: make(chan error, 1)}
	done := make(chan error, 1)
	go func() { done <- waitWitnessRelease(reader) }()
	<-reader.entered
	select {
	case err := <-done:
		t.Fatalf("witness holder returned before release: %v", err)
	default:
	}
	reader.released <- marker
	if err := <-done; !errors.Is(err, marker) {
		t.Fatalf("witness holder release error=%v, want %v", err, marker)
	}
}

func startWitnessCommand(t *testing.T, command *exec.Cmd, stderrPath string, captureStdout bool) {
	t.Helper()
	stderr, err := os.OpenFile(stderrPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	checkWitness(t, err)
	command.Stderr = stderr
	if captureStdout {
		command.Stdout = stderr
	}
	err = command.Start()
	closeErr := stderr.Close()
	checkWitness(t, err)
	checkWitness(t, closeErr)
}

func waitWitnessRef(t *testing.T, dir, path string, stderrPaths ...string) Ref {
	t.Helper()
	var result Ref
	waitWitness(t, dir, "fixture child to publish "+filepath.Base(path), func() witnessEventSource {
		return armWitnessFileEvent(t, path)
	}, func() (bool, string) {
		data, readErr := os.ReadFile(path)
		if os.IsNotExist(readErr) {
			return false, fmt.Sprintf("path=%s read=%v", path, readErr)
		}
		if readErr != nil {
			t.Fatalf("read published pid %s: %v\n%s", path, readErr, witnessDiagnostics(dir, 0))
		}
		pid, parseErr := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
		if parseErr != nil {
			t.Fatalf("parse published pid %s data=%q: %v\n%s", path, data, parseErr, witnessDiagnostics(dir, 0))
		}
		exact, state, probeErr := (KernelProber{}).Probe(pid)
		observed := fmt.Sprintf("path=%s data=%q read=%v parse=%v pid=%d state=%s zombie=%t probe=%v",
			path, data, readErr, parseErr, pid, state, exact.Zombie, probeErr)
		if probeErr != nil || state != Alive {
			t.Fatalf("published pid was not alive: %s\n%s", observed, witnessDiagnostics(dir, 0))
		}
		result = exact.Ref()
		return true, observed
	})
	checkWitness(t, recordWitnessRef(dir, filepath.Base(path), result))
	return result
}

func waitWitnessFile(t *testing.T, dir, path string) {
	t.Helper()
	waitWitness(t, dir, "file "+filepath.Base(path)+" to appear", func() witnessEventSource {
		return armWitnessFileEvent(t, path)
	}, func() (bool, string) {
		info, err := os.Stat(path)
		return err == nil, fmt.Sprintf("path=%s mode=%v error=%v", path, func() os.FileMode {
			if info == nil {
				return 0
			}
			return info.Mode()
		}(), err)
	})
}

func waitWitnessCustodian(t *testing.T, dir string, owner Ref) Ref {
	t.Helper()
	awaitOwnerPublication := func() {}
	if owner.Pid != int64(os.Getpid()) {
		awaitOwnerPublication = func() {
			waitWitnessFile(t, dir, filepath.Join(dir, "ownerpid"))
		}
	}
	result, observed, err := witnessCustodianAfterOwnerPublication(awaitOwnerPublication, func() (Ref, string, error) {
		return censusWitnessCustodian(owner)
	})
	if err != nil {
		t.Fatalf("find fixture custodian for owner %d: %v; observation=%s\n%s",
			owner.Pid, err, observed, witnessDiagnostics(dir, 0))
	}
	checkWitness(t, recordWitnessRef(dir, "custodian", result))
	return result
}

func witnessCustodianAfterOwnerPublication(
	awaitOwnerPublication func(),
	census func() (Ref, string, error),
) (Ref, string, error) {
	awaitOwnerPublication()
	return census()
}

func censusWitnessCustodian(owner Ref) (Ref, string, error) {
	ownerValue, err := EncodeRef(owner)
	if err != nil {
		return Ref{}, "owner reference could not be encoded", err
	}
	want := FixtureCustodianOwnerEnv + "=" + ownerValue
	pids, err := AllPids()
	if err != nil {
		return Ref{}, "enumerate processes", err
	}
	observed := "no matching child"
	for _, pid := range pids {
		parent, known := ParentPid(pid)
		if !known || parent != owner.Pid {
			continue
		}
		exact, state, probeErr := (KernelProber{}).Probe(pid)
		observed = fmt.Sprintf("pid=%d ppid=%d state=%s zombie=%t environ-known=%t err=%v",
			pid, parent, state, exact.Zombie, exact.EnvironKnown, probeErr)
		if probeErr == nil && state == Alive && exact.EnvironKnown && slices.Contains(exact.Environ, want) {
			return exact.Ref(), observed, nil
		}
	}
	return Ref{}, observed, errors.New("one census found no matching live custodian")
}

func TestWitnessCustodianCensusWaitsForOwnerPublication(t *testing.T) {
	t.Parallel()

	published := false
	want := fixtureExact(703, 73).Ref()
	got, _, err := witnessCustodianAfterOwnerPublication(func() {
		published = true
	}, func() (Ref, string, error) {
		if !published {
			return Ref{}, "census before owner publication", errors.New("census ran before owner publication")
		}
		return want, "matching child", nil
	})
	if err != nil || got != want {
		t.Fatalf("custodian after publication=%+v err=%v, want %+v", got, err, want)
	}
}

func waitWitnessLog(t *testing.T, dir, path, want string) []byte {
	t.Helper()
	var result []byte
	waitWitness(t, dir, fmt.Sprintf("custodian log to contain %q", want), func() witnessEventSource {
		return armWitnessFileEvent(t, path)
	}, func() (bool, string) {
		data, err := os.ReadFile(path)
		result = data
		return err == nil && strings.Contains(string(data), want), fmt.Sprintf("path=%s read=%v content=%q", path, err, data)
	})
	return result
}

func waitWitnessCustodianObserved(t *testing.T, dir, path string, refs ...Ref) {
	t.Helper()
	waitWitness(t, dir, "custodian to observe every fixture descendant", func() witnessEventSource {
		return armWitnessFileEvent(t, path)
	}, func() (bool, string) {
		data, err := os.ReadFile(path)
		missing := missingCustodianObservations(data, refs)
		return err == nil && len(missing) == 0, fmt.Sprintf("path=%s read=%v missing=%q content=%q", path, err, missing, data)
	})
}

func missingCustodianObservations(log []byte, refs []Ref) []string {
	missing := make([]string, 0)
	for _, ref := range refs {
		identity, err := EncodeRef(ref)
		if err != nil {
			missing = append(missing, fmt.Sprintf("invalid exact identity for pid %d", ref.Pid))
			continue
		}
		want := fmt.Sprintf("action=observe pid=%d carrier=descendant identity=%s", ref.Pid, identity)
		if !strings.Contains(string(log), want) {
			missing = append(missing, want)
		}
	}
	return missing
}

func TestWitnessRequiresCustodianObservationOfEveryChild(t *testing.T) {
	t.Parallel()

	first, second := fixtureExact(701, 71).Ref(), fixtureExact(702, 72).Ref()
	line := func(ref Ref) string {
		identity, err := EncodeRef(ref)
		checkWitness(t, err)
		return fmt.Sprintf("fixture-custodian action=observe pid=%d carrier=descendant identity=%s\n", ref.Pid, identity)
	}
	if missing := missingCustodianObservations([]byte(line(first)), []Ref{first, second}); len(missing) != 1 || !strings.Contains(missing[0], "pid=702") {
		t.Fatalf("one-child log missing=%q, want only pid 702", missing)
	}
	if missing := missingCustodianObservations([]byte(line(first)+line(second)), []Ref{first, second}); len(missing) != 0 {
		t.Fatalf("complete observation log still missing=%q", missing)
	}
}

type witnessDeath struct {
	ref   Ref
	event witnessEventSource
}

func armWitnessDeath(t *testing.T, ref Ref) witnessDeath {
	t.Helper()
	return witnessDeath{ref: ref, event: armWitnessDeathEvent(t, ref)}
}

func witnessIdentityReleased(ref Ref) (bool, string) {
	exact, state, err := (KernelProber{}).Probe(ref.Pid)
	refState := state
	if state == Alive {
		comparison := Compare(exact, ref)
		switch {
		case comparison.Mode == CompareInvalid:
			refState = Unknown
		case !comparison.Matches:
			refState = Dead
		}
	}
	// A zombie or exiting process is terminal; only its parent owns the reap.
	// Linux does not report that reap to a process that is not the parent.
	released := fixtureCustodianIdentityReleased(exact, state, err, ref) ||
		err == nil && state == Alive && SameIdentity(exact, ref) && exact.Exiting
	return released, fmt.Sprintf("pid=%d exact-state=%s ref-state=%s same=%t zombie=%t exiting=%t probe=%v",
		ref.Pid, state, refState, SameIdentity(exact, ref), exact.Zombie, exact.Exiting, err)
}

func waitWitnessDead(t *testing.T, dir string, death witnessDeath) {
	t.Helper()
	ref := death.ref
	checkWitness(t, recordWitnessRef(dir, fmt.Sprintf("wait-dead-%d", ref.Pid), ref))
	waitWitness(t, dir, fmt.Sprintf("process %d to die", ref.Pid), func() witnessEventSource {
		return death.event
	}, func() (bool, string) {
		return witnessIdentityReleased(ref)
	})
}

func publishWitnessPID(path string, pid int) error {
	return publishWitnessFile(path, []byte(strconv.Itoa(pid)), 0o600)
}

func waitWitnessCommand(t *testing.T, dir string, command *exec.Cmd, done <-chan error, output func() string) error {
	t.Helper()
	started := time.Now()
	bound := witnessKernelBound(witnessCustodianPoll, witnessCustodianBound)
	timer := time.NewTimer(bound)
	defer timer.Stop()
	select {
	case err := <-done:
		return err
	case <-timer.C:
		elapsed := time.Since(started)
		reapTimer := time.NewTimer(bound)
		defer reapTimer.Stop()
		atBound, stopErr, waitErr, reaped := captureWitnessTimeout(func() string {
			exact, state, probeErr := (KernelProber{}).Probe(int64(command.Process.Pid))
			return fmt.Sprintf("state=%s zombie=%t probe=%v\n%s", state, exact.Zombie, probeErr, witnessDiagnostics(dir, elapsed))
		}, command.Process.Kill, done, reapTimer.C)
		if !reaped {
			exact, state, probeErr := (KernelProber{}).Probe(int64(command.Process.Pid))
			t.Fatalf("timed out waiting for process %d to exit after %s (bound %s): observed-at-bound={%s}; stop-error=%v; after stop it did not report Wait within another %s: state=%s zombie=%t probe=%v",
				command.Process.Pid, elapsed, bound, atBound, stopErr, bound, state, exact.Zombie, probeErr)
		}
		t.Fatalf("timed out waiting for process %d to exit after %s (bound %s): observed-at-bound={%s} stop-error=%v wait-after-stop=%v output=%q",
			command.Process.Pid, elapsed, bound, atBound, stopErr, waitErr, output())
		return waitErr
	}
}

func captureWitnessTimeout(observe func() string, stop func() error, done <-chan error, bound <-chan time.Time) (string, error, error, bool) {
	observation := observe()
	stopErr := stop()
	select {
	case err := <-done:
		return observation, stopErr, err, true
	case <-bound:
		return observation, stopErr, nil, false
	}
}

func TestWitnessCommandTimeoutCapturesStateBeforeKill(t *testing.T) {
	t.Parallel()

	done := make(chan error, 1)
	done <- errors.New("killed")
	bound := make(chan time.Time)
	var order []string
	stopFailure := errors.New("kill refused")
	observation, stopErr, err, reaped := captureWitnessTimeout(func() string {
		order = append(order, "observe")
		return "alive at bound"
	}, func() error {
		order = append(order, "kill")
		return stopFailure
	}, done, bound)
	if observation != "alive at bound" || !errors.Is(stopErr, stopFailure) || err == nil || !reaped || !slices.Equal(order, []string{"observe", "kill"}) {
		t.Fatalf("timeout observation=%q stopErr=%v err=%v reaped=%t order=%v", observation, stopErr, err, reaped, order)
	}
	expired := make(chan time.Time, 1)
	expired <- time.Unix(1, 0)
	_, stopErr, err, reaped = captureWitnessTimeout(func() string { return "still alive" }, func() error { return nil }, make(chan error), expired)
	if stopErr != nil || err != nil || reaped {
		t.Fatalf("second bound stopErr=%v err=%v reaped=%t, want bounded unreaped result", stopErr, err, reaped)
	}
}

func waitWitness(t *testing.T, dir, want string, arm func() witnessEventSource, observe func() (bool, string)) {
	t.Helper()
	started := time.Now()
	waitWitnessWithClock(dir, want, arm, func() time.Duration {
		return time.Since(started)
	}, observe, func(message string) {
		t.Fatal(message)
	}, os.Stderr)
}

func waitWitnessWithClock(
	dir, want string,
	arm func() witnessEventSource,
	elapsed func() time.Duration,
	observe func() (bool, string),
	fail func(string),
	writer io.Writer,
) {
	waitForWitnessEvent(want, arm, observe, elapsed, func(name string, observations []string, label time.Duration) {
		writeWitnessSnapshot(writer, dir, name, observations, label)
	}, fail)
}

func writeWitnessSnapshot(writer io.Writer, dir, want string, observations []string, elapsed time.Duration) {
	lines := []string{
		"wait=" + want,
		fmt.Sprintf("elapsed-label=%s", elapsed),
	}
	for index, observation := range observations {
		lines = append(lines, fmt.Sprintf("observation[%d]=%s", index, observation))
	}
	lines = append(lines, strings.Split(witnessDiagnostics(dir, elapsed), "\n")...)
	for _, line := range lines {
		_, _ = fmt.Fprintln(writer, "witness-wait: "+line)
	}
}

func recordWitnessRef(dir, name string, ref Ref) error {
	value, err := EncodeRef(ref)
	if err != nil {
		return err
	}
	name = strings.NewReplacer("/", "-", "\\", "-").Replace(name)
	return os.WriteFile(filepath.Join(dir, "."+name+".witness-ref"), []byte(value), 0o600)
}

func witnessProcessStates(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "pid-states unavailable: " + err.Error()
	}
	var states []string
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".witness-ref") {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(dir, entry.Name()))
		ref, parseErr := ParseRef(strings.TrimSpace(string(data)))
		if readErr != nil || parseErr != nil {
			states = append(states, fmt.Sprintf("%s read=%v parse=%v", entry.Name(), readErr, parseErr))
			continue
		}
		exact, state, probeErr := (KernelProber{}).Probe(ref.Pid)
		states = append(states, fmt.Sprintf("%s pid=%d state=%s same=%t zombie=%t ppid=%d probe=%v",
			entry.Name(), ref.Pid, state, SameIdentity(exact, ref), exact.Zombie, func() int64 {
				parent, _ := ParentPid(ref.Pid)
				return parent
			}(), probeErr))
	}
	sort.Strings(states)
	if len(states) == 0 {
		return "pid-states: none published"
	}
	return "pid-states: " + strings.Join(states, "; ")
}

func witnessDiagnostics(dir string, elapsed time.Duration) string {
	var diagnostics strings.Builder
	fmt.Fprintf(&diagnostics, "elapsed=%s\n%s", elapsed, witnessProcessStates(dir))
	logPathData, pathErr := os.ReadFile(filepath.Join(dir, "logpath"))
	logPath := strings.TrimSpace(string(logPathData))
	logData, logErr := os.ReadFile(logPath)
	fmt.Fprintf(&diagnostics, "\ncustodian-log path=%q path-read=%v log-read=%v content=%q", logPath, pathErr, logErr, logData)
	entries, _ := os.ReadDir(dir)
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".stderr") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		fmt.Fprintf(&diagnostics, "\n%s read=%v content=%q", entry.Name(), err, data)
	}
	return diagnostics.String()
}

func TestWitnessDiagnosticsIncludesLogStatesAndElapsed(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	exact, state, err := (KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != Alive {
		t.Fatalf("probe diagnostic fixture: state=%s err=%v", state, err)
	}
	checkWitness(t, recordWitnessRef(dir, "owner", exact.Ref()))
	logPath := filepath.Join(dir, "custodian.log")
	checkWitness(t, os.WriteFile(logPath, []byte("diagnostic log"), 0o600))
	checkWitness(t, os.WriteFile(filepath.Join(dir, "logpath"), []byte(logPath), 0o600))
	diagnostics := witnessDiagnostics(dir, 123*time.Millisecond)
	for _, want := range []string{"elapsed=123ms", "state=alive", "custodian-log path=", "diagnostic log"} {
		if !strings.Contains(diagnostics, want) {
			t.Fatalf("diagnostics %q omit %q", diagnostics, want)
		}
	}
}

func TestWaitWitnessFailureIncludesLogStatesAndElapsed(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	exact, state, err := (KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != Alive {
		t.Fatalf("probe wait diagnostic fixture: state=%s err=%v", state, err)
	}
	checkWitness(t, recordWitnessRef(dir, "owner", exact.Ref()))
	logPath := filepath.Join(dir, "custodian.log")
	checkWitness(t, os.WriteFile(logPath, []byte("wait diagnostic log"), 0o600))
	checkWitness(t, os.WriteFile(filepath.Join(dir, "logpath"), []byte(logPath), 0o600))
	var snapshots strings.Builder
	terminal := errors.New("scripted terminal watch error")
	var failure string
	waitWitnessWithClock(dir, "test fact", func() witnessEventSource {
		return witnessEventFunc(func() error {
			if !strings.Contains(snapshots.String(), "observation[0]=still alive") {
				return errors.New("event blocked before initial snapshot")
			}
			return terminal
		})
	}, func() time.Duration {
		return 2 * time.Second
	}, func() (bool, string) {
		return false, "still alive"
	}, func(message string) {
		failure = message
	}, &snapshots)
	for _, want := range []string{"wait=test fact", "elapsed-label=2s", "state=alive", "wait diagnostic log", terminal.Error()} {
		if !strings.Contains(snapshots.String(), want) {
			t.Fatalf("wait snapshots %q omit %q; failure=%q", snapshots.String(), want, failure)
		}
	}
	if !strings.Contains(failure, terminal.Error()) {
		t.Fatalf("wait failure %q omits %q", failure, terminal)
	}
}

func checkWitness(t *testing.T, err error) {
	if err != nil {
		t.Fatal(err)
	}
}
