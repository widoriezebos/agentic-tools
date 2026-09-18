package identity

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

const launcherWitnessMode, launcherBasePath = "FIXTURE_LAUNCHER_WITNESS_MODE", "PATH=/usr/bin:/bin"

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
			command.Env = []string{launcherBasePath, launcherWitnessMode + "=owner|" + dir, RunOwnerEnv + "=" + value}
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
	command := exec.Command("sleep", "30")
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
		environment, err := ExportRunOwner([]string{launcherBasePath, launcherWitnessMode + "=owner|" + dir})
		checkWitness(t, err)
		command := exec.Command("/bin/sh", "-c", `"$1" -test.run="^$2$" -test.count=1; while :; do sleep 1; done`, "sh", os.Args[0], t.Name())
		command.Env = environment
		checkWitness(t, command.Start())
		checkWitness(t, os.WriteFile(filepath.Join(dir, "shellpid"), []byte(strconv.Itoa(command.Process.Pid)), 0o600))
		time.Sleep(24 * time.Hour)
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
		command.Env = []string{launcherBasePath, launcherWitnessMode + "=launcher|" + dir}
	} else {
		command = exec.Command("/bin/sh", "-c", `"$1" -test.run="^$2$" -test.count=1; exit $?`, "sh", os.Args[0], t.Name())
		command.Env = []string{launcherBasePath, launcherWitnessMode + "=owner|" + dir}
	}
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	startWitnessCommand(t, command, filepath.Join(dir, "launcher.stderr"), false)
	t.Cleanup(func() { _ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL); _, _ = command.Process.Wait() })
	owner := waitWitnessRef(t, filepath.Join(dir, "ownerpid"), filepath.Join(dir, "launcher.stderr"))
	child := waitWitnessRef(t, filepath.Join(dir, "pid0"), filepath.Join(dir, "launcher.stderr"))
	t.Cleanup(func() { _ = SignalExact(KernelProber{}, child, syscall.SIGKILL) })
	logPath, _ := os.ReadFile(filepath.Join(dir, "logpath"))
	t.Cleanup(func() { _ = os.Remove(string(logPath)) })
	other := waitWitnessRef(t, filepath.Join(dir, "pid1"), filepath.Join(dir, "launcher.stderr"))
	t.Cleanup(func() { _ = SignalExact(KernelProber{}, other, syscall.SIGKILL) })
	custodian := waitWitnessCustodian(t, owner)
	t.Cleanup(func() { _ = SignalExact(KernelProber{}, custodian, syscall.SIGKILL) })
	var shell Ref
	if exported {
		shell = waitWitnessRef(t, filepath.Join(dir, "shellpid"), filepath.Join(dir, "launcher.stderr"))
	}
	launcherValue := liveWitnessProcessRef(t, int64(command.Process.Pid), owner.Pid)
	_ = command.Process.Kill()
	_, _ = command.Process.Wait()
	waitWitnessDead(t, owner)
	waitWitnessDead(t, child)
	waitWitnessDead(t, other)
	if exported && AliveRef(KernelProber{}, shell) != Alive {
		t.Fatal("intermediate shell died with the exported launcher")
	}
	waitWitnessLog(t, string(logPath), "dead-launcher="+launcherValue)
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
		custodian := waitWitnessCustodian(t, exact.Ref())
		checkWitness(t, os.WriteFile(filepath.Join(dir, "custodianpid"), []byte(strconv.FormatInt(custodian.Pid, 10)), 0o600))
		waitWitnessFile(t, filepath.Join(dir, "release"))
		return
	}
	dir := t.TempDir()
	command := exec.Command(os.Args[0], "-test.run=^"+t.Name()+"$", "-test.count=1")
	command.Env = append(os.Environ(), "FIXTURE_QUIET_CUSTODIAN_WITNESS="+dir)
	startWitnessCommand(t, command, filepath.Join(dir, "owner.stderr"), true)
	t.Cleanup(func() { _ = command.Process.Kill(); _, _ = command.Process.Wait() })
	custodian := waitWitnessRef(t, filepath.Join(dir, "custodianpid"), filepath.Join(dir, "owner.stderr"))
	t.Cleanup(func() { _ = SignalExact(KernelProber{}, custodian, syscall.SIGKILL) })
	logPath, err := os.ReadFile(filepath.Join(dir, "logpath"))
	checkWitness(t, err)
	recordsPath, err := os.ReadFile(filepath.Join(dir, "recordspath"))
	checkWitness(t, err)
	t.Cleanup(func() { _ = os.Remove(string(logPath)) })
	waitWitnessFile(t, string(logPath))
	waitWitnessFile(t, string(recordsPath))
	checkWitness(t, os.WriteFile(filepath.Join(dir, "release"), nil, 0o600))
	if err := command.Wait(); err != nil {
		output, _ := os.ReadFile(filepath.Join(dir, "owner.stderr"))
		t.Fatalf("quiet owner exit: %v output=%q", err, output)
	}
	waitWitnessDead(t, custodian)
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
	output, err := os.CreateTemp(t.TempDir(), "custodian-output")
	checkWitness(t, err)
	defer output.Close()
	checkWitness(t, unix.SetNonblock(int(output.Fd()), true))
	command := exec.Command(os.Args[0])
	command.Env = append(os.Environ(), FixtureCustodianEnv+"=1", FixtureCustodianOwnerEnv+"="+owner)
	command.Stdout, command.Stderr = output, output
	checkWitness(t, command.Start())
	t.Cleanup(func() { _ = command.Process.Kill() })
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	var waitErr error
	select {
	case waitErr = <-done:
	case <-time.After(2 * time.Second):
		_ = command.Process.Kill()
		<-done
		data, _ := os.ReadFile(output.Name())
		t.Fatalf("custodian accepted descriptor 3 opened by the Go runtime; output=%q", data)
	}
	exit, ok := waitErr.(*exec.ExitError)
	data, _ := os.ReadFile(output.Name())
	if !ok || exit.ExitCode() != 2 || !strings.Contains(string(data), "descriptor 3") {
		t.Fatalf("custodian exit=%v output=%q; want exit 2 naming descriptor 3", waitErr, data)
	}
}

