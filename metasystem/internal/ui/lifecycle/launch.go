package lifecycle

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const defaultReadyWait = 10 * time.Second

var ErrReadyTimeout = errors.New("interface readiness timed out")

func ServeArgs(checkout, metasystemRoot, listen string) []string {
	return []string{
		"ui", "serve",
		"--repo", checkout,
		"--metasystem-root", metasystemRoot,
		"--listen", listen,
		"--ready-fd", "3",
	}
}

type LaunchSpec struct {
	Executable string
	Args       []string
	Dir        string
	LogPath    string
}

type Child interface {
	ReadyLine(wait time.Duration) (string, error)
	Pid() int
	Kill() error
	Release() error
}

type Spawn func(LaunchSpec) (Child, error)

func Launch(spec LaunchSpec, spawn Spawn, readyWait time.Duration) (address string, pid int, err error) {
	// The state directory is the one the log lives in, under the state root; the
	// launcher opens the log there, so it is created before the child is spawned.
	if err := os.MkdirAll(filepath.Dir(spec.LogPath), 0o755); err != nil {
		return "", 0, fmt.Errorf("cannot launch the interface server: %w", err)
	}
	child, err := spawn(spec)
	if err != nil {
		return "", 0, fmt.Errorf("cannot launch the interface server: %w", err)
	}
	if readyWait == 0 {
		readyWait = defaultReadyWait
	}
	line, err := child.ReadyLine(readyWait)
	if err == nil {
		if address, ok := strings.CutPrefix(line, "ready "); ok && address != "" {
			pid := child.Pid()
			_ = child.Release()
			return address, pid, nil
		}
		if message, ok := strings.CutPrefix(line, "failed "); ok {
			return "", 0, errors.New(message)
		}
	}
	_ = child.Kill()
	return "", 0, fmt.Errorf("the interface server did not become ready; see %s", spec.LogPath)
}

func ExecSpawn(spec LaunchSpec) (Child, error) {
	readyRead, readyWrite, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	closePipe := func() {
		_ = readyRead.Close()
		_ = readyWrite.Close()
	}

	nullInput, err := os.Open(os.DevNull)
	if err != nil {
		closePipe()
		return nil, err
	}
	log, err := os.OpenFile(spec.LogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		_ = nullInput.Close()
		closePipe()
		return nil, err
	}

	command := exec.Command(spec.Executable, spec.Args...)
	command.Dir = spec.Dir
	command.Stdin = nullInput
	command.Stdout = log
	command.Stderr = log
	command.ExtraFiles = []*os.File{readyWrite}
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := command.Start(); err != nil {
		_ = log.Close()
		_ = nullInput.Close()
		closePipe()
		return nil, err
	}
	_ = log.Close()
	_ = nullInput.Close()
	_ = readyWrite.Close()
	return &execChild{process: command.Process, ready: readyRead}, nil
}

type execChild struct {
	process *os.Process
	ready   *os.File
}

func (c *execChild) ReadyLine(wait time.Duration) (string, error) {
	defer c.ready.Close()
	if err := c.ready.SetReadDeadline(time.Now().Add(wait)); err != nil {
		return "", err
	}
	line, err := bufio.NewReader(c.ready).ReadString('\n')
	if err != nil {
		if errors.Is(err, os.ErrDeadlineExceeded) {
			return "", ErrReadyTimeout
		}
		return "", err
	}
	return strings.TrimSuffix(line, "\n"), nil
}

func (c *execChild) Pid() int { return c.process.Pid }

func (c *execChild) Kill() error {
	_ = c.ready.Close()
	return c.process.Kill()
}

func (c *execChild) Release() error {
	_ = c.ready.Close()
	return c.process.Release()
}
