package testexec

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
)

// ReadyFD is the descriptor on which a child started by StartReady reports
// that its own image is running.
const ReadyFD = 3

// ReadyPrologue is the shell text a /bin/sh or bash child runs first to
// report readiness from its running image and then close the descriptor, so
// nothing it starts later inherits it. A shell command started by
// StartReady begins with it.
const ReadyPrologue = `printf 'ready\n' >&3; exec 3>&-; `

// StartReady starts command and returns only once the child's own running
// image has written "ready\n" on descriptor ReadyFD (ReadyPrologue in a
// shell, ReportReady in a Go helper main).
//
// On Linux, Start returns once the exec has passed its point of no return,
// but /proc/<pid>/cmdline reads empty until the new image has laid out its
// arguments: an argv, command or tag read of a child right after Start can
// see nothing. A child that reports from its own image is past that window,
// so every identity read of a child that needs its command line starts the
// child here.
//
// The command must pass no inherited files of its own: the readiness
// descriptor is descriptor 3. A child that exits or closes the descriptor
// without reporting is killed if still running, reaped, and an error.
func StartReady(command *exec.Cmd) error {
	if len(command.ExtraFiles) != 0 {
		return errors.New("testexec: StartReady owns descriptor 3; the command must pass no inherited files")
	}
	readyRead, readyWrite, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("testexec: readiness pipe: %w", err)
	}
	defer readyRead.Close()
	command.ExtraFiles = []*os.File{readyWrite}
	startErr := command.Start()
	_ = readyWrite.Close()
	if startErr != nil {
		return startErr
	}
	line, readErr := bufio.NewReader(readyRead).ReadString('\n')
	if readErr == nil && line == "ready\n" {
		return nil
	}
	_ = command.Process.Kill()
	_ = command.Wait()
	return fmt.Errorf("testexec: child %d did not report ready: read %q: %v", command.Process.Pid, line, readErr)
}

// ReportReady is a Go helper main's side of StartReady: it writes the
// readiness line on descriptor ReadyFD and closes it. Only a helper started
// by StartReady calls it.
func ReportReady() error {
	ready := os.NewFile(ReadyFD, "testexec-ready")
	if ready == nil {
		return errors.New("testexec: no readiness descriptor")
	}
	_, writeErr := ready.WriteString("ready\n")
	closeErr := ready.Close()
	return errors.Join(writeErr, closeErr)
}
