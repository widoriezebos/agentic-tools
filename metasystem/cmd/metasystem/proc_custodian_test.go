package main

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

const (
	procCustodianHelperEnv = "METASYSTEM_PROC_CUSTODIAN_TEST_HELPER"
	procCustodianOwnerEnv  = "METASYSTEM_PROC_CUSTODIAN_TEST_OWNER"
	procCustodianLogEnv    = "METASYSTEM_PROC_CUSTODIAN_TEST_LOG"
	procCustodianTestBound = 2 * time.Second
)

func TestProcCustodianProcessBoundaries(t *testing.T) {
	if os.Getenv(procCustodianHelperEnv) == "1" {
		os.Exit(runFixtureCustodian([]string{"--owner", os.Getenv(procCustodianOwnerEnv), "--log", os.Getenv(procCustodianLogEnv)}))
	}
	t.Run("missing watch descriptor", func(t *testing.T) {
		process := startProcCustodianProcess(t, nil, true, false, false)
		assertProcCustodianExit2(t, process, "descriptor 3")
	})
	t.Run("regular watch descriptor", func(t *testing.T) {
		regular, err := os.CreateTemp(t.TempDir(), "not-a-pipe")
		if err != nil {
			t.Fatal(err)
		}
		defer regular.Close()
		process := startProcCustodianProcess(t, []*os.File{regular}, false, false, false)
		assertProcCustodianExit2(t, process, "descriptor 3")
	})
	t.Run("inherited fixture owner", func(t *testing.T) {
		reader, writer, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		defer reader.Close()
		process := startProcCustodianProcess(t, []*os.File{reader}, false, true, true)
		process.watchWriter = writer
		assertProcCustodianExit2(t, process, identity.FixtureOwnerEnv)
	})
	t.Run("inherited descriptor", func(t *testing.T) {
		watchReader, watchWriter, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		defer watchReader.Close()
		heldReader, heldWriter, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		defer heldReader.Close()
		process := startProcCustodianProcess(t, []*os.File{watchReader, heldWriter}, false, false, false)
		process.watchWriter = watchWriter
		_ = heldWriter.Close()
		ready := make(chan struct {
			data string
			err  error
		}, 1)
		go func() {
			data, err := io.ReadAll(heldReader)
			ready <- struct {
				data string
				err  error
			}{string(data), err}
		}()
		select {
		case result := <-ready:
			if result.err != nil || result.data != "ready\n" {
				t.Fatalf("descriptor 4 read = %q, %v; want ready and EOF", result.data, result.err)
			}
		case <-time.After(procCustodianTestBound):
			t.Fatal("custodian kept inherited descriptor 4 open")
		}
		var status syscall.WaitStatus
		pid, err := syscall.Wait4(process.command.Process.Pid, &status, syscall.WNOHANG, nil)
		if err != nil || pid != 0 {
			t.Fatalf("custodian was not running at descriptor 4 EOF: pid=%d status=%v err=%v", pid, status, err)
		}
	})
}

type procCustodianProcess struct {
	command        *exec.Cmd
	owner          *exec.Cmd
	output         *os.File
	watchWriter    *os.File
	done           chan struct{}
	waitErr        error
	waitHasStarted bool
}

func startProcCustodianProcess(t *testing.T, extraFiles []*os.File, nonblocking, tagged, groupLeader bool) *procCustodianProcess {
	t.Helper()
	owner := exec.Command("sleep", "30")
	if err := owner.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = owner.Process.Kill(); _, _ = owner.Process.Wait() })
	var exact identity.Exact
	var state identity.Liveness
	var err error
	for deadline := time.Now().Add(procCustodianTestBound); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		exact, state, err = (identity.KernelProber{}).Probe(int64(owner.Process.Pid))
		if err == nil && state == identity.Alive {
			break
		}
	}
	if err != nil || state != identity.Alive {
		t.Fatalf("probe custodian owner: state=%s err=%v", state, err)
	}
	ownerValue, err := identity.EncodeRef(exact.Ref())
	if err != nil {
		t.Fatal(err)
	}
	output, err := os.CreateTemp(t.TempDir(), "proc-custodian-output")
	if err != nil {
		t.Fatal(err)
	}
	if nonblocking {
		if err := unix.SetNonblock(int(output.Fd()), true); err != nil {
			t.Fatal(err)
		}
	}
	environment := make([]string, 0, len(os.Environ())+4)
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if name != procCustodianHelperEnv && name != procCustodianOwnerEnv && name != procCustodianLogEnv && name != identity.FixtureOwnerEnv {
			environment = append(environment, entry)
		}
	}
	environment = append(environment, procCustodianHelperEnv+"=1", procCustodianOwnerEnv+"="+ownerValue,
		procCustodianLogEnv+"="+filepath.Join(t.TempDir(), "custodian.log"))
	if tagged {
		environment = append(environment, identity.FixtureOwnerEnv+"=")
	}
	command := exec.Command(os.Args[0], "-test.run=^TestProcCustodianProcessBoundaries$", "-test.count=1")
	command.Env, command.ExtraFiles = environment, extraFiles
	command.Stdout, command.Stderr = output, output
	if groupLeader {
		command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	process := &procCustodianProcess{command: command, owner: owner, output: output}
	t.Cleanup(process.cleanup)
	return process
}

func (process *procCustodianProcess) startWait() {
	if process.waitHasStarted {
		return
	}
	process.waitHasStarted = true
	process.done = make(chan struct{})
	go func() {
		process.waitErr = process.command.Wait()
		close(process.done)
	}()
}

func (process *procCustodianProcess) cleanup() {
	if process.watchWriter != nil {
		_ = process.watchWriter.Close()
	}
	_ = process.owner.Process.Kill()
	_, _ = process.owner.Process.Wait()
	process.startWait()
	select {
	case <-process.done:
	case <-time.After(3 * time.Second):
		_ = syscall.Kill(process.command.Process.Pid, syscall.SIGKILL)
		<-process.done
	}
	_ = process.output.Close()
}

func assertProcCustodianExit2(t *testing.T, process *procCustodianProcess, want string) {
	t.Helper()
	process.startWait()
	select {
	case <-process.done:
	case <-time.After(procCustodianTestBound):
		t.Fatalf("custodian did not reject %s", want)
	}
	exit, ok := process.waitErr.(*exec.ExitError)
	data, _ := os.ReadFile(process.output.Name())
	if !ok || exit.ExitCode() != 2 || !strings.Contains(string(data), want) {
		t.Fatalf("custodian exit=%v output=%q; want exit 2 naming %s", process.waitErr, data, want)
	}
}
