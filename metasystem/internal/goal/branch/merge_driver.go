package branch

import (
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/contractgit"
)

var branchMergeDriverArgs = func() ([]string, error) { return contractgit.DriverArgs(os.Executable) }

func branchGitCommand(repo string, args ...string) ([]string, error) {
	driver, err := branchMergeDriverArgs()
	if err != nil {
		return nil, err
	}
	full := []string{"-C", repo}
	full = append(full, driver...)
	return append(full, gittree.ParallelCheckoutArgs(args)...), nil
}