func TestCustodianRejectsMissingReadyDescriptor(t *testing.T) {
	owner := liveWitnessProcessRef(t, int64(os.Getpid()), 0)
	watch, held, err := os.Pipe()
	checkWitness(t, err)
	t.Cleanup(func() { _ = watch.Close(); _ = held.Close() })
	var output strings.Builder
	command := exec.Command(os.Args[0])
	command.Env = append(os.Environ(), FixtureCustodianEnv+"=1", FixtureCustodianOwnerEnv+"="+owner)
	command.ExtraFiles, command.Stderr = []*os.File{watch}, &output
	checkWitness(t, command.Start())
	t.Cleanup(func() { _ = command.Process.Kill(); _, _ = command.Process.Wait() })
	started, _ := ParseRef(liveWitnessProcessRef(t, int64(command.Process.Pid), 0))
	t.Cleanup(func() { _ = SignalExact(KernelProber{}, started, syscall.SIGKILL) })
	waitErr := command.Wait()
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
	command.Env = append(os.Environ(), "FIXTURE_CUSTODIAN_WITNESS="+dir)
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	startWitnessCommand(t, command, filepath.Join(dir, "owner.stderr"), true)
	owner, state, err := (KernelProber{}).Probe(int64(command.Process.Pid))
	if err != nil || state != Alive {
		t.Fatalf("probe witness owner: state=%s err=%v", state, err)
	}
	ownerValue, err := EncodeRef(owner.Ref())
	checkWitness(t, err)
	t.Cleanup(func() { _ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL); _, _ = command.Process.Wait() })
	refs := make([]Ref, 3)
	var logPath []byte
	for i := range refs {
		pidPath := filepath.Join(dir, "pid"+strconv.Itoa(i))
		refs[i] = waitWitnessRef(t, pidPath, filepath.Join(dir, "owner.stderr"), pidPath+".stderr")
		t.Cleanup(func() { _ = SignalExact(KernelProber{}, refs[i], syscall.SIGKILL) })
		if i == 0 {
			logPath, _ = os.ReadFile(filepath.Join(dir, "logpath"))
			t.Cleanup(func() { _ = os.Remove(string(logPath)) })
		}
	}
	custodian := waitWitnessCustodian(t, owner.Ref())
	t.Cleanup(func() { _ = SignalExact(KernelProber{}, custodian, syscall.SIGKILL) })
	if hard {
		checkWitness(t, os.WriteFile(filepath.Join(dir, "stop"), nil, 0o600))
		waitWitnessFile(t, filepath.Join(dir, "stopped"))
		_ = syscall.Kill(-command.Process.Pid, syscall.SIGTERM)
	} else {
		_ = command.Process.Kill()
	}
	_, _ = command.Process.Wait()
	for _, ref := range refs {
		waitWitnessDead(t, ref)
	}
	waitWitnessDead(t, custodian)
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
	checkWitness(t, os.WriteFile(filepath.Join(dir, "ownerpid"), []byte(strconv.Itoa(os.Getpid())), 0o600))
	word := fixtureWord(t, FixtureKey{Owner: exact.Ref(), Test: t.Name(), Nonce: "1234abcd"})
	script := filepath.Join(dir, "detached.sh")
	body := `if [ -n "${METASYSTEM_FIXTURE_OWNER-}" ]; then tag="METASYSTEM_FIXTURE_OWNER=$METASYSTEM_FIXTURE_OWNER"; [ "${1-}" = "$tag" ] || exec /bin/sh "$0" "$tag" "$@"; shift; fi; printf %s $$ > "$1"; trap '' TERM; while :; do sleep 1; done`
	checkWitness(t, os.WriteFile(script, []byte(body), 0o700))
	detached := exec.Command("/bin/sh", "-c", `exec /bin/sh "$1" "$2"`, "sh", script, filepath.Join(dir, "pid0"))
	detached.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	commands := []*exec.Cmd{
		detached,
		exec.Command("/bin/sh", "-c", `printf %s $$ > "$2"; while :; do sleep 1; done`, "sh", word, filepath.Join(dir, "pid1")),
		exec.Command("/bin/sh", "-c", `printf %s $$ > "$1"; while :; do sleep 1; done`, "sh", filepath.Join(dir, "pid2")),
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
	for {
		if _, err := os.Stat(filepath.Join(dir, "stop")); err == nil {
			checkWitness(t, commands[1].Process.Signal(syscall.SIGSTOP))
			var status syscall.WaitStatus
			_, err := syscall.Wait4(commands[1].Process.Pid, &status, syscall.WUNTRACED, nil)
			checkWitness(t, err)
			if status.Exited() || status.Signaled() {
				t.Fatalf("fixture child stop status = %v", status)
			}
			checkWitness(t, os.WriteFile(filepath.Join(dir, "stopped"), nil, 0o600))
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	for {
		time.Sleep(time.Hour)
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

func waitWitnessRef(t *testing.T, path string, stderrPaths ...string) Ref {
	for deadline := time.Now().Add(wiringBound); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		data, _ := os.ReadFile(path)
		pid, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
		if exact, state, _ := (KernelProber{}).Probe(pid); err == nil && state == Alive {
			return exact.Ref()
		}
	}
	var diagnostics strings.Builder
	for _, stderrPath := range stderrPaths {
		data, _ := os.ReadFile(stderrPath)
		fmt.Fprintf(&diagnostics, "\n%s:\n%s", filepath.Base(stderrPath), data)
	}
	t.Fatalf("fixture child did not publish its pid%s", diagnostics.String())
	return Ref{}
}

func waitWitnessFile(t *testing.T, path string) {
	for deadline := time.Now().Add(wiringBound); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if _, err := os.Stat(path); err == nil {
			return
		}
	}
	t.Fatalf("fixture child did not publish %s", filepath.Base(path))
}

func waitWitnessCustodian(t *testing.T, owner Ref) Ref {
	ownerValue, err := EncodeRef(owner)
	checkWitness(t, err)
	want, observed := FixtureCustodianOwnerEnv+"="+ownerValue, "no matching child"
	for deadline := time.Now().Add(wiringBound); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		pids, err := AllPids()
		if err != nil {
			observed = err.Error()
			continue
		}
		for _, pid := range pids {
			if parent, known := ParentPid(pid); !known || parent != owner.Pid {
				continue
			}
			exact, state, probeErr := (KernelProber{}).Probe(pid)
			observed = fmt.Sprintf("pid=%d state=%s err=%v", pid, state, probeErr)
			if probeErr == nil && state == Alive && exact.EnvironKnown && slices.Contains(exact.Environ, want) {
				return exact.Ref()
			}
		}
	}
	t.Fatalf("fixture custodian for owner %d was not found: %s", owner.Pid, observed)
	return Ref{}
}
func waitWitnessLog(t *testing.T, path, want string) []byte {
	for deadline := time.Now().Add(wiringBound); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if data, err := os.ReadFile(path); err == nil && strings.Contains(string(data), want) {
			return data
		}
	}
	t.Fatalf("fixture custodian log did not publish %q: log=%q", want, func() []byte { data, _ := os.ReadFile(path); return data }())
	return nil
}

func waitWitnessDead(t *testing.T, ref Ref) {
	for deadline := time.Now().Add(wiringBound); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if AliveRef(KernelProber{}, ref) == Dead {
			return
		}
	}
	t.Fatalf("process %d stayed alive", ref.Pid)
}

func checkWitness(t *testing.T, err error) {
	if err != nil {
		t.Fatal(err)
	}
}
