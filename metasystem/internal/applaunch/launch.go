package applaunch

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/lifecycle"
)

// The detached spawn is the interface's, unchanged: its own session, a log
// it opens before the child exists, and a readiness pipe on descriptor 3.
// Sharing it is the point — one way to start a detached child on this seat,
// and the interface's verbs keep their own words because only the words
// around it differ.
type (
	LaunchSpec = lifecycle.LaunchSpec
	Spawn      = lifecycle.Spawn
)

// ExecSpawn is the engine's detached spawn.
var ExecSpawn Spawn = lifecycle.ExecSpawn

// ServeArgs is the argument vector that makes an engine the supervisor of
// one run.
func ServeArgs(repo, installation, key, ref, goal, address string) []string {
	args := []string{"app", "serve", "--repo", repo, "--metasystem-root", installation, "--key", key, "--ready-fd", "3"}
	if ref != "" {
		args = append(args, "--at", ref)
	}
	if goal != "" {
		args = append(args, "--goal", goal)
	}
	if address != "" {
		args = append(args, "--address", address)
	}
	return args
}

// LaunchSupervisor starts one run's supervisor detached and returns when it
// reports ready or failed, or when the wait runs out. A supervisor that has
// not reported by then is ended, because a run nobody waited for is a run
// nobody owns.
func LaunchSupervisor(spec LaunchSpec, spawn Spawn, wait time.Duration) (string, int, error) {
	if spawn == nil {
		spawn = ExecSpawn
	}
	if wait <= 0 {
		wait = time.Duration(DefaultReadyMS) * time.Millisecond
	}
	if err := os.MkdirAll(directoryOf(spec.LogPath), 0o755); err != nil {
		return "", 0, fmt.Errorf("cannot start the application: %w", err)
	}
	child, err := spawn(spec)
	if err != nil {
		return "", 0, fmt.Errorf("cannot start the application: %w", err)
	}
	pid := child.Pid()
	line, err := child.ReadyLine(wait)
	if err == nil {
		if address, ok := strings.CutPrefix(line, "ready "); ok {
			_ = child.Release()
			return strings.TrimSpace(address), pid, nil
		}
		if message, ok := strings.CutPrefix(line, "failed "); ok {
			_ = child.Release()
			Reap(pid)
			return "", 0, errors.New(message)
		}
	}
	_ = child.Kill()
	// A launcher that killed its supervisor reaps it. An unreaped supervisor
	// stays in the process table as a zombie, and a reader that re-proves
	// identity would read that zombie as a living owner.
	Reap(pid)
	return "", 0, fmt.Errorf("the application did not become ready; see %s", spec.LogPath)
}

// Reap collects one exited child of this process, briefly. It is a no-op for
// a process this one did not start.
func Reap(pid int) {
	if pid <= 0 {
		return
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		var status syscall.WaitStatus
		reaped, err := syscall.Wait4(pid, &status, syscall.WNOHANG, nil)
		if reaped == pid || err != nil {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func directoryOf(path string) string {
	if index := strings.LastIndexByte(path, '/'); index > 0 {
		return path[:index]
	}
	return "."
}

// Tail returns the last lines of a run's log.
func Tail(path string, lines int) ([]string, error) {
	if lines <= 0 {
		lines = 40
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	ring := make([]string, 0, lines)
	for scanner.Scan() {
		if len(ring) == lines {
			ring = ring[1:]
		}
		ring = append(ring, scanner.Text())
	}
	return ring, scanner.Err()
}

// Follow prints the log from its end as it grows, until the context ends.
// It is the reader of the file the engine captured or the file the contract
// named; it never holds the writer open.
func Follow(ctx context.Context, path string, out io.Writer, poll time.Duration) error {
	if poll <= 0 {
		poll = 200 * time.Millisecond
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Seek(0, io.SeekEnd); err != nil {
		return err
	}
	reader := bufio.NewReader(file)
	for {
		line, err := reader.ReadString('\n')
		if line != "" {
			if _, writeErr := io.WriteString(out, line); writeErr != nil {
				return writeErr
			}
		}
		if err == nil {
			continue
		}
		if !errors.Is(err, io.EOF) {
			return err
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(poll):
		}
	}
}

// Rejoin waits for a run whose supervisor is already alive to be ready. A
// second start does not start a second application: it rejoins the one that
// runs, and it still waits for readiness before it says started, because a
// caller that is told "started" and finds nothing answering has been told
// something that was not observed.
func Rejoin(ctx context.Context, stateRoot, key string, contract Contract, o ReadOptions, wait time.Duration) (Status, error) {
	if wait <= 0 {
		wait = time.Duration(contract.ReadyWaitMS()) * time.Millisecond
	}
	deadline := time.Now().Add(wait)
	for {
		status, err := Read(stateRoot, key, contract, o)
		if err != nil {
			return status, err
		}
		switch {
		case status.State == Running && (status.Readiness == Answering || status.Readiness == ObservedOnce || status.Readiness == NoProbe):
			return status, nil
		case status.State != Running && status.State != Starting:
			return status, errors.New("the run is " + string(status.State) + ", not starting")
		}
		if !time.Now().Before(deadline) {
			return status, errors.New("the running application did not become ready in time")
		}
		select {
		case <-ctx.Done():
			return status, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}
