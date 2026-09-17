package batch

import (
	"bytes"
	"os"
	"os/exec"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/contractgit"
)

var batchMergeDriverArgs = func() ([]string, error) { return contractgit.DriverArgs(os.Executable) }

func batchMergeGitCommand(root string, args ...string) ([]string, error) {
	driver, err := batchMergeDriverArgs()
	if err != nil {
		return nil, err
	}
	full := []string{"-C", root}
	full = append(full, driver...)
	return append(full, args...), nil
}

func runBatchMergeGit(root string, input []byte, args ...string) ([]byte, error) {
	full, err := batchMergeGitCommand(root, args...)
	if err != nil {
		return nil, err
	}
	command := exec.Command("git", full...)
	command.Env, command.Stdin = gittree.ScrubbedEnviron(), bytes.NewReader(input)
	return command.CombinedOutput()
}
