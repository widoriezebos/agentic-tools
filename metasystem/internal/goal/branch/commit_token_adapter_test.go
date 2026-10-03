package branch

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/landpath"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/validate"
)

// scratchTokenGuard runs in Git's hook subprocess so the real guard must
// prove the token's process ancestry and kernel start second.
func scratchTokenGuard(prefix, log string) int {
	worktree, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	root := filepath.Join(worktree, prefix)
	status := landpath.Guard(landpath.GuardOwners{
		CallerPID: int64(os.Getpid()),
		Classify:  func(string, int64) (string, error) { return "AGENT", nil },
		WrapperToken: func(path string, pid int64) bool {
			return validate.WrapperToken(path, pid, validate.KernelProcessTree{})
		},
		AllowNewPlan: true,
		Git: func(call landpath.GitCall) landpath.GitResult {
			out, err := exec.Command("git", append([]string{"-C", call.Dir}, call.Args...)...).Output()
			code := 0
			if err != nil {
				code = 1
			}
			return landpath.GitResult{Stdout: out, Code: code}
		},
	}, root, worktree, os.Stdout, os.Stderr)
	if status != 0 {
		return status
	}
	file, err := os.OpenFile(log, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer file.Close()
	if _, err := fmt.Fprintln(file, landpath.TokenPath(root)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
