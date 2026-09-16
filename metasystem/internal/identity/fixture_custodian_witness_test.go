package identity

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestKilledTestBinaryLeavesNoFixtureChild(t *testing.T) { custodianWitness(t, false) }
func TestCustodianReapsStoppedAndDetached(t *testing.T)     { custodianWitness(t, true) }

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

func custodianWitness(t *testing.T, hard bool) {
	if dir := os.Getenv("FIXTURE_CUSTODIAN_WITNESS"); dir != "" {
		runWitnessOwner(t, dir)
		return
	}
	dir := t.TempDir()
	command := exec.Command(os.Args[0], "-test.run=^"+t.Name()+"$", "-test.count=1")
	command.Env = append(os.Environ(), "FIXTURE_CUSTODIAN_WITNESS="+dir, FixtureCustodianStartEnv+"=1")
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	startWitnessCommand(t, command, filepath.Join(dir, "owner.stderr"), true)
	owner, state, err := (KernelProber{}).Probe(int64(command.Process.Pid))
	if err != nil || state != Alive {
		t.Fatalf("probe witness owner: state=%s err=%v", state, err)
	}
	ownerValue, err := EncodeRef(owner.Ref())
	checkWitness(t, err)
	t.Cleanup(func() { _ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL); _, _ = command.Process.Wait() })
	refs := make([]Ref, 2)
	for i := range refs {
		pidPath := filepath.Join(dir, "pid"+strconv.Itoa(i))
		refs[i] = waitWitnessRef(t, pidPath, filepath.Join(dir, "owner.stderr"), pidPath+".stderr")
		defer SignalExact(KernelProber{}, refs[i], syscall.SIGKILL)
	}
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
	logPath, _ := os.ReadFile(filepath.Join(dir, "logpath"))
	log := waitWitnessLog(t, string(logPath), "owner="+ownerValue+" action=complete")
	if hard {
		for _, ref := range refs {
			if !strings.Contains(string(log), "action=kill pid="+strconv.FormatInt(ref.Pid, 10)+" ") {
				t.Fatalf("custodian log %q omits pid %d", log, ref.Pid)
			}
		}
	}
}

func runWitnessOwner(t *testing.T, dir string) {
	exact, state, err := (KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != Alive {
		t.Fatalf("probe owner: state=%s err=%v", state, err)
	}
	word := fixtureWord(t, FixtureKey{Owner: exact.Ref(), Test: t.Name(), Nonce: "1234abcd"})
	script := filepath.Join(dir, "detached.sh")
	body := `if [ -n "${METASYSTEM_FIXTURE_OWNER-}" ]; then tag="METASYSTEM_FIXTURE_OWNER=$METASYSTEM_FIXTURE_OWNER"; [ "${1-}" = "$tag" ] || exec /bin/sh "$0" "$tag" "$@"; shift; fi; printf %s $$ > "$1"; trap '' TERM; while :; do sleep 1; done`
	checkWitness(t, os.WriteFile(script, []byte(body), 0o700))
	detached := exec.Command("/bin/sh", "-c", `exec /bin/sh "$1" "$2"`, "sh", script, filepath.Join(dir, "pid0"))
	detached.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	commands := []*exec.Cmd{detached, exec.Command("/bin/sh", "-c", `printf %s $$ > "$2"; while :; do sleep 1; done`, "sh", word, filepath.Join(dir, "pid1"))}
	commands[1].SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	registry := os.Getenv("METASYSTEM_SUPERVISION_REGISTRY_HOME")
	if registry == "" {
		t.Fatal("witness owner has no test registry home")
	}
	checkWitness(t, os.WriteFile(filepath.Join(dir, "logpath"), []byte(registry+".custodian.log"), 0o600))
	for i, command := range commands {
		command.Env = append(os.Environ(), word)
		startWitnessCommand(t, command, filepath.Join(dir, "pid"+strconv.Itoa(i)+".stderr"), false)
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

func waitWitnessLog(t *testing.T, path, want string) []byte {
	for deadline := time.Now().Add(wiringBound); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if data, err := os.ReadFile(path); err == nil && strings.Contains(string(data), want) {
			return data
		}
	}
	t.Fatalf("fixture custodian log did not publish %q", want)
	return nil
}

func waitWitnessDead(t *testing.T, ref Ref) {
	for deadline := time.Now().Add(wiringBound); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if AliveRef(KernelProber{}, ref) == Dead {
			return
		}
	}
	t.Fatalf("fixture child %d survived owner death", ref.Pid)
}

func checkWitness(t *testing.T, err error) {
	if err != nil {
		t.Fatal(err)
	}
}
