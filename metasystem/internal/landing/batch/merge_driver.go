package batch

import (
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/contractgit"
)

var batchMergeDriverArgs = func() []string { return contractgit.DriverArgs(os.Executable) }

func batchMergeGitCommand(root string, args ...string) []string {
	full := []string{"-C", root}
	full = append(full, batchMergeDriverArgs()...)
	return append(full, args...)
}
