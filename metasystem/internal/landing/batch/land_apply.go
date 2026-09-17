package batch

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
)

// ApplyCertifiedPatch stages exactly the transported, certified chain patch.
func ApplyCertifiedPatch(root, worktree, chain string) error {
	patch, err := os.ReadFile(filepath.Join(root, "artifacts", "agents", "landing-batches", "chains", chain, "diff.patch"))
	if err != nil {
		return err
	}
	command := exec.Command("git", "-C", worktree, "-c", "core.useReplaceRefs=false", "-c", "core.hooksPath=/dev/null", "apply", "--index", "--3way", "--binary", "--whitespace=nowarn", "-")
	command.Env, command.Stdin = gittree.ScrubbedEnviron(), bytes.NewReader(patch)
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("apply certified patch for %s: %s: %w", chain, bytes.TrimSpace(output), err)
	}
	return nil
}
