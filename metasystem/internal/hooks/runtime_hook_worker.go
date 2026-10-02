package hooks

import (
	"errors"
	"os"
	"os/exec"
	"syscall"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
)

// engineWorker is a launched Stop worker, reaped as soon as it exits.
type engineWorker struct {
	pid    int
	done   chan struct{}
	status int
}

func (w *engineWorker) Pid() int              { return w.pid }
func (w *engineWorker) Done() <-chan struct{} { return w.done }
func (w *engineWorker) Status() int           { return w.status }

// LaunchEngineWorker is the production Stop worker launcher: the engine
// running the deadline parent runs its own `internal hook RUNTIME stop`
// entry as the parent's child, in the installation directory, with the
// payload as its input (VOA-18). No script stands between the parent and
// its worker; the deadline-parent identity rides in env.
func LaunchEngineWorker(installation, runtime string, env []string, stdin, stdout, stderr *os.File) (Worker, error) {
	engine, err := os.Executable()
	if err != nil {
		return nil, err
	}
	return launchWorker(engine, installation, runtime, env, stdin, stdout, stderr)
}

func launchWorker(engine, installation, runtime string, env []string, stdin, stdout, stderr *os.File) (Worker, error) {
	command := exec.Command(engine, "internal", "hook", runtime, "stop")
	command.Dir = installation
	command.Env = env
	command.Stdin, command.Stdout, command.Stderr = stdin, stdout, stderr
	// The worker holds a writer lock of the scratch root of its own: a Stop
	// killed at its budget leaves a root its surviving worker still holds.
	if err := diskstore.StartChild(command); err != nil {
		return nil, err
	}
	worker := &engineWorker{pid: command.Process.Pid, done: make(chan struct{})}
	go func() {
		err := command.Wait()
		worker.status = 0
		if err != nil {
			var exitError *exec.ExitError
			if errors.As(err, &exitError) {
				worker.status = exitError.ExitCode()
				if worker.status < 0 {
					worker.status = 128 + int(exitError.Sys().(syscall.WaitStatus).Signal())
				}
			} else {
				worker.status = 127
			}
		}
		close(worker.done)
	}()
	return worker, nil
}
