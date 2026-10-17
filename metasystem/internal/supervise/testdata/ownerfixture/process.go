// Package ownerfixture shares process plumbing without supervision policy.
package ownerfixture

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

func Argument(args []string, name string) string {
	for i := range args {
		if args[i] == name && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

func Command(flag string, args ...string) (*exec.Cmd, error) {
	if err := syscall.Mkfifo(Argument(args, "--gate"), 0600); err != nil {
		return nil, err
	}
	return exec.Command(os.Args[0], append([]string{"--", flag}, args...)...), nil
}

func AwaitPublication(args []string) (identity.Exact, error) {
	gate := Argument(args, "--gate")
	reader, err := os.Open(gate)
	if err != nil {
		return identity.Exact{}, err
	}
	defer reader.Close()
	defer os.Remove(gate)
	var release [1]byte
	if _, err = reader.Read(release[:]); err != nil {
		return identity.Exact{}, err
	}
	exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		return exact, fmt.Errorf("owner identity: %s %v", state, err)
	}
	return exact, nil
}

func Component(flag, tag string, extra ...string) (identity.Ref, error) {
	command := exec.Command(os.Args[0], append([]string{"--", flag, tag}, extra...)...)
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := testexec.StartReady(command); err != nil {
		return identity.Ref{}, err
	}
	exact, state, err := (identity.KernelProber{}).Probe(int64(command.Process.Pid))
	if err != nil || state != identity.Alive {
		_ = command.Process.Kill()
		return identity.Ref{}, fmt.Errorf("component identity: %s %v", state, err)
	}
	go func() { _ = command.Wait() }()
	return exact.Ref(), nil
}

// Hold waits on an owned pipe until the helper is signalled.
func Hold() error {
	reader, writer, err := os.Pipe()
	if err != nil {
		return err
	}
	defer reader.Close()
	defer writer.Close()
	var buffer [1]byte
	_, err = reader.Read(buffer[:])
	return err
}
