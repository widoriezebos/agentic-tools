package testutil

import (
	"io"
	"os"
	"os/exec"
	"testing"
)

// HeldProcess owns a child whose command publishes one readiness byte on
// descriptor 3 and then waits for standard input to close. The command's
// argv, process attributes, signals, and output streams remain caller-owned.
type HeldProcess struct {
	Command *exec.Cmd
	release *os.File
}

// StartHeldProcess starts command and waits until it has entered its
// release-held lifetime. Commands passed here must leave stdin and inherited
// files available for the hold protocol.
func StartHeldProcess(t testing.TB, command *exec.Cmd) *HeldProcess {
	t.Helper()
	if command.Stdin != nil || len(command.ExtraFiles) != 0 {
		t.Fatalf("held process requires unassigned stdin and inherited files")
	}
	readyRead, readyWrite, err := os.Pipe()
	if err != nil {
		t.Fatalf("create held-process readiness pipe: %v", err)
	}
	releaseRead, releaseWrite, err := os.Pipe()
	if err != nil {
		_ = readyRead.Close()
		_ = readyWrite.Close()
		t.Fatalf("create held-process release pipe: %v", err)
	}
	command.Stdin = releaseRead
	command.ExtraFiles = []*os.File{readyWrite}
	if err := command.Start(); err != nil {
		_ = readyRead.Close()
		_ = readyWrite.Close()
		_ = releaseRead.Close()
		_ = releaseWrite.Close()
		t.Fatalf("start held process: %v", err)
	}
	_ = readyWrite.Close()
	_ = releaseRead.Close()
	held := &HeldProcess{Command: command, release: releaseWrite}
	t.Cleanup(func() { _ = held.Release() })
	var ready [1]byte
	if _, err := io.ReadFull(readyRead, ready[:]); err != nil || ready[0] != 'x' {
		_ = readyRead.Close()
		t.Fatalf("held process %d did not publish readiness: byte=%q err=%v", command.Process.Pid, ready, err)
	}
	_ = readyRead.Close()
	return held
}

// Release closes the process's owned lifetime and joins it. A caller that
// already killed and joined the command may still call Release safely.
func (h *HeldProcess) Release() error {
	if h == nil || h.Command == nil {
		return nil
	}
	if h.release != nil {
		_ = h.release.Close()
		h.release = nil
	}
	if h.Command.ProcessState != nil {
		return nil
	}
	return h.Command.Wait()
}

// Kill terminates the held process and joins it.
func (h *HeldProcess) Kill() error {
	if h == nil || h.Command == nil || h.Command.ProcessState != nil {
		return nil
	}
	killErr := h.Command.Process.Kill()
	if h.release != nil {
		_ = h.release.Close()
		h.release = nil
	}
	waitErr := h.Command.Wait()
	if killErr != nil {
		return killErr
	}
	return waitErr
}
