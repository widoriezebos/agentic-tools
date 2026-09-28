package gaterun

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

// RunGuardMember is the checkout execution guard's member wrapper, the Go
// form of the retired checkout-execution-guard.sh run-member: the wrapper is
// the registered guard member, it runs argv as its child in its own process
// group, and it releases its membership when the child exits or when it is
// itself terminated (exit 143 on TERM, 130 on INT), so the guard frees as soon
// as the launched work ends. It returns the child's exit status.
func RunGuardMember(root string, argv []string, stdout, stderr io.Writer) int {
	if root == "" || len(argv) == 0 {
		fmt.Fprintln(stderr, "checkout execution guard wrapper: root and command are required")
		return 2
	}
	release := func() { _ = ReleaseExecutionGuard(root, int64(os.Getpid())) }
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(signals)
	command := exec.Command(argv[0], argv[1:]...)
	command.Stdin, command.Stdout, command.Stderr = os.Stdin, stdout, stderr
	if err := command.Start(); err != nil {
		release()
		fmt.Fprintln(stderr, err)
		return 127
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	select {
	case err := <-done:
		release()
		return exitStatus(err)
	case received := <-signals:
		release()
		if received == syscall.SIGINT {
			return 130
		}
		return 143
	}
}

func exitStatus(err error) int {
	if err == nil {
		return 0
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		if status, ok := exit.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			return 128 + int(status.Signal())
		}
		return exit.ExitCode()
	}
	return 1
}
