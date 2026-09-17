package branch

import (
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/contractgit"
)

var branchMergeDriverArgs = func() []string { return contractgit.DriverArgs(os.Executable) }

func branchGitCommand(repo string, args ...string) []string {
	full := []string{"-C", repo}
	full = append(full, branchMergeDriverArgs()...)
	return append(full, args...)
}
