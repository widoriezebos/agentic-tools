package landing

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
)

// Transport mirrors origin, never the local branch: SyncTransport fetches
// origin's branch head by its FULL ref into the exact tracking ref, then
// pushes that tracking ref to the transport remote — a commit origin has not
// accepted, a tag shadowing the branch name, or a stale tracking ref cannot
// select what lands. The branch is validated as one whole byte string before
// git sees it; an explicitly empty branch is an error, not a default.

// TransportGit runs one git command in dir and returns its combined output.
type TransportGit func(dir string, args ...string) (string, error)

// TransportError is a refused or failed transport sync; Code is the exit
// status the verb reports (2 for an unlawful branch, 1 otherwise).
type TransportError struct {
	Code   int
	Detail string
}

func (e *TransportError) Error() string { return e.Detail }

var plainTransportBranch = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]*$`)

// SyncTransport mirrors origin's branch to the transport remote from the
// checkout at root and returns the last line of the push's output. A nil git
// runs the real git with the scrubbed environment.
func SyncTransport(root, branch string, git TransportGit) (string, error) {
	if git == nil {
		git = runTransportGit
	}
	if branch == "" || strings.HasPrefix(branch, "-") || strings.Contains(branch, "..") ||
		strings.Contains(branch, "//") || strings.ContainsAny(branch, " \n\t") ||
		!plainTransportBranch.MatchString(branch) {
		return "", &TransportError{Code: 2, Detail: fmt.Sprintf("sync-transport refused: branch name '%s' is not a plain branch", branch)}
	}
	if _, err := git(root, "fetch", "--quiet", "origin", "+refs/heads/"+branch+":refs/remotes/origin/"+branch); err != nil {
		return "", &TransportError{Code: 1, Detail: fmt.Sprintf("sync-transport refused: origin has no branch '%s'", branch)}
	}
	output, err := git(root, "push", "transport", "refs/remotes/origin/"+branch+":refs/heads/"+branch)
	last := lastTransportLine(output)
	if err != nil {
		detail := "sync-transport: push to transport failed"
		if last != "" {
			detail += ": " + last
		}
		return last, &TransportError{Code: 1, Detail: detail}
	}
	return last, nil
}

func lastTransportLine(output string) string {
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	return lines[len(lines)-1]
}

func runTransportGit(dir string, args ...string) (string, error) {
	command := exec.Command("git", append([]string{"-C", dir}, args...)...)
	command.Env = gittree.ScrubbedEnviron()
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output
	err := command.Run()
	var exit *exec.ExitError
	if err != nil && !errors.As(err, &exit) {
		return output.String(), err
	}
	return output.String(), err
}
