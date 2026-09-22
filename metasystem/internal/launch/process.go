package launch

import (
	"errors"
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

type OSProcesses struct{ Prober identity.Prober }

func (p OSProcesses) SelfRef() (identity.Ref, error) {
	exact, state, err := p.Prober.Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		return identity.Ref{}, fmt.Errorf("supervisor identity is not observable: %s: %w", state, err)
	}
	return exact.Ref(), nil
}

type osChild struct {
	command *exec.Cmd
	files   []*os.File
}

func (child *osChild) Wait() (int, error) {
	err := child.command.Wait()
	for _, file := range child.files {
		if closeErr := file.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}
	if child.command.ProcessState == nil {
		return 1, err
	}
	status := child.command.ProcessState.Sys().(syscall.WaitStatus)
	if status.Signaled() {
		return 128 + int(status.Signal()), err
	}
	return child.command.ProcessState.ExitCode(), err
}
func (p OSProcesses) StartChild(spec Command) (Child, identity.Ref, error) {
	log, err := os.OpenFile(spec.LogPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, identity.Ref{}, err
	}
	command := exec.Command(spec.Program, spec.Args...)
	command.Dir = spec.Directory
	command.Env = childEnvironment(os.Environ(), spec.Environment)
	command.Stdin = strings.NewReader(spec.Stdin)
	files := []*os.File{log}
	command.Stdout, command.Stderr = log, log
	if spec.StdoutPath != "" {
		stdout, openErr := os.OpenFile(spec.StdoutPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
		if openErr != nil {
			log.Close()
			return nil, identity.Ref{}, openErr
		}
		files = append(files, stdout)
		command.Stdout = stdout
	}
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := command.Start(); err != nil {
		for _, file := range files {
			file.Close()
		}
		return nil, identity.Ref{}, err
	}
	pid := int64(command.Process.Pid)
	exact, state, err := p.Prober.Probe(pid)
	if err != nil || state != identity.Alive {
		_ = syscall.Kill(-int(pid), syscall.SIGKILL)
		_, _ = command.Process.Wait()
		for _, file := range files {
			file.Close()
		}
		// The child has existed even if its exact identity could not be read.
		// Return its group ID so the launch retains output ownership until the
		// whole group is proved dead.
		return nil, identity.Ref{Pid: pid}, fmt.Errorf("child pid %d is not observable: %s: %w", pid, state, err)
	}
	return &osChild{command: command, files: files}, exact.Ref(), nil
}

func childEnvironment(parent, overrides []string) []string {
	keys := map[string]bool{}
	for _, entry := range overrides {
		if index := strings.IndexByte(entry, '='); index >= 0 {
			keys[entry[:index]] = true
		}
	}
	environment := make([]string, 0, len(parent)+len(overrides))
	for _, entry := range parent {
		index := strings.IndexByte(entry, '=')
		if index < 0 || !keys[entry[:index]] {
			environment = append(environment, entry)
		}
	}
	return append(environment, overrides...)
}
func (OSProcesses) SignalGroup(pgid int64, signal syscall.Signal) error {
	err := syscall.Kill(-int(pgid), signal)
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return err
}
func (OSProcesses) Signal(pid int64, signal syscall.Signal) error {
	return syscall.Kill(int(pid), signal)
}
func (OSProcesses) GroupAlive(pgid int64) (bool, error) {
	err := syscall.Kill(-int(pgid), 0)
	switch {
	case err == nil, errors.Is(err, syscall.EPERM):
		return true, nil
	case errors.Is(err, syscall.ESRCH):
		return false, nil
	default:
		return false, err
	}
}

type OSSupervisorStarter struct {
	Executable string
	Prober     identity.Prober
}

func (s OSSupervisorStarter) StartSupervisor(id, stateDir string) (identity.Ref, error) {
	executable := s.Executable
	if executable == "" {
		var err error
		executable, err = os.Executable()
		if err != nil {
			return identity.Ref{}, err
		}
	}
	log, err := os.OpenFile(stateDir+"/supervisor.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return identity.Ref{}, err
	}
	command := exec.Command(executable, "proc", "setsid", "--", executable, "launch", "supervise", "--id", id)
	command.Stdout, command.Stderr = log, log
	if err := command.Start(); err != nil {
		log.Close()
		return identity.Ref{}, err
	}
	log.Close()
	pid := int64(command.Process.Pid)
	prober := s.Prober
	if prober == nil {
		prober = identity.KernelProber{}
	}
	exact, state, err := prober.Probe(pid)
	if err != nil || state != identity.Alive {
		_ = command.Process.Kill()
		_, _ = command.Process.Wait()
		return identity.Ref{}, fmt.Errorf("supervisor pid %d is not observable: %s: %w", pid, state, err)
	}
	go func() { _ = command.Wait() }()
	return exact.Ref(), nil
}
