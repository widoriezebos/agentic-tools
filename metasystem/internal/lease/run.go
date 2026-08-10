package lease

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
)

// runProcess runs the command run-held was asked to gate and returns its exit
// code. This is the one legitimate exec in the lease: run-held's whole job is
// to invoke a command under the lease lock, so launching it is invocation, not
// a decision shelling out to a tool.
func runProcess(argv []string) (int, error) {
	if len(argv) == 0 {
		return 2, fmt.Errorf("run-held requires a command")
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err == nil {
		return 0, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode(), nil
	}
	return 1, err
}
