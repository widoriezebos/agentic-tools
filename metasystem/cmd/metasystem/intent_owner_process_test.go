package main

import (
	"bytes"
	"errors"
	"os/exec"
)

// runIntentOwnerProcess runs one owner with its own output pipes: the test
// beds' real engine process behind their fake owner routes.
func runIntentOwnerProcess(process intentProcess) intentProcessResult {
	command := exec.Command(process.argv[0], process.argv[1:]...)
	command.Dir = process.dir
	command.Stdin = nil
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()
	result := intentProcessResult{stdout: stdout.Bytes(), stderr: stderr.Bytes(), code: commandExitCode(err)}
	var exit *exec.ExitError
	if err != nil && !errors.As(err, &exit) {
		result.err = err
	}
	return result
}
