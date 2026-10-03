package deploy

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
)

// OriginURL is the fetch URL of the checkout's origin, the repository's
// name for ProjectKey.
func OriginURL(root string) (string, error) {
	return git(root, "remote", "get-url", "origin")
}

// CheckoutGit is the runner's Git in the checkout at root: it fetches
// origin's main the way landing push does, and makes clean trees as
// detached worktrees of that checkout.
func CheckoutGit(root string) Git {
	return Git{
		FetchMain: func() (string, error) {
			if _, err := git(root, "fetch", "--quiet", "origin", "+refs/heads/main:refs/remotes/origin/main"); err != nil {
				return "", err
			}
			return git(root, "rev-parse", "--verify", "refs/remotes/origin/main^{commit}")
		},
		AddTree: func(dir, commit string) error {
			_, err := git(root, "worktree", "add", "--detach", "--force", dir, commit)
			return err
		},
		RemoveTree: func(dir string) error {
			// A tree another checkout of the repository registered is not
			// this checkout's to remove through git; its files go, and that
			// checkout prunes its registration.
			_, _ = git(root, "worktree", "remove", "--force", dir)
			if err := os.RemoveAll(dir); err != nil {
				return err
			}
			_, _ = git(root, "worktree", "prune")
			return nil
		},
	}
}

func git(root string, args ...string) (string, error) {
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	command.Env = gittree.ScrubbedEnviron()
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}
