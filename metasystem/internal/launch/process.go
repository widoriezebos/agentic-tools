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
	log     *os.File
}

func (child *osChild) Wait() (int, error) {
	err := child.command.Wait()
	closeErr := child.log.Close()
	if closeErr != nil && err == nil {
		err = closeErr
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
	command.Env = append(os.Environ(), spec.Environment...)
	command.Stdin = strings.NewReader(spec.Stdin)
	command.Stdout, command.Stderr = log, log
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := command.Start(); err != nil {
		log.Close()
		return nil, identity.Ref{}, err
	}
	pid := int64(command.Process.Pid)
	exact, state, err := p.Prober.Probe(pid)
	if err != nil || state != identity.Alive {
		_ = syscall.Kill(-int(pid), syscall.SIGKILL)
		_, _ = command.Process.Wait()
		log.Close()
		return nil, identity.Ref{}, fmt.Errorf("child pid %d is not observable: %s: %w", pid, state, err)
	}
	return &osChild{command: command, log: log}, exact.Ref(), nil
}
func (OSProcesses) SignalGroup(pgid int64, signal syscall.Signal) error {
	err := syscall.Kill(-int(pgid), signal)
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return err
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
	_ = command.Process.Release()
	return exact.Ref(), nil
}
