package batch

import (
	"errors"
	"os/exec"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/contractgit"
)

// patchCompositionConflict recognizes only Git evidence that a patch cannot
// apply to this tree. Other command, contract, or policy failures are not a
// basis for returning a batch member.
func patchCompositionConflict(root string, output []byte, applyErr error) bool {
	var exit *exec.ExitError
	if !errors.As(applyErr, &exit) || exit.ExitCode() != 1 {
		return false
	}
	command := exec.Command("git", "-C", root, "diff", "--name-only", "--diff-filter=U", "-z")
	command.Env = gittree.ScrubbedEnviron("LC_ALL=C")
	unmerged, err := command.Output()
	if err != nil {
		return false
	}
	hasOrdinaryConflict := false
	for _, path := range strings.Split(strings.TrimSuffix(string(unmerged), "\x00"), "\x00") {
		if path == "" {
			continue
		}
		if path == contractgit.TestingContractPath {
			return false
		}
		hasOrdinaryConflict = true
	}
	if hasOrdinaryConflict {
		return true
	}
	hasPathConflict := false
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if strings.HasPrefix(line, "fatal: ") {
			return false
		}
		if !strings.HasPrefix(line, "error: ") {
			continue
		}
		matched := false
		for _, suffix := range []string{": does not exist in index", ": already exists in working directory", ": already exists in index"} {
			if path, found := strings.CutSuffix(strings.TrimPrefix(line, "error: "), suffix); found && path != "" && path != contractgit.TestingContractPath {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
		hasPathConflict = true
	}
	return hasPathConflict
}

type patchApplyConflict struct{ cause error }

func (conflict *patchApplyConflict) Error() string { return conflict.cause.Error() }
func (conflict *patchApplyConflict) Unwrap() error { return conflict.cause }
